// Package api wires HTTP routing, middleware and the C2 handlers.
package api

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/prathpatel/gogame-backend/internal/logging"
	"github.com/prathpatel/gogame-backend/internal/metrics"
)

type ctxKeyRequestID struct{}

// statusRecorder captures the response status for logs and metrics.
type statusRecorder struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if !r.wrote {
		r.status = code
		r.wrote = true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if !r.wrote {
		r.status = http.StatusOK
		r.wrote = true
	}
	return r.ResponseWriter.Write(b)
}

// withObservability assigns a request id, logs one structured line per
// request, records metrics, and converts panics into a 500 instead of
// tearing down the server.
func withObservability(lg *slog.Logger, route string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		reqID := r.Header.Get("X-Request-Id")
		if reqID == "" {
			reqID = uuid.NewString()
		}
		w.Header().Set("X-Request-Id", reqID)

		reqLog := lg.With("requestId", reqID, "route", route, "method", r.Method)
		ctx := logging.WithLogger(r.Context(), reqLog)
		ctx = context.WithValue(ctx, ctxKeyRequestID{}, reqID)

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			if p := recover(); p != nil {
				reqLog.Error("panic recovered", "panic", p)
				if !rec.wrote {
					writeError(rec, http.StatusInternalServerError, "internal_error", "internal error")
				}
			}
			elapsed := time.Since(start)
			metrics.HTTPRequests.WithLabelValues(route, r.Method, strconv.Itoa(rec.status)).Inc()
			metrics.HTTPDuration.WithLabelValues(route, r.Method).Observe(elapsed.Seconds())
			reqLog.Info("request",
				"status", rec.status,
				"durationMs", elapsed.Milliseconds(),
				"ip", clientIP(r),
			)
		}()

		next(rec, r.WithContext(ctx))
	}
}

// clientIP prefers the left-most X-Forwarded-For entry, which is what Fly.io
// and Cloudflare set, and falls back to the socket address.
func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("Fly-Client-IP"); fwd != "" {
		return fwd
	}
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		for i := 0; i < len(fwd); i++ {
			if fwd[i] == ',' {
				return trimSpace(fwd[:i])
			}
		}
		return trimSpace(fwd)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}
