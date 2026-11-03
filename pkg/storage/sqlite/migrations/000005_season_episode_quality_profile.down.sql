-- Rebuild-less downgrade: modern SQLite (>= 3.35) can drop these columns in
-- place. Each column is the child side of a self-contained foreign key
-- (REFERENCES quality_profile ... ON DELETE SET NULL) created by the up
-- script, so DROP COLUMN removes the constraint with the column. No other
-- table references these columns, and neither season nor episode is dropped,
-- so nothing referencing them (episode.season_id, season_transition,
-- episode_transition, and their indexes) is touched. This preserves all data,
-- constraints, indexes, and triggers exactly as they were before migration 5.
--
-- A rebuild-with-copy is deliberately avoided: with PRAGMA foreign_keys=ON
-- (mediaz's normal mode) dropping the old `season` table implicitly deletes
-- its rows, which cascades through episode.season_id ON DELETE CASCADE and
-- destroys every episode while the new season table is being swapped in.

ALTER TABLE "season" DROP COLUMN "quality_profile_id";

ALTER TABLE "episode" DROP COLUMN "quality_profile_id";
