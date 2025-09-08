-- Migration to fix season/episode relationship issues
-- This migration addresses three main problems:
-- 1. Duplicate seasons in the same series
-- 2. season_metadata.series_id referencing metadata IDs instead of entity IDs
-- 3. episode_metadata.season_id referencing metadata IDs instead of entity IDs

BEGIN TRANSACTION;

-- Step 1: Clean up duplicate seasons
-- First, update episodes to point to the season with the lowest ID for each series+season_number combination
UPDATE episode 
SET season_id = (
    SELECT MIN(s.id) 
    FROM season s 
    WHERE s.series_id = (
        SELECT orig_s.series_id 
        FROM season orig_s 
        WHERE orig_s.id = episode.season_id
    ) 
    AND s.season_number = (
        SELECT orig_s.season_number 
        FROM season orig_s 
        WHERE orig_s.id = episode.season_id
    )
)
WHERE season_id NOT IN (
    SELECT MIN(id) 
    FROM season 
    GROUP BY series_id, season_number
);

-- Delete season_transitions for duplicate seasons to avoid constraint violations
-- We'll keep the transitions for the season with the lowest ID (the one we're keeping)
DELETE FROM season_transition 
WHERE season_id NOT IN (
    SELECT MIN(id) 
    FROM season 
    GROUP BY series_id, season_number
);

-- Delete duplicate seasons, keeping only the one with the lowest ID for each series+season_number
DELETE FROM season 
WHERE id NOT IN (
    SELECT MIN(id) 
    FROM season 
    GROUP BY series_id, season_number
);

-- Step 2: Fix season_metadata.series_id to reference series entity IDs instead of series_metadata IDs
-- First, let's backup the current problematic references by creating a mapping table for recovery
CREATE TEMP TABLE season_metadata_backup AS
SELECT id, series_id as old_series_id, number, tmdb_id, title
FROM season_metadata;

-- Update season_metadata.series_id to reference actual series entity IDs
UPDATE season_metadata 
SET series_id = (
    SELECT s.id 
    FROM series s 
    WHERE s.series_metadata_id = season_metadata.series_id
)
WHERE EXISTS (
    SELECT 1 
    FROM series s 
    WHERE s.series_metadata_id = season_metadata.series_id
);

-- Step 3: Fix episode_metadata.season_id to reference season entity IDs instead of season_metadata IDs  
-- First backup current problematic references
CREATE TEMP TABLE episode_metadata_backup AS
SELECT id, season_id as old_season_id, number, tmdb_id, title
FROM episode_metadata;

-- Update episode_metadata.season_id to reference actual season entity IDs
-- Match based on series and season number
UPDATE episode_metadata 
SET season_id = (
    SELECT s.id 
    FROM season s 
    JOIN season_metadata sm ON sm.id = episode_metadata.season_id
    JOIN series ser ON ser.id = s.series_id 
    JOIN series_metadata serm ON serm.id = ser.series_metadata_id
    WHERE s.season_number = sm.number 
    AND serm.id = sm.series_id
    LIMIT 1
)
WHERE EXISTS (
    SELECT 1
    FROM season s 
    JOIN season_metadata sm ON sm.id = episode_metadata.season_id
    JOIN series ser ON ser.id = s.series_id 
    JOIN series_metadata serm ON serm.id = ser.series_metadata_id
    WHERE s.season_number = sm.number 
    AND serm.id = sm.series_id
);

-- Step 4: Link existing seasons to their metadata where possible
UPDATE season 
SET season_metadata_id = (
    SELECT sm.id 
    FROM season_metadata sm 
    WHERE sm.series_id = season.series_id 
    AND sm.number = season.season_number
    LIMIT 1
)
WHERE season_metadata_id IS NULL
AND EXISTS (
    SELECT 1 
    FROM season_metadata sm 
    WHERE sm.series_id = season.series_id 
    AND sm.number = season.season_number
);

-- Step 5: Add unique constraint to prevent future duplicates
CREATE UNIQUE INDEX IF NOT EXISTS "idx_season_unique_series_number" ON "season" ("series_id", "season_number");

COMMIT;

-- Verification queries (run these after the migration to verify success)
-- 1. Check for remaining duplicate seasons:
-- SELECT series_id, season_number, COUNT(*) as count FROM season GROUP BY series_id, season_number HAVING COUNT(*) > 1;

-- 2. Check that season_metadata.series_id now references series entity IDs:
-- SELECT sm.id, sm.series_id, s.id as actual_series_id, sm.title 
-- FROM season_metadata sm 
-- LEFT JOIN series s ON s.id = sm.series_id 
-- WHERE s.id IS NULL LIMIT 10;

-- 3. Check that episode_metadata.season_id now references season entity IDs:
-- SELECT em.id, em.season_id, s.id as actual_season_id, em.title 
-- FROM episode_metadata em 
-- LEFT JOIN season s ON s.id = em.season_id 
-- WHERE s.id IS NULL LIMIT 10;