-- Migration 000006 down: Rollback unmonitor discovered media changes
-- NOTE: This rollback sets discovered media back to monitored = 1
-- WARNING: This assumes the previous behavior was to have discovered media monitored
--          Any user modifications to monitoring status will be overwritten

-- Restore movies in discovered state to monitored
UPDATE movie
SET monitored = 1
WHERE id IN (
    SELECT movie_id
    FROM movie_transition
    WHERE to_state = 'discovered' AND most_recent = 1
);

-- Restore series in discovered state to monitored
UPDATE series
SET monitored = 1
WHERE id IN (
    SELECT series_id
    FROM series_transition
    WHERE to_state = 'discovered' AND most_recent = 1
);

-- Restore seasons in discovered state to monitored
UPDATE season
SET monitored = 1
WHERE id IN (
    SELECT season_id
    FROM season_transition
    WHERE to_state = 'discovered' AND most_recent = 1
);

-- Restore episodes in discovered state to monitored
UPDATE episode
SET monitored = 1
WHERE id IN (
    SELECT episode_id
    FROM episode_transition
    WHERE to_state = 'discovered' AND most_recent = 1
);
