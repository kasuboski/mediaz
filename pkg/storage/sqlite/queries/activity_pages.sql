-- Per-entity history pages: window over complete history before paging.
-- name: GetMovieHistoryPage :many
WITH h AS (SELECT mm.title AS entity_title, mm.images AS poster_path, mt.to_state, mt.from_state, mt.created_at, mt.sort_key, mt.id, LEAD(mt.created_at) OVER (ORDER BY mt.sort_key,mt.id) AS next_created_at, CAST(JSON_OBJECT('downloadClient',JSON_OBJECT('id',dc.id,'host',dc.host,'port',dc.port),'downloadID',mt.download_id) AS TEXT) AS metadata FROM movie m JOIN movie_transition mt ON m.id=mt.movie_id LEFT JOIN movie_metadata mm ON m.movie_metadata_id=mm.id LEFT JOIN download_client dc ON mt.download_client_id=dc.id WHERE m.id=sqlc.arg(entity_id)) SELECT entity_title,poster_path,to_state,from_state,created_at,sort_key,metadata,next_created_at FROM h ORDER BY sort_key,id LIMIT sqlc.arg(lim) OFFSET sqlc.arg(offs);
-- name: CountMovieHistory :one
SELECT COUNT(*) FROM movie_transition WHERE movie_id=sqlc.arg(entity_id);

-- name: GetSeriesHistoryPage :many
WITH h AS (SELECT sm.title AS entity_title,sm.poster_path,st.to_state,st.from_state,st.created_at,st.sort_key,st.id,LEAD(st.created_at) OVER (ORDER BY st.sort_key,st.id) AS next_created_at FROM series s JOIN series_transition st ON s.id=st.series_id LEFT JOIN series_metadata sm ON s.series_metadata_id=sm.id WHERE s.id=sqlc.arg(entity_id)) SELECT entity_title,poster_path,to_state,from_state,created_at,sort_key,next_created_at FROM h ORDER BY sort_key,id LIMIT sqlc.arg(lim) OFFSET sqlc.arg(offs);
-- name: CountSeriesHistory :one
SELECT COUNT(*) FROM series_transition WHERE series_id=sqlc.arg(entity_id);

-- name: GetSeasonHistoryPage :many
WITH h AS (SELECT sm.title AS entity_title,sm.poster_path,st.to_state,st.from_state,st.created_at,st.sort_key,st.id,CAST(COALESCE(CAST(JSON_OBJECT('downloadClient',JSON_OBJECT('id',dc.id,'host',dc.host,'port',dc.port),'downloadID',st.download_id) AS TEXT), '') AS TEXT) AS metadata,LEAD(st.created_at) OVER (ORDER BY st.sort_key,st.id) AS next_created_at FROM season s JOIN season_transition st ON s.id=st.season_id JOIN series ser ON s.series_id=ser.id LEFT JOIN series_metadata sm ON ser.series_metadata_id=sm.id LEFT JOIN download_client dc ON st.download_client_id=dc.id WHERE s.id=sqlc.arg(entity_id)) SELECT entity_title,poster_path,to_state,from_state,created_at,sort_key,metadata,next_created_at FROM h ORDER BY sort_key,id LIMIT sqlc.arg(lim) OFFSET sqlc.arg(offs);
-- name: CountSeasonHistory :one
SELECT COUNT(*) FROM season_transition WHERE season_id=sqlc.arg(entity_id);

-- name: GetEpisodeHistoryPage :many
WITH h AS (SELECT sm.title AS entity_title,sm.poster_path,et.to_state,et.from_state,et.created_at,et.sort_key,et.id,LEAD(et.created_at) OVER (ORDER BY et.sort_key,et.id) AS next_created_at FROM episode e JOIN episode_transition et ON e.id=et.episode_id JOIN season s ON e.season_id=s.id JOIN series ser ON s.series_id=ser.id LEFT JOIN series_metadata sm ON ser.series_metadata_id=sm.id WHERE e.id=sqlc.arg(entity_id)) SELECT entity_title,poster_path,to_state,from_state,created_at,sort_key,next_created_at FROM h ORDER BY sort_key,id LIMIT sqlc.arg(lim) OFFSET sqlc.arg(offs);
-- name: CountEpisodeHistory :one
SELECT COUNT(*) FROM episode_transition WHERE episode_id=sqlc.arg(entity_id);

-- name: GetJobHistoryPage :many
WITH h AS (SELECT j.type AS entity_title,'' AS poster_path,jt.to_state,jt.from_state,jt.created_at,jt.sort_key,jt.id,LEAD(jt.created_at) OVER (ORDER BY jt.sort_key,jt.id) AS next_created_at FROM job j JOIN job_transition jt ON j.id=jt.job_id WHERE j.id=sqlc.arg(entity_id)) SELECT entity_title,poster_path,to_state,from_state,created_at,sort_key,next_created_at FROM h ORDER BY sort_key,id LIMIT sqlc.arg(lim) OFFSET sqlc.arg(offs);
-- name: CountJobHistory :one
SELECT COUNT(*) FROM job_transition WHERE job_id=sqlc.arg(entity_id);
