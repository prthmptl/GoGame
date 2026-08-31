// Package profile implements C3: cloud profile sync, achievements and
// leaderboards.
package profile

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
)

// ErrNotFound is returned when a user or record does not exist.
var ErrNotFound = errors.New("profile: not found")

// Profile is the user-facing profile document.
type Profile struct {
	ID          uuid.UUID `json:"id"`
	DisplayName string    `json:"displayName"`
	AvatarURL   *string   `json:"avatarUrl,omitempty"`
	Country     *string   `json:"country,omitempty"`
	Bio         *string   `json:"bio,omitempty"`
	Rating      int       `json:"rating"`
	IsGuest     bool      `json:"isGuest"`
	Revision    int64     `json:"revision"`
	CreatedAt   time.Time `json:"createdAt"`
}

// PatchRequest is the body of PATCH /users/me. Nil fields are left unchanged,
// which is what distinguishes "not supplied" from "set to empty".
type PatchRequest struct {
	DisplayName *string `json:"displayName,omitempty"`
	AvatarURL   *string `json:"avatarUrl,omitempty"`
	Country     *string `json:"country,omitempty"`
	Bio         *string `json:"bio,omitempty"`
}

// ProgressEntry is one synced progression record.
type ProgressEntry struct {
	Kind      string          `json:"kind"`
	ItemID    string          `json:"itemId"`
	Payload   json.RawMessage `json:"payload"`
	Revision  int64           `json:"revision"`
	UpdatedAt time.Time       `json:"updatedAt"`
}

// SyncRequest is the client's delta push. `Since` scopes the response to
// records changed after that instant so the client does not re-download its
// whole history on every sync.
type SyncRequest struct {
	Entries []ProgressEntry `json:"entries"`
	Streak  *StreakPayload  `json:"streak,omitempty"`
	Since   *time.Time      `json:"since,omitempty"`
}

// StreakPayload mirrors PuzzleRepo's streak counters.
type StreakPayload struct {
	Current      int     `json:"current"`
	Best         int     `json:"best"`
	LastSolvedOn *string `json:"lastSolvedOn,omitempty"` // YYYY-MM-DD
	Revision     int64   `json:"revision"`
}

// SyncResponse returns whatever the server holds that the client does not.
type SyncResponse struct {
	Accepted  int             `json:"accepted"`
	Conflicts []ProgressEntry `json:"conflicts"`
	Entries   []ProgressEntry `json:"entries"`
	Streak    *StreakPayload  `json:"streak,omitempty"`
	SyncedAt  time.Time       `json:"syncedAt"`
}

// Achievement is a catalogue entry, optionally with this user's progress.
type Achievement struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Icon        *string    `json:"icon,omitempty"`
	Hidden      bool       `json:"hidden"`
	Earned      bool       `json:"earned"`
	Progress    int        `json:"progress"`
	EarnedAt    *time.Time `json:"earnedAt,omitempty"`
}

// LeaderboardRow is one ranked player.
type LeaderboardRow struct {
	UserID      uuid.UUID `json:"userId"`
	DisplayName string    `json:"displayName"`
	Country     *string   `json:"country,omitempty"`
	AvatarURL   *string   `json:"avatarUrl,omitempty"`
	Rating      int       `json:"rating"`
	Rank        int64     `json:"rank"`
}

// Service implements the C3 endpoints.
type Service struct{ db *pgxpool.Pool }

// NewService builds the profile service.
func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

// Get returns a user's profile.
func (s *Service) Get(ctx context.Context, userID uuid.UUID) (*Profile, error) {
	var p Profile
	err := s.db.QueryRow(ctx, `
		SELECT id, display_name, avatar_url, country, bio, rating, is_guest,
		       profile_revision, created_at
		FROM users WHERE id = $1 AND deleted_at IS NULL`, userID,
	).Scan(&p.ID, &p.DisplayName, &p.AvatarURL, &p.Country, &p.Bio, &p.Rating,
		&p.IsGuest, &p.Revision, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get profile: %w", err)
	}
	return &p, nil
}

