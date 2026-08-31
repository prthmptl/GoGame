package api

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/prathpatel/gogame-backend/internal/anticheat"
	"github.com/prathpatel/gogame-backend/internal/archive"
	"github.com/prathpatel/gogame-backend/internal/game"
	"github.com/prathpatel/gogame-backend/internal/notify"
	"github.com/prathpatel/gogame-backend/internal/rating"
)

// GameHooks wires what happens when a game ends: E1 rates it, E2 collects
// anti-cheat signals, C4 archives the SGF and C5 notifies the players.
//
// Every step is independent and failure-tolerant: a rating update that fails
// must not stop the SGF being written, and neither must break the game that
// already finished.
func GameHooks(db *pgxpool.Pool, ratings *rating.Service, cheats *anticheat.Service,
	arch *archive.Service, notifier *notify.Service, lg *slog.Logger) game.Hooks {
	return game.Hooks{
		OnGameEnded: func(ctx context.Context, gameID uuid.UUID, result game.Result) {
			// The hook runs on the session's actor goroutine, so hand the work
			// off rather than blocking the game loop on database round-trips.
			go func() {
				bg := context.WithoutCancel(ctx)
				finishGame(bg, db, ratings, cheats, arch, notifier, lg, gameID, result)
			}()
		},
	}
}

func finishGame(ctx context.Context, db *pgxpool.Pool, ratings *rating.Service,
	cheats *anticheat.Service, arch *archive.Service, notifier *notify.Service,
	lg *slog.Logger, gameID uuid.UUID, result game.Result) {

	var (
		blackID, whiteID *uuid.UUID
		boardSize        int
		timeClass, mode  string
	)
	if err := db.QueryRow(ctx, `
		SELECT black_user_id, white_user_id, board_size, time_class, mode
		FROM games WHERE id = $1`, gameID,
	).Scan(&blackID, &whiteID, &boardSize, &timeClass, &mode); err != nil {
		lg.Error("finish game: load failed", "gameId", gameID, "error", err)
		return
	}

	// E1: only rated modes move ratings, and only between two real accounts.
	if mode == "ranked" && blackID != nil && whiteID != nil && result.Winner != "" {
		if err := ratings.ApplyGame(ctx, rating.GameResult{
			GameID: gameID, BoardSize: boardSize, TimeClass: timeClass,
			BlackID: *blackID, WhiteID: *whiteID, Winner: result.Winner,
		}); err != nil {
			lg.Error("rating update failed", "gameId", gameID, "error", err)
		}
	}

	// E2: collect signals from the finished game.
	for _, id := range []*uuid.UUID{blackID, whiteID} {
		if id == nil {
			continue
		}
		if err := collectSignals(ctx, db, cheats, gameID, *id); err != nil {
			lg.Error("anti-cheat collection failed", "gameId", gameID, "userId", *id, "error", err)
		}
	}

	// C4: write the SGF so the archive does not have to generate it on read.
	if g, err := arch.Get(ctx, gameID, uuid.Nil); err == nil {
		if _, err := arch.GenerateSGF(ctx, g); err != nil {
			lg.Error("sgf generation failed", "gameId", gameID, "error", err)
		}
	}

	// C5: tell each player their game is over.
	for _, id := range []*uuid.UUID{blackID, whiteID} {
		if id == nil {
			continue
		}
		_ = notifier.Enqueue(ctx, nil, notify.Notification{
			UserID:    *id,
			EventType: notify.EventCorrespondenceTurn,
			Title:     "Game finished",
			Body:      "Your game ended: " + result.Result,
			Data:      map[string]string{"gameId": gameID.String()},
			// Collapsing on the game id keeps one notification per game.
			CollapseKey: "game-" + gameID.String(),
		})
	}
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
