-- F5: coach marketplace.

CREATE TABLE coaches (
    user_id        UUID        PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    headline       TEXT        NOT NULL,
    bio            TEXT,
    -- Minor units (cents) to avoid float money entirely.
    hourly_rate_cents INTEGER  NOT NULL CHECK (hourly_rate_cents >= 0),
    currency       TEXT        NOT NULL DEFAULT 'USD',
    languages      TEXT[]      NOT NULL DEFAULT '{}',
    rank_label     TEXT,
    -- Set once the coach completes payout onboarding with the processor.
    payout_account_id TEXT,
    status         TEXT        NOT NULL DEFAULT 'pending'
                   CHECK (status IN ('pending','active','paused','removed')),
    rating_avg     REAL        NOT NULL DEFAULT 0,
    rating_count   INTEGER     NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX coaches_active_idx ON coaches (status, rating_avg DESC) WHERE status = 'active';

-- Weekly recurring availability, stored per weekday in UTC minutes.
CREATE TABLE coach_availability (
    coach_id     UUID     NOT NULL REFERENCES coaches(user_id) ON DELETE CASCADE,
    weekday      SMALLINT NOT NULL CHECK (weekday BETWEEN 0 AND 6),
    start_minute SMALLINT NOT NULL CHECK (start_minute BETWEEN 0 AND 1439),
    end_minute   SMALLINT NOT NULL CHECK (end_minute BETWEEN 1 AND 1440),
    PRIMARY KEY (coach_id, weekday, start_minute),
    CONSTRAINT coach_availability_order CHECK (end_minute > start_minute)
);

CREATE TABLE coaching_sessions (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    coach_id       UUID        NOT NULL REFERENCES coaches(user_id) ON DELETE CASCADE,
    student_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    starts_at      TIMESTAMPTZ NOT NULL,
    duration_minutes SMALLINT  NOT NULL DEFAULT 60,
    price_cents    INTEGER     NOT NULL,
    currency       TEXT        NOT NULL DEFAULT 'USD',
    status         TEXT        NOT NULL DEFAULT 'requested'
                   CHECK (status IN ('requested','confirmed','paid','completed','cancelled','refunded')),
    payment_ref    TEXT,
    notes          TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT coaching_no_self CHECK (coach_id <> student_id)
);

CREATE INDEX coaching_sessions_coach_idx ON coaching_sessions (coach_id, starts_at DESC);
CREATE INDEX coaching_sessions_student_idx ON coaching_sessions (student_id, starts_at DESC);
-- A coach cannot be double-booked for the same slot.
CREATE UNIQUE INDEX coaching_sessions_slot_idx
    ON coaching_sessions (coach_id, starts_at)
    WHERE status IN ('requested','confirmed','paid');

CREATE TABLE coach_reviews (
    coach_id   UUID        NOT NULL REFERENCES coaches(user_id) ON DELETE CASCADE,
    student_id UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    session_id UUID        REFERENCES coaching_sessions(id) ON DELETE SET NULL,
    rating     SMALLINT    NOT NULL CHECK (rating BETWEEN 1 AND 5),
    comment    TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (coach_id, student_id, session_id)
);

-- G1: subscriptions and in-app purchases.

CREATE TABLE subscriptions (
    user_id        UUID        PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    tier           TEXT        NOT NULL DEFAULT 'free'
                   CHECK (tier IN ('free','gold','platinum','diamond')),
    platform       TEXT        CHECK (platform IN ('ios','android','web','promo') OR platform IS NULL),
    -- The store's own subscription identifier, used to reconcile webhooks.
    original_txn_id TEXT       UNIQUE,
    product_id     TEXT,
    status         TEXT        NOT NULL DEFAULT 'none'
                   CHECK (status IN ('none','active','in_grace','expired','refunded','cancelled')),
    -- Access is granted until this instant, which survives a cancelled
    -- auto-renew: a user who cancels keeps what they paid for.
    expires_at     TIMESTAMPTZ,
    auto_renew     BOOLEAN     NOT NULL DEFAULT TRUE,
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX subscriptions_expiry_idx ON subscriptions (expires_at)
    WHERE status IN ('active','in_grace');

-- Raw store notifications, kept for reconciliation and dispute handling.
CREATE TABLE billing_events (
    id           BIGSERIAL   PRIMARY KEY,
    platform     TEXT        NOT NULL,
    -- The store's notification id; unique so a redelivered webhook is a
    -- no-op rather than a double grant.
    event_uid    TEXT        NOT NULL,
    user_id      UUID        REFERENCES users(id) ON DELETE SET NULL,
    kind         TEXT        NOT NULL,
    payload      JSONB       NOT NULL,
    processed_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (platform, event_uid)
);

-- G2: feature gating. Quotas are consumed per user per period.
CREATE TABLE entitlement_usage (
    user_id      UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    feature      TEXT        NOT NULL,
    -- Period start, truncated to the feature's window (day or week).
    period_start DATE        NOT NULL,
    used         INTEGER     NOT NULL DEFAULT 0,
    PRIMARY KEY (user_id, feature, period_start)
);
