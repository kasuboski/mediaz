-- name: CreateMovieMetadata :one
-- Plain insert: the legacy ON CONFLICT (id) DO NOTHING clause could never
-- fire because id was never inserted (autoincrement assigns it).
INSERT INTO movie_metadata (
    tmdb_id, imdb_id, images, genres, title, sort_title, clean_title,
    original_title, clean_original_title, original_language, status,
    last_info_sync, runtime, in_cinemas, release_date, year, secondary_year,
    ratings, recommendations, certification, youtube_trailer_id, studio,
    overview, website, popularity, collection_tmdb_id, collection_title
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: UpdateMovieMetadataQuery :exec
UPDATE movie_metadata SET
    last_info_sync = CURRENT_TIMESTAMP,
    imdb_id = ?,
    images = ?,
    genres = ?,
    title = ?,
    sort_title = ?,
    clean_title = ?,
    original_title = ?,
    clean_original_title = ?,
    original_language = ?,
    status = ?,
    runtime = ?,
    in_cinemas = ?,
    release_date = ?,
    year = ?,
    secondary_year = ?,
    ratings = ?,
    recommendations = ?,
    certification = ?,
    youtube_trailer_id = ?,
    studio = ?,
    overview = ?,
    website = ?,
    popularity = ?,
    collection_tmdb_id = ?,
    collection_title = ?
WHERE id = ?;

-- name: DeleteMovieMetadata :exec
DELETE FROM movie_metadata WHERE id = ?;

-- name: ListMovieMetadata :many
SELECT
    id,
    tmdb_id,
    imdb_id,
    images,
    genres,
    title,
    sort_title,
    clean_title,
    original_title,
    clean_original_title,
    original_language,
    status,
    last_info_sync,
    runtime,
    in_cinemas,
    release_date,
    year,
    secondary_year,
    ratings,
    recommendations,
    certification,
    youtube_trailer_id,
    studio,
    overview,
    website,
    popularity,
    collection_tmdb_id,
    collection_title
FROM movie_metadata
ORDER BY last_info_sync ASC, id ASC;

-- name: GetMovieMetadataByID :one
SELECT
    id,
    tmdb_id,
    imdb_id,
    images,
    genres,
    title,
    sort_title,
    clean_title,
    original_title,
    clean_original_title,
    original_language,
    status,
    last_info_sync,
    runtime,
    in_cinemas,
    release_date,
    year,
    secondary_year,
    ratings,
    recommendations,
    certification,
    youtube_trailer_id,
    studio,
    overview,
    website,
    popularity,
    collection_tmdb_id,
    collection_title
FROM movie_metadata
WHERE id = ?;

-- name: GetMovieMetadataByTmdbID :one
SELECT
    id,
    tmdb_id,
    imdb_id,
    images,
    genres,
    title,
    sort_title,
    clean_title,
    original_title,
    clean_original_title,
    original_language,
    status,
    last_info_sync,
    runtime,
    in_cinemas,
    release_date,
    year,
    secondary_year,
    ratings,
    recommendations,
    certification,
    youtube_trailer_id,
    studio,
    overview,
    website,
    popularity,
    collection_tmdb_id,
    collection_title
FROM movie_metadata
WHERE tmdb_id = ?;
