-- name: CreateIndexer :one
INSERT INTO indexer (name, priority, uri, api_key, indexer_source_id, categories)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (indexer_source_id, name) DO UPDATE SET
 priority = excluded.priority,
 uri = excluded.uri,
 api_key = COALESCE(excluded.api_key, indexer.api_key),
 categories = excluded.categories
RETURNING id;

-- name: GetIndexerBySourceAndName :one
SELECT
    id,
    name,
    priority,
    uri,
    api_key,
    indexer_source_id,
    categories
FROM indexer
WHERE indexer_source_id IS ?
  AND name = ?;

-- name: UpdateIndexer :exec
UPDATE indexer
SET
    name = sqlc.arg('name'),
    priority = sqlc.arg('priority'),
    uri = sqlc.arg('uri'),
    api_key = COALESCE(sqlc.narg('api_key'), api_key),
    categories = COALESCE(NULLIF(CAST(sqlc.arg('categories') AS TEXT), ''), categories)
WHERE id = sqlc.arg('id');

-- name: DeleteIndexer :exec
DELETE FROM indexer WHERE id = ?;

-- name: ListIndexers :many
SELECT
    id,
    name,
    priority,
    uri,
    api_key,
    indexer_source_id,
    categories
FROM indexer
ORDER BY priority DESC, id;
