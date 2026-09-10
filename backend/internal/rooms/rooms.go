// Package rooms implements D5's friend rooms.
package rooms

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Errors returned by the service.
var (
	ErrNotFound = errors.New("rooms: not found")
	ErrClosed   = errors.New("rooms: room is not open")
	ErrFull     = errors.New("rooms: room already has two players")
)

// Room is a private lobby.
type Room struct {
	ID        uuid.UUID       `json:"id"`
	OwnerID   uuid.UUID       `json:"ownerId"`
	Code      string          `json:"code"`
	Settings  json.RawMessage `json:"settings"`
	GameID    *uuid.UUID      `json:"gameId,omitempty"`
	Status    string          `json:"status"`
	Members   []Member        `json:"members,omitempty"`
	CreatedAt time.Time       `json:"createdAt"`
	ExpiresAt time.Time       `json:"expiresAt"`
}

// Member is a room participant.
type Member struct {
	UserID      uuid.UUID `json:"userId"`
	DisplayName string    `json:"displayName"`
	Role        string    `json:"role"`
}

// Service manages rooms.
type Service struct{ db *pgxpool.Pool }

// NewService builds the rooms service.
func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

// codeAlphabet omits characters that are easy to misread aloud (0/O, 1/I).
const codeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func newCode() (string, error) {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, 6)
	for i, b := range buf {
		out[i] = codeAlphabet[int(b)%len(codeAlphabet)]
	}
	return string(out), nil
}

// Create opens a room owned by the caller.
func (s *Service) Create(ctx context.Context, ownerID uuid.UUID, settings json.RawMessage) (*Room, error) {
	if len(settings) == 0 {
		settings = json.RawMessage(`{}`)
	}
	// Retry on the (very unlikely) code collision rather than failing.
	for attempt := 0; attempt < 5; attempt++ {
		code, err := newCode()
		if err != nil {
			return nil, err
		}
		var r Room
		err = s.db.QueryRow(ctx, `
			INSERT INTO rooms (owner_id, code, settings)
			VALUES ($1, $2, $3)
			RETURNING id, owner_id, code, settings, game_id, status, created_at, expires_at`,
			ownerID, code, settings,
		).Scan(&r.ID, &r.OwnerID, &r.Code, &r.Settings, &r.GameID, &r.Status,
			&r.CreatedAt, &r.ExpiresAt)
		if err != nil {
			if isUniqueViolation(err) {
				continue
			}
			return nil, fmt.Errorf("create room: %w", err)
		}
		if _, err := s.db.Exec(ctx, `
			INSERT INTO room_members (room_id, user_id, role) VALUES ($1, $2, 'player')`,
			r.ID, ownerID); err != nil {
			return nil, fmt.Errorf("add owner: %w", err)
		}
		return &r, nil
	}
	return nil, errors.New("rooms: could not allocate a unique code")
}

func isUniqueViolation(err error) bool {
	return err != nil && (contains(err.Error(), "23505") || contains(err.Error(), "duplicate key"))
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}

// Join adds a member by room code. The second player joins as a player; every
// later arrival becomes a spectator, which is how D5's spectating starts.
func (s *Service) Join(ctx context.Context, code string, userID uuid.UUID) (*Room, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var r Room
	// FOR UPDATE serialises two people racing to take the second seat.
	err = tx.QueryRow(ctx, `
		SELECT id, owner_id, code, settings, game_id, status, created_at, expires_at
		FROM rooms WHERE code = $1 FOR UPDATE`, code,
	).Scan(&r.ID, &r.OwnerID, &r.Code, &r.Settings, &r.GameID, &r.Status,
		&r.CreatedAt, &r.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find room: %w", err)
	}
	if r.Status == "closed" || time.Now().After(r.ExpiresAt) {
		return nil, ErrClosed
	}

	var players int
	if err := tx.QueryRow(ctx,
		`SELECT count(*) FROM room_members WHERE room_id = $1 AND role = 'player'`,
		r.ID).Scan(&players); err != nil {
		return nil, err
	}
	role := "spectator"
	if players < 2 {
		role = "player"
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO room_members (room_id, user_id, role) VALUES ($1, $2, $3)
		ON CONFLICT (room_id, user_id) DO NOTHING`, r.ID, userID, role); err != nil {
		return nil, fmt.Errorf("join room: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.Get(ctx, r.Code)
}

// Get loads a room and its members by code.
func (s *Service) Get(ctx context.Context, code string) (*Room, error) {
	var r Room
	err := s.db.QueryRow(ctx, `
		SELECT id, owner_id, code, settings, game_id, status, created_at, expires_at
		FROM rooms WHERE code = $1`, code,
	).Scan(&r.ID, &r.OwnerID, &r.Code, &r.Settings, &r.GameID, &r.Status,
		&r.CreatedAt, &r.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get room: %w", err)
	}

	rows, err := s.db.Query(ctx, `
		SELECT rm.user_id, u.display_name, rm.role
		FROM room_members rm JOIN users u ON u.id = rm.user_id
		WHERE rm.room_id = $1 ORDER BY rm.joined_at`, r.ID)
	if err != nil {
		return nil, fmt.Errorf("load members: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.UserID, &m.DisplayName, &m.Role); err != nil {
			return nil, err
		}
		r.Members = append(r.Members, m)
	}
	return &r, rows.Err()
}

// Players returns the two seated players, in join order.
func (s *Service) Players(ctx context.Context, roomID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := s.db.Query(ctx, `
		SELECT user_id FROM room_members
		WHERE room_id = $1 AND role = 'player' ORDER BY joined_at LIMIT 2`, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// Start attaches a created game to the room and closes it to new players.
func (s *Service) Start(ctx context.Context, roomID, gameID uuid.UUID) error {
	tag, err := s.db.Exec(ctx,
		`UPDATE rooms SET game_id = $2, status = 'started' WHERE id = $1
          AND status='open' AND game_id IS NULL AND expires_at > now()`, roomID, gameID)
	if err == nil && tag.RowsAffected() != 1 {
		return ErrClosed
	}
	return err
}

// SweepExpired closes rooms nobody used. The worker calls it.
func (s *Service) SweepExpired(ctx context.Context) (int64, error) {
	tag, err := s.db.Exec(ctx,
		`UPDATE rooms SET status = 'closed' WHERE status = 'open' AND expires_at < now()`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
