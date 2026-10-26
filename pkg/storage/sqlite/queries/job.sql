-- name: GetJobByTypeAndState :one
SELECT
    j.id,
    j.type,
    j.created_at,
    jt.to_state,
    jt.updated_at,
    jt.error
FROM job AS j
INNER JOIN job_transition AS jt ON j.id = jt.job_id
WHERE
    jt.type = ?
    AND jt.to_state = ?
    AND jt.most_recent = 1;

-- name: CreateJob :one
INSERT INTO job (type) VALUES (?)
RETURNING id;

-- name: GetJobByID :one
SELECT
    j.id,
    j.type,
    j.created_at,
    jt.to_state,
    jt.updated_at,
    jt.error
FROM job AS j
INNER JOIN job_transition AS jt ON j.id = jt.job_id
WHERE
    j.id = ?
    AND jt.most_recent = 1;

-- name: CountJobs :one
SELECT COUNT(*) AS count
FROM job AS j
INNER JOIN job_transition AS jt ON j.id = jt.job_id
WHERE
    jt.most_recent = 1
    AND (
        sqlc.narg('job_type') IS NULL
        OR j.type = sqlc.narg('job_type')
    )
    AND (
        sqlc.narg('state') IS NULL
        OR jt.to_state = sqlc.narg('state')
    );

-- name: ListJobs :many
SELECT
    j.id,
    j.type,
    j.created_at,
    jt.to_state,
    jt.updated_at,
    jt.error
FROM job AS j
INNER JOIN job_transition AS jt ON j.id = jt.job_id
WHERE
    jt.most_recent = 1
    AND (
        sqlc.narg('job_type') IS NULL
        OR j.type = sqlc.narg('job_type')
    )
    AND (
        sqlc.narg('state') IS NULL
        OR jt.to_state = sqlc.narg('state')
    )
ORDER BY j.created_at DESC
LIMIT COALESCE(NULLIF(sqlc.arg('lim'), 0), -1) OFFSET sqlc.arg('offs');

-- name: DeactivateJobTransition :one
UPDATE job_transition
SET
    most_recent = 0,
    updated_at = DATETIME(sqlc.arg('updated_at'))
WHERE job_id = sqlc.arg('job_id') AND most_recent = 1
RETURNING id, job_id, type, to_state, from_state, most_recent, sort_key, error, created_at, updated_at;

-- name: CreateJobTransition :exec
INSERT INTO job_transition (
    job_id, type, to_state, from_state, most_recent, sort_key, error
) VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: DeleteJob :exec
DELETE FROM job WHERE id = ?;

-- name: DeleteJobsByIDs :execrows
DELETE FROM job WHERE id IN (sqlc.slice('ids'));
