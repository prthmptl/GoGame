package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/prathpatel/gogame-backend/internal/anticheat"
	"github.com/prathpatel/gogame-backend/internal/archive"
	"github.com/prathpatel/gogame-backend/internal/auth"
	"github.com/prathpatel/gogame-backend/internal/billing"
	"github.com/prathpatel/gogame-backend/internal/chat"
	"github.com/prathpatel/gogame-backend/internal/clubs"
	"github.com/prathpatel/gogame-backend/internal/coaching"
	"github.com/prathpatel/gogame-backend/internal/correspondence"
	"github.com/prathpatel/gogame-backend/internal/game"
	"github.com/prathpatel/gogame-backend/internal/matchmaking"
	"github.com/prathpatel/gogame-backend/internal/notify"
	"github.com/prathpatel/gogame-backend/internal/openings"
	"github.com/prathpatel/gogame-backend/internal/profile"
	"github.com/prathpatel/gogame-backend/internal/progames"
	"github.com/prathpatel/gogame-backend/internal/rating"
	"github.com/prathpatel/gogame-backend/internal/rooms"
	"github.com/prathpatel/gogame-backend/internal/social"
	"github.com/prathpatel/gogame-backend/internal/store"
	"github.com/prathpatel/gogame-backend/internal/tournament"
	"github.com/prathpatel/gogame-backend/internal/ws"
)

// Server holds the API dependencies.
type Server struct {
	store          *store.Store
	auth           *auth.Service
	profile        *profile.Service
	archive        *archive.Service
	notify         *notify.Service
	hub            *game.Hub
	matchmaking    *matchmaking.Service
	rooms          *rooms.Service
	chat           *chat.Service
	ws             *ws.Server
	rating         *rating.Service
	anticheat      *anticheat.Service
	correspondence *correspondence.Service
	tournament     *tournament.Service
	clubs          *clubs.Service
	social         *social.Service
	openings       *openings.Service
	progames       *progames.Service
	coaching       *coaching.Service
	billing        *billing.Service
	iap            *billing.IAPService
	log            *slog.Logger
	version        string
}

// Deps are the services the API depends on.
type Deps struct {
	Store          *store.Store
	Auth           *auth.Service
	Profile        *profile.Service
	Archive        *archive.Service
	Notify         *notify.Service
	Hub            *game.Hub
	Matchmaking    *matchmaking.Service
	Rooms          *rooms.Service
	Chat           *chat.Service
	WS             *ws.Server
	Rating         *rating.Service
	Anticheat      *anticheat.Service
	Correspondence *correspondence.Service
	Tournament     *tournament.Service
	Clubs          *clubs.Service
	Social         *social.Service
	Openings       *openings.Service
	ProGames       *progames.Service
	Coaching       *coaching.Service
	Billing        *billing.Service
	IAP            *billing.IAPService
	Log            *slog.Logger
	Version        string
}

