// Command worker runs scheduled background jobs.
//
// Delivery and maintenance run here. Live clocks, recovery and matchmaking
// run in the API process alongside the game hub.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prathpatel/gogame-backend/internal/billing"
	"github.com/prathpatel/gogame-backend/internal/config"
	"github.com/prathpatel/gogame-backend/internal/correspondence"
	"github.com/prathpatel/gogame-backend/internal/logging"
	"github.com/prathpatel/gogame-backend/internal/notify"
	"github.com/prathpatel/gogame-backend/internal/profile"
	"github.com/prathpatel/gogame-backend/internal/rating"
	"github.com/prathpatel/gogame-backend/internal/rooms"
	"github.com/prathpatel/gogame-backend/internal/store"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	lg := logging.New(string(cfg.Env)).With("service", "worker", "version", version)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st, err := store.Open(ctx, cfg.DatabaseURL, cfg.RedisURL)
	if err != nil {
		return err
	}
	defer st.Close()

	if err := st.Migrate(ctx); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	profiles := profile.NewService(st.DB)
	var sender notify.Sender
	if fcm := notify.NewFCMFromEnv(); fcm != nil {
		sender = fcm
		lg.Info("push notifications enabled", "project", fcm.ProjectID)
	} else {
		lg.Warn("FCM_PROJECT_ID not set; notifications will be queued but not delivered")
	}
	notifier := notify.NewService(st.DB, sender)

	// Each job runs on its own cadence: pushes must go out promptly, while
	// token pruning and the leaderboard rebuild are cheap to defer.
	hourly := time.NewTicker(time.Hour)
	defer hourly.Stop()
	leaderboard := time.NewTicker(5 * time.Minute)
	defer leaderboard.Stop()
	pushes := time.NewTicker(10 * time.Second)
	defer pushes.Stop()
	roomSvc := rooms.NewService(st.DB)
	ratingSvc := rating.NewService(st.DB)
	corrSvc := correspondence.NewService(st.DB)
	iapSvc := billing.NewIAPService(st.DB, billing.UnconfiguredValidator{})

	// E4: correspondence deadlines are checked every minute. Daily games have
	// day-long budgets, so a minute of slack is immaterial and the query is
	// cheap against the partial index.
	daily := time.NewTicker(time.Minute)
	defer daily.Stop()

	lg.Info("worker started")
	pruneExpiredTokens(ctx, st, lg)
	refreshLeaderboard(ctx, profiles, lg)

	for {
		select {
		case <-ctx.Done():
			lg.Info("worker stopped cleanly")
			return nil
		case <-hourly.C:
			pruneExpiredTokens(ctx, st, lg)
			sweepRooms(ctx, roomSvc, lg)
			decayRatings(ctx, ratingSvc, lg)
			expireSubscriptions(ctx, iapSvc, lg)
		case <-leaderboard.C:
			refreshLeaderboard(ctx, profiles, lg)
		case <-pushes.C:
			deliverNotifications(ctx, notifier, lg)
		case <-daily.C:
			expireCorrespondence(ctx, corrSvc, st, lg)
			drainVacations(ctx, corrSvc, lg)
		}
	}
}

// refreshLeaderboard rebuilds the C3 materialized view. C3 asks for a refresh
// "every few minutes"; five is frequent enough that a rank feels live without
// the rebuild ever overlapping itself.
func refreshLeaderboard(ctx context.Context, p *profile.Service, lg *slog.Logger) {
	start := time.Now()
	if err := p.RefreshLeaderboard(ctx); err != nil {
		lg.Error("leaderboard refresh failed", "error", err)
		return
	}
	lg.Info("leaderboard refreshed", "durationMs", time.Since(start).Milliseconds())
}

// sweepRooms closes D5 rooms nobody used.
func sweepRooms(ctx context.Context, r *rooms.Service, lg *slog.Logger) {
	n, err := r.SweepExpired(ctx)
	if err != nil {
		lg.Error("room sweep failed", "error", err)
		return
	}
	if n > 0 {
		lg.Info("expired rooms closed", "rooms", n)
	}
}

