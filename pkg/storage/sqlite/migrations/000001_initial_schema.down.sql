-- Migration 000001 down: Drop all tables (rollback to empty database)
-- Drops tables in reverse dependency order to avoid foreign key violations

DROP TABLE IF EXISTS job_transition;
DROP TABLE IF EXISTS job;
DROP TABLE IF EXISTS episode_transition;
DROP TABLE IF EXISTS season_transition;
DROP TABLE IF EXISTS series_transition;
DROP TABLE IF EXISTS movie_transition;
DROP TABLE IF EXISTS download_client;
DROP TABLE IF EXISTS episode_metadata;
DROP TABLE IF EXISTS episode_file;
DROP TABLE IF EXISTS episode;
DROP TABLE IF EXISTS season_metadata;
DROP TABLE IF EXISTS season;
DROP TABLE IF EXISTS series_metadata;
DROP TABLE IF EXISTS series;
DROP TABLE IF EXISTS movie;
DROP TABLE IF EXISTS movie_metadata;
DROP TABLE IF EXISTS movie_file;
DROP TABLE IF EXISTS quality_profile_item;
DROP TABLE IF EXISTS quality_profile;
DROP TABLE IF EXISTS quality_definition;
DROP TABLE IF EXISTS indexer;
