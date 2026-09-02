package game

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

	mu       sync.RWMutex
	sessions map[uuid.UUID]*Session
}

// NewHub builds a hub. instanceID must be unique per process.
func NewHub(db *pgxpool.Pool, rdb *redis.Client, instanceID string, hooks Hooks) *Hub {
	return &Hub{
		db: db, rdb: rdb, instance: instanceID, hooks: hooks,
		sessions: map[uuid.UUID]*Session{},
	}
}

func ownerKey(gameID uuid.UUID) string { return "game:owner:" + gameID.String() }

// Create persists a new game and starts its session on this instance.
func (h *Hub) Create(ctx context.Context, cfg Config) (*Session, error) {
	tcJSON, err := json.Marshal(cfg.TimeControl)
	if err != nil {
		return nil, err
	}
	var id uuid.UUID
	err = h.db.QueryRow(ctx, `
		INSERT INTO games (black_user_id, white_user_id, black_bot_id, white_bot_id,
		                   board_size, ruleset, komi, handicap, time_control,
		                   time_class, mode, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'active')
		RETURNING id`,
		cfg.Black.UserID, cfg.White.UserID, cfg.Black.BotID, cfg.White.BotID,
		cfg.Rules.BoardSize, string(cfg.Rules.Ruleset), cfg.Rules.Komi,
		cfg.Rules.Handicap, tcJSON, cfg.TimeControl.TimeClass(), cfg.Mode,
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("create game: %w", err)
	}

	sess := New(ctx, h.db, id, cfg, h.hooks)
	h.mu.Lock()
	h.sessions[id] = sess
	h.mu.Unlock()
	h.claim(ctx, id)
	return sess, nil
}

// Get returns the live session for a game, loading it from Postgres if this
// instance does not already hold it.
func (h *Hub) Get(ctx context.Context, gameID uuid.UUID) (*Session, error) {
	h.mu.RLock()
	sess, ok := h.sessions[gameID]
	h.mu.RUnlock()
	if ok {
		return sess, nil
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	// Re-check: another goroutine may have loaded it while we waited.
	if sess, ok := h.sessions[gameID]; ok {
		return sess, nil
	}

	sess, err := h.load(ctx, gameID)
	if err != nil {
		return nil, err
	}
	h.sessions[gameID] = sess
	h.claim(ctx, gameID)
	return sess, nil
}

// OwnerOf reports which instance currently holds a game, so a load balancer
// or peer can route a connection to it. An empty string means unclaimed.
func (h *Hub) OwnerOf(ctx context.Context, gameID uuid.UUID) string {
	owner, err := h.rdb.Get(ctx, ownerKey(gameID)).Result()
	if err != nil {
		return ""
	}
	return owner
}

// claim records this instance as the owner and keeps refreshing the claim
// until the session is released.
func (h *Hub) claim(ctx context.Context, gameID uuid.UUID) {
	_ = h.rdb.Set(ctx, ownerKey(gameID), h.instance, ownershipTTL).Err()
	go func() {
		t := time.NewTicker(ownershipTTL / 3)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				h.mu.RLock()
				_, live := h.sessions[gameID]
				h.mu.RUnlock()
				if !live {
					return
				}
				// Refresh with a fresh context: the request that created the
				// game is long gone by now.
				bg, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				_ = h.rdb.Set(bg, ownerKey(gameID), h.instance, ownershipTTL).Err()
				cancel()
			}
		}
	}()
}

// load rebuilds a session from the database.
func (h *Hub) load(ctx context.Context, gameID uuid.UUID) (*Session, error) {
	var (
		cfg       Config
		boardSize int
		ruleset   string
		komi      float64
		handicap  int
		tcRaw     []byte
		status    string
	)
	err := h.db.QueryRow(ctx, `
		SELECT black_user_id, white_user_id, black_bot_id, white_bot_id,
		       board_size, ruleset, komi, handicap, time_control, mode, status
		FROM games WHERE id = $1`, gameID,
	).Scan(&cfg.Black.UserID, &cfg.White.UserID, &cfg.Black.BotID, &cfg.White.BotID,
		&boardSize, &ruleset, &komi, &handicap, &tcRaw, &cfg.Mode, &status)
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
			_ = json.Unmarshal(clockRaw, &lastClock)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if lastClock.Control.Kind == "" {
		// No moves yet, so seed the clock from the game's configured control.
		lastClock = clock.New(cfg.TimeControl, "black").Export(time.Now().UTC())
	}

	return Restore(ctx, h.db, gameID, cfg, moves, lastClock, h.hooks)
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
	// Only clear the key if we still own it, so a game already reclaimed by
	// another instance is not orphaned.
	_ = h.rdb.Eval(ctx,
		`if redis.call("GET", KEYS[1]) == ARGV[1] then return redis.call("DEL", KEYS[1]) end return 0`,
		[]string{ownerKey(gameID)}, h.instance).Err()
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
	var finished []uuid.UUID
	for id, s := range h.sessions {
		if s.Finished() {
			finished = append(finished, id)
		}
	}
	h.mu.Unlock()
	for _, id := range finished {
		h.Release(ctx, id)
	}
	return len(finished)
}

// LiveCount reports how many games this instance is running.
func (h *Hub) LiveCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.sessions)
}
