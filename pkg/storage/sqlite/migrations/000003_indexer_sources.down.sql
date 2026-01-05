DROP INDEX "idx_indexer_by_source";

DELETE FROM "indexer";

ALTER TABLE "indexer" DROP COLUMN "indexer_source_id";

DROP INDEX "idx_indexer_source_enabled";
DROP INDEX "idx_indexer_source_implementation";

DROP TABLE "indexer_source";
