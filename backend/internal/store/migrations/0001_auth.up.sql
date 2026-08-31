-- C2: users + auth_identities.
--
-- A user row is created on first contact, guest or not. Upgrading a guest to
-- a Google account links an identity to the SAME user row, so a player never
-- loses local progress by signing in.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    display_name    TEXT        NOT NULL,
    avatar_url      TEXT,
    country         TEXT,

    -- Mirrors ProfileStore.rating on the client. Split per board size and
    -- time class in E1; kept as a single value here so C3 has a column to
    -- sync against before the rating service exists.
    rating          INTEGER     NOT NULL DEFAULT 1000,

    -- A guest has no auth_identities row and cannot be recovered if the
    -- device is lost. Cleared when the first real identity is linked.
    is_guest        BOOLEAN     NOT NULL DEFAULT TRUE,

    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);

CREATE INDEX users_last_seen_idx ON users (last_seen_at DESC) WHERE deleted_at IS NULL;

CREATE TABLE auth_identities (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- 'google' today; 'apple' is required before iOS App Store review passes
    -- if Google sign-in ships, so the column is a provider key from the start.
    provider        TEXT        NOT NULL,
    provider_uid    TEXT        NOT NULL,
    email           TEXT,
    email_verified  BOOLEAN     NOT NULL DEFAULT FALSE,

    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- One account per provider subject, and one identity per provider per user.
    UNIQUE (provider, provider_uid),
    UNIQUE (user_id, provider)
);

-- Refresh tokens are opaque, stored only as a SHA-256 hash, and rotated on
-- every use. `family_id` groups every token descended from one login so a
-- replay can revoke the whole chain at once.
CREATE TABLE refresh_tokens (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    family_id       UUID        NOT NULL,
    token_hash      BYTEA       NOT NULL UNIQUE,

    issued_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at      TIMESTAMPTZ NOT NULL,

    -- Set when this token is exchanged. A second exchange of a token that
    -- already has rotated_at set is a replay: see AuthService.Refresh.
    rotated_at      TIMESTAMPTZ,
    revoked_at      TIMESTAMPTZ,
    revoked_reason  TEXT,

    user_agent      TEXT,
    ip              INET
);

CREATE INDEX refresh_tokens_user_idx   ON refresh_tokens (user_id);
CREATE INDEX refresh_tokens_family_idx ON refresh_tokens (family_id);
CREATE INDEX refresh_tokens_expiry_idx ON refresh_tokens (expires_at)
    WHERE revoked_at IS NULL;
