// Command worker runs scheduled background jobs.
//
// C1 requires the worker process to exist in the skeleton. Today it runs one
// job: pruning refresh tokens that are expired or long revoked. Later phases
// add the D3 clock-timeout watcher and the D4 matchmaking pairing loop here.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prathpatel/gogame-backend/internal/config"
	"github.com/prathpatel/gogame-backend/internal/logging"
	"github.com/prathpatel/gogame-backend/internal/notify"
	"github.com/prathpatel/gogame-backend/internal/profile"
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

	profiles := profile.NewService(st.DB)
	var sender notify.Sender = notify.NoopSender{}
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
		case <-leaderboard.C:
			refreshLeaderboard(ctx, profiles, lg)
		case <-pushes.C:
			deliverNotifications(ctx, notifier, lg)
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
