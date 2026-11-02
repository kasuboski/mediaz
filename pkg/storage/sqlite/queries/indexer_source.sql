-- name: CreateIndexerSource :one
INSERT INTO indexer_source (name, implementation, scheme, host, port, api_key, enabled, api_path)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: GetIndexerSource :one
SELECT id, name, implementation, scheme, host, port, api_key, enabled, created_at, updated_at, api_path
FROM indexer_source WHERE id = ?;

-- name: ListIndexerSources :many
SELECT id, name, implementation, scheme, host, port, api_key, enabled, created_at, updated_at, api_path
FROM indexer_source ORDER BY name ASC;

-- name: ListEnabledIndexerSources :many
SELECT id, name, implementation, scheme, host, port, api_key, enabled, created_at, updated_at, api_path
FROM indexer_source WHERE enabled = 1 ORDER BY name ASC, id ASC;

-- name: UpdateIndexerSource :exec
UPDATE indexer_source SET name = ?, implementation = ?, scheme = ?, host = ?, port = ?, enabled = ?, api_key = COALESCE(?, api_key), api_path = ? WHERE id = ?;

-- name: DeleteIndexerSource :exec
DELETE FROM indexer_source WHERE id = ?;

-- name: DeleteIndexersBySourceID :exec
DELETE FROM indexer WHERE indexer_source_id = ?;
