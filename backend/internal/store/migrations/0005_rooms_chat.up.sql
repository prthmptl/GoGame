-- D5: friend rooms, spectating and chat.

CREATE TABLE rooms (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    -- Short human-shareable code, e.g. "K7QP4M".
    code        TEXT        NOT NULL UNIQUE,
    settings    JSONB       NOT NULL DEFAULT '{}'::jsonb,
    game_id     UUID        REFERENCES games(id) ON DELETE SET NULL,
    status      TEXT        NOT NULL DEFAULT 'open'
                CHECK (status IN ('open','started','closed')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Rooms are ephemeral; the worker sweeps stale ones.
    expires_at  TIMESTAMPTZ NOT NULL DEFAULT now() + INTERVAL '2 hours'
);

CREATE INDEX rooms_open_idx ON rooms (status, expires_at) WHERE status = 'open';

CREATE TABLE room_members (
    room_id   UUID        NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    user_id   UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role      TEXT        NOT NULL DEFAULT 'player' CHECK (role IN ('player','spectator')),
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (room_id, user_id)
);

-- Per-game chat with persistent history, per D5.
CREATE TABLE chat_messages (
    id         BIGSERIAL   PRIMARY KEY,
    game_id    UUID        NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    body       TEXT        NOT NULL,
    -- Set when the profanity filter or a moderator hides a line; the row
    -- survives for moderation review rather than being deleted.
    hidden_at  TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX chat_messages_game_idx ON chat_messages (game_id, created_at DESC);
