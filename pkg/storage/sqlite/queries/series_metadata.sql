-- Series/season/episode metadata queries plus the series-entity statements that
-- live in series_metadata.go (UpdateSeriesState, UpdateSeriesByID,
-- LinkSeriesMetadata). Mirrors the legacy Jet statements:
--   - Upserts insert the id column only when it is non-zero (sqlc.narg); a NULL
--     id lets SQLite AUTOINCREMENT assign a fresh rowid, matching the two
--     legacy insert branches.
--   - Inserted nullable columns pass through as explicit NULLs, so the
--     last_info_sync CURRENT_TIMESTAMP default behaves exactly like the legacy
--     MODEL inserts.
--   - Conflict branches match legacy behavior exactly: series metadata updates
--     only title/last_info_sync/season_count/episode_count/status (last_info_sync
--     to CURRENT_TIMESTAMP), season metadata does nothing, and episode metadata
--     updates only the title.
--   - Reads return every column with no ordering, like the legacy SELECTs.

-- name: UpsertSeriesMetadata :exec
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
    "last_info_sync" = CURRENT_TIMESTAMP,
    "season_count" = excluded."season_count",
    "episode_count" = excluded."episode_count",
    "status" = excluded."status";

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
    "last_info_sync" = ?,
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

-- name: UpsertSeasonMetadata :exec
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
ON CONFLICT ("tmdb_id") DO NOTHING;

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

-- name: UpsertEpisodeMetadata :exec
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
    "title" = excluded."title";

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

-- DemoteSeriesTransition backs UpdateSeriesState, which lives in
-- series_metadata.go. It mirrors the season/episode transition statements in
-- season.sql/episode.sql: the previous most-recent transition is demoted
-- (RETURNING its sort key). The new transition is inserted with
-- InsertSeriesTransition from series.sql, omitting created_at/updated_at so
-- the column defaults (CURRENT_TIMESTAMP) apply.

-- name: DemoteSeriesTransition :one
UPDATE
    series_transition
SET
    "most_recent" = 0
WHERE
    series_id = ?
    AND most_recent = 1
RETURNING sort_key;

-- LinkSeriesMetadata / UpdateSeriesByID back LinkSeriesMetadata and
-- UpdateSeriesByID in series_metadata.go. Like the legacy statements they only
-- ever touch the given columns: the metadata link sets series_metadata_id, and
-- the series update sets only "monitored".

-- name: LinkSeriesMetadata :exec
UPDATE
    series
SET
    "series_metadata_id" = ?
WHERE
    id = ?;

-- name: UpdateSeriesByID :exec
UPDATE
    series
SET
    "monitored" = ?
WHERE
    id = ?;
