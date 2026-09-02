-- E4: daily / correspondence games.

-- Per-move budget in days rather than a running clock. A player's deadline is
-- extended by their budget after each of their opponent's moves.
CREATE TABLE correspondence_state (
    game_id         UUID        PRIMARY KEY REFERENCES games(id) ON DELETE CASCADE,
    days_per_move   SMALLINT    NOT NULL CHECK (days_per_move IN (1,3,7,14)),
    -- Whose deadline is running, and when it expires.
    to_move         TEXT        NOT NULL CHECK (to_move IN ('black','white')),
    deadline_at     TIMESTAMPTZ NOT NULL,
    -- Set while either player is on vacation, which freezes the deadline.
    paused_at       TIMESTAMPTZ,
    last_move_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX correspondence_deadline_idx
    ON correspondence_state (deadline_at) WHERE paused_at IS NULL;

-- Banked vacation days, per account rather than per game: taking a holiday
-- should pause every game at once.
CREATE TABLE vacation_accounts (
    user_id        UUID        PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    days_remaining REAL        NOT NULL DEFAULT 30,
    -- Set while the player is actually away.
    active_since   TIMESTAMPTZ,
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Conditional moves: "if my opponent plays X, reply Y". Stored as an ordered
-- sequence the server walks when the opponent's move arrives.
CREATE TABLE conditional_moves (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    game_id     UUID        NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    -- The move number this branch applies at, so a stale plan cannot fire
    -- against a position it was not written for.
    from_move   INTEGER     NOT NULL,
    -- [{"if":{"row":3,"col":3},"then":{"row":4,"col":4}}, ...]
    sequence    JSONB       NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- One active plan per player per game.
    UNIQUE (game_id, user_id)
);
