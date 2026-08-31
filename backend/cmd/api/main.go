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
	"github.com/prathpatel/gogame-backend/internal/archive"
	"github.com/prathpatel/gogame-backend/internal/auth"
	"github.com/prathpatel/gogame-backend/internal/blob"
	"github.com/prathpatel/gogame-backend/internal/config"
	"github.com/prathpatel/gogame-backend/internal/logging"
	"github.com/prathpatel/gogame-backend/internal/notify"
	"github.com/prathpatel/gogame-backend/internal/profile"
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

	// Object storage is optional: without S3_BUCKET the archive keeps SGF
	// bodies in memory, which is fine for local development.
	var blobStore blob.Store
	if s3 := blob.NewS3FromEnv(); s3 != nil {
		blobStore = s3
		lg.Info("object storage enabled", "bucket", s3.Bucket)
	} else {
		blobStore = blob.NewMemory()
		lg.Warn("S3_BUCKET not set; SGF bodies are in-memory and will not survive a restart")
	}

	var pushSender notify.Sender = notify.NoopSender{}
	if fcm := notify.NewFCMFromEnv(); fcm != nil {
		pushSender = fcm
		lg.Info("push notifications enabled", "project", fcm.ProjectID)
	}

	srv := &http.Server{
		Addr: fmt.Sprintf(":%d", cfg.Port),
		Handler: api.New(api.Deps{
			Store:   st,
			Auth:    authSvc,
			Profile: profile.NewService(st.DB),
			Archive: archive.NewService(st.DB, blobStore),
			Notify:  notify.NewService(st.DB, pushSender),
			Log:     lg,
			Version: version,
		}).Routes(),
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
