// Command api serves the GoGame HTTP API (C1 skeleton, C2 auth).
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prathpatel/gogame-backend/internal/api"
	"github.com/prathpatel/gogame-backend/internal/auth"
	"github.com/prathpatel/gogame-backend/internal/config"
	"github.com/prathpatel/gogame-backend/internal/logging"
	"github.com/prathpatel/gogame-backend/internal/store"
)

// version is stamped at build time: -ldflags "-X main.version=$(git rev-parse --short HEAD)".
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
	lg := logging.New(string(cfg.Env)).With("service", "api", "version", version)

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
	lg.Info("migrations applied")

	authSvc := auth.NewService(auth.Options{
		DB:         st.DB,
		Verifier:   auth.NewGoogleVerifier(cfg.GoogleClientIDs),
		JWTSecret:  cfg.JWTSecret,
		Issuer:     cfg.JWTIssuer,
		AccessTTL:  cfg.AccessTokenTTL,
		RefreshTTL: cfg.RefreshTokenTTL,
	})

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           api.New(st, authSvc, lg, version).Routes(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		lg.Info("listening", "addr", srv.Addr, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
		lg.Info("shutdown signal received")
	}

	// Drain in-flight requests before exiting so a deploy does not cut
	// connections mid-response.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownGracePeriod)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	lg.Info("stopped cleanly")
	return nil
}
