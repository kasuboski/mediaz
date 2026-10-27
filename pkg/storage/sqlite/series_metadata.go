package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"sort"

	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

// CreateSeriesMetadata upserts the given series metadata keyed by tmdb_id.
// When meta.ID is zero SQLite assigns a fresh autoincrement id.
func (s *SQLite) CreateSeriesMetadata(ctx context.Context, seriesMeta sqlcdb.SeriesMetadatum) (int64, error) {
	return s.Queries.UpsertSeriesMetadata(ctx, sqlcdb.UpsertSeriesMetadataParams{
		ID:             sql.NullInt64{Int64: seriesMeta.ID, Valid: seriesMeta.ID != 0},
		TmdbID:         seriesMeta.TmdbID,
		Title:          seriesMeta.Title,
		Overview:       seriesMeta.Overview,
		LastInfoSync:   seriesMeta.LastInfoSync,
		FirstAirDate:   seriesMeta.FirstAirDate,
		LastAirDate:    seriesMeta.LastAirDate,
		SeasonCount:    seriesMeta.SeasonCount,
		EpisodeCount:   seriesMeta.EpisodeCount,
		Status:         seriesMeta.Status,
		PosterPath:     seriesMeta.PosterPath,
		ExternalIds:    seriesMeta.ExternalIds,
		WatchProviders: seriesMeta.WatchProviders,
	})
}

// UpdateSeriesMetadata updates an existing series metadata record. The
// last_info_sync column is stamped with CURRENT_TIMESTAMP (UTC) by the query.
func (s *SQLite) UpdateSeriesMetadata(ctx context.Context, metadata sqlcdb.SeriesMetadatum) error {
	err := s.Queries.UpdateSeriesMetadata(ctx, sqlcdb.UpdateSeriesMetadataParams{
		Title:          metadata.Title,
		Overview:       metadata.Overview,
		FirstAirDate:   metadata.FirstAirDate,
		LastAirDate:    metadata.LastAirDate,
		SeasonCount:    metadata.SeasonCount,
		EpisodeCount:   metadata.EpisodeCount,
		Status:         metadata.Status,
		PosterPath:     metadata.PosterPath,
		ExternalIds:    metadata.ExternalIds,
		WatchProviders: metadata.WatchProviders,
		ID:             metadata.ID,
	})
	if err != nil {
		return fmt.Errorf("failed to update series metadata: %w", err)
	}

	return nil
}

// DeleteSeriesMetadata deletes a Series metadata by id
func (s *SQLite) DeleteSeriesMetadata(ctx context.Context, id int64) error {
	if err := s.Queries.DeleteSeriesMetadata(ctx, id); err != nil {
		return fmt.Errorf("failed to delete Series metadata: %w", err)
	}
	return nil
}

// ListSeriesMetadata lists all Series metadata
func (s *SQLite) ListSeriesMetadata(ctx context.Context) ([]sqlcdb.SeriesMetadatum, error) {
	rows, err := s.Queries.ListSeriesMetadata(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list Series metadata: %w", err)
	}
	return rows, nil
}

// ListSeriesMetadataByStatus lists all Series metadata with the given status
func (s *SQLite) ListSeriesMetadataByStatus(ctx context.Context, status string) ([]sqlcdb.SeriesMetadatum, error) {
	rows, err := s.Queries.ListSeriesMetadataByStatus(ctx, status)
	if err != nil {
		return nil, fmt.Errorf("failed to list Series metadata: %w", err)
	}
	return rows, nil
}

// GetSeriesMetadataByID get a Series metadata by id
func (s *SQLite) GetSeriesMetadataByID(ctx context.Context, id int64) (sqlcdb.SeriesMetadatum, error) {
	row, err := s.Queries.GetSeriesMetadataByID(ctx, id)
	if err != nil {
		return sqlcdb.SeriesMetadatum{}, mapQueryErr(err, "failed to get Series metadata")
	}
	return row, nil
}

