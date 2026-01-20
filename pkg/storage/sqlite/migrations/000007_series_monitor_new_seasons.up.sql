PRAGMA foreign_keys=off;

CREATE TABLE "series_new" (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "path" TEXT,
    "monitored" INTEGER NOT NULL,
    "added" DATETIME DEFAULT CURRENT_TIMESTAMP,
    "quality_profile_id" INTEGER NOT NULL,
    "series_metadata_id" INTEGER UNIQUE,
    "monitor_new_seasons" INTEGER NOT NULL DEFAULT 0
);

INSERT INTO "series_new" ("id", "path", "monitored", "added", "quality_profile_id", "series_metadata_id", "monitor_new_seasons")
SELECT "id", "path", "monitored", "added", "quality_profile_id", "series_metadata_id", 0 FROM "series";

DROP TABLE "series";
ALTER TABLE "series_new" RENAME TO "series";

PRAGMA foreign_keys=on;
