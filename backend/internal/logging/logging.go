// Package logging provides the structured logger required by C1.
package logging

import (
	"context"
	"log/slog"
	"os"
)

type ctxKey struct{}

// New builds a JSON structured logger. Dev uses text at debug level so local
// output stays readable; staging and production emit JSON for log ingestion.
func New(env string) *slog.Logger {
	if env == "dev" {
		return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}

// WithLogger stores lg on the context so handlers can log with request scope.
func WithLogger(ctx context.Context, lg *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, lg)
}

// FromContext returns the request-scoped logger, or the default logger.
func FromContext(ctx context.Context) *slog.Logger {
	if lg, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok {
		return lg
	}
	return slog.Default()
}
