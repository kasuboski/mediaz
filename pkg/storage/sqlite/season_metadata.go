package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

// CreateSeasonMetadata creates the given seasonMeta
func (s *SQLite) CreateSeasonMetadata(ctx context.Context, seasonMeta model.SeasonMetadata) (int64, error) {
	// don't insert a zeroed ID
	var id sql.NullInt64
	if seasonMeta.ID != 0 {
		id = sql.NullInt64{Int64: int64(seasonMeta.ID), Valid: true}
	}

	err := s.Queries.UpsertSeasonMetadata(ctx, sqlcdb.UpsertSeasonMetadataParams{
		ID:               id,
		SeriesMetadataID: int64(seasonMeta.SeriesMetadataID),
		Number:           int64(seasonMeta.Number),
		TmdbID:           int64(seasonMeta.TmdbID),
		Title:            seasonMeta.Title,
		Overview:         metaStrPtr(seasonMeta.Overview),
		AirDate:          metaTimePtr(seasonMeta.AirDate),
	})
	if err != nil {
		return 0, err
	}

	metadataID, err := s.Queries.GetSeasonMetadataIDByTmdbID(ctx, int64(seasonMeta.TmdbID))
	if err != nil {
		return 0, fmt.Errorf("failed to get season metadata ID after upsert: %w", err)
	}

	return metadataID, nil
}

// DeleteSeasonMetadata deletes a season metadata by id
func (s *SQLite) DeleteSeasonMetadata(ctx context.Context, id int64) error {
	err := s.Queries.DeleteSeasonMetadata(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to delete season metadata: %w", err)
	}

	return nil
}

// ListSeasonMetadataBySeriesMetadataID lists all season metadata for the given series metadata
func (s *SQLite) ListSeasonMetadataBySeriesMetadataID(ctx context.Context, seriesMetadataID int32) ([]*model.SeasonMetadata, error) {
	rows, err := s.Queries.ListSeasonMetadataBySeriesMetadataID(ctx, int64(seriesMetadataID))
	if err != nil {
		return nil, fmt.Errorf("failed to list season metadata: %w", err)
	}

	seasonMetadata := make([]*model.SeasonMetadata, 0, len(rows))
	for _, row := range rows {
		seasonMetadata = append(seasonMetadata, seasonMetadataFromRow(row))
	}

	return seasonMetadata, nil
}

// GetSeasonMetadataByID get a season metadata by id
func (s *SQLite) GetSeasonMetadataByID(ctx context.Context, id int64) (*model.SeasonMetadata, error) {
	row, err := s.Queries.GetSeasonMetadataByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get season metadata: %w", err)
	}

	return seasonMetadataFromRow(row), nil
}

// GetSeasonMetadataByTmdbID get a season metadata by tmdb id
func (s *SQLite) GetSeasonMetadataByTmdbID(ctx context.Context, tmdbID int64) (*model.SeasonMetadata, error) {
	row, err := s.Queries.GetSeasonMetadataByTmdbID(ctx, tmdbID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get season metadata: %w", err)
	}

	return seasonMetadataFromRow(row), nil
}

// seasonMetadataFromRow converts a sqlc season_metadata row into the domain
// model, which preserves the legacy Jet shapes (int32 IDs, pointer nullables).
func seasonMetadataFromRow(row sqlcdb.SeasonMetadatum) *model.SeasonMetadata {
	return &model.SeasonMetadata{
		ID:               int32(row.ID),
		SeriesMetadataID: int32(row.SeriesMetadataID),
		Number:           int32(row.Number),
		TmdbID:           int32(row.TmdbID),
		Title:            row.Title,
		Overview:         metaNullStr(row.Overview),
		AirDate:          metaNullTime(row.AirDate),
	}
}
