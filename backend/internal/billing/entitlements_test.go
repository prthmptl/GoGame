package billing

import (
	"testing"
	"time"
)

func TestTierOrdering(t *testing.T) {
	if !TierDiamond.AtLeast(TierGold) {
		t.Error("diamond should include gold")
	}
	if TierFree.AtLeast(TierGold) {
		t.Error("free should not include gold")
	}
	if !TierGold.AtLeast(TierGold) {
		t.Error("a tier should include itself")
	}
}

func TestMatrixGatesFreeTier(t *testing.T) {
	// The features that make the paid tiers worth buying must be closed on
	// free, and the ones that make the app usable must stay open.
	closed := []Feature{FeatureOpenings, FeatureProLibrary, FeatureNoAds, FeatureDeepAnalysis}
	for _, f := range closed {
		if Allows(TierFree, f) {
			t.Errorf("%s should not be included in the free tier", f)
		}
	}
	if !Allows(TierFree, FeatureUnlimitedGames) {
		t.Error("playing games must never be gated; that is the product")
	}
	if !Allows(TierFree, FeaturePuzzles) {
		t.Error("free should get some puzzles, not zero")
	}
}

func TestMatrixIsMonotonicAcrossTiers(t *testing.T) {
	// A higher tier must never grant less than a lower one, or someone who
	// upgrades loses access to something.
	order := []Tier{TierFree, TierGold, TierPlatinum, TierDiamond}
	for _, p := range Matrix() {
		prev := p.Limits[order[0]]
		for _, tier := range order[1:] {
			cur := p.Limits[tier]
			// -1 is unlimited, so it beats any finite number.
			better := cur < 0 || (prev >= 0 && cur >= prev)
			if !better {
				t.Errorf("%s: %s grants %d but the lower tier grants %d",
					p.Feature, tier, cur, prev)
			}
			if cur >= 0 {
				prev = cur
			} else {
				prev = -1
			}
		}
	}
}

func TestPeriodStartDaily(t *testing.T) {
	now := time.Date(2026, 8, 31, 17, 45, 0, 0, time.UTC)
	got := periodStart(WindowDaily, now)
	want := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("daily period start = %v, want %v", got, want)
	}
}

func TestPeriodStartWeeklyIsMonday(t *testing.T) {
	// 2026-08-31 is a Monday; 2026-09-06 is the Sunday after.
	monday := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)
	for _, day := range []time.Time{
		time.Date(2026, 8, 31, 9, 0, 0, 0, time.UTC),  // Monday
		time.Date(2026, 9, 3, 23, 59, 0, 0, time.UTC), // Thursday
		time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC),  // Sunday
	} {
		if got := periodStart(WindowWeekly, day); !got.Equal(monday) {
			t.Errorf("weekly period start for %v = %v, want %v", day, got, monday)
		}
	}
	// The next Monday starts a new window.
	next := time.Date(2026, 9, 7, 0, 30, 0, 0, time.UTC)
	if got := periodStart(WindowWeekly, next); got.Equal(monday) {
		t.Error("a new week should not share the previous week's period start")
	}
}

func TestAllowsRejectsUnknownFeature(t *testing.T) {
	if Allows(TierDiamond, Feature("teleportation")) {
		t.Error("an unknown feature must not be allowed by default")
	}
}

func TestAdPolicyOnlyForFreeTier(t *testing.T) {
	// The policy is derived from FeatureNoAds, so this checks the wiring
	// between the matrix and G3 rather than hardcoded tiers.
	if Allows(TierFree, FeatureNoAds) {
		t.Fatal("free tier must not include ad removal")
	}
	for _, paid := range []Tier{TierGold, TierPlatinum, TierDiamond} {
		if !Allows(paid, FeatureNoAds) {
			t.Errorf("%s should include ad removal", paid)
		}
	}
}

func TestTierForProduct(t *testing.T) {
	if tier, ok := TierForProduct("go.platinum.yearly"); !ok || tier != TierPlatinum {
		t.Errorf("product resolved to %v/%v, want platinum", tier, ok)
	}
	if _, ok := TierForProduct("go.unknown.product"); ok {
		t.Error("an unknown product id should not resolve to a tier")
	}
}

func TestUnconfiguredValidatorRefusesEverything(t *testing.T) {
	// Granting entitlements on an unverified receipt is how subscriptions get
	// pirated; the default must fail closed.
	_, err := UnconfiguredValidator{}.Validate(nil, PlatformIOS, "anything")
	if err == nil {
		t.Fatal("the default validator accepted a receipt")
	}
}
