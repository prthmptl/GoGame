package rating

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Record is a stored rating row.
type Record struct {
	UserID     uuid.UUID  `json:"userId"`
	BoardSize  int        `json:"boardSize"`
	TimeClass  string     `json:"timeClass"`
	Rating     float64    `json:"rating"`
	Deviation  float64    `json:"deviation"`
	Volatility float64    `json:"-"`
	Display    int        `json:"display"`
	Games      int        `json:"games"`
	Wins       int        `json:"wins"`
	Losses     int        `json:"losses"`
	Draws      int        `json:"draws"`
	LastPlayed *time.Time `json:"lastPlayedAt,omitempty"`
	// Provisional marks a rating still too uncertain to publish on a
	// leaderboard, the usual convention being fewer than ~10 games.
	Provisional bool `json:"provisional"`
}

// HistoryPoint is one entry in a rating graph.
type HistoryPoint struct {
	GameID    *uuid.UUID `json:"gameId,omitempty"`
	Rating    float64    `json:"rating"`
	Deviation float64    `json:"deviation"`
	At        time.Time  `json:"at"`
}

// provisionalGames is how many rated games a player needs before their rating
// is treated as established.
const provisionalGames = 10

// Service persists ratings.
type Service struct{ db *pgxpool.Pool }

// NewService builds the rating service.
func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

// Get returns one rating, creating the default row if it does not exist.
func (s *Service) Get(ctx context.Context, userID uuid.UUID, boardSize int, timeClass string) (*Record, error) {
	var r Record
	err := s.db.QueryRow(ctx, `
		INSERT INTO ratings (user_id, board_size, time_class)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, board_size, time_class) DO UPDATE
		SET updated_at = ratings.updated_at
		RETURNING user_id, board_size, time_class, rating, deviation, volatility,
		          games, wins, losses, draws, last_played_at`,
		userID, boardSize, timeClass,
	).Scan(&r.UserID, &r.BoardSize, &r.TimeClass, &r.Rating, &r.Deviation,
		&r.Volatility, &r.Games, &r.Wins, &r.Losses, &r.Draws, &r.LastPlayed)
	if err != nil {
		return nil, fmt.Errorf("get rating: %w", err)
	}
	r.Display = ConservativeRating(Player{Rating: r.Rating, Deviation: r.Deviation})
	r.Provisional = r.Games < provisionalGames
	return &r, nil
}

// All returns every rating a user holds.
func (s *Service) All(ctx context.Context, userID uuid.UUID) ([]Record, error) {
	rows, err := s.db.Query(ctx, `
		SELECT user_id, board_size, time_class, rating, deviation, volatility,
		       games, wins, losses, draws, last_played_at
		FROM ratings WHERE user_id = $1
		ORDER BY board_size, time_class`, userID)
	if err != nil {
		return nil, fmt.Errorf("list ratings: %w", err)
	}
	defer rows.Close()
	out := []Record{}
	for rows.Next() {
		var r Record
		if err := rows.Scan(&r.UserID, &r.BoardSize, &r.TimeClass, &r.Rating,
			&r.Deviation, &r.Volatility, &r.Games, &r.Wins, &r.Losses,
			&r.Draws, &r.LastPlayed); err != nil {
			return nil, err
		}
		r.Display = ConservativeRating(Player{Rating: r.Rating, Deviation: r.Deviation})
		r.Provisional = r.Games < provisionalGames
		out = append(out, r)
	}
	return out, rows.Err()
}

// GameResult describes a finished rated game.
type GameResult struct {
	GameID    uuid.UUID
	BoardSize int
	TimeClass string
	BlackID   uuid.UUID
	WhiteID   uuid.UUID
	// Winner is "black", "white" or "draw".
	Winner string
}

