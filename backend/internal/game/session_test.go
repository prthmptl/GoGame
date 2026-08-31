package game_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/prathpatel/gogame-backend/internal/clock"
	"github.com/prathpatel/gogame-backend/internal/game"
	"github.com/prathpatel/gogame-backend/internal/goban"
	"github.com/prathpatel/gogame-backend/internal/store"
)

var pool *pgxpool.Pool

func TestMain(m *testing.M) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		os.Stderr.WriteString("TEST_DATABASE_URL not set; skipping game tests\n")
		os.Exit(0)
	}
	st, err := store.Open(context.Background(), dsn, os.Getenv("TEST_REDIS_URL"))
	if err != nil {
		panic(err)
	}
	if err := st.Migrate(context.Background()); err != nil {
		panic(err)
	}
	pool = st.DB
	code := m.Run()
	st.Close()
	os.Exit(code)
}

func newUser(t *testing.T, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO users (display_name, is_guest) VALUES ($1, FALSE) RETURNING id`,
		name).Scan(&id); err != nil {
		t.Fatalf("create user: %v", err)
	}
	return id
}

// newGame creates a persisted game row plus a live session.
func newGame(t *testing.T, tc clock.Control) (*game.Session, uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	black, white := newUser(t, "Black"), newUser(t, "White")
	rules := goban.NewConfig(9, goban.Chinese, 0)
	tcJSON, _ := json.Marshal(tc)

	var gameID uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO games (black_user_id, white_user_id, board_size, ruleset, komi,
		                   handicap, time_control, time_class, mode, status)
		VALUES ($1,$2,9,'chinese',7.5,0,$3,$4,'casual','active')
		RETURNING id`, black, white, tcJSON, tc.TimeClass()).Scan(&gameID); err != nil {
		t.Fatalf("create game: %v", err)
	}

	cfg := game.Config{
		Black: game.Player{UserID: &black, Color: "black"},
		White: game.Player{UserID: &white, Color: "white"},
		Rules: rules, TimeControl: tc, Mode: "casual",
	}
	sess := game.New(ctx, pool, gameID, cfg, game.Hooks{})
	t.Cleanup(sess.Close)
	return sess, gameID, black, white
}

// collect drains events for a subscriber until quiet, returning them by type.
func collect(sub *game.Subscriber, d time.Duration) []game.Event {
	var out []game.Event
	deadline := time.After(d)
	for {
		select {
		case ev, ok := <-sub.Out:
			if !ok {
				return out
			}
			out = append(out, ev)
		case <-deadline:
			return out
		}
	}
}

func typesOf(events []game.Event) []string {
	out := make([]string, len(events))
	for i, e := range events {
		out[i] = e.Type
	}
	return out
}

func has(events []game.Event, t string) bool {
	for _, e := range events {
		if e.Type == t {
			return true
		}
	}
	return false
}

func find(events []game.Event, t string) *game.Event {
	for i := range events {
		if events[i].Type == t {
			return &events[i]
		}
	}
	return nil
}

func TestSubscribeReceivesSnapshot(t *testing.T) {
	sess, _, black, _ := newGame(t, clock.Absolute(600))
	sub := &game.Subscriber{UserID: black, Out: make(chan game.Event, 32)}
	if err := sess.Subscribe(sub); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	events := collect(sub, 300*time.Millisecond)
	snap := find(events, "GAME_SNAPSHOT")
	if snap == nil {
		t.Fatalf("no snapshot; got %v", typesOf(events))
	}
	var payload map[string]any
	if err := json.Unmarshal(snap.Payload, &payload); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	if payload["toMove"] != "black" {
		t.Errorf("toMove = %v, want black", payload["toMove"])
	}
	if payload["stateHash"] == "" || payload["stateHash"] == nil {
		t.Error("snapshot is missing the state hash reconnect depends on")
	}
	rows, ok := payload["board"].([]any)
	if !ok || len(rows) != 9 {
		t.Errorf("board = %v, want 9 rows", payload["board"])
	}
}

