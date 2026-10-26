-- Episode file queries. Mirrors the legacy Jet statements in episode_file.go:
--   - CreateEpisodeFile inserts the id column only when it is non-zero, and
--     omits "added" in the auto-increment branch so the column default
--     CURRENT_TIMESTAMP applies (CreateEpisodeFileWithID carries both).
--   - The legacy ON CONFLICT (id) DO UPDATE assigned every mutable column to
--     its own current value (episode_file."col" refers to the existing row), so
--     a conflicting insert never overwrites the stored row and returns the
--     existing id; both insert statements reproduce that behavior exactly.
--   - GetEpisodeFileByID returns every matching row (the caller maps an empty
--     result to storage.ErrNotFound), ListEpisodeFiles orders by id ASC, and
--     UpdateEpisodeFile sets every mutable column like the legacy
--     MutableColumns update.

-- name: CreateEpisodeFile :one
INSERT INTO
    episode_file (
        "quality",
        "size",
        "relative_path",
        "original_file_path"
    )
VALUES (
    sqlc.arg('quality'),
    sqlc.arg('size'),
    sqlc.narg('relative_path'),
    sqlc.narg('original_file_path')
)
ON CONFLICT ("id") DO UPDATE SET
    "quality" = episode_file."quality",
    "size" = episode_file."size",
    "added" = episode_file."added",
    "relative_path" = episode_file."relative_path",
    "original_file_path" = episode_file."original_file_path"
RETURNING "id";

-- name: CreateEpisodeFileWithID :one
INSERT INTO
    episode_file (
        "id",
        "quality",
        "size",
        "added",
        "relative_path",
        "original_file_path"
    )
VALUES (
    sqlc.arg('id'),
    sqlc.arg('quality'),
    sqlc.arg('size'),
    sqlc.arg('added'),
    sqlc.narg('relative_path'),
    sqlc.narg('original_file_path')
)
ON CONFLICT ("id") DO UPDATE SET
    "quality" = episode_file."quality",
    "size" = episode_file."size",
    "added" = episode_file."added",
    "relative_path" = episode_file."relative_path",
    "original_file_path" = episode_file."original_file_path"
RETURNING "id";

-- name: GetEpisodeFileByID :many
SELECT
    id,
    quality,
    size,
    added,
    relative_path,
    original_file_path
FROM
    episode_file
WHERE
    id = ?;

-- name: GetEpisodeFile :one
SELECT
    id,
    quality,
    size,
    added,
    relative_path,
    original_file_path
FROM
    episode_file
WHERE
    id = ?;

-- name: UpdateEpisodeFile :exec
UPDATE
    episode_file
SET
    "quality" = ?,
    "size" = ?,
    "added" = ?,
    "relative_path" = ?,
    "original_file_path" = ?
WHERE
    id = ?;

-- name: DeleteEpisodeFile :exec
DELETE FROM episode_file WHERE id = ?;

-- name: ListEpisodeFiles :many
SELECT
    id,
    quality,
    size,
    added,
    relative_path,
    original_file_path
FROM
    episode_file
ORDER BY
    id ASC;
