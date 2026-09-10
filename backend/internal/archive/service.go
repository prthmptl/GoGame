// Package archive implements C4: the games archive and SGF retrieval.
package archive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/prathpatel/gogame-backend/internal/blob"
	"github.com/prathpatel/gogame-backend/internal/goban"
)

// ErrNotFound is returned for a missing or inaccessible game.
var ErrNotFound = errors.New("archive: game not found")

// Game is an archive row.
type Game struct {
	ID          uuid.UUID       `json:"id"`
	BlackUserID *uuid.UUID      `json:"blackUserId,omitempty"`
	WhiteUserID *uuid.UUID      `json:"whiteUserId,omitempty"`
	BlackBotID  *string         `json:"blackBotId,omitempty"`
	WhiteBotID  *string         `json:"whiteBotId,omitempty"`
	BlackName   string          `json:"blackName"`
	WhiteName   string          `json:"whiteName"`
	BoardSize   int             `json:"boardSize"`
	Ruleset     string          `json:"ruleset"`
	Komi        float64         `json:"komi"`
	Handicap    int             `json:"handicap"`
	TimeControl json.RawMessage `json:"timeControl"`
	TimeClass   string          `json:"timeClass"`
	Mode        string          `json:"mode"`
	Status      string          `json:"status"`
	Result      *string         `json:"result,omitempty"`
	Winner      *string         `json:"winner,omitempty"`
	EndReason   *string         `json:"endReason,omitempty"`
	MoveCount   int             `json:"moveCount"`
	StartedAt   time.Time       `json:"startedAt"`
	EndedAt     *time.Time      `json:"endedAt,omitempty"`
}

// MoveRow is one persisted move.
type MoveRow struct {
	MoveNumber  int             `json:"moveNumber"`
	Player      string          `json:"player"`
	Kind        string          `json:"kind"`
	Row         *int            `json:"row,omitempty"`
	Col         *int            `json:"col,omitempty"`
	Captured    json.RawMessage `json:"captured"`
	StateHash   string          `json:"stateHash"`
	ThinkMillis int             `json:"thinkMillis"`
	PlayedAt    time.Time       `json:"playedAt"`
}

// SearchQuery filters the archive. C4 asks for opponent, result, time control
// and date; all four are here.
type SearchQuery struct {
	UserID      uuid.UUID
	Opponent    *uuid.UUID
	Result      *string // 'win' | 'loss' | 'draw' from UserID's perspective
	TimeClass   *string
	Mode        *string
	BoardSize   *int
	Since       *time.Time
	Until       *time.Time
	Limit       int
	BeforeStart *time.Time // keyset cursor
}

// Service reads and writes the archive.
type Service struct {
	db   *pgxpool.Pool
	blob blob.Store
}

// NewService builds the archive service.
func NewService(db *pgxpool.Pool, store blob.Store) *Service {
	return &Service{db: db, blob: store}
}

