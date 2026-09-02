-- E2: anti-cheat signals, scores and the review queue.

-- One row per (user, game, signal). Signals are computed after a game ends
-- and kept as evidence for a human reviewer.
CREATE TABLE cheat_signals (
    id          BIGSERIAL   PRIMARY KEY,
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    game_id     UUID        REFERENCES games(id) ON DELETE CASCADE,
    kind        TEXT        NOT NULL,
    -- Normalised 0..1, where 1 is maximally suspicious.
    score       REAL        NOT NULL CHECK (score >= 0 AND score <= 1),
    -- Whatever the collector wants a reviewer to see: matched move counts,
    -- timing histograms, the rating jump.
    detail      JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, game_id, kind)
);

CREATE INDEX cheat_signals_user_idx ON cheat_signals (user_id, created_at DESC);

-- Rolling per-user suspicion score, recomputed as signals arrive.
CREATE TABLE cheat_scores (
    user_id     UUID        PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    score       REAL        NOT NULL DEFAULT 0,
    signals     INTEGER     NOT NULL DEFAULT 0,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX cheat_scores_ranked_idx ON cheat_scores (score DESC);

-- The review queue. A case is opened when a user crosses the soft-flag
-- threshold and stays open until a human resolves it.
CREATE TABLE cheat_cases (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status      TEXT        NOT NULL DEFAULT 'open'
                CHECK (status IN ('open','reviewing','actioned','dismissed')),
    score_at_open REAL      NOT NULL,
    -- 'warn' | 'restrict_ranked' | 'suspend' | 'none'
    action      TEXT,
    notes       TEXT,
    reviewer_id UUID        REFERENCES users(id) ON DELETE SET NULL,
    opened_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at TIMESTAMPTZ
);

-- At most one open case per user, so a noisy signal stream does not flood
-- the queue with duplicates of the same player.
CREATE UNIQUE INDEX cheat_cases_one_open_idx
    ON cheat_cases (user_id) WHERE status IN ('open','reviewing');

CREATE INDEX cheat_cases_queue_idx ON cheat_cases (status, opened_at);

-- Enforcement actions taken against an account.
CREATE TABLE account_restrictions (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind        TEXT        NOT NULL CHECK (kind IN ('warn','restrict_ranked','suspend')),
    reason      TEXT,
    case_id     UUID        REFERENCES cheat_cases(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- NULL means indefinite.
    expires_at  TIMESTAMPTZ,
    lifted_at   TIMESTAMPTZ
);

CREATE INDEX account_restrictions_active_idx
    ON account_restrictions (user_id)
    WHERE lifted_at IS NULL;
