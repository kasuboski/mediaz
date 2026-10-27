-- TV episode queries. Entity reads join the most recent episode_transition via
-- embed(e) plus named transition columns. Get-by-id keeps the legacy LEFT
-- JOIN (an episode without transitions is still returned) while ListEpisodes
-- uses an INNER JOIN. ListEpisodes supports any-length state filters through
-- slice guarded by a nullable param (A3). Unlike the legacy statement,
-- GetEpisodeByEpisodeFileID now also selects is_entire_season_download so all
-- episode reads share one column shape.

-- name: CreateEpisode :one
-- A NULL id lets SQLite assign a fresh AUTOINCREMENT rowid; a non-NULL id
-- inserts explicitly or lands in the ON CONFLICT branch (upsert by id).
INSERT INTO
    episode (
        "id",
        "season_id",
        "episode_number",
        "monitored",
        "episode_metadata_id",
        "episode_file_id"
    )
VALUES (
    CAST(sqlc.narg('id') AS INTEGER),
    sqlc.arg('season_id'),
    sqlc.arg('episode_number'),
    sqlc.arg('monitored'),
    sqlc.narg('episode_metadata_id'),
    sqlc.narg('episode_file_id')
)
ON CONFLICT ("id") DO UPDATE SET
    "season_id" = excluded."season_id",
    "episode_number" = excluded."episode_number",
    "monitored" = excluded."monitored",
    "episode_metadata_id" = excluded."episode_metadata_id",
    "episode_file_id" = excluded."episode_file_id"
RETURNING "id";

-- name: GetEpisodeByID :one
SELECT
    sqlc.embed(e),
    et.to_state AS state,
    et.download_id,
    et.download_client_id,
    et.is_entire_season_download
FROM
    episode AS e
    LEFT JOIN episode_transition AS et ON e.id = et.episode_id
    AND et.most_recent = 1
WHERE e.id = sqlc.arg('id');

-- name: ListEpisodes :many
-- states_any is a NULL guard: when NULL no state constraint applies; when set,
-- the states slice constrains the current transition state (any length).
SELECT
    sqlc.embed(e),
    et.to_state AS state,
    et.download_id,
    et.download_client_id,
    et.is_entire_season_download
FROM
    episode AS e
    INNER JOIN episode_transition AS et ON e.id = et.episode_id
    AND et.most_recent = 1
WHERE
    (
        CAST(sqlc.narg('season_id') AS INTEGER) IS NULL
        OR e.season_id = CAST(sqlc.narg('season_id') AS INTEGER)
    )
    AND (
        CAST(sqlc.narg('states_any') AS TEXT) IS NULL
        OR et.to_state IN (sqlc.slice('states'))
    )
ORDER BY e.season_id ASC, e.episode_number ASC, e.id ASC;

-- name: GetEpisodeByEpisodeFileID :one
SELECT
    sqlc.embed(e),
    et.to_state AS state,
    et.download_id,
    et.download_client_id,
    et.is_entire_season_download
FROM
    episode AS e
    LEFT JOIN episode_transition AS et ON e.id = et.episode_id
    AND et.most_recent = 1
WHERE
    e.episode_file_id = ?;

-- name: DeleteEpisode :exec
DELETE FROM episode
WHERE id = ?;

-- name: UpdateEpisodeEpisodeFileID :exec
UPDATE
    episode
SET
    "episode_file_id" = ?
WHERE
    id = ?;

-- name: LinkEpisodeMetadata :exec
UPDATE
    episode
SET
    "season_id" = ?,
    "episode_metadata_id" = ?
WHERE
    id = ?;

-- name: DemoteEpisodeTransition :one
UPDATE
    episode_transition
SET
    "most_recent" = 0
WHERE
    episode_id = ?
    AND most_recent = 1
RETURNING sort_key;

-- name: InsertEpisodeTransition :exec
INSERT INTO
    episode_transition (
        "episode_id",
        "to_state",
        "from_state",
        "most_recent",
        "sort_key",
        "download_client_id",
        "download_id",
        "is_entire_season_download"
    )
VALUES (?, ?, ?, ?, ?, ?, ?, ?);
