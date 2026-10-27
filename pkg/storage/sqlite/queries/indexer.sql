-- name: CreateIndexer :one
INSERT INTO indexer (name, priority, uri, api_key, indexer_source_id, categories, prowlarr_id)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (indexer_source_id, prowlarr_id) DO UPDATE SET
 priority = excluded.priority,
 uri = excluded.uri,
 api_key = COALESCE(excluded.api_key, indexer.api_key),
 categories = excluded.categories,
 name = excluded.name
RETURNING id;

-- name: UpdateIndexer :execrows
UPDATE indexer
SET name = sqlc.arg('name'), priority = sqlc.arg('priority'), uri = sqlc.arg('uri'),
    api_key = COALESCE(sqlc.narg('api_key'), api_key),
    categories = COALESCE(NULLIF(CAST(sqlc.arg('categories') AS TEXT), ''), categories)
WHERE id = sqlc.arg('id') AND indexer_source_id IS NULL;

-- name: DeleteIndexer :execrows
DELETE FROM indexer WHERE id = ? AND indexer_source_id IS NULL;

-- name: ListIndexers :many
SELECT id, name, priority, uri, api_key, indexer_source_id, categories, prowlarr_id
FROM indexer
ORDER BY priority DESC, id;
