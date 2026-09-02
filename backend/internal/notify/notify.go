// Package notify implements C5: device registration, per-event preferences,
// and a transactional outbox delivered to FCM and APNs.
package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EventType identifies a notification kind. These are the events C5 lists.
type EventType string

const (
	EventCorrespondenceTurn EventType = "correspondence_turn"
	EventFriendOnline       EventType = "friend_online"
	EventTournamentStarting EventType = "tournament_starting"
	EventDailyPuzzleReady   EventType = "daily_puzzle_ready"
	EventClubInvitation     EventType = "club_invitation"
	EventGameChallenge      EventType = "game_challenge"
	EventDirectMessage      EventType = "direct_message"
)

// defaultEnabled decides what happens with no explicit preference row.
// Turn notifications default on because a correspondence game stalls without
// them; social noise defaults on but is the first thing users switch off.
var defaultEnabled = map[EventType]bool{
	EventCorrespondenceTurn: true,
	EventFriendOnline:       false,
	EventTournamentStarting: true,
	EventDailyPuzzleReady:   true,
	EventClubInvitation:     true,
	EventGameChallenge:      true,
	EventDirectMessage:      true,
}

// Pref is one user-facing toggle.
type Pref struct {
	EventType EventType `json:"eventType"`
	Enabled   bool      `json:"enabled"`
}

// Device is a registered push target.
type Device struct {
	ID         uuid.UUID `json:"id"`
	Platform   string    `json:"platform"`
	CreatedAt  time.Time `json:"createdAt"`
	LastSeenAt time.Time `json:"lastSeenAt"`
}

// Notification is one queued message.
type Notification struct {
	UserID      uuid.UUID
	EventType   EventType
	Title       string
	Body        string
	Data        map[string]string
	CollapseKey string
}

// Sender delivers to a push provider. Implementations: FCM for Android and
// web, APNs for iOS.
type Sender interface {
	Send(ctx context.Context, token, platform string, n Notification) error
}

// PermanentError marks a token as dead so the worker stops retrying it.
type PermanentError struct{ Reason string }

func (e *PermanentError) Error() string { return "push permanently failed: " + e.Reason }

// Service manages devices, preferences and the outbox.
type Service struct {
	db     *pgxpool.Pool
	sender Sender
}

// NewService builds the notification service.
func NewService(db *pgxpool.Pool, sender Sender) *Service {
	return &Service{db: db, sender: sender}
}

// RegisterDevice records or refreshes a push token. The same token moving to
// a different user (a shared device) reassigns it rather than duplicating.
func (s *Service) RegisterDevice(ctx context.Context, userID uuid.UUID, token, platform, locale, appVersion string) error {
	switch platform {
	case "android", "ios", "web":
	default:
		return fmt.Errorf("unsupported platform %q", platform)
	}
	_, err := s.db.Exec(ctx, `
		INSERT INTO device_tokens (user_id, token, platform, locale, app_version)
		VALUES ($1, $2, $3, NULLIF($4,''), NULLIF($5,''))
		ON CONFLICT (token) DO UPDATE
		SET user_id = EXCLUDED.user_id,
		    platform = EXCLUDED.platform,
		    locale = EXCLUDED.locale,
		    app_version = EXCLUDED.app_version,
		    last_seen_at = now(),
		    disabled_at = NULL,
		    disable_reason = NULL`,
		userID, token, platform, locale, appVersion)
	if err != nil {
		return fmt.Errorf("register device: %w", err)
	}
	return nil
}

// UnregisterDevice removes a token, e.g. on sign-out.
func (s *Service) UnregisterDevice(ctx context.Context, userID uuid.UUID, token string) error {
	_, err := s.db.Exec(ctx,
		`DELETE FROM device_tokens WHERE user_id = $1 AND token = $2`, userID, token)
	return err
}

// Prefs returns every event type with its effective setting.
func (s *Service) Prefs(ctx context.Context, userID uuid.UUID) ([]Pref, error) {
	rows, err := s.db.Query(ctx,
		`SELECT event_type, enabled FROM notification_prefs WHERE user_id = $1`, userID)
	if err != nil {
		return nil, fmt.Errorf("query prefs: %w", err)
	}
	defer rows.Close()

	explicit := map[EventType]bool{}
	for rows.Next() {
		var et EventType
		var enabled bool
		if err := rows.Scan(&et, &enabled); err != nil {
			return nil, err
		}
		explicit[et] = enabled
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]Pref, 0, len(defaultEnabled))
	for et, def := range defaultEnabled {
		enabled := def
		if v, ok := explicit[et]; ok {
			enabled = v
		}
		out = append(out, Pref{EventType: et, Enabled: enabled})
	}
	return out, nil
}

// SetPref updates one toggle.
func (s *Service) SetPref(ctx context.Context, userID uuid.UUID, et EventType, enabled bool) error {
	if _, known := defaultEnabled[et]; !known {
		return fmt.Errorf("unknown event type %q", et)
	}
	_, err := s.db.Exec(ctx, `
		INSERT INTO notification_prefs (user_id, event_type, enabled)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, event_type) DO UPDATE
		SET enabled = EXCLUDED.enabled, updated_at = now()`,
		userID, et, enabled)
	return err
}

