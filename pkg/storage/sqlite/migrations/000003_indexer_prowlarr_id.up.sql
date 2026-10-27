ALTER TABLE indexer ADD COLUMN prowlarr_id INTEGER;
DROP INDEX IF EXISTS idx_indexer_unique_source_name;
CREATE UNIQUE INDEX idx_indexer_unique_source_prowlarr_id
    ON indexer (indexer_source_id, prowlarr_id);
