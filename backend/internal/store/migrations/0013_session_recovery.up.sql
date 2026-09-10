-- Persist scoring, sequence numbers and clocks along with accepted moves.
-- The version fences stale actors after reconnects or ownership changes.
ALTER TABLE games ADD COLUMN session_state JSONB;
ALTER TABLE games ADD COLUMN session_version BIGINT NOT NULL DEFAULT 0;
