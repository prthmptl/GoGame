package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/prathpatel/gogame-backend/internal/archive"
	"github.com/prathpatel/gogame-backend/internal/auth"
	"github.com/prathpatel/gogame-backend/internal/notify"
	"github.com/prathpatel/gogame-backend/internal/profile"
	"github.com/prathpatel/gogame-backend/internal/store"
)

// Server holds the API dependencies.
type Server struct {
	store   *store.Store
	auth    *auth.Service
	profile *profile.Service
	archive *archive.Service
	notify  *notify.Service
	log     *slog.Logger
	version string
}

// Deps are the services the API depends on.
type Deps struct {
	Store   *store.Store
	Auth    *auth.Service
	Profile *profile.Service
	Archive *archive.Service
	Notify  *notify.Service
	Log     *slog.Logger
	Version string
}

// New builds a Server.
func New(d Deps) *Server {
	return &Server{
		store: d.Store, auth: d.Auth, profile: d.Profile, archive: d.Archive,
		notify: d.Notify, log: d.Log, version: d.Version,
	}
}

// Routes returns the HTTP handler for the API.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	// C1: health check + metrics.
	mux.HandleFunc("GET /healthz", withObservability(s.log, "/healthz", s.handleLive))
	mux.HandleFunc("GET /readyz", withObservability(s.log, "/readyz", s.handleReady))
	mux.Handle("GET /metrics", promhttp.Handler())

	// C2: auth.
	mux.HandleFunc("POST /auth/guest", withObservability(s.log, "/auth/guest", s.handleGuest))
	mux.HandleFunc("POST /auth/google", withObservability(s.log, "/auth/google", s.handleGoogle))
	mux.HandleFunc("POST /auth/refresh", withObservability(s.log, "/auth/refresh", s.handleRefresh))

	// C3: profile, sync, achievements, leaderboards.
	get := func(path string, h http.HandlerFunc) {
		mux.HandleFunc("GET "+path, withObservability(s.log, path, s.authenticated(h)))
	}
	post := func(path string, h http.HandlerFunc) {
		mux.HandleFunc("POST "+path, withObservability(s.log, path, s.authenticated(h)))
	}
	get("/users/me", s.handleGetMe)
	mux.HandleFunc("PATCH /users/me", withObservability(s.log, "/users/me", s.authenticated(s.handlePatchMe)))
	post("/users/me/sync", s.handleSync)
	get("/users/me/achievements", s.handleAchievements)
	get("/leaderboards/global", s.handleLeaderboard)

	// C4: game archive.
	get("/games", s.handleListGames)
	get("/games/{id}", s.handleGetGame)
	get("/games/{id}/moves", s.handleGameMoves)
	get("/games/{id}/sgf", s.handleGameSGF)

	// C5: notifications.
	post("/notifications/devices", s.handleRegisterDevice)
	get("/notifications/prefs", s.handleGetNotificationPrefs)
	mux.HandleFunc("PUT /notifications/prefs", withObservability(s.log, "/notifications/prefs", s.authenticated(s.handleSetNotificationPref)))

	return mux
}

// handleLive reports process liveness. It must not touch dependencies:
// a failing database should not cause the orchestrator to restart the pod.
func (s *Server) handleLive(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"version": s.version,
	})
}

// handleReady reports whether this instance can serve traffic, checking
// Postgres and Redis. Load balancers use this to drain an unhealthy instance.
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	checks := map[string]string{"postgres": "ok", "redis": "ok"}
	healthy := true
	if err := s.store.DB.Ping(ctx); err != nil {
		checks["postgres"] = err.Error()
		healthy = false
	}
	if err := s.store.Redis.Ping(ctx).Err(); err != nil {
		checks["redis"] = err.Error()
		healthy = false
	}

	status := http.StatusOK
	if !healthy {
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, map[string]any{
		"status":  map[bool]string{true: "ok", false: "degraded"}[healthy],
		"version": s.version,
		"checks":  checks,
	})
}

type guestRequest struct {
	DisplayName string `json:"displayName"`
}

type authResponse struct {
	User   *auth.User      `json:"user"`
	Tokens *auth.TokenPair `json:"tokens"`
}

func (s *Server) handleGuest(w http.ResponseWriter, r *http.Request) {
	var req guestRequest
	// An empty body is valid: the server picks the default display name.
	if err := decodeJSON(r, &req); err != nil && !errors.Is(err, errEmptyBody) {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	pair, user, err := s.auth.Guest(r.Context(), req.DisplayName, metaFrom(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, authResponse{User: user, Tokens: pair})
}

type googleRequest struct {
	IDToken string `json:"idToken"`
}

func (s *Server) handleGoogle(w http.ResponseWriter, r *http.Request) {
	var req googleRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	if strings.TrimSpace(req.IDToken) == "" {
		writeError(w, http.StatusBadRequest, "missing_id_token", "idToken is required")
		return
	}

	// A valid access token on this call means "link Google to the account I
	// am already signed in as" — the guest-upgrade path. An absent or invalid
	// token is not an error here; it just means a plain sign-in.
	var currentUserID *uuid.UUID
	if bearer := bearerToken(r); bearer != "" {
		if claims, err := s.auth.ParseAccessToken(bearer); err == nil {
			if id, err := claims.UserID(); err == nil {
				currentUserID = &id
			}
		}
	}

	pair, user, err := s.auth.Google(r.Context(), req.IDToken, currentUserID, metaFrom(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, authResponse{User: user, Tokens: pair})
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	if strings.TrimSpace(req.RefreshToken) == "" {
		writeError(w, http.StatusBadRequest, "missing_refresh_token", "refreshToken is required")
		return
	}
	pair, err := s.auth.Refresh(r.Context(), req.RefreshToken, metaFrom(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tokens": pair})
}

// fail maps domain errors to status codes, logging anything unexpected.
func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, auth.ErrTokenReplayed):
		// 401 with a distinct code: the client must discard its tokens and
		// send the user back through sign-in.
		writeError(w, http.StatusUnauthorized, "refresh_reuse_detected",
			"refresh token was replayed; all sessions revoked")
	case errors.Is(err, auth.ErrTokenExpired):
		writeError(w, http.StatusUnauthorized, "token_expired", "refresh token expired")
	case errors.Is(err, auth.ErrTokenRevoked):
		writeError(w, http.StatusUnauthorized, "token_revoked", "refresh token revoked")
	case errors.Is(err, auth.ErrInvalidToken):
		writeError(w, http.StatusUnauthorized, "invalid_token", "token is not valid")
	case errors.Is(err, profile.ErrNotFound), errors.Is(err, archive.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "not found")
	case errors.Is(err, auth.ErrIdentityTaken):
		writeError(w, http.StatusConflict, "identity_taken",
			"that Google account is already linked to another player")
	default:
		logging_(r).Error("request failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal error")
	}
}

func metaFrom(r *http.Request) auth.SessionMeta {
	return auth.SessionMeta{
		UserAgent: r.UserAgent(),
		IP:        clientIP(r),
	}
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if len(h) > 7 && strings.EqualFold(h[:7], "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

var errEmptyBody = errors.New("empty request body")

// decodeJSON reads a bounded JSON body and rejects unknown fields so a typo
// in a client payload fails loudly instead of being silently ignored.
func decodeJSON(r *http.Request, dst any) error {
	if r.Body == nil {
		return errEmptyBody
	}
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, http.ErrBodyReadAfterClose) {
			return errEmptyBody
		}
		if err.Error() == "EOF" {
			return errEmptyBody
		}
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}
