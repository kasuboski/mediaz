PRAGMA foreign_keys=off;

CREATE TABLE "season_new" (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "series_id" INTEGER NOT NULL,
    "season_number" INTEGER NOT NULL,
    "season_metadata_id" INTEGER,
    "monitored" INTEGER NOT NULL,
    "last_search_time" DATETIME,
    FOREIGN KEY ("series_id") REFERENCES "series" ("id") ON DELETE CASCADE
);

INSERT INTO "season_new" ("id", "series_id", "season_number", "season_metadata_id", "monitored")
SELECT "id", "series_id", "season_number", "season_metadata_id", "monitored" FROM "season";

DROP TABLE "season";
ALTER TABLE "season_new" RENAME TO "season";

CREATE UNIQUE INDEX IF NOT EXISTS "idx_season_unique_series_number" ON "season" ("series_id", "season_number");

PRAGMA foreign_keys=on;
