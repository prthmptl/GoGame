package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/prathpatel/gogame-backend/internal/metrics"
)

// Sentinel errors. Handlers map these to status codes; everything else is 500.
var (
	ErrInvalidToken  = errors.New("auth: invalid token")
	ErrTokenExpired  = errors.New("auth: token expired")
	ErrTokenReplayed = errors.New("auth: refresh token replayed")
	ErrTokenRevoked  = errors.New("auth: token revoked")
	ErrIdentityTaken = errors.New("auth: identity already linked to another account")
)

// IDTokenVerifier validates a Google ID token and returns its payload. It is
// an interface so tests can run without reaching Google.
type IDTokenVerifier interface {
	Verify(ctx context.Context, idToken string) (*GooglePayload, error)
}

// GooglePayload is the subset of Google's ID token we consume.
type GooglePayload struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
	Picture       string
}

// User is a row of the users table.
type User struct {
	ID          uuid.UUID `json:"id"`
	DisplayName string    `json:"displayName"`
	AvatarURL   *string   `json:"avatarUrl,omitempty"`
	Country     *string   `json:"country,omitempty"`
	Rating      int       `json:"rating"`
	IsGuest     bool      `json:"isGuest"`
	CreatedAt   time.Time `json:"createdAt"`
}

// Service implements the C2 endpoints.
type Service struct {
	db         *pgxpool.Pool
	verifier   IDTokenVerifier
	jwtSecret  []byte
	issuer     string
	accessTTL  time.Duration
	refreshTTL time.Duration
	now        func() time.Time
}

// Options configures a Service.
type Options struct {
	DB         *pgxpool.Pool
	Verifier   IDTokenVerifier
	JWTSecret  []byte
	Issuer     string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
	// Now is injectable so tests can drive expiry deterministically.
	Now func() time.Time
}

// NewService builds the auth service.
func NewService(o Options) *Service {
	now := o.Now
	if now == nil {
		now = time.Now
	}
	return &Service{
		db:         o.DB,
		verifier:   o.Verifier,
		jwtSecret:  o.JWTSecret,
		issuer:     o.Issuer,
		accessTTL:  o.AccessTTL,
		refreshTTL: o.RefreshTTL,
		now:        now,
	}
}

// SessionMeta is the request context recorded against an issued refresh token.
type SessionMeta struct {
	UserAgent string
	IP        string
}

// Guest implements POST /auth/guest: create a throwaway user and a token pair.
func (s *Service) Guest(ctx context.Context, displayName string, meta SessionMeta) (*TokenPair, *User, error) {
	displayName = normalizeDisplayName(displayName)

	var u User
	err := s.db.QueryRow(ctx, `
		INSERT INTO users (display_name, is_guest)
		VALUES ($1, TRUE)
		RETURNING id, display_name, avatar_url, country, rating, is_guest, created_at`,
		displayName,
	).Scan(&u.ID, &u.DisplayName, &u.AvatarURL, &u.Country, &u.Rating, &u.IsGuest, &u.CreatedAt)
	if err != nil {
		metrics.AuthEvents.WithLabelValues("guest", "failed").Inc()
		return nil, nil, fmt.Errorf("create guest: %w", err)
	}

	pair, err := s.issuePair(ctx, u.ID, u.IsGuest, uuid.New(), meta)
	if err != nil {
		metrics.AuthEvents.WithLabelValues("guest", "failed").Inc()
		return nil, nil, err
	}
	metrics.AuthEvents.WithLabelValues("guest", "ok").Inc()
	return pair, &u, nil
}

