-- Episode files with explicit IDs upsert mutable fields.

-- name: CreateEpisodeFile :one
INSERT INTO episode_file (quality, size, relative_path, original_file_path)
VALUES (sqlc.arg('quality'), sqlc.arg('size'), sqlc.narg('relative_path'), sqlc.narg('original_file_path'))
RETURNING id;

-- name: CreateEpisodeFileWithID :one
INSERT INTO episode_file (id, quality, size, added, relative_path, original_file_path)
VALUES (sqlc.arg('id'), sqlc.arg('quality'), sqlc.arg('size'), sqlc.arg('added'), sqlc.narg('relative_path'), sqlc.narg('original_file_path'))
ON CONFLICT (id) DO UPDATE SET
 quality = excluded.quality,
 size = excluded.size,
 added = excluded.added,
 relative_path = excluded.relative_path,
 original_file_path = excluded.original_file_path
RETURNING id;

-- name: GetEpisodeFileByID :many
SELECT * FROM episode_file WHERE id = sqlc.arg('id');

-- name: GetEpisodeFile :one
SELECT * FROM episode_file WHERE id = sqlc.arg('id');

-- name: UpdateEpisodeFile :exec
UPDATE episode_file SET quality = sqlc.arg('quality'), size = sqlc.arg('size'), added = sqlc.arg('added'), relative_path = sqlc.narg('relative_path'), original_file_path = sqlc.narg('original_file_path') WHERE id = sqlc.arg('id');

-- name: DeleteEpisodeFile :exec
DELETE FROM episode_file WHERE id = sqlc.arg('id');

-- name: ListEpisodeFiles :many
SELECT * FROM episode_file ORDER BY id;
