-- Migration 000002 down: Rollback quality profile upgrade policy changes
-- NOTE: This rollback preserves user modifications but makes best-effort guesses for NULL values
-- WARNING: This reintroduces the foreign key bug (quality_id -> quality_id instead of id)

PRAGMA foreign_keys = OFF;

-- Recreate quality_profile table with NOT NULL constraint
CREATE TABLE quality_profile_new (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "name" TEXT NOT NULL,
    "cutoff_quality_id" INTEGER NOT NULL,
    "upgrade_allowed" BOOLEAN NOT NULL
);

-- Preserve existing values where possible; use defaults for NULL cutoffs
INSERT INTO quality_profile_new (id, name, cutoff_quality_id, upgrade_allowed)
SELECT
    id,
    name,
    COALESCE(cutoff_quality_id,
        CASE id
            WHEN 1 THEN 2
            WHEN 2 THEN 8
            WHEN 3 THEN 13
            WHEN 4 THEN 16
            WHEN 5 THEN 23
            WHEN 6 THEN 27
            ELSE 2
        END
    ) as cutoff_quality_id,
    upgrade_allowed
FROM quality_profile;

DROP TABLE quality_profile;
ALTER TABLE quality_profile_new RENAME TO quality_profile;

-- Recreate quality_profile_item table with old (incorrect) foreign key
-- WARNING: This reintroduces the foreign key bug for backwards compatibility
CREATE TABLE quality_profile_item_new (
    "id" INTEGER PRIMARY KEY AUTOINCREMENT,
    "profile_id" INTEGER NOT NULL,
    "quality_id" INTEGER NOT NULL,
    FOREIGN KEY ("profile_id") REFERENCES "quality_profile" ("id"),
    FOREIGN KEY ("quality_id") REFERENCES "quality_definition" ("quality_id")
);

INSERT INTO quality_profile_item_new (id, profile_id, quality_id)
SELECT id, profile_id, quality_id
FROM quality_profile_item;

DROP TABLE quality_profile_item;
ALTER TABLE quality_profile_item_new RENAME TO quality_profile_item;

CREATE UNIQUE INDEX IF NOT EXISTS "idx_quality_profile_item_profile_quality" ON "quality_profile_item" ("profile_id", "quality_id");

PRAGMA foreign_keys = ON;