// Google implements POST /auth/google. If currentUserID is non-nil the caller
// presented a valid access token, so the identity is linked to that existing
// account — this is the guest-upgrade path, and it preserves the user row
// (and therefore the rating and game archive) rather than creating a new one.
func (s *Service) Google(ctx context.Context, idToken string, currentUserID *uuid.UUID, meta SessionMeta) (*TokenPair, *User, error) {
	payload, err := s.verifier.Verify(ctx, idToken)
	if err != nil {
		metrics.AuthEvents.WithLabelValues("google", "failed").Inc()
		return nil, nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Existing identity wins: the same Google account always resolves to the
	// same user, regardless of which device or guest session presented it.
	var userID uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT user_id FROM auth_identities
		WHERE provider = 'google' AND provider_uid = $1`,
		payload.Subject,
	).Scan(&userID)

	switch {
	case err == nil:
		// Linking an identity that already belongs to a different account
		// would silently merge two histories; refuse instead.
		if currentUserID != nil && *currentUserID != userID {
			metrics.AuthEvents.WithLabelValues("google", "failed").Inc()
			return nil, nil, ErrIdentityTaken
		}
	case errors.Is(err, pgx.ErrNoRows):
		userID, err = s.linkNewIdentity(ctx, tx, payload, currentUserID)
		if err != nil {
			metrics.AuthEvents.WithLabelValues("google", "failed").Inc()
			return nil, nil, err
		}
	default:
		return nil, nil, fmt.Errorf("lookup identity: %w", err)
	}

	var u User
	if err := tx.QueryRow(ctx, `
		UPDATE users SET last_seen_at = now(), updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, display_name, avatar_url, country, rating, is_guest, created_at`,
		userID,
	).Scan(&u.ID, &u.DisplayName, &u.AvatarURL, &u.Country, &u.Rating, &u.IsGuest, &u.CreatedAt); err != nil {
		return nil, nil, fmt.Errorf("load user: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("commit: %w", err)
	}

	pair, err := s.issuePair(ctx, u.ID, u.IsGuest, uuid.New(), meta)
	if err != nil {
		metrics.AuthEvents.WithLabelValues("google", "failed").Inc()
		return nil, nil, err
	}
	metrics.AuthEvents.WithLabelValues("google", "ok").Inc()
	return pair, &u, nil
}

// linkNewIdentity attaches a never-seen Google subject either to the caller's
// existing (guest) account or to a freshly created user.
func (s *Service) linkNewIdentity(ctx context.Context, tx pgx.Tx, p *GooglePayload, currentUserID *uuid.UUID) (uuid.UUID, error) {
	var userID uuid.UUID
	if currentUserID != nil {
		// Upgrade in place: the guest keeps their id, rating and games.
		if err := tx.QueryRow(ctx, `
			UPDATE users
			SET is_guest = FALSE,
			    display_name = CASE WHEN is_guest THEN $2 ELSE display_name END,
			    avatar_url = COALESCE(avatar_url, NULLIF($3, '')),
			    updated_at = now()
			WHERE id = $1 AND deleted_at IS NULL
			RETURNING id`,
			*currentUserID, normalizeDisplayName(p.Name), p.Picture,
		).Scan(&userID); err != nil {
			return uuid.Nil, fmt.Errorf("upgrade guest: %w", err)
		}
	} else {
		if err := tx.QueryRow(ctx, `
			INSERT INTO users (display_name, avatar_url, is_guest)
			VALUES ($1, NULLIF($2, ''), FALSE)
			RETURNING id`,
			normalizeDisplayName(p.Name), p.Picture,
		).Scan(&userID); err != nil {
			return uuid.Nil, fmt.Errorf("create user: %w", err)
		}
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO auth_identities (user_id, provider, provider_uid, email, email_verified)
		VALUES ($1, 'google', $2, NULLIF($3, ''), $4)`,
		userID, p.Subject, p.Email, p.EmailVerified,
	); err != nil {
		return uuid.Nil, fmt.Errorf("link identity: %w", err)
	}
	return userID, nil
}

