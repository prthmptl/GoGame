// Package social implements F2: the friends graph, direct messages, reports
// and the activity feed.
package social

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Errors returned by the service.
var (
	ErrNotFound = errors.New("social: not found")
	ErrBlocked  = errors.New("social: blocked")
	ErrSelf     = errors.New("social: cannot target yourself")
)

// Relation summarises how two users are connected.
type Relation struct {
	Following  bool `json:"following"`
	FollowedBy bool `json:"followedBy"`
	// Friends is the mutual case, which is what the UI calls a friend.
	Friends bool `json:"friends"`
	Blocked bool `json:"blocked"`
}

// Person is a user in a follow list.
type Person struct {
	UserID      uuid.UUID `json:"userId"`
	DisplayName string    `json:"displayName"`
	AvatarURL   *string   `json:"avatarUrl,omitempty"`
	Country     *string   `json:"country,omitempty"`
	Rating      int       `json:"rating"`
	Online      bool      `json:"online"`
}

// Message is a direct message.
type Message struct {
	ID          int64      `json:"id"`
	SenderID    uuid.UUID  `json:"senderId"`
	RecipientID uuid.UUID  `json:"recipientId"`
	Body        string     `json:"body"`
	ReadAt      *time.Time `json:"readAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
}

// ActivityEvent is one entry in the feed.
type ActivityEvent struct {
	ID          int64           `json:"id"`
	UserID      uuid.UUID       `json:"userId"`
	DisplayName string          `json:"displayName"`
	Kind        string          `json:"kind"`
	Payload     json.RawMessage `json:"payload"`
	CreatedAt   time.Time       `json:"createdAt"`
}

// Service manages the social graph.
type Service struct{ db *pgxpool.Pool }

// NewService builds the social service.
func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

// Follow creates a one-way edge. Following is asymmetric and needs no
// consent, per F2; blocking is what stops it.
func (s *Service) Follow(ctx context.Context, followerID, followeeID uuid.UUID) error {
	if followerID == followeeID {
		return ErrSelf
	}
	blocked, err := s.eitherBlocked(ctx, followerID, followeeID)
	if err != nil {
		return err
	}
	if blocked {
		return ErrBlocked
	}
	_, err = s.db.Exec(ctx, `
		INSERT INTO friendships (follower_id, followee_id) VALUES ($1,$2)
		ON CONFLICT DO NOTHING`, followerID, followeeID)
	if err != nil {
		return fmt.Errorf("follow: %w", err)
	}
	return nil
}

// Unfollow removes the edge.
func (s *Service) Unfollow(ctx context.Context, followerID, followeeID uuid.UUID) error {
	_, err := s.db.Exec(ctx,
		`DELETE FROM friendships WHERE follower_id = $1 AND followee_id = $2`,
		followerID, followeeID)
	return err
}

// Block cuts the connection in both directions and removes any follows, so a
// blocked user cannot keep seeing the blocker's activity.
func (s *Service) Block(ctx context.Context, blockerID, blockedID uuid.UUID) error {
	if blockerID == blockedID {
		return ErrSelf
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
		INSERT INTO blocks (blocker_id, blocked_id) VALUES ($1,$2)
		ON CONFLICT DO NOTHING`, blockerID, blockedID); err != nil {
		return fmt.Errorf("block: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM friendships
		WHERE (follower_id = $1 AND followee_id = $2)
		   OR (follower_id = $2 AND followee_id = $1)`,
		blockerID, blockedID); err != nil {
		return fmt.Errorf("clear follows: %w", err)
	}
	return tx.Commit(ctx)
}

// Unblock removes a block.
func (s *Service) Unblock(ctx context.Context, blockerID, blockedID uuid.UUID) error {
	_, err := s.db.Exec(ctx,
		`DELETE FROM blocks WHERE blocker_id = $1 AND blocked_id = $2`,
		blockerID, blockedID)
	return err
}

func (s *Service) eitherBlocked(ctx context.Context, a, b uuid.UUID) (bool, error) {
	var blocked bool
	err := s.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM blocks
			WHERE (blocker_id = $1 AND blocked_id = $2)
			   OR (blocker_id = $2 AND blocked_id = $1)
		)`, a, b).Scan(&blocked)
	return blocked, err
}

