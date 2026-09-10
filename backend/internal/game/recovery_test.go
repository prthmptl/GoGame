package game_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prathpatel/gogame-backend/internal/clock"
	"github.com/prathpatel/gogame-backend/internal/game"
	"github.com/prathpatel/gogame-backend/internal/goban"
	"github.com/redis/go-redis/v9"
)

func hubFor(t *testing.T) *game.Hub {
	t.Helper()
	opts, err := redis.ParseURL(os.Getenv("TEST_REDIS_URL"))
	if err != nil {
		t.Fatal(err)
	}
	rdb := redis.NewClient(opts)
	h := game.NewHub(pool, rdb, uuid.NewString(), game.Hooks{})
	t.Cleanup(func() { h.Close(); rdb.Close() })
	return h
}

func TestConcurrentRoomStartCreatesOnlyOneGame(t *testing.T) {
	ctx := context.Background()
	h := hubFor(t)
	black, white := newUser(t, "Room black"), newUser(t, "Room white")
	var roomID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO rooms(owner_id,code) VALUES ($1,$2) RETURNING id`, black, uuid.NewString()[:6]).Scan(&roomID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM rooms WHERE id=$1`, roomID) })
	cfg := game.Config{Black: game.Player{UserID: &black}, White: game.Player{UserID: &white}, Rules: goban.NewConfig(9, goban.Chinese, 0), TimeControl: clock.NoControl(), Mode: "friend"}
	ids := make(chan uuid.UUID, 8)
	errs := make(chan error, 8)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, err := h.CreateInRoom(ctx, cfg, roomID)
			var started *game.RoomAlreadyStarted
			if errors.As(err, &started) {
				ids <- started.GameID
			} else if err != nil {
				errs <- err
			} else {
				ids <- s.ID
			}
		}()
	}
	wg.Wait()
	close(ids)
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	var id uuid.UUID
	for got := range ids {
		if id != uuid.Nil && id != got {
			t.Fatal("duplicate games")
		}
		id = got
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM games WHERE black_user_id=$1 AND white_user_id=$2`, black, white).Scan(&count); err != nil || count != 1 {
		t.Fatalf("games=%d error=%v", count, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE rooms SET game_id=NULL,status='open',expires_at=now()-interval '1 hour' WHERE id=$1`, roomID); err != nil {
		t.Fatal(err)
	}
	if _, err := h.CreateInRoom(ctx, cfg, roomID); !errors.Is(err, game.ErrRoomClosed) {
		t.Fatalf("expired room start: %v", err)
	}
}

func subscribe(t *testing.T, s *game.Session, user uuid.UUID) *game.Subscriber {
	t.Helper()
	sub := &game.Subscriber{UserID: user, Out: make(chan game.Event, 128)}
	if err := s.Subscribe(sub); err != nil {
		t.Fatal(err)
	}
	return sub
}

func snapshot(t *testing.T, s *game.Session, user uuid.UUID) map[string]json.RawMessage {
	t.Helper()
	sub := subscribe(t, s, user)
	defer s.Unsubscribe(sub)
	select {
	case event := <-sub.Out:
		if event.Type != "GAME_SNAPSHOT" {
			t.Fatalf("unexpected %s", event.Type)
		}
		var value map[string]json.RawMessage
		if err := json.Unmarshal(event.Payload, &value); err != nil {
			t.Fatal(err)
		}
		return value
	case <-time.After(time.Second):
		t.Fatal("snapshot timed out")
	}
	return nil
}

