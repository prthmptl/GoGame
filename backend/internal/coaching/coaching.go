// Package coaching implements F5: the coach marketplace.
//
// Payments are modelled but not executed here: PaymentProcessor is an
// interface, and the only implementation shipped is a no-op. Taking real
// money needs a processor account, a payout onboarding flow and tax handling,
// none of which are code this repository can complete on its own.
package coaching

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Errors returned by the service.
var (
	ErrNotFound    = errors.New("coaching: not found")
	ErrSlotTaken   = errors.New("coaching: that slot is already booked")
	ErrUnavailable = errors.New("coaching: the coach is not available then")
	ErrForbidden   = errors.New("coaching: not your session")
)

// Coach is a marketplace profile.
type Coach struct {
	UserID      uuid.UUID `json:"userId"`
	DisplayName string    `json:"displayName"`
	Headline    string    `json:"headline"`
	Bio         *string   `json:"bio,omitempty"`
	HourlyRate  int       `json:"hourlyRateCents"`
	Currency    string    `json:"currency"`
	Languages   []string  `json:"languages"`
	RankLabel   *string   `json:"rankLabel,omitempty"`
	Status      string    `json:"status"`
	RatingAvg   float64   `json:"ratingAvg"`
	RatingCount int       `json:"ratingCount"`
}

// Slot is a recurring weekly availability window, in UTC.
type Slot struct {
	Weekday     int `json:"weekday"`
	StartMinute int `json:"startMinute"`
	EndMinute   int `json:"endMinute"`
}

// Session is a booking.
type Session struct {
	ID         uuid.UUID `json:"id"`
	CoachID    uuid.UUID `json:"coachId"`
	StudentID  uuid.UUID `json:"studentId"`
	StartsAt   time.Time `json:"startsAt"`
	Duration   int       `json:"durationMinutes"`
	PriceCents int       `json:"priceCents"`
	Currency   string    `json:"currency"`
	Status     string    `json:"status"`
	Notes      *string   `json:"notes,omitempty"`
}

// PaymentProcessor abstracts the payment provider (Stripe Connect or
// equivalent). Implementations handle the charge and the coach's payout.
type PaymentProcessor interface {
	// Authorize reserves the student's funds and returns a reference.
	Authorize(ctx context.Context, s Session) (reference string, err error)
	// Capture takes the authorized payment once the session is delivered.
	Capture(ctx context.Context, reference string) error
	// Refund returns the money.
	Refund(ctx context.Context, reference string) error
}

// NoopProcessor records nothing and charges nothing. It is the default so the
// marketplace can be exercised end-to-end before a processor account exists —
// bookings work, money does not move.
type NoopProcessor struct{}

// Authorize returns a placeholder reference.
func (NoopProcessor) Authorize(_ context.Context, s Session) (string, error) {
	return "noop-" + s.ID.String(), nil
}

// Capture does nothing successfully.
func (NoopProcessor) Capture(context.Context, string) error { return nil }

// Refund does nothing successfully.
func (NoopProcessor) Refund(context.Context, string) error { return nil }

// Service manages coaches and bookings.
type Service struct {
	db      *pgxpool.Pool
	payment PaymentProcessor
}

// NewService builds the coaching service.
func NewService(db *pgxpool.Pool, p PaymentProcessor) *Service {
	if p == nil {
		p = NoopProcessor{}
	}
	return &Service{db: db, payment: p}
}

