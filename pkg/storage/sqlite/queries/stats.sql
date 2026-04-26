-- name: GetMovieStatsByState :many
SELECT movie_transition.to_state AS state,
       COUNT(movie.id) AS count
FROM movie
INNER JOIN movie_transition ON (movie.id = movie_transition.movie_id AND movie_transition.most_recent = 1)
GROUP BY movie_transition.to_state
ORDER BY movie_transition.to_state;

-- name: GetTVStatsByState :many
SELECT series_transition.to_state AS state,
       COUNT(series.id) AS count
FROM series
INNER JOIN series_transition ON (series.id = series_transition.series_id AND series_transition.most_recent = 1)
GROUP BY series_transition.to_state
ORDER BY series_transition.to_state;
