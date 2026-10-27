-- TV season queries. Entity reads join the most recent season_transition via
-- embed(s) plus named transition columns. Get-by-id keeps the legacy LEFT
-- JOIN (a season without transitions is still returned) while ListSeasons uses
-- an INNER JOIN. ListSeasons supports any-length state filters through
-- slice guarded by a nullable param (A3).

-- name: CreateSeason :one
-- A NULL id lets SQLite assign a fresh AUTOINCREMENT rowid; a non-NULL id
-- inserts explicitly or lands in the ON CONFLICT branch (upsert by id).
INSERT INTO
    season (
        "id",
        "series_id",
        "season_number",
        "season_metadata_id",
        "monitored"
    )
VALUES (
    CAST(sqlc.narg('id') AS INTEGER),
    sqlc.arg('series_id'),
    sqlc.arg('season_number'),
    sqlc.narg('season_metadata_id'),
    sqlc.arg('monitored')
)
ON CONFLICT ("id") DO UPDATE SET
    "series_id" = excluded."series_id",
    "season_number" = excluded."season_number",
    "season_metadata_id" = excluded."season_metadata_id",
    "monitored" = excluded."monitored"
RETURNING "id";

-- name: GetSeasonByID :one
SELECT
    sqlc.embed(s),
    st.to_state AS state,
    st.download_id,
    st.download_client_id,
    st.is_entire_season_download
FROM
    season AS s
    LEFT JOIN season_transition AS st ON s.id = st.season_id
    AND st.most_recent = 1
WHERE s.id = sqlc.arg('id');

-- name: GetSeasonBySeriesIDAndNumber :one
SELECT
    sqlc.embed(s),
    st.to_state AS state,
    st.download_id,
    st.download_client_id,
    st.is_entire_season_download
FROM
    season AS s
    LEFT JOIN season_transition AS st ON s.id = st.season_id
    AND st.most_recent = 1
WHERE
    s.series_id = sqlc.arg('series_id')
    AND s.season_number = sqlc.arg('season_number');

-- name: ListSeasons :many
-- states_any is a NULL guard: when NULL no state constraint applies; when set,
-- the states slice constrains the current transition state (any length).
SELECT
    sqlc.embed(s),
    st.to_state AS state,
    st.download_id,
    st.download_client_id,
    st.is_entire_season_download
FROM
    season AS s
    INNER JOIN season_transition AS st ON s.id = st.season_id
    AND st.most_recent = 1
WHERE
    (
        CAST(sqlc.narg('series_id') AS INTEGER) IS NULL
        OR s.series_id = CAST(sqlc.narg('series_id') AS INTEGER)
    )
    AND (
        CAST(sqlc.narg('states_any') AS TEXT) IS NULL
        OR st.to_state IN (sqlc.slice('states'))
    )
ORDER BY s.series_id ASC, s.season_number ASC, s.id ASC;

-- name: DeleteSeason :exec
DELETE FROM season
WHERE id = ?;

-- name: LinkSeasonMetadata :exec
UPDATE
    season
SET
    "season_metadata_id" = ?
WHERE
    id = ?;

-- name: DemoteSeasonTransition :one
UPDATE
    season_transition
SET
    "most_recent" = 0
WHERE
    season_id = ?
    AND most_recent = 1
RETURNING sort_key;

-- name: InsertSeasonTransition :exec
INSERT INTO
    season_transition (
        "season_id",
        "to_state",
        "from_state",
        "most_recent",
        "sort_key",
        "download_client_id",
        "download_id",
        "is_entire_season_download"
    )
VALUES (?, ?, ?, ?, ?, ?, ?, ?);