// Apply creates or updates a coach profile. New profiles start pending, so a
// human can vet them before they appear in the marketplace.
func (s *Service) Apply(ctx context.Context, userID uuid.UUID, headline, bio string,
	hourlyRateCents int, currency string, languages []string, rankLabel string) (*Coach, error) {

	headline = strings.TrimSpace(headline)
	if headline == "" || len([]rune(headline)) > 120 {
		return nil, errors.New("coaching: headline must be 1-120 characters")
	}
	if hourlyRateCents < 0 || hourlyRateCents > 100_000_00 {
		return nil, errors.New("coaching: hourly rate is out of range")
	}
	if currency == "" {
		currency = "USD"
	}

	var c Coach
	err := s.db.QueryRow(ctx, `
		INSERT INTO coaches (user_id, headline, bio, hourly_rate_cents, currency,
		                     languages, rank_label)
		VALUES ($1,$2,NULLIF($3,''),$4,$5,$6,NULLIF($7,''))
		ON CONFLICT (user_id) DO UPDATE SET
			headline = EXCLUDED.headline, bio = EXCLUDED.bio,
			hourly_rate_cents = EXCLUDED.hourly_rate_cents,
			currency = EXCLUDED.currency, languages = EXCLUDED.languages,
			rank_label = EXCLUDED.rank_label
		RETURNING user_id, headline, bio, hourly_rate_cents, currency, languages,
		          rank_label, status, rating_avg, rating_count`,
		userID, headline, bio, hourlyRateCents, currency, languages, rankLabel,
	).Scan(&c.UserID, &c.Headline, &c.Bio, &c.HourlyRate, &c.Currency,
		&c.Languages, &c.RankLabel, &c.Status, &c.RatingAvg, &c.RatingCount)
	if err != nil {
		return nil, fmt.Errorf("apply as coach: %w", err)
	}
	return &c, nil
}

