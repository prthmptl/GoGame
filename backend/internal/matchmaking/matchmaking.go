// Package matchmaking implements D4: queues, rating expansion and pairing.
package matchmaking

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/prathpatel/gogame-backend/internal/clock"
	"github.com/prathpatel/gogame-backend/internal/goban"
)

// ErrTicketNotFound is returned for an unknown or expired ticket.
var ErrTicketNotFound = errors.New("matchmaking: ticket not found")

// ticketTTL bounds how long a ticket survives without being paired, so a
// client that disappears mid-queue does not linger forever.
const ticketTTL = 10 * time.Minute

// Status is a ticket's state.
type Status string

const (
	StatusQueued    Status = "queued"
	StatusMatched   Status = "matched"
	StatusCancelled Status = "cancelled"
)

// Request is what a client asks for.
type Request struct {
	Mode        string        `json:"mode"` // casual | ranked
	BoardSize   int           `json:"boardSize"`
	TimeControl clock.Control `json:"timeControl"`
	Ruleset     string        `json:"ruleset"`
	Region      string        `json:"region"`
}

// Ticket is a queue entry.
type Ticket struct {
	ID        uuid.UUID     `json:"id"`
	UserID    uuid.UUID     `json:"userId"`
	Rating    int           `json:"rating"`
	Request   Request       `json:"request"`
	Status    Status        `json:"status"`
	GameID    *uuid.UUID    `json:"gameId,omitempty"`
	CreatedAt time.Time     `json:"createdAt"`
	QueuedFor time.Duration `json:"-"`
}

// queueKey is D4's key layout: one sorted set per exact game shape, so every
// member of a queue is already compatible and pairing only has to consider
// rating.
func queueKey(r Request) string {
	return fmt.Sprintf("matchmaking:%s:%d:%s:%s",
		r.Mode, r.BoardSize, timeControlKey(r.TimeControl), r.Region)
}

func timeControlKey(c clock.Control) string {
	switch c.Kind {
	case clock.KindFischer:
		return fmt.Sprintf("fischer-%d-%d", c.MainSeconds, c.IncrementSeconds)
	case clock.KindByoYomi:
		return fmt.Sprintf("byoyomi-%d-%dx%d", c.MainSeconds, c.Periods, c.PeriodSeconds)
	case clock.KindCanadian:
		return fmt.Sprintf("canadian-%d-%d/%d", c.MainSeconds, c.StonesPerPeriod, c.PeriodSeconds)
	case clock.KindNone:
		return "none"
	default:
		return fmt.Sprintf("absolute-%d", c.MainSeconds)
	}
}

func ticketKey(id uuid.UUID) string { return "matchmaking:ticket:" + id.String() }

// Service manages queues and tickets.
type Service struct {
	rdb *redis.Client
	// Pair is called when two tickets match; it creates the game and returns
	// its id. Injected so this package does not depend on the game hub.
	Pair func(ctx context.Context, a, b Ticket) (uuid.UUID, error)
}

// NewService builds the matchmaking service.
func NewService(rdb *redis.Client) *Service { return &Service{rdb: rdb} }

// Enqueue creates a ticket and adds the player to the matching queue.
func (s *Service) Enqueue(ctx context.Context, userID uuid.UUID, rating int, req Request) (*Ticket, error) {
	if req.BoardSize != 9 && req.BoardSize != 13 && req.BoardSize != 19 {
		return nil, fmt.Errorf("unsupported board size %d", req.BoardSize)
	}
	if req.Mode != "casual" && req.Mode != "ranked" {
		return nil, fmt.Errorf("unsupported mode %q", req.Mode)
	}
	if req.Region == "" {
		req.Region = "global"
	}
	if req.Ruleset == "" {
		req.Ruleset = string(goban.Chinese)
	}

	t := &Ticket{
		ID: uuid.New(), UserID: userID, Rating: rating, Request: req,
		Status: StatusQueued, CreatedAt: time.Now().UTC(),
	}
	raw, err := json.Marshal(t)
	if err != nil {
		return nil, err
	}

	pipe := s.rdb.TxPipeline()
	pipe.Set(ctx, ticketKey(t.ID), raw, ticketTTL)
	// Score is the creation time, so ZRANGE yields longest-waiting first.
	pipe.ZAdd(ctx, queueKey(req), redis.Z{
		Score:  float64(t.CreatedAt.UnixMilli()),
		Member: t.ID.String(),
	})
	pipe.Expire(ctx, queueKey(req), ticketTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, fmt.Errorf("enqueue: %w", err)
	}
	return t, nil
}

