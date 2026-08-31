// Package billing implements G1 (subscriptions and IAP), G2 (feature gating)
// and G3 (ad policy).
package billing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Tier is a subscription level.
type Tier string

// The four tiers.
const (
	TierFree     Tier = "free"
	TierGold     Tier = "gold"
	TierPlatinum Tier = "platinum"
	TierDiamond  Tier = "diamond"
)

// rank orders tiers so a comparison like "at least gold" is possible.
var rank = map[Tier]int{TierFree: 0, TierGold: 1, TierPlatinum: 2, TierDiamond: 3}

// AtLeast reports whether t includes everything u does.
func (t Tier) AtLeast(u Tier) bool { return rank[t] >= rank[u] }

// Feature is a gated capability.
type Feature string

// The gated features.
const (
	FeaturePuzzles        Feature = "puzzles"
	FeatureAIReview       Feature = "ai_review"
	FeatureOpenings       Feature = "opening_explorer"
	FeatureProLibrary     Feature = "pro_library"
	FeatureUnlimitedGames Feature = "unlimited_games"
	FeatureNoAds          Feature = "no_ads"
	FeatureDeepAnalysis   Feature = "deep_analysis"
)

// Window is how often a quota resets.
type Window string

// Quota reset windows.
const (
	WindowDaily  Window = "daily"
	WindowWeekly Window = "weekly"
	WindowNone   Window = "none" // boolean capability, no counter
)

// Policy is one row of the gating matrix.
type Policy struct {
	Feature Feature `json:"feature"`
	Window  Window  `json:"window"`
	// Limits per tier. A negative limit means unlimited; zero means denied.
	Limits map[Tier]int `json:"limits"`
}

// matrix is G2's centralised tier -> feature -> quota policy. Everything that
// gates a feature reads it from here, so there is exactly one place to change
// when pricing changes.
var matrix = []Policy{
	{FeaturePuzzles, WindowDaily, map[Tier]int{
		TierFree: 5, TierGold: 25, TierPlatinum: -1, TierDiamond: -1,
	}},
	{FeatureAIReview, WindowWeekly, map[Tier]int{
		TierFree: 3, TierGold: 20, TierPlatinum: -1, TierDiamond: -1,
	}},
	{FeatureDeepAnalysis, WindowWeekly, map[Tier]int{
		TierFree: 0, TierGold: 5, TierPlatinum: 30, TierDiamond: -1,
	}},
	{FeatureOpenings, WindowNone, map[Tier]int{
		TierFree: 0, TierGold: -1, TierPlatinum: -1, TierDiamond: -1,
	}},
	{FeatureProLibrary, WindowNone, map[Tier]int{
		TierFree: 0, TierGold: -1, TierPlatinum: -1, TierDiamond: -1,
	}},
	{FeatureUnlimitedGames, WindowNone, map[Tier]int{
		TierFree: -1, TierGold: -1, TierPlatinum: -1, TierDiamond: -1,
	}},
	{FeatureNoAds, WindowNone, map[Tier]int{
		TierFree: 0, TierGold: -1, TierPlatinum: -1, TierDiamond: -1,
	}},
}

var policyByFeature = func() map[Feature]Policy {
	m := make(map[Feature]Policy, len(matrix))
	for _, p := range matrix {
		m[p.Feature] = p
	}
	return m
}()

// ErrQuotaExceeded is returned when a user has used up their allowance.
var ErrQuotaExceeded = errors.New("billing: quota exceeded for this tier")

// ErrNotEntitled is returned when a tier does not include a feature at all.
var ErrNotEntitled = errors.New("billing: feature not included in this tier")

// Entitlement is what GET /entitlements reports per feature.
type Entitlement struct {
	Feature   Feature `json:"feature"`
	Allowed   bool    `json:"allowed"`
	Limit     int     `json:"limit"`
	Used      int     `json:"used"`
	Remaining int     `json:"remaining"`
	Window    Window  `json:"window"`
	Unlimited bool    `json:"unlimited"`
}

