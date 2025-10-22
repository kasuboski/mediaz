package sqlite

import (
	"context"

	"github.com/kasuboski/mediaz/pkg/storage"
)

// MovieStatsByState represents the result of movie statistics aggregation
type MovieStatsByState struct {
	State string `json:"state"`
	Count int    `json:"count"`
}

// TVStatsByState represents the result of TV series statistics aggregation
type TVStatsByState struct {
	State string `json:"state"`
	Count int    `json:"count"`
}

// GetMovieStatsByState returns movie counts aggregated by state in a single query
func (s *SQLite) GetMovieStatsByState(ctx context.Context) ([]storage.MovieStatsByState, error) {
	// Use raw SQL since Jet ORM doesn't properly handle aggregate queries with custom structs
	s.mu.Lock()
	rows, err := s.db.QueryContext(ctx, `
		SELECT movie_transition.to_state AS state,
		       COUNT(movie.id) AS count
		FROM movie
		INNER JOIN movie_transition ON (movie.id = movie_transition.movie_id AND movie_transition.most_recent = 1)
		GROUP BY movie_transition.to_state
		ORDER BY movie_transition.to_state
	`)
	s.mu.Unlock()

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dest []storage.MovieStatsByState
	for rows.Next() {
		var stat storage.MovieStatsByState
		if err := rows.Scan(&stat.State, &stat.Count); err != nil {
			return nil, err
		}
		dest = append(dest, stat)
	}

	return dest, rows.Err()
}

// GetTVStatsByState returns TV series counts aggregated by state in a single query
func (s *SQLite) GetTVStatsByState(ctx context.Context) ([]storage.TVStatsByState, error) {
	// Use raw SQL since Jet ORM doesn't properly handle aggregate queries with custom structs
	s.mu.Lock()
	rows, err := s.db.QueryContext(ctx, `
		SELECT series_transition.to_state AS state,
		       COUNT(series.id) AS count
		FROM series
		INNER JOIN series_transition ON (series.id = series_transition.series_id AND series_transition.most_recent = 1)
		GROUP BY series_transition.to_state
		ORDER BY series_transition.to_state
	`)
	s.mu.Unlock()

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dest []storage.TVStatsByState
	for rows.Next() {
		var stat storage.TVStatsByState
		if err := rows.Scan(&stat.State, &stat.Count); err != nil {
			return nil, err
		}
		dest = append(dest, stat)
	}

	return dest, rows.Err()
}

// GetLibraryStats returns comprehensive library statistics in minimal queries
func (s *SQLite) GetLibraryStats(ctx context.Context) (*storage.LibraryStats, error) {
	// Execute queries sequentially to avoid SQLite connection pool issues with in-memory databases
	movieRes, err := s.GetMovieStatsByState(ctx)
	if err != nil {
		return nil, err
	}

	tvRes, err := s.GetTVStatsByState(ctx)
	if err != nil {
		return nil, err
	}

	// Transform movie stats
	movieStats := storage.MovieStats{
		ByState: make(map[storage.MovieState]int),
	}
	for _, stat := range movieRes {
		movieStats.ByState[storage.MovieState(stat.State)] = stat.Count
		movieStats.Total += stat.Count
	}

	// Transform TV stats
	tvStats := storage.TVStats{
		ByState: make(map[storage.SeriesState]int),
	}
	for _, stat := range tvRes {
		tvStats.ByState[storage.SeriesState(stat.State)] = stat.Count
		tvStats.Total += stat.Count
	}

	return &storage.LibraryStats{
		Movies: movieStats,
		TV:     tvStats,
	}, nil
}
