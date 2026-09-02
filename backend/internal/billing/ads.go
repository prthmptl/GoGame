package billing

import (
	"context"

	"github.com/google/uuid"
)

// AdPolicy tells the client what to show. G3: ads on the free tier only.
type AdPolicy struct {
	// ShowAds is false for any paying tier.
	ShowAds bool `json:"showAds"`
	// ShowBanner puts a banner on the home page.
	ShowBanner bool `json:"showBanner"`
	// InterstitialEveryNGames is how often a full-screen ad may run. Zero
	// means never.
	InterstitialEveryNGames int `json:"interstitialEveryNGames"`
	// MaxInterstitialsPerSession caps how many a single session may show,
	// so a long play session does not become mostly advertising.
	MaxInterstitialsPerSession int `json:"maxInterstitialsPerSession"`
	// MinSecondsBetweenInterstitials stops two ads landing back to back when
	// someone plays several very short games.
	MinSecondsBetweenInterstitials int `json:"minSecondsBetweenInterstitials"`
}

// AdPolicyFor returns the ad configuration for a user's tier.
func (s *Service) AdPolicyFor(ctx context.Context, userID uuid.UUID) (*AdPolicy, error) {
	tier, err := s.TierFor(ctx, userID)
	if err != nil {
		return nil, err
	}
	// Any paid tier includes FeatureNoAds.
	if Allows(tier, FeatureNoAds) {
		return &AdPolicy{ShowAds: false}, nil
	}
	return &AdPolicy{
		ShowAds:                        true,
		ShowBanner:                     true,
		InterstitialEveryNGames:        3,
		MaxInterstitialsPerSession:     4,
		MinSecondsBetweenInterstitials: 180,
	}, nil
}
