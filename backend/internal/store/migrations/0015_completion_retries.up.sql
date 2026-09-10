ALTER TABLE game_completion_outbox
    ADD COLUMN attempts INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now();
CREATE INDEX game_completion_ready_idx ON game_completion_outbox(next_attempt_at, created_at);
