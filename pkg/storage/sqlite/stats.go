package sqlite

import (
	"context"

	"github.com/kasuboski/mediaz/pkg/storage"
)

func (s *SQLite) GetLibraryStats(ctx context.Context) (*storage.LibraryStats, error) {
	movieRows, err := s.Queries.GetMovieStatsByState(ctx)
	if err != nil {
		return nil, err
	}

	tvRows, err := s.Queries.GetTVStatsByState(ctx)
	if err != nil {
		return nil, err
	}

	movieStats := storage.MovieStats{
		ByState: make(map[storage.MovieState]int),
	}
	for _, r := range movieRows {
		movieStats.ByState[storage.MovieState(r.State)] = int(r.Count)
		movieStats.Total += int(r.Count)
	}

	tvStats := storage.TVStats{
		ByState: make(map[storage.SeriesState]int),
	}
	for _, r := range tvRows {
		tvStats.ByState[storage.SeriesState(r.State)] = int(r.Count)
		tvStats.Total += int(r.Count)
	}

	return &storage.LibraryStats{
		Movies: movieStats,
		TV:     tvStats,
	}, nil
}
