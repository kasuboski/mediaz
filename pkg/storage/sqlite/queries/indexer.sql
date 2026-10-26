-- name: CreateIndexer :execresult
INSERT INTO indexer (name, priority, uri, api_key, indexer_source_id)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT (indexer_source_id, name) DO NOTHING;

-- name: UpdateIndexer :exec
UPDATE indexer
SET
    name = ?,
    priority = ?,
    uri = ?,
    api_key = COALESCE(?, api_key)
WHERE id = ?;

-- name: DeleteIndexer :exec
DELETE FROM indexer WHERE id = ?;

-- name: ListIndexers :many
SELECT
    id,
    name,
    priority,
    uri,
    api_key,
    indexer_source_id
FROM indexer
ORDER BY priority DESC;
