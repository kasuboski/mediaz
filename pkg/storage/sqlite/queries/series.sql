-- TV series queries. Entity reads join the most recent series_transition via
-- embed(s) plus the named state column. ListSeries supports any-length
-- state filters through slice guarded by a nullable param (A3). Inserts
-- are plain INSERTs: the legacy ON CONFLICT (id) clause could never fire
-- because id was never inserted.

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
RETURNING "id";

-- name: CreateSeriesWithAdded :one
-- Used when the caller supplies an explicit "added" timestamp; otherwise
-- CreateSeries lets the column default (CURRENT_TIMESTAMP, UTC) apply.
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
RETURNING "id";

-- name: GetSeries :one
-- Flexible lookup: each nullable param that is set constrains the match.
SELECT
    sqlc.embed(s),
    st.to_state AS state
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
-- states_any is a NULL guard: when NULL no state constraint applies; when set,
-- the states slice constrains the current transition state (any length).
SELECT
    sqlc.embed(s),
    st.to_state AS state
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
        OR st.to_state IN (sqlc.slice('states'))
    )
ORDER BY
    sm.title ASC, s.id ASC;

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
