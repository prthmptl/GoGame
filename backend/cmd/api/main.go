// Command api serves the GoGame HTTP API (C1 skeleton, C2 auth).
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"

	"github.com/prathpatel/gogame-backend/internal/anticheat"
	"github.com/prathpatel/gogame-backend/internal/api"
	"github.com/prathpatel/gogame-backend/internal/archive"
	"github.com/prathpatel/gogame-backend/internal/auth"
	"github.com/prathpatel/gogame-backend/internal/billing"
	"github.com/prathpatel/gogame-backend/internal/blob"
	"github.com/prathpatel/gogame-backend/internal/chat"
	"github.com/prathpatel/gogame-backend/internal/clubs"
	"github.com/prathpatel/gogame-backend/internal/coaching"
	"github.com/prathpatel/gogame-backend/internal/config"
	"github.com/prathpatel/gogame-backend/internal/correspondence"
	"github.com/prathpatel/gogame-backend/internal/game"
	"github.com/prathpatel/gogame-backend/internal/logging"
	"github.com/prathpatel/gogame-backend/internal/matchmaking"
	"github.com/prathpatel/gogame-backend/internal/notify"
	"github.com/prathpatel/gogame-backend/internal/openings"
	"github.com/prathpatel/gogame-backend/internal/profile"
	"github.com/prathpatel/gogame-backend/internal/progames"
	"github.com/prathpatel/gogame-backend/internal/rating"
	"github.com/prathpatel/gogame-backend/internal/rooms"
	"github.com/prathpatel/gogame-backend/internal/social"
	"github.com/prathpatel/gogame-backend/internal/store"
	"github.com/prathpatel/gogame-backend/internal/tournament"
	"github.com/prathpatel/gogame-backend/internal/ws"
)

// version is stamped at build time: -ldflags "-X main.version=$(git rev-parse --short HEAD)".
var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

// allowedOrigins lists the browser origins permitted to open a WebSocket.
// The mobile app sends no Origin header, so it is unaffected.
func allowedOrigins() []string {
	if raw := os.Getenv("ALLOWED_WS_ORIGINS"); raw != "" {
		return strings.Split(raw, ",")
	}
	return nil
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

	// One hub per process. The instance id is what Redis records as the owner
	// of each live game, so it must be unique per running API instance.
	instanceID := os.Getenv("FLY_MACHINE_ID")
	if instanceID == "" {
		instanceID = uuid.NewString()
	}
	ratingSvc := rating.NewService(st.DB)
	cheatSvc := anticheat.NewService(st.DB)
	archiveSvc := archive.NewService(st.DB, blobStore)
	notifySvc := notify.NewService(st.DB, pushSender)

	hub := game.NewHub(st.DB, st.Redis, instanceID,
		api.GameHooks(st.DB, ratingSvc, cheatSvc, archiveSvc, notifySvc, lg))
	matcher := matchmaking.NewService(st.Redis)
	matcher.Pair = api.PairPlayers(hub)
	lg.Info("game hub ready", "instance", instanceID)

	srv := &http.Server{
		Addr: fmt.Sprintf(":%d", cfg.Port),
		Handler: api.New(api.Deps{
			Store:          st,
			Auth:           authSvc,
			Profile:        profile.NewService(st.DB),
			Archive:        archiveSvc,
			Notify:         notifySvc,
			Rating:         ratingSvc,
			Anticheat:      cheatSvc,
			Correspondence: correspondence.NewService(st.DB),
			Tournament:     tournament.NewService(st.DB),
			Clubs:          clubs.NewService(st.DB),
			Social:         social.NewService(st.DB),
			Openings:       openings.NewService(st.DB),
			ProGames:       progames.NewService(st.DB, blobStore),
			// Payments are not executed without a processor account; the
			// no-op processor lets bookings work while money does not move.
			Coaching: coaching.NewService(st.DB, coaching.NoopProcessor{}),
			Billing:  billing.NewService(st.DB),
			// Receipt validation fails closed until store credentials exist,
			// so an unverified receipt can never grant an entitlement.
			IAP:         billing.NewIAPService(st.DB, billing.UnconfiguredValidator{}),
			Hub:         hub,
			Matchmaking: matcher,
			Rooms:       rooms.NewService(st.DB),
			Chat:        chat.NewService(st.DB, st.Redis),
			WS:          ws.NewServer(authSvc, hub, lg, allowedOrigins()),
			Log:         lg,
			Version:     version,
		}).Routes(),
		ReadHeaderTimeout: 10 * time.Second,
		// No ReadTimeout or WriteTimeout: they would kill WebSocket
		// connections mid-game. Per-request deadlines are applied by the
		// handlers, and the WebSocket layer enforces its own idle timeout.
		IdleTimeout: 120 * time.Second,
	}

	// D3's timeout watcher: charge every live game so a player who abandons
	// mid-game actually loses on time.
	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				hub.TickAll()
			}
		}
	}()

	// D4's pairing loop runs here rather than in the worker: creating a game
	// means starting its session, and sessions live in this process's hub.
	go func() {
		t := time.NewTicker(2 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if _, err := matcher.PairOnce(ctx); err != nil {
					lg.Error("matchmaking sweep failed", "error", err)
				}
			}
		}
	}()

	// Drop finished sessions so a long-lived instance does not accumulate them.
	go func() {
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if n := hub.Reap(ctx); n > 0 {
					lg.Info("reaped finished games", "games", n, "live", hub.LiveCount())
				}
			}
		}
	}()

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
