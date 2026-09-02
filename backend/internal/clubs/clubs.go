// Package clubs implements F1: clubs, roles, team matches and forums.
package clubs

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Errors returned by the service.
var (
	ErrNotFound  = errors.New("clubs: not found")
	ErrForbidden = errors.New("clubs: insufficient role")
	ErrNotOpen   = errors.New("clubs: club is invitation-only")
)

// Club is a community.
type Club struct {
	ID          uuid.UUID `json:"id"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	AvatarURL   *string   `json:"avatarUrl,omitempty"`
	Country     *string   `json:"country,omitempty"`
	Visibility  string    `json:"visibility"`
	MemberCount int       `json:"memberCount"`
	CreatedAt   time.Time `json:"createdAt"`
	// MyRole is the caller's role, empty when they are not a member.
	MyRole string `json:"myRole,omitempty"`
}

// Member is a club member.
type Member struct {
	UserID      uuid.UUID `json:"userId"`
	DisplayName string    `json:"displayName"`
	Rating      int       `json:"rating"`
	Role        string    `json:"role"`
	JoinedAt    time.Time `json:"joinedAt"`
}

// Thread is a forum topic.
type Thread struct {
	ID         uuid.UUID  `json:"id"`
	Title      string     `json:"title"`
	AuthorID   *uuid.UUID `json:"authorId,omitempty"`
	AuthorName string     `json:"authorName,omitempty"`
	Pinned     bool       `json:"pinned"`
	Locked     bool       `json:"locked"`
	PostCount  int        `json:"postCount"`
	CreatedAt  time.Time  `json:"createdAt"`
	LastPostAt time.Time  `json:"lastPostAt"`
}

// Post is a forum reply.
type Post struct {
	ID         int64      `json:"id"`
	AuthorID   *uuid.UUID `json:"authorId,omitempty"`
	AuthorName string     `json:"authorName,omitempty"`
	Body       string     `json:"body"`
	CreatedAt  time.Time  `json:"createdAt"`
}

// Service manages clubs.
type Service struct{ db *pgxpool.Pool }

// NewService builds the clubs service.
func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

var slugPattern = regexp.MustCompile(`[^a-z0-9]+`)

// slugify turns a club name into a URL-safe identifier.
func slugify(name string) string {
	s := slugPattern.ReplaceAllString(strings.ToLower(name), "-")
	s = strings.Trim(s, "-")
	if len(s) > 48 {
		s = s[:48]
	}
	if s == "" {
		s = "club"
	}
	return s
}

// Create opens a club with the caller as owner.
func (s *Service) Create(ctx context.Context, ownerID uuid.UUID, name, description string) (*Club, error) {
	name = strings.TrimSpace(name)
	if len([]rune(name)) < 3 || len([]rune(name)) > 60 {
		return nil, fmt.Errorf("club name must be 3-60 characters")
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	base := slugify(name)
	var c Club
	// Suffix the slug on collision rather than rejecting the name; two clubs
	// may legitimately want the same one.
	for attempt := 0; attempt < 8; attempt++ {
		slug := base
		if attempt > 0 {
			slug = fmt.Sprintf("%s-%d", base, attempt+1)
		}
		err = tx.QueryRow(ctx, `
			INSERT INTO clubs (slug, name, description, created_by, member_count)
			VALUES ($1,$2,NULLIF($3,''),$4,1)
			RETURNING id, slug, name, description, avatar_url, country,
			          visibility, member_count, created_at`,
			slug, name, description, ownerID,
		).Scan(&c.ID, &c.Slug, &c.Name, &c.Description, &c.AvatarURL, &c.Country,
			&c.Visibility, &c.MemberCount, &c.CreatedAt)
		if err == nil {
			break
		}
		if !strings.Contains(err.Error(), "23505") && !strings.Contains(err.Error(), "duplicate") {
			return nil, fmt.Errorf("create club: %w", err)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("create club: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO club_members (club_id, user_id, role) VALUES ($1,$2,'owner')`,
		c.ID, ownerID); err != nil {
		return nil, fmt.Errorf("seat owner: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	c.MyRole = "owner"
	return &c, nil
}

// Get returns a club by slug, including the caller's role.
func (s *Service) Get(ctx context.Context, slug string, viewerID uuid.UUID) (*Club, error) {
	var c Club
	var role *string
	err := s.db.QueryRow(ctx, `
		SELECT c.id, c.slug, c.name, c.description, c.avatar_url, c.country,
		       c.visibility, c.member_count, c.created_at, m.role
		FROM clubs c
		LEFT JOIN club_members m ON m.club_id = c.id AND m.user_id = $2
		WHERE c.slug = $1`, slug, viewerID,
	).Scan(&c.ID, &c.Slug, &c.Name, &c.Description, &c.AvatarURL, &c.Country,
		&c.Visibility, &c.MemberCount, &c.CreatedAt, &role)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get club: %w", err)
	}
	if role != nil {
		c.MyRole = *role
	}
	return &c, nil
}

