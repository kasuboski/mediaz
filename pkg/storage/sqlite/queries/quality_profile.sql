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
    qp.id AS profile_id,
    qp.name AS profile_name,
    qp.cutoff_quality_id AS profile_cutoff_quality_id,
    qp.upgrade_allowed AS profile_upgrade_allowed,
    qd.id AS quality_id,
    qd.name AS quality_name,
    qd.preferred_size,
    qd.min_size,
    qd.max_size,
    qd.media_type
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
ORDER BY qd.min_size DESC;
