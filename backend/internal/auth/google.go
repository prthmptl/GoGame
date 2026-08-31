package auth

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/api/idtoken"
)

// googleVerifier validates ID tokens against Google's published keys.
// google.golang.org/api/idtoken handles JWKS fetching, caching and rotation.
type googleVerifier struct {
	audiences []string
}

// NewGoogleVerifier accepts tokens minted for any of the given OAuth client
// IDs — Android, iOS and web each have their own.
func NewGoogleVerifier(clientIDs []string) IDTokenVerifier {
	return &googleVerifier{audiences: clientIDs}
}

func (g *googleVerifier) Verify(ctx context.Context, raw string) (*GooglePayload, error) {
	if len(g.audiences) == 0 {
		return nil, errors.New("no Google client IDs configured")
	}
	// Validate once per configured audience; a token is valid if any accepts.
	var lastErr error
	for _, aud := range g.audiences {
		payload, err := idtoken.Validate(ctx, raw, aud)
		if err != nil {
			lastErr = err
			continue
		}
		// Reject anything not actually minted by Google's account issuer.
		if payload.Issuer != "https://accounts.google.com" && payload.Issuer != "accounts.google.com" {
			lastErr = fmt.Errorf("unexpected issuer %q", payload.Issuer)
			continue
		}
		out := &GooglePayload{Subject: payload.Subject}
		if v, ok := payload.Claims["email"].(string); ok {
			out.Email = v
		}
		if v, ok := payload.Claims["email_verified"].(bool); ok {
			out.EmailVerified = v
		}
		if v, ok := payload.Claims["name"].(string); ok {
			out.Name = v
		}
		if v, ok := payload.Claims["picture"].(string); ok {
			out.Picture = v
		}
		return out, nil
	}
	return nil, fmt.Errorf("validate google id token: %w", lastErr)
}
