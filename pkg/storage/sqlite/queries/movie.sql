-- Movie queries. Entity reads join the most recent movie_transition and use
-- embed so the generated *Row types carry the movie entity plus named
-- transition columns (state, download metadata) - no manual join-row structs.
-- Transition writers let SQLite assign created_at/updated_at (CURRENT_TIMESTAMP,
-- UTC); the demote statement stamps updated_at with CURRENT_TIMESTAMP so no
-- application code ever hand-formats timestamps (B5).

-- name: CreateMovie :one
INSERT INTO movie (
    path, monitored, quality_profile_id, tags, add_options, movie_file_id,
    minimum_availability, movie_metadata_id, last_search_time
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: CreateMovieWithAdded :one
-- Used when the caller supplies an explicit "added" timestamp; otherwise
-- CreateMovie lets the column default (CURRENT_TIMESTAMP, UTC) apply.
INSERT INTO movie (
    added, path, monitored, quality_profile_id, tags, add_options, movie_file_id,
    minimum_availability, movie_metadata_id, last_search_time
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: GetMovieByID :one
SELECT
    sqlc.embed(m),
    mt.to_state AS state,
    mt.download_id,
    mt.download_client_id
FROM movie AS m
INNER JOIN movie_transition AS mt ON m.id = mt.movie_id AND mt.most_recent = 1
WHERE m.id = ?;

-- name: GetMovieByFileID :one
SELECT
    sqlc.embed(m),
    mt.to_state AS state,
    mt.download_id,
    mt.download_client_id
FROM movie AS m
INNER JOIN movie_transition AS mt ON m.id = mt.movie_id AND mt.most_recent = 1
WHERE m.movie_file_id = ?;

-- name: ListMovies :many
SELECT
    sqlc.embed(m),
    mt.to_state AS state,
    mt.download_id,
    mt.download_client_id
FROM movie AS m
INNER JOIN movie_transition AS mt ON m.id = mt.movie_id AND mt.most_recent = 1
LEFT JOIN movie_metadata AS mm ON m.movie_metadata_id = mm.id
ORDER BY mm.title ASC, m.id ASC;

-- name: ListMoviesByStates :many
-- states_any is a NULL guard: when NULL no state constraint applies; when set,
-- the states slice constrains the current transition state (any length).
SELECT
    sqlc.embed(m),
    mt.to_state AS state,
    mt.download_id,
    mt.download_client_id
FROM movie AS m
INNER JOIN movie_transition AS mt ON m.id = mt.movie_id AND mt.most_recent = 1
WHERE (
    CAST(sqlc.narg('states_any') AS TEXT) IS NULL
    OR mt.to_state IN (sqlc.slice('states'))
)
ORDER BY m.added ASC, m.id ASC;

-- name: GetMovieByMetadataID :one
SELECT
    sqlc.embed(m),
    mt.to_state AS state,
    mt.download_id,
    mt.download_client_id
FROM movie AS m
INNER JOIN movie_transition AS mt ON m.id = mt.movie_id AND mt.most_recent = 1
WHERE m.movie_metadata_id = ?;

-- name: GetMovieByPath :one
SELECT
    sqlc.embed(m),
    mt.to_state AS state,
    mt.download_id,
    mt.download_client_id
FROM movie AS m
INNER JOIN movie_transition AS mt ON m.id = mt.movie_id AND mt.most_recent = 1
WHERE m.path = ?;

-- name: DeleteMovie :exec
DELETE FROM movie WHERE id = ?;

-- name: UpdateMovieMovieFileID :exec
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
    updated_at = CURRENT_TIMESTAMP
WHERE movie_id = sqlc.arg('movie_id') AND most_recent = 1
RETURNING sort_key;

-- name: CreateMovieTransition :exec
INSERT INTO movie_transition (
    movie_id, to_state, from_state, most_recent, sort_key, download_client_id, download_id
) VALUES (?, ?, ?, ?, ?, ?, ?);
