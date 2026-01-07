-- Migration 000006: Set all discovered media to not monitored
-- Updates movies, series, seasons, and episodes in "discovered" state to monitored = 0

-- Update movies in discovered state
UPDATE movie
SET monitored = 0
WHERE id IN (
    SELECT movie_id
    FROM movie_transition
    WHERE to_state = 'discovered' AND most_recent = 1
);

-- Update series in discovered state
UPDATE series
SET monitored = 0
WHERE id IN (
    SELECT series_id
    FROM series_transition
    WHERE to_state = 'discovered' AND most_recent = 1
);

-- Update seasons in discovered state
UPDATE season
SET monitored = 0
WHERE id IN (
    SELECT season_id
    FROM season_transition
    WHERE to_state = 'discovered' AND most_recent = 1
);

-- Update episodes in discovered state
UPDATE episode
SET monitored = 0
WHERE id IN (
    SELECT episode_id
    FROM episode_transition
    WHERE to_state = 'discovered' AND most_recent = 1
);
