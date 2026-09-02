package anticheat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned for a missing case.
var ErrNotFound = errors.New("anticheat: not found")

// Case is a review-queue entry.
type Case struct {
	ID          uuid.UUID  `json:"id"`
	UserID      uuid.UUID  `json:"userId"`
	DisplayName string     `json:"displayName"`
	Status      string     `json:"status"`
	ScoreAtOpen float64    `json:"scoreAtOpen"`
	Action      *string    `json:"action,omitempty"`
	Notes       *string    `json:"notes,omitempty"`
	OpenedAt    time.Time  `json:"openedAt"`
	ResolvedAt  *time.Time `json:"resolvedAt,omitempty"`
	Signals     []Signal   `json:"signals,omitempty"`
}

// Restriction is an enforcement action.
type Restriction struct {
	Kind      string     `json:"kind"`
	Reason    *string    `json:"reason,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
}

// Service records signals and manages the queue.
type Service struct{ db *pgxpool.Pool }

// NewService builds the anti-cheat service.
func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

// RecordSignals stores a batch of signals for one user and game, recomputes
// their composite score, and opens a review case if warranted.
func (s *Service) RecordSignals(ctx context.Context, userID, gameID uuid.UUID, signals []Signal) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, sig := range signals {
		detail, err := json.Marshal(sig.Detail)
		if err != nil {
			detail = []byte(`{}`)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO cheat_signals (user_id, game_id, kind, score, detail)
			VALUES ($1,$2,$3,$4,$5)
			ON CONFLICT (user_id, game_id, kind) DO UPDATE
			SET score = EXCLUDED.score, detail = EXCLUDED.detail`,
			userID, gameID, string(sig.Kind), clamp(sig.Score), detail); err != nil {
			return fmt.Errorf("insert signal: %w", err)
		}
	}

	// Recompute from the user's recent history rather than this game alone:
	// a single game is far too small a sample to act on.
	recent, err := s.recentSignals(ctx, tx, userID, 50)
	if err != nil {
		return err
	}
	composite := Composite(recent)

	if _, err := tx.Exec(ctx, `
		INSERT INTO cheat_scores (user_id, score, signals, updated_at)
		VALUES ($1,$2,$3, now())
		ON CONFLICT (user_id) DO UPDATE
		SET score = EXCLUDED.score, signals = EXCLUDED.signals, updated_at = now()`,
		userID, composite, len(recent)); err != nil {
		return fmt.Errorf("update score: %w", err)
	}

	if ShouldFlag(recent, composite) {
		// The partial unique index makes this a no-op when a case is already
		// open for this user.
		if _, err := tx.Exec(ctx, `
			INSERT INTO cheat_cases (user_id, score_at_open)
			VALUES ($1, $2)
			ON CONFLICT DO NOTHING`, userID, composite); err != nil {
			return fmt.Errorf("open case: %w", err)
		}
	}
	return tx.Commit(ctx)
}

// recentSignals returns the strongest recent signal of each kind, so one bad
// game does not dominate a user's score through sheer repetition.
func (s *Service) recentSignals(ctx context.Context, tx pgx.Tx, userID uuid.UUID, limit int) ([]Signal, error) {
	rows, err := tx.Query(ctx, `
		SELECT DISTINCT ON (kind) kind, score, detail
		FROM (
			SELECT kind, score, detail, created_at
			FROM cheat_signals WHERE user_id = $1
			ORDER BY created_at DESC LIMIT $2
		) recent
		ORDER BY kind, score DESC`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("read signals: %w", err)
	}
	defer rows.Close()

	var out []Signal
	for rows.Next() {
		var sig Signal
		var raw []byte
		if err := rows.Scan(&sig.Kind, &sig.Score, &raw); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(raw, &sig.Detail)
		out = append(out, sig)
	}
	return out, rows.Err()
}

