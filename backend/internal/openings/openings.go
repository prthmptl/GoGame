// Package openings implements F3: the joseki / opening explorer.
package openings

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/prathpatel/gogame-backend/internal/goban"
)

// ErrNotFound is returned for an unseen position.
var ErrNotFound = errors.New("openings: position not in the index")

// MoveStat is one continuation from a position.
type MoveStat struct {
	Row int `json:"row"`
	Col int `json:"col"`
	// Games is how often this move was played from here.
	Games int `json:"games"`
	// WinRate is for the player who plays the move, not for black — so the
	// number always reads "how well this move did for me".
	WinRate  float64 `json:"winRate"`
	Draws    int     `json:"draws"`
	NextHash *string `json:"nextHash,omitempty"`
	Name     *string `json:"name,omitempty"`
}

// Lookup is the explorer's answer for one position.
type Lookup struct {
	PositionHash string     `json:"positionHash"`
	BoardSize    int        `json:"boardSize"`
	ToMove       string     `json:"toMove"`
	Games        int        `json:"games"`
	Moves        []MoveStat `json:"moves"`
}

// Service reads and builds the index.
type Service struct{ db *pgxpool.Pool }

// NewService builds the openings service.
func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

// Lookup returns the recorded continuations for a position hash.
func (s *Service) Lookup(ctx context.Context, positionHash string, limit int) (*Lookup, error) {
	if limit <= 0 || limit > 50 {
		limit = 12
	}
	var out Lookup
	err := s.db.QueryRow(ctx, `
		SELECT position_hash, board_size, to_move, games
		FROM opening_positions WHERE position_hash = $1`, positionHash,
	).Scan(&out.PositionHash, &out.BoardSize, &out.ToMove, &out.Games)
	if err != nil {
		return nil, ErrNotFound
	}

	rows, err := s.db.Query(ctx, `
		SELECT row, col, games, wins, draws, next_hash, name
		FROM opening_moves
		WHERE position_hash = $1
		ORDER BY games DESC
		LIMIT $2`, positionHash, limit)
	if err != nil {
		return nil, fmt.Errorf("lookup moves: %w", err)
	}
	defer rows.Close()

	out.Moves = []MoveStat{}
	for rows.Next() {
		var m MoveStat
		var wins int
		if err := rows.Scan(&m.Row, &m.Col, &m.Games, &wins, &m.Draws,
			&m.NextHash, &m.Name); err != nil {
			return nil, err
		}
		if m.Games > 0 {
			// Draws count as half, matching how a score is normally read.
			m.WinRate = (float64(wins) + 0.5*float64(m.Draws)) / float64(m.Games)
		}
		out.Moves = append(out.Moves, m)
	}
	return &out, rows.Err()
}

// LookupPosition hashes a live position and looks it up, which is what the
// client's explorer view calls while a game or review is open.
func (s *Service) LookupPosition(ctx context.Context, state *goban.State, limit int) (*Lookup, error) {
	return s.Lookup(ctx, state.StateHash(), limit)
}

// IndexGame walks one finished game and records every position in it, up to
// maxMoves. Beyond the opening the tree becomes too sparse to be useful and
// the index would grow without bound.
//
// winner is "black", "white" or "draw".
func (s *Service) IndexGame(ctx context.Context, cfg goban.Config, moves []goban.Intent, winner string, maxMoves int) error {
	if maxMoves <= 0 {
		maxMoves = 40
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	state := goban.NewGame(cfg)
	for i, intent := range moves {
		if i >= maxMoves {
			break
		}
		if intent.Kind != goban.Place || intent.Point == nil {
			// Passes and resignations end the useful part of an opening line.
			break
		}
		mover := colorName(state.ToMove)
		hash := state.StateHash()

		next, _, err := state.Apply(intent)
		if err != nil {
			// A game that no longer validates should not corrupt the index.
			return fmt.Errorf("index move %d: %w", i+1, err)
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO opening_positions (position_hash, board_size, to_move, move_number, games)
			VALUES ($1,$2,$3,$4,1)
			ON CONFLICT (position_hash) DO UPDATE
			SET games = opening_positions.games + 1, updated_at = now()`,
			hash, cfg.BoardSize, mover, i); err != nil {
			return fmt.Errorf("upsert position: %w", err)
		}

		won, drew := 0, 0
		switch {
		case winner == "draw":
			drew = 1
		case winner == mover:
			won = 1
		}
		nextHash := next.StateHash()
		if _, err := tx.Exec(ctx, `
			INSERT INTO opening_moves (position_hash, row, col, games, wins, draws, next_hash)
			VALUES ($1,$2,$3,1,$4,$5,$6)
			ON CONFLICT (position_hash, row, col) DO UPDATE
			SET games = opening_moves.games + 1,
			    wins = opening_moves.wins + EXCLUDED.wins,
			    draws = opening_moves.draws + EXCLUDED.draws,
			    next_hash = COALESCE(opening_moves.next_hash, EXCLUDED.next_hash)`,
			hash, intent.Point.Row, intent.Point.Col, won, drew, nextHash); err != nil {
			return fmt.Errorf("upsert move: %w", err)
		}
		state = next
	}
	return tx.Commit(ctx)
}

// NameSequence labels a position's continuation, e.g. "3-3 invasion".
func (s *Service) NameSequence(ctx context.Context, positionHash string, row, col int, name string) error {
	_, err := s.db.Exec(ctx, `
		UPDATE opening_moves SET name = NULLIF($4,'')
		WHERE position_hash = $1 AND row = $2 AND col = $3`,
		positionHash, row, col, name)
	return err
}

func colorName(c goban.Color) string {
	if c == goban.Black {
		return "black"
	}
	return "white"
}
