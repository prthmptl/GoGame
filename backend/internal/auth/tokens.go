// Package auth implements C2: guest and Google sign-in, and refresh rotation.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// TokenPair is what every auth endpoint returns to the client.
type TokenPair struct {
	AccessToken  string    `json:"accessToken"`
	RefreshToken string    `json:"refreshToken"`
	ExpiresAt    time.Time `json:"expiresAt"`
	TokenType    string    `json:"tokenType"`
}

// Claims are the access-token claims. `IsGuest` lets handlers gate
// guest-restricted actions (ranked play, chat) without a database read.
type Claims struct {
	jwt.RegisteredClaims
	IsGuest bool `json:"guest"`
}

// UserID returns the authenticated user's id.
func (c *Claims) UserID() (uuid.UUID, error) {
	return uuid.Parse(c.Subject)
}

// issueAccessToken mints a short-lived HS256 JWT for userID.
func (s *Service) issueAccessToken(userID uuid.UUID, isGuest bool, now time.Time) (string, time.Time, error) {
	expiresAt := now.Add(s.accessTTL)
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			Issuer:    s.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			ID:        uuid.NewString(),
		},
		IsGuest: isGuest,
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtSecret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}
	return signed, expiresAt, nil
}

// ParseAccessToken validates signature, expiry and issuer, and returns the
// claims. It rejects any algorithm other than HS256 so a token cannot be
// downgraded to "none" or swapped to an asymmetric algorithm.
func (s *Service) ParseAccessToken(raw string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
		}
		return s.jwtSecret, nil
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(s.issuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	if id, err := claims.UserID(); err != nil || id == uuid.Nil {
		return nil, fmt.Errorf("%w: subject is not a uuid", ErrInvalidToken)
	}
	return claims, nil
}

// newRefreshToken returns a fresh opaque token and its storage hash. The
// plaintext is returned to the caller once and never persisted.
func newRefreshToken() (plaintext string, hash []byte, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, fmt.Errorf("generate refresh token: %w", err)
	}
	plaintext = base64.RawURLEncoding.EncodeToString(buf)
	sum := sha256.Sum256([]byte(plaintext))
	return plaintext, sum[:], nil
}

// hashRefreshToken maps a presented token to its stored hash. SHA-256 is the
// right primitive here (not bcrypt): the token is 256 bits of entropy we
// generated, so there is nothing to brute-force, and lookups stay indexable.
func hashRefreshToken(plaintext string) []byte {
	sum := sha256.Sum256([]byte(plaintext))
	return sum[:]
}
