-- E1: per-user, per-board-size, per-time-class Glicko-2 ratings.
--
-- Glicko-2 needs three numbers, not one: the rating, the deviation (how
-- uncertain we are) and the volatility (how erratic the player is). The
-- single users.rating column stays as a display cache of the player's
-- headline rating.

CREATE TABLE ratings (
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    board_size  SMALLINT    NOT NULL,
    time_class  TEXT        NOT NULL,

    rating      DOUBLE PRECISION NOT NULL DEFAULT 1500,
    deviation   DOUBLE PRECISION NOT NULL DEFAULT 350,
    volatility  DOUBLE PRECISION NOT NULL DEFAULT 0.06,

    games       INTEGER     NOT NULL DEFAULT 0,
    wins        INTEGER     NOT NULL DEFAULT 0,
    losses      INTEGER     NOT NULL DEFAULT 0,
    draws       INTEGER     NOT NULL DEFAULT 0,

    -- Glicko-2 inflates deviation for inactivity; this is what that is
    -- measured from.
    last_played_at TIMESTAMPTZ,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (user_id, board_size, time_class)
);

CREATE INDEX ratings_leaderboard_idx ON ratings (board_size, time_class, rating DESC);

CREATE TABLE ratings_history (
    id          BIGSERIAL   PRIMARY KEY,
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    game_id     UUID        REFERENCES games(id) ON DELETE SET NULL,
    board_size  SMALLINT    NOT NULL,
    time_class  TEXT        NOT NULL,
    rating_before DOUBLE PRECISION NOT NULL,
    rating_after  DOUBLE PRECISION NOT NULL,
    deviation_after DOUBLE PRECISION NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX ratings_history_user_idx ON ratings_history (user_id, created_at DESC);
-- One rating change per player per game, so a replayed completion event
-- cannot double-count.
CREATE UNIQUE INDEX ratings_history_game_user_idx
    ON ratings_history (game_id, user_id) WHERE game_id IS NOT NULL;
