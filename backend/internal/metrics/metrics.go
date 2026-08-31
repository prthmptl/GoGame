// Package metrics holds the Prometheus collectors required by C1.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTPRequests counts requests by route template, method and status. The
	// route template (not the raw path) keeps cardinality bounded.
	HTTPRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "gogame_http_requests_total",
		Help: "Total HTTP requests by route, method and status class.",
	}, []string{"route", "method", "status"})

	// HTTPDuration measures request latency in seconds.
	HTTPDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "gogame_http_request_duration_seconds",
		Help:    "HTTP request latency in seconds by route and method.",
		Buckets: prometheus.DefBuckets,
	}, []string{"route", "method"})

	// AuthEvents counts auth outcomes: guest/google/refresh x ok/failed.
	AuthEvents = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "gogame_auth_events_total",
		Help: "Authentication events by kind and outcome.",
	}, []string{"kind", "outcome"})

	// RefreshReuseDetected fires when a already-rotated refresh token is
	// replayed, which means the token leaked. Alert on any increase.
	RefreshReuseDetected = promauto.NewCounter(prometheus.CounterOpts{
		Name: "gogame_auth_refresh_reuse_total",
		Help: "Refresh tokens replayed after rotation (token theft signal).",
	})
)
