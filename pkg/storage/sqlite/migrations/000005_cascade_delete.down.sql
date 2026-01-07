PRAGMA foreign_keys=off;

CREATE TABLE "movie_transition_new" (
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

INSERT INTO "movie_transition_new" SELECT * FROM "movie_transition";
DROP TABLE "movie_transition";
ALTER TABLE "movie_transition_new" RENAME TO "movie_transition";

CREATE TABLE "series_transition_new" (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "series_id" INTEGER NOT NULL REFERENCES "series"("id"),
    "to_state" TEXT NOT NULL,
    "from_state" TEXT,
    "most_recent" BOOLEAN NOT NULL,
    "sort_key" INTEGER NOT NULL,
    "created_at" DATETIME DEFAULT CURRENT_TIMESTAMP,
    "updated_at" DATETIME DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO "series_transition_new" SELECT * FROM "series_transition";
DROP TABLE "series_transition";
ALTER TABLE "series_transition_new" RENAME TO "series_transition";

CREATE TABLE "season_new" (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "series_id" INTEGER NOT NULL,
    "season_number" INTEGER NOT NULL,
    "season_metadata_id" INTEGER,
    "monitored" INTEGER NOT NULL,
    FOREIGN KEY ("series_id") REFERENCES "series" ("id")
);

INSERT INTO "season_new" SELECT * FROM "season";
DROP TABLE "season";
ALTER TABLE "season_new" RENAME TO "season";

CREATE TABLE "season_transition_new" (
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

INSERT INTO "season_transition_new" SELECT * FROM "season_transition";
DROP TABLE "season_transition";
ALTER TABLE "season_transition_new" RENAME TO "season_transition";

CREATE TABLE "episode_new" (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "season_id" INTEGER NOT NULL,
    "episode_number" INTEGER NOT NULL,
    "monitored" INTEGER NOT NULL,
    "episode_metadata_id" INTEGER UNIQUE,
    "episode_file_id" INTEGER,
    FOREIGN KEY ("season_id") REFERENCES "season" ("id")
);

INSERT INTO "episode_new" SELECT * FROM "episode";
DROP TABLE "episode";
ALTER TABLE "episode_new" RENAME TO "episode";

CREATE TABLE "episode_transition_new" (
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

INSERT INTO "episode_transition_new" SELECT * FROM "episode_transition";
DROP TABLE "episode_transition";
ALTER TABLE "episode_transition_new" RENAME TO "episode_transition";

CREATE UNIQUE INDEX IF NOT EXISTS "idx_season_unique_series_number" ON "season" ("series_id", "season_number");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_movie_transitions_by_parent_most_recent" ON "movie_transition"("movie_id", "most_recent") WHERE "most_recent" = 1;
CREATE UNIQUE INDEX IF NOT EXISTS "idx_movie_transitions_by_parent_sort_key" ON "movie_transition"("movie_id", "sort_key");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_series_transitions_by_parent_most_recent" ON "series_transition"("series_id", "most_recent") WHERE "most_recent" = 1;
CREATE UNIQUE INDEX IF NOT EXISTS "idx_series_transitions_by_parent_sort_key" ON "series_transition"("series_id", "sort_key");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_season_transitions_by_parent_most_recent" ON "season_transition"("season_id", "most_recent") WHERE "most_recent" = 1;
CREATE UNIQUE INDEX IF NOT EXISTS "idx_season_transitions_by_parent_sort_key" ON "season_transition"("season_id", "sort_key");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_episode_transitions_by_parent_most_recent" ON "episode_transition"("episode_id", "most_recent") WHERE "most_recent" = 1;
CREATE UNIQUE INDEX IF NOT EXISTS "idx_episode_transitions_by_parent_sort_key" ON "episode_transition"("episode_id", "sort_key");

PRAGMA foreign_keys=on;
