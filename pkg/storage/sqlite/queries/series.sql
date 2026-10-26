-- TV series queries. Mirrors the legacy Jet statements that used to live in
-- series.go:
--   - CreateSeries never inserts the id column, so the ON CONFLICT (id)
--     target is kept only for parity; it cannot fire.
--   - Legacy omitted "added" from the insert column list when the caller left
--     it nil/zero, letting the column default (CURRENT_TIMESTAMP) apply. That
--     conditional column list becomes two queries: CreateSeries (default
--     added) and CreateSeriesWithAdded (explicit added).
--   - Reads use an INNER JOIN on the most recent series_transition, matching
--     the legacy GetSeries/ListSeries joins. ListSeries LEFT JOINs
--     series_metadata purely for title ordering, like the legacy statement.
--   - Filter params are nullable (sqlc narg) so zero-value filters mean "no
--     constraint". States uses fixed nullable slots standing in for an IN
--     list; the first slot doubles as the "any state" guard. Nullable params
--     are wrapped in typed casts so sqlc emits sql.Null types instead of
--     interface; the casts are identity no-ops for the bound values. Never
--     cast to datetime types: SQLite affinity would truncate timestamps.

-- name: CreateSeries :one
INSERT INTO
    series (
        "path",
        "monitored",
        "quality_profile_id",
        "series_metadata_id",
        "last_search_time",
        "monitor_new_seasons"
    )
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT ("id") DO UPDATE SET
    "path" = excluded."path",
    "monitored" = excluded."monitored",
    "quality_profile_id" = excluded."quality_profile_id",
    "series_metadata_id" = excluded."series_metadata_id",
    "last_search_time" = excluded."last_search_time",
    "monitor_new_seasons" = excluded."monitor_new_seasons"
RETURNING "id";

-- name: CreateSeriesWithAdded :one
INSERT INTO
    series (
        "path",
        "monitored",
        "added",
        "quality_profile_id",
        "series_metadata_id",
        "last_search_time",
        "monitor_new_seasons"
    )
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT ("id") DO UPDATE SET
    "path" = excluded."path",
    "monitored" = excluded."monitored",
    "added" = excluded."added",
    "quality_profile_id" = excluded."quality_profile_id",
    "series_metadata_id" = excluded."series_metadata_id",
    "last_search_time" = excluded."last_search_time",
    "monitor_new_seasons" = excluded."monitor_new_seasons"
RETURNING "id";

-- name: GetSeries :one
SELECT
    s.id,
    s.path,
    s.monitored,
    s.added,
    s.quality_profile_id,
    s.series_metadata_id,
    s.last_search_time,
    s.monitor_new_seasons,
    st.to_state
FROM
    series AS s
    INNER JOIN series_transition AS st ON s.id = st.series_id
    AND st.most_recent = 1
WHERE
    (
        CAST(sqlc.narg('id') AS INTEGER) IS NULL
        OR s.id = CAST(sqlc.narg('id') AS INTEGER)
    )
    AND (
        CAST(sqlc.narg('series_metadata_id') AS INTEGER) IS NULL
        OR s.series_metadata_id = CAST(sqlc.narg('series_metadata_id') AS INTEGER)
    )
    AND (
        CAST(sqlc.narg('path') AS TEXT) IS NULL
        OR s.path = CAST(sqlc.narg('path') AS TEXT)
    );

-- name: ListSeries :many
SELECT
    s.id,
    s.path,
    s.monitored,
    s.added,
    s.quality_profile_id,
    s.series_metadata_id,
    s.last_search_time,
    s.monitor_new_seasons,
    st.to_state
FROM
    series AS s
    INNER JOIN series_transition AS st ON s.id = st.series_id
    AND st.most_recent = 1
    LEFT JOIN series_metadata AS sm ON s.series_metadata_id = sm.id
WHERE
    (
        CAST(sqlc.narg('monitored') AS INTEGER) IS NULL
        OR s.monitored = CAST(sqlc.narg('monitored') AS INTEGER)
    )
    AND (
        CAST(sqlc.narg('states_any') AS TEXT) IS NULL
        OR st.to_state IN (
            CAST(sqlc.narg('states_any') AS TEXT),
            CAST(sqlc.narg('state_2') AS TEXT),
            CAST(sqlc.narg('state_3') AS TEXT),
            CAST(sqlc.narg('state_4') AS TEXT)
        )
    )
ORDER BY
    sm.title ASC;

-- name: DeleteSeries :exec
DELETE FROM series
WHERE id = ?;

-- name: InsertSeriesTransition :exec
INSERT INTO
    series_transition (
        "series_id",
        "to_state",
        "from_state",
        "most_recent",
        "sort_key"
    )
VALUES (?, ?, ?, ?, ?);
