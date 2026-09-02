// Package correspondence implements E4: daily games, vacation mode and
// conditional moves.
package correspondence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/prathpatel/gogame-backend/internal/goban"
)

// Errors returned by the service.
var (
	ErrNotFound       = errors.New("correspondence: not found")
	ErrNoVacationLeft = errors.New("correspondence: no vacation days remaining")
	ErrStalePlan      = errors.New("correspondence: conditional plan is for an earlier position")
)

// State is a daily game's clock.
type State struct {
	GameID      uuid.UUID  `json:"gameId"`
	DaysPerMove int        `json:"daysPerMove"`
	ToMove      string     `json:"toMove"`
	DeadlineAt  time.Time  `json:"deadlineAt"`
	PausedAt    *time.Time `json:"pausedAt,omitempty"`
	LastMoveAt  time.Time  `json:"lastMoveAt"`
}

// Paused reports whether the deadline is frozen.
func (s State) Paused() bool { return s.PausedAt != nil }

// Vacation is a player's banked time off.
type Vacation struct {
	DaysRemaining float64    `json:"daysRemaining"`
	ActiveSince   *time.Time `json:"activeSince,omitempty"`
}

// Active reports whether the player is currently away.
func (v Vacation) Active() bool { return v.ActiveSince != nil }

// Branch is one "if they play X, I answer Y" step.
type Branch struct {
	If   goban.Point `json:"if"`
	Then goban.Point `json:"then"`
}

// Plan is a stored conditional-move sequence.
type Plan struct {
	GameID   uuid.UUID `json:"gameId"`
	UserID   uuid.UUID `json:"userId"`
	FromMove int       `json:"fromMove"`
	Sequence []Branch  `json:"sequence"`
}

// Service manages correspondence games.
type Service struct{ db *pgxpool.Pool }

// NewService builds the correspondence service.
func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

// Start creates the daily clock for a game.
func (s *Service) Start(ctx context.Context, gameID uuid.UUID, daysPerMove int, toMove string) (*State, error) {
	switch daysPerMove {
	case 1, 3, 7, 14:
	default:
		return nil, fmt.Errorf("unsupported budget of %d days", daysPerMove)
	}
	var st State
	err := s.db.QueryRow(ctx, `
		INSERT INTO correspondence_state (game_id, days_per_move, to_move, deadline_at)
		VALUES ($1, $2, $3, now() + ($2 || ' days')::interval)
		RETURNING game_id, days_per_move, to_move, deadline_at, paused_at, last_move_at`,
		gameID, daysPerMove, toMove,
	).Scan(&st.GameID, &st.DaysPerMove, &st.ToMove, &st.DeadlineAt, &st.PausedAt, &st.LastMoveAt)
	if err != nil {
		return nil, fmt.Errorf("start correspondence: %w", err)
	}
	return &st, nil
}

// Get returns a game's daily state.
func (s *Service) Get(ctx context.Context, gameID uuid.UUID) (*State, error) {
	var st State
	err := s.db.QueryRow(ctx, `
		SELECT game_id, days_per_move, to_move, deadline_at, paused_at, last_move_at
		FROM correspondence_state WHERE game_id = $1`, gameID,
	).Scan(&st.GameID, &st.DaysPerMove, &st.ToMove, &st.DeadlineAt, &st.PausedAt, &st.LastMoveAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get correspondence: %w", err)
	}
	return &st, nil
}

// RecordMove hands the deadline to the opponent with a fresh budget.
func (s *Service) RecordMove(ctx context.Context, gameID uuid.UUID, mover string) error {
	next := "white"
	if mover == "white" {
		next = "black"
	}
	tag, err := s.db.Exec(ctx, `
		UPDATE correspondence_state
		SET to_move = $2,
		    deadline_at = now() + (days_per_move || ' days')::interval,
		    last_move_at = now()
		WHERE game_id = $1`, gameID, next)
	if err != nil {
		return fmt.Errorf("record correspondence move: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Expired lists games whose deadline has passed, for the timeout job.
// Paused games are excluded by the partial index.
func (s *Service) Expired(ctx context.Context, limit int) ([]State, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(ctx, `
		SELECT cs.game_id, cs.days_per_move, cs.to_move, cs.deadline_at,
		       cs.paused_at, cs.last_move_at
		FROM correspondence_state cs
		JOIN games g ON g.id = cs.game_id
		WHERE cs.paused_at IS NULL AND cs.deadline_at < now() AND g.status = 'active'
		ORDER BY cs.deadline_at
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("expired games: %w", err)
	}
	defer rows.Close()
	out := []State{}
	for rows.Next() {
		var st State
		if err := rows.Scan(&st.GameID, &st.DaysPerMove, &st.ToMove,
			&st.DeadlineAt, &st.PausedAt, &st.LastMoveAt); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

// --- vacation ---

// GetVacation returns a player's banked days, creating the account on first
// use.
func (s *Service) GetVacation(ctx context.Context, userID uuid.UUID) (*Vacation, error) {
	var v Vacation
	err := s.db.QueryRow(ctx, `
		INSERT INTO vacation_accounts (user_id) VALUES ($1)
		ON CONFLICT (user_id) DO UPDATE SET updated_at = vacation_accounts.updated_at
		RETURNING days_remaining, active_since`, userID,
	).Scan(&v.DaysRemaining, &v.ActiveSince)
	if err != nil {
		return nil, fmt.Errorf("get vacation: %w", err)
	}
	return &v, nil
}

// StartVacation pauses every daily game the player is in.
func (s *Service) StartVacation(ctx context.Context, userID uuid.UUID) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var days float64
	var activeSince *time.Time
	if err := tx.QueryRow(ctx, `
		INSERT INTO vacation_accounts (user_id) VALUES ($1)
		ON CONFLICT (user_id) DO UPDATE SET updated_at = now()
		RETURNING days_remaining, active_since`, userID,
	).Scan(&days, &activeSince); err != nil {
		return fmt.Errorf("lock vacation: %w", err)
	}
	if activeSince != nil {
		return nil // already away
	}
	if days <= 0 {
		return ErrNoVacationLeft
	}

	if _, err := tx.Exec(ctx,
		`UPDATE vacation_accounts SET active_since = now(), updated_at = now() WHERE user_id = $1`,
		userID); err != nil {
		return err
	}
	// Freeze every active daily game this player is in, whether or not it is
	// their turn: coming back to a clock that ran down while away defeats
	// the purpose.
	if _, err := tx.Exec(ctx, `
		UPDATE correspondence_state cs
		SET paused_at = now()
		FROM games g
		WHERE g.id = cs.game_id AND g.status = 'active' AND cs.paused_at IS NULL
		  AND (g.black_user_id = $1 OR g.white_user_id = $1)`, userID); err != nil {
		return fmt.Errorf("pause games: %w", err)
	}
	return tx.Commit(ctx)
}

// EndVacation deducts the days used and resumes the player's games, pushing
// each deadline out by exactly how long the pause lasted.
func (s *Service) EndVacation(ctx context.Context, userID uuid.UUID) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var activeSince *time.Time
	if err := tx.QueryRow(ctx,
		`SELECT active_since FROM vacation_accounts WHERE user_id = $1 FOR UPDATE`,
		userID).Scan(&activeSince); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if activeSince == nil {
		return nil
	}

	used := time.Since(*activeSince).Hours() / 24
	if _, err := tx.Exec(ctx, `
		UPDATE vacation_accounts
		SET days_remaining = GREATEST(0, days_remaining - $2),
		    active_since = NULL, updated_at = now()
		WHERE user_id = $1`, userID, used); err != nil {
		return err
	}

	// Only resume games where no *other* participant is still away.
	if _, err := tx.Exec(ctx, `
		UPDATE correspondence_state cs
		SET deadline_at = cs.deadline_at + (now() - cs.paused_at),
		    paused_at = NULL
		FROM games g
		WHERE g.id = cs.game_id AND cs.paused_at IS NOT NULL
		  AND (g.black_user_id = $1 OR g.white_user_id = $1)
		  AND NOT EXISTS (
			SELECT 1 FROM vacation_accounts va
			WHERE va.active_since IS NOT NULL
			  AND va.user_id IN (g.black_user_id, g.white_user_id)
			  AND va.user_id <> $1
		  )`, userID); err != nil {
		return fmt.Errorf("resume games: %w", err)
	}
	return tx.Commit(ctx)
}

// DrainVacationDays deducts elapsed time from players who are currently away,
// ending the vacation for anyone who runs out. The worker runs this hourly.
func (s *Service) DrainVacationDays(ctx context.Context) ([]uuid.UUID, error) {
	rows, err := s.db.Query(ctx, `
		SELECT user_id FROM vacation_accounts
		WHERE active_since IS NOT NULL
		  AND days_remaining <= EXTRACT(EPOCH FROM (now() - active_since)) / 86400`)
	if err != nil {
		return nil, fmt.Errorf("find exhausted vacations: %w", err)
	}
	defer rows.Close()
	var exhausted []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		exhausted = append(exhausted, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	for _, id := range exhausted {
		if err := s.EndVacation(ctx, id); err != nil {
			return exhausted, err
		}
	}
	return exhausted, nil
}

// --- conditional moves ---

// SetPlan stores a player's conditional sequence for the current position.
func (s *Service) SetPlan(ctx context.Context, gameID, userID uuid.UUID, fromMove int, seq []Branch) error {
	if len(seq) == 0 {
		_, err := s.db.Exec(ctx,
			`DELETE FROM conditional_moves WHERE game_id = $1 AND user_id = $2`, gameID, userID)
		return err
	}
	if len(seq) > 50 {
		return fmt.Errorf("a plan may hold at most 50 branches, got %d", len(seq))
	}
	raw, err := json.Marshal(seq)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(ctx, `
		INSERT INTO conditional_moves (game_id, user_id, from_move, sequence)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (game_id, user_id) DO UPDATE
		SET from_move = EXCLUDED.from_move, sequence = EXCLUDED.sequence,
		    created_at = now()`,
		gameID, userID, fromMove, raw)
	if err != nil {
		return fmt.Errorf("store plan: %w", err)
	}
	return nil
}

// GetPlan returns a player's stored plan.
func (s *Service) GetPlan(ctx context.Context, gameID, userID uuid.UUID) (*Plan, error) {
	var p Plan
	var raw []byte
	err := s.db.QueryRow(ctx, `
		SELECT game_id, user_id, from_move, sequence
		FROM conditional_moves WHERE game_id = $1 AND user_id = $2`, gameID, userID,
	).Scan(&p.GameID, &p.UserID, &p.FromMove, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get plan: %w", err)
	}
	if err := json.Unmarshal(raw, &p.Sequence); err != nil {
		return nil, err
	}
	return &p, nil
}

// ClearPlan removes a stored plan.
func (s *Service) ClearPlan(ctx context.Context, gameID, userID uuid.UUID) error {
	_, err := s.db.Exec(ctx,
		`DELETE FROM conditional_moves WHERE game_id = $1 AND user_id = $2`, gameID, userID)
	return err
}

// ResolvePlan walks a plan against the opponent's actual move and returns the
// replies to play, plus whatever remains of the plan.
//
// The chain continues as long as each successive opponent move matches the
// next branch. `atMove` guards against a plan written for an earlier position
// firing later, which would play a move into a board the author never saw.
func ResolvePlan(p *Plan, atMove int, opponentMove goban.Point) (reply *goban.Point, rest []Branch, err error) {
	if p == nil || len(p.Sequence) == 0 {
		return nil, nil, nil
	}
	if p.FromMove != atMove {
		return nil, nil, ErrStalePlan
	}
	head := p.Sequence[0]
	if head.If != opponentMove {
		// The opponent played something else, so the whole plan is void.
		return nil, nil, nil
	}
	answer := head.Then
	return &answer, p.Sequence[1:], nil
}
