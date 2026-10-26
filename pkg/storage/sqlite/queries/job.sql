-- Job queries. Entity reads join the most recent job_transition via
-- embed(j) plus named transition columns. ListJobs paginates with a
-- stable ORDER BY (created_at DESC, id DESC) so same-second rows never
-- duplicate/skip across pages (A4). The demote statement stamps updated_at
-- with CURRENT_TIMESTAMP (UTC) - no application-formatted timestamps (B5).

-- name: GetJobByTypeAndState :one
SELECT
    sqlc.embed(j),
    jt.to_state AS state,
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
    sqlc.embed(j),
    jt.to_state AS state,
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
-- Stable pagination: created_at DESC with id DESC tie-break (CURRENT_TIMESTAMP
-- has 1-second resolution). lim = 0 means unlimited.
SELECT
    sqlc.embed(j),
    jt.to_state AS state,
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
ORDER BY j.created_at DESC, j.id DESC
LIMIT COALESCE(NULLIF(sqlc.arg('lim'), 0), -1) OFFSET sqlc.arg('offs');

-- name: DeactivateJobTransition :one
UPDATE job_transition
SET
    most_recent = 0,
    updated_at = CURRENT_TIMESTAMP
WHERE job_id = sqlc.arg('job_id') AND most_recent = 1
RETURNING id, job_id, type, to_state, from_state, most_recent, sort_key, error, created_at, updated_at;

-- name: CreateJobTransition :exec
INSERT INTO job_transition (
    job_id, type, to_state, from_state, most_recent, sort_key, error
) VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: DeleteJob :exec
DELETE FROM job WHERE id = ?;

-- name: DeleteJobsByIDs :execrows
-- An empty ids slice expands to IN (NULL) and deletes nothing (no-op).
DELETE FROM job WHERE id IN (sqlc.slice('ids'));
