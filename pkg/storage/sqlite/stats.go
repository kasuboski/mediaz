package sqlite

import (
	"context"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
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

// GetMovieStatsByState returns movie counts aggregated by state in a single query using Jet ORM
func (s *SQLite) GetMovieStatsByState(ctx context.Context) ([]storage.MovieStatsByState, error) {
	stmt := sqlite.SELECT(
		table.MovieTransition.ToState.AS("state"),
		sqlite.COUNT(table.Movie.ID).AS("count"),
	).
		FROM(table.Movie.LEFT_JOIN(table.MovieTransition, table.Movie.ID.EQ(table.MovieTransition.MovieID))).
		GROUP_BY(table.MovieTransition.ToState).
		ORDER_BY(table.MovieTransition.ToState)

	var dest []storage.MovieStatsByState
	err := stmt.QueryContext(ctx, s.db, &dest)
	if err != nil {
		return nil, err
	}

	return dest, nil
}

// GetTVStatsByState returns TV series counts aggregated by state in a single query using Jet ORM
func (s *SQLite) GetTVStatsByState(ctx context.Context) ([]storage.TVStatsByState, error) {
	stmt := sqlite.SELECT(
		table.SeriesTransition.ToState.AS("state"),
		sqlite.COUNT(table.Series.ID).AS("count"),
	).
		FROM(table.Series.LEFT_JOIN(table.SeriesTransition, table.Series.ID.EQ(table.SeriesTransition.SeriesID))).
		GROUP_BY(table.SeriesTransition.ToState).
		ORDER_BY(table.SeriesTransition.ToState)

	var dest []storage.TVStatsByState
	err := stmt.QueryContext(ctx, s.db, &dest)
	if err != nil {
		return nil, err
	}

	return dest, nil
}

// GetMovieStatsByStateRaw executes the same query using raw SQL for maximum performance
func (s *SQLite) GetMovieStatsByStateRaw(ctx context.Context) ([]storage.MovieStatsByState, error) {
	query := `
		SELECT 
			COALESCE(mt.to_state, '') as state,
			COUNT(m.id) as count
		FROM movie m
		LEFT JOIN movie_transition mt ON mt.movie_id = m.id
		GROUP BY mt.to_state
		ORDER BY mt.to_state
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []storage.MovieStatsByState
	for rows.Next() {
		var stat storage.MovieStatsByState
		err := rows.Scan(&stat.State, &stat.Count)
		if err != nil {
			return nil, err
		}
		results = append(results, stat)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

// GetTVStatsByStateRaw executes the same query using raw SQL for maximum performance
func (s *SQLite) GetTVStatsByStateRaw(ctx context.Context) ([]storage.TVStatsByState, error) {
	query := `
		SELECT 
			COALESCE(st.to_state, '') as state,
			COUNT(s.id) as count
		FROM series s
		LEFT JOIN series_transition st ON st.series_id = s.id
		GROUP BY st.to_state
		ORDER BY st.to_state
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []storage.TVStatsByState
	for rows.Next() {
		var stat storage.TVStatsByState
		err := rows.Scan(&stat.State, &stat.Count)
		if err != nil {
			return nil, err
		}
		results = append(results, stat)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

// GetLibraryStats returns comprehensive library statistics in minimal queries
func (s *SQLite) GetLibraryStats(ctx context.Context) (*storage.LibraryStats, error) {
	// Execute both queries in parallel for better performance
	type movieResult struct {
		stats []storage.MovieStatsByState
		err   error
	}
	type tvResult struct {
		stats []storage.TVStatsByState
		err   error
	}

	movieCh := make(chan movieResult, 1)
	tvCh := make(chan tvResult, 1)

	// Get movie stats
	go func() {
		stats, err := s.GetMovieStatsByStateRaw(ctx)
		movieCh <- movieResult{stats: stats, err: err}
	}()

	// Get TV stats
	go func() {
		stats, err := s.GetTVStatsByStateRaw(ctx)
		tvCh <- tvResult{stats: stats, err: err}
	}()

	// Wait for both results
	movieRes := <-movieCh
	tvRes := <-tvCh

	if movieRes.err != nil {
		return nil, movieRes.err
	}
	if tvRes.err != nil {
		return nil, tvRes.err
	}

	// Transform movie stats
	movieStats := storage.MovieStats{
		ByState: make(map[storage.MovieState]int),
	}
	for _, stat := range movieRes.stats {
		movieStats.ByState[storage.MovieState(stat.State)] = stat.Count
		movieStats.Total += stat.Count
	}

	// Transform TV stats
	tvStats := storage.TVStats{
		ByState: make(map[storage.SeriesState]int),
	}
	for _, stat := range tvRes.stats {
		tvStats.ByState[storage.SeriesState(stat.State)] = stat.Count
		tvStats.Total += stat.Count
	}

	return &storage.LibraryStats{
		Movies: movieStats,
		TV:     tvStats,
	}, nil
}
