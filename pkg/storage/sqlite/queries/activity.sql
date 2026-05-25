-- name: ListDownloadingMovies :many
SELECT
    m.id,
    mm.tmdb_id,
    mm.title,
    mm.year,
    mm.images,
    mt.to_state,
    mt.sort_key,
    mt.download_id,
    dc.id AS dc_id,
    dc.host AS dc_host,
    dc.port AS dc_port
FROM movie m
INNER JOIN movie_transition mt ON m.id = mt.movie_id AND mt.most_recent = 1
INNER JOIN movie_metadata mm ON m.movie_metadata_id = mm.id
LEFT JOIN download_client dc ON mt.download_client_id = dc.id
WHERE mt.to_state = 'downloading'
ORDER BY mt.sort_key DESC;

-- name: ListDownloadingSeries :many
SELECT
    se.id,
    sm.tmdb_id,
    sm.title,
    sm.poster_path,
    st.to_state,
    st.sort_key,
    st.download_id,
    se.season_number,
    dc.id AS dc_id,
    dc.host AS dc_host,
    dc.port AS dc_port
FROM season se
INNER JOIN season_transition st ON se.id = st.season_id AND st.most_recent = 1
INNER JOIN series ser ON se.series_id = ser.id
LEFT JOIN series_metadata sm ON ser.series_metadata_id = sm.id
LEFT JOIN download_client dc ON st.download_client_id = dc.id
WHERE st.to_state = 'downloading'
ORDER BY st.sort_key DESC;

-- name: ListRunningJobs :many
SELECT
    j.id,
    j.type,
    jt.to_state,
    j.created_at,
    jt.updated_at
FROM job j
INNER JOIN job_transition jt ON j.id = jt.job_id AND jt.most_recent = 1
WHERE jt.to_state = 'running'
ORDER BY j.created_at DESC;

-- name: ListErrorJobs :many
SELECT
    j.id,
    j.type,
    jt.to_state,
    j.created_at,
    jt.updated_at
FROM job j
INNER JOIN job_transition jt ON j.id = jt.job_id AND jt.most_recent = 1
WHERE jt.to_state = 'error'
  AND jt.updated_at >= sqlc.arg(cutoff_datetime)
ORDER BY jt.updated_at DESC;

-- name: CountTransitionsByDate :one
SELECT (
    SELECT COUNT(*) FROM movie_transition mt
    INNER JOIN movie m ON m.id = mt.movie_id
    WHERE mt.created_at >= datetime(?) AND mt.created_at <= datetime(?)
      AND mt.most_recent = 1
) + (
    SELECT COUNT(*) FROM season_transition st
    INNER JOIN season s ON s.id = st.season_id
    WHERE st.created_at >= datetime(?) AND st.created_at <= datetime(?)
      AND st.most_recent = 1
) + (
    SELECT COUNT(*) FROM episode_transition et
    INNER JOIN episode e ON e.id = et.episode_id
    WHERE et.created_at >= datetime(?) AND et.created_at <= datetime(?)
      AND et.most_recent = 1
) + (
    SELECT COUNT(*) FROM job_transition jt
    INNER JOIN job j ON j.id = jt.job_id
    WHERE jt.created_at >= datetime(?) AND jt.created_at <= datetime(?)
      AND jt.most_recent = 1
) AS total;

-- name: GetMovieTransitionsByDate :many
SELECT
    CAST(STRFTIME('%Y-%m-%d', mt.created_at) AS TEXT) AS date,
    COUNT(DISTINCT CASE WHEN mt.to_state = 'downloaded' THEN m.id END) AS downloaded,
    COUNT(DISTINCT CASE WHEN mt.to_state = 'downloading' THEN m.id END) AS downloading
FROM movie m
INNER JOIN movie_transition mt ON m.id = mt.movie_id
WHERE mt.created_at >= datetime(?)
  AND mt.created_at <= datetime(?)
  AND mt.most_recent = 1
GROUP BY STRFTIME('%Y-%m-%d', mt.created_at)
ORDER BY date;

-- name: GetSeriesTransitionsByDate :many
SELECT
    CAST(STRFTIME('%Y-%m-%d', st.created_at) AS TEXT) AS date,
    COUNT(DISTINCT CASE WHEN st.to_state = 'completed' THEN ser.id END) AS completed,
    COUNT(DISTINCT CASE WHEN st.to_state = 'downloading' THEN ser.id END) AS downloading