// Refresh implements POST /auth/refresh with rotation and theft detection.
//
// Every exchange marks the presented token rotated and issues a successor in
// the same family. Presenting an already-rotated token means two parties hold
// it, so the entire family is revoked and the caller must sign in again.
func (s *Service) Refresh(ctx context.Context, presented string, meta SessionMeta) (*TokenPair, error) {
	hash := hashRefreshToken(presented)
	now := s.now()

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var (
		tokenID   uuid.UUID
		userID    uuid.UUID
		familyID  uuid.UUID
		expiresAt time.Time
		rotatedAt *time.Time
		revokedAt *time.Time
		isGuest   bool
	)
	// FOR UPDATE serializes concurrent refreshes of the same token so two
	// racing clients cannot both rotate it and both look legitimate.
	err = tx.QueryRow(ctx, `
		SELECT rt.id, rt.user_id, rt.family_id, rt.expires_at, rt.rotated_at, rt.revoked_at, u.is_guest
		FROM refresh_tokens rt
		JOIN users u ON u.id = rt.user_id
		WHERE rt.token_hash = $1 AND u.deleted_at IS NULL
		FOR UPDATE OF rt`,
		hash,
	).Scan(&tokenID, &userID, &familyID, &expiresAt, &rotatedAt, &revokedAt, &isGuest)
	if errors.Is(err, pgx.ErrNoRows) {
		metrics.AuthEvents.WithLabelValues("refresh", "failed").Inc()
		return nil, ErrInvalidToken
	}
	if err != nil {
		return nil, fmt.Errorf("lookup refresh token: %w", err)
	}

	if rotatedAt != nil {
		// Replay. Revoke the whole family: we cannot tell whether the thief
		// or the legitimate client is calling, so both must re-authenticate.
		if _, err := tx.Exec(ctx, `
			UPDATE refresh_tokens
			SET revoked_at = now(), revoked_reason = 'reuse_detected'
			WHERE family_id = $1 AND revoked_at IS NULL`,
			familyID,
		); err != nil {
			return nil, fmt.Errorf("revoke family: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit revocation: %w", err)
		}
		metrics.RefreshReuseDetected.Inc()
		metrics.AuthEvents.WithLabelValues("refresh", "replayed").Inc()
		return nil, ErrTokenReplayed
	}
	if revokedAt != nil {
		metrics.AuthEvents.WithLabelValues("refresh", "revoked").Inc()
		return nil, ErrTokenRevoked
	}
	if now.After(expiresAt) {
		metrics.AuthEvents.WithLabelValues("refresh", "expired").Inc()
		return nil, ErrTokenExpired
	}

	if _, err := tx.Exec(ctx,
		`UPDATE refresh_tokens SET rotated_at = now() WHERE id = $1`, tokenID); err != nil {
		return nil, fmt.Errorf("mark rotated: %w", err)
	}

	plaintext, newHash, err := newRefreshToken()
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO refresh_tokens (user_id, family_id, token_hash, expires_at, user_agent, ip)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, '')::inet)`,
		userID, familyID, newHash, now.Add(s.refreshTTL), meta.UserAgent, meta.IP,
	); err != nil {
		return nil, fmt.Errorf("store rotated token: %w", err)
	}

	access, accessExp, err := s.issueAccessToken(userID, isGuest, now)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	metrics.AuthEvents.WithLabelValues("refresh", "ok").Inc()
	return &TokenPair{
		AccessToken:  access,
		RefreshToken: plaintext,
		ExpiresAt:    accessExp,
		TokenType:    "Bearer",
	}, nil
}

// issuePair mints an access token plus a first-generation refresh token.
func (s *Service) issuePair(ctx context.Context, userID uuid.UUID, isGuest bool, familyID uuid.UUID, meta SessionMeta) (*TokenPair, error) {
	now := s.now()
	access, accessExp, err := s.issueAccessToken(userID, isGuest, now)
	if err != nil {
		return nil, err
	}
	plaintext, hash, err := newRefreshToken()
	if err != nil {
		return nil, err
	}
	if _, err := s.db.Exec(ctx, `
		INSERT INTO refresh_tokens (user_id, family_id, token_hash, expires_at, user_agent, ip)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, '')::inet)`,
		userID, familyID, hash, now.Add(s.refreshTTL), meta.UserAgent, meta.IP,
	); err != nil {
		return nil, fmt.Errorf("store refresh token: %w", err)
	}
	return &TokenPair{
		AccessToken:  access,
		RefreshToken: plaintext,
		ExpiresAt:    accessExp,
		TokenType:    "Bearer",
	}, nil
}

// normalizeDisplayName trims and bounds a name, falling back to "Player" to
// match the client's ProfileStore default.
func normalizeDisplayName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "Player"
	}
	if len([]rune(name)) > 32 {
		name = string([]rune(name)[:32])
	}
	return name
}
