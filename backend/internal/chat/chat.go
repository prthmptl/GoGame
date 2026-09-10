// Package chat implements D5's per-game chat: persistence, rate limiting and
// a profanity filter.
package chat

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// Errors returned to callers.
var (
	ErrRateLimited = errors.New("chat: too many messages")
	ErrTooLong     = errors.New("chat: message too long")
	ErrEmpty       = errors.New("chat: message is empty")
)

// Rate limit: a burst of 5 lines, then roughly one every 2 seconds.
const (
	rateWindow      = 10 * time.Second
	rateMaxInWindow = 5
	maxRunes        = 500
)

// Message is a stored chat line.
type Message struct {
	ID          int64     `json:"id"`
	GameID      uuid.UUID `json:"gameId"`
	UserID      uuid.UUID `json:"userId"`
	DisplayName string    `json:"displayName"`
	Body        string    `json:"body"`
	CreatedAt   time.Time `json:"createdAt"`
}

// Service handles chat.
type Service struct {
	db     *pgxpool.Pool
	rdb    *redis.Client
	filter *Filter
}

// NewService builds the chat service.
func NewService(db *pgxpool.Pool, rdb *redis.Client) *Service {
	return &Service{db: db, rdb: rdb, filter: NewFilter(nil)}
}

// Post validates, filters, rate-limits and stores one message.
func (s *Service) Post(ctx context.Context, gameID, userID uuid.UUID, body string) (*Message, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, ErrEmpty
	}
	if len([]rune(body)) > maxRunes {
		return nil, ErrTooLong
	}
	ok, err := s.allow(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrRateLimited
	}

	body = s.filter.Clean(body)

	var m Message
	err = s.db.QueryRow(ctx, `
		INSERT INTO chat_messages (game_id, user_id, body)
		VALUES ($1, $2, $3)
		RETURNING id, game_id, user_id, body, created_at`,
		gameID, userID, body,
	).Scan(&m.ID, &m.GameID, &m.UserID, &m.Body, &m.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("store chat: %w", err)
	}
	_ = s.db.QueryRow(ctx,
		`SELECT display_name FROM users WHERE id = $1`, userID).Scan(&m.DisplayName)
	return &m, nil
}

// allow implements a sliding-window rate limit in Redis. INCR plus an expiry
// on first use is cheap and needs no stored timestamps.
func (s *Service) allow(ctx context.Context, userID uuid.UUID) (bool, error) {
	key := "chat:rate:" + userID.String()
	n, err := s.rdb.Eval(ctx, `local n = redis.call("INCR", KEYS[1])
        if n == 1 then redis.call("PEXPIRE", KEYS[1], ARGV[1]) end return n`,
		[]string{key}, rateWindow.Milliseconds()).Int64()
	if err != nil {
		return false, err
	}
	return n <= rateMaxInWindow, nil
}

// History returns recent messages for a game, oldest first.
func (s *Service) History(ctx context.Context, gameID uuid.UUID, limit int) ([]Message, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.Query(ctx, `
		SELECT c.id, c.game_id, c.user_id, u.display_name, c.body, c.created_at
		FROM chat_messages c JOIN users u ON u.id = c.user_id
		WHERE c.game_id = $1 AND c.hidden_at IS NULL
		ORDER BY c.created_at DESC
		LIMIT $2`, gameID, limit)
	if err != nil {
		return nil, fmt.Errorf("chat history: %w", err)
	}
	defer rows.Close()

	out := []Message{}
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.GameID, &m.UserID, &m.DisplayName,
			&m.Body, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Query is newest-first for the LIMIT; callers want oldest-first.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

// Hide soft-deletes a message for moderation.
func (s *Service) Hide(ctx context.Context, id int64) error {
	_, err := s.db.Exec(ctx,
		`UPDATE chat_messages SET hidden_at = now() WHERE id = $1`, id)
	return err
}

// Filter masks banned words.
//
// This is a deliberately simple substring filter over a small word list. It
// catches casual abuse and nothing more; real trust-and-safety needs a
// managed service and human review, which is E2/F2 territory.
type Filter struct{ words []string }

// defaultWords is a minimal starter list. Operations should replace it with a
// curated, localised list loaded from configuration.
var defaultWords = []string{"fuck", "shit", "bitch", "cunt", "faggot", "nigger", "retard"}

// NewFilter builds a filter, using the default list when words is nil.
func NewFilter(words []string) *Filter {
	if words == nil {
		words = defaultWords
	}
	lower := make([]string, len(words))
	for i, w := range words {
		lower[i] = strings.ToLower(w)
	}
	return &Filter{words: lower}
}

// leetMap folds common character substitutions so "f0ck" and "sh!t" are
// caught alongside their plain spellings.
var leetMap = map[rune]rune{
	'0': 'o', '1': 'i', '3': 'e', '4': 'a', '5': 's', '7': 't',
	'@': 'a', '$': 's', '!': 'i', '|': 'i', '+': 't',
}

// normalizeToken reduces one token to comparable letters: lowercased, leet
// substitutions folded, everything else dropped. "f.u.c.k" and "F0ck" both
// become "fuck".
func normalizeToken(token string) string {
	var b strings.Builder
	for _, r := range token {
		if sub, ok := leetMap[r]; ok {
			b.WriteRune(sub)
			continue
		}
		if unicode.IsLetter(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

// Clean masks banned words with asterisks, preserving everything else.
//
// Matching is per whitespace-separated token and requires the whole
// normalised token to equal a banned word. Substring matching is deliberately
// avoided: it censors "pass" for containing "ass", which in a Go client is
// worse than letting the occasional insult through. Real coverage needs a
// managed moderation service with human review.
func (f *Filter) Clean(s string) string {
	if len(f.words) == 0 {
		return s
	}
	banned := make(map[string]struct{}, len(f.words))
	for _, w := range f.words {
		banned[normalizeToken(w)] = struct{}{}
	}

	var out strings.Builder
	out.Grow(len(s))
	token := strings.Builder{}

	flush := func() {
		if token.Len() == 0 {
			return
		}
		t := token.String()
		token.Reset()
		if _, bad := banned[normalizeToken(t)]; bad {
			out.WriteString(strings.Repeat("*", len([]rune(t))))
			return
		}
		out.WriteString(t)
	}

	for _, r := range s {
		if unicode.IsSpace(r) {
			flush()
			out.WriteRune(r)
			continue
		}
		token.WriteRune(r)
	}
	flush()
	return out.String()
}
