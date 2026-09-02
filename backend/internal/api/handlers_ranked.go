package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/prathpatel/gogame-backend/internal/correspondence"
	"github.com/prathpatel/gogame-backend/internal/goban"
)

// --- E1: ratings ---

func (s *Server) handleMyRatings(w http.ResponseWriter, r *http.Request) {
	out, err := s.rating.All(r.Context(), callerFrom(r.Context()).ID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ratings": out})
}

func (s *Server) handleRatingHistory(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	boardSize, _ := strconv.Atoi(q.Get("boardSize"))
	if boardSize == 0 {
		boardSize = 19
	}
	timeClass := q.Get("timeClass")
	if timeClass == "" {
		timeClass = "rapid"
	}
	limit, _ := strconv.Atoi(q.Get("limit"))
	points, err := s.rating.History(r.Context(), callerFrom(r.Context()).ID,
		boardSize, timeClass, limit)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"points": points})
}

// --- E3: tournaments ---

func (s *Server) handleListTournaments(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	out, err := s.tournament.List(r.Context(), r.URL.Query().Get("status"), limit)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tournaments": out})
}

func (s *Server) handleGetTournament(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "tournament id must be a uuid")
		return
	}
	t, err := s.tournament.Get(r.Context(), id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) handleTournamentStandings(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "tournament id must be a uuid")
		return
	}
	standings, err := s.tournament.Standings(r.Context(), id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"standings": standings})
}

func (s *Server) handleJoinTournament(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "tournament id must be a uuid")
		return
	}
	caller := callerFrom(r.Context())
	if caller.IsGuest {
		writeError(w, http.StatusForbidden, "guest_not_allowed",
			"sign in with Google to enter tournaments")
		return
	}
	// E2: a player restricted from ranked play may not enter events either.
	if blocked, err := s.anticheat.IsRankedRestricted(r.Context(), caller.ID); err == nil && blocked {
		writeError(w, http.StatusForbidden, "restricted",
			"your account is currently restricted from rated play")
		return
	}

	var rating int
	if err := s.store.DB.QueryRow(r.Context(),
		`SELECT rating FROM users WHERE id = $1`, caller.ID).Scan(&rating); err != nil {
		s.fail(w, r, err)
		return
	}
	if err := s.tournament.Join(r.Context(), id, caller.ID, rating); err != nil {
		writeError(w, http.StatusConflict, "join_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

func (s *Server) handleWithdrawTournament(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "tournament id must be a uuid")
		return
	}
	if err := s.tournament.Withdraw(r.Context(), id, callerFrom(r.Context()).ID); err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

// --- E4: correspondence ---

func (s *Server) handleCorrespondenceState(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "game id must be a uuid")
		return
	}
	st, err := s.correspondence.Get(r.Context(), id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleGetVacation(w http.ResponseWriter, r *http.Request) {
	v, err := s.correspondence.GetVacation(r.Context(), callerFrom(r.Context()).ID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) handleStartVacation(w http.ResponseWriter, r *http.Request) {
	err := s.correspondence.StartVacation(r.Context(), callerFrom(r.Context()).ID)
	if errors.Is(err, correspondence.ErrNoVacationLeft) {
		writeError(w, http.StatusConflict, "no_vacation_left",
			"you have no banked vacation days")
		return
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	v, _ := s.correspondence.GetVacation(r.Context(), callerFrom(r.Context()).ID)
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) handleEndVacation(w http.ResponseWriter, r *http.Request) {
	if err := s.correspondence.EndVacation(r.Context(), callerFrom(r.Context()).ID); err != nil {
		s.fail(w, r, err)
		return
	}
	v, _ := s.correspondence.GetVacation(r.Context(), callerFrom(r.Context()).ID)
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) handleGetPlan(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "game id must be a uuid")
		return
	}
	plan, err := s.correspondence.GetPlan(r.Context(), id, callerFrom(r.Context()).ID)
	if errors.Is(err, correspondence.ErrNotFound) {
		writeJSON(w, http.StatusOK, map[string]any{"sequence": []any{}})
		return
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

type planRequest struct {
	FromMove int `json:"fromMove"`
	Sequence []struct {
		If   goban.Point `json:"if"`
		Then goban.Point `json:"then"`
	} `json:"sequence"`
}

func (s *Server) handleSetPlan(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "game id must be a uuid")
		return
	}
	var req planRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	seq := make([]correspondence.Branch, 0, len(req.Sequence))
	for _, b := range req.Sequence {
		seq = append(seq, correspondence.Branch{If: b.If, Then: b.Then})
	}
	if err := s.correspondence.SetPlan(r.Context(), id, callerFrom(r.Context()).ID,
		req.FromMove, seq); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_plan", err.Error())
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

// --- E2: admin review queue ---

// requireAdmin gates the moderation endpoints.
//
// Admin identity is not modelled yet: this checks an allowlist of user ids in
// ADMIN_USER_IDS. A real deployment needs proper roles before the review
// queue is exposed beyond a trusted operator.
func (s *Server) handleCheatQueue(w http.ResponseWriter, r *http.Request) {
	if !s.isAdmin(r) {
		writeError(w, http.StatusForbidden, "not_admin", "admin access required")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	cases, err := s.anticheat.Queue(r.Context(), r.URL.Query().Get("status"), limit)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"cases": cases})
}

func (s *Server) handleGetCheatCase(w http.ResponseWriter, r *http.Request) {
	if !s.isAdmin(r) {
		writeError(w, http.StatusForbidden, "not_admin", "admin access required")
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "case id must be a uuid")
		return
	}
	c, err := s.anticheat.GetCase(r.Context(), id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

type resolveRequest struct {
	Action    string     `json:"action"`
	Notes     string     `json:"notes,omitempty"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
}

func (s *Server) handleResolveCheatCase(w http.ResponseWriter, r *http.Request) {
	if !s.isAdmin(r) {
		writeError(w, http.StatusForbidden, "not_admin", "admin access required")
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "case id must be a uuid")
		return
	}
	var req resolveRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	if err := s.anticheat.Resolve(r.Context(), id, callerFrom(r.Context()).ID,
		req.Action, req.Notes, req.ExpiresAt); err != nil {
		writeError(w, http.StatusBadRequest, "resolve_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

var _ = json.Marshal