// Get returns one game if the caller played in it. Games are private to their
// participants until finished; a finished game is world-readable so it can be
// shared and spectated.
func (s *Service) Get(ctx context.Context, gameID, viewerID uuid.UUID) (*Game, error) {
	g, err := s.scanOne(ctx, `
		SELECT g.id, g.black_user_id, g.white_user_id, g.black_bot_id, g.white_bot_id,
		       COALESCE(bu.display_name, g.black_bot_id, 'Black'),
		       COALESCE(wu.display_name, g.white_bot_id, 'White'),
		       g.board_size, g.ruleset, g.komi, g.handicap, g.time_control,
		       g.time_class, g.mode, g.status, g.result, g.winner, g.end_reason,
		       g.move_count, g.started_at, g.ended_at
		FROM games g
		LEFT JOIN users bu ON bu.id = g.black_user_id
		LEFT JOIN users wu ON wu.id = g.white_user_id
		WHERE g.id = $1`, gameID)
	if err != nil {
		return nil, err
	}
	if g.Status == "active" && !participates(g, viewerID) {
		// A private room code grants its members spectator access. Guessing
		// a game id alone never grants access to a live private game.
		var member bool
		if err := s.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM rooms r
            JOIN room_members m ON m.room_id=r.id WHERE r.game_id=$1 AND m.user_id=$2)`,
			gameID, viewerID).Scan(&member); err != nil {
			return nil, err
		}
		if !member {
			return nil, ErrNotFound
		}
	}
	return g, nil
}

func participates(g *Game, userID uuid.UUID) bool {
	return (g.BlackUserID != nil && *g.BlackUserID == userID) ||
		(g.WhiteUserID != nil && *g.WhiteUserID == userID)
}

func (s *Service) scanOne(ctx context.Context, sql string, args ...any) (*Game, error) {
	var g Game
	err := s.db.QueryRow(ctx, sql, args...).Scan(
		&g.ID, &g.BlackUserID, &g.WhiteUserID, &g.BlackBotID, &g.WhiteBotID,
		&g.BlackName, &g.WhiteName, &g.BoardSize, &g.Ruleset, &g.Komi, &g.Handicap,
		&g.TimeControl, &g.TimeClass, &g.Mode, &g.Status, &g.Result, &g.Winner,
		&g.EndReason, &g.MoveCount, &g.StartedAt, &g.EndedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get game: %w", err)
	}
	return &g, nil
}

// Moves returns a game's move list in order.
func (s *Service) Moves(ctx context.Context, gameID uuid.UUID) ([]MoveRow, error) {
	rows, err := s.db.Query(ctx, `
		SELECT move_number, player, kind, row, col, captured, state_hash,
		       think_millis, played_at
		FROM game_moves WHERE game_id = $1 ORDER BY move_number`, gameID)
	if err != nil {
		return nil, fmt.Errorf("query moves: %w", err)
	}
	defer rows.Close()
	out := []MoveRow{}
	for rows.Next() {
		var m MoveRow
		if err := rows.Scan(&m.MoveNumber, &m.Player, &m.Kind, &m.Row, &m.Col,
			&m.Captured, &m.StateHash, &m.ThinkMillis, &m.PlayedAt); err != nil {
			return nil, fmt.Errorf("scan move: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// Search returns a page of the caller's games, newest first.
func (s *Service) Search(ctx context.Context, q SearchQuery) ([]Game, error) {
	if q.Limit <= 0 || q.Limit > 100 {
		q.Limit = 25
	}
	var (
		where = []string{"(g.black_user_id = $1 OR g.white_user_id = $1)"}
		args  = []any{q.UserID}
	)
	add := func(clause string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}
	if q.Opponent != nil {
		add("(g.black_user_id = $%[1]d OR g.white_user_id = $%[1]d)", *q.Opponent)
	}
	if q.TimeClass != nil {
		add("g.time_class = $%d", *q.TimeClass)
	}
	if q.Mode != nil {
		add("g.mode = $%d", *q.Mode)
	}
	if q.BoardSize != nil {
		add("g.board_size = $%d", *q.BoardSize)
	}
	if q.Since != nil {
		add("g.started_at >= $%d", *q.Since)
	}
	if q.Until != nil {
		add("g.started_at <= $%d", *q.Until)
	}
	if q.BeforeStart != nil {
		add("g.started_at < $%d", *q.BeforeStart)
	}
	if q.Result != nil {
		// Result is relative to the caller, so it has to be expressed in terms
		// of which side they played.
		switch *q.Result {
		case "win":
			where = append(where, `((g.winner = 'black' AND g.black_user_id = $1) OR (g.winner = 'white' AND g.white_user_id = $1))`)
		case "loss":
			where = append(where, `((g.winner = 'white' AND g.black_user_id = $1) OR (g.winner = 'black' AND g.white_user_id = $1))`)
		case "draw":
			where = append(where, `g.winner = 'draw'`)
		}
	}

	args = append(args, q.Limit)
	sql := fmt.Sprintf(`
		SELECT g.id, g.black_user_id, g.white_user_id, g.black_bot_id, g.white_bot_id,
		       COALESCE(bu.display_name, g.black_bot_id, 'Black'),
		       COALESCE(wu.display_name, g.white_bot_id, 'White'),
		       g.board_size, g.ruleset, g.komi, g.handicap, g.time_control,
		       g.time_class, g.mode, g.status, g.result, g.winner, g.end_reason,
		       g.move_count, g.started_at, g.ended_at
		FROM games g
		LEFT JOIN users bu ON bu.id = g.black_user_id
		LEFT JOIN users wu ON wu.id = g.white_user_id
		WHERE %s
		ORDER BY g.started_at DESC
		LIMIT $%d`, strings.Join(where, " AND "), len(args))

	rows, err := s.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("search games: %w", err)
	}
	defer rows.Close()
	out := []Game{}
	for rows.Next() {
		var g Game
		if err := rows.Scan(&g.ID, &g.BlackUserID, &g.WhiteUserID, &g.BlackBotID,
			&g.WhiteBotID, &g.BlackName, &g.WhiteName, &g.BoardSize, &g.Ruleset,
			&g.Komi, &g.Handicap, &g.TimeControl, &g.TimeClass, &g.Mode, &g.Status,
			&g.Result, &g.Winner, &g.EndReason, &g.MoveCount, &g.StartedAt,
			&g.EndedAt); err != nil {
			return nil, fmt.Errorf("scan game: %w", err)
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// SGF returns a game's SGF, generating and caching it on first request if the
// game finished before the uploader ran.
func (s *Service) SGF(ctx context.Context, gameID, viewerID uuid.UUID) ([]byte, error) {
	g, err := s.Get(ctx, gameID, viewerID)
	if err != nil {
		return nil, err
	}
	var key *string
	if err := s.db.QueryRow(ctx,
		`SELECT sgf_object_key FROM games WHERE id = $1`, gameID).Scan(&key); err != nil {
		return nil, fmt.Errorf("read sgf key: %w", err)
	}
	if key != nil && *key != "" {
		if body, err := s.blob.Get(ctx, *key); err == nil {
			return body, nil
		}
		// Fall through and regenerate: a missing object should not 500.
	}
	body, err := s.GenerateSGF(ctx, g)
	if len(body) > 0 {
		// Reads can serve a regenerated body during a storage outage. Completion
		// jobs call GenerateSGF directly and must retry the failed upload.
		return body, nil
	}
	return body, err
}

// GenerateSGF replays game_moves and renders SGF, then stores it.
func (s *Service) GenerateSGF(ctx context.Context, g *Game) ([]byte, error) {
	moves, err := s.Moves(ctx, g.ID)
	if err != nil {
		return nil, err
	}
	cfg := goban.NewConfig(g.BoardSize, goban.Ruleset(g.Ruleset), g.Handicap)
	cfg.Komi = g.Komi
	state := goban.NewGame(cfg)

	for _, m := range moves {
		if state.Status == goban.StatusScoring {
			state.Status = goban.StatusActive
			state.ConsecutivePasses = 0
		}
		player := goban.Black
		if m.Player == "white" {
			player = goban.White
		} else if m.Player != "black" {
			return nil, fmt.Errorf("move %d has invalid player", m.MoveNumber)
		}
		if m.Kind == "resign" {
			state.ToMove = player
		}
		if player != state.ToMove || m.MoveNumber != state.MoveNumber+1 {
			return nil, fmt.Errorf("move %d has invalid order", m.MoveNumber)
		}
		var intent goban.Intent
		switch m.Kind {
		case "place":
			if m.Row == nil || m.Col == nil {
				return nil, fmt.Errorf("move %d is a placement without coordinates", m.MoveNumber)
			}
			intent = goban.PlaceAt(goban.Point{Row: *m.Row, Col: *m.Col})
		case "pass":
			intent = goban.PassIntent()
		case "resign":
			intent = goban.ResignIntent()
		default:
			return nil, fmt.Errorf("move %d has invalid kind", m.MoveNumber)
		}
		next, _, err := state.Apply(intent)
		if err != nil {
			// A stored move that no longer validates means the archive and the
			// engine disagree; surface it rather than writing a corrupt SGF.
			return nil, fmt.Errorf("replay move %d: %w", m.MoveNumber, err)
		}
		state = next
	}

	meta := goban.SGFMeta{
		BlackName: g.BlackName,
		WhiteName: g.WhiteName,
		Date:      g.StartedAt,
	}
	if g.Result != nil {
		meta.Result = *g.Result
	}
	body := []byte(goban.ExportSGF(state, meta))

	key := blob.GameKey(g.ID.String())
	if err := s.blob.Put(ctx, key, body, "application/x-go-sgf"); err != nil {
		return body, fmt.Errorf("store sgf: %w", err)
	}
	_, err = s.db.Exec(ctx, `UPDATE games SET sgf_object_key = $2 WHERE id = $1`, g.ID, key)
	return body, err
}

// SGFURL returns a time-limited direct download URL when the SGF is stored.
func (s *Service) SGFURL(ctx context.Context, gameID uuid.UUID, ttl time.Duration) (string, error) {
	var key *string
	if err := s.db.QueryRow(ctx,
		`SELECT sgf_object_key FROM games WHERE id = $1`, gameID).Scan(&key); err != nil {
		return "", ErrNotFound
	}
	if key == nil || *key == "" {
		return "", ErrNotFound
	}
	return s.blob.SignedURL(*key, ttl)
}
