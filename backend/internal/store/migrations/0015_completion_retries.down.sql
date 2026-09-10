DROP INDEX game_completion_ready_idx;
ALTER TABLE game_completion_outbox DROP COLUMN next_attempt_at, DROP COLUMN attempts;