// List returns active coaches, best rated first.
func (s *Service) List(ctx context.Context, language string, maxRate, limit int) ([]Coach, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	sql := `
		SELECT c.user_id, u.display_name, c.headline, c.bio, c.hourly_rate_cents,
		       c.currency, c.languages, c.rank_label, c.status, c.rating_avg, c.rating_count
		FROM coaches c JOIN users u ON u.id = c.user_id
		WHERE c.status = 'active'`
	args := []any{limit}
	if language != "" {
		args = append(args, language)
		sql += fmt.Sprintf(" AND $%d = ANY(c.languages)", len(args))
	}
	if maxRate > 0 {
		args = append(args, maxRate)
		sql += fmt.Sprintf(" AND c.hourly_rate_cents <= $%d", len(args))
	}
	sql += ` ORDER BY c.rating_avg DESC, c.rating_count DESC LIMIT $1`

	rows, err := s.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("list coaches: %w", err)
	}
	defer rows.Close()
	out := []Coach{}
	for rows.Next() {
		var c Coach
		if err := rows.Scan(&c.UserID, &c.DisplayName, &c.Headline, &c.Bio,
			&c.HourlyRate, &c.Currency, &c.Languages, &c.RankLabel, &c.Status,
			&c.RatingAvg, &c.RatingCount); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SetAvailability replaces a coach's weekly calendar.
func (s *Service) SetAvailability(ctx context.Context, coachID uuid.UUID, slots []Slot) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`DELETE FROM coach_availability WHERE coach_id = $1`, coachID); err != nil {
		return err
	}
	for _, sl := range slots {
		if sl.Weekday < 0 || sl.Weekday > 6 || sl.EndMinute <= sl.StartMinute {
			return fmt.Errorf("invalid slot %+v", sl)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO coach_availability (coach_id, weekday, start_minute, end_minute)
			VALUES ($1,$2,$3,$4)`, coachID, sl.Weekday, sl.StartMinute, sl.EndMinute); err != nil {
			return fmt.Errorf("set availability: %w", err)
		}
	}
	return tx.Commit(ctx)
}

// Availability returns a coach's weekly calendar.
func (s *Service) Availability(ctx context.Context, coachID uuid.UUID) ([]Slot, error) {
	rows, err := s.db.Query(ctx, `
		SELECT weekday, start_minute, end_minute FROM coach_availability
		WHERE coach_id = $1 ORDER BY weekday, start_minute`, coachID)
	if err != nil {
		return nil, fmt.Errorf("availability: %w", err)
	}
	defer rows.Close()
	out := []Slot{}
	for rows.Next() {
		var sl Slot
		if err := rows.Scan(&sl.Weekday, &sl.StartMinute, &sl.EndMinute); err != nil {
			return nil, err
		}
		out = append(out, sl)
	}
	return out, rows.Err()
}

// Book requests a session.
//
// The slot must fall inside the coach's published availability, and the
// unique index on (coach_id, starts_at) is what actually prevents a double
// booking under concurrency — the availability check alone would race.
func (s *Service) Book(ctx context.Context, coachID, studentID uuid.UUID,
	startsAt time.Time, durationMinutes int, notes string) (*Session, error) {

	if coachID == studentID {
		return nil, errors.New("coaching: you cannot book yourself")
	}
	if durationMinutes < 15 || durationMinutes > 240 {
		return nil, errors.New("coaching: duration must be 15-240 minutes")
	}
	if startsAt.Before(time.Now()) {
		return nil, errors.New("coaching: cannot book a slot in the past")
	}

	var rate int
	var currency, status string
	if err := s.db.QueryRow(ctx, `
		SELECT hourly_rate_cents, currency, status FROM coaches WHERE user_id = $1`,
		coachID).Scan(&rate, &currency, &status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if status != "active" {
		return nil, ErrUnavailable
	}

	ok, err := s.withinAvailability(ctx, coachID, startsAt, durationMinutes)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrUnavailable
	}

	price := rate * durationMinutes / 60
	var sess Session
	err = s.db.QueryRow(ctx, `
		INSERT INTO coaching_sessions (coach_id, student_id, starts_at,
		                               duration_minutes, price_cents, currency, notes)
		VALUES ($1,$2,$3,$4,$5,$6,NULLIF($7,''))
		RETURNING id, coach_id, student_id, starts_at, duration_minutes,
		          price_cents, currency, status, notes`,
		coachID, studentID, startsAt, durationMinutes, price, currency, notes,
	).Scan(&sess.ID, &sess.CoachID, &sess.StudentID, &sess.StartsAt, &sess.Duration,
		&sess.PriceCents, &sess.Currency, &sess.Status, &sess.Notes)
	if err != nil {
		if strings.Contains(err.Error(), "23505") || strings.Contains(err.Error(), "duplicate") {
			return nil, ErrSlotTaken
		}
		return nil, fmt.Errorf("book session: %w", err)
	}

	ref, err := s.payment.Authorize(ctx, sess)
	if err != nil {
		// Roll the booking back rather than holding a slot nobody paid for.
		_, _ = s.db.Exec(ctx,
			`UPDATE coaching_sessions SET status = 'cancelled' WHERE id = $1`, sess.ID)
		return nil, fmt.Errorf("authorize payment: %w", err)
	}
	_, _ = s.db.Exec(ctx,
		`UPDATE coaching_sessions SET payment_ref = $2 WHERE id = $1`, sess.ID, ref)
	return &sess, nil
}

// withinAvailability checks a requested slot against the weekly calendar.
func (s *Service) withinAvailability(ctx context.Context, coachID uuid.UUID, startsAt time.Time, duration int) (bool, error) {
	utc := startsAt.UTC()
	weekday := int(utc.Weekday())
	startMinute := utc.Hour()*60 + utc.Minute()
	endMinute := startMinute + duration

	var ok bool
	err := s.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM coach_availability
			WHERE coach_id = $1 AND weekday = $2
			  AND start_minute <= $3 AND end_minute >= $4
		)`, coachID, weekday, startMinute, endMinute).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("availability check: %w", err)
	}
	return ok, nil
}