// Patch applies a partial profile update and bumps the revision.
func (s *Service) Patch(ctx context.Context, userID uuid.UUID, req PatchRequest) (*Profile, error) {
	if req.DisplayName != nil {
		name := strings.TrimSpace(*req.DisplayName)
		if name == "" {
			return nil, fmt.Errorf("displayName cannot be empty")
		}
		if len([]rune(name)) > 32 {
			name = string([]rune(name)[:32])
		}
		req.DisplayName = &name
	}
	if req.Bio != nil && len([]rune(*req.Bio)) > 280 {
		trimmed := string([]rune(*req.Bio)[:280])
		req.Bio = &trimmed
	}

	var p Profile
	// COALESCE keeps unsupplied fields untouched in a single statement.
	err := s.db.QueryRow(ctx, `
		UPDATE users SET
			display_name     = COALESCE($2, display_name),
			avatar_url       = COALESCE($3, avatar_url),
			country          = COALESCE($4, country),
			bio              = COALESCE($5, bio),
			profile_revision = profile_revision + 1,
			updated_at       = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, display_name, avatar_url, country, bio, rating, is_guest,
		          profile_revision, created_at`,
		userID, req.DisplayName, req.AvatarURL, req.Country, req.Bio,
	).Scan(&p.ID, &p.DisplayName, &p.AvatarURL, &p.Country, &p.Bio, &p.Rating,
		&p.IsGuest, &p.Revision, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("patch profile: %w", err)
	}
	return &p, nil
}

