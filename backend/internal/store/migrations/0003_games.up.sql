-- C4: games archive. Written by D2's game-session service; read by the
-- archive API. The SGF body lives in object storage, not Postgres — a 19x19
-- game is a few KB, but millions of them belong in S3 at a fraction of the
-- cost and with CDN caching in front.

CREATE TABLE games (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),

    black_user_id     UUID        REFERENCES users(id) ON DELETE SET NULL,
    white_user_id     UUID        REFERENCES users(id) ON DELETE SET NULL,
    -- Set instead of a user id when the opponent is a bot from bot_catalog.
    black_bot_id      TEXT,
    white_bot_id      TEXT,

    board_size        SMALLINT    NOT NULL,
    ruleset           TEXT        NOT NULL DEFAULT 'chinese',
    komi              REAL        NOT NULL,
    handicap          SMALLINT    NOT NULL DEFAULT 0,

    time_control      JSONB       NOT NULL DEFAULT '{}'::jsonb,
    -- 'blitz' | 'rapid' | 'classical' | 'daily'. Denormalised from
    -- time_control because E1 rates each class separately and the archive
    -- filters on it.
    time_class        TEXT        NOT NULL DEFAULT 'rapid',

    -- 'casual' | 'ranked' | 'friend' | 'tournament' | 'daily'
    mode              TEXT        NOT NULL DEFAULT 'casual',

    status            TEXT        NOT NULL DEFAULT 'active',
    -- SGF-style result: 'B+7.5', 'W+R', 'B+T', 'Draw', 'Void'.
    result            TEXT,
    winner            TEXT CHECK (winner IN ('black','white','draw') OR winner IS NULL),
    -- 'score' | 'resign' | 'timeout' | 'forfeit' | 'agreement'
    end_reason        TEXT,
    black_score       REAL,
    white_score       REAL,
    move_count        INTEGER     NOT NULL DEFAULT 0,

    -- Object storage key for the SGF; NULL until the game finishes and the
    -- worker uploads it.
    sgf_object_key    TEXT,

    started_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at          TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- A game is either between two users or involves a bot, but it always
    -- needs at least one real player on each side of the board.
    CONSTRAINT games_black_player CHECK (black_user_id IS NOT NULL OR black_bot_id IS NOT NULL),
    CONSTRAINT games_white_player CHECK (white_user_id IS NOT NULL OR white_bot_id IS NOT NULL)
);

-- The archive's primary access pattern: "my games, newest first", filtered.
CREATE INDEX games_black_idx ON games (black_user_id, started_at DESC) WHERE black_user_id IS NOT NULL;
CREATE INDEX games_white_idx ON games (white_user_id, started_at DESC) WHERE white_user_id IS NOT NULL;
CREATE INDEX games_active_idx ON games (status) WHERE status = 'active';
CREATE INDEX games_time_class_idx ON games (time_class, started_at DESC);

-- C4 asks for search on opponent, result, time control and date. Those are
-- all covered by the composite indexes above plus this one for mode.
CREATE INDEX games_mode_started_idx ON games (mode, started_at DESC);

-- D2: every accepted move, with the state hash after it. Replaying
-- game_moves reconstructs any position without touching object storage.
CREATE TABLE game_moves (
    game_id       UUID        NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    move_number   INTEGER     NOT NULL,
    player        TEXT        NOT NULL CHECK (player IN ('black','white')),
    kind          TEXT        NOT NULL CHECK (kind IN ('place','pass','resign')),
    row           SMALLINT,
    col           SMALLINT,
    captured      JSONB       NOT NULL DEFAULT '[]'::jsonb,

    -- SHA-256 of (size, side-to-move, board bytes) after this move. The
    -- client compares it against its own replay to detect desync.
    state_hash    TEXT        NOT NULL,

    -- Milliseconds the mover spent, and their clock after the move. Feeds
    -- E2's move-timing anti-cheat signal.
    think_millis  INTEGER     NOT NULL DEFAULT 0,
    clock_after   JSONB,

    played_at     TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (game_id, move_number)
);

-- Placements must carry coordinates; passes and resignations must not.
ALTER TABLE game_moves ADD CONSTRAINT game_moves_coords CHECK (
    (kind = 'place' AND row IS NOT NULL AND col IS NOT NULL) OR
    (kind <> 'place' AND row IS NULL AND col IS NULL)
);
