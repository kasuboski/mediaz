-- Fresh-start baseline schema for mediaz (single DDL source).
-- This file is the ONLY schema definition: golang-migrate applies it to fresh
-- databases and sqlc compiles every application query against it (see sqlc.yaml).
-- Breaking change: databases created by earlier releases (squashed migrations
-- 000001-000007) are NOT upgraded automatically; operators must recreate the
-- mediaz database file. Media files on disk are never touched.
-- Default seed data lives in defaults.sql (applied after migrations, idempotent).

-- Application schema after migrations 000001 through 000007.
-- Keep this snapshot aligned with migrations: sqlc compiles every application query against it.

CREATE TABLE IF NOT EXISTS "indexer_source" (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "name" TEXT NOT NULL,
    "implementation" TEXT NOT NULL,
    "scheme" TEXT NOT NULL DEFAULT 'http',
    "host" TEXT NOT NULL,
    "port" INTEGER,
    "api_key" TEXT,
    "enabled" BOOLEAN NOT NULL DEFAULT 1,
    "created_at" DATETIME DEFAULT CURRENT_TIMESTAMP,
    "updated_at" DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS "idx_indexer_source_implementation" ON "indexer_source" ("implementation");
CREATE INDEX IF NOT EXISTS "idx_indexer_source_enabled" ON "indexer_source" ("enabled");

CREATE TABLE IF NOT EXISTS "indexer" (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "name" TEXT NOT NULL,
    "priority" INTEGER NOT NULL DEFAULT 25,
    "uri" TEXT NOT NULL,
    "api_key" TEXT,
    "indexer_source_id" INTEGER REFERENCES "indexer_source"("id") ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS "quality_definition" (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "quality_id" INTEGER NOT NULL,
    "name" TEXT NOT NULL,
    "preferred_size" NUMERIC NOT NULL,
    "min_size" NUMERIC NOT NULL,
    "max_size" NUMERIC NOT NULL,
    "media_type" TEXT NOT NULL
);


CREATE TABLE IF NOT EXISTS "quality_profile" (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "name" TEXT NOT NULL,
    "cutoff_quality_id" INTEGER,
    "upgrade_allowed" BOOLEAN NOT NULL
);

CREATE TABLE IF NOT EXISTS "quality_profile_item" (
    "id" INTEGER PRIMARY KEY AUTOINCREMENT,
    "profile_id" INTEGER NOT NULL,
    "quality_id" INTEGER NOT NULL,
    FOREIGN KEY ("profile_id") REFERENCES "quality_profile" ("id"),
    FOREIGN KEY ("quality_id") REFERENCES "quality_definition" ("id")
);

CREATE UNIQUE INDEX IF NOT EXISTS "idx_quality_profile_item_profile_quality" ON "quality_profile_item" ("profile_id", "quality_id");

CREATE TABLE IF NOT EXISTS "movie_file" (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "quality" TEXT NOT NULL,
    "size" BIGINT NOT NULL,
    "date_added" DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "scene_name" TEXT,
    "media_info" TEXT,
    "release_group" TEXT,
    "relative_path" TEXT UNIQUE,
    "edition" TEXT,
    "languages" TEXT NOT NULL,
    "indexer_flags" INTEGER NOT NULL,
    "original_file_path" TEXT
);

CREATE TABLE IF NOT EXISTS "movie_metadata" (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "tmdb_id" INTEGER NOT NULL,
    "imdb_id" TEXT,
    "images" TEXT NOT NULL,
    "genres" TEXT,
    "title" TEXT NOT NULL,
    "sort_title" TEXT,
    "clean_title" TEXT,
    "original_title" TEXT,
    "clean_original_title" TEXT,
    "original_language" INTEGER NOT NULL,
    "status" INTEGER NOT NULL,
    "last_info_sync" DATETIME,
    "runtime" INTEGER NOT NULL,
    "in_cinemas" DATETIME,
    "release_date" DATETIME,
    "year" INTEGER,
    "secondary_year" INTEGER,
    "ratings" TEXT,
    "recommendations" TEXT NOT NULL,
    "certification" TEXT,
    "youtube_trailer_id" TEXT,
    "studio" TEXT,
    "overview" TEXT,
    "website" TEXT,
    "popularity" NUMERIC,
    "collection_tmdb_id" INTEGER,
    "collection_title" TEXT
);

CREATE TABLE IF NOT EXISTS "movie" (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "path" TEXT,
    "monitored" INTEGER NOT NULL,
    "quality_profile_id" INTEGER NOT NULL REFERENCES "quality_profile"("id") ON DELETE RESTRICT,
    "added" DATETIME DEFAULT CURRENT_TIMESTAMP,
    "tags" TEXT,
    "add_options" TEXT,
    "movie_file_id" INTEGER REFERENCES "movie_file"("id") ON DELETE SET NULL,
    "minimum_availability" INTEGER NOT NULL,
    "movie_metadata_id" INTEGER UNIQUE REFERENCES "movie_metadata"("id") ON DELETE SET NULL,
    "last_search_time" DATETIME
);

CREATE TABLE IF NOT EXISTS "series" (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "path" TEXT,
    "monitored" INTEGER NOT NULL,
    "added" DATETIME DEFAULT CURRENT_TIMESTAMP,
    "quality_profile_id" INTEGER NOT NULL REFERENCES "quality_profile"("id") ON DELETE RESTRICT,
    "series_metadata_id" INTEGER UNIQUE REFERENCES "series_metadata"("id") ON DELETE SET NULL,
    "last_search_time" DATETIME,
    "monitor_new_seasons" INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS "series_metadata" (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "tmdb_id" INTEGER NOT NULL UNIQUE,
    "title" TEXT NOT NULL,
    "overview" TEXT,
    "last_info_sync" DATETIME DEFAULT CURRENT_TIMESTAMP,
    "first_air_date" DATETIME,
    "last_air_date" DATETIME,
    "season_count" INTEGER NOT NULL,
    "episode_count" INTEGER NOT NULL,
    "status" TEXT NOT NULL,
    "poster_path" TEXT,
    "external_ids" TEXT,
    "watch_providers" TEXT
);

CREATE TABLE IF NOT EXISTS "season" (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "series_id" INTEGER NOT NULL,
    "season_number" INTEGER NOT NULL,
    "season_metadata_id" INTEGER REFERENCES "season_metadata"("id") ON DELETE SET NULL,
    "monitored" INTEGER NOT NULL,
    FOREIGN KEY ("series_id") REFERENCES "series" ("id") ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS "idx_season_unique_series_number" ON "season" ("series_id", "season_number");

CREATE TABLE IF NOT EXISTS "season_metadata" (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "series_metadata_id" INTEGER NOT NULL,
    "number" INTEGER NOT NULL,
    "tmdb_id" INTEGER NOT NULL UNIQUE,
    "title" TEXT NOT NULL,
    "overview" TEXT,
    "air_date" DATETIME,
    FOREIGN KEY ("series_metadata_id") REFERENCES "series_metadata" ("id")
);

CREATE TABLE IF NOT EXISTS "episode" (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "season_id" INTEGER NOT NULL,
    "episode_number" INTEGER NOT NULL,
    "monitored" INTEGER NOT NULL,
    "episode_metadata_id" INTEGER UNIQUE REFERENCES "episode_metadata"("id") ON DELETE SET NULL,
    "episode_file_id" INTEGER REFERENCES "episode_file"("id") ON DELETE SET NULL,
    FOREIGN KEY ("season_id") REFERENCES "season" ("id") ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS "episode_file" (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "quality" TEXT NOT NULL,
    "size" BIGINT NOT NULL,
    "added" DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "relative_path" TEXT UNIQUE,
    "original_file_path" TEXT
);

CREATE TABLE IF NOT EXISTS "episode_metadata" (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "season_metadata_id" INTEGER NOT NULL,
    "number" INTEGER NOT NULL,
    "tmdb_id" INTEGER NOT NULL UNIQUE,
    "title" TEXT NOT NULL,
    "overview" TEXT,
    "air_date" DATETIME,
    "runtime" INTEGER,
    "still_path" TEXT,
    FOREIGN KEY ("season_metadata_id") REFERENCES "season_metadata" ("id")
);

CREATE TABLE IF NOT EXISTS "movie_transition" (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "movie_id" INTEGER NOT NULL REFERENCES "movie"("id") ON DELETE CASCADE,
    "to_state" TEXT NOT NULL,
    "from_state" TEXT,
    "most_recent" BOOLEAN NOT NULL,
    "sort_key" INTEGER NOT NULL,
    "download_client_id" INTEGER REFERENCES "download_client"("id") ON DELETE SET NULL,
    "download_id" TEXT,
    "created_at" DATETIME DEFAULT CURRENT_TIMESTAMP,
    "updated_at" DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS "series_transition" (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "series_id" INTEGER NOT NULL REFERENCES "series"("id") ON DELETE CASCADE,
    "to_state" TEXT NOT NULL,
    "from_state" TEXT,
    "most_recent" BOOLEAN NOT NULL,
    "sort_key" INTEGER NOT NULL,
    "created_at" DATETIME DEFAULT CURRENT_TIMESTAMP,
    "updated_at" DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS "season_transition" (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "season_id" INTEGER NOT NULL REFERENCES "season"("id") ON DELETE CASCADE,
    "to_state" TEXT NOT NULL,
    "from_state" TEXT,
    "most_recent" BOOLEAN NOT NULL,
    "sort_key" INTEGER NOT NULL,
    "download_client_id" INTEGER REFERENCES "download_client"("id") ON DELETE SET NULL,
    "download_id" TEXT,
    "is_entire_season_download" BOOLEAN,
    "created_at" DATETIME DEFAULT CURRENT_TIMESTAMP,
    "updated_at" DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS "episode_transition" (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "episode_id" INTEGER NOT NULL REFERENCES "episode"("id") ON DELETE CASCADE,
    "to_state" TEXT NOT NULL,
    "from_state" TEXT,
    "most_recent" BOOLEAN NOT NULL,
    "sort_key" INTEGER NOT NULL,
    "download_client_id" INTEGER REFERENCES "download_client"("id") ON DELETE SET NULL,
    "download_id" TEXT,
    "is_entire_season_download" BOOLEAN,
    "created_at" DATETIME DEFAULT CURRENT_TIMESTAMP,
    "updated_at" DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS "download_client" (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "type" TEXT NOT NULL,
    "implementation" TEXT NOT NULL,
    "scheme" TEXT NOT NULL,
    "host" TEXT NOT NULL,
    "port" INTEGER NOT NULL,
    "api_key" TEXT
);

CREATE TABLE IF NOT EXISTS "job" (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "type" TEXT NOT NULL,
    "created_at" DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS "job_transition" (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "job_id" INTEGER NOT NULL REFERENCES "job"("id") ON DELETE CASCADE,
    "type" TEXT NOT NULL,
    "to_state" TEXT NOT NULL,
    "from_state" TEXT,
    "most_recent" BOOLEAN NOT NULL,
    "sort_key" INTEGER NOT NULL,
    "error" TEXT,
    "created_at" DATETIME DEFAULT CURRENT_TIMESTAMP,
    "updated_at" DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS "idx_indexer_by_source" ON "indexer" ("indexer_source_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_indexer_unique_source_name" ON "indexer" ("indexer_source_id", "name");

CREATE UNIQUE INDEX IF NOT EXISTS "idx_movie_transitions_by_parent_most_recent" ON "movie_transition"("movie_id", "most_recent")
WHERE
    "most_recent" = 1;

CREATE UNIQUE INDEX IF NOT EXISTS "idx_movie_transitions_by_parent_sort_key" ON "movie_transition"("movie_id", "sort_key");

CREATE UNIQUE INDEX IF NOT EXISTS "idx_series_transitions_by_parent_most_recent" ON "series_transition"("series_id", "most_recent")
WHERE
    "most_recent" = 1;

CREATE UNIQUE INDEX IF NOT EXISTS "idx_series_transitions_by_parent_sort_key" ON "series_transition"("series_id", "sort_key");

CREATE UNIQUE INDEX IF NOT EXISTS "idx_season_transitions_by_parent_most_recent" ON "season_transition"("season_id", "most_recent")
WHERE
    "most_recent" = 1;

CREATE UNIQUE INDEX IF NOT EXISTS "idx_season_transitions_by_parent_sort_key" ON "season_transition"("season_id", "sort_key");

CREATE UNIQUE INDEX IF NOT EXISTS "idx_episode_transitions_by_parent_most_recent" ON "episode_transition"("episode_id", "most_recent")
WHERE
    "most_recent" = 1;

CREATE UNIQUE INDEX IF NOT EXISTS "idx_episode_transitions_by_parent_sort_key" ON "episode_transition"("episode_id", "sort_key");

CREATE UNIQUE INDEX IF NOT EXISTS "idx_season_metadata_unique_series_season" ON "season_metadata" ("series_metadata_id", "number");

CREATE UNIQUE INDEX IF NOT EXISTS "idx_episode_metadata_unique_season_episode" ON "episode_metadata" ("season_metadata_id", "number");

CREATE UNIQUE INDEX IF NOT EXISTS "idx_job_transitions_by_parent_most_recent"
ON "job_transition"("job_id") WHERE "most_recent" = 1;

CREATE UNIQUE INDEX IF NOT EXISTS "idx_job_transitions_by_parent_sort_key"
ON "job_transition"("job_id", "sort_key");

CREATE INDEX IF NOT EXISTS "idx_job_transition_type_state_updated"
ON "job_transition"("type", "to_state", "updated_at" DESC)
WHERE "most_recent" = 1;

CREATE UNIQUE INDEX IF NOT EXISTS "idx_job_type_pending" ON "job_transition"("type", "to_state")
WHERE
    "to_state" = 'pending' AND "most_recent" = 1;

CREATE INDEX IF NOT EXISTS "idx_job_created_at" ON "job"("created_at" DESC);
CREATE INDEX IF NOT EXISTS "idx_job_type" ON "job"("type");
