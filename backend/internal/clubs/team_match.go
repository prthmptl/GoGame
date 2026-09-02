package clubs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// TeamMatch is a club-versus-club event.
type TeamMatch struct {
	ID          uuid.UUID       `json:"id"`
	HomeClubID  uuid.UUID       `json:"homeClubId"`
	AwayClubID  uuid.UUID       `json:"awayClubId"`
	HomeName    string          `json:"homeName"`
	AwayName    string          `json:"awayName"`
	BoardCount  int             `json:"boardCount"`
	BoardSize   int             `json:"boardSize"`
	TimeControl json.RawMessage `json:"timeControl"`
	Status      string          `json:"status"`
	HomeScore   float64         `json:"homeScore"`
	AwayScore   float64         `json:"awayScore"`
	Boards      []Board         `json:"boards,omitempty"`
	ScheduledAt *time.Time      `json:"scheduledAt,omitempty"`
}

// Board is one game in a team match.
type Board struct {
	BoardNumber int        `json:"boardNumber"`
	HomeUserID  *uuid.UUID `json:"homeUserId,omitempty"`
	AwayUserID  *uuid.UUID `json:"awayUserId,omitempty"`
	HomeName    string     `json:"homeName,omitempty"`
	AwayName    string     `json:"awayName,omitempty"`
	GameID      *uuid.UUID `json:"gameId,omitempty"`
	Result      *string    `json:"result,omitempty"`
}

// ProposeTeamMatch offers a match to another club. Only owners and admins of
// the home club may propose.
func (s *Service) ProposeTeamMatch(ctx context.Context, homeClubID, awayClubID, actorID uuid.UUID,
	boardCount, boardSize int, timeControl json.RawMessage, scheduledAt *time.Time) (*TeamMatch, error) {

	role, err := s.roleOf(ctx, homeClubID, actorID)
	if err != nil {
		return nil, err
	}
	if role != "owner" && role != "admin" {
		return nil, ErrForbidden
	}
	if homeClubID == awayClubID {
		return nil, errors.New("clubs: a club cannot play itself")
	}
	if boardCount < 1 || boardCount > 20 {
		return nil, fmt.Errorf("board count must be 1-20, got %d", boardCount)
	}

	var m TeamMatch
	err = s.db.QueryRow(ctx, `
		INSERT INTO club_team_matches (home_club_id, away_club_id, board_count,
		                               board_size, time_control, scheduled_at)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, home_club_id, away_club_id, board_count, board_size,
		          time_control, status, home_score, away_score, scheduled_at`,
		homeClubID, awayClubID, boardCount, boardSize, timeControl, scheduledAt,
	).Scan(&m.ID, &m.HomeClubID, &m.AwayClubID, &m.BoardCount, &m.BoardSize,
		&m.TimeControl, &m.Status, &m.HomeScore, &m.AwayScore, &m.ScheduledAt)
	if err != nil {
		return nil, fmt.Errorf("propose team match: %w", err)
	}
	return &m, nil
}