// Sync merges a client's progression deltas and returns anything newer held
// by the server.
//
// Conflicts resolve on `revision`, not wall-clock time: a client's device
// clock can be wrong by hours, but its own per-record counter only ever
// increases. A push whose revision is not greater than the stored one is
// rejected and echoed back in Conflicts so the client can reconcile.
func (s *Service) Sync(ctx context.Context, userID uuid.UUID, req SyncRequest) (*SyncResponse, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	resp := &SyncResponse{Conflicts: []ProgressEntry{}, Entries: []ProgressEntry{}}

	for _, e := range req.Entries {
		switch e.Kind {
		case "puzzle", "drill", "lesson":
		default:
			return nil, fmt.Errorf("unknown progress kind %q", e.Kind)
		}
		payload := e.Payload
		if len(payload) == 0 {
			payload = json.RawMessage(`{}`)
		}
		tag, err := tx.Exec(ctx, `
			INSERT INTO progress_entries (user_id, kind, item_id, payload, revision, updated_at)
			VALUES ($1, $2, $3, $4, $5, now())
			ON CONFLICT (user_id, kind, item_id) DO UPDATE
			SET payload = EXCLUDED.payload,
			    revision = EXCLUDED.revision,
			    updated_at = now()
			WHERE progress_entries.revision < EXCLUDED.revision`,
			userID, e.Kind, e.ItemID, payload, e.Revision)
		if err != nil {
			return nil, fmt.Errorf("upsert progress: %w", err)
		}
		if tag.RowsAffected() == 0 {
			resp.Conflicts = append(resp.Conflicts, e)
		} else {
			resp.Accepted++
		}
	}

	if req.Streak != nil {
		if _, err := tx.Exec(ctx, `
			INSERT INTO user_streaks (user_id, current_streak, best_streak, last_solved_on, revision, updated_at)
			VALUES ($1, $2, $3, $4::date, $5, now())
			ON CONFLICT (user_id) DO UPDATE
			SET current_streak = EXCLUDED.current_streak,
			    -- best_streak must never regress, even if a stale device wins
			    -- the revision race on the other columns.
			    best_streak = GREATEST(user_streaks.best_streak, EXCLUDED.best_streak),
			    last_solved_on = EXCLUDED.last_solved_on,
			    revision = EXCLUDED.revision,
			    updated_at = now()
			WHERE user_streaks.revision < EXCLUDED.revision`,
			userID, req.Streak.Current, req.Streak.Best, req.Streak.LastSolvedOn, req.Streak.Revision,
		); err != nil {
			return nil, fmt.Errorf("upsert streak: %w", err)
		}
	}

	// Return server-side records the client has not seen.
	since := time.Unix(0, 0)
	if req.Since != nil {
		since = *req.Since
	}
	rows, err := tx.Query(ctx, `
		SELECT kind, item_id, payload, revision, updated_at
		FROM progress_entries
		WHERE user_id = $1 AND updated_at > $2
		ORDER BY updated_at ASC
		LIMIT 1000`, userID, since)
	if err != nil {
		return nil, fmt.Errorf("query progress: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var e ProgressEntry
		if err := rows.Scan(&e.Kind, &e.ItemID, &e.Payload, &e.Revision, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan progress: %w", err)
		}
		resp.Entries = append(resp.Entries, e)
	}
	rows.Close()

	var st StreakPayload
	var lastSolved *time.Time
	err = tx.QueryRow(ctx, `
		SELECT current_streak, best_streak, last_solved_on, revision
		FROM user_streaks WHERE user_id = $1`, userID,
	).Scan(&st.Current, &st.Best, &lastSolved, &st.Revision)
	if err == nil {
		if lastSolved != nil {
			d := lastSolved.Format("2006-01-02")
			st.LastSolvedOn = &d
		}
		resp.Streak = &st
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("query streak: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	resp.SyncedAt = time.Now().UTC()
	return resp, nil
}

// Achievements lists the catalogue with this user's progress merged in.
// Hidden achievements appear only once earned.
func (s *Service) Achievements(ctx context.Context, userID uuid.UUID) ([]Achievement, error) {
	rows, err := s.db.Query(ctx, `
		SELECT a.id, a.title, a.description, a.icon, a.hidden,
		       ua.user_id IS NOT NULL AS earned,
		       COALESCE(ua.progress, 0), ua.earned_at
		FROM achievements a
		LEFT JOIN user_achievements ua
		       ON ua.achievement_id = a.id AND ua.user_id = $1
		WHERE a.hidden = FALSE OR ua.user_id IS NOT NULL
		ORDER BY a.sort_order, a.id`, userID)
	if err != nil {
		return nil, fmt.Errorf("query achievements: %w", err)
	}
	defer rows.Close()

	out := []Achievement{}
	for rows.Next() {
		var a Achievement
		if err := rows.Scan(&a.ID, &a.Title, &a.Description, &a.Icon, &a.Hidden,
			&a.Earned, &a.Progress, &a.EarnedAt); err != nil {
			return nil, fmt.Errorf("scan achievement: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// Award grants an achievement, or advances progress toward it. It is
// idempotent: re-awarding never moves earned_at, so a replayed event does not
// reorder someone's achievement history.
func (s *Service) Award(ctx context.Context, userID uuid.UUID, achievementID string, progress int) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO user_achievements (user_id, achievement_id, progress)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, achievement_id) DO UPDATE
		SET progress = GREATEST(user_achievements.progress, EXCLUDED.progress)`,
		userID, achievementID, progress)
	if err != nil {
		return fmt.Errorf("award achievement: %w", err)
	}
	return nil
}

// Leaderboard returns a page of the global rankings from the materialized
// view. Keyset pagination on rank keeps deep pages cheap.
func (s *Service) Leaderboard(ctx context.Context, afterRank int64, limit int) ([]LeaderboardRow, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.Query(ctx, `
		SELECT user_id, display_name, country, avatar_url, rating, rank
		FROM leaderboard_global
		WHERE rank > $1
		ORDER BY rank
		LIMIT $2`, afterRank, limit)
	if err != nil {
		return nil, fmt.Errorf("query leaderboard: %w", err)
	}
	defer rows.Close()

	out := []LeaderboardRow{}
	for rows.Next() {
		var r LeaderboardRow
		if err := rows.Scan(&r.UserID, &r.DisplayName, &r.Country, &r.AvatarURL,
			&r.Rating, &r.Rank); err != nil {
			return nil, fmt.Errorf("scan leaderboard: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// RefreshLeaderboard rebuilds the materialized view. The worker calls this on
// a schedule. CONCURRENTLY keeps the view readable throughout, which is why
// the unique index on user_id exists.
func (s *Service) RefreshLeaderboard(ctx context.Context) error {
	if _, err := s.db.Exec(ctx, `REFRESH MATERIALIZED VIEW CONCURRENTLY leaderboard_global`); err != nil {
		// The first refresh after creation cannot be concurrent, because the
		// view was created WITH NO DATA and is not yet populated.
		if _, err2 := s.db.Exec(ctx, `REFRESH MATERIALIZED VIEW leaderboard_global`); err2 != nil {
			return fmt.Errorf("refresh leaderboard: %w (concurrent: %v)", err2, err)
		}
	}
	return nil
}
