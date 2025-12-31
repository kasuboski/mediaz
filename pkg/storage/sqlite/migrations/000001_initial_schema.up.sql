-- Migration 000001: Initial schema baseline (matching main branch)
-- This establishes the baseline schema for existing databases


CREATE TABLE IF NOT EXISTS "indexer" (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "name" TEXT NOT NULL,
    "priority" INTEGER NOT NULL DEFAULT 25,
    "uri" TEXT NOT NULL,
    "api_key" TEXT
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
    "cutoff_quality_id" INTEGER NOT NULL,
    "upgrade_allowed" BOOLEAN NOT NULL
);

CREATE TABLE IF NOT EXISTS "quality_profile_item" (
    "id" INTEGER PRIMARY KEY AUTOINCREMENT,
    "profile_id" INTEGER NOT NULL,
    "quality_id" INTEGER NOT NULL,
    FOREIGN KEY ("profile_id") REFERENCES "quality_profile" ("id"),
    FOREIGN KEY ("quality_id") REFERENCES "quality_definition" ("quality_id")
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
    "quality_profile_id" INTEGER NOT NULL,
    "added" DATETIME DEFAULT CURRENT_TIMESTAMP,
    "tags" TEXT,
    "add_options" TEXT,
    "movie_file_id" INTEGER,
    "minimum_availability" INTEGER NOT NULL,
    "movie_metadata_id" INTEGER UNIQUE,
    "last_search_time" DATETIME
);