// AcceptTeamMatch lets the away club agree to play.
func (s *Service) AcceptTeamMatch(ctx context.Context, matchID, actorID uuid.UUID) error {
	var awayClubID uuid.UUID
	var status string
	if err := s.db.QueryRow(ctx,
		`SELECT away_club_id, status FROM club_team_matches WHERE id = $1`,
		matchID).Scan(&awayClubID, &status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if status != "proposed" {
		return fmt.Errorf("match is %s and cannot be accepted", status)
	}
	role, err := s.roleOf(ctx, awayClubID, actorID)
	if err != nil {
		return err
	}
	if role != "owner" && role != "admin" {
		return ErrForbidden
	}
	_, err = s.db.Exec(ctx,
		`UPDATE club_team_matches SET status = 'accepted' WHERE id = $1`, matchID)
	return err
}

// SeatBoards pairs the two clubs board by board.
//
// Each club's strongest available players are seated in rating order, so
// board 1 is the two clubs' best against each other. This is the standard
// team-match convention: it stops a club stacking its top player against the
// opponent's weakest.
func (s *Service) SeatBoards(ctx context.Context, matchID uuid.UUID) ([]Board, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var m TeamMatch
	if err := tx.QueryRow(ctx, `
		SELECT id, home_club_id, away_club_id, board_count, status
		FROM club_team_matches WHERE id = $1 FOR UPDATE`, matchID,
	).Scan(&m.ID, &m.HomeClubID, &m.AwayClubID, &m.BoardCount, &m.Status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if m.Status != "accepted" {
		return nil, fmt.Errorf("match is %s; accept it before seating boards", m.Status)
	}

	home, err := s.topMembers(ctx, tx, m.HomeClubID, m.BoardCount)
	if err != nil {
		return nil, err
	}
	away, err := s.topMembers(ctx, tx, m.AwayClubID, m.BoardCount)
	if err != nil {
		return nil, err
	}
	// A club that cannot field a full team plays as many boards as it can;
	// the remaining boards are forfeited rather than blocking the match.
	seats := min(len(home), len(away))
	if seats == 0 {
		return nil, errors.New("clubs: neither club can field a player")
	}

	boards := make([]Board, 0, seats)
	for i := 0; i < seats; i++ {
		h, a := home[i], away[i]
		if _, err := tx.Exec(ctx, `
			INSERT INTO club_team_boards (match_id, board_number, home_user_id, away_user_id)
			VALUES ($1,$2,$3,$4)
			ON CONFLICT (match_id, board_number) DO UPDATE
			SET home_user_id = EXCLUDED.home_user_id, away_user_id = EXCLUDED.away_user_id`,
			matchID, i+1, h.UserID, a.UserID); err != nil {
			return nil, fmt.Errorf("seat board %d: %w", i+1, err)
		}
		hid, aid := h.UserID, a.UserID
		boards = append(boards, Board{
			BoardNumber: i + 1, HomeUserID: &hid, AwayUserID: &aid,
			HomeName: h.DisplayName, AwayName: a.DisplayName,
		})
	}
	if _, err := tx.Exec(ctx,
		`UPDATE club_team_matches SET status = 'running' WHERE id = $1`, matchID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return boards, nil
}

func (s *Service) topMembers(ctx context.Context, tx pgx.Tx, clubID uuid.UUID, n int) ([]Member, error) {
	rows, err := tx.Query(ctx, `
		SELECT m.user_id, u.display_name, u.rating
		FROM club_members m JOIN users u ON u.id = m.user_id
		WHERE m.club_id = $1 AND u.deleted_at IS NULL
		ORDER BY u.rating DESC LIMIT $2`, clubID, n)
	if err != nil {
		return nil, fmt.Errorf("top members: %w", err)
	}
	defer rows.Close()
	var out []Member
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.UserID, &m.DisplayName, &m.Rating); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// AttachBoardGame records the game created for a board.
func (s *Service) AttachBoardGame(ctx context.Context, matchID uuid.UUID, boardNumber int, gameID uuid.UUID) error {
	_, err := s.db.Exec(ctx,
		`UPDATE club_team_boards SET game_id = $3 WHERE match_id = $1 AND board_number = $2`,
		matchID, boardNumber, gameID)
	return err
}

// RecordBoardResult scores one board and finishes the match once every board
// has a result.
func (s *Service) RecordBoardResult(ctx context.Context, matchID uuid.UUID, boardNumber int, winner string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var existing *string
	if err := tx.QueryRow(ctx, `
		SELECT result FROM club_team_boards
		WHERE match_id = $1 AND board_number = $2 FOR UPDATE`,
		matchID, boardNumber).Scan(&existing); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if existing != nil {
		return nil // already scored
	}

	if _, err := tx.Exec(ctx,
		`UPDATE club_team_boards SET result = $3 WHERE match_id = $1 AND board_number = $2`,
		matchID, boardNumber, winner); err != nil {
		return err
	}

	// The home player is black on odd boards and white on even ones, so a
	// club is not systematically advantaged by colour.
	homeIsBlack := boardNumber%2 == 1
	var homeDelta, awayDelta float64
	switch winner {
	case "black":
		if homeIsBlack {
			homeDelta = 1
		} else {
			awayDelta = 1
		}
	case "white":
		if homeIsBlack {
			awayDelta = 1
		} else {
			homeDelta = 1
		}
	default:
		homeDelta, awayDelta = 0.5, 0.5
	}
	if _, err := tx.Exec(ctx, `
		UPDATE club_team_matches
		SET home_score = home_score + $2, away_score = away_score + $3
		WHERE id = $1`, matchID, homeDelta, awayDelta); err != nil {
		return err
	}

	var remaining int
	if err := tx.QueryRow(ctx,
		`SELECT count(*) FROM club_team_boards WHERE match_id = $1 AND result IS NULL`,
		matchID).Scan(&remaining); err != nil {
		return err
	}
	if remaining == 0 {
		if _, err := tx.Exec(ctx,
			`UPDATE club_team_matches SET status = 'finished' WHERE id = $1`, matchID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// GetTeamMatch loads a match with its boards.
func (s *Service) GetTeamMatch(ctx context.Context, matchID uuid.UUID) (*TeamMatch, error) {
	var m TeamMatch
	err := s.db.QueryRow(ctx, `
		SELECT tm.id, tm.home_club_id, tm.away_club_id, hc.name, ac.name,
		       tm.board_count, tm.board_size, tm.time_control, tm.status,
		       tm.home_score, tm.away_score, tm.scheduled_at
		FROM club_team_matches tm
		JOIN clubs hc ON hc.id = tm.home_club_id
		JOIN clubs ac ON ac.id = tm.away_club_id
		WHERE tm.id = $1`, matchID,
	).Scan(&m.ID, &m.HomeClubID, &m.AwayClubID, &m.HomeName, &m.AwayName,
		&m.BoardCount, &m.BoardSize, &m.TimeControl, &m.Status,
		&m.HomeScore, &m.AwayScore, &m.ScheduledAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get team match: %w", err)
	}

	rows, err := s.db.Query(ctx, `
		SELECT b.board_number, b.home_user_id, b.away_user_id,
		       COALESCE(hu.display_name,''), COALESCE(au.display_name,''),
		       b.game_id, b.result
		FROM club_team_boards b
		LEFT JOIN users hu ON hu.id = b.home_user_id
		LEFT JOIN users au ON au.id = b.away_user_id
		WHERE b.match_id = $1 ORDER BY b.board_number`, matchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var b Board
		if err := rows.Scan(&b.BoardNumber, &b.HomeUserID, &b.AwayUserID,
			&b.HomeName, &b.AwayName, &b.GameID, &b.Result); err != nil {
			return nil, err
		}
		m.Boards = append(m.Boards, b)
	}
	return &m, rows.Err()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
