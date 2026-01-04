DROP INDEX IF EXISTS "idx_indexer_name";

CREATE TABLE IF NOT EXISTS "indexer_source" (
    "id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    "name" TEXT NOT NULL,
    "implementation" TEXT NOT NULL,
    "scheme" TEXT NOT NULL DEFAULT 'http',
    "host" TEXT NOT NULL,
    "port" INTEGER,
    "api_key" TEXT,
    "enabled" BOOLEAN NOT NULL DEFAULT 1,
    "created_at" DATETIME DEFAULT CURRENT_TIMESTAMP,
    "updated_at" DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS "idx_indexer_source_implementation" ON "indexer_source" ("implementation");
CREATE INDEX IF NOT EXISTS "idx_indexer_source_enabled" ON "indexer_source" ("enabled");

ALTER TABLE "indexer" ADD COLUMN "indexer_source_id" INTEGER REFERENCES "indexer_source"("id") ON DELETE CASCADE;

DELETE FROM "indexer";

CREATE INDEX IF NOT EXISTS "idx_indexer_by_source" ON "indexer" ("indexer_source_id");
CREATE UNIQUE INDEX IF NOT EXISTS "idx_indexer_unique_source_name" ON "indexer" ("indexer_source_id", "name");
