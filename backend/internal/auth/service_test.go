package auth_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/prathpatel/gogame-backend/internal/auth"
	"github.com/prathpatel/gogame-backend/internal/store"
)

// stubVerifier returns a canned payload so tests never call Google.
type stubVerifier struct {
	payload *auth.GooglePayload
	err     error
}

func (s *stubVerifier) Verify(context.Context, string) (*auth.GooglePayload, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.payload, nil
}

var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		// Without a database these tests cannot be meaningful; skip loudly
		// rather than passing vacuously.
		os.Stderr.WriteString("TEST_DATABASE_URL not set; skipping auth integration tests\n")
		os.Exit(0)
	}
	ctx := context.Background()
	st, err := store.Open(ctx, dsn, os.Getenv("TEST_REDIS_URL"))
	if err != nil {
		panic(err)
	}
	if err := st.Migrate(ctx); err != nil {
		panic(err)
	}
	testPool = st.DB
	code := m.Run()
	st.Close()
	os.Exit(code)
}

func newService(t *testing.T, v auth.IDTokenVerifier, now func() time.Time) *auth.Service {
	t.Helper()
	return auth.NewService(auth.Options{
		DB:         testPool,
		Verifier:   v,
		JWTSecret:  []byte("test-secret-at-least-32-bytes-long!!"),
		Issuer:     "gogame-test",
		AccessTTL:  15 * time.Minute,
		RefreshTTL: 24 * time.Hour,
		Now:        now,
	})
}

func TestGuestIssuesUsableTokens(t *testing.T) {
	svc := newService(t, &stubVerifier{}, nil)
	pair, user, err := svc.Guest(context.Background(), "  Prath  ", auth.SessionMeta{})
	if err != nil {
		t.Fatalf("Guest: %v", err)
	}
	if !user.IsGuest {
		t.Error("expected IsGuest true")
	}
	if user.DisplayName != "Prath" {
		t.Errorf("display name = %q, want %q (trimmed)", user.DisplayName, "Prath")
	}
	if user.Rating != 1000 {
		t.Errorf("rating = %d, want 1000 to match the client default", user.Rating)
	}
	claims, err := svc.ParseAccessToken(pair.AccessToken)
	if err != nil {
		t.Fatalf("ParseAccessToken: %v", err)
	}
	got, err := claims.UserID()
	if err != nil || got != user.ID {
		t.Errorf("token subject = %v, want %v", got, user.ID)
	}
	if !claims.IsGuest {
		t.Error("expected guest claim on a guest token")
	}
}

func TestGuestDefaultsDisplayName(t *testing.T) {
	svc := newService(t, &stubVerifier{}, nil)
	_, user, err := svc.Guest(context.Background(), "   ", auth.SessionMeta{})
	if err != nil {
		t.Fatalf("Guest: %v", err)
	}
	if user.DisplayName != "Player" {
		t.Errorf("display name = %q, want the ProfileStore default %q", user.DisplayName, "Player")
	}
}

func TestRefreshRotatesAndInvalidatesOldToken(t *testing.T) {
	svc := newService(t, &stubVerifier{}, nil)
	ctx := context.Background()
	first, _, err := svc.Guest(ctx, "rotator", auth.SessionMeta{})
	if err != nil {
		t.Fatalf("Guest: %v", err)
	}

	second, err := svc.Refresh(ctx, first.RefreshToken, auth.SessionMeta{})
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if second.RefreshToken == first.RefreshToken {
		t.Fatal("refresh token was not rotated")
	}
	// The successor must work.
	if _, err := svc.Refresh(ctx, second.RefreshToken, auth.SessionMeta{}); err != nil {
		t.Fatalf("rotated token should be usable: %v", err)
	}
}

func TestRefreshReuseRevokesEntireFamily(t *testing.T) {
	svc := newService(t, &stubVerifier{}, nil)
	ctx := context.Background()
	first, _, err := svc.Guest(ctx, "victim", auth.SessionMeta{})
	if err != nil {
		t.Fatalf("Guest: %v", err)
	}
	second, err := svc.Refresh(ctx, first.RefreshToken, auth.SessionMeta{})
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	// Replaying the first token is the theft signal.
	if _, err := svc.Refresh(ctx, first.RefreshToken, auth.SessionMeta{}); !errors.Is(err, auth.ErrTokenReplayed) {
		t.Fatalf("replay error = %v, want ErrTokenReplayed", err)
	}
	// And it must take the live successor down with it, or the thief keeps
	// their session while only the victim is logged out.
	if _, err := svc.Refresh(ctx, second.RefreshToken, auth.SessionMeta{}); !errors.Is(err, auth.ErrTokenRevoked) {
		t.Fatalf("successor error = %v, want ErrTokenRevoked", err)
	}
}

