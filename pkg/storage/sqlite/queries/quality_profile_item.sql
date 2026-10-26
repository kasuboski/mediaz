-- name: CreateQualityProfileItem :one
INSERT INTO quality_profile_item (profile_id, quality_id)
VALUES (?, ?)
RETURNING id;

-- name: GetQualityProfileItem :one
SELECT
    id,
    profile_id,
    quality_id
FROM quality_profile_item
WHERE id = ?;

-- name: ListQualityProfileItems :many
SELECT
    id,
    profile_id,
    quality_id
FROM quality_profile_item
ORDER BY id ASC;

-- name: DeleteQualityProfileItem :exec
DELETE FROM quality_profile_item WHERE id = ?;

-- name: DeleteQualityProfileItemsByProfileID :exec
DELETE FROM quality_profile_item WHERE profile_id = ?;
