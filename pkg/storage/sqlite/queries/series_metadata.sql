-- Series/season/episode metadata queries plus the series-entity statements
-- that live in series_metadata.go (UpdateSeriesByID, LinkSeriesMetadata).
-- Upserts insert the id column only when non-zero (narg); a NULL id lets
-- SQLite AUTOINCREMENT assign a fresh rowid. UpdateSeriesMetadata stamps
-- last_info_sync with CURRENT_TIMESTAMP (UTC) in SQL - writers never
-- hand-format local timestamps (B5).

-- name: UpsertSeriesMetadata :one
INSERT INTO
    series_metadata (
        "id",
        "tmdb_id",
        "title",
        "overview",
        "last_info_sync",
        "first_air_date",
        "last_air_date",
        "season_count",
        "episode_count",
        "status",
        "poster_path",
        "external_ids",
        "watch_providers"
    )
VALUES (
    sqlc.narg('id'),
    sqlc.arg('tmdb_id'),
    sqlc.arg('title'),
    sqlc.narg('overview'),
    sqlc.narg('last_info_sync'),
    sqlc.narg('first_air_date'),
    sqlc.narg('last_air_date'),
    sqlc.arg('season_count'),
    sqlc.arg('episode_count'),
    sqlc.arg('status'),
    sqlc.narg('poster_path'),
    sqlc.narg('external_ids'),
    sqlc.narg('watch_providers')
)
ON CONFLICT ("tmdb_id") DO UPDATE SET
    "title" = excluded."title",
    "overview" = excluded."overview",
    "last_info_sync" = CURRENT_TIMESTAMP,
    "first_air_date" = excluded."first_air_date",
    "last_air_date" = excluded."last_air_date",
    "season_count" = excluded."season_count",
    "episode_count" = excluded."episode_count",
    "status" = excluded."status",
    "poster_path" = excluded."poster_path",
    "external_ids" = excluded."external_ids",
    "watch_providers" = excluded."watch_providers"
RETURNING id;

-- name: GetSeriesMetadataIDByTmdbID :one
SELECT
    id
FROM
    series_metadata
WHERE
    tmdb_id = ?;

-- name: GetSeriesMetadataByID :one
SELECT
    id,
    tmdb_id,
    title,
    overview,
    last_info_sync,
    first_air_date,
    last_air_date,
    season_count,
    episode_count,
    status,
    poster_path,
    external_ids,
    watch_providers
FROM
    series_metadata
WHERE
    id = ?;

-- name: GetSeriesMetadataByTmdbID :one
SELECT
    id,
    tmdb_id,
    title,
    overview,
    last_info_sync,
    first_air_date,
    last_air_date,
    season_count,
    episode_count,
    status,
    poster_path,
    external_ids,
    watch_providers
FROM
    series_metadata
WHERE
    tmdb_id = ?;

-- name: ListSeriesMetadata :many
SELECT
    id,
    tmdb_id,
    title,
    overview,
    last_info_sync,
    first_air_date,
    last_air_date,
    season_count,
    episode_count,
    status,
    poster_path,
    external_ids,
    watch_providers
FROM
    series_metadata;

-- name: ListSeriesMetadataByStatus :many
SELECT
    id,
    tmdb_id,
    title,
    overview,
    last_info_sync,
    first_air_date,
    last_air_date,
    season_count,
    episode_count,
    status,
    poster_path,
    external_ids,
    watch_providers
FROM
    series_metadata
WHERE
    status = ?;

-- name: UpdateSeriesMetadata :exec
UPDATE
    series_metadata
SET
    "title" = ?,
    "overview" = ?,
    "last_info_sync" = CURRENT_TIMESTAMP,
    "first_air_date" = ?,
    "last_air_date" = ?,
    "season_count" = ?,
    "episode_count" = ?,
    "status" = ?,
    "poster_path" = ?,
    "external_ids" = ?,
    "watch_providers" = ?
WHERE
    id = ?;

-- name: DeleteSeriesMetadata :exec
DELETE FROM series_metadata WHERE id = ?;

-- name: UpsertSeasonMetadata :one
INSERT INTO
    season_metadata (
        "id",
        "series_metadata_id",
        "number",
        "tmdb_id",
        "title",
        "overview",
        "air_date"
    )
