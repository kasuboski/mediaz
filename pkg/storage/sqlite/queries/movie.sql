-- name: CreateMovie :one
INSERT INTO movie (
    path, monitored, quality_profile_id, tags, add_options, movie_file_id,
    minimum_availability, movie_metadata_id, last_search_time
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (id) DO UPDATE SET
    path = excluded.path,
    monitored = excluded.monitored,
    quality_profile_id = excluded.quality_profile_id,
    tags = excluded.tags,
    add_options = excluded.add_options,
    movie_file_id = excluded.movie_file_id,
    minimum_availability = excluded.minimum_availability,
    movie_metadata_id = excluded.movie_metadata_id,
    last_search_time = excluded.last_search_time
RETURNING id;

-- name: CreateMovieWithAdded :one
INSERT INTO movie (
    added, path, monitored, quality_profile_id, tags, add_options, movie_file_id,
    minimum_availability, movie_metadata_id, last_search_time
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (id) DO UPDATE SET
    added = excluded.added,
    path = excluded.path,
    monitored = excluded.monitored,
    quality_profile_id = excluded.quality_profile_id,
    tags = excluded.tags,
    add_options = excluded.add_options,
    movie_file_id = excluded.movie_file_id,
    minimum_availability = excluded.minimum_availability,
    movie_metadata_id = excluded.movie_metadata_id,
    last_search_time = excluded.last_search_time
RETURNING id;

-- name: GetMovieByID :one
SELECT
    m.id,
    m.path,
    m.monitored,
    m.quality_profile_id,
    m.added,
    m.tags,
    m.add_options,
    m.movie_file_id,
    m.minimum_availability,
    m.movie_metadata_id,
    m.last_search_time,
    mt.to_state,
    mt.download_client_id,
    mt.download_id
FROM movie AS m
INNER JOIN movie_transition AS mt ON m.id = mt.movie_id AND mt.most_recent = 1
WHERE m.id = ?;

-- name: GetMovieByFileID :one
SELECT
    m.id,
    m.path,
    m.monitored,
    m.quality_profile_id,
    m.added,
    m.tags,
    m.add_options,
    m.movie_file_id,
    m.minimum_availability,
    m.movie_metadata_id,
    m.last_search_time,
    mt.to_state,
    mt.download_client_id,
    mt.download_id
FROM movie AS m
INNER JOIN movie_transition AS mt ON m.id = mt.movie_id AND mt.most_recent = 1
WHERE m.movie_file_id = ?;

-- name: ListMovies :many
SELECT
    m.id,
    m.path,
    m.monitored,
    m.quality_profile_id,
    m.added,
    m.tags,
    m.add_options,
    m.movie_file_id,
    m.minimum_availability,
    m.movie_metadata_id,
    m.last_search_time,
    mt.to_state,
    mt.download_client_id,
    mt.download_id
FROM movie AS m
INNER JOIN movie_transition AS mt ON m.id = mt.movie_id AND mt.most_recent = 1
LEFT JOIN movie_metadata AS mm ON m.movie_metadata_id = mm.id
ORDER BY mm.title ASC;

-- name: ListMoviesByState :many
SELECT
    m.id,
    m.path,
    m.monitored,
    m.quality_profile_id,
    m.added,
    m.tags,
    m.add_options,
    m.movie_file_id,
    m.minimum_availability,
    m.movie_metadata_id,
    m.last_search_time,
    mt.to_state,
    mt.download_client_id,
    mt.download_id
FROM movie AS m
INNER JOIN movie_transition AS mt ON m.id = mt.movie_id AND mt.most_recent = 1
WHERE mt.to_state = ?
ORDER BY m.added ASC;

-- name: GetMovieByMetadataIDQuery :one
SELECT
    m.id,
    m.path,
    m.monitored,
    m.quality_profile_id,
    m.added,
    m.tags,
    m.add_options,
    m.movie_file_id,
    m.minimum_availability,
    m.movie_metadata_id,
    m.last_search_time,
    mt.to_state
FROM movie AS m
INNER JOIN movie_transition AS mt ON m.id = mt.movie_id AND mt.most_recent = 1
WHERE m.movie_metadata_id = ?;

-- name: GetMovieByPathQuery :one
SELECT
    m.id,
    m.path,
    m.monitored,
    m.quality_profile_id,
    m.added,
    m.tags,
    m.add_options,
    m.movie_file_id,
    m.minimum_availability,
    m.movie_metadata_id,
    m.last_search_time,
    mt.to_state,
    mt.download_client_id,
    mt.download_id
FROM movie AS m
INNER JOIN movie_transition AS mt ON m.id = mt.movie_id AND mt.most_recent = 1
WHERE m.path = ?;

-- name: DeleteMovie :exec
DELETE FROM movie WHERE id = ?;

-- name: UpdateMovieMovieFileIDQuery :exec
UPDATE movie SET movie_file_id = ? WHERE id = ?;

-- name: UpdateMovieMonitored :exec
UPDATE movie SET monitored = ? WHERE id = ?;

-- name: UpdateMovieQualityProfileID :exec
UPDATE movie SET quality_profile_id = ? WHERE id = ?;

-- name: LinkMovieMetadata :exec
UPDATE movie SET movie_metadata_id = ? WHERE id = ?;

-- name: DeactivateMovieTransition :one
UPDATE movie_transition
SET
    most_recent = 0,
    updated_at = DATETIME(sqlc.arg('updated_at'))
WHERE movie_id = sqlc.arg('movie_id') AND most_recent = 1
RETURNING id, movie_id, to_state, from_state, most_recent, sort_key, download_client_id, download_id, created_at, updated_at;

-- name: CreateMovieTransition :exec
INSERT INTO movie_transition (
    movie_id, to_state, from_state, most_recent, sort_key, download_client_id, download_id
) VALUES (?, ?, ?, ?, ?, ?, ?);