// decayRatings raises deviation for players who have not played in a while,
// so a rating that has gone stale becomes uncertain again (E1).
func decayRatings(ctx context.Context, r *rating.Service, lg *slog.Logger) {
	n, err := r.DecayInactive(ctx, 30*24*time.Hour)
	if err != nil {
		lg.Error("rating decay failed", "error", err)
		return
	}
	if n > 0 {
		lg.Info("ratings decayed for inactivity", "rows", n)
	}
}

// expireCorrespondence forfeits daily games whose deadline has passed (E4).
func expireCorrespondence(ctx context.Context, c *correspondence.Service, st *store.Store, lg *slog.Logger) {
	expired, err := c.Expired(ctx, 100)
	if err != nil {
		lg.Error("correspondence expiry scan failed", "error", err)
		return
	}
	for _, state := range expired {
		winner := "white"
		letter := "W"
		if state.ToMove == "white" {
			winner, letter = "black", "B"
		}
		// The player on move ran out of days, so they lose on time.
		if _, err := st.DB.Exec(ctx, `
			UPDATE games SET status = 'completed', winner = $2, result = $3,
			       end_reason = 'timeout', ended_at = now()
			WHERE id = $1 AND status = 'active'`,
			state.GameID, winner, letter+"+T"); err != nil {
			lg.Error("correspondence timeout failed", "gameId", state.GameID, "error", err)
			continue
		}
		lg.Info("correspondence game timed out", "gameId", state.GameID, "winner", winner)
	}
}

// drainVacations ends vacations for players whose banked days ran out (E4).
func drainVacations(ctx context.Context, c *correspondence.Service, lg *slog.Logger) {
	exhausted, err := c.DrainVacationDays(ctx)
	if err != nil {
		lg.Error("vacation drain failed", "error", err)
		return
	}
	if len(exhausted) > 0 {
		lg.Info("vacations ended", "users", len(exhausted))
	}
}

// expireSubscriptions downgrades lapsed subscribers (G1).
//
// Access is driven by expires_at everywhere else, so this is bookkeeping
// rather than enforcement: a lapsed row already resolves to the free tier.
func expireSubscriptions(ctx context.Context, iap *billing.IAPService, lg *slog.Logger) {
	n, err := iap.ExpireLapsed(ctx)
	if err != nil {
		lg.Error("subscription expiry failed", "error", err)
		return
	}
	if n > 0 {
		lg.Info("subscriptions expired", "rows", n)
	}
}

// deliverNotifications drains the C5 outbox.
func deliverNotifications(ctx context.Context, n *notify.Service, lg *slog.Logger) {
	delivered, failed, err := n.DeliverBatch(ctx, 100)
	if err != nil {
		lg.Error("notification delivery failed", "error", err)
		return
	}
	if delivered > 0 || failed > 0 {
		lg.Info("notifications processed", "delivered", delivered, "failed", failed)
	}
}

// pruneExpiredTokens deletes refresh tokens that can no longer be exchanged.
// Revoked rows are kept for 30 days first: a reuse-revoked family is the
// evidence trail for a token theft, and deleting it immediately would erase
// the only record that the theft happened.
func pruneExpiredTokens(ctx context.Context, st *store.Store, lg *slog.Logger) {
	tag, err := st.DB.Exec(ctx, `
		DELETE FROM refresh_tokens
		WHERE expires_at < now() - INTERVAL '7 days'
		   OR (revoked_at IS NOT NULL AND revoked_at < now() - INTERVAL '30 days')`)
	if err != nil {
		lg.Error("prune refresh tokens failed", "error", err)
		return
	}
	if n := tag.RowsAffected(); n > 0 {
		lg.Info("pruned refresh tokens", "rows", n)
	}
}
