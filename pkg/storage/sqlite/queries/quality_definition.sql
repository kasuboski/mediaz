-- name: CreateQualityDefinition :one
INSERT INTO quality_definition (quality_id, name, preferred_size, min_size, max_size, media_type)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: GetQualityDefinition :one
SELECT
    id,
    quality_id,
    name,
    preferred_size,
    min_size,
    max_size,
    media_type
FROM quality_definition
WHERE id = ?
ORDER BY id ASC;

-- name: ListQualityDefinitions :many
SELECT
    id,
    quality_id,
    name,
    preferred_size,
    min_size,
    max_size,
    media_type
FROM quality_definition
ORDER BY id ASC;

-- name: UpdateQualityDefinition :exec
UPDATE quality_definition
SET
    name = ?,
    preferred_size = ?,
    min_size = ?,
    max_size = ?,
    media_type = ?
WHERE id = ?;

-- name: DeleteQualityDefinition :exec
DELETE FROM quality_definition WHERE id = ?;
