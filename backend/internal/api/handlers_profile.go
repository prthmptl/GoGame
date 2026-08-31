package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/prathpatel/gogame-backend/internal/archive"
	"github.com/prathpatel/gogame-backend/internal/notify"
	"github.com/prathpatel/gogame-backend/internal/profile"
)

// --- C3: profile, sync, achievements, leaderboards ---

func (s *Server) handleGetMe(w http.ResponseWriter, r *http.Request) {
	p, err := s.profile.Get(r.Context(), callerFrom(r.Context()).ID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handlePatchMe(w http.ResponseWriter, r *http.Request) {
	var req profile.PatchRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	p, err := s.profile.Patch(r.Context(), callerFrom(r.Context()).ID, req)
	if err != nil {
		if errors.Is(err, profile.ErrNotFound) {
			s.fail(w, r, err)
			return
		}
		writeError(w, http.StatusBadRequest, "invalid_profile", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	var req profile.SyncRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	resp, err := s.profile.Sync(r.Context(), callerFrom(r.Context()).ID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "sync_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleAchievements(w http.ResponseWriter, r *http.Request) {
	out, err := s.profile.Achievements(r.Context(), callerFrom(r.Context()).ID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"achievements": out})
}

func (s *Server) handleLeaderboard(w http.ResponseWriter, r *http.Request) {
	after, _ := strconv.ParseInt(r.URL.Query().Get("afterRank"), 10, 64)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	rows, err := s.profile.Leaderboard(r.Context(), after, limit)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": rows})
}

// --- C4: game archive ---

func (s *Server) handleListGames(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	query := archive.SearchQuery{UserID: callerFrom(r.Context()).ID}
	if v := q.Get("opponent"); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			query.Opponent = &id
		}
	}
	for param, dst := range map[string]**string{
		"result": &query.Result, "timeClass": &query.TimeClass, "mode": &query.Mode,
	} {
		if v := q.Get(param); v != "" {
			val := v
			*dst = &val
		}
	}
	if v, err := strconv.Atoi(q.Get("boardSize")); err == nil && v > 0 {
		query.BoardSize = &v
	}
	for param, dst := range map[string]**time.Time{
		"since": &query.Since, "until": &query.Until, "before": &query.BeforeStart,
	} {
		if v := q.Get(param); v != "" {
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				tv := t
				*dst = &tv
			}
		}
	}
	if v, err := strconv.Atoi(q.Get("limit")); err == nil {
		query.Limit = v
	}

	games, err := s.archive.Search(r.Context(), query)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"games": games})
}

func (s *Server) handleGetGame(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "game id must be a uuid")
		return
	}
	g, err := s.archive.Get(r.Context(), id, callerFrom(r.Context()).ID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, g)
}

func (s *Server) handleGameMoves(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "game id must be a uuid")
		return
	}
	if _, err := s.archive.Get(r.Context(), id, callerFrom(r.Context()).ID); err != nil {
		s.fail(w, r, err)
		return
	}
	moves, err := s.archive.Moves(r.Context(), id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"moves": moves})
}

func (s *Server) handleGameSGF(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "game id must be a uuid")
		return
	}
	body, err := s.archive.SGF(r.Context(), id, callerFrom(r.Context()).ID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/x-go-sgf")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+id.String()+".sgf\"")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

// --- C5: notifications ---

type deviceRequest struct {
	Token      string `json:"token"`
	Platform   string `json:"platform"`
	Locale     string `json:"locale,omitempty"`
	AppVersion string `json:"appVersion,omitempty"`
}

func (s *Server) handleRegisterDevice(w http.ResponseWriter, r *http.Request) {
	var req deviceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	if req.Token == "" {
		writeError(w, http.StatusBadRequest, "missing_token", "token is required")
		return
	}
	if err := s.notify.RegisterDevice(r.Context(), callerFrom(r.Context()).ID,
		req.Token, req.Platform, req.Locale, req.AppVersion); err != nil {
		writeError(w, http.StatusBadRequest, "register_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

func (s *Server) handleGetNotificationPrefs(w http.ResponseWriter, r *http.Request) {
	prefs, err := s.notify.Prefs(r.Context(), callerFrom(r.Context()).ID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"prefs": prefs})
}

type prefRequest struct {
	EventType string `json:"eventType"`
	Enabled   bool   `json:"enabled"`
}

func (s *Server) handleSetNotificationPref(w http.ResponseWriter, r *http.Request) {
	var req prefRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	if err := s.notify.SetPref(r.Context(), callerFrom(r.Context()).ID,
		notify.EventType(req.EventType), req.Enabled); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_pref", err.Error())
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}
