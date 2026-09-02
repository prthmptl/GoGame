-- F3: joseki / opening explorer.
--
-- Position-keyed index: for each position we have seen, how often each
-- follow-up was played and how it scored for the player to move.

CREATE TABLE opening_positions (
    -- SHA-256 of (board size, side to move, board bytes) — the same
    -- canonical hash the game engine emits, so a live position can be looked
    -- up directly without recomputing anything.
    position_hash TEXT       PRIMARY KEY,
    board_size    SMALLINT   NOT NULL,
    to_move       TEXT       NOT NULL CHECK (to_move IN ('black','white')),
    move_number   INTEGER    NOT NULL,
    games         INTEGER    NOT NULL DEFAULT 0,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX opening_positions_popular_idx ON opening_positions (board_size, games DESC);

CREATE TABLE opening_moves (
    position_hash TEXT       NOT NULL REFERENCES opening_positions(position_hash) ON DELETE CASCADE,
    row           SMALLINT   NOT NULL,
    col           SMALLINT   NOT NULL,
    games         INTEGER    NOT NULL DEFAULT 0,
    -- Wins for the player who played this move, so a single number reads the
    -- same regardless of colour.
    wins          INTEGER    NOT NULL DEFAULT 0,
    draws         INTEGER    NOT NULL DEFAULT 0,
    -- Resulting position, so the client can walk the tree without replaying.
    next_hash     TEXT,
    -- Set for named joseki / fuseki sequences.
    name          TEXT,
    PRIMARY KEY (position_hash, row, col)
);

CREATE INDEX opening_moves_popular_idx ON opening_moves (position_hash, games DESC);

-- F4: professional game library, kept separate from user games.
CREATE TABLE pro_games (
    id            UUID       PRIMARY KEY DEFAULT gen_random_uuid(),
    -- Stable id from the source collection, so re-imports update rather than
    -- duplicate.
    source        TEXT       NOT NULL,
    source_ref    TEXT       NOT NULL,

    black_name    TEXT       NOT NULL,
    white_name    TEXT       NOT NULL,
    black_rank    TEXT,
    white_rank    TEXT,
    event         TEXT,
    round         TEXT,
    place         TEXT,
    played_on     DATE,

    board_size    SMALLINT   NOT NULL DEFAULT 19,
    komi          REAL,
    handicap      SMALLINT   NOT NULL DEFAULT 0,
    result        TEXT,
    winner        TEXT CHECK (winner IN ('black','white','draw') OR winner IS NULL),
    move_count    INTEGER    NOT NULL DEFAULT 0,

    sgf_object_key TEXT,
    -- Full-text search over players and event.
    search_text   TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (source, source_ref)
);

CREATE INDEX pro_games_date_idx ON pro_games (played_on DESC NULLS LAST);
CREATE INDEX pro_games_search_idx ON pro_games USING gin (to_tsvector('simple', coalesce(search_text,'')));

-- Live tournament relays: an SGF stream ingested from a broadcaster.
CREATE TABLE relays (
    id            UUID       PRIMARY KEY DEFAULT gen_random_uuid(),
    title         TEXT       NOT NULL,
    event         TEXT,
    source        TEXT       NOT NULL,
    status        TEXT       NOT NULL DEFAULT 'scheduled'
                  CHECK (status IN ('scheduled','live','finished','cancelled')),
    board_size    SMALLINT   NOT NULL DEFAULT 19,
    black_name    TEXT,
    white_name    TEXT,
    -- The moves received so far, appended as the broadcast progresses.
    moves         JSONB      NOT NULL DEFAULT '[]'::jsonb,
    current_hash  TEXT,
    started_at    TIMESTAMPTZ,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX relays_live_idx ON relays (status, started_at DESC);