// Get returns a ticket so the client can poll it, per D4.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Ticket, error) {
	raw, err := s.rdb.Get(ctx, ticketKey(id)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrTicketNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get ticket: %w", err)
	}
	var t Ticket
	if err := json.Unmarshal(raw, &t); err != nil {
		return nil, err
	}
	t.QueuedFor = time.Since(t.CreatedAt)
	return &t, nil
}

// Cancel removes a ticket from its queue.
func (s *Service) Cancel(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	t, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if t.UserID != userID {
		return ErrTicketNotFound
	}
	if t.Status == StatusMatched {
		return errors.New("matchmaking: already matched")
	}
	t.Status = StatusCancelled
	raw, _ := json.Marshal(t)

	pipe := s.rdb.TxPipeline()
	pipe.ZRem(ctx, queueKey(t.Request), t.ID.String())
	pipe.Set(ctx, ticketKey(t.ID), raw, time.Minute)
	_, err = pipe.Exec(ctx)
	return err
}

// ratingWindow implements D4's expansion schedule: ±100 for the first 15s,
// then ±200, ±350, and ±500 after a minute. A player who has waited long
// enough is eventually matched with anyone.
func ratingWindow(waited time.Duration) int {
	switch {
	case waited < 15*time.Second:
		return 100
	case waited < 30*time.Second:
		return 200
	case waited < 60*time.Second:
		return 350
	default:
		return 500
	}
}

// PairOnce scans every active queue and pairs whoever it can. The worker runs
// this every 2 seconds, as D4 specifies.
func (s *Service) PairOnce(ctx context.Context) (int, error) {
	keys, err := s.rdb.Keys(ctx, "matchmaking:*:*:*:*").Result()
	if err != nil {
		return 0, fmt.Errorf("scan queues: %w", err)
	}
	paired := 0
	for _, key := range keys {
		n, err := s.pairQueue(ctx, key)
		if err != nil {
			return paired, err
		}
		paired += n
	}
	return paired, nil
}

func (s *Service) pairQueue(ctx context.Context, key string) (int, error) {
	ids, err := s.rdb.ZRange(ctx, key, 0, 199).Result()
	if err != nil || len(ids) < 2 {
		return 0, nil
	}

	// Load every waiting ticket once; a ticket whose key expired is swept
	// out of the queue here rather than blocking pairing forever.
	tickets := make([]Ticket, 0, len(ids))
	for _, raw := range ids {
		id, err := uuid.Parse(raw)
		if err != nil {
			continue
		}
		t, err := s.Get(ctx, id)
		if err != nil || t.Status != StatusQueued {
			s.rdb.ZRem(ctx, key, raw)
			continue
		}
		tickets = append(tickets, *t)
	}

	paired := 0
	used := make([]bool, len(tickets))
	// Longest-waiting first: their window is widest, so they get first pick.
	for i := range tickets {
		if used[i] {
			continue
		}
		a := tickets[i]
		windowA := ratingWindow(time.Since(a.CreatedAt))

		best, bestGap := -1, 1<<30
		for j := i + 1; j < len(tickets); j++ {
			if used[j] {
				continue
			}
			b := tickets[j]
			if b.UserID == a.UserID {
				// Never pair someone against themselves on two devices.
				continue
			}
			gap := abs(a.Rating - b.Rating)
			// Both windows must accept: a fresh ticket should not be dragged
			// into a mismatch just because the other side has waited.
			if gap > windowA || gap > ratingWindow(time.Since(b.CreatedAt)) {
				continue
			}
			if gap < bestGap {
				best, bestGap = j, gap
			}
		}
		if best < 0 {
			continue
		}

		b := tickets[best]
		if err := s.commitPair(ctx, key, a, b); err != nil {
			// Losing one pairing should not abort the whole sweep.
			continue
		}
		used[i], used[best] = true, true
		paired++
	}
	return paired, nil
}

// commitPair removes both tickets from the queue and creates their game. The
// ZRem happens first: if game creation fails, the players are re-queued by
// their clients rather than being handed a game that does not exist.
func (s *Service) commitPair(ctx context.Context, key string, a, b Ticket) error {
	removed, err := s.rdb.ZRem(ctx, key, a.ID.String(), b.ID.String()).Result()
	if err != nil {
		return err
	}
	if removed != 2 {
		// Another worker claimed one of them first.
		return errors.New("matchmaking: tickets already claimed")
	}
	if s.Pair == nil {
		return errors.New("matchmaking: no pair function configured")
	}
	gameID, err := s.Pair(ctx, a, b)
	if err != nil {
		return err
	}
	for _, t := range []Ticket{a, b} {
		t.Status = StatusMatched
		t.GameID = &gameID
		raw, _ := json.Marshal(t)
		// Keep matched tickets briefly so a polling client can read the result.
		s.rdb.Set(ctx, ticketKey(t.ID), raw, 5*time.Minute)
	}
	return nil
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
