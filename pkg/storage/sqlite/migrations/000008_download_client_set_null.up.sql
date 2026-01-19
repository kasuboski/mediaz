-- Migration 000008: Add ON DELETE SET NULL to download_client_id foreign keys
-- This allows download clients to be deleted while preserving transition state history
-- The download_client_id will be set to NULL when the referenced client is deleted

PRAGMA foreign_keys=off;

-- Recreate movie_transition with ON DELETE SET NULL
CREATE TABLE "movie_transition_new" (
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

INSERT INTO "movie_transition_new" SELECT * FROM "movie_transition";
DROP TABLE "movie_transition";
ALTER TABLE "movie_transition_new" RENAME TO "movie_transition";

-- Recreate season_transition with ON DELETE SET NULL
CREATE TABLE "season_transition_new" (
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

INSERT INTO "season_transition_new" SELECT * FROM "season_transition";
DROP TABLE "season_transition";
ALTER TABLE "season_transition_new" RENAME TO "season_transition";

-- Recreate episode_transition with ON DELETE SET NULL
CREATE TABLE "episode_transition_new" (
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

INSERT INTO "episode_transition_new" SELECT * FROM "episode_transition";
DROP TABLE "episode_transition";
ALTER TABLE "episode_transition_new" RENAME TO "episode_transition";

-- Recreate indexes
CREATE UNIQUE INDEX IF NOT EXISTS "idx_movie_transitions_by_parent_most_recent" ON "movie_transition"("movie_id", "most_recent") WHERE "most_recent" = 1;
CREATE UNIQUE INDEX IF NOT EXISTS "idx_movie_transitions_by_parent_sort_key" ON "movie_transition"("movie_id", "sort_key");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_season_transitions_by_parent_most_recent" ON "season_transition"("season_id", "most_recent") WHERE "most_recent" = 1;
CREATE UNIQUE INDEX IF NOT EXISTS "idx_season_transitions_by_parent_sort_key" ON "season_transition"("season_id", "sort_key");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_episode_transitions_by_parent_most_recent" ON "episode_transition"("episode_id", "most_recent") WHERE "most_recent" = 1;
CREATE UNIQUE INDEX IF NOT EXISTS "idx_episode_transitions_by_parent_sort_key" ON "episode_transition"("episode_id", "sort_key");

PRAGMA foreign_keys=on;
