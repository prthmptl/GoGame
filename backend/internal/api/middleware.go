// Package api wires HTTP routing, middleware and the C2 handlers.
package api

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
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

func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }
func (r *statusRecorder) WriteHeader(code int) {
	if r.wrote {
		return
	}
	r.status, r.wrote = code, true
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
		deadline := start.Add(15 * time.Second)
		_ = http.NewResponseController(w).SetReadDeadline(deadline)
		_ = http.NewResponseController(w).SetWriteDeadline(deadline)
		defer http.NewResponseController(w).SetReadDeadline(time.Time{})
		defer http.NewResponseController(w).SetWriteDeadline(time.Time{})
		requestCtx, cancel := context.WithDeadline(r.Context(), deadline)
		defer cancel()
		r = r.WithContext(requestCtx)
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

// Forwarded addresses are trusted only when the immediate peer is in an
// explicitly configured proxy network. Walk right-to-left to ignore spoofed
// addresses prepended by a caller.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	var trusted []*net.IPNet
	for _, raw := range strings.Split(os.Getenv("TRUSTED_PROXY_CIDRS"), ",") {
		_, network, err := net.ParseCIDR(strings.TrimSpace(raw))
		if err == nil {
			trusted = append(trusted, network)
		}
	}
	isTrusted := func(ip net.IP) bool {
		for _, network := range trusted {
			if network.Contains(ip) {
				return true
			}
		}
		return false
	}
	chain := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(chain) - 1; i >= 0 && isTrusted(ip); i-- {
		next := net.ParseIP(strings.TrimSpace(chain[i]))
		if next == nil {
			break
		}
		ip = next
	}
	if ip != nil {
		return ip.String()
	}
	return host
}

func (s *Server) limitAuth(limit int, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := "auth:rate:" + r.URL.Path + ":" + clientIP(r)
		n, err := s.store.Redis.Eval(r.Context(), `local n = redis.call("INCR", KEYS[1])
            if n == 1 then redis.call("EXPIRE", KEYS[1], 60) end return n`, []string{key}).Int()
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "unavailable", "authentication temporarily unavailable")
			return
		}
		if n > limit {
			w.Header().Set("Retry-After", "60")
			writeError(w, http.StatusTooManyRequests, "rate_limited", "too many requests")
			return
		}
		next(w, r)
	}
}
