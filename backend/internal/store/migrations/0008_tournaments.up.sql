-- E3: tournaments in three formats.

CREATE TABLE tournaments (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name          TEXT        NOT NULL,
    description   TEXT,
    -- 'arena'  : continuous re-pairing for a fixed duration
    -- 'swiss'  : round-based, players never meet twice
    -- 'mcmahon': Swiss with seeded starting scores, the Go convention
    -- 'knockout': single elimination
    format        TEXT        NOT NULL
                  CHECK (format IN ('arena','swiss','mcmahon','knockout')),
    board_size    SMALLINT    NOT NULL DEFAULT 19,
    ruleset       TEXT        NOT NULL DEFAULT 'chinese',
    time_control  JSONB       NOT NULL,
    rounds        SMALLINT,                 -- NULL for arena
    duration_minutes INTEGER,               -- arena only
    max_entries   INTEGER,
    min_rating    INTEGER,
    max_rating    INTEGER,

    status        TEXT        NOT NULL DEFAULT 'scheduled'
                  CHECK (status IN ('scheduled','registering','running','finished','cancelled')),
    starts_at     TIMESTAMPTZ NOT NULL,
    ends_at       TIMESTAMPTZ,
    created_by    UUID        REFERENCES users(id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX tournaments_upcoming_idx ON tournaments (status, starts_at);

CREATE TABLE tournament_entries (
    tournament_id UUID        NOT NULL REFERENCES tournaments(id) ON DELETE CASCADE,
    user_id       UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    seed_rating   INTEGER     NOT NULL,
    -- McMahon players start on a bar score derived from their rating.
    score         REAL        NOT NULL DEFAULT 0,
    -- Sum of opponents' scores, the standard Swiss/McMahon tiebreak.
    tiebreak      REAL        NOT NULL DEFAULT 0,
    wins          INTEGER     NOT NULL DEFAULT 0,
    losses        INTEGER     NOT NULL DEFAULT 0,
    draws         INTEGER     NOT NULL DEFAULT 0,
    -- Knockout: set when the player is eliminated.
    eliminated_in SMALLINT,
    withdrawn     BOOLEAN     NOT NULL DEFAULT FALSE,
    joined_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tournament_id, user_id)
);

CREATE INDEX tournament_entries_standings_idx
    ON tournament_entries (tournament_id, score DESC, tiebreak DESC);

CREATE TABLE tournament_rounds (
    tournament_id UUID        NOT NULL REFERENCES tournaments(id) ON DELETE CASCADE,
    round_number  SMALLINT    NOT NULL,
    status        TEXT        NOT NULL DEFAULT 'pending'
                  CHECK (status IN ('pending','running','complete')),
    started_at    TIMESTAMPTZ,
    completed_at  TIMESTAMPTZ,
    PRIMARY KEY (tournament_id, round_number)
);

CREATE TABLE tournament_pairings (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tournament_id UUID        NOT NULL REFERENCES tournaments(id) ON DELETE CASCADE,
    round_number  SMALLINT    NOT NULL,
    black_id      UUID        REFERENCES users(id) ON DELETE SET NULL,
    white_id      UUID        REFERENCES users(id) ON DELETE SET NULL,
    game_id       UUID        REFERENCES games(id) ON DELETE SET NULL,
    -- A bye is a pairing with one player and an automatic point.
    is_bye        BOOLEAN     NOT NULL DEFAULT FALSE,
    result        TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX tournament_pairings_round_idx
    ON tournament_pairings (tournament_id, round_number);
-- Swiss and McMahon must never repeat a pairing; this makes that checkable
-- with an index rather than a scan.
CREATE INDEX tournament_pairings_players_idx
    ON tournament_pairings (tournament_id, black_id, white_id);
