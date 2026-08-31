// Command worker runs scheduled background jobs.
//
// C1 requires the worker process to exist in the skeleton. Today it runs one
// job: pruning refresh tokens that are expired or long revoked. Later phases
// add the D3 clock-timeout watcher and the D4 matchmaking pairing loop here.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prathpatel/gogame-backend/internal/config"
	"github.com/prathpatel/gogame-backend/internal/logging"
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

	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	lg.Info("worker started")
	pruneExpiredTokens(ctx, st, lg)
	for {
		select {
		case <-ctx.Done():
			lg.Info("worker stopped cleanly")
			return nil
		case <-ticker.C:
			pruneExpiredTokens(ctx, st, lg)
		}
	}
}

// pruneExpiredTokens deletes refresh tokens that can no longer be exchanged.
// Revoked rows are kept for 30 days first: a reuse-revoked family is the
// evidence trail for a token theft, and deleting it immediately would erase
// the only record that the theft happened.
func pruneExpiredTokens(ctx context.Context, st *store.Store, lg interface {
	Info(string, ...any)
	Error(string, ...any)
}) {
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