// GetSeriesMetadataByTmdbID get a Series metadata by tmdb id
func (s *SQLite) GetSeriesMetadataByTmdbID(ctx context.Context, tmdbID int64) (sqlcdb.SeriesMetadatum, error) {
	row, err := s.Queries.GetSeriesMetadataByTmdbID(ctx, tmdbID)
	if err != nil {
		return sqlcdb.SeriesMetadatum{}, mapQueryErr(err, "failed to get Series metadata")
	}
	return row, nil
}

// CreateSeasonMetadata upserts the given season metadata keyed by tmdb_id.
// When meta.ID is zero SQLite assigns a fresh autoincrement id.
func (s *SQLite) CreateSeasonMetadata(ctx context.Context, seasonMeta sqlcdb.SeasonMetadatum) (int64, error) {
	return s.Queries.UpsertSeasonMetadata(ctx, sqlcdb.UpsertSeasonMetadataParams{
		ID:               sql.NullInt64{Int64: seasonMeta.ID, Valid: seasonMeta.ID != 0},
		SeriesMetadataID: seasonMeta.SeriesMetadataID,
		Number:           seasonMeta.Number,
		TmdbID:           seasonMeta.TmdbID,
		Title:            seasonMeta.Title,
		Overview:         seasonMeta.Overview,
		AirDate:          seasonMeta.AirDate,
	})
}

// DeleteSeasonMetadata deletes a season metadata by id
func (s *SQLite) DeleteSeasonMetadata(ctx context.Context, id int64) error {
	if err := s.Queries.DeleteSeasonMetadata(ctx, id); err != nil {
		return fmt.Errorf("failed to delete season metadata: %w", err)
	}
	return nil
}

// ListSeasonMetadataBySeriesMetadataID lists all season metadata for the given series metadata
func (s *SQLite) ListSeasonMetadataBySeriesMetadataID(ctx context.Context, seriesMetadataID int64) ([]sqlcdb.SeasonMetadatum, error) {
	rows, err := s.Queries.ListSeasonMetadataBySeriesMetadataID(ctx, seriesMetadataID)
	if err != nil {
		return nil, fmt.Errorf("failed to list season metadata: %w", err)
	}
	return rows, nil
}

// GetSeasonMetadataByID get a season metadata by id
func (s *SQLite) GetSeasonMetadataByID(ctx context.Context, id int64) (sqlcdb.SeasonMetadatum, error) {
	row, err := s.Queries.GetSeasonMetadataByID(ctx, id)
	if err != nil {
		return sqlcdb.SeasonMetadatum{}, mapQueryErr(err, "failed to get season metadata")
	}
	return row, nil
}

func uniqueSortedIDs(ids []int64) []int64 {
	ids = append([]int64(nil), ids...)
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	unique := ids[:0]
	for _, id := range ids {
		if len(unique) == 0 || unique[len(unique)-1] != id {
			unique = append(unique, id)
		}
	}
	return unique
}

// ListSeasonMetadataByIDs lists season metadata rows by primary key.
func (s *SQLite) ListSeasonMetadataByIDs(ctx context.Context, ids []int64) ([]sqlcdb.SeasonMetadatum, error) {
	ids = uniqueSortedIDs(ids)
	rows := make([]sqlcdb.SeasonMetadatum, 0)
	for start := 0; start < len(ids); start += sqliteBindLimit {
		end := min(start+sqliteBindLimit, len(ids))
		batch, err := s.Queries.ListSeasonMetadataByIDs(ctx, ids[start:end])
		if err != nil {
			return nil, fmt.Errorf("failed to list season metadata: %w", err)
		}
		rows = append(rows, batch...)
	}
	return rows, nil
}

// GetSeasonMetadataByTmdbID get a season metadata by tmdb id
func (s *SQLite) GetSeasonMetadataByTmdbID(ctx context.Context, tmdbID int64) (sqlcdb.SeasonMetadatum, error) {
	row, err := s.Queries.GetSeasonMetadataByTmdbID(ctx, tmdbID)
	if err != nil {
		return sqlcdb.SeasonMetadatum{}, mapQueryErr(err, "failed to get season metadata")
	}
	return row, nil
}