// Queue lists open cases, most suspicious first.
func (s *Service) Queue(ctx context.Context, status string, limit int) ([]Case, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	if status == "" {
		status = "open"
	}
	rows, err := s.db.Query(ctx, `
		SELECT c.id, c.user_id, u.display_name, c.status, c.score_at_open,
		       c.action, c.notes, c.opened_at, c.resolved_at
		FROM cheat_cases c JOIN users u ON u.id = c.user_id
		WHERE c.status = $1
		ORDER BY c.score_at_open DESC, c.opened_at ASC
		LIMIT $2`, status, limit)
	if err != nil {
		return nil, fmt.Errorf("queue: %w", err)
	}
	defer rows.Close()

	out := []Case{}
	for rows.Next() {
		var c Case
		if err := rows.Scan(&c.ID, &c.UserID, &c.DisplayName, &c.Status,
			&c.ScoreAtOpen, &c.Action, &c.Notes, &c.OpenedAt, &c.ResolvedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetCase returns one case with its supporting signals.
func (s *Service) GetCase(ctx context.Context, id uuid.UUID) (*Case, error) {
	var c Case
	err := s.db.QueryRow(ctx, `
		SELECT c.id, c.user_id, u.display_name, c.status, c.score_at_open,
		       c.action, c.notes, c.opened_at, c.resolved_at
		FROM cheat_cases c JOIN users u ON u.id = c.user_id
		WHERE c.id = $1`, id,
	).Scan(&c.ID, &c.UserID, &c.DisplayName, &c.Status, &c.ScoreAtOpen,
		&c.Action, &c.Notes, &c.OpenedAt, &c.ResolvedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get case: %w", err)
	}

	rows, err := s.db.Query(ctx, `
		SELECT kind, score, detail FROM cheat_signals
		WHERE user_id = $1 ORDER BY created_at DESC LIMIT 50`, c.UserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var sig Signal
		var raw []byte
		if err := rows.Scan(&sig.Kind, &sig.Score, &raw); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(raw, &sig.Detail)
		c.Signals = append(c.Signals, sig)
	}
	c.Signals = TopSignals(c.Signals)
	return &c, rows.Err()
}

// Resolve closes a case with an action. Passing "none" dismisses it.
//
// Enforcement is deliberately a separate, explicit step from detection: no
// code path in this package suspends an account without a reviewer id.
func (s *Service) Resolve(ctx context.Context, caseID, reviewerID uuid.UUID, action, notes string, expires *time.Time) error {
	switch action {
	case "none", "warn", "restrict_ranked", "suspend":
	default:
		return fmt.Errorf("unsupported action %q", action)
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	status := "actioned"
	if action == "none" {
		status = "dismissed"
	}
	var userID uuid.UUID
	err = tx.QueryRow(ctx, `
		UPDATE cheat_cases
		SET status = $2, action = $3, notes = NULLIF($4,''), reviewer_id = $5,
		    resolved_at = now()
		WHERE id = $1 AND status IN ('open','reviewing')
		RETURNING user_id`,
		caseID, status, action, notes, reviewerID).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("resolve case: %w", err)
	}

	if action != "none" {
		if _, err := tx.Exec(ctx, `
			INSERT INTO account_restrictions (user_id, kind, reason, case_id, expires_at)
			VALUES ($1,$2,NULLIF($3,''),$4,$5)`,
			userID, action, notes, caseID, expires); err != nil {
			return fmt.Errorf("apply restriction: %w", err)
		}
	}
	return tx.Commit(ctx)
}

// Restrictions returns a user's active restrictions. Handlers call this to
// gate ranked play and sign-in.
func (s *Service) Restrictions(ctx context.Context, userID uuid.UUID) ([]Restriction, error) {
	rows, err := s.db.Query(ctx, `
		SELECT kind, reason, created_at, expires_at
		FROM account_restrictions
		WHERE user_id = $1 AND lifted_at IS NULL
		  AND (expires_at IS NULL OR expires_at > now())
		ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("restrictions: %w", err)
	}
	defer rows.Close()
	out := []Restriction{}
	for rows.Next() {
		var r Restriction
		if err := rows.Scan(&r.Kind, &r.Reason, &r.CreatedAt, &r.ExpiresAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// IsRankedRestricted reports whether a user may not play ranked right now.
func (s *Service) IsRankedRestricted(ctx context.Context, userID uuid.UUID) (bool, error) {
	var blocked bool
	err := s.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM account_restrictions
			WHERE user_id = $1 AND lifted_at IS NULL
			  AND kind IN ('restrict_ranked','suspend')
			  AND (expires_at IS NULL OR expires_at > now())
		)`, userID).Scan(&blocked)
	return blocked, err
}
