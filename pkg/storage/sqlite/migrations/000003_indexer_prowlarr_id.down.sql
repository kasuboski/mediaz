DROP INDEX IF EXISTS idx_indexer_unique_source_name;
DROP INDEX IF EXISTS idx_indexer_unique_source_prowlarr_id;
CREATE UNIQUE INDEX idx_indexer_unique_source_name ON indexer (indexer_source_id, name);
ALTER TABLE indexer DROP COLUMN prowlarr_id;