// Enqueue queues a notification, honouring the user's preference. Pass a tx
// to enqueue atomically with the event that caused it; pass nil to use the
// pool directly.
func (s *Service) Enqueue(ctx context.Context, tx pgx.Tx, n Notification) error {
	enabled, err := s.isEnabled(ctx, n.UserID, n.EventType)
	if err != nil {
		return err
	}
	if !enabled {
		return nil
	}
	data, err := json.Marshal(n.Data)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	const sql = `
		INSERT INTO notification_outbox (user_id, event_type, title, body, data, collapse_key)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6,''))`
	if tx != nil {
		_, err = tx.Exec(ctx, sql, n.UserID, n.EventType, n.Title, n.Body, data, n.CollapseKey)
	} else {
		_, err = s.db.Exec(ctx, sql, n.UserID, n.EventType, n.Title, n.Body, data, n.CollapseKey)
	}
	if err != nil {
		return fmt.Errorf("enqueue notification: %w", err)
	}
	return nil
}

func (s *Service) isEnabled(ctx context.Context, userID uuid.UUID, et EventType) (bool, error) {
	var enabled bool
	err := s.db.QueryRow(ctx,
		`SELECT enabled FROM notification_prefs WHERE user_id = $1 AND event_type = $2`,
		userID, et).Scan(&enabled)
	switch {
	case err == nil:
		return enabled, nil
	case err == pgx.ErrNoRows:
		def, known := defaultEnabled[et]
		if !known {
			return false, fmt.Errorf("unknown event type %q", et)
		}
		return def, nil
	default:
		return false, fmt.Errorf("read pref: %w", err)
	}
}

// maxAttempts bounds retries before a notification is abandoned.
const maxAttempts = 5

// DeliverBatch claims up to limit due notifications and sends them. Returns
// how many were delivered and how many failed.
//
// FOR UPDATE SKIP LOCKED lets several worker instances drain the same outbox
// without handing the same row to two of them.
func (s *Service) DeliverBatch(ctx context.Context, limit int) (delivered, failed int, err error) {
	if s.sender == nil {
		return 0, 0, nil
	}
	if limit <= 0 {
		limit = 100
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, `
		SELECT id, user_id, event_type, title, body, data, collapse_key, attempts
		FROM notification_outbox
		WHERE delivered_at IS NULL AND failed_at IS NULL AND next_attempt_at <= now()
		ORDER BY id
		LIMIT $1
		FOR UPDATE SKIP LOCKED`, limit)
	if err != nil {
		return 0, 0, fmt.Errorf("claim outbox: %w", err)
	}

	type job struct {
		id       int64
		n        Notification
		attempts int
	}
	var jobs []job
	for rows.Next() {
		var j job
		var raw []byte
		var collapse *string
		if err := rows.Scan(&j.id, &j.n.UserID, &j.n.EventType, &j.n.Title,
			&j.n.Body, &raw, &collapse, &j.attempts); err != nil {
			rows.Close()
			return 0, 0, fmt.Errorf("scan outbox: %w", err)
		}
		_ = json.Unmarshal(raw, &j.n.Data)
		if collapse != nil {
			j.n.CollapseKey = *collapse
		}
		jobs = append(jobs, j)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, 0, err
	}

	for _, j := range jobs {
		devices, derr := s.devicesFor(ctx, tx, j.n.UserID)
		if derr != nil {
			return delivered, failed, derr
		}
		if len(devices) == 0 {
			// Nothing to deliver to; retiring the row keeps the outbox from
			// filling with undeliverable work.
			_, _ = tx.Exec(ctx,
				`UPDATE notification_outbox SET failed_at = now(), last_error = 'no devices' WHERE id = $1`, j.id)
			failed++
			continue
		}

		var lastErr error
		anySent := false
		for _, d := range devices {
			serr := s.sender.Send(ctx, d.token, d.platform, j.n)
			if serr == nil {
				anySent = true
				continue
			}
			var perm *PermanentError
			if errorsAs(serr, &perm) {
				_, _ = tx.Exec(ctx, `
					UPDATE device_tokens SET disabled_at = now(), disable_reason = $2
					WHERE token = $1`, d.token, perm.Reason)
				continue
			}
			lastErr = serr
		}

		switch {
		case anySent:
			_, _ = tx.Exec(ctx,
				`UPDATE notification_outbox SET delivered_at = now(), attempts = attempts + 1 WHERE id = $1`, j.id)
			delivered++
		case j.attempts+1 >= maxAttempts:
			_, _ = tx.Exec(ctx, `
				UPDATE notification_outbox
				SET failed_at = now(), attempts = attempts + 1, last_error = $2
				WHERE id = $1`, j.id, errString(lastErr))
			failed++
		default:
			// Exponential backoff: 1, 2, 4, 8 minutes.
			backoff := time.Duration(1<<j.attempts) * time.Minute
			_, _ = tx.Exec(ctx, `
				UPDATE notification_outbox
				SET attempts = attempts + 1, next_attempt_at = now() + $2::interval, last_error = $3
				WHERE id = $1`, j.id, fmt.Sprintf("%d seconds", int(backoff.Seconds())), errString(lastErr))
			failed++
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return delivered, failed, fmt.Errorf("commit: %w", err)
	}
	return delivered, failed, nil
}

type deviceRow struct{ token, platform string }

func (s *Service) devicesFor(ctx context.Context, tx pgx.Tx, userID uuid.UUID) ([]deviceRow, error) {
	rows, err := tx.Query(ctx,
		`SELECT token, platform FROM device_tokens WHERE user_id = $1 AND disabled_at IS NULL`, userID)
	if err != nil {
		return nil, fmt.Errorf("query devices: %w", err)
	}
	defer rows.Close()
	var out []deviceRow
	for rows.Next() {
		var d deviceRow
		if err := rows.Scan(&d.token, &d.platform); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
