package sqlite

import (
	"context"

	"github.com/kasuboski/mediaz/pkg/storage"
)

func (s *SQLite) GetLibraryStats(ctx context.Context) (*storage.LibraryStats, error) {
	rows, err := s.Queries.GetLibraryStatsByState(ctx)
	if err != nil {
		return nil, err
	}

	movieStats := storage.MovieStats{
		ByState: make(map[storage.MovieState]int),
	}
	tvStats := storage.TVStats{
		ByState: make(map[storage.SeriesState]int),
	}

	for _, r := range rows {
		switch r.MediaType {
		case "movie":
			movieStats.ByState[storage.MovieState(r.State)] += int(r.Count)
			movieStats.Total += int(r.Count)
		case "series":
			tvStats.ByState[storage.SeriesState(r.State)] += int(r.Count)
			tvStats.Total += int(r.Count)
		}
	}

	return &storage.LibraryStats{
		Movies: movieStats,
		TV:     tvStats,
	}, nil
}
