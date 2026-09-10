package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/prathpatel/gogame-backend/internal/anticheat"
	"github.com/prathpatel/gogame-backend/internal/archive"
	"github.com/prathpatel/gogame-backend/internal/game"
	"github.com/prathpatel/gogame-backend/internal/notify"
	"github.com/prathpatel/gogame-backend/internal/rating"
)

func finishGame(ctx context.Context, db *pgxpool.Pool, ratings *rating.Service,
	cheats *anticheat.Service, arch *archive.Service, notifier *notify.Service,
	lg *slog.Logger, gameID uuid.UUID, result game.Result, tx pgx.Tx) error {

	var (
		blackID, whiteID *uuid.UUID
		boardSize        int
		timeClass, mode  string
	)
	if err := db.QueryRow(ctx, `
		SELECT black_user_id, white_user_id, board_size, time_class, mode
		FROM games WHERE id = $1`, gameID,
	).Scan(&blackID, &whiteID, &boardSize, &timeClass, &mode); err != nil {
		return err
	}

	// E1: only rated modes move ratings, and only between two real accounts.
	if mode == "ranked" && blackID != nil && whiteID != nil && result.Winner != "" {
		if err := ratings.ApplyGame(ctx, rating.GameResult{
			GameID: gameID, BoardSize: boardSize, TimeClass: timeClass,
			BlackID: *blackID, WhiteID: *whiteID, Winner: result.Winner,
		}); err != nil {
			return err
		}
	}

	// E2: collect signals from the finished game.
	for _, id := range []*uuid.UUID{blackID, whiteID} {
		if id == nil {
			continue
		}
		if err := collectSignals(ctx, db, cheats, gameID, *id); err != nil {
			return err
		}
	}

	// C4: write the SGF so the archive does not have to generate it on read.
	g, err := arch.Get(ctx, gameID, uuid.Nil)
	if err != nil {
		return err
	}
	if _, err := arch.GenerateSGF(ctx, g); err != nil {
		return err
	}

	// C5: tell each player their game is over.
	for _, id := range []*uuid.UUID{blackID, whiteID} {
		if id == nil {
			continue
		}
		if err := notifier.Enqueue(ctx, tx, notify.Notification{
			UserID:    *id,
			EventType: notify.EventCorrespondenceTurn,
			Title:     "Game finished",
			Body:      "Your game ended: " + result.Result,
			Data:      map[string]string{"gameId": gameID.String()},
			// Collapsing on the game id keeps one notification per game.
			CollapseKey: "game-" + gameID.String(),
		}); err != nil {
			return err
		}
	}
	return nil
}

// ProcessGameCompletions retries durable completion work left by a crash or
// dependency outage. Multiple instances claim different jobs with row locks.
func ProcessGameCompletions(ctx context.Context, db *pgxpool.Pool, ratings *rating.Service,
	cheats *anticheat.Service, arch *archive.Service, notifier *notify.Service, lg *slog.Logger) error {
	var failures error
	for i := 0; i < 100; i++ {
		if err := processCompletion(ctx, db, ratings, cheats, arch, notifier, lg, nil); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return failures
			}
			failures = errors.Join(failures, err)
			var retry *completionRetry
			if !errors.As(err, &retry) {
				return failures
			}
		}
	}
	return failures
}

type completionRetry struct{ error }

func processCompletion(ctx context.Context, db *pgxpool.Pool, ratings *rating.Service,
	cheats *anticheat.Service, arch *archive.Service, notifier *notify.Service,
	lg *slog.Logger, gameID *uuid.UUID) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var id uuid.UUID
	var raw []byte
	err = tx.QueryRow(ctx, `SELECT game_id, result FROM game_completion_outbox
        WHERE (($1::uuid IS NULL AND next_attempt_at <= now()) OR game_id = $1) ORDER BY created_at
        LIMIT 1 FOR UPDATE SKIP LOCKED`, gameID).Scan(&id, &raw)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `SAVEPOINT completion_work`); err != nil {
		return err
	}
	var result game.Result
	err = json.Unmarshal(raw, &result)
	if err == nil {
		err = finishGame(ctx, db, ratings, cheats, arch, notifier, lg, id, result, tx)
	}
	if err != nil {
		workErr := err
		if _, err := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT completion_work`); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE game_completion_outbox SET attempts=attempts+1,
            next_attempt_at=now()+make_interval(secs => LEAST(3600, 30*power(2, LEAST(attempts,7)))::int)
            WHERE game_id=$1`, id); err != nil {
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		lg.Warn("game completion deferred", "gameId", id, "error", workErr)
		return &completionRetry{workErr}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM game_completion_outbox WHERE game_id=$1`, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// collectSignals computes E2's per-game signals for one player.
func collectSignals(ctx context.Context, db *pgxpool.Pool, cheats *anticheat.Service,
	gameID, userID uuid.UUID) error {

	rows, err := db.Query(ctx, `
		SELECT gm.think_millis
		FROM game_moves gm
		JOIN games g ON g.id = gm.game_id
		WHERE gm.game_id = $1
		  AND ((gm.player = 'black' AND g.black_user_id = $2)
		    OR (gm.player = 'white' AND g.white_user_id = $2))
		ORDER BY gm.move_number`, gameID, userID)
	if err != nil {
		return err
	}
	defer rows.Close()
	var timings []int
	for rows.Next() {
		var ms int
		if err := rows.Scan(&ms); err != nil {
			return err
		}
		timings = append(timings, ms)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(timings) == 0 {
		return nil
	}

	// Move-timing is the only signal computable from stored data alone.
	// Engine correlation needs the analysis engine to run over the game,
	// which belongs in a separate job rather than the completion path.
	signals := []anticheat.Signal{anticheat.MoveTimingUniformity(timings)}
	return cheats.RecordSignals(ctx, userID, gameID, signals)
}