VALUES (
    sqlc.narg('id'),
    sqlc.arg('series_metadata_id'),
    sqlc.arg('number'),
    sqlc.arg('tmdb_id'),
    sqlc.arg('title'),
    sqlc.narg('overview'),
    sqlc.narg('air_date')
)
ON CONFLICT ("tmdb_id") DO UPDATE SET
    "series_metadata_id" = excluded."series_metadata_id",
    "number" = excluded."number",
    "title" = excluded."title",
    "overview" = excluded."overview",
    "air_date" = excluded."air_date"
RETURNING id;

-- name: GetSeasonMetadataIDByTmdbID :one
SELECT
    id
FROM
    season_metadata
WHERE
    tmdb_id = ?;

-- name: GetSeasonMetadataByID :one
SELECT
    id,
    series_metadata_id,
    number,
    tmdb_id,
    title,
    overview,
    air_date
FROM
    season_metadata
WHERE
    id = ?;

-- name: GetSeasonMetadataByTmdbID :one
SELECT
    id,
    series_metadata_id,
    number,
    tmdb_id,
    title,
    overview,
    air_date
FROM
    season_metadata
WHERE
    tmdb_id = ?;

-- name: ListSeasonMetadataBySeriesMetadataID :many
SELECT
    id,
    series_metadata_id,
    number,
    tmdb_id,
    title,
    overview,
    air_date
FROM
    season_metadata
WHERE
    series_metadata_id = ?;

-- name: DeleteSeasonMetadata :exec
DELETE FROM season_metadata WHERE id = ?;

-- name: UpsertEpisodeMetadata :one
INSERT INTO
    episode_metadata (
        "id",
        "season_metadata_id",
        "number",
        "tmdb_id",
        "title",
        "overview",
        "air_date",
        "runtime",
        "still_path"
    )
VALUES (
    sqlc.narg('id'),
    sqlc.arg('season_metadata_id'),
    sqlc.arg('number'),
    sqlc.arg('tmdb_id'),
    sqlc.arg('title'),
    sqlc.narg('overview'),
    sqlc.narg('air_date'),
    sqlc.narg('runtime'),
    sqlc.narg('still_path')
)
ON CONFLICT ("tmdb_id") DO UPDATE SET
    "season_metadata_id" = excluded."season_metadata_id",
    "number" = excluded."number",
    "title" = excluded."title",
    "overview" = excluded."overview",
    "air_date" = excluded."air_date",
    "runtime" = excluded."runtime",
    "still_path" = excluded."still_path"
RETURNING id;

-- name: GetEpisodeMetadataIDByTmdbID :one
SELECT
    id
FROM
    episode_metadata
WHERE
    tmdb_id = ?;

-- name: GetEpisodeMetadataByID :one
SELECT
    id,
    season_metadata_id,
    number,
    tmdb_id,
    title,
    overview,
    air_date,
    runtime,
    still_path
FROM
    episode_metadata
WHERE
    id = ?;

-- name: GetEpisodeMetadataByTmdbID :one
SELECT
    id,
    season_metadata_id,
    number,
    tmdb_id,
    title,
    overview,
    air_date,
    runtime,
    still_path
FROM
    episode_metadata
WHERE
    tmdb_id = ?;

-- name: GetEpisodeMetadataBySeasonAndNumber :one
SELECT
    id,
    season_metadata_id,
    number,
    tmdb_id,
    title,
    overview,
    air_date,
    runtime,
    still_path
FROM
    episode_metadata
WHERE
    season_metadata_id = ?
    AND number = ?;

-- name: ListEpisodeMetadataBySeasonMetadataID :many
SELECT
    id,
    season_metadata_id,
    number,
    tmdb_id,
    title,
    overview,
    air_date,
    runtime,
    still_path
FROM
    episode_metadata
WHERE
    season_metadata_id = ?;

-- name: ListEpisodeMetadataByIDs :many
SELECT
    id,
    season_metadata_id,
    number,
    tmdb_id,
    title,
    overview,
    air_date,
    runtime,
    still_path
FROM
    episode_metadata
WHERE
    id IN (sqlc.slice('ids'));

-- name: DeleteEpisodeMetadata :exec
DELETE FROM episode_metadata WHERE id = ?;

-- name: DemoteSeriesTransition :one
UPDATE
    series_transition
SET
    "most_recent" = 0
WHERE
    series_id = ?
    AND most_recent = 1
RETURNING sort_key;

-- name: LinkSeriesMetadata :exec
UPDATE
    series
SET
    "series_metadata_id" = ?
WHERE
    id = ?;

-- name: UpdateSeriesByID :exec
-- Legacy parity: only the monitored column is ever updated through this path.
UPDATE
    series
SET
    "monitored" = ?
WHERE
    id = ?;
