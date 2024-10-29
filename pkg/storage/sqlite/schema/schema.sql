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

CREATE TABLE IF NOT EXISTS "movie_file" (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "movie_id" INTEGER UNIQUE NOT NULL,
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
    "physical_release" DATETIME,
    "digital_release" DATETIME,
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
    "added" DATETIME,
    "tags" TEXT,
    "add_options" TEXT,
    "movie_file_id" INTEGER,
    "minimum_availability" INTEGER NOT NULL,
    "movie_metadata_id" INTEGER UNIQUE,
    "last_search_time" DATETIME
);

CREATE TABLE IF NOT EXISTS "download_client" (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "type" TEXT NOT NULL, -- usenet or torrent
    "implementation" TEXT NOT NULL,
    "scheme" TEXT NOT NULL,
    "host" TEXT NOT NULL,
    "port" INTEGER NOT NULL,
    "api_key" TEXT
);

CREATE UNIQUE INDEX IF NOT EXISTS "ix_indexer_name" ON "indexer" ("name" ASC);
