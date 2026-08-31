package tournament

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

// ErrNotFound is returned for a missing tournament.
var ErrNotFound = errors.New("tournament: not found")

// Tournament is an event.
type Tournament struct {
	ID          uuid.UUID       `json:"id"`
	Name        string          `json:"name"`
	Description *string         `json:"description,omitempty"`
	Format      string          `json:"format"`
	BoardSize   int             `json:"boardSize"`
	Ruleset     string          `json:"ruleset"`
	TimeControl json.RawMessage `json:"timeControl"`
	Rounds      *int            `json:"rounds,omitempty"`
	Status      string          `json:"status"`
	StartsAt    time.Time       `json:"startsAt"`
	EndsAt      *time.Time      `json:"endsAt,omitempty"`
	Entries     int             `json:"entries"`
}

// Standing is one row of the leaderboard.
type Standing struct {
	UserID      uuid.UUID `json:"userId"`
	DisplayName string    `json:"displayName"`
	Rank        int       `json:"rank"`
	Score       float64   `json:"score"`
	Tiebreak    float64   `json:"tiebreak"`
	Wins        int       `json:"wins"`
	Losses      int       `json:"losses"`
	Draws       int       `json:"draws"`
}

// Service manages tournaments.
type Service struct{ db *pgxpool.Pool }

// NewService builds the tournament service.
func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

// List returns tournaments by status, soonest first.
func (s *Service) List(ctx context.Context, status string, limit int) ([]Tournament, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	sql := `
		SELECT t.id, t.name, t.description, t.format, t.board_size, t.ruleset,
		       t.time_control, t.rounds, t.status, t.starts_at, t.ends_at,
		       (SELECT count(*) FROM tournament_entries e WHERE e.tournament_id = t.id)
		FROM tournaments t`
	args := []any{limit}
	if status != "" {
		sql += ` WHERE t.status = $2`
		args = append(args, status)
	}
	sql += ` ORDER BY t.starts_at ASC LIMIT $1`

	rows, err := s.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("list tournaments: %w", err)
	}
	defer rows.Close()
	out := []Tournament{}
	for rows.Next() {
		var t Tournament
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.Format, &t.BoardSize,
			&t.Ruleset, &t.TimeControl, &t.Rounds, &t.Status, &t.StartsAt,
			&t.EndsAt, &t.Entries); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Get returns one tournament.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Tournament, error) {
	var t Tournament
	err := s.db.QueryRow(ctx, `
		SELECT t.id, t.name, t.description, t.format, t.board_size, t.ruleset,
		       t.time_control, t.rounds, t.status, t.starts_at, t.ends_at,
		       (SELECT count(*) FROM tournament_entries e WHERE e.tournament_id = t.id)
		FROM tournaments t WHERE t.id = $1`, id,
	).Scan(&t.ID, &t.Name, &t.Description, &t.Format, &t.BoardSize, &t.Ruleset,
		&t.TimeControl, &t.Rounds, &t.Status, &t.StartsAt, &t.EndsAt, &t.Entries)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get tournament: %w", err)
	}
	return &t, nil
}