FROM season s
INNER JOIN season_transition st ON s.id = st.season_id
INNER JOIN series ser ON s.series_id = ser.id
WHERE st.created_at >= datetime(?)
  AND st.created_at <= datetime(?)
  AND st.most_recent = 1
GROUP BY STRFTIME('%Y-%m-%d', st.created_at)
ORDER BY date;

-- name: GetJobTransitionsByDate :many
SELECT
    CAST(STRFTIME('%Y-%m-%d', jt.created_at) AS TEXT) AS date,
    COUNT(DISTINCT CASE WHEN jt.to_state = 'done' THEN j.id END) AS done,
    COUNT(DISTINCT CASE WHEN jt.to_state = 'error' THEN j.id END) AS error
FROM job j
INNER JOIN job_transition jt ON j.id = jt.job_id
WHERE jt.created_at >= datetime(?)
  AND jt.created_at <= datetime(?)
  AND jt.most_recent = 1
GROUP BY STRFTIME('%Y-%m-%d', jt.created_at)
ORDER BY date;

-- name: GetMovieTransitionItems :many
SELECT
    mt.id,
    'movie' AS entity_type,
    m.id AS entity_id,
    COALESCE(mm.title, '') AS entity_title,
    mt.to_state,
    mt.from_state,
    mt.created_at
FROM movie m
INNER JOIN movie_transition mt ON m.id = mt.movie_id
LEFT JOIN movie_metadata mm ON m.movie_metadata_id = mm.id
WHERE mt.created_at >= datetime(?)
  AND mt.created_at <= datetime(?)
  AND mt.most_recent = 1
ORDER BY mt.created_at DESC
LIMIT ? OFFSET ?;

-- name: GetMovieTransitionItemsNoLimit :many
SELECT
    mt.id,
    'movie' AS entity_type,
    m.id AS entity_id,
    COALESCE(mm.title, '') AS entity_title,
    mt.to_state,
    mt.from_state,
    mt.created_at
FROM movie m
INNER JOIN movie_transition mt ON m.id = mt.movie_id
LEFT JOIN movie_metadata mm ON m.movie_metadata_id = mm.id
WHERE mt.created_at >= datetime(?)
  AND mt.created_at <= datetime(?)
  AND mt.most_recent = 1
ORDER BY mt.created_at DESC;

-- name: GetSeasonTransitionItems :many
SELECT
    st.id,
    'season' AS entity_type,
    s.id AS entity_id,
    COALESCE(sm.title, '') AS entity_title,
    st.to_state,
    st.from_state,
    st.created_at
FROM season s
INNER JOIN season_transition st ON s.id = st.season_id
INNER JOIN series ser ON s.series_id = ser.id
LEFT JOIN series_metadata sm ON ser.series_metadata_id = sm.id
WHERE st.created_at >= datetime(?)
  AND st.created_at <= datetime(?)
  AND st.most_recent = 1
ORDER BY st.created_at DESC
LIMIT ? OFFSET ?;

-- name: GetSeasonTransitionItemsNoLimit :many
SELECT
    st.id,
    'season' AS entity_type,
    s.id AS entity_id,
    COALESCE(sm.title, '') AS entity_title,
    st.to_state,
    st.from_state,
    st.created_at
FROM season s
INNER JOIN season_transition st ON s.id = st.season_id
INNER JOIN series ser ON s.series_id = ser.id
LEFT JOIN series_metadata sm ON ser.series_metadata_id = sm.id
WHERE st.created_at >= datetime(?)
  AND st.created_at <= datetime(?)
  AND st.most_recent = 1
ORDER BY st.created_at DESC;

-- name: GetEpisodeTransitionItems :many
SELECT
    et.id,
    'episode' AS entity_type,
    e.id AS entity_id,
    COALESCE(sm.title, '') AS entity_title,
    et.to_state,
    et.from_state,
    et.created_at
FROM episode e
INNER JOIN episode_transition et ON e.id = et.episode_id
INNER JOIN season s ON e.season_id = s.id
INNER JOIN series ser ON s.series_id = ser.id
LEFT JOIN series_metadata sm ON ser.series_metadata_id = sm.id
WHERE et.created_at >= datetime(?)
  AND et.created_at <= datetime(?)
  AND et.most_recent = 1
ORDER BY et.created_at DESC
LIMIT ? OFFSET ?;

-- name: GetEpisodeTransitionItemsNoLimit :many
SELECT
    et.id,
    'episode' AS entity_type,
    e.id AS entity_id,
    COALESCE(sm.title, '') AS entity_title,
    et.to_state,
    et.from_state,
    et.created_at