// New builds a Server.
func New(d Deps) *Server {
	return &Server{
		store: d.Store, auth: d.Auth, profile: d.Profile, archive: d.Archive,
		notify: d.Notify, hub: d.Hub, matchmaking: d.Matchmaking,
		rooms: d.Rooms, chat: d.Chat, ws: d.WS,
		rating: d.Rating, anticheat: d.Anticheat,
		correspondence: d.Correspondence, tournament: d.Tournament,
		clubs: d.Clubs, social: d.Social, openings: d.Openings,
		progames: d.ProGames, coaching: d.Coaching,
		billing: d.Billing, iap: d.IAP,
		log: d.Log, version: d.Version,
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
	mux.HandleFunc("POST /auth/guest", withObservability(s.log, "/auth/guest", s.limitAuth(20, s.handleGuest)))
	mux.HandleFunc("POST /auth/google", withObservability(s.log, "/auth/google", s.limitAuth(30, s.handleGoogle)))
	mux.HandleFunc("POST /auth/refresh", withObservability(s.log, "/auth/refresh", s.limitAuth(120, s.handleRefresh)))

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

	// D1: the WebSocket endpoint. Not wrapped in withObservability, whose
	// per-request logging and latency histogram are meaningless for a
	// long-lived connection.
	if s.ws != nil {
		mux.HandleFunc("GET /ws", s.ws.Handle)
	}

	// D4: matchmaking.
	post("/matchmaking/tickets", s.handleEnqueue)
	get("/matchmaking/tickets/{id}", s.handleGetTicket)
	mux.HandleFunc("DELETE /matchmaking/tickets/{id}",
		withObservability(s.log, "/matchmaking/tickets/{id}", s.authenticated(s.handleCancelTicket)))

	// D5: rooms and chat.
	post("/rooms", s.handleCreateRoom)
	get("/rooms/{code}", s.handleGetRoom)
	post("/rooms/{code}/join", s.handleJoinRoom)
	post("/rooms/{code}/start", s.handleStartRoom)
	get("/games/{id}/chat", s.handleChatHistory)

	// E1: ratings.
	get("/users/me/ratings", s.handleMyRatings)
	get("/users/me/ratings/history", s.handleRatingHistory)

	// E3: tournaments.
	get("/tournaments", s.handleListTournaments)
	get("/tournaments/{id}", s.handleGetTournament)
	get("/tournaments/{id}/standings", s.handleTournamentStandings)
	post("/tournaments/{id}/join", s.handleJoinTournament)
	post("/tournaments/{id}/withdraw", s.handleWithdrawTournament)

	// E4: correspondence.
	get("/games/{id}/correspondence", s.handleCorrespondenceState)
	get("/users/me/vacation", s.handleGetVacation)
	post("/users/me/vacation/start", s.handleStartVacation)
	post("/users/me/vacation/end", s.handleEndVacation)
	get("/games/{id}/conditional-moves", s.handleGetPlan)
	mux.HandleFunc("PUT /games/{id}/conditional-moves",
		withObservability(s.log, "/games/{id}/conditional-moves", s.authenticated(s.handleSetPlan)))

	// F1: clubs.
	get("/clubs", s.handleListClubs)
	post("/clubs", s.handleCreateClub)
	get("/clubs/{slug}", s.handleGetClub)
	get("/clubs/{slug}/members", s.handleClubMembers)
	post("/clubs/{slug}/join", s.handleJoinClub)
	post("/clubs/{slug}/leave", s.handleLeaveClub)
	get("/clubs/{slug}/threads", s.handleClubThreads)
	post("/clubs/{slug}/threads", s.handleCreateThread)
	get("/threads/{id}/posts", s.handleThreadPosts)
	post("/threads/{id}/posts", s.handleReplyToThread)

	// F2: friends, DMs, feed and reports.
	post("/users/{id}/follow", s.handleFollow)
	post("/users/{id}/unfollow", s.handleUnfollow)
	post("/users/{id}/block", s.handleBlock)
	get("/users/me/friends", s.handleFriends)
	get("/users/me/following", s.handleFollowing)
	post("/users/{id}/messages", s.handleSendDM)
	get("/users/{id}/messages", s.handleConversation)
	get("/users/me/feed", s.handleFeed)
	post("/reports", s.handleReport)

	// F3: opening explorer.
	post("/openings/lookup", s.handleOpeningLookup)

	// F4: pro games and relays.
	get("/pro-games", s.handleSearchProGames)
	get("/pro-games/{id}/sgf", s.handleProGameSGF)
	get("/relays", s.handleLiveRelays)
	get("/relays/{id}", s.handleGetRelay)

	// F5: coaching.
	get("/coaches", s.handleListCoaches)
	post("/coaches/apply", s.handleApplyAsCoach)
	get("/coaches/{id}/availability", s.handleCoachAvailability)
	mux.HandleFunc("PUT /coaches/me/availability",
		withObservability(s.log, "/coaches/me/availability", s.authenticated(s.handleSetAvailability)))
	post("/coaches/{id}/book", s.handleBookCoach)
	get("/users/me/coaching-sessions", s.handleMyCoachingSessions)
	post("/coaching-sessions/{id}/review", s.handleReviewCoach)

	// G1/G2/G3: billing, entitlements and ads.
	get("/entitlements", s.handleEntitlements)
	post("/entitlements/{feature}/consume", s.handleConsume)
	get("/subscription", s.handleSubscription)
	post("/purchases/redeem", s.handleRedeemPurchase)
	get("/ads/policy", s.handleAdPolicy)
	// Store webhooks carry no bearer token; the handler verifies the
	// notification itself.
	mux.HandleFunc("POST /webhooks/store/{platform}",
		withObservability(s.log, "/webhooks/store/{platform}", s.handleStoreWebhook))

	// E2: the admin review queue.
	get("/admin/cheat-cases", s.handleCheatQueue)
	get("/admin/cheat-cases/{id}", s.handleGetCheatCase)
	post("/admin/cheat-cases/{id}/resolve", s.handleResolveCheatCase)

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
	case errors.Is(err, coaching.ErrPaymentsUnavailable):
		writeError(w, http.StatusServiceUnavailable, "payments_unconfigured", "coaching payments are not configured")
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
	case errors.Is(err, profile.ErrNotFound), errors.Is(err, archive.ErrNotFound),
		errors.Is(err, game.ErrGameNotFound), errors.Is(err, rooms.ErrNotFound),
		errors.Is(err, tournament.ErrNotFound), errors.Is(err, correspondence.ErrNotFound),
		errors.Is(err, anticheat.ErrNotFound), errors.Is(err, clubs.ErrNotFound),
		errors.Is(err, social.ErrNotFound), errors.Is(err, coaching.ErrNotFound),
		errors.Is(err, progames.ErrNotFound):
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
	var raw json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		if errors.Is(err, io.EOF) {
			return errEmptyBody
		}
		return err
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return errors.New("body must be a JSON object")
	}
	if err := dec.Decode(new(json.RawMessage)); !errors.Is(err, io.EOF) {
		if err != nil {
			return err
		}
		return errors.New("body must contain exactly one JSON object")
	}
	body := json.NewDecoder(bytes.NewReader(raw))
	body.DisallowUnknownFields()
	return body.Decode(dst)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if status != http.StatusNoContent {
		_ = json.NewEncoder(w).Encode(body)
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}
