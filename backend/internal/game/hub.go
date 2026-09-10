package game

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/prathpatel/gogame-backend/internal/clock"
	"github.com/prathpatel/gogame-backend/internal/goban"
)

// ErrGameNotFound is returned for an unknown or finished game.
var ErrGameNotFound = errors.New("game: not found")
var ErrOwnedElsewhere = errors.New("game: owned by another instance")
var ErrRoomClosed = errors.New("game: room is closed or expired")

type RoomAlreadyStarted struct{ GameID uuid.UUID }

func (e *RoomAlreadyStarted) Error() string { return "game: room already started" }

// ownershipTTL is how long this instance's claim on a game lasts without a
// refresh. Short enough that a crashed instance releases its games quickly,
// long enough to survive a GC pause.
const ownershipTTL = 30 * time.Second

// Hub owns the live sessions on this instance and the Redis routing that
// tells other instances which instance owns which game (D2's "sticky
// routing").
type Hub struct {
	db       *pgxpool.Pool
	rdb      *redis.Client
	instance string
	hooks    Hooks
	ctx      context.Context
	cancel   context.CancelFunc

	mu       sync.RWMutex
	sessions map[uuid.UUID]*Session
}

// NewHub builds a hub. instanceID must be unique per process.
func NewHub(db *pgxpool.Pool, rdb *redis.Client, instanceID string, hooks Hooks) *Hub {
	ctx, cancel := context.WithCancel(context.Background())
	return &Hub{
		db: db, rdb: rdb, instance: instanceID, hooks: hooks, ctx: ctx, cancel: cancel,
		sessions: map[uuid.UUID]*Session{},
	}
}

func ownerKey(gameID uuid.UUID) string { return "game:owner:" + gameID.String() }

// Create persists a new game and starts its session on this instance.
func (h *Hub) Create(ctx context.Context, cfg Config) (*Session, error) {
	return h.create(ctx, cfg, nil)
}

// CreateInRoom commits the game and its room link together. The row lock also
// prevents concurrent starts (including requests to different API instances).
func (h *Hub) CreateInRoom(ctx context.Context, cfg Config, roomID uuid.UUID) (*Session, error) {
	return h.create(ctx, cfg, &roomID)
}