// CreateEpisodeMetadata upserts the given episode metadata keyed by tmdb_id.
// When meta.ID is zero SQLite assigns a fresh autoincrement id.
func (s *SQLite) CreateEpisodeMetadata(ctx context.Context, episodeMeta sqlcdb.EpisodeMetadatum) (int64, error) {
	return s.Queries.UpsertEpisodeMetadata(ctx, sqlcdb.UpsertEpisodeMetadataParams{
		ID:               sql.NullInt64{Int64: episodeMeta.ID, Valid: episodeMeta.ID != 0},
		SeasonMetadataID: episodeMeta.SeasonMetadataID,
		Number:           episodeMeta.Number,
		TmdbID:           episodeMeta.TmdbID,
		Title:            episodeMeta.Title,
		Overview:         episodeMeta.Overview,
		AirDate:          episodeMeta.AirDate,
		Runtime:          episodeMeta.Runtime,
		StillPath:        episodeMeta.StillPath,
	})
}

// DeleteEpisodeMetadata deletes an episode metadata by id
func (s *SQLite) DeleteEpisodeMetadata(ctx context.Context, id int64) error {
	if err := s.Queries.DeleteEpisodeMetadata(ctx, id); err != nil {
		return fmt.Errorf("failed to delete episode metadata: %w", err)
	}
	return nil
}

// ListEpisodeMetadataBySeasonMetadataID lists all episode metadata for the given season metadata
func (s *SQLite) ListEpisodeMetadataBySeasonMetadataID(ctx context.Context, seasonMetadataID int64) ([]sqlcdb.EpisodeMetadatum, error) {
	rows, err := s.Queries.ListEpisodeMetadataBySeasonMetadataID(ctx, seasonMetadataID)
	if err != nil {
		return nil, fmt.Errorf("failed to list episode metadata: %w", err)
	}
	return rows, nil
}

// ListEpisodeMetadataByIDs lists all episode metadata for the given ids
func (s *SQLite) ListEpisodeMetadataByIDs(ctx context.Context, ids []int64) ([]sqlcdb.EpisodeMetadatum, error) {
	ids = uniqueSortedIDs(ids)
	rows := make([]sqlcdb.EpisodeMetadatum, 0)
	for start := 0; start < len(ids); start += sqliteBindLimit {
		end := min(start+sqliteBindLimit, len(ids))
		batch, err := s.Queries.ListEpisodeMetadataByIDs(ctx, ids[start:end])
		if err != nil {
			return nil, fmt.Errorf("failed to list episode metadata: %w", err)
		}
		rows = append(rows, batch...)
	}
	return rows, nil
}

// GetEpisodeMetadataByID get an episode metadata by id
func (s *SQLite) GetEpisodeMetadataByID(ctx context.Context, id int64) (sqlcdb.EpisodeMetadatum, error) {
	row, err := s.Queries.GetEpisodeMetadataByID(ctx, id)
	if err != nil {
		return sqlcdb.EpisodeMetadatum{}, mapQueryErr(err, "failed to get episode metadata")
	}
	return row, nil
}

// GetEpisodeMetadataByTmdbID get an episode metadata by tmdb id
func (s *SQLite) GetEpisodeMetadataByTmdbID(ctx context.Context, tmdbID int64) (sqlcdb.EpisodeMetadatum, error) {
	row, err := s.Queries.GetEpisodeMetadataByTmdbID(ctx, tmdbID)
	if err != nil {
		return sqlcdb.EpisodeMetadatum{}, mapQueryErr(err, "failed to get episode metadata")
	}
	return row, nil
}

// GetEpisodeMetadataBySeasonAndNumber get an episode metadata by season metadata id and episode number
func (s *SQLite) GetEpisodeMetadataBySeasonAndNumber(ctx context.Context, seasonMetadataID, number int64) (sqlcdb.EpisodeMetadatum, error) {
	row, err := s.Queries.GetEpisodeMetadataBySeasonAndNumber(ctx, sqlcdb.GetEpisodeMetadataBySeasonAndNumberParams{
		SeasonMetadataID: seasonMetadataID,
		Number:           number,
	})
	if err != nil {
		return sqlcdb.EpisodeMetadatum{}, mapQueryErr(err, "failed to get episode metadata")
	}
	return row, nil
}
