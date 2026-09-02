-- C5: push notifications.

-- One row per installed app instance. A user with a phone and a tablet has
-- two; a reinstall produces a new token for the same device.
CREATE TABLE device_tokens (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token        TEXT        NOT NULL UNIQUE,
    platform     TEXT        NOT NULL CHECK (platform IN ('android','ios','web')),
    locale       TEXT,
    app_version  TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Set when FCM/APNs reports the token permanently dead, so the sender
    -- stops retrying it but the row survives for debugging.
    disabled_at  TIMESTAMPTZ,
    disable_reason TEXT
);

CREATE INDEX device_tokens_user_idx ON device_tokens (user_id) WHERE disabled_at IS NULL;

-- Per-user, per-event-type toggles. Absence of a row means the event's
-- default applies, so a new event type does not require backfilling rows.
CREATE TABLE notification_prefs (
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_type  TEXT        NOT NULL,
    enabled     BOOLEAN     NOT NULL DEFAULT TRUE,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, event_type)
);

-- Outbox: notifications are enqueued in the same transaction as the event
-- that caused them, then delivered by the worker. This is what stops a push
-- being sent for a move that later rolls back.
CREATE TABLE notification_outbox (
    id           BIGSERIAL   PRIMARY KEY,
    user_id      UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_type   TEXT        NOT NULL,
    title        TEXT        NOT NULL,
    body         TEXT        NOT NULL,
    data         JSONB       NOT NULL DEFAULT '{}'::jsonb,

    -- Collapse key: a second "your turn" push for the same game replaces the
    -- first on the device instead of stacking.
    collapse_key TEXT,

    attempts     INTEGER     NOT NULL DEFAULT 0,
    -- Exponential backoff target; the worker only claims rows due now.
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    delivered_at TIMESTAMPTZ,
    failed_at    TIMESTAMPTZ,
    last_error   TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX notification_outbox_pending_idx
    ON notification_outbox (next_attempt_at)
    WHERE delivered_at IS NULL AND failed_at IS NULL;
