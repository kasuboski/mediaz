-- Migration 000002: Quality profile upgrade policy changes
-- 1. Makes cutoff_quality_id nullable
-- 2. Updates unmodified default profiles to disable upgrades

-- Defer foreign key checks until transaction commit (works in transactions)
PRAGMA defer_foreign_keys = ON;

DROP INDEX IF EXISTS idx_quality_definition_quality_id;
DROP INDEX IF EXISTS idx_quality_profile_item_profile_quality;

-- Recreate quality_profile table with nullable cutoff_quality_id (no FKs to worry about)
CREATE TABLE quality_profile_new (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "name" TEXT NOT NULL,
    "cutoff_quality_id" INTEGER,
    "upgrade_allowed" BOOLEAN NOT NULL
);

INSERT INTO quality_profile_new (id, name, cutoff_quality_id, upgrade_allowed)
SELECT id, name, cutoff_quality_id, upgrade_allowed
FROM quality_profile;

-- Recreate quality_profile_item WITHOUT FK constraints temporarily
CREATE TABLE quality_profile_item_new (
    "id" INTEGER PRIMARY KEY AUTOINCREMENT,
    "profile_id" INTEGER NOT NULL,
    "quality_id" INTEGER NOT NULL
);

INSERT INTO quality_profile_item_new (id, profile_id, quality_id)
SELECT id, profile_id, quality_id
FROM quality_profile_item;

-- Drop old tables
DROP TABLE quality_profile_item;
DROP TABLE quality_profile;

-- Rename new tables
ALTER TABLE quality_profile_new RENAME TO quality_profile;
ALTER TABLE quality_profile_item_new RENAME TO quality_profile_item;

-- Now add back the FK constraints by recreating the table one more time
CREATE TABLE quality_profile_item_final (
    "id" INTEGER PRIMARY KEY AUTOINCREMENT,
    "profile_id" INTEGER NOT NULL,
    "quality_id" INTEGER NOT NULL,
    FOREIGN KEY ("profile_id") REFERENCES "quality_profile" ("id"),
    FOREIGN KEY ("quality_id") REFERENCES "quality_definition" ("id")
);

INSERT INTO quality_profile_item_final (id, profile_id, quality_id)
SELECT id, profile_id, quality_id
FROM quality_profile_item;

DROP TABLE quality_profile_item;
ALTER TABLE quality_profile_item_final RENAME TO quality_profile_item;

CREATE UNIQUE INDEX IF NOT EXISTS "idx_quality_profile_item_profile_quality" ON "quality_profile_item" ("profile_id", "quality_id");

-- Update unmodified default profiles to disable upgrades and set NULL cutoff
-- A profile is "unmodified" if ALL fields match original values
UPDATE quality_profile
SET cutoff_quality_id = NULL, upgrade_allowed = FALSE
WHERE
    (id = 1 AND name = 'Standard Definition' AND cutoff_quality_id = 2 AND upgrade_allowed = TRUE)
    OR (id = 2 AND name = 'High Definition' AND cutoff_quality_id = 8 AND upgrade_allowed = TRUE)
    OR (id = 3 AND name = 'Ultra High Definition' AND cutoff_quality_id = 13 AND upgrade_allowed = FALSE)
    OR (id = 4 AND name = 'Standard Definition' AND cutoff_quality_id = 16 AND upgrade_allowed = TRUE)
    OR (id = 5 AND name = 'High Definition' AND cutoff_quality_id = 23 AND upgrade_allowed = TRUE)
    OR (id = 6 AND name = 'Ultra High Definition' AND cutoff_quality_id = 27 AND upgrade_allowed = FALSE);