func (h *Hub) create(ctx context.Context, cfg Config, roomID *uuid.UUID) (*Session, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	tcJSON, err := json.Marshal(cfg.TimeControl)
	if err != nil {
		return nil, err
	}
	first := "black"
	if cfg.Rules.Handicap > 0 {
		first = "white"
	}
	now := time.Now().UTC()
	runtimeJSON, err := json.Marshal(runtimeState{Rules: cfg.Rules, Status: goban.StatusActive,
		Clock: clock.New(cfg.TimeControl, first).Export(now), LastMoveAt: now})
	if err != nil {
		return nil, err
	}
	id := uuid.New()
	token := h.instance + "|" + uuid.NewString()
	claimed, err := h.rdb.SetNX(ctx, ownerKey(id), token, ownershipTTL).Result()
	if err != nil {
		return nil, err
	}
	if !claimed {
		return nil, ErrOwnedElsewhere
	}
	attached := false
	defer func() {
		if !attached {
			releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			h.releaseClaim(releaseCtx, &Session{ID: id, owner: token})
		}
	}()
	tx, err := h.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if roomID != nil {
		var existing *uuid.UUID
		var status string
		var expires time.Time
		if err := tx.QueryRow(ctx, `SELECT game_id, status, expires_at FROM rooms WHERE id=$1 FOR UPDATE`, roomID).Scan(&existing, &status, &expires); err != nil {
			return nil, err
		}
		if existing != nil {
			return nil, &RoomAlreadyStarted{GameID: *existing}
		}
		if status != "open" || !time.Now().Before(expires) {
			return nil, ErrRoomClosed
		}
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO games (black_user_id, white_user_id, black_bot_id, white_bot_id,
		                   board_size, ruleset, komi, handicap, time_control,
		                   time_class, mode, status, session_state, id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'active',$12,$13)
		RETURNING id`,
		cfg.Black.UserID, cfg.White.UserID, cfg.Black.BotID, cfg.White.BotID,
		cfg.Rules.BoardSize, string(cfg.Rules.Ruleset), cfg.Rules.Komi,
		cfg.Rules.Handicap, tcJSON, cfg.TimeControl.TimeClass(), cfg.Mode, runtimeJSON, id,
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("create game: %w", err)
	}
	if roomID != nil {
		if _, err := tx.Exec(ctx, `UPDATE rooms SET game_id=$2, status='started' WHERE id=$1`, roomID, id); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	sess := newSession(h.db, id, cfg, h.hooks)
	sess.owner = token
	sess.lastMoveAt, sess.lastChargeAt = now, now
	h.mu.Lock()
	if h.ctx.Err() != nil {
		h.mu.Unlock()
		return nil, errSessionClosed
	}
	h.sessions[id] = sess
	h.mu.Unlock()
	attached = true
	go sess.run(h.ctx)
	go h.renewClaim(sess)
	return sess, nil
}

// Get returns the single live owner, rebuilding from durable state after a
// disconnect or process restart. Request cancellation never stops the actor.
func (h *Hub) Get(ctx context.Context, gameID uuid.UUID) (*Session, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.ctx.Err() != nil {
		return nil, errSessionClosed
	}
	if sess, ok := h.sessions[gameID]; ok {
		if !sess.Closed() {
			return sess, nil
		}
		<-sess.stopped
		h.releaseClaim(ctx, sess)
		delete(h.sessions, gameID)
	}
	token := h.instance + "|" + uuid.NewString()
	claimed, err := h.rdb.SetNX(ctx, ownerKey(gameID), token, ownershipTTL).Result()
	if err != nil {
		return nil, fmt.Errorf("claim game: %w", err)
	}
	if !claimed {
		return nil, ErrOwnedElsewhere
	}
	sess, err := h.load(ctx, gameID)
	if err != nil {
		h.releaseClaim(ctx, &Session{ID: gameID, owner: token})
		return nil, err
	}
	sess.owner = token
	h.sessions[gameID] = sess
	go sess.run(h.ctx)
	go h.renewClaim(sess)
	return sess, nil
}

// OwnerOf reports which instance currently holds a game, so a load balancer
// or peer can route a connection to it. An empty string means unclaimed.
func (h *Hub) OwnerOf(ctx context.Context, gameID uuid.UUID) string {
	owner, err := h.rdb.Get(ctx, ownerKey(gameID)).Result()
	if err != nil {
		return ""
	}
	return strings.SplitN(owner, "|", 2)[0]
}

func (h *Hub) renewClaim(sess *Session) {
	t := time.NewTicker(ownershipTTL / 3)
	defer t.Stop()
	for {
		select {
		case <-sess.done:
			return
		case <-h.ctx.Done():
			return
		case <-t.C:
			ctx, cancel := context.WithTimeout(h.ctx, 5*time.Second)
			renewed, err := h.rdb.Eval(ctx, `if redis.call("GET", KEYS[1]) == ARGV[1]
                then return redis.call("PEXPIRE", KEYS[1], ARGV[2]) end return 0`,
				[]string{ownerKey(sess.ID)}, sess.owner, ownershipTTL.Milliseconds()).Int()
			cancel()
			if err != nil || renewed != 1 {
				sess.Close()
				return
			}
		}
	}
}

func (h *Hub) releaseClaim(ctx context.Context, sess *Session) {
	_ = h.rdb.Eval(ctx, `if redis.call("GET", KEYS[1]) == ARGV[1]
        then return redis.call("DEL", KEYS[1]) end return 0`,
		[]string{ownerKey(sess.ID)}, sess.owner).Err()
}

// load rebuilds a session from the database.
func (h *Hub) load(ctx context.Context, gameID uuid.UUID) (*Session, error) {
	var (
		cfg        Config
		boardSize  int
		ruleset    string
		komi       float64
		handicap   int
		tcRaw      []byte
		status     string
		startedAt  time.Time
		runtimeRaw []byte
		revision   int64
	)
	err := h.db.QueryRow(ctx, `
		UPDATE games SET session_version = session_version + 1
 WHERE id = $1 AND status = 'active'
 RETURNING black_user_id, white_user_id, black_bot_id, white_bot_id,
		       board_size, ruleset, komi, handicap, time_control, mode, status, started_at, session_state, session_version`, gameID,
	).Scan(&cfg.Black.UserID, &cfg.White.UserID, &cfg.Black.BotID, &cfg.White.BotID,
		&boardSize, &ruleset, &komi, &handicap, &tcRaw, &cfg.Mode, &status, &startedAt, &runtimeRaw, &revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrGameNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load game: %w", err)
	}
	if status != "active" {
		return nil, ErrGameNotFound
	}

	cfg.Black.Color, cfg.White.Color = "black", "white"
	cfg.Rules = goban.NewConfig(boardSize, goban.Ruleset(ruleset), handicap)
	cfg.Rules.Komi = komi
	if err := json.Unmarshal(tcRaw, &cfg.TimeControl); err != nil {
		return nil, fmt.Errorf("decode time control: %w", err)
	}

	var runtime runtimeState
	if len(runtimeRaw) > 0 {
		if err := json.Unmarshal(runtimeRaw, &runtime); err != nil {
			return nil, fmt.Errorf("decode session: %w", err)
		}
		cfg.Rules = runtime.Rules
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	rows, err := h.db.Query(ctx, `
		SELECT move_number, player, kind, row, col, clock_after
		FROM game_moves WHERE game_id = $1 ORDER BY move_number`, gameID)
	if err != nil {
		return nil, fmt.Errorf("load moves: %w", err)
	}
	defer rows.Close()

	var moves []StoredMove
	var lastClock clock.State
	for rows.Next() {
		var m StoredMove
		var clockRaw []byte
		if err := rows.Scan(&m.MoveNumber, &m.Player, &m.Kind, &m.Row, &m.Col, &clockRaw); err != nil {
			return nil, fmt.Errorf("scan move: %w", err)
		}
		moves = append(moves, m)
		if len(clockRaw) > 0 {
			if err := json.Unmarshal(clockRaw, &lastClock); err != nil {
				return nil, fmt.Errorf("decode clock: %w", err)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if lastClock.Control.Kind == "" {
		// No moves yet, so seed the clock from the game's configured control.
		first := "black"
		if cfg.Rules.Handicap > 0 {
			first = "white"
		}
		lastClock = clock.New(cfg.TimeControl, first).Export(startedAt)
	}

	sess, err := restoreSession(h.db, gameID, cfg, moves, lastClock, h.hooks)
	if err != nil {
		return nil, err
	}
	sess.revision = revision
	if len(runtimeRaw) > 0 {
		sess.state.Status = runtime.Status
		if runtime.Status == goban.StatusActive && sess.state.ConsecutivePasses >= 2 {
			sess.state.ConsecutivePasses = 0
		}
		for _, p := range runtime.Dead {
			if !sess.state.Board.InBounds(p) {
				return nil, errors.New("invalid stored dead point")
			}
			sess.dead[p] = true
		}
		if runtime.Confirmed != nil {
			sess.confirmed = runtime.Confirmed
		}
		if runtime.LastSeq != nil {
			sess.lastSeq = runtime.LastSeq
		}
		at := time.Now().UTC()
		if runtime.Status != goban.StatusActive {
			at = runtime.Clock.UpdatedAt
		}
		sess.clock = clock.Restore(runtime.Clock, at)
		sess.lastChargeAt, sess.lastMoveAt = at, runtime.LastMoveAt
	}
	return sess, nil
}

// Release drops a session from this instance and gives up its claim.
func (h *Hub) Release(ctx context.Context, gameID uuid.UUID) {
	h.mu.Lock()
	sess, ok := h.sessions[gameID]
	delete(h.sessions, gameID)
	h.mu.Unlock()
	if ok {
		sess.Close()
	}
	if ok {
		<-sess.stopped
		h.releaseClaim(ctx, sess)
	}
}

// TickAll charges elapsed time on every live game. The worker drives this,
// which is D3's timeout watcher.
func (h *Hub) TickAll() {
	h.mu.RLock()
	sessions := make([]*Session, 0, len(h.sessions))
	for _, s := range h.sessions {
		sessions = append(sessions, s)
	}
	h.mu.RUnlock()
	for _, s := range sessions {
		_ = s.Tick()
	}
}

// Reap removes finished sessions so memory does not grow without bound.
func (h *Hub) Reap(ctx context.Context) int {
	h.mu.Lock()
	var finished []*Session
	for id, s := range h.sessions {
		if s.Finished() || s.Closed() {
			finished = append(finished, s)
			delete(h.sessions, id)
		}
	}
	h.mu.Unlock()
	for _, s := range finished {
		s.Close()
		<-s.stopped
		h.releaseClaim(ctx, s)
	}
	return len(finished)
}

// LiveCount reports how many games this instance is running.
func (h *Hub) LiveCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.sessions)
}

// RecoverActive restarts abandoned games even when no player reconnects, so
// their authoritative clocks still expire. Another owner's lease is respected.
func (h *Hub) RecoverActive(ctx context.Context) error {
	rows, err := h.db.Query(ctx, `SELECT id FROM games WHERE status = 'active' AND mode <> 'daily'`)
	if err != nil {
		return err
	}
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := h.Get(ctx, id); err != nil && !errors.Is(err, ErrOwnedElsewhere) && !errors.Is(err, ErrGameNotFound) {
			return err
		}
	}
	return nil
}

func (h *Hub) Close() {
	h.cancel()
	h.mu.RLock()
	var ids []uuid.UUID
	for id := range h.sessions {
		ids = append(ids, id)
	}
	h.mu.RUnlock()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, id := range ids {
		h.Release(ctx, id)
	}
}
