package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/prathpatel/gogame-backend/internal/chat"
	"github.com/prathpatel/gogame-backend/internal/clock"
	"github.com/prathpatel/gogame-backend/internal/game"
	"github.com/prathpatel/gogame-backend/internal/goban"
	"github.com/prathpatel/gogame-backend/internal/matchmaking"
	"github.com/prathpatel/gogame-backend/internal/rooms"
)

// --- D4: matchmaking ---

type queueRequest struct {
	Mode        string        `json:"mode"`
	BoardSize   int           `json:"boardSize"`
	TimeControl clock.Control `json:"timeControl"`
	Ruleset     string        `json:"ruleset,omitempty"`
	Region      string        `json:"region,omitempty"`
}

func (s *Server) handleEnqueue(w http.ResponseWriter, r *http.Request) {
	var req queueRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	caller := callerFrom(r.Context())
	if req.Mode == "ranked" && caller.IsGuest {
		writeError(w, http.StatusForbidden, "guest_not_allowed",
			"sign in with Google to play ranked")
		return
	}

	var rating int
	if err := s.store.DB.QueryRow(r.Context(),
		`SELECT rating FROM users WHERE id = $1`, caller.ID).Scan(&rating); err != nil {
		s.fail(w, r, err)
		return
	}
	ticket, err := s.matchmaking.Enqueue(r.Context(), caller.ID, rating, matchmaking.Request{
		Mode: req.Mode, BoardSize: req.BoardSize, TimeControl: req.TimeControl,
		Ruleset: req.Ruleset, Region: req.Region,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, "enqueue_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, ticket)
}

func (s *Server) handleGetTicket(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "ticket id must be a uuid")
		return
	}
	ticket, err := s.matchmaking.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "ticket not found or expired")
		return
	}
	if ticket.UserID != callerFrom(r.Context()).ID {
		// Do not confirm the existence of someone else's ticket.
		writeError(w, http.StatusNotFound, "not_found", "ticket not found or expired")
		return
	}
	writeJSON(w, http.StatusOK, ticket)
}

func (s *Server) handleCancelTicket(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "ticket id must be a uuid")
		return
	}
	if err := s.matchmaking.Cancel(r.Context(), id, callerFrom(r.Context()).ID); err != nil {
		if errors.Is(err, matchmaking.ErrTicketNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "ticket not found")
			return
		}
		writeError(w, http.StatusConflict, "cancel_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

// --- D5: rooms ---

type createRoomRequest struct {
	Settings json.RawMessage `json:"settings,omitempty"`
}

func (s *Server) handleCreateRoom(w http.ResponseWriter, r *http.Request) {
	var req createRoomRequest
	if err := decodeJSON(r, &req); err != nil && !errors.Is(err, errEmptyBody) {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	room, err := s.rooms.Create(r.Context(), callerFrom(r.Context()).ID, req.Settings)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, room)
}

func (s *Server) handleJoinRoom(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	room, err := s.rooms.Join(r.Context(), code, callerFrom(r.Context()).ID)
	if err != nil {
		switch {
		case errors.Is(err, rooms.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", "no room with that code")
		case errors.Is(err, rooms.ErrClosed):
			writeError(w, http.StatusConflict, "room_closed", "that room is no longer open")
		default:
			s.fail(w, r, err)
		}
		return
	}
	writeJSON(w, http.StatusOK, room)
}

func (s *Server) handleGetRoom(w http.ResponseWriter, r *http.Request) {
	room, err := s.rooms.Get(r.Context(), r.PathValue("code"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "no room with that code")
		return
	}
	writeJSON(w, http.StatusOK, room)
}

// handleStartRoom creates the game for a full room. Only the owner may start.
func (s *Server) handleStartRoom(w http.ResponseWriter, r *http.Request) {
	caller := callerFrom(r.Context())
	room, err := s.rooms.Get(r.Context(), r.PathValue("code"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "no room with that code")
		return
	}
	if room.OwnerID != caller.ID {
		writeError(w, http.StatusForbidden, "not_owner", "only the room owner can start the game")
		return
	}
	if room.GameID != nil {
		writeJSON(w, http.StatusOK, map[string]any{"gameId": room.GameID})
		return
	}
	players, err := s.rooms.Players(r.Context(), room.ID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if len(players) < 2 {
		writeError(w, http.StatusConflict, "not_enough_players", "the room needs a second player")
		return
	}

	var settings struct {
		BoardSize   int           `json:"boardSize"`
		Ruleset     string        `json:"ruleset"`
		Handicap    int           `json:"handicap"`
		TimeControl clock.Control `json:"timeControl"`
	}
	_ = json.Unmarshal(room.Settings, &settings)
	if settings.BoardSize == 0 {
		settings.BoardSize = 19
	}
	if settings.Ruleset == "" {
		settings.Ruleset = string(goban.Chinese)
	}
	if settings.TimeControl.Kind == "" {
		settings.TimeControl = clock.Absolute(20 * 60)
	}

	sess, err := s.hub.Create(r.Context(), game.Config{
		Black:       game.Player{UserID: &players[0], Color: "black"},
		White:       game.Player{UserID: &players[1], Color: "white"},
		Rules:       goban.NewConfig(settings.BoardSize, goban.Ruleset(settings.Ruleset), settings.Handicap),
		TimeControl: settings.TimeControl,
		Mode:        "friend",
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if err := s.rooms.Start(r.Context(), room.ID, sess.ID); err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"gameId": sess.ID})
}

// --- D5: chat history ---

func (s *Server) handleChatHistory(w http.ResponseWriter, r *http.Request) {
	gameID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "game id must be a uuid")
		return
	}
	// Reuse the archive's visibility rule so chat is not a way around it.
	if _, err := s.archive.Get(r.Context(), gameID, callerFrom(r.Context()).ID); err != nil {
		s.fail(w, r, err)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	msgs, err := s.chat.History(r.Context(), gameID, limit)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": msgs})
}

// PairPlayers is the matchmaking Pair hook: it creates the game two matched
// tickets have earned. Colour is assigned by ticket age so the player who
// waited longer gets black, which is a small compensation for the wait.
func PairPlayers(hub *game.Hub) func(context.Context, matchmaking.Ticket, matchmaking.Ticket) (uuid.UUID, error) {
	return func(ctx context.Context, a, b matchmaking.Ticket) (uuid.UUID, error) {
		black, white := a, b
		if b.CreatedAt.Before(a.CreatedAt) {
			black, white = b, a
		}
		rules := goban.NewConfig(a.Request.BoardSize, goban.Ruleset(a.Request.Ruleset), 0)
		sess, err := hub.Create(ctx, game.Config{
			Black:       game.Player{UserID: &black.UserID, Color: "black"},
			White:       game.Player{UserID: &white.UserID, Color: "white"},
			Rules:       rules,
			TimeControl: a.Request.TimeControl,
			Mode:        a.Request.Mode,
		})
		if err != nil {
			return uuid.Nil, err
		}
		return sess.ID, nil
	}
}

var _ = chat.ErrRateLimited