func TestMoveIsBroadcastAndPersisted(t *testing.T) {
	sess, gameID, black, white := newGame(t, clock.Absolute(600))
	bs := &game.Subscriber{UserID: black, Out: make(chan game.Event, 32)}
	ws := &game.Subscriber{UserID: white, Out: make(chan game.Event, 32)}
	if err := sess.Subscribe(bs); err != nil {
		t.Fatal(err)
	}
	if err := sess.Subscribe(ws); err != nil {
		t.Fatal(err)
	}
	collect(bs, 100*time.Millisecond)
	collect(ws, 100*time.Millisecond)

	if err := sess.PlayMove(black, 1, goban.Point{Row: 4, Col: 4}); err != nil {
		t.Fatalf("play move: %v", err)
	}
	bEvents := collect(bs, 400*time.Millisecond)
	wEvents := collect(ws, 200*time.Millisecond)

	if !has(bEvents, "MOVE_ACCEPTED") {
		t.Errorf("mover did not get MOVE_ACCEPTED; got %v", typesOf(bEvents))
	}
	if !has(bEvents, "MOVE_PLAYED") || !has(wEvents, "MOVE_PLAYED") {
		t.Errorf("MOVE_PLAYED not broadcast to both: black=%v white=%v",
			typesOf(bEvents), typesOf(wEvents))
	}
	if has(wEvents, "MOVE_ACCEPTED") {
		t.Error("the opponent should not receive MOVE_ACCEPTED")
	}
	if !has(bEvents, "CLOCK_UPDATE") {
		t.Error("no CLOCK_UPDATE after an accepted move")
	}

	// D2 requires every accepted move to be persisted with its state hash.
	var count int
	var hash string
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*), COALESCE(max(state_hash),'') FROM game_moves WHERE game_id = $1`,
		gameID).Scan(&count, &hash); err != nil {
		t.Fatalf("query moves: %v", err)
	}
	if count != 1 {
		t.Errorf("persisted moves = %d, want 1", count)
	}
	if hash == "" {
		t.Error("persisted move has no state hash")
	}
}

func TestOutOfTurnMoveIsRejectedToSenderOnly(t *testing.T) {
	sess, _, black, white := newGame(t, clock.Absolute(600))
	bs := &game.Subscriber{UserID: black, Out: make(chan game.Event, 32)}
	ws := &game.Subscriber{UserID: white, Out: make(chan game.Event, 32)}
	_ = sess.Subscribe(bs)
	_ = sess.Subscribe(ws)
	collect(bs, 100*time.Millisecond)
	collect(ws, 100*time.Millisecond)

	// White moves first, but it is black's turn.
	if err := sess.PlayMove(white, 7, goban.Point{Row: 0, Col: 0}); err != nil {
		t.Fatal(err)
	}
	wEvents := collect(ws, 300*time.Millisecond)
	bEvents := collect(bs, 100*time.Millisecond)

	rej := find(wEvents, "MOVE_REJECTED")
	if rej == nil {
		t.Fatalf("no rejection; got %v", typesOf(wEvents))
	}
	var payload map[string]any
	_ = json.Unmarshal(rej.Payload, &payload)
	if payload["reason"] != string(goban.RejNotYourTurn) {
		t.Errorf("reason = %v, want %s", payload["reason"], goban.RejNotYourTurn)
	}
	if payload["seq"] != float64(7) {
		t.Errorf("seq = %v, want the sender's 7 echoed back", payload["seq"])
	}
	if has(bEvents, "MOVE_REJECTED") {
		t.Error("the opponent must not see the other player's rejection")
	}
}

func TestNonParticipantCannotMove(t *testing.T) {
	sess, _, black, _ := newGame(t, clock.Absolute(600))
	stranger := newUser(t, "Stranger")
	sub := &game.Subscriber{UserID: stranger, Out: make(chan game.Event, 32)}
	_ = sess.Subscribe(sub)
	collect(sub, 100*time.Millisecond)

	if err := sess.PlayMove(stranger, 1, goban.Point{Row: 0, Col: 0}); err != nil {
		t.Fatal(err)
	}
	events := collect(sub, 300*time.Millisecond)
	rej := find(events, "MOVE_REJECTED")
	if rej == nil {
		t.Fatalf("a spectator's move was not rejected; got %v", typesOf(events))
	}
	var payload map[string]any
	_ = json.Unmarshal(rej.Payload, &payload)
	if payload["reason"] != "not_a_player" {
		t.Errorf("reason = %v, want not_a_player", payload["reason"])
	}
	_ = black
}

func TestIllegalMoveIsRejectedByTheEngine(t *testing.T) {
	sess, _, black, white := newGame(t, clock.Absolute(600))
	sub := &game.Subscriber{UserID: black, Out: make(chan game.Event, 64)}
	_ = sess.Subscribe(sub)
	collect(sub, 100*time.Millisecond)

	_ = sess.PlayMove(black, 1, goban.Point{Row: 4, Col: 4})
	collect(sub, 200*time.Millisecond)
	_ = sess.PlayMove(white, 2, goban.Point{Row: 0, Col: 0})
	collect(sub, 200*time.Millisecond)
	// Black plays on top of its own stone.
	_ = sess.PlayMove(black, 3, goban.Point{Row: 4, Col: 4})

	events := collect(sub, 300*time.Millisecond)
	rej := find(events, "MOVE_REJECTED")
	if rej == nil {
		t.Fatalf("occupied point accepted; got %v", typesOf(events))
	}
	var payload map[string]any
	_ = json.Unmarshal(rej.Payload, &payload)
	if payload["reason"] != string(goban.RejOccupied) {
		t.Errorf("reason = %v, want %s", payload["reason"], goban.RejOccupied)
	}
}

func TestResignEndsGameAndPersistsResult(t *testing.T) {
	sess, gameID, black, white := newGame(t, clock.Absolute(600))
	sub := &game.Subscriber{UserID: white, Out: make(chan game.Event, 32)}
	_ = sess.Subscribe(sub)
	collect(sub, 100*time.Millisecond)

	if err := sess.Resign(black, 1); err != nil {
		t.Fatal(err)
	}
	events := collect(sub, 400*time.Millisecond)
	ended := find(events, "GAME_ENDED")
	if ended == nil {
		t.Fatalf("no GAME_ENDED; got %v", typesOf(events))
	}
	var r game.Result
	_ = json.Unmarshal(ended.Payload, &r)
	if r.Winner != "white" || r.Result != "W+R" || r.EndReason != "resign" {
		t.Errorf("result = %+v, want white W+R resign", r)
	}

	var status, dbResult, winner string
	if err := pool.QueryRow(context.Background(),
		`SELECT status, result, winner FROM games WHERE id = $1`, gameID,
	).Scan(&status, &dbResult, &winner); err != nil {
		t.Fatalf("query game: %v", err)
	}
	if status != "completed" || dbResult != "W+R" || winner != "white" {
		t.Errorf("db row = %s/%s/%s, want completed/W+R/white", status, dbResult, winner)
	}
	_ = white
}

func TestTwoPassesStartScoringAndConfirmEndsGame(t *testing.T) {
	sess, _, black, white := newGame(t, clock.Absolute(600))
	sub := &game.Subscriber{UserID: black, Out: make(chan game.Event, 64)}
	_ = sess.Subscribe(sub)
	collect(sub, 100*time.Millisecond)

	_ = sess.Pass(black, 1)
	collect(sub, 200*time.Millisecond)
	_ = sess.Pass(white, 2)
	events := collect(sub, 300*time.Millisecond)
	if !has(events, "SCORING_STARTED") {
		t.Fatalf("no SCORING_STARTED after two passes; got %v", typesOf(events))
	}

	// Both players must confirm before the game ends.
	_ = sess.ConfirmScore(black, 3)
	events = collect(sub, 300*time.Millisecond)
	if has(events, "GAME_ENDED") {
		t.Fatal("game ended on a single confirmation")
	}
	_ = sess.ConfirmScore(white, 4)
	events = collect(sub, 400*time.Millisecond)
	ended := find(events, "GAME_ENDED")
	if ended == nil {
		t.Fatalf("no GAME_ENDED after both confirmed; got %v", typesOf(events))
	}
	var r game.Result
	_ = json.Unmarshal(ended.Payload, &r)
	if r.EndReason != "score" {
		t.Errorf("endReason = %s, want score", r.EndReason)
	}
	// An empty 9x9 under Chinese rules is white by komi.
	if r.Winner != "white" {
		t.Errorf("winner = %s, want white (komi on an empty board)", r.Winner)
	}
}

func TestDisputeScoreReturnsToPlay(t *testing.T) {
	sess, _, black, white := newGame(t, clock.Absolute(600))
	sub := &game.Subscriber{UserID: black, Out: make(chan game.Event, 64)}
	_ = sess.Subscribe(sub)
	collect(sub, 100*time.Millisecond)

	_ = sess.Pass(black, 1)
	collect(sub, 150*time.Millisecond)
	_ = sess.Pass(white, 2)
	collect(sub, 250*time.Millisecond)

	_ = sess.DisputeScore(black, 3)
	events := collect(sub, 300*time.Millisecond)
	snap := find(events, "GAME_SNAPSHOT")
	if snap == nil {
		t.Fatalf("dispute did not resend a snapshot; got %v", typesOf(events))
	}
	var payload map[string]any
	_ = json.Unmarshal(snap.Payload, &payload)
	if payload["status"] != string(goban.StatusActive) {
		t.Errorf("status = %v, want the game back in play", payload["status"])
	}

	// And play must actually be possible again.
	_ = sess.PlayMove(black, 4, goban.Point{Row: 2, Col: 2})
	events = collect(sub, 300*time.Millisecond)
	if !has(events, "MOVE_PLAYED") {
		t.Errorf("could not move after a dispute; got %v", typesOf(events))
	}
}

func TestClockTimeoutEndsTheGame(t *testing.T) {
	// A one-second budget so the flag falls inside the test.
	sess, gameID, black, white := newGame(t, clock.Absolute(1))
	sub := &game.Subscriber{UserID: white, Out: make(chan game.Event, 32)}
	_ = sess.Subscribe(sub)
	collect(sub, 100*time.Millisecond)

	time.Sleep(1200 * time.Millisecond)
	if err := sess.Tick(); err != nil {
		t.Fatal(err)
	}
	events := collect(sub, 400*time.Millisecond)
	ended := find(events, "GAME_ENDED")
	if ended == nil {
		t.Fatalf("clock did not end the game; got %v", typesOf(events))
	}
	var r game.Result
	_ = json.Unmarshal(ended.Payload, &r)
	if r.EndReason != "timeout" {
		t.Errorf("endReason = %s, want timeout", r.EndReason)
	}
	// Black was on move, so white wins on time.
	if r.Winner != "white" || r.Result != "W+T" {
		t.Errorf("result = %s / %s, want white W+T", r.Winner, r.Result)
	}
	if !sess.Finished() {
		t.Error("session should report finished")
	}

	var status string
	_ = pool.QueryRow(context.Background(),
		`SELECT status FROM games WHERE id = $1`, gameID).Scan(&status)
	if status != "completed" {
		t.Errorf("db status = %s, want completed", status)
	}
	_ = black
	_ = white
}

func TestCaptureIsAppliedAndBroadcast(t *testing.T) {
	sess, _, black, white := newGame(t, clock.Absolute(600))
	sub := &game.Subscriber{UserID: black, Out: make(chan game.Event, 128)}
	_ = sess.Subscribe(sub)
	collect(sub, 100*time.Millisecond)

	// Black surrounds a white stone at (0,0) in the corner.
	seq := []struct {
		who uuid.UUID
		p   goban.Point
	}{
		{black, goban.Point{Row: 0, Col: 1}},
		{white, goban.Point{Row: 0, Col: 0}},
		{black, goban.Point{Row: 5, Col: 5}},
		{white, goban.Point{Row: 8, Col: 8}},
		{black, goban.Point{Row: 1, Col: 0}}, // closes the last liberty
	}
	for i, mv := range seq {
		if err := sess.PlayMove(mv.who, int64(i+1), mv.p); err != nil {
			t.Fatal(err)
		}
		collect(sub, 150*time.Millisecond)
	}

	// The final snapshot should show the captured stone gone.
	resub := &game.Subscriber{UserID: black, Out: make(chan game.Event, 32)}
	_ = sess.Subscribe(resub)
	events := collect(resub, 300*time.Millisecond)
	snap := find(events, "GAME_SNAPSHOT")
	if snap == nil {
		t.Fatal("no snapshot")
	}
	var payload struct {
		Board    []string       `json:"board"`
		Captures map[string]int `json:"captures"`
	}
	if err := json.Unmarshal(snap.Payload, &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.Board[0][0] != '.' {
		t.Errorf("board row 0 = %q, want the corner stone captured", payload.Board[0])
	}
	if payload.Captures["black"] != 1 {
		t.Errorf("black captures = %d, want 1", payload.Captures["black"])
	}
}

func TestReconnectSnapshotIsCappedAtFiftyMoves(t *testing.T) {
	sess, _, black, white := newGame(t, clock.Absolute(3600))
	sub := &game.Subscriber{UserID: black, Out: make(chan game.Event, 512)}
	_ = sess.Subscribe(sub)
	collect(sub, 100*time.Millisecond)

	// 60 alternating moves down separate rows so nothing is captured.
	players := []uuid.UUID{black, white}
	for i := 0; i < 60; i++ {
		p := goban.Point{Row: i / 9, Col: i % 9}
		if err := sess.PlayMove(players[i%2], int64(i+1), p); err != nil {
			t.Fatal(err)
		}
		collect(sub, 20*time.Millisecond)
	}

	fresh := &game.Subscriber{UserID: black, Out: make(chan game.Event, 32)}
	_ = sess.Subscribe(fresh)
	events := collect(fresh, 400*time.Millisecond)
	snap := find(events, "GAME_SNAPSHOT")
	if snap == nil {
		t.Fatal("no snapshot on reconnect")
	}
	var payload struct {
		RecentMoves []json.RawMessage `json:"recentMoves"`
		MoveNumber  int               `json:"moveNumber"`
	}
	if err := json.Unmarshal(snap.Payload, &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(payload.RecentMoves) != 50 {
		t.Errorf("recentMoves = %d, want the last 50 per D2", len(payload.RecentMoves))
	}
	if payload.MoveNumber != 60 {
		t.Errorf("moveNumber = %d, want 60", payload.MoveNumber)
	}
}
