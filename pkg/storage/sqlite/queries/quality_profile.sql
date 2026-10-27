-- Profiles and joined definition rows remain generated types throughout storage.

-- name: GetQualityProfile :one
SELECT * FROM quality_profile WHERE id = sqlc.arg('id');

-- name: ListQualityProfiles :many
SELECT qp.* FROM quality_profile AS qp
LEFT JOIN quality_profile_item AS qpi ON qpi.profile_id = qp.id
LEFT JOIN quality_definition AS qd ON qd.id = qpi.quality_id
GROUP BY qp.id
ORDER BY MAX(qd.min_size) DESC, qp.id;

-- name: ListQualityProfilesByMediaType :many
SELECT qp.* FROM quality_profile AS qp
JOIN quality_profile_item AS qpi ON qpi.profile_id = qp.id
JOIN quality_definition AS qd ON qd.id = qpi.quality_id
WHERE qd.media_type = sqlc.arg('media_type')
GROUP BY qp.id
ORDER BY MAX(qd.min_size) DESC, qp.id;

-- name: CreateQualityProfile :one
INSERT INTO quality_profile (name, cutoff_quality_id, upgrade_allowed)
VALUES (?, ?, ?)
RETURNING id;

-- name: UpdateQualityProfile :exec
UPDATE quality_profile
SET
    name = ?,
    cutoff_quality_id = ?,
    upgrade_allowed = ?
WHERE id = ?;

-- name: DeleteQualityProfile :exec
DELETE FROM quality_profile WHERE id = ?;

-- name: ListQualityProfileRows :many
SELECT
    sqlc.embed(qp),
    sqlc.embed(qd)
FROM quality_profile AS qp
INNER JOIN quality_profile_item AS qpi ON qpi.profile_id = qp.id
INNER JOIN quality_definition AS qd ON qd.id = qpi.quality_id
WHERE
    (
        sqlc.narg('profile_id') IS NULL
        OR qp.id = sqlc.narg('profile_id')
    )
    AND (
        sqlc.narg('media_type') IS NULL
        OR qd.media_type = sqlc.narg('media_type')
    )
ORDER BY qd.min_size DESC, qp.id, qd.id;
