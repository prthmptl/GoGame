package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Platform identifies a store.
type Platform string

// Supported stores.
const (
	PlatformIOS     Platform = "ios"
	PlatformAndroid Platform = "android"
	PlatformPromo   Platform = "promo"
)

// Receipt is a normalised purchase, whatever store it came from.
type Receipt struct {
	Platform Platform
	// OriginalTransactionID is the store's stable subscription identifier;
	// renewals share it, which is what lets a renewal update the same row.
	OriginalTransactionID string
	ProductID             string
	ExpiresAt             time.Time
	AutoRenew             bool
	// Revoked is set for a refund or a chargeback.
	Revoked bool
}

// ReceiptValidator verifies a purchase with the store.
//
// Server-side validation is mandatory: a client-supplied receipt is
// attacker-controlled, and trusting it is how apps get their subscriptions
// pirated. The App Store and Play implementations require store credentials,
// which is why the shipped default refuses everything rather than pretending.
type ReceiptValidator interface {
	Validate(ctx context.Context, platform Platform, token string) (*Receipt, error)
}

// ErrValidatorNotConfigured is returned when no store credentials are set.
var ErrValidatorNotConfigured = errors.New("billing: receipt validation is not configured")

// UnconfiguredValidator rejects every receipt. It is the default so that an
// environment without store credentials cannot accidentally grant entitlements
// on an unverified receipt.
type UnconfiguredValidator struct{}

// Validate always fails.
func (UnconfiguredValidator) Validate(context.Context, Platform, string) (*Receipt, error) {
	return nil, ErrValidatorNotConfigured
}

// productTier maps a store product id to the tier it grants. Product ids are
// configured in App Store Connect and the Play Console; these are the
// expected identifiers.
var productTier = map[string]Tier{
	"go.gold.monthly":     TierGold,
	"go.gold.yearly":      TierGold,
	"go.platinum.monthly": TierPlatinum,
	"go.platinum.yearly":  TierPlatinum,
	"go.diamond.monthly":  TierDiamond,
	"go.diamond.yearly":   TierDiamond,
}

// TierForProduct resolves a product id.
func TierForProduct(productID string) (Tier, bool) {
	t, ok := productTier[productID]
	return t, ok
}

// IAPService handles purchases and store webhooks.
type IAPService struct {
	db        *pgxpool.Pool
	validator ReceiptValidator
}

// NewIAPService builds the purchase service.
func NewIAPService(db *pgxpool.Pool, v ReceiptValidator) *IAPService {
	if v == nil {
		v = UnconfiguredValidator{}
	}
	return &IAPService{db: db, validator: v}
}

// Redeem validates a client-supplied receipt and grants the entitlement.
func (s *IAPService) Redeem(ctx context.Context, userID uuid.UUID, platform Platform, token string) (*Subscription, error) {
	receipt, err := s.validator.Validate(ctx, platform, token)
	if err != nil {
		return nil, fmt.Errorf("validate receipt: %w", err)
	}
	if err := s.apply(ctx, &userID, receipt); err != nil {
		return nil, err
	}
	return NewService(s.db).Subscription(ctx, userID)
}

// apply writes a validated receipt to the subscription row.
//
// The store's original transaction id is unique, so a subscription that moves
// between accounts is reassigned rather than duplicated — which is also what
// stops one purchase entitling two users.
func (s *IAPService) apply(ctx context.Context, userID *uuid.UUID, r *Receipt) error {
	tier, ok := TierForProduct(r.ProductID)
	if !ok {
		return fmt.Errorf("unknown product %q", r.ProductID)
	}
	status := "active"
	switch {
	case r.Revoked:
		status, tier = "refunded", TierFree
	case r.ExpiresAt.Before(time.Now()):
		status = "expired"
	}

	if userID == nil {
		// A webhook carries no user id, so resolve it from the transaction.
		var found uuid.UUID
		err := s.db.QueryRow(ctx,
			`SELECT user_id FROM subscriptions WHERE original_txn_id = $1`,
			r.OriginalTransactionID).Scan(&found)
		if err != nil {
			return fmt.Errorf("unknown subscription %q: %w", r.OriginalTransactionID, err)
		}
		userID = &found
	}

	_, err := s.db.Exec(ctx, `
		INSERT INTO subscriptions (user_id, tier, platform, original_txn_id,
		                           product_id, status, expires_at, auto_renew, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8, now())
		ON CONFLICT (user_id) DO UPDATE SET
			tier = EXCLUDED.tier, platform = EXCLUDED.platform,
			original_txn_id = EXCLUDED.original_txn_id,
			product_id = EXCLUDED.product_id, status = EXCLUDED.status,
			expires_at = EXCLUDED.expires_at, auto_renew = EXCLUDED.auto_renew,
			updated_at = now()`,
		*userID, tier, string(r.Platform), r.OriginalTransactionID,
		r.ProductID, status, r.ExpiresAt, r.AutoRenew)
	if err != nil {
		return fmt.Errorf("store subscription: %w", err)
	}
	return nil
}

// HandleWebhook records and applies a store notification.
//
// Store notifications are redelivered on any doubt, so the (platform,
// event_uid) unique constraint is what makes reprocessing safe: a duplicate
// is recorded once and applied once.
func (s *IAPService) HandleWebhook(ctx context.Context, platform Platform, eventUID, kind string,
	payload json.RawMessage, receipt *Receipt) error {

	tag, err := s.db.Exec(ctx, `
		INSERT INTO billing_events (platform, event_uid, kind, payload)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (platform, event_uid) DO NOTHING`,
		string(platform), eventUID, kind, payload)
	if err != nil {
		return fmt.Errorf("record billing event: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil // already handled
	}

	if receipt != nil {
		if err := s.apply(ctx, nil, receipt); err != nil {
			return err
		}
	}
	_, err = s.db.Exec(ctx, `
		UPDATE billing_events SET processed_at = now()
		WHERE platform = $1 AND event_uid = $2`, string(platform), eventUID)
	return err
}

// GrantPromo gives a tier without a purchase, for testing and support.
func (s *IAPService) GrantPromo(ctx context.Context, userID uuid.UUID, tier Tier, until time.Time) error {
	if _, ok := rank[tier]; !ok {
		return fmt.Errorf("unknown tier %q", tier)
	}
	_, err := s.db.Exec(ctx, `
		INSERT INTO subscriptions (user_id, tier, platform, status, expires_at, auto_renew, updated_at)
		VALUES ($1,$2,'promo','active',$3,FALSE, now())
		ON CONFLICT (user_id) DO UPDATE SET
			tier = EXCLUDED.tier, platform = 'promo', status = 'active',
			expires_at = EXCLUDED.expires_at, auto_renew = FALSE, updated_at = now()`,
		userID, tier, until)
	return err
}

// ExpireLapsed marks subscriptions past their expiry. The worker runs it.
func (s *IAPService) ExpireLapsed(ctx context.Context) (int64, error) {
	tag, err := s.db.Exec(ctx, `
		UPDATE subscriptions SET status = 'expired', tier = 'free', updated_at = now()
		WHERE status IN ('active','in_grace') AND expires_at IS NOT NULL AND expires_at < now()`)
	if err != nil {
		return 0, fmt.Errorf("expire subscriptions: %w", err)
	}
	return tag.RowsAffected(), nil
}
