// Package progames implements F4: the professional game library and live
// tournament relays.
package progames

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/prathpatel/gogame-backend/internal/blob"
	"github.com/prathpatel/gogame-backend/internal/goban"
)

// ErrNotFound is returned for a missing record.
var ErrNotFound = errors.New("progames: not found")

// ProGame is one archived professional game.
type ProGame struct {
	ID        uuid.UUID  `json:"id"`
	Source    string     `json:"source"`
	SourceRef string     `json:"sourceRef"`
	BlackName string     `json:"blackName"`
	WhiteName string     `json:"whiteName"`
	BlackRank *string    `json:"blackRank,omitempty"`
	WhiteRank *string    `json:"whiteRank,omitempty"`
	Event     *string    `json:"event,omitempty"`
	Round     *string    `json:"round,omitempty"`
	Place     *string    `json:"place,omitempty"`
	PlayedOn  *time.Time `json:"playedOn,omitempty"`
	BoardSize int        `json:"boardSize"`
	Komi      *float64   `json:"komi,omitempty"`
	Handicap  int        `json:"handicap"`
	Result    *string    `json:"result,omitempty"`
	Winner    *string    `json:"winner,omitempty"`
	MoveCount int        `json:"moveCount"`
}

// Relay is a live broadcast.
type Relay struct {
	ID          uuid.UUID       `json:"id"`
	Title       string          `json:"title"`
	Event       *string         `json:"event,omitempty"`
	Source      string          `json:"source"`
	Status      string          `json:"status"`
	BoardSize   int             `json:"boardSize"`
	BlackName   *string         `json:"blackName,omitempty"`
	WhiteName   *string         `json:"whiteName,omitempty"`
	Moves       json.RawMessage `json:"moves"`
	CurrentHash *string         `json:"currentHash,omitempty"`
	StartedAt   *time.Time      `json:"startedAt,omitempty"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}

// Service manages the library and relays.
type Service struct {
	db   *pgxpool.Pool
	blob blob.Store
}

// NewService builds the pro-games service.
func NewService(db *pgxpool.Pool, store blob.Store) *Service {
	return &Service{db: db, blob: store}
}

// ImportSGF stores one professional game from its SGF.
//
// `source` identifies the collection (e.g. a licensed game database) and
// `sourceRef` its id within it, so a re-import updates rather than duplicates.
// Whether a given collection may be redistributed is a licensing question,
// not a technical one — this only provides the mechanism.
func (s *Service) ImportSGF(ctx context.Context, source, sourceRef, sgf string) (*ProGame, error) {
	parsed, err := goban.ParseSGF(sgf)
	if err != nil {
		return nil, fmt.Errorf("parse sgf: %w", err)
	}

	// Validate by replaying: a record that does not obey the rules is a
	// corrupt file, and indexing it would poison the opening explorer.
	state := goban.NewGame(parsed.Config())
	for i, m := range parsed.Moves {
		next, _, err := state.Apply(m)
		if err != nil {
			return nil, fmt.Errorf("sgf move %d is illegal: %w", i+1, err)
		}
		state = next
	}

	var playedOn *time.Time
	if parsed.Date != "" {
		// SGF dates are usually YYYY-MM-DD but may be partial.
		for _, layout := range []string{"2006-01-02", "2006-01", "2006"} {
			if t, err := time.Parse(layout, strings.TrimSpace(strings.Split(parsed.Date, ",")[0])); err == nil {
				playedOn = &t
				break
			}
		}
	}
	winner := parsed.Winner()
	var winnerPtr *string
	if winner != "" {
		winnerPtr = &winner
	}
	searchText := strings.Join([]string{
		parsed.BlackName, parsed.WhiteName, parsed.Event, parsed.Place,
	}, " ")

	var g ProGame
	err = s.db.QueryRow(ctx, `
		INSERT INTO pro_games (source, source_ref, black_name, white_name,
		    black_rank, white_rank, event, round, place, played_on, board_size,
		    komi, handicap, result, winner, move_count, search_text)
		VALUES ($1,$2,$3,$4,NULLIF($5,''),NULLIF($6,''),NULLIF($7,''),
		        NULLIF($8,''),NULLIF($9,''),$10,$11,$12,$13,NULLIF($14,''),$15,$16,$17)
		ON CONFLICT (source, source_ref) DO UPDATE SET
			black_name = EXCLUDED.black_name, white_name = EXCLUDED.white_name,
			event = EXCLUDED.event, played_on = EXCLUDED.played_on,
			result = EXCLUDED.result, winner = EXCLUDED.winner,
			move_count = EXCLUDED.move_count, search_text = EXCLUDED.search_text
		RETURNING id, source, source_ref, black_name, white_name, black_rank,
		          white_rank, event, round, place, played_on, board_size, komi,
		          handicap, result, winner, move_count`,
		source, sourceRef, parsed.BlackName, parsed.WhiteName, parsed.BlackRank,
		parsed.WhiteRank, parsed.Event, parsed.Round, parsed.Place, playedOn,
		parsed.BoardSize, parsed.Komi, parsed.Handicap, parsed.Result, winnerPtr,
		len(parsed.Moves), searchText,
	).Scan(&g.ID, &g.Source, &g.SourceRef, &g.BlackName, &g.WhiteName, &g.BlackRank,
		&g.WhiteRank, &g.Event, &g.Round, &g.Place, &g.PlayedOn, &g.BoardSize,
		&g.Komi, &g.Handicap, &g.Result, &g.Winner, &g.MoveCount)
	if err != nil {
		return nil, fmt.Errorf("store pro game: %w", err)
	}

	key := "pro/" + source + "/" + g.ID.String() + ".sgf"
	if err := s.blob.Put(ctx, key, []byte(sgf), "application/x-go-sgf"); err == nil {
		_, _ = s.db.Exec(ctx, `UPDATE pro_games SET sgf_object_key = $2 WHERE id = $1`, g.ID, key)
	}
	return &g, nil
}

// Search finds games by player, event or place.
func (s *Service) Search(ctx context.Context, query string, limit int) ([]ProGame, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	sql := `
		SELECT id, source, source_ref, black_name, white_name, black_rank,
		       white_rank, event, round, place, played_on, board_size, komi,
		       handicap, result, winner, move_count
		FROM pro_games`
	args := []any{limit}
	if strings.TrimSpace(query) != "" {
		sql += ` WHERE to_tsvector('simple', coalesce(search_text,''))
		         @@ plainto_tsquery('simple', $2)`
		args = append(args, query)
	}
	sql += ` ORDER BY played_on DESC NULLS LAST LIMIT $1`

	rows, err := s.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("search pro games: %w", err)
	}
	defer rows.Close()
	out := []ProGame{}
	for rows.Next() {
		var g ProGame
		if err := rows.Scan(&g.ID, &g.Source, &g.SourceRef, &g.BlackName, &g.WhiteName,
			&g.BlackRank, &g.WhiteRank, &g.Event, &g.Round, &g.Place, &g.PlayedOn,
			&g.BoardSize, &g.Komi, &g.Handicap, &g.Result, &g.Winner, &g.MoveCount); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// SGF returns a pro game's record.
func (s *Service) SGF(ctx context.Context, id uuid.UUID) ([]byte, error) {
	var key *string
	if err := s.db.QueryRow(ctx,
		`SELECT sgf_object_key FROM pro_games WHERE id = $1`, id).Scan(&key); err != nil {
		return nil, ErrNotFound
	}
	if key == nil || *key == "" {
		return nil, ErrNotFound
	}
	return s.blob.Get(ctx, *key)
}

// --- relays ---

// CreateRelay opens a broadcast channel.
func (s *Service) CreateRelay(ctx context.Context, title, event, source string, boardSize int) (*Relay, error) {
	var r Relay
	err := s.db.QueryRow(ctx, `
		INSERT INTO relays (title, event, source, board_size, moves)
		VALUES ($1, NULLIF($2,''), $3, $4, '[]'::jsonb)
		RETURNING id, title, event, source, status, board_size, black_name,
		          white_name, moves, current_hash, started_at, updated_at`,
		title, event, source, boardSize,
	).Scan(&r.ID, &r.Title, &r.Event, &r.Source, &r.Status, &r.BoardSize,
		&r.BlackName, &r.WhiteName, &r.Moves, &r.CurrentHash, &r.StartedAt, &r.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create relay: %w", err)
	}
	return &r, nil
}

// IngestSGF replaces a relay's move list from a broadcaster's SGF snapshot.
//
// Feeds resend the whole game each time rather than deltas, so this validates
// the record and stores the full line. Subscribers are notified by the caller
// through the WebSocket watch channel.
func (s *Service) IngestSGF(ctx context.Context, relayID uuid.UUID, sgf string) (*Relay, error) {
	parsed, err := goban.ParseSGF(sgf)
	if err != nil {
		return nil, fmt.Errorf("parse relay sgf: %w", err)
	}
	state := goban.NewGame(parsed.Config())
	type wireMove struct {
		Row    *int   `json:"row,omitempty"`
		Col    *int   `json:"col,omitempty"`
		Kind   string `json:"kind"`
		Player string `json:"player"`
	}
	moves := make([]wireMove, 0, len(parsed.Moves))
	for i, m := range parsed.Moves {
		next, applied, err := state.Apply(m)
		if err != nil {
			// A truncated or garbled feed should not wipe a good relay.
			return nil, fmt.Errorf("relay move %d is illegal: %w", i+1, err)
		}
		wm := wireMove{Kind: string(applied.Kind), Player: colorName(applied.Player)}
		if applied.Point != nil {
			r, c := applied.Point.Row, applied.Point.Col
			wm.Row, wm.Col = &r, &c
		}
		moves = append(moves, wm)
		state = next
	}
	raw, err := json.Marshal(moves)
	if err != nil {
		return nil, err
	}
	hash := state.StateHash()

	var r Relay
	err = s.db.QueryRow(ctx, `
		UPDATE relays SET moves = $2, current_hash = $3,
		    black_name = COALESCE(NULLIF($4,''), black_name),
		    white_name = COALESCE(NULLIF($5,''), white_name),
		    status = CASE WHEN status = 'scheduled' THEN 'live' ELSE status END,
		    started_at = COALESCE(started_at, now()),
		    updated_at = now()
		WHERE id = $1
		RETURNING id, title, event, source, status, board_size, black_name,
		          white_name, moves, current_hash, started_at, updated_at`,
		relayID, raw, hash, parsed.BlackName, parsed.WhiteName,
	).Scan(&r.ID, &r.Title, &r.Event, &r.Source, &r.Status, &r.BoardSize,
		&r.BlackName, &r.WhiteName, &r.Moves, &r.CurrentHash, &r.StartedAt, &r.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("ingest relay: %w", err)
	}
	return &r, nil
}

// LiveRelays lists broadcasts currently running.
func (s *Service) LiveRelays(ctx context.Context, limit int) ([]Relay, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	rows, err := s.db.Query(ctx, `
		SELECT id, title, event, source, status, board_size, black_name,
		       white_name, moves, current_hash, started_at, updated_at
		FROM relays WHERE status IN ('live','scheduled')
		ORDER BY status DESC, started_at DESC NULLS LAST
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("live relays: %w", err)
	}
	defer rows.Close()
	out := []Relay{}
	for rows.Next() {
		var r Relay
		if err := rows.Scan(&r.ID, &r.Title, &r.Event, &r.Source, &r.Status,
			&r.BoardSize, &r.BlackName, &r.WhiteName, &r.Moves, &r.CurrentHash,
			&r.StartedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetRelay returns one broadcast.
func (s *Service) GetRelay(ctx context.Context, id uuid.UUID) (*Relay, error) {
	var r Relay
	err := s.db.QueryRow(ctx, `
		SELECT id, title, event, source, status, board_size, black_name,
		       white_name, moves, current_hash, started_at, updated_at
		FROM relays WHERE id = $1`, id,
	).Scan(&r.ID, &r.Title, &r.Event, &r.Source, &r.Status, &r.BoardSize,
		&r.BlackName, &r.WhiteName, &r.Moves, &r.CurrentHash, &r.StartedAt, &r.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get relay: %w", err)
	}
	return &r, nil
}

// FinishRelay closes a broadcast.
func (s *Service) FinishRelay(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.Exec(ctx,
		`UPDATE relays SET status = 'finished', updated_at = now() WHERE id = $1`, id)
	return err
}

func colorName(c goban.Color) string {
	if c == goban.Black {
		return "black"
	}
	return "white"
}
