-- F1: clubs, team matches and forums.

CREATE TABLE clubs (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    slug         TEXT        NOT NULL UNIQUE,
    name         TEXT        NOT NULL,
    description  TEXT,
    avatar_url   TEXT,
    country      TEXT,
    -- Open clubs anyone may join; closed clubs need an invitation.
    visibility   TEXT        NOT NULL DEFAULT 'open'
                 CHECK (visibility IN ('open','closed')),
    member_count INTEGER     NOT NULL DEFAULT 0,
    created_by   UUID        REFERENCES users(id) ON DELETE SET NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX clubs_popular_idx ON clubs (member_count DESC);

CREATE TABLE club_members (
    club_id   UUID        NOT NULL REFERENCES clubs(id) ON DELETE CASCADE,
    user_id   UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role      TEXT        NOT NULL DEFAULT 'member'
              CHECK (role IN ('owner','admin','member')),
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (club_id, user_id)
);

CREATE INDEX club_members_user_idx ON club_members (user_id);

CREATE TABLE club_invitations (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    club_id    UUID        NOT NULL REFERENCES clubs(id) ON DELETE CASCADE,
    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    invited_by UUID        REFERENCES users(id) ON DELETE SET NULL,
    status     TEXT        NOT NULL DEFAULT 'pending'
               CHECK (status IN ('pending','accepted','declined')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (club_id, user_id)
);

-- A team match is N boards played simultaneously between two clubs.
CREATE TABLE club_team_matches (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    home_club_id UUID        NOT NULL REFERENCES clubs(id) ON DELETE CASCADE,
    away_club_id UUID        NOT NULL REFERENCES clubs(id) ON DELETE CASCADE,
    board_count  SMALLINT    NOT NULL DEFAULT 5,
    board_size   SMALLINT    NOT NULL DEFAULT 19,
    time_control JSONB       NOT NULL,
    status       TEXT        NOT NULL DEFAULT 'proposed'
                 CHECK (status IN ('proposed','accepted','running','finished','cancelled')),
    home_score   REAL        NOT NULL DEFAULT 0,
    away_score   REAL        NOT NULL DEFAULT 0,
    scheduled_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT club_team_matches_distinct CHECK (home_club_id <> away_club_id)
);

-- One row per board. Players are seated by club rating order, so board 1 is
-- each club's strongest available player.
CREATE TABLE club_team_boards (
    match_id      UUID       NOT NULL REFERENCES club_team_matches(id) ON DELETE CASCADE,
    board_number  SMALLINT   NOT NULL,
    home_user_id  UUID       REFERENCES users(id) ON DELETE SET NULL,
    away_user_id  UUID       REFERENCES users(id) ON DELETE SET NULL,
    game_id       UUID       REFERENCES games(id) ON DELETE SET NULL,
    result        TEXT,
    PRIMARY KEY (match_id, board_number)
);

CREATE TABLE club_threads (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    club_id    UUID        NOT NULL REFERENCES clubs(id) ON DELETE CASCADE,
    author_id  UUID        REFERENCES users(id) ON DELETE SET NULL,
    title      TEXT        NOT NULL,
    pinned     BOOLEAN     NOT NULL DEFAULT FALSE,
    locked     BOOLEAN     NOT NULL DEFAULT FALSE,
    post_count INTEGER     NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_post_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX club_threads_recent_idx ON club_threads (club_id, pinned DESC, last_post_at DESC);

CREATE TABLE club_thread_posts (
    id         BIGSERIAL   PRIMARY KEY,
    thread_id  UUID        NOT NULL REFERENCES club_threads(id) ON DELETE CASCADE,
    author_id  UUID        REFERENCES users(id) ON DELETE SET NULL,
    body       TEXT        NOT NULL,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX club_thread_posts_thread_idx ON club_thread_posts (thread_id, created_at);

-- F2: friends graph, DMs and the activity feed.

-- Asymmetric follow-style edges, as F2 specifies: following someone does not
-- require their consent, and mutual follows are what the UI calls friends.
CREATE TABLE friendships (
    follower_id UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    followee_id UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (follower_id, followee_id),
    CONSTRAINT friendships_no_self CHECK (follower_id <> followee_id)
);

CREATE INDEX friendships_followee_idx ON friendships (followee_id);

CREATE TABLE blocks (
    blocker_id UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    blocked_id UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (blocker_id, blocked_id),
    CONSTRAINT blocks_no_self CHECK (blocker_id <> blocked_id)
);

CREATE TABLE direct_messages (
    id          BIGSERIAL   PRIMARY KEY,
    sender_id   UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    recipient_id UUID       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    body        TEXT        NOT NULL,
    read_at     TIMESTAMPTZ,
    -- Soft delete per side: deleting a message hides it for that user only.
    deleted_by_sender_at    TIMESTAMPTZ,
    deleted_by_recipient_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Conversation view: both directions between a pair, newest first.
CREATE INDEX direct_messages_pair_idx
    ON direct_messages (LEAST(sender_id, recipient_id), GREATEST(sender_id, recipient_id), created_at DESC);
CREATE INDEX direct_messages_unread_idx
    ON direct_messages (recipient_id) WHERE read_at IS NULL;

CREATE TABLE reports (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    reporter_id  UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    subject_id   UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind         TEXT        NOT NULL CHECK (kind IN ('message','chat','profile','conduct')),
    reference_id TEXT,
    reason       TEXT        NOT NULL,
    status       TEXT        NOT NULL DEFAULT 'open'
                 CHECK (status IN ('open','reviewing','actioned','dismissed')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX reports_queue_idx ON reports (status, created_at);

-- Activity feed read model, assembled from games, puzzles and achievements.
CREATE TABLE activity_events (
    id         BIGSERIAL   PRIMARY KEY,
    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind       TEXT        NOT NULL,
    payload    JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX activity_events_user_idx ON activity_events (user_id, created_at DESC);