// List returns clubs, most popular first.
func (s *Service) List(ctx context.Context, query string, limit int) ([]Club, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	sql := `SELECT id, slug, name, description, avatar_url, country, visibility,
	               member_count, created_at FROM clubs`
	args := []any{limit}
	if query != "" {
		sql += ` WHERE name ILIKE '%' || $2 || '%'`
		args = append(args, query)
	}
	sql += ` ORDER BY member_count DESC, created_at DESC LIMIT $1`

	rows, err := s.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("list clubs: %w", err)
	}
	defer rows.Close()
	out := []Club{}
	for rows.Next() {
		var c Club
		if err := rows.Scan(&c.ID, &c.Slug, &c.Name, &c.Description, &c.AvatarURL,
			&c.Country, &c.Visibility, &c.MemberCount, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Join adds the caller to an open club, or accepts a pending invitation to a
// closed one.
func (s *Service) Join(ctx context.Context, clubID, userID uuid.UUID) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var visibility string
	if err := tx.QueryRow(ctx,
		`SELECT visibility FROM clubs WHERE id = $1 FOR UPDATE`, clubID).Scan(&visibility); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if visibility == "closed" {
		var invited bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (SELECT 1 FROM club_invitations
			               WHERE club_id = $1 AND user_id = $2 AND status = 'pending')`,
			clubID, userID).Scan(&invited); err != nil {
			return err
		}
		if !invited {
			return ErrNotOpen
		}
		if _, err := tx.Exec(ctx, `
			UPDATE club_invitations SET status = 'accepted'
			WHERE club_id = $1 AND user_id = $2`, clubID, userID); err != nil {
			return err
		}
	}

	tag, err := tx.Exec(ctx, `
		INSERT INTO club_members (club_id, user_id) VALUES ($1,$2)
		ON CONFLICT DO NOTHING`, clubID, userID)
	if err != nil {
		return fmt.Errorf("join club: %w", err)
	}
	// Only bump the counter on an actual insert, so re-joining is idempotent.
	if tag.RowsAffected() > 0 {
		if _, err := tx.Exec(ctx,
			`UPDATE clubs SET member_count = member_count + 1 WHERE id = $1`, clubID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// Leave removes a member. The last owner may not leave, or the club would be
// left with nobody able to administer it.
func (s *Service) Leave(ctx context.Context, clubID, userID uuid.UUID) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var role string
	if err := tx.QueryRow(ctx,
		`SELECT role FROM club_members WHERE club_id = $1 AND user_id = $2 FOR UPDATE`,
		clubID, userID).Scan(&role); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if role == "owner" {
		var owners int
		if err := tx.QueryRow(ctx,
			`SELECT count(*) FROM club_members WHERE club_id = $1 AND role = 'owner'`,
			clubID).Scan(&owners); err != nil {
			return err
		}
		if owners <= 1 {
			return errors.New("clubs: promote another owner before leaving")
		}
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM club_members WHERE club_id = $1 AND user_id = $2`, clubID, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE clubs SET member_count = GREATEST(0, member_count - 1) WHERE id = $1`,
		clubID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Members lists a club's roster, strongest first.
func (s *Service) Members(ctx context.Context, clubID uuid.UUID, limit int) ([]Member, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.Query(ctx, `
		SELECT m.user_id, u.display_name, u.rating, m.role, m.joined_at
		FROM club_members m JOIN users u ON u.id = m.user_id
		WHERE m.club_id = $1
		ORDER BY u.rating DESC
		LIMIT $2`, clubID, limit)
	if err != nil {
		return nil, fmt.Errorf("club members: %w", err)
	}
	defer rows.Close()
	out := []Member{}
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.UserID, &m.DisplayName, &m.Rating, &m.Role, &m.JoinedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// SetRole changes a member's role. Only an owner may appoint or demote.
func (s *Service) SetRole(ctx context.Context, clubID, actorID, targetID uuid.UUID, role string) error {
	switch role {
	case "owner", "admin", "member":
	default:
		return fmt.Errorf("unsupported role %q", role)
	}
	actorRole, err := s.roleOf(ctx, clubID, actorID)
	if err != nil {
		return err
	}
	if actorRole != "owner" {
		return ErrForbidden
	}
	tag, err := s.db.Exec(ctx,
		`UPDATE club_members SET role = $3 WHERE club_id = $1 AND user_id = $2`,
		clubID, targetID, role)
	if err != nil {
		return fmt.Errorf("set role: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Invite records an invitation to a closed club. Owners and admins may invite.
func (s *Service) Invite(ctx context.Context, clubID, actorID, targetID uuid.UUID) error {
	role, err := s.roleOf(ctx, clubID, actorID)
	if err != nil {
		return err
	}
	if role != "owner" && role != "admin" {
		return ErrForbidden
	}
	_, err = s.db.Exec(ctx, `
		INSERT INTO club_invitations (club_id, user_id, invited_by)
		VALUES ($1,$2,$3)
		ON CONFLICT (club_id, user_id) DO UPDATE SET status = 'pending', created_at = now()`,
		clubID, targetID, actorID)
	return err
}

func (s *Service) roleOf(ctx context.Context, clubID, userID uuid.UUID) (string, error) {
	var role string
	err := s.db.QueryRow(ctx,
		`SELECT role FROM club_members WHERE club_id = $1 AND user_id = $2`,
		clubID, userID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrForbidden
	}
	if err != nil {
		return "", fmt.Errorf("role lookup: %w", err)
	}
	return role, nil
}

// --- forum ---

// Threads lists a club's forum, pinned first.
func (s *Service) Threads(ctx context.Context, clubID uuid.UUID, limit int) ([]Thread, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	rows, err := s.db.Query(ctx, `
		SELECT t.id, t.title, t.author_id, COALESCE(u.display_name, ''), t.pinned,
		       t.locked, t.post_count, t.created_at, t.last_post_at
		FROM club_threads t LEFT JOIN users u ON u.id = t.author_id
		WHERE t.club_id = $1
		ORDER BY t.pinned DESC, t.last_post_at DESC
		LIMIT $2`, clubID, limit)
	if err != nil {
		return nil, fmt.Errorf("threads: %w", err)
	}
	defer rows.Close()
	out := []Thread{}
	for rows.Next() {
		var t Thread
		if err := rows.Scan(&t.ID, &t.Title, &t.AuthorID, &t.AuthorName, &t.Pinned,
			&t.Locked, &t.PostCount, &t.CreatedAt, &t.LastPostAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// CreateThread opens a topic. Members only.
func (s *Service) CreateThread(ctx context.Context, clubID, authorID uuid.UUID, title, body string) (*Thread, error) {
	if _, err := s.roleOf(ctx, clubID, authorID); err != nil {
		return nil, err
	}
	title = strings.TrimSpace(title)
	if title == "" || len([]rune(title)) > 140 {
		return nil, errors.New("clubs: title must be 1-140 characters")
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var t Thread
	if err := tx.QueryRow(ctx, `
		INSERT INTO club_threads (club_id, author_id, title, post_count)
		VALUES ($1,$2,$3,1)
		RETURNING id, title, author_id, pinned, locked, post_count, created_at, last_post_at`,
		clubID, authorID, title,
	).Scan(&t.ID, &t.Title, &t.AuthorID, &t.Pinned, &t.Locked, &t.PostCount,
		&t.CreatedAt, &t.LastPostAt); err != nil {
		return nil, fmt.Errorf("create thread: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO club_thread_posts (thread_id, author_id, body) VALUES ($1,$2,$3)`,
		t.ID, authorID, body); err != nil {
		return nil, fmt.Errorf("create first post: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &t, nil
}

// Reply adds a post to a thread.
func (s *Service) Reply(ctx context.Context, threadID, authorID uuid.UUID, body string) error {
	body = strings.TrimSpace(body)
	if body == "" || len([]rune(body)) > 10000 {
		return errors.New("clubs: post must be 1-10000 characters")
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var clubID uuid.UUID
	var locked bool
	if err := tx.QueryRow(ctx,
		`SELECT club_id, locked FROM club_threads WHERE id = $1`, threadID).Scan(&clubID, &locked); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if locked {
		return errors.New("clubs: thread is locked")
	}
	var member bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM club_members WHERE club_id = $1 AND user_id = $2)`,
		clubID, authorID).Scan(&member); err != nil {
		return err
	}
	if !member {
		return ErrForbidden
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO club_thread_posts (thread_id, author_id, body) VALUES ($1,$2,$3)`,
		threadID, authorID, body); err != nil {
		return fmt.Errorf("reply: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`UPDATE club_threads SET post_count = post_count + 1, last_post_at = now() WHERE id = $1`,
		threadID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Posts returns a thread's replies oldest first.
func (s *Service) Posts(ctx context.Context, threadID uuid.UUID, limit int) ([]Post, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.Query(ctx, `
		SELECT p.id, p.author_id, COALESCE(u.display_name,''), p.body, p.created_at
		FROM club_thread_posts p LEFT JOIN users u ON u.id = p.author_id
		WHERE p.thread_id = $1 AND p.deleted_at IS NULL
		ORDER BY p.created_at
		LIMIT $2`, threadID, limit)
	if err != nil {
		return nil, fmt.Errorf("posts: %w", err)
	}
	defer rows.Close()
	out := []Post{}
	for rows.Next() {
		var p Post
		if err := rows.Scan(&p.ID, &p.AuthorID, &p.AuthorName, &p.Body, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