// Confirm lets a coach accept a request.
func (s *Service) Confirm(ctx context.Context, sessionID, coachID uuid.UUID) error {
	tag, err := s.db.Exec(ctx, `
		UPDATE coaching_sessions SET status = 'confirmed'
		WHERE id = $1 AND coach_id = $2 AND status = 'requested'`, sessionID, coachID)
	if err != nil {
		return fmt.Errorf("confirm: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrForbidden
	}
	return nil
}

// Complete captures payment once the session has been delivered.
func (s *Service) Complete(ctx context.Context, sessionID, coachID uuid.UUID) error {
	var ref *string
	var status string
	if err := s.db.QueryRow(ctx, `
		SELECT payment_ref, status FROM coaching_sessions
		WHERE id = $1 AND coach_id = $2`, sessionID, coachID).Scan(&ref, &status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrForbidden
		}
		return err
	}
	if status == "completed" {
		return nil
	}
	if ref != nil {
		if err := s.payment.Capture(ctx, *ref); err != nil {
			return fmt.Errorf("capture payment: %w", err)
		}
	}
	_, err := s.db.Exec(ctx,
		`UPDATE coaching_sessions SET status = 'completed' WHERE id = $1`, sessionID)
	return err
}

// Cancel voids a booking and refunds any authorization.
func (s *Service) Cancel(ctx context.Context, sessionID, actorID uuid.UUID) error {
	var ref *string
	var status string
	if err := s.db.QueryRow(ctx, `
		SELECT payment_ref, status FROM coaching_sessions
		WHERE id = $1 AND (coach_id = $2 OR student_id = $2)`,
		sessionID, actorID).Scan(&ref, &status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrForbidden
		}
		return err
	}
	if status == "completed" {
		return errors.New("coaching: a completed session cannot be cancelled")
	}
	if ref != nil {
		if err := s.payment.Refund(ctx, *ref); err != nil {
			return fmt.Errorf("refund: %w", err)
		}
	}
	_, err := s.db.Exec(ctx,
		`UPDATE coaching_sessions SET status = 'cancelled' WHERE id = $1`, sessionID)
	return err
}

// Sessions lists a user's bookings, as coach or student.
func (s *Service) Sessions(ctx context.Context, userID uuid.UUID, limit int) ([]Session, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	rows, err := s.db.Query(ctx, `
		SELECT id, coach_id, student_id, starts_at, duration_minutes,
		       price_cents, currency, status, notes
		FROM coaching_sessions
		WHERE coach_id = $1 OR student_id = $1
		ORDER BY starts_at DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("sessions: %w", err)
	}
	defer rows.Close()
	out := []Session{}
	for rows.Next() {
		var sess Session
		if err := rows.Scan(&sess.ID, &sess.CoachID, &sess.StudentID, &sess.StartsAt,
			&sess.Duration, &sess.PriceCents, &sess.Currency, &sess.Status, &sess.Notes); err != nil {
			return nil, err
		}
		out = append(out, sess)
	}
	return out, rows.Err()
}

// Review rates a coach. Only a student who completed a session may review it,
// which is what stops review-bombing by people who never booked.
func (s *Service) Review(ctx context.Context, sessionID, studentID uuid.UUID, rating int, comment string) error {
	if rating < 1 || rating > 5 {
		return errors.New("coaching: rating must be 1-5")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var coachID uuid.UUID
	var status string
	if err := tx.QueryRow(ctx, `
		SELECT coach_id, status FROM coaching_sessions
		WHERE id = $1 AND student_id = $2`, sessionID, studentID).Scan(&coachID, &status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrForbidden
		}
		return err
	}
	if status != "completed" {
		return errors.New("coaching: you can only review a completed session")
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO coach_reviews (coach_id, student_id, session_id, rating, comment)
		VALUES ($1,$2,$3,$4,NULLIF($5,''))
		ON CONFLICT (coach_id, student_id, session_id) DO UPDATE
		SET rating = EXCLUDED.rating, comment = EXCLUDED.comment`,
		coachID, studentID, sessionID, rating, comment); err != nil {
		return fmt.Errorf("store review: %w", err)
	}
	// Recompute rather than incrementally adjust, so an edited review cannot
	// drift the average.
	if _, err := tx.Exec(ctx, `
		UPDATE coaches SET
			rating_avg = COALESCE((SELECT avg(rating)::real FROM coach_reviews WHERE coach_id = $1), 0),
			rating_count = (SELECT count(*) FROM coach_reviews WHERE coach_id = $1)
		WHERE user_id = $1`, coachID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