// RelationTo describes the caller's relationship with another user.
func (s *Service) RelationTo(ctx context.Context, viewerID, otherID uuid.UUID) (*Relation, error) {
	var r Relation
	err := s.db.QueryRow(ctx, `
		SELECT
			EXISTS (SELECT 1 FROM friendships WHERE follower_id = $1 AND followee_id = $2),
			EXISTS (SELECT 1 FROM friendships WHERE follower_id = $2 AND followee_id = $1),
			EXISTS (SELECT 1 FROM blocks WHERE blocker_id = $1 AND blocked_id = $2)`,
		viewerID, otherID).Scan(&r.Following, &r.FollowedBy, &r.Blocked)
	if err != nil {
		return nil, fmt.Errorf("relation: %w", err)
	}
	r.Friends = r.Following && r.FollowedBy
	return &r, nil
}

// Following lists who a user follows.
func (s *Service) Following(ctx context.Context, userID uuid.UUID, limit int) ([]Person, error) {
	return s.people(ctx, `
		SELECT u.id, u.display_name, u.avatar_url, u.country, u.rating
		FROM friendships f JOIN users u ON u.id = f.followee_id
		WHERE f.follower_id = $1 AND u.deleted_at IS NULL
		ORDER BY u.display_name LIMIT $2`, userID, limit)
}

// Followers lists who follows a user.
func (s *Service) Followers(ctx context.Context, userID uuid.UUID, limit int) ([]Person, error) {
	return s.people(ctx, `
		SELECT u.id, u.display_name, u.avatar_url, u.country, u.rating
		FROM friendships f JOIN users u ON u.id = f.follower_id
		WHERE f.followee_id = $1 AND u.deleted_at IS NULL
		ORDER BY u.display_name LIMIT $2`, userID, limit)
}

// Friends lists mutual follows.
func (s *Service) Friends(ctx context.Context, userID uuid.UUID, limit int) ([]Person, error) {
	return s.people(ctx, `
		SELECT u.id, u.display_name, u.avatar_url, u.country, u.rating
		FROM friendships a
		JOIN friendships b ON b.follower_id = a.followee_id AND b.followee_id = a.follower_id
		JOIN users u ON u.id = a.followee_id
		WHERE a.follower_id = $1 AND u.deleted_at IS NULL
		ORDER BY u.display_name LIMIT $2`, userID, limit)
}

