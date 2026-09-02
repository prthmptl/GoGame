package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/prathpatel/gogame-backend/internal/clubs"
	"github.com/prathpatel/gogame-backend/internal/coaching"
	"github.com/prathpatel/gogame-backend/internal/social"
)

// --- F1: clubs ---

func (s *Server) handleListClubs(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	out, err := s.clubs.List(r.Context(), r.URL.Query().Get("q"), limit)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"clubs": out})
}

type createClubRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

func (s *Server) handleCreateClub(w http.ResponseWriter, r *http.Request) {
	var req createClubRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	c, err := s.clubs.Create(r.Context(), callerFrom(r.Context()).ID, req.Name, req.Description)
	if err != nil {
		writeError(w, http.StatusBadRequest, "create_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

func (s *Server) handleGetClub(w http.ResponseWriter, r *http.Request) {
	c, err := s.clubs.Get(r.Context(), r.PathValue("slug"), callerFrom(r.Context()).ID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) handleClubMembers(w http.ResponseWriter, r *http.Request) {
	c, err := s.clubs.Get(r.Context(), r.PathValue("slug"), callerFrom(r.Context()).ID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	members, err := s.clubs.Members(r.Context(), c.ID, limit)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": members})
}

func (s *Server) handleJoinClub(w http.ResponseWriter, r *http.Request) {
	caller := callerFrom(r.Context())
	c, err := s.clubs.Get(r.Context(), r.PathValue("slug"), caller.ID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if err := s.clubs.Join(r.Context(), c.ID, caller.ID); err != nil {
		if errors.Is(err, clubs.ErrNotOpen) {
			writeError(w, http.StatusForbidden, "invite_only", "this club is invitation-only")
			return
		}
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

func (s *Server) handleLeaveClub(w http.ResponseWriter, r *http.Request) {
	caller := callerFrom(r.Context())
	c, err := s.clubs.Get(r.Context(), r.PathValue("slug"), caller.ID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if err := s.clubs.Leave(r.Context(), c.ID, caller.ID); err != nil {
		writeError(w, http.StatusConflict, "leave_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

func (s *Server) handleClubThreads(w http.ResponseWriter, r *http.Request) {
	c, err := s.clubs.Get(r.Context(), r.PathValue("slug"), callerFrom(r.Context()).ID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	threads, err := s.clubs.Threads(r.Context(), c.ID, limit)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"threads": threads})
}

type threadRequest struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

func (s *Server) handleCreateThread(w http.ResponseWriter, r *http.Request) {
	caller := callerFrom(r.Context())
	c, err := s.clubs.Get(r.Context(), r.PathValue("slug"), caller.ID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	var req threadRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	t, err := s.clubs.CreateThread(r.Context(), c.ID, caller.ID, req.Title, req.Body)
	if err != nil {
		if errors.Is(err, clubs.ErrForbidden) {
			writeError(w, http.StatusForbidden, "not_a_member", "join the club to post")
			return
		}
		writeError(w, http.StatusBadRequest, "create_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (s *Server) handleThreadPosts(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "thread id must be a uuid")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	posts, err := s.clubs.Posts(r.Context(), id, limit)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"posts": posts})
}

type replyRequest struct {
	Body string `json:"body"`
}

func (s *Server) handleReplyToThread(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "thread id must be a uuid")
		return
	}
	var req replyRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	if err := s.clubs.Reply(r.Context(), id, callerFrom(r.Context()).ID, req.Body); err != nil {
		if errors.Is(err, clubs.ErrForbidden) {
			writeError(w, http.StatusForbidden, "not_a_member", "join the club to post")
			return
		}
		writeError(w, http.StatusBadRequest, "reply_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

// --- F2: social ---

func (s *Server) handleFollow(w http.ResponseWriter, r *http.Request) {
	target, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "user id must be a uuid")
		return
	}
	if err := s.social.Follow(r.Context(), callerFrom(r.Context()).ID, target); err != nil {
		if errors.Is(err, social.ErrBlocked) {
			writeError(w, http.StatusForbidden, "blocked", "you cannot follow this user")
			return
		}
		writeError(w, http.StatusBadRequest, "follow_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

func (s *Server) handleUnfollow(w http.ResponseWriter, r *http.Request) {
	target, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "user id must be a uuid")
		return
	}
	if err := s.social.Unfollow(r.Context(), callerFrom(r.Context()).ID, target); err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

func (s *Server) handleBlock(w http.ResponseWriter, r *http.Request) {
	target, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "user id must be a uuid")
		return
	}
	if err := s.social.Block(r.Context(), callerFrom(r.Context()).ID, target); err != nil {
		writeError(w, http.StatusBadRequest, "block_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

func (s *Server) handleFriends(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	people, err := s.social.Friends(r.Context(), callerFrom(r.Context()).ID, limit)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"friends": people})
}

func (s *Server) handleFollowing(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	people, err := s.social.Following(r.Context(), callerFrom(r.Context()).ID, limit)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"following": people})
}

type dmRequest struct {
	Body string `json:"body"`
}

func (s *Server) handleSendDM(w http.ResponseWriter, r *http.Request) {
	target, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "user id must be a uuid")
		return
	}
	caller := callerFrom(r.Context())
	if caller.IsGuest {
		writeError(w, http.StatusForbidden, "guest_not_allowed", "sign in to send messages")
		return
	}
	var req dmRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	m, err := s.social.SendMessage(r.Context(), caller.ID, target, req.Body)
	if err != nil {
		if errors.Is(err, social.ErrBlocked) {
			writeError(w, http.StatusForbidden, "blocked", "you cannot message this user")
			return
		}
		writeError(w, http.StatusBadRequest, "send_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) handleConversation(w http.ResponseWriter, r *http.Request) {
	target, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "user id must be a uuid")
		return
	}
	caller := callerFrom(r.Context())
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	msgs, err := s.social.Conversation(r.Context(), caller.ID, target, limit)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	_ = s.social.MarkRead(r.Context(), caller.ID, target)
	writeJSON(w, http.StatusOK, map[string]any{"messages": msgs})
}

func (s *Server) handleFeed(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	events, err := s.social.Feed(r.Context(), callerFrom(r.Context()).ID, limit)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}

type reportRequest struct {
	SubjectID   uuid.UUID `json:"subjectId"`
	Kind        string    `json:"kind"`
	ReferenceID string    `json:"referenceId,omitempty"`
	Reason      string    `json:"reason"`
}

func (s *Server) handleReport(w http.ResponseWriter, r *http.Request) {
	var req reportRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	if err := s.social.Report(r.Context(), callerFrom(r.Context()).ID, req.SubjectID,
		req.Kind, req.ReferenceID, req.Reason); err != nil {
		writeError(w, http.StatusBadRequest, "report_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

// --- F3: opening explorer ---

type lookupRequest struct {
	PositionHash string `json:"positionHash"`
	Limit        int    `json:"limit,omitempty"`
}

func (s *Server) handleOpeningLookup(w http.ResponseWriter, r *http.Request) {
	// G2: the explorer is a paid feature.
	if err := s.requireFeature(r, "opening_explorer"); err != nil {
		writeError(w, http.StatusForbidden, "not_entitled", err.Error())
		return
	}
	var req lookupRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	out, err := s.openings.Lookup(r.Context(), req.PositionHash, req.Limit)
	if err != nil {
		// An unseen position is a normal answer, not an error.
		writeJSON(w, http.StatusOK, map[string]any{
			"positionHash": req.PositionHash, "games": 0, "moves": []any{},
		})
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// --- F4: pro games and relays ---

func (s *Server) handleSearchProGames(w http.ResponseWriter, r *http.Request) {
	if err := s.requireFeature(r, "pro_library"); err != nil {
		writeError(w, http.StatusForbidden, "not_entitled", err.Error())
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	out, err := s.progames.Search(r.Context(), r.URL.Query().Get("q"), limit)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"games": out})
}

func (s *Server) handleProGameSGF(w http.ResponseWriter, r *http.Request) {
	if err := s.requireFeature(r, "pro_library"); err != nil {
		writeError(w, http.StatusForbidden, "not_entitled", err.Error())
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "game id must be a uuid")
		return
	}
	body, err := s.progames.SGF(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "no SGF for that game")
		return
	}
	w.Header().Set("Content-Type", "application/x-go-sgf")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (s *Server) handleLiveRelays(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	out, err := s.progames.LiveRelays(r.Context(), limit)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"relays": out})
}

func (s *Server) handleGetRelay(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "relay id must be a uuid")
		return
	}
	out, err := s.progames.GetRelay(r.Context(), id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// --- F5: coaching ---

func (s *Server) handleListCoaches(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	maxRate, _ := strconv.Atoi(q.Get("maxRateCents"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	out, err := s.coaching.List(r.Context(), q.Get("language"), maxRate, limit)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"coaches": out})
}

type coachApplyRequest struct {
	Headline        string   `json:"headline"`
	Bio             string   `json:"bio,omitempty"`
	HourlyRateCents int      `json:"hourlyRateCents"`
	Currency        string   `json:"currency,omitempty"`
	Languages       []string `json:"languages,omitempty"`
	RankLabel       string   `json:"rankLabel,omitempty"`
}

func (s *Server) handleApplyAsCoach(w http.ResponseWriter, r *http.Request) {
	var req coachApplyRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	c, err := s.coaching.Apply(r.Context(), callerFrom(r.Context()).ID, req.Headline,
		req.Bio, req.HourlyRateCents, req.Currency, req.Languages, req.RankLabel)
	if err != nil {
		writeError(w, http.StatusBadRequest, "apply_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) handleCoachAvailability(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "coach id must be a uuid")
		return
	}
	slots, err := s.coaching.Availability(r.Context(), id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"slots": slots})
}

type availabilityRequest struct {
	Slots []coaching.Slot `json:"slots"`
}

func (s *Server) handleSetAvailability(w http.ResponseWriter, r *http.Request) {
	var req availabilityRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	if err := s.coaching.SetAvailability(r.Context(), callerFrom(r.Context()).ID, req.Slots); err != nil {
		writeError(w, http.StatusBadRequest, "availability_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

type bookRequest struct {
	StartsAt        time.Time `json:"startsAt"`
	DurationMinutes int       `json:"durationMinutes"`
	Notes           string    `json:"notes,omitempty"`
}

func (s *Server) handleBookCoach(w http.ResponseWriter, r *http.Request) {
	coachID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "coach id must be a uuid")
		return
	}
	var req bookRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	sess, err := s.coaching.Book(r.Context(), coachID, callerFrom(r.Context()).ID,
		req.StartsAt, req.DurationMinutes, req.Notes)
	if err != nil {
		switch {
		case errors.Is(err, coaching.ErrSlotTaken):
			writeError(w, http.StatusConflict, "slot_taken", "that slot is already booked")
		case errors.Is(err, coaching.ErrUnavailable):
			writeError(w, http.StatusConflict, "unavailable", "the coach is not available then")
		case errors.Is(err, coaching.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", "no such coach")
		default:
			writeError(w, http.StatusBadRequest, "book_failed", err.Error())
		}
		return
	}
	writeJSON(w, http.StatusCreated, sess)
}

func (s *Server) handleMyCoachingSessions(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	out, err := s.coaching.Sessions(r.Context(), callerFrom(r.Context()).ID, limit)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": out})
}

type reviewRequest struct {
	Rating  int    `json:"rating"`
	Comment string `json:"comment,omitempty"`
}

func (s *Server) handleReviewCoach(w http.ResponseWriter, r *http.Request) {
	sessionID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "session id must be a uuid")
		return
	}
	var req reviewRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	if err := s.coaching.Review(r.Context(), sessionID, callerFrom(r.Context()).ID,
		req.Rating, req.Comment); err != nil {
		writeError(w, http.StatusBadRequest, "review_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

var _ = json.Marshal