func TestHubSurvivesRequestCancellationAndRestoresScoring(t *testing.T) {
	h := hubFor(t)
	black, white := newUser(t, "Recovery black"), newUser(t, "Recovery white")
	ctx, cancel := context.WithCancel(context.Background())
	cfg := game.Config{Black: game.Player{UserID: &black}, White: game.Player{UserID: &white},
		Rules: goban.NewConfig(9, goban.Chinese, 0), TimeControl: clock.Absolute(60), Mode: "friend"}
	s, err := h.Create(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	if err := s.PlayMove(black, 1, goban.Point{Row: 0, Col: 0}); err != nil {
		t.Fatal(err)
	}
	if _, err := hubFor(t).Get(context.Background(), s.ID); !errors.Is(err, game.ErrOwnedElsewhere) {
		t.Fatalf("second owner: %v", err)
	}
	_ = s.Pass(white, 1)
	_ = s.Pass(black, 2)
	_ = s.MarkDead(white, 2, goban.Point{Row: 0, Col: 0})
	_ = s.ConfirmScore(white, 3)
	h.Release(context.Background(), s.ID)
	// Hours in scoring must never run down the paused clock on recovery.
	_, err = pool.Exec(context.Background(), `UPDATE games SET session_state = jsonb_set(session_state,
        '{clock,updatedAt}', to_jsonb(to_char((now() - interval '1 hour') AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'))) WHERE id=$1`, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	s, err = h.Get(context.Background(), s.ID)
	if err != nil {
		t.Fatal(err)
	}
	snap := snapshot(t, s, black)
	var status string
	var dead []goban.Point
	var confirmed map[string]bool
	var clocks clock.State
	_ = json.Unmarshal(snap["status"], &status)
	_ = json.Unmarshal(snap["deadStones"], &dead)
	_ = json.Unmarshal(snap["confirmed"], &confirmed)
	_ = json.Unmarshal(snap["clock"], &clocks)
	if status != "scoring" || len(dead) != 1 || !confirmed["white"] || clocks.Black.Flagged || clocks.White.Flagged {
		t.Fatalf("lost scoring state: %s", snap)
	}
	// Hostile coordinates must be rejected without crashing the actor.
	_ = s.MarkDead(black, 3, goban.Point{Row: -1, Col: 999})
	_ = s.DisputeScore(black, 3)
	h.Release(context.Background(), s.ID)
	s, err = h.Get(context.Background(), s.ID)
	if err != nil {
		t.Fatal(err)
	}
	_ = s.PlayMove(white, 4, goban.Point{Row: 1, Col: 1})
	h.Release(context.Background(), s.ID)
	s, err = h.Get(context.Background(), s.ID)
	if err != nil {
		t.Fatalf("replay after dispute: %v", err)
	}
	sub := subscribe(t, s, white)
	_ = s.Pass(black, 4)
	_ = s.Pass(white, 4) // Same seq as white's persisted placement.
	events := collect(sub, 10*time.Millisecond)
	rej := find(events, "MOVE_REJECTED")
	if rej == nil {
		t.Fatal("replayed command accepted after restart")
	}
	var payload struct {
		Reason string `json:"reason"`
	}
	_ = json.Unmarshal(rej.Payload, &payload)
	if payload.Reason != "duplicate_or_stale_seq" {
		t.Fatalf("reason = %s", payload.Reason)
	}
}

func TestPersistenceConflictNeverAcknowledgesMove(t *testing.T) {
	s, id, black, _ := newGame(t, clock.NoControl())
	sub := subscribe(t, s, black)
	_, err := pool.Exec(context.Background(), `UPDATE games SET session_version = session_version + 1 WHERE id=$1`, id)
	if err != nil {
		t.Fatal(err)
	}
	_ = s.PlayMove(black, 1, goban.Point{Row: 2, Col: 2})
	events := collect(sub, 50*time.Millisecond)
	if has(events, "MOVE_ACCEPTED") || has(events, "MOVE_PLAYED") {
		t.Fatalf("uncommitted move broadcast: %v", typesOf(events))
	}
	if !s.Closed() {
		t.Fatal("stale actor must stop")
	}
	var count int
	_ = pool.QueryRow(context.Background(), `SELECT count(*) FROM game_moves WHERE game_id=$1`, id).Scan(&count)
	if count != 0 {
		t.Fatal("stale move persisted")
	}
}

func TestInvalidMovesDoNotDoubleChargeAndTicksPreserveThinkTime(t *testing.T) {
	s, id, black, _ := newGame(t, clock.Absolute(60))
	start := time.Now()
	for i := 0; i < 8; i++ {
		time.Sleep(15 * time.Millisecond)
		_ = s.PlayMove(black, 1, goban.Point{Row: -1, Col: 0})
		_ = s.Tick()
	}
	_ = s.PlayMove(black, 1, goban.Point{Row: 2, Col: 2})
	elapsed := time.Since(start).Milliseconds()
	var think int
	var raw []byte
	if err := pool.QueryRow(context.Background(), `SELECT think_millis, clock_after FROM game_moves WHERE game_id=$1`, id).Scan(&think, &raw); err != nil {
		t.Fatal(err)
	}
	var clocks clock.State
	_ = json.Unmarshal(raw, &clocks)
	charged := int64(60000 - clocks.Black.MainMillis)
	if charged > elapsed+50 || charged < elapsed-50 {
		t.Fatalf("charged %dms for %dms", charged, elapsed)
	}
	if int64(think) < elapsed-50 {
		t.Fatalf("ticks erased think time: %dms for %dms", think, elapsed)
	}
}

func TestEitherPlayerCanResignAndCloseIsConcurrentSafe(t *testing.T) {
	s, id, _, white := newGame(t, clock.NoControl())
	if err := s.Resign(white, 1); err != nil {
		t.Fatal(err)
	}
	var result string
	if err := pool.QueryRow(context.Background(), `SELECT result FROM games WHERE id=$1`, id).Scan(&result); err != nil {
		t.Fatal(err)
	}
	if result != "B+R" {
		t.Fatalf("result %s", result)
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); s.Close() }()
	}
	wg.Wait()
}

func TestSlowSubscriberIsDisconnected(t *testing.T) {
	s, _, black, _ := newGame(t, clock.NoControl())
	sub := &game.Subscriber{UserID: black, Out: make(chan game.Event, 1)}
	_ = s.Subscribe(sub)
	_ = s.PlayMove(black, 1, goban.Point{Row: 0, Col: 0})
	<-sub.Out // Buffered snapshot.
	if _, ok := <-sub.Out; ok {
		t.Fatal("slow connection must close for resync")
	}
}