func (s *Service) people(ctx context.Context, sql string, userID uuid.UUID, limit int) ([]Person, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.Query(ctx, sql, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list people: %w", err)
	}
	defer rows.Close()
	out := []Person{}
	for rows.Next() {
		var p Person
		if err := rows.Scan(&p.UserID, &p.DisplayName, &p.AvatarURL, &p.Country, &p.Rating); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// --- direct messages ---

// SendMessage delivers a DM. A block in either direction stops it, and only
// mutual follows may open a conversation — otherwise DMs become a spam
// channel the moment the user base is large enough to be worth spamming.
func (s *Service) SendMessage(ctx context.Context, senderID, recipientID uuid.UUID, body string) (*Message, error) {
	if senderID == recipientID {
		return nil, ErrSelf
	}
	body = strings.TrimSpace(body)
	if body == "" || len([]rune(body)) > 2000 {
		return nil, errors.New("social: message must be 1-2000 characters")
	}
	rel, err := s.RelationTo(ctx, senderID, recipientID)
	if err != nil {
		return nil, err
	}
	if rel.Blocked {
		return nil, ErrBlocked
	}
	blocked, err := s.eitherBlocked(ctx, senderID, recipientID)
	if err != nil {
		return nil, err
	}
	if blocked {
		return nil, ErrBlocked
	}
	if !rel.Friends {
		// Allow a reply to an existing conversation, so a first message can
		// be answered without having to follow back first.
		var existing bool
		if err := s.db.QueryRow(ctx, `
			SELECT EXISTS (SELECT 1 FROM direct_messages
			               WHERE sender_id = $2 AND recipient_id = $1)`,
			senderID, recipientID).Scan(&existing); err != nil {
			return nil, err
		}
		if !existing {
			return nil, errors.New("social: you can only message people who follow you back")
		}
	}

	var m Message
	err = s.db.QueryRow(ctx, `
		INSERT INTO direct_messages (sender_id, recipient_id, body)
		VALUES ($1,$2,$3)
		RETURNING id, sender_id, recipient_id, body, read_at, created_at`,
		senderID, recipientID, body,
	).Scan(&m.ID, &m.SenderID, &m.RecipientID, &m.Body, &m.ReadAt, &m.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("send message: %w", err)
	}
	return &m, nil
}

// Conversation returns the messages between two users, oldest first,
// excluding anything the viewer soft-deleted.
func (s *Service) Conversation(ctx context.Context, viewerID, otherID uuid.UUID, limit int) ([]Message, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.Query(ctx, `
		SELECT id, sender_id, recipient_id, body, read_at, created_at
		FROM direct_messages
		WHERE ((sender_id = $1 AND recipient_id = $2) OR (sender_id = $2 AND recipient_id = $1))
		  AND ((sender_id = $1 AND deleted_by_sender_at IS NULL)
		    OR (recipient_id = $1 AND deleted_by_recipient_at IS NULL))
		ORDER BY created_at DESC LIMIT $3`, viewerID, otherID, limit)
	if err != nil {
		return nil, fmt.Errorf("conversation: %w", err)
	}
	defer rows.Close()
	out := []Message{}
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.SenderID, &m.RecipientID, &m.Body,
			&m.ReadAt, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

// MarkRead marks every message from one sender as read.
func (s *Service) MarkRead(ctx context.Context, viewerID, otherID uuid.UUID) error {
	_, err := s.db.Exec(ctx, `
		UPDATE direct_messages SET read_at = now()
		WHERE recipient_id = $1 AND sender_id = $2 AND read_at IS NULL`,
		viewerID, otherID)
	return err
}

// DeleteMessage soft-deletes for the caller only, leaving the other side's
// copy intact — and the row available for a moderation review.
func (s *Service) DeleteMessage(ctx context.Context, viewerID uuid.UUID, messageID int64) error {
	tag, err := s.db.Exec(ctx, `
		UPDATE direct_messages
		SET deleted_by_sender_at = CASE WHEN sender_id = $2 THEN now() ELSE deleted_by_sender_at END,
		    deleted_by_recipient_at = CASE WHEN recipient_id = $2 THEN now() ELSE deleted_by_recipient_at END
		WHERE id = $1 AND (sender_id = $2 OR recipient_id = $2)`, messageID, viewerID)
	if err != nil {
		return fmt.Errorf("delete message: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UnreadCount reports how many unread DMs a user has.
func (s *Service) UnreadCount(ctx context.Context, userID uuid.UUID) (int, error) {
	var n int
	err := s.db.QueryRow(ctx, `
		SELECT count(*) FROM direct_messages
		WHERE recipient_id = $1 AND read_at IS NULL AND deleted_by_recipient_at IS NULL`,
		userID).Scan(&n)
	return n, err
}

// Report files a moderation report.
func (s *Service) Report(ctx context.Context, reporterID, subjectID uuid.UUID, kind, referenceID, reason string) error {
	if reporterID == subjectID {
		return ErrSelf
	}
	switch kind {
	case "message", "chat", "profile", "conduct":
	default:
		return fmt.Errorf("unsupported report kind %q", kind)
	}
	if strings.TrimSpace(reason) == "" {
		return errors.New("social: a reason is required")
	}
	_, err := s.db.Exec(ctx, `
		INSERT INTO reports (reporter_id, subject_id, kind, reference_id, reason)
		VALUES ($1,$2,$3,NULLIF($4,''),$5)`,
		reporterID, subjectID, kind, referenceID, reason)
	if err != nil {
		return fmt.Errorf("report: %w", err)
	}
	return nil
}

// --- activity feed ---

// RecordActivity appends an event to a user's timeline.
func (s *Service) RecordActivity(ctx context.Context, userID uuid.UUID, kind string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		raw = []byte(`{}`)
	}
	_, err = s.db.Exec(ctx,
		`INSERT INTO activity_events (user_id, kind, payload) VALUES ($1,$2,$3)`,
		userID, kind, raw)
	return err
}

// Feed returns the caller's timeline: their own activity plus everyone they
// follow, newest first.
func (s *Service) Feed(ctx context.Context, viewerID uuid.UUID, limit int) ([]ActivityEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	rows, err := s.db.Query(ctx, `
		SELECT e.id, e.user_id, u.display_name, e.kind, e.payload, e.created_at
		FROM activity_events e JOIN users u ON u.id = e.user_id
		WHERE e.user_id = $1
		   OR e.user_id IN (SELECT followee_id FROM friendships WHERE follower_id = $1)
		ORDER BY e.created_at DESC
		LIMIT $2`, viewerID, limit)
	if err != nil {
		return nil, fmt.Errorf("feed: %w", err)
	}
	defer rows.Close()
	out := []ActivityEvent{}
	for rows.Next() {
		var e ActivityEvent
		if err := rows.Scan(&e.ID, &e.UserID, &e.DisplayName, &e.Kind,
			&e.Payload, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