// Join registers a player, seeding their McMahon score from their rating.
func (s *Service) Join(ctx context.Context, tournamentID, userID uuid.UUID, rating int) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var format, status string
	var maxEntries, minRating, maxRating *int
	if err := tx.QueryRow(ctx, `
		SELECT format, status, max_entries, min_rating, max_rating
		FROM tournaments WHERE id = $1 FOR UPDATE`, tournamentID,
	).Scan(&format, &status, &maxEntries, &minRating, &maxRating); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if status != "registering" && status != "scheduled" {
		return fmt.Errorf("tournament is %s and not accepting entries", status)
	}
	if minRating != nil && rating < *minRating {
		return fmt.Errorf("rating %d is below the %d minimum", rating, *minRating)
	}
	if maxRating != nil && rating > *maxRating {
		return fmt.Errorf("rating %d is above the %d maximum", rating, *maxRating)
	}
	if maxEntries != nil {
		var entries int
		if err := tx.QueryRow(ctx,
			`SELECT count(*) FROM tournament_entries WHERE tournament_id = $1`,
			tournamentID).Scan(&entries); err != nil {
			return err
		}
		if entries >= *maxEntries {
			return errors.New("tournament is full")
		}
	}

	// McMahon seeds players by strength so the top group meets immediately;
	// every other format starts everyone on zero.
	score := 0.0
	if format == "mcmahon" {
		score = McMahonSeed(rating, 2000, 100)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO tournament_entries (tournament_id, user_id, seed_rating, score)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (tournament_id, user_id) DO NOTHING`,
		tournamentID, userID, rating, score); err != nil {
		return fmt.Errorf("join tournament: %w", err)
	}
	return tx.Commit(ctx)
}

// Withdraw marks a player as no longer participating.
func (s *Service) Withdraw(ctx context.Context, tournamentID, userID uuid.UUID) error {
	_, err := s.db.Exec(ctx, `
		UPDATE tournament_entries SET withdrawn = TRUE
		WHERE tournament_id = $1 AND user_id = $2`, tournamentID, userID)
	return err
}

// Entrants loads the field with its pairing history.
func (s *Service) Entrants(ctx context.Context, tournamentID uuid.UUID) ([]Entrant, error) {
	rows, err := s.db.Query(ctx, `
		SELECT user_id, seed_rating, score, tiebreak, withdrawn, eliminated_in
		FROM tournament_entries WHERE tournament_id = $1`, tournamentID)
	if err != nil {
		return nil, fmt.Errorf("load entrants: %w", err)
	}
	defer rows.Close()

	byID := map[uuid.UUID]*Entrant{}
	var out []Entrant
	for rows.Next() {
		var e Entrant
		e.Opponents = map[uuid.UUID]bool{}
		if err := rows.Scan(&e.UserID, &e.SeedRating, &e.Score, &e.Tiebreak,
			&e.Withdrawn, &e.EliminatedIn); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	for i := range out {
		byID[out[i].UserID] = &out[i]
	}

	// Fill in who has already met whom, and who has had a bye, so the pairing
	// algorithms can honour both.
	pairRows, err := s.db.Query(ctx, `
		SELECT black_id, white_id, is_bye FROM tournament_pairings
		WHERE tournament_id = $1`, tournamentID)
	if err != nil {
		return nil, fmt.Errorf("load pairings: %w", err)
	}
	defer pairRows.Close()
	for pairRows.Next() {
		var black, white *uuid.UUID
		var isBye bool
		if err := pairRows.Scan(&black, &white, &isBye); err != nil {
			return nil, err
		}
		if isBye && black != nil {
			if e := byID[*black]; e != nil {
				e.HadBye = true
			}
			continue
		}
		if black == nil || white == nil {
			continue
		}
		if e := byID[*black]; e != nil {
			e.Opponents[*white] = true
		}
		if e := byID[*white]; e != nil {
			e.Opponents[*black] = true
		}
	}
	return out, pairRows.Err()
}

// StartRound pairs the next round and records it. Returns the pairings so the
// caller can create the games.
func (s *Service) StartRound(ctx context.Context, tournamentID uuid.UUID, round int) ([]Pairing, error) {
	t, err := s.Get(ctx, tournamentID)
	if err != nil {
		return nil, err
	}
	entrants, err := s.Entrants(ctx, tournamentID)
	if err != nil {
		return nil, err
	}

	var pairings []Pairing
	switch t.Format {
	case "swiss", "mcmahon":
		pairings = SwissPairings(entrants)
	case "knockout":
		pairings = KnockoutPairings(entrants)
	case "arena":
		pairings = ArenaPairings(entrants)
	default:
		return nil, fmt.Errorf("unsupported format %q", t.Format)
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
		INSERT INTO tournament_rounds (tournament_id, round_number, status, started_at)
		VALUES ($1,$2,'running', now())
		ON CONFLICT (tournament_id, round_number) DO UPDATE
		SET status = 'running', started_at = now()`, tournamentID, round); err != nil {
		return nil, fmt.Errorf("open round: %w", err)
	}

	for _, p := range pairings {
		if p.IsBye {
			// A bye is a free point, recorded so standings stay consistent.
			if _, err := tx.Exec(ctx, `
				INSERT INTO tournament_pairings (tournament_id, round_number, black_id, is_bye, result)
				VALUES ($1,$2,$3,TRUE,'bye')`, tournamentID, round, p.Black); err != nil {
				return nil, err
			}
			if _, err := tx.Exec(ctx, `
				UPDATE tournament_entries SET score = score + 1
				WHERE tournament_id = $1 AND user_id = $2`, tournamentID, p.Black); err != nil {
				return nil, err
			}
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO tournament_pairings (tournament_id, round_number, black_id, white_id)
			VALUES ($1,$2,$3,$4)`, tournamentID, round, p.Black, p.White); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return pairings, nil
}

// RecordResult applies one finished tournament game to the standings.
func (s *Service) RecordResult(ctx context.Context, tournamentID, gameID uuid.UUID, winner string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var black, white *uuid.UUID
	var round int
	var existing *string
	if err := tx.QueryRow(ctx, `
		SELECT black_id, white_id, round_number, result FROM tournament_pairings
		WHERE tournament_id = $1 AND game_id = $2 FOR UPDATE`, tournamentID, gameID,
	).Scan(&black, &white, &round, &existing); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if existing != nil {
		// Already recorded; a replayed completion must not double-score.
		return nil
	}
	if black == nil || white == nil {
		return errors.New("tournament: pairing has no opponents")
	}

	if _, err := tx.Exec(ctx,
		`UPDATE tournament_pairings SET result = $3 WHERE tournament_id = $1 AND game_id = $2`,
		tournamentID, gameID, winner); err != nil {
		return err
	}

	type delta struct {
		id                  uuid.UUID
		score               float64
		wins, losses, draws int
	}
	var deltas []delta
	switch winner {
	case "black":
		deltas = []delta{{*black, 1, 1, 0, 0}, {*white, 0, 0, 1, 0}}
	case "white":
		deltas = []delta{{*black, 0, 0, 1, 0}, {*white, 1, 1, 0, 0}}
	default:
		deltas = []delta{{*black, 0.5, 0, 0, 1}, {*white, 0.5, 0, 0, 1}}
	}
	for _, d := range deltas {
		if _, err := tx.Exec(ctx, `
			UPDATE tournament_entries
			SET score = score + $3, wins = wins + $4, losses = losses + $5, draws = draws + $6
			WHERE tournament_id = $1 AND user_id = $2`,
			tournamentID, d.id, d.score, d.wins, d.losses, d.draws); err != nil {
			return err
		}
	}

	// Knockout: the loser is out.
	var format string
	if err := tx.QueryRow(ctx, `SELECT format FROM tournaments WHERE id = $1`,
		tournamentID).Scan(&format); err != nil {
		return err
	}
	if format == "knockout" && winner != "draw" {
		loser := *black
		if winner == "black" {
			loser = *white
		}
		if _, err := tx.Exec(ctx, `
			UPDATE tournament_entries SET eliminated_in = $3
			WHERE tournament_id = $1 AND user_id = $2`, tournamentID, loser, round); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// Standings returns the leaderboard with tiebreaks applied.
func (s *Service) Standings(ctx context.Context, tournamentID uuid.UUID) ([]Standing, error) {
	entrants, err := s.Entrants(ctx, tournamentID)
	if err != nil {
		return nil, err
	}
	ordered := Standings(entrants)

	names := map[uuid.UUID]string{}
	rows, err := s.db.Query(ctx, `
		SELECT e.user_id, u.display_name, e.wins, e.losses, e.draws
		FROM tournament_entries e JOIN users u ON u.id = e.user_id
		WHERE e.tournament_id = $1`, tournamentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type record struct{ wins, losses, draws int }
	stats := map[uuid.UUID]record{}
	for rows.Next() {
		var id uuid.UUID
		var name string
		var r record
		if err := rows.Scan(&id, &name, &r.wins, &r.losses, &r.draws); err != nil {
			return nil, err
		}
		names[id] = name
		stats[id] = r
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]Standing, 0, len(ordered))
	for i, e := range ordered {
		r := stats[e.UserID]
		out = append(out, Standing{
			UserID: e.UserID, DisplayName: names[e.UserID], Rank: i + 1,
			Score: e.Score, Tiebreak: e.Tiebreak,
			Wins: r.wins, Losses: r.losses, Draws: r.draws,
		})
	}
	return out, nil
}

// AttachGame records which game a pairing is being played in.
func (s *Service) AttachGame(ctx context.Context, tournamentID uuid.UUID, black, white, gameID uuid.UUID, round int) error {
	_, err := s.db.Exec(ctx, `
		UPDATE tournament_pairings SET game_id = $5
		WHERE tournament_id = $1 AND round_number = $4 AND black_id = $2 AND white_id = $3`,
		tournamentID, black, white, round, gameID)
	return err
}
