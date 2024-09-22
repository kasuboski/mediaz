CREATE TABLE IF NOT EXISTS "indexer" (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "name" TEXT NOT NULL,
    "priority" INTEGER NOT NULL DEFAULT 25,
    "uri" TEXT NOT NULL,
    "api_key" TEXT
);

CREATE TABLE IF NOT EXISTS "quality_definition" (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "quality_id" INTEGER,
    "name" TEXT NOT NULL,
    "preferred_size" NUMERIC NOT NULL,
    "min_size" NUMERIC NOT NULL,
    "max_size" NUMERIC NOT NULL,
    "media_type" TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS "quality_profile" (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "name" TEXT NOT NULL,
    "cutoff" INTEGER NOT NULL,
    "upgrade_allowed" BOOLEAN NOT NULL
);

CREATE TABLE IF NOT EXISTS "quality_item" (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "quality_id" INTEGER NOT NULL, -- quality_definition table id
    "name" TEXT NOT NULL,
    "allowed" BOOLEAN NOT NULL,
    "parent_id" INTEGER DEFAULT NULL,
    FOREIGN KEY ("parent_id") REFERENCES "quality_item" ("id"),
    FOREIGN KEY ("quality_id") REFERENCES "quality_definition"("id")
);  -- Removed trailing comma here

CREATE TABLE IF NOT EXISTS "sub_quality_item" (  -- Changed table name to singular
    "id" INTEGER PRIMARY KEY AUTOINCREMENT,
    "quality_id" INTEGER NOT NULL,
    "name" TEXT NOT NULL,
    "allowed" BOOLEAN DEFAULT 0,
    "parent_id" INTEGER NOT NULL,
    FOREIGN KEY ("parent_id") REFERENCES "quality_item"("id")  -- Changed to quality_item
);

CREATE TABLE IF NOT EXISTS "profile_quality_item" (
    "id" INTEGER PRIMARY KEY AUTOINCREMENT,
    "profile_id" INTEGER NOT NULL,
    "quality_item_id" INTEGER NOT NULL,
    FOREIGN KEY ("profile_id") REFERENCES "quality_profile" ("id"), 
    FOREIGN KEY ("quality_item_id") REFERENCES "quality_item" ("id")
);

CREATE UNIQUE INDEX IF NOT EXISTS "ix_indexer_name" ON "indexer" ("name" ASC);
