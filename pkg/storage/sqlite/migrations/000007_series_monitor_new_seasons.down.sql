PRAGMA foreign_keys=off;

CREATE TABLE "series_new" (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "path" TEXT,
    "monitored" INTEGER NOT NULL,
    "added" DATETIME DEFAULT CURRENT_TIMESTAMP,
    "quality_profile_id" INTEGER NOT NULL,
    "series_metadata_id" INTEGER UNIQUE,
    "last_search_time" DATETIME
);

INSERT INTO "series_new" ("id", "path", "monitored", "added", "quality_profile_id", "series_metadata_id")
SELECT "id", "path", "monitored", "added", "quality_profile_id", "series_metadata_id" FROM "series";

DROP TABLE "series";
ALTER TABLE "series_new" RENAME TO "series";

PRAGMA foreign_keys=on;