CREATE TABLE IF NOT EXISTS "series" (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "path" TEXT,
    "monitored" INTEGER NOT NULL,
    "added" DATETIME DEFAULT CURRENT_TIMESTAMP,
    "quality_profile_id" INTEGER NOT NULL,
    "series_metadata_id" INTEGER UNIQUE,
    "last_search_time" DATETIME
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
    "season_metadata_id" INTEGER,
    "monitored" INTEGER NOT NULL,
    FOREIGN KEY ("series_id") REFERENCES "series" ("id")
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
    "episode_metadata_id" INTEGER UNIQUE,
    "episode_file_id" INTEGER,
    FOREIGN KEY ("season_id") REFERENCES "season" ("id")
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
    "movie_id" INTEGER NOT NULL REFERENCES "movie"("id"),
    "to_state" TEXT NOT NULL,
    "from_state" TEXT,
    "most_recent" BOOLEAN NOT NULL,
    "sort_key" INTEGER NOT NULL,
    "download_client_id" INTEGER REFERENCES "download_client"("id"),
    "download_id" TEXT,
    "created_at" DATETIME DEFAULT CURRENT_TIMESTAMP,
    "updated_at" DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS "series_transition" (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "series_id" INTEGER NOT NULL REFERENCES "series"("id"),
    "to_state" TEXT NOT NULL,
    "from_state" TEXT,
    "most_recent" BOOLEAN NOT NULL,
    "sort_key" INTEGER NOT NULL,
    "created_at" DATETIME DEFAULT CURRENT_TIMESTAMP,
    "updated_at" DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS "season_transition" (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "season_id" INTEGER NOT NULL REFERENCES "season"("id"),
    "to_state" TEXT NOT NULL,
    "from_state" TEXT,
    "most_recent" BOOLEAN NOT NULL,
    "sort_key" INTEGER NOT NULL,
    "download_client_id" INTEGER REFERENCES "download_client"("id"),
    "download_id" TEXT,
    "is_entire_season_download" BOOLEAN,
    "created_at" DATETIME DEFAULT CURRENT_TIMESTAMP,
    "updated_at" DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS "episode_transition" (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "episode_id" INTEGER NOT NULL REFERENCES "episode"("id"),
    "to_state" TEXT NOT NULL,
    "from_state" TEXT,
    "most_recent" BOOLEAN NOT NULL,
    "sort_key" INTEGER NOT NULL,
    "download_client_id" INTEGER REFERENCES "download_client"("id"),
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
    "job_id" INTEGER NOT NULL REFERENCES "job"("id"),
    "type" TEXT NOT NULL,
    "to_state" TEXT NOT NULL,
    "from_state" TEXT,
    "most_recent" BOOLEAN NOT NULL,
    "sort_key" INTEGER NOT NULL,
    "error" TEXT,
    "created_at" DATETIME DEFAULT CURRENT_TIMESTAMP,
    "updated_at" DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS "idx_indexer_name" ON "indexer" ("name" ASC);

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

CREATE UNIQUE INDEX IF NOT EXISTS "idx_job_transitions_by_parent_most_recent" ON "job_transition"("job_id", "most_recent")
WHERE
    "most_recent" = 1;

CREATE UNIQUE INDEX IF NOT EXISTS "idx_job_transitions_by_parent_sort_key" ON "job_transition"("job_id", "sort_key");

CREATE UNIQUE INDEX IF NOT EXISTS "idx_job_type_pending" ON "job_transition"("type", "to_state")
WHERE
    "to_state" = 'pending' AND "most_recent" = 1;

-- Insert default data
INSERT
    OR IGNORE INTO quality_definition (
        quality_id,
        name,
        preferred_size,
        min_size,
        max_size,
        media_type
    )
VALUES
    (1, 'HDTV-720p', 1999, 17.1, 2000, 'movie'),
    (2, 'WEBDL-720p', 1999, 12.5, 2000, 'movie'),
    (3, 'WEBRip-720p', 1999, 12.5, 2000, 'movie'),
    (4, 'Bluray-720p', 1999, 25.7, 2000, 'movie'),
    (5, 'HDTV-1080p', 1999, 33.8, 2000, 'movie'),
    (6, 'WEBDL-1080p', 1999, 12.5, 2000, 'movie'),
    (7, 'WEBRip-1080p', 1999, 12.5, 2000, 'movie'),
    (8, 'Bluray-1080p', 1999, 50.8, 2000, 'movie'),
    (9, 'Remux-1080p', 1999, 102, 2000, 'movie'),
    (10, 'HDTV-2160p', 1999, 85, 2000, 'movie'),
    (11, 'WEBDL-2160p', 1999, 34.5, 2000, 'movie'),
    (12, 'WEBRip-2160p', 1999, 34.5, 2000, 'movie'),
    (13, 'Bluray-2160p', 1999, 102, 2000, 'movie'),
    (14, 'Remux-2160p', 1999, 187.4, 2000, 'movie');

INSERT
    OR IGNORE INTO quality_definition (
        quality_id,
        name,
        preferred_size,
        min_size,
        max_size,
        media_type
    )
VALUES
    (15, 'HDTV-720p', 995, 10.0, 1000, 'episode'),
    (16, 'WEBDL-720p', 995, 10.0, 1000, 'episode'),
    (17, 'WEBRip-720p', 995, 10.0, 1000, 'episode'),
    (18, 'Bluray-720p', 995, 17.1, 1000, 'episode'),
    (19, 'HDTV-1080p', 995, 15.0, 1000, 'episode'),
    (20, 'WEBDL-1080p', 995, 15.0, 1000, 'episode'),
    (21, 'WEBRip-1080p', 995, 15.0, 1000, 'episode'),
    (22, 'Bluray-1080p', 995, 50.4, 1000, 'episode'),
    (23, 'Remux-1080p', 995, 69.1, 1000, 'episode'),
    (24, 'HDTV-2160p', 995, 25.0, 1000, 'episode'),
    (25, 'WEBDL-2160p', 995, 25.0, 1000, 'episode'),
    (26, 'WEBRip-2160p', 995, 25.0, 1000, 'episode'),
    (27, 'Bluray-2160p', 995, 94.6, 1000, 'episode'),
    (28, 'Remux-2160p', 995, 187.4, 1000, 'episode');

INSERT
    OR IGNORE INTO quality_profile (id, name, cutoff_quality_id, upgrade_allowed)
VALUES
    (1, 'Standard Definition', 2, TRUE),
    (2, 'High Definition', 8, TRUE),
    (3, 'Ultra High Definition', 13, FALSE);

INSERT
    OR IGNORE INTO quality_profile_item (profile_id, quality_id)
VALUES
    (1, 1),
    (1, 2),
    (2, 3),
    (2, 4),
    (2, 5),
    (2, 6),
    (2, 7),
    (2, 8),
    (3, 9),
    (3, 10),
    (3, 11),
    (3, 12),
    (3, 13);

INSERT
    OR IGNORE INTO quality_profile (id, name, cutoff_quality_id, upgrade_allowed)
VALUES
    (4, 'Standard Definition', 16, TRUE),
    (5, 'High Definition', 23, TRUE),
    (6, 'Ultra High Definition', 27, FALSE);

INSERT
    OR IGNORE INTO quality_profile_item (profile_id, quality_id)
VALUES
    (4, 15),
    (4, 16),
    (5, 17),
    (5, 18),
    (5, 19),
    (5, 20),
    (5, 21),
    (5, 22),
    (5, 23),
    (6, 23),
    (6, 24),
    (6, 25),
    (6, 26),
    (6, 27);
