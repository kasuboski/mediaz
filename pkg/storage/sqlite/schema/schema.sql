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

CREATE UNIQUE INDEX IF NOT EXISTS "idx_quality_definition_quality_id" ON "quality_definition" ("quality_id");

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
    "date_added" DATETIME NOT NULL DEFAULT current_timestamp,
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
    "added" DATETIME DEFAULT current_timestamp,
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
    "added" DATETIME DEFAULT current_timestamp,
    "quality_profile_id" INTEGER NOT NULL,
    "series_metadata_id" INTEGER UNIQUE,
    "last_search_time" DATETIME
);

CREATE TABLE IF NOT EXISTS "series_metadata" (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "tmdb_id" INTEGER NOT NULL UNIQUE,
    "title" TEXT NOT NULL,
    "overview" TEXT,
    "last_info_sync" DATETIME,
    "first_air_date" DATETIME,
    "last_air_date" DATETIME,
    "season_count" INTEGER NOT NULL,
    "episode_count" INTEGER NOT NULL,
    "status" TEXT NOT NULL,
    "poster_path" TEXT
);

CREATE TABLE IF NOT EXISTS "season" (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "series_id" INTEGER NOT NULL,
    "season_number" INTEGER NOT NULL,
    "season_metadata_id" INTEGER,
    "monitored" INTEGER NOT NULL,
    FOREIGN KEY ("series_id") REFERENCES "series" ("id")
);

CREATE TABLE IF NOT EXISTS "season_metadata" (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "series_id" INTEGER NOT NULL,
    "number" INTEGER NOT NULL,
    "tmdb_id" INTEGER NOT NULL UNIQUE,
    "title" TEXT NOT NULL,
    "overview" TEXT,
    "air_date" DATETIME,
    FOREIGN KEY ("series_id") REFERENCES "series" ("id")
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
    "added" DATETIME NOT NULL DEFAULT current_timestamp,
    "relative_path" TEXT UNIQUE,
    "original_file_path" TEXT
);

CREATE TABLE IF NOT EXISTS "episode_metadata" (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "season_id" INTEGER NOT NULL,
    "number" INTEGER NOT NULL,
    "tmdb_id" INTEGER NOT NULL UNIQUE,
    "title" TEXT NOT NULL,
    "overview" TEXT,
    "air_date" DATETIME,
    "runtime" INTEGER,
    FOREIGN KEY ("season_id") REFERENCES "season" ("id")
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