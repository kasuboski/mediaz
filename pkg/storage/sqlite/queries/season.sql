-- TV season queries. Mirrors the legacy Jet statements that used to live in
-- season.go:
--   - CreateSeason inserted the id column only when it was non-zero. A NULL id
--     makes SQLite AUTOINCREMENT assign a fresh rowid; a non-NULL id lands in
--     the ON CONFLICT (id) DO UPDATE branch, matching the two legacy branches
--     without conditional SQL.
--   - GetSeason used a LEFT JOIN on the most recent season_transition (a
--     season without transitions is still returned) while ListSeasons used an
--     INNER JOIN; preserved exactly.
--   - Transition inserts omit created_at/updated_at so the column defaults
--     (CURRENT_TIMESTAMP) apply.
--   - Filter params are nullable (sqlc narg) so zero-value filters mean "no
--     constraint". States uses fixed nullable slots standing in for an IN
--     list; the first slot doubles as the "any state" guard. Nullable params
--     are wrapped in typed casts so sqlc emits sql.Null types instead of
--     interface; the casts are identity no-ops for the bound values. Never
--     cast to datetime types: SQLite affinity would truncate timestamps.

-- name: CreateSeason :one
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

-- name: GetSeason :one
SELECT
    s.id,
    s.series_id,
    s.season_number,
    s.season_metadata_id,
    s.monitored,
    st.to_state,
    st.download_id,
    st.download_client_id
FROM
    season AS s
    LEFT JOIN season_transition AS st ON s.id = st.season_id
    AND st.most_recent = 1
WHERE
    (
        CAST(sqlc.narg('id') AS INTEGER) IS NULL
        OR s.id = CAST(sqlc.narg('id') AS INTEGER)
    )
    AND (
        CAST(sqlc.narg('series_id') AS INTEGER) IS NULL
        OR s.series_id = CAST(sqlc.narg('series_id') AS INTEGER)
    )
    AND (
        CAST(sqlc.narg('season_number') AS INTEGER) IS NULL
        OR s.season_number = CAST(sqlc.narg('season_number') AS INTEGER)
    );

-- name: ListSeasons :many
SELECT
    s.id,
    s.series_id,
    s.season_number,
    s.season_metadata_id,
    s.monitored,
    st.to_state,
    st.download_id,
    st.download_client_id
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
        OR st.to_state IN (
            CAST(sqlc.narg('states_any') AS TEXT),
            CAST(sqlc.narg('state_2') AS TEXT),
            CAST(sqlc.narg('state_3') AS TEXT),
            CAST(sqlc.narg('state_4') AS TEXT)
        )
    );

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