func TestRefreshRejectsExpiredToken(t *testing.T) {
	base := time.Now()
	clock := func() time.Time { return base }
	svc := newService(t, &stubVerifier{}, func() time.Time { return clock() })
	ctx := context.Background()
	pair, _, err := svc.Guest(ctx, "expiring", auth.SessionMeta{})
	if err != nil {
		t.Fatalf("Guest: %v", err)
	}
	clock = func() time.Time { return base.Add(48 * time.Hour) } // past RefreshTTL
	if _, err := svc.Refresh(ctx, pair.RefreshToken, auth.SessionMeta{}); !errors.Is(err, auth.ErrTokenExpired) {
		t.Fatalf("error = %v, want ErrTokenExpired", err)
	}
}

func TestRefreshRejectsUnknownToken(t *testing.T) {
	svc := newService(t, &stubVerifier{}, nil)
	if _, err := svc.Refresh(context.Background(), "not-a-real-token", auth.SessionMeta{}); !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("error = %v, want ErrInvalidToken", err)
	}
}

func TestGoogleSignInCreatesThenReusesOneUser(t *testing.T) {
	sub := "google-" + uuid.NewString()
	svc := newService(t, &stubVerifier{payload: &auth.GooglePayload{
		Subject: sub, Email: "p@example.com", EmailVerified: true, Name: "Prath",
	}}, nil)
	ctx := context.Background()

	_, u1, err := svc.Google(ctx, "tok", nil, auth.SessionMeta{})
	if err != nil {
		t.Fatalf("first sign-in: %v", err)
	}
	if u1.IsGuest {
		t.Error("a Google user must not be a guest")
	}
	_, u2, err := svc.Google(ctx, "tok", nil, auth.SessionMeta{})
	if err != nil {
		t.Fatalf("second sign-in: %v", err)
	}
	if u1.ID != u2.ID {
		t.Errorf("same Google subject produced two users: %v and %v", u1.ID, u2.ID)
	}
}

func TestGoogleUpgradesGuestInPlace(t *testing.T) {
	sub := "google-" + uuid.NewString()
	svc := newService(t, &stubVerifier{payload: &auth.GooglePayload{
		Subject: sub, Email: "up@example.com", EmailVerified: true, Name: "Upgraded",
	}}, nil)
	ctx := context.Background()

	_, guest, err := svc.Guest(ctx, "Temp", auth.SessionMeta{})
	if err != nil {
		t.Fatalf("Guest: %v", err)
	}
	// Give the guest a rating so we can prove progress survives the upgrade.
	if _, err := testPool.Exec(ctx, `UPDATE users SET rating = 1234 WHERE id = $1`, guest.ID); err != nil {
		t.Fatalf("seed rating: %v", err)
	}

	_, upgraded, err := svc.Google(ctx, "tok", &guest.ID, auth.SessionMeta{})
	if err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	if upgraded.ID != guest.ID {
		t.Fatalf("upgrade created a new user %v, want in-place upgrade of %v", upgraded.ID, guest.ID)
	}
	if upgraded.IsGuest {
		t.Error("user should no longer be a guest")
	}
	if upgraded.Rating != 1234 {
		t.Errorf("rating = %d, want 1234 preserved across the upgrade", upgraded.Rating)
	}
}

func TestGoogleRefusesIdentityOwnedByAnotherUser(t *testing.T) {
	sub := "google-" + uuid.NewString()
	svc := newService(t, &stubVerifier{payload: &auth.GooglePayload{
		Subject: sub, Email: "owner@example.com", EmailVerified: true, Name: "Owner",
	}}, nil)
	ctx := context.Background()

	if _, _, err := svc.Google(ctx, "tok", nil, auth.SessionMeta{}); err != nil {
		t.Fatalf("establish owner: %v", err)
	}
	_, other, err := svc.Guest(ctx, "Other", auth.SessionMeta{})
	if err != nil {
		t.Fatalf("Guest: %v", err)
	}
	if _, _, err := svc.Google(ctx, "tok", &other.ID, auth.SessionMeta{}); !errors.Is(err, auth.ErrIdentityTaken) {
		t.Fatalf("error = %v, want ErrIdentityTaken", err)
	}
}

func TestAccessTokenRejectsWrongSecretAndAlgNone(t *testing.T) {
	svc := newService(t, &stubVerifier{}, nil)
	pair, _, err := svc.Guest(context.Background(), "sig", auth.SessionMeta{})
	if err != nil {
		t.Fatalf("Guest: %v", err)
	}

	other := auth.NewService(auth.Options{
		DB: testPool, JWTSecret: []byte("a-completely-different-secret-key!!!"),
		Issuer: "gogame-test", AccessTTL: time.Minute, RefreshTTL: time.Hour,
	})
	if _, err := other.ParseAccessToken(pair.AccessToken); err == nil {
		t.Error("token verified under the wrong secret")
	}
	// alg=none must never be accepted.
	if _, err := svc.ParseAccessToken("eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJzdWIiOiJhIn0."); err == nil {
		t.Error("alg=none token was accepted")
	}
}
