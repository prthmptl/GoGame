package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/prathpatel/gogame-backend/internal/billing"
)

// requireFeature enforces the G2 gating matrix on a boolean capability.
//
// This is the server-side half of feature gating: the client's own check is
// a UX affordance, and anything actually worth money has to be refused here
// too.
func (s *Server) requireFeature(r *http.Request, feature string) error {
	tier, err := s.billing.TierFor(r.Context(), callerFrom(r.Context()).ID)
	if err != nil {
		return err
	}
	if !billing.Allows(tier, billing.Feature(feature)) {
		return fmt.Errorf("%s is not included in the %s tier", feature, tier)
	}
	return nil
}

// consumeQuota enforces a metered feature, returning false once the caller's
// allowance for the period is spent.
func (s *Server) consumeQuota(w http.ResponseWriter, r *http.Request, feature billing.Feature) bool {
	err := s.billing.Consume(r.Context(), callerFrom(r.Context()).ID, feature)
	switch {
	case err == nil:
		return true
	case errors.Is(err, billing.ErrQuotaExceeded):
		writeError(w, http.StatusTooManyRequests, "quota_exceeded",
			"you have used your allowance for this period")
	case errors.Is(err, billing.ErrNotEntitled):
		writeError(w, http.StatusForbidden, "not_entitled",
			"this feature is not included in your plan")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "internal error")
	}
	return false
}

// --- G2: entitlements ---

func (s *Server) handleEntitlements(w http.ResponseWriter, r *http.Request) {
	out, err := s.billing.Entitlements(r.Context(), callerFrom(r.Context()).ID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	sub, err := s.billing.Subscription(r.Context(), callerFrom(r.Context()).ID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"subscription": sub,
		"entitlements": out,
	})
}

// handleConsume meters one use of a feature. The client calls this before
// starting a puzzle or requesting an AI review.
func (s *Server) handleConsume(w http.ResponseWriter, r *http.Request) {
	feature := billing.Feature(r.PathValue("feature"))
	if !s.consumeQuota(w, r, feature) {
		return
	}
	out, err := s.billing.Entitlements(r.Context(), callerFrom(r.Context()).ID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entitlements": out})
}

// --- G1: purchases ---

type redeemRequest struct {
	Platform string `json:"platform"`
	Token    string `json:"token"`
}

func (s *Server) handleRedeemPurchase(w http.ResponseWriter, r *http.Request) {
	var req redeemRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	sub, err := s.iap.Redeem(r.Context(), callerFrom(r.Context()).ID,
		billing.Platform(req.Platform), req.Token)
	if err != nil {
		if errors.Is(err, billing.ErrValidatorNotConfigured) {
			// Fail loudly rather than silently granting: an unverified
			// receipt must never become an entitlement.
			writeError(w, http.StatusServiceUnavailable, "billing_unconfigured",
				"purchase validation is not configured on this server")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid_receipt", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sub)
}

func (s *Server) handleSubscription(w http.ResponseWriter, r *http.Request) {
	sub, err := s.billing.Subscription(r.Context(), callerFrom(r.Context()).ID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, sub)
}

// handleStoreWebhook receives renewal, refund and expiry notifications.
//
// Deliberately unauthenticated in the routing sense — stores cannot present a
// bearer token — so the handler must verify the notification's own signature
// before trusting it. That verification lives in the ReceiptValidator, which
// is why an unconfigured server records the event but grants nothing.
func (s *Server) handleStoreWebhook(w http.ResponseWriter, r *http.Request) {
	platform := billing.Platform(r.PathValue("platform"))
	switch platform {
	case billing.PlatformIOS, billing.PlatformAndroid:
	default:
		writeError(w, http.StatusBadRequest, "unknown_platform", "unsupported store")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "unreadable", "could not read body")
		return
	}
	var envelope struct {
		NotificationUID string `json:"notificationUID"`
		Kind            string `json:"kind"`
	}
	_ = json.Unmarshal(body, &envelope)
	if envelope.NotificationUID == "" {
		writeError(w, http.StatusBadRequest, "missing_uid",
			"the notification has no identifier to deduplicate on")
		return
	}

	// The receipt is nil here: without store credentials the payload cannot
	// be verified, so the event is recorded for reconciliation and no
	// entitlement is changed.
	if err := s.iap.HandleWebhook(r.Context(), platform, envelope.NotificationUID,
		envelope.Kind, body, nil); err != nil {
		logging_(r).Error("webhook handling failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- G3: ads ---

func (s *Server) handleAdPolicy(w http.ResponseWriter, r *http.Request) {
	policy, err := s.billing.AdPolicyFor(r.Context(), callerFrom(r.Context()).ID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, policy)
}
