-- C3: cloud profile sync, achievements, leaderboards.
--
-- Every synced row carries updated_at and a client-supplied revision so the
-- client can push deltas instead of its whole progression set. The client's
-- stores are flat SharedPreferences keys today with no timestamps; the
-- `revision` column is what lets the two converge.

ALTER TABLE users
    ADD COLUMN bio               TEXT,
    ADD COLUMN profile_revision  BIGINT NOT NULL DEFAULT 0;

-- Per-user progression records, one row per (kind, item). Kinds: 'puzzle',
-- 'drill', 'lesson' — matching PuzzleRepo, DrillRepo and LessonRepo.
CREATE TABLE progress_entries (
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind        TEXT        NOT NULL CHECK (kind IN ('puzzle','drill','lesson')),
    item_id     TEXT        NOT NULL,

    -- Kind-specific payload: attempt status/mistakes/hints for puzzles, win
    -- counters for drills, completion for lessons. Keeping it as JSONB means
    -- adding a field to a client repo does not need a migration.
    payload     JSONB       NOT NULL DEFAULT '{}'::jsonb,

    -- Monotonic per-record counter from the client. Last-writer-wins is
    -- resolved on this, not wall-clock time, because device clocks drift.
    revision    BIGINT      NOT NULL DEFAULT 1,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (user_id, kind, item_id)
);

CREATE INDEX progress_entries_sync_idx ON progress_entries (user_id, updated_at DESC);

-- Streaks are per-user singletons rather than per-item, so they get their own
-- table instead of a progress_entries row.
CREATE TABLE user_streaks (
    user_id         UUID        PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    current_streak  INTEGER     NOT NULL DEFAULT 0,
    best_streak     INTEGER     NOT NULL DEFAULT 0,
    last_solved_on  DATE,
    revision        BIGINT      NOT NULL DEFAULT 1,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE achievements (
    id          TEXT        PRIMARY KEY,
    title       TEXT        NOT NULL,
    description TEXT        NOT NULL,
    icon        TEXT,
    -- Hidden achievements are not listed until earned.
    hidden      BOOLEAN     NOT NULL DEFAULT FALSE,
    sort_order  INTEGER     NOT NULL DEFAULT 0
);

CREATE TABLE user_achievements (
    user_id        UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    achievement_id TEXT        NOT NULL REFERENCES achievements(id) ON DELETE CASCADE,
    earned_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Progress toward multi-step achievements ("solve 100 puzzles").
    progress       INTEGER     NOT NULL DEFAULT 0,
    PRIMARY KEY (user_id, achievement_id)
);

CREATE INDEX user_achievements_earned_idx ON user_achievements (user_id, earned_at DESC);

-- Leaderboard read model. Refreshed on a schedule by the worker rather than
-- computed per request: at chess.com scale the top-N query is far too hot to
-- run against users directly.
CREATE MATERIALIZED VIEW leaderboard_global AS
SELECT
    u.id            AS user_id,
    u.display_name,
    u.country,
    u.avatar_url,
    u.rating,
    ROW_NUMBER() OVER (ORDER BY u.rating DESC, u.created_at ASC) AS rank
FROM users u
WHERE u.deleted_at IS NULL
  AND u.is_guest = FALSE
WITH NO DATA;

-- UNIQUE index is required for REFRESH MATERIALIZED VIEW CONCURRENTLY, which
-- is what keeps the leaderboard readable during a refresh.
CREATE UNIQUE INDEX leaderboard_global_user_idx ON leaderboard_global (user_id);
CREATE INDEX leaderboard_global_rank_idx ON leaderboard_global (rank);
