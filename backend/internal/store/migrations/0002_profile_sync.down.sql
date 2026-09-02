DROP MATERIALIZED VIEW IF EXISTS leaderboard_global;
DROP TABLE IF EXISTS user_achievements;
DROP TABLE IF EXISTS achievements;
DROP TABLE IF EXISTS user_streaks;
DROP TABLE IF EXISTS progress_entries;
ALTER TABLE users DROP COLUMN IF EXISTS bio, DROP COLUMN IF EXISTS profile_revision;