// Subscription is a user's billing state.
type Subscription struct {
	Tier      Tier       `json:"tier"`
	Status    string     `json:"status"`
	Platform  *string    `json:"platform,omitempty"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	AutoRenew bool       `json:"autoRenew"`
}

// Service resolves entitlements and records usage.
type Service struct{ db *pgxpool.Pool }

// NewService builds the billing service.
func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

// TierFor returns a user's effective tier.
//
// Access is decided by expires_at, not by status: a user who cancels
// auto-renew keeps what they already paid for until the period ends.
func (s *Service) TierFor(ctx context.Context, userID uuid.UUID) (Tier, error) {
	var tier Tier
	var expires *time.Time
	err := s.db.QueryRow(ctx,
		`SELECT tier, expires_at FROM subscriptions WHERE user_id = $1`,
		userID).Scan(&tier, &expires)
	if errors.Is(err, pgx.ErrNoRows) {
		return TierFree, nil
	}
	if err != nil {
		return TierFree, fmt.Errorf("resolve tier: %w", err)
	}
	if tier == TierFree {
		return TierFree, nil
	}
	if expires == nil || expires.Before(time.Now()) {
		return TierFree, nil
	}
	return tier, nil
}

// Subscription returns a user's full billing state.
func (s *Service) Subscription(ctx context.Context, userID uuid.UUID) (*Subscription, error) {
	var sub Subscription
	err := s.db.QueryRow(ctx, `
		SELECT tier, status, platform, expires_at, auto_renew
		FROM subscriptions WHERE user_id = $1`, userID,
	).Scan(&sub.Tier, &sub.Status, &sub.Platform, &sub.ExpiresAt, &sub.AutoRenew)
	if errors.Is(err, pgx.ErrNoRows) {
		return &Subscription{Tier: TierFree, Status: "none"}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get subscription: %w", err)
	}
	// Report the effective tier, not the stored one.
	if sub.ExpiresAt != nil && sub.ExpiresAt.Before(time.Now()) {
		sub.Tier = TierFree
	}
	return &sub, nil
}

// periodStart truncates now to the start of the feature's quota window.
func periodStart(w Window, now time.Time) time.Time {
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	switch w {
	case WindowWeekly:
		// ISO weeks start on Monday.
		offset := (int(day.Weekday()) + 6) % 7
		return day.AddDate(0, 0, -offset)
	default:
		return day
	}
}

// Entitlements reports the caller's whole matrix, which is what the client
// reads once at startup to decide what to show.
func (s *Service) Entitlements(ctx context.Context, userID uuid.UUID) ([]Entitlement, error) {
	tier, err := s.TierFor(ctx, userID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()

	usage := map[Feature]int{}
	rows, err := s.db.Query(ctx,
		`SELECT feature, period_start, used FROM entitlement_usage WHERE user_id = $1`, userID)
	if err != nil {
		return nil, fmt.Errorf("read usage: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var f Feature
		var start time.Time
		var used int
		if err := rows.Scan(&f, &start, &used); err != nil {
			return nil, err
		}
		p, ok := policyByFeature[f]
		if !ok {
			continue
		}
		// Ignore counters from an expired window.
		if start.UTC().Equal(periodStart(p.Window, now)) {
			usage[f] = used
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]Entitlement, 0, len(matrix))
	for _, p := range matrix {
		limit := p.Limits[tier]
		e := Entitlement{
			Feature: p.Feature, Limit: limit, Window: p.Window,
			Used: usage[p.Feature], Unlimited: limit < 0,
		}
		switch {
		case limit < 0:
			e.Allowed, e.Remaining = true, -1
		case limit == 0:
			e.Allowed, e.Remaining = false, 0
		default:
			e.Remaining = limit - e.Used
			if e.Remaining < 0 {
				e.Remaining = 0
			}
			e.Allowed = e.Remaining > 0
		}
		out = append(out, e)
	}
	return out, nil
}

// Consume records one use of a metered feature, refusing when the quota is
// spent. This is the server-side enforcement G2 requires: the client's own
// check is a UX affordance, not a control.
func (s *Service) Consume(ctx context.Context, userID uuid.UUID, feature Feature) error {
	p, ok := policyByFeature[feature]
	if !ok {
		return fmt.Errorf("unknown feature %q", feature)
	}
	tier, err := s.TierFor(ctx, userID)
	if err != nil {
		return err
	}
	limit := p.Limits[tier]
	if limit == 0 {
		return ErrNotEntitled
	}
	if limit < 0 || p.Window == WindowNone {
		return nil // unlimited, nothing to meter
	}

	start := periodStart(p.Window, time.Now().UTC())
	// The conditional insert-or-increment is atomic, so two concurrent
	// requests cannot both slip past the last unit of quota.
	tag, err := s.db.Exec(ctx, `
		INSERT INTO entitlement_usage (user_id, feature, period_start, used)
		VALUES ($1,$2,$3,1)
		ON CONFLICT (user_id, feature, period_start) DO UPDATE
		SET used = entitlement_usage.used + 1
		WHERE entitlement_usage.used < $4`,
		userID, feature, start, limit)
	if err != nil {
		return fmt.Errorf("consume quota: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrQuotaExceeded
	}
	return nil
}

// Allows reports whether a tier includes a feature at all, ignoring quota.
func Allows(tier Tier, feature Feature) bool {
	p, ok := policyByFeature[feature]
	if !ok {
		return false
	}
	return p.Limits[tier] != 0
}

// Matrix exposes the policy table, so an admin screen or the client can
// render the tier comparison without hardcoding it a second time.
func Matrix() []Policy { return matrix }
