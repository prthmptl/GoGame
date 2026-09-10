package game

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/prathpatel/gogame-backend/internal/clock"
	"github.com/prathpatel/gogame-backend/internal/goban"
)

var errStaleSession = errors.New("game: session changed; reconnect required")

type runtimeState struct {
	Rules      goban.Config     `json:"rules"`
	Status     goban.Status     `json:"status"`
	Clock      clock.State      `json:"clock"`
	LastMoveAt time.Time        `json:"lastMoveAt"`
	Dead       []goban.Point    `json:"dead"`
	Confirmed  map[string]bool  `json:"confirmed"`
	LastSeq    map[string]int64 `json:"lastSeq"`
}

// persist commits the move, runtime state and final result atomically. Nothing
// is acknowledged to a client before this transaction commits.
func (s *Session) persist(ctx context.Context, move *goban.Move, thinkMillis int, result *Result) error {
	raw, err := json.Marshal(runtimeState{
		Rules: s.cfg.Rules, Status: s.state.Status,
		Clock: s.clock.Export(s.lastChargeAt), LastMoveAt: s.lastMoveAt,
		Dead: s.deadPoints(), Confirmed: s.confirmed, LastSeq: s.lastSeq,
	})
	if err != nil {
		return err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE games
        SET session_state = $2, session_version = session_version + 1, move_count = $3
        WHERE id = $1 AND session_version = $4 AND status = 'active'`,
		s.ID, raw, s.state.MoveNumber, s.revision)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errStaleSession
	}
	if move != nil {
		var row, col *int
		if move.Point != nil {
			row, col = &move.Point.Row, &move.Point.Col
		}
		captured := move.Captured
		if captured == nil {
			captured = []goban.Point{}
		}
		captures, _ := json.Marshal(captured)
		clockJSON, _ := json.Marshal(s.clock.Export(s.lastChargeAt))
		_, err = tx.Exec(ctx, `INSERT INTO game_moves
            (game_id, move_number, player, kind, row, col, captured, state_hash, think_millis, clock_after)
            VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			s.ID, move.Number, colorName(move.Player), string(move.Kind), row, col,
			captures, s.state.StateHash(), thinkMillis, clockJSON)
		if err != nil {
			return fmt.Errorf("insert move: %w", err)
		}
	}
	if result != nil {
		var black, white *float64
		if result.Score != nil {
			b, w := result.Score.BlackTotal(), result.Score.WhiteTotal()
			black, white = &b, &w
		}
		_, err = tx.Exec(ctx, `UPDATE games SET status = 'completed', result = $2,
            winner = $3, end_reason = $4, black_score = $5, white_score = $6, ended_at = now()
            WHERE id = $1`, s.ID, result.Result, result.Winner, result.EndReason, black, white)
		if err != nil {
			return err
		}
		payload, err := json.Marshal(result)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO game_completion_outbox (game_id, result) VALUES ($1,$2)`, s.ID, payload); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	s.revision++
	return nil
}

func (s *Session) persistenceFailed(c command, err error) {
	slog.Error("game persistence failed", "gameId", s.ID, "error", err)
	s.reject(c, "persist_failed")
	// A commit error can have an ambiguous outcome. Require a reload from
	// durable state so a retry can never overwrite a committed transition.
	s.emit("ERROR", map[string]string{"code": "resync_required", "message": "Reconnect to restore the saved game."})
	s.Close()
}
