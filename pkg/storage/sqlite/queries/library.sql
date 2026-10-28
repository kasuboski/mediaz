-- Library reads deliberately require metadata so paging/counts reflect visible library entries.
-- name: ListLibraryMovies :many
SELECT sqlc.embed(m), sqlc.embed(mm), mt.to_state AS state
FROM movie AS m
INNER JOIN movie_metadata AS mm ON m.movie_metadata_id = mm.id
INNER JOIN movie_transition AS mt ON m.id = mt.movie_id AND mt.most_recent = 1
WHERE json_array_length(CAST(sqlc.arg('states') AS TEXT)) = 0 OR mt.to_state IN (SELECT value FROM json_each(CAST(sqlc.arg('states') AS TEXT)))
ORDER BY mm.title ASC, m.id ASC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountLibraryMovies :one
SELECT COUNT(*)
FROM movie AS m
INNER JOIN movie_metadata AS mm ON m.movie_metadata_id = mm.id
INNER JOIN movie_transition AS mt ON m.id = mt.movie_id AND mt.most_recent = 1
WHERE json_array_length(CAST(sqlc.arg('states') AS TEXT)) = 0 OR mt.to_state IN (SELECT value FROM json_each(CAST(sqlc.arg('states') AS TEXT)));

-- name: CountLibraryMoviesByState :many
SELECT mt.to_state AS state, COUNT(*) AS count
FROM movie AS m
INNER JOIN movie_metadata AS mm ON m.movie_metadata_id = mm.id
INNER JOIN movie_transition AS mt ON m.id = mt.movie_id AND mt.most_recent = 1
GROUP BY mt.to_state;

-- name: ListLibraryShows :many
SELECT sqlc.embed(s), sqlc.embed(sm), st.to_state AS state
FROM series AS s
INNER JOIN series_metadata AS sm ON s.series_metadata_id = sm.id
INNER JOIN series_transition AS st ON s.id = st.series_id AND st.most_recent = 1
WHERE json_array_length(CAST(sqlc.arg('states') AS TEXT)) = 0 OR st.to_state IN (SELECT value FROM json_each(CAST(sqlc.arg('states') AS TEXT)))
ORDER BY sm.title ASC, s.id ASC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountLibraryShows :one
SELECT COUNT(*)
FROM series AS s
INNER JOIN series_metadata AS sm ON s.series_metadata_id = sm.id
INNER JOIN series_transition AS st ON s.id = st.series_id AND st.most_recent = 1
WHERE json_array_length(CAST(sqlc.arg('states') AS TEXT)) = 0 OR st.to_state IN (SELECT value FROM json_each(CAST(sqlc.arg('states') AS TEXT)));

-- name: CountLibraryShowsByState :many
SELECT st.to_state AS state, COUNT(*) AS count
FROM series AS s
INNER JOIN series_metadata AS sm ON s.series_metadata_id = sm.id
INNER JOIN series_transition AS st ON s.id = st.series_id AND st.most_recent = 1
GROUP BY st.to_state;