FROM episode e
INNER JOIN episode_transition et ON e.id = et.episode_id
INNER JOIN season s ON e.season_id = s.id
INNER JOIN series ser ON s.series_id = ser.id
LEFT JOIN series_metadata sm ON ser.series_metadata_id = sm.id
WHERE et.created_at >= datetime(?)
  AND et.created_at <= datetime(?)
  AND et.most_recent = 1
ORDER BY et.created_at DESC;

-- name: GetJobTransitionItems :many
SELECT
    jt.id,
    'job' AS entity_type,
    j.id AS entity_id,
    j.type AS entity_title,
    jt.to_state,
    jt.from_state,
    jt.created_at
FROM job j
INNER JOIN job_transition jt ON j.id = jt.job_id
WHERE jt.created_at >= datetime(?)
  AND jt.created_at <= datetime(?)
  AND jt.most_recent = 1
ORDER BY jt.created_at DESC
LIMIT ? OFFSET ?;

-- name: GetJobTransitionItemsNoLimit :many
SELECT
    jt.id,
    'job' AS entity_type,
    j.id AS entity_id,
    j.type AS entity_title,
    jt.to_state,
    jt.from_state,
    jt.created_at
FROM job j
INNER JOIN job_transition jt ON j.id = jt.job_id
WHERE jt.created_at >= datetime(?)
  AND jt.created_at <= datetime(?)
  AND jt.most_recent = 1
ORDER BY jt.created_at DESC;

-- name: GetEntityTransitionsMovie :many
SELECT
    mm.title AS entity_title,
    mm.images AS poster_path,
    mt.to_state,
    mt.from_state,
    mt.created_at,
    mt.sort_key,
    CAST(json_object(
        'downloadClient', json_object('id', dc.id, 'host', dc.host, 'port', dc.port),
        'downloadID', mt.download_id
    ) AS TEXT) AS metadata
FROM movie m
INNER JOIN movie_transition mt ON m.id = mt.movie_id
LEFT JOIN movie_metadata mm ON m.movie_metadata_id = mm.id
LEFT JOIN download_client dc ON mt.download_client_id = dc.id
WHERE m.id = ?
ORDER BY mt.sort_key ASC;

-- name: GetEntityTransitionsSeries :many
SELECT
    sm.title AS entity_title,
    sm.poster_path AS poster_path,
    st.to_state,
    st.from_state,
    st.created_at,
    st.sort_key
FROM series s
INNER JOIN series_transition st ON s.id = st.series_id
LEFT JOIN series_metadata sm ON s.series_metadata_id = sm.id
WHERE s.id = ?
ORDER BY st.sort_key ASC;

-- name: GetEntityTransitionsSeason :many
SELECT
    sm.title AS entity_title,
    sm.poster_path AS poster_path,
    st.to_state,
    st.from_state,
    st.created_at,
    st.sort_key,
    CAST(json_object(
        'downloadClient', json_object('id', dc.id, 'host', dc.host, 'port', dc.port),
        'downloadID', st.download_id
    ) AS TEXT) AS metadata
FROM season s
INNER JOIN season_transition st ON s.id = st.season_id
INNER JOIN series ser ON s.series_id = ser.id
LEFT JOIN series_metadata sm ON ser.series_metadata_id = sm.id
LEFT JOIN download_client dc ON st.download_client_id = dc.id
WHERE s.id = ?
ORDER BY st.sort_key ASC;

-- name: GetEntityTransitionsEpisode :many
SELECT
    sm.title AS entity_title,
    sm.poster_path AS poster_path,
    et.to_state,
    et.from_state,
    et.created_at,
    et.sort_key
FROM episode e
INNER JOIN episode_transition et ON e.id = et.episode_id
INNER JOIN season s ON e.season_id = s.id
INNER JOIN series ser ON s.series_id = ser.id
LEFT JOIN series_metadata sm ON ser.series_metadata_id = sm.id
WHERE e.id = ?
ORDER BY et.sort_key ASC;

-- name: GetEntityTransitionsJob :many
SELECT
    j.type AS entity_title,
    '' AS poster_path,
    jt.to_state,
    jt.from_state,
    jt.created_at,
    jt.sort_key
FROM job j
INNER JOIN job_transition jt ON j.id = jt.job_id
WHERE j.id = ?
ORDER BY jt.sort_key ASC;