// ApplyGame updates both players' ratings for one completed game.
//
// Both updates use the ratings as they stood *before* the game, so the order
// in which the two players are processed cannot affect the outcome. The whole
// thing is one transaction, and the unique index on (game_id, user_id) in
// ratings_history makes a replayed completion event a no-op rather than a
// double count.
func (s *Service) ApplyGame(ctx context.Context, res GameResult) error {
	if res.Winner != "black" && res.Winner != "white" && res.Winner != "draw" {
		return fmt.Errorf("unsupported winner %q", res.Winner)
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Serialize completion retries before checking history. Advisory locks are
	// transaction-scoped, so a crash releases them automatically.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, res.GameID.String()); err != nil {
		return err
	}
	// Already rated? Then this is a replay.
	var already bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM ratings_history WHERE game_id = $1)`,
		res.GameID).Scan(&already); err != nil {
		return fmt.Errorf("check history: %w", err)
	}
	if already {
		return nil
	}

	// Lock user rows in a stable order across board/time categories too;
	// users.rating is updated in this transaction alongside the category row.
	ids := []uuid.UUID{res.BlackID, res.WhiteID}
	if bytes.Compare(ids[0][:], ids[1][:]) > 0 {
		ids[0], ids[1] = ids[1], ids[0]
	}
	locked := map[uuid.UUID]Player{}
	for _, id := range ids {
		var found uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, id).Scan(&found); err != nil {
			return err
		}
		player, err := s.lockRating(ctx, tx, id, res.BoardSize, res.TimeClass)
		if err != nil {
			return err
		}
		locked[id] = player
	}
	black, white := locked[res.BlackID], locked[res.WhiteID]

	blackScore, whiteScore := Draw, Draw
	switch res.Winner {
	case "black":
		blackScore, whiteScore = Win, Loss
	case "white":
		blackScore, whiteScore = Loss, Win
	}

	newBlack := Update(black, white, blackScore)
	newWhite := Update(white, black, whiteScore)

	for _, u := range []struct {
		id      uuid.UUID
		before  Player
		after   Player
		outcome Outcome
	}{
		{res.BlackID, black, newBlack, blackScore},
		{res.WhiteID, white, newWhite, whiteScore},
	} {
		wins, losses, draws := 0, 0, 0
		switch u.outcome {
		case Win:
			wins = 1
		case Loss:
			losses = 1
		default:
			draws = 1
		}
		if _, err := tx.Exec(ctx, `
			UPDATE ratings SET rating = $4, deviation = $5, volatility = $6,
			       games = games + 1, wins = wins + $7, losses = losses + $8,
			       draws = draws + $9, last_played_at = now(), updated_at = now()
			WHERE user_id = $1 AND board_size = $2 AND time_class = $3`,
			u.id, res.BoardSize, res.TimeClass,
			u.after.Rating, u.after.Deviation, u.after.Volatility,
			wins, losses, draws); err != nil {
			return fmt.Errorf("update rating: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO ratings_history (user_id, game_id, board_size, time_class,
			                             rating_before, rating_after, deviation_after)
			VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			u.id, res.GameID, res.BoardSize, res.TimeClass,
			u.before.Rating, u.after.Rating, u.after.Deviation); err != nil {
			return fmt.Errorf("insert history: %w", err)
		}
		// Keep users.rating as the display cache the client and C3 read.
		if _, err := tx.Exec(ctx,
			`UPDATE users SET rating = $2, updated_at = now() WHERE id = $1`,
			u.id, ConservativeRating(u.after)); err != nil {
			return fmt.Errorf("update display rating: %w", err)
		}
	}

	return tx.Commit(ctx)
}

// lockRating reads a rating FOR UPDATE, inserting the default row first if
// needed, so two concurrent games for the same player serialise.
func (s *Service) lockRating(ctx context.Context, tx pgx.Tx, userID uuid.UUID, boardSize int, timeClass string) (Player, error) {
	if _, err := tx.Exec(ctx, `
		INSERT INTO ratings (user_id, board_size, time_class)
		VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`,
		userID, boardSize, timeClass); err != nil {
		return Player{}, fmt.Errorf("ensure rating row: %w", err)
	}
	var p Player
	err := tx.QueryRow(ctx, `
		SELECT rating, deviation, volatility FROM ratings
		WHERE user_id = $1 AND board_size = $2 AND time_class = $3
		FOR UPDATE`, userID, boardSize, timeClass,
	).Scan(&p.Rating, &p.Deviation, &p.Volatility)
	if errors.Is(err, pgx.ErrNoRows) {
		return NewPlayer(), nil
	}
	if err != nil {
		return Player{}, fmt.Errorf("lock rating: %w", err)
	}
	return p, nil
}

// History returns a user's rating graph for one board size and time class.
func (s *Service) History(ctx context.Context, userID uuid.UUID, boardSize int, timeClass string, limit int) ([]HistoryPoint, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.Query(ctx, `
		SELECT game_id, rating_after, deviation_after, created_at
		FROM ratings_history
		WHERE user_id = $1 AND board_size = $2 AND time_class = $3
		ORDER BY created_at DESC LIMIT $4`,
		userID, boardSize, timeClass, limit)
	if err != nil {
		return nil, fmt.Errorf("rating history: %w", err)
	}
	defer rows.Close()
	out := []HistoryPoint{}
	for rows.Next() {
		var p HistoryPoint
		if err := rows.Scan(&p.GameID, &p.Rating, &p.Deviation, &p.At); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Oldest first, which is what a graph wants.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

// DecayInactive raises deviation for players who have not played recently.
// The worker runs this daily.
func (s *Service) DecayInactive(ctx context.Context, idleFor time.Duration) (int64, error) {
	// One rating period is taken as a week: long enough that a regular player
	// is never decayed, short enough that a year away is clearly reflected.
	tag, err := s.db.Exec(ctx, `
		UPDATE ratings
		SET deviation = LEAST(350, sqrt(deviation^2 +
		        (EXTRACT(EPOCH FROM (now() - GREATEST(last_played_at, updated_at))) / 604800) * volatility^2 * 173.7178^2)),
		    updated_at = now()
		WHERE last_played_at IS NOT NULL
		  AND last_played_at < now() - $1::interval
		  AND deviation < 350`,
		fmt.Sprintf("%d seconds", int(idleFor.Seconds())))
	if err != nil {
		return 0, fmt.Errorf("decay ratings: %w", err)
	}
	return tag.RowsAffected(), nil
}
