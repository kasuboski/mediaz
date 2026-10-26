-- name: GetMovieFilesByMovieName :many
SELECT
    mf.id,
    mf.quality,
    mf.size,
    mf.date_added,
    mf.scene_name,
    mf.media_info,
    mf.release_group,
    mf.relative_path,
    mf.edition,
    mf.languages,
    mf.indexer_flags,
    mf.original_file_path
FROM movie_file AS mf
INNER JOIN movie AS m ON mf.relative_path LIKE (m.path || '%')
WHERE m.path = ?;

-- name: CreateMovieFile :one
INSERT INTO movie_file (
    quality, size, scene_name, media_info, release_group, relative_path,
    edition, languages, indexer_flags, original_file_path
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: DeleteMovieFile :exec
DELETE FROM movie_file WHERE id = ?;

-- name: ListMovieFiles :many
SELECT
    id,
    quality,
    size,
    date_added,
    scene_name,
    media_info,
    release_group,
    relative_path,
    edition,
    languages,
    indexer_flags,
    original_file_path
FROM movie_file
ORDER BY id ASC;
