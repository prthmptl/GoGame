CREATE TABLE game_completion_outbox (
    game_id UUID PRIMARY KEY REFERENCES games(id) ON DELETE CASCADE,
    result JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
