-- TV episode queries. Mirrors the legacy Jet statements that used to live in
-- episode.go:
--   - CreateEpisode inserted the id column only when it was non-zero. A NULL
--     id makes SQLite AUTOINCREMENT assign a fresh rowid; a non-NULL id lands
--     in the ON CONFLICT (id) DO UPDATE branch, matching the two legacy
--     branches without conditional SQL.
--   - GetEpisode used a LEFT JOIN on the most recent episode_transition (an
--     episode without transitions is still returned) while ListEpisodes used
--     an INNER JOIN; preserved exactly.
--   - GetEpisodeByEpisodeFileID intentionally selects only to_state,
--     download_id and download_client_id from the transition, matching the
--     legacy column list (is_entire_season_download stays false on this path).
--   - Transition inserts omit created_at/updated_at so the column defaults
--     (CURRENT_TIMESTAMP) apply.
--   - Filter params are nullable (sqlc narg) so zero-value filters mean "no
--     constraint". States uses fixed nullable slots standing in for an IN
--     list; the first slot doubles as the "any state" guard. Nullable params
--     are wrapped in typed casts so sqlc emits sql.Null types instead of
--     interface; the casts are identity no-ops for the bound values. Never
--     cast to datetime types: SQLite affinity would truncate timestamps.

-- name: CreateEpisode :one
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

-- name: GetEpisode :one
SELECT
    e.id,
    e.season_id,
    e.episode_number,
    e.monitored,
    e.episode_metadata_id,
    e.episode_file_id,
    et.to_state,
    et.download_id,
    et.download_client_id,
    et.is_entire_season_download
FROM
    episode AS e
    LEFT JOIN episode_transition AS et ON e.id = et.episode_id
    AND et.most_recent = 1
WHERE
    (
        CAST(sqlc.narg('id') AS INTEGER) IS NULL
        OR e.id = CAST(sqlc.narg('id') AS INTEGER)
    )
    AND (
        CAST(sqlc.narg('season_id') AS INTEGER) IS NULL
        OR e.season_id = CAST(sqlc.narg('season_id') AS INTEGER)
    );

-- name: ListEpisodes :many
SELECT
    e.id,
    e.season_id,
    e.episode_number,
    e.monitored,
    e.episode_metadata_id,
    e.episode_file_id,
    et.to_state,
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
        OR et.to_state IN (
            CAST(sqlc.narg('states_any') AS TEXT),
            CAST(sqlc.narg('state_2') AS TEXT),
            CAST(sqlc.narg('state_3') AS TEXT),
            CAST(sqlc.narg('state_4') AS TEXT)
        )
    );

-- name: GetEpisodeByEpisodeFileID :one
SELECT
    e.id,
    e.season_id,
    e.episode_number,
    e.monitored,
    e.episode_metadata_id,
    e.episode_file_id,
    et.to_state,
    et.download_id,
    et.download_client_id
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
