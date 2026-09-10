package matchmaking

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/prathpatel/gogame-backend/internal/clock"
	"github.com/redis/go-redis/v9"
	"os"
	"testing"
)

func serviceFor(t *testing.T) *Service {
	t.Helper()
	raw := os.Getenv("TEST_REDIS_URL")
	if raw == "" {
		t.Skip("TEST_REDIS_URL not set")
	}
	opts, err := redis.ParseURL(raw)
	if err != nil {
		t.Fatal(err)
	}
	rdb := redis.NewClient(opts)
	t.Cleanup(func() { rdb.Close() })
	return NewService(rdb)
}

func enqueue(t *testing.T, s *Service, user uuid.UUID, req Request) *Ticket {
	t.Helper()
	ticket, err := s.Enqueue(context.Background(), user, 1000, req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.rdb.Del(context.Background(), ticketKey(ticket.ID), userKey(user), queueKey(ticket.Request)) })
	return ticket
}

func TestMatchmakingKeepsRulesSeparateAndOneTicketPerUser(t *testing.T) {
	s := serviceFor(t)
	req := Request{Mode: "casual", BoardSize: 9, Ruleset: "chinese", Region: uuid.NewString()[:12], TimeControl: clock.NoControl()}
	user := uuid.New()
	a := enqueue(t, s, user, req)
	if again := enqueue(t, s, user, req); again.ID != a.ID {
		t.Fatal("created duplicate ticket")
	}
	other := req
	other.Ruleset = "japanese"
	b := enqueue(t, s, uuid.New(), other)
	if queueKey(a.Request) == queueKey(b.Request) {
		t.Fatal("different rules share queue")
	}
	// No callback must leave both queued.
	if _, err := s.PairOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Get(context.Background(), a.ID)
	if got.Status != StatusQueued {
		t.Fatal("unconfigured sweep consumed ticket")
	}
}

func TestFailedPairRestoresBothTicketsAndCancelledPairDoesNotStealPartner(t *testing.T) {
	s := serviceFor(t)
	req := Request{Mode: "casual", BoardSize: 9, Ruleset: "chinese", Region: uuid.NewString()[:12], TimeControl: clock.Absolute(60)}
	a, b := enqueue(t, s, uuid.New(), req), enqueue(t, s, uuid.New(), req)
	s.Pair = func(context.Context, Ticket, Ticket) (uuid.UUID, error) {
		return uuid.Nil, errors.New("database unavailable")
	}
	if err := s.commitPair(context.Background(), queueKey(req), *a, *b); err == nil {
		t.Fatal("expected failure")
	}
	for _, ticket := range []*Ticket{a, b} {
		got, _ := s.Get(context.Background(), ticket.ID)
		if got.Status != StatusQueued {
			t.Fatal("failed pairing stranded player")
		}
	}
	if err := s.Cancel(context.Background(), a.ID, a.UserID); err != nil {
		t.Fatal(err)
	}
	if err := s.commitPair(context.Background(), queueKey(req), *a, *b); err == nil {
		t.Fatal("cancelled pair succeeded")
	}
	if _, err := s.rdb.ZScore(context.Background(), queueKey(req), b.ID.String()).Result(); err != nil {
		t.Fatal("partner removed by failed claim")
	}
}

func TestInvalidTimeControlRejected(t *testing.T) {
	s := serviceFor(t)
	_, err := s.Enqueue(context.Background(), uuid.New(), 1000, Request{Mode: "casual", BoardSize: 9, TimeControl: clock.ByoYomi(0, 100, 0)})
	if err == nil {
		t.Fatal("zero-length overtime accepted")
	}
}
