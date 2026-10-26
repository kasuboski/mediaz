package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

// CreateSeriesMetadata creates the given series metadata
func (s *SQLite) CreateSeriesMetadata(ctx context.Context, seriesMeta model.SeriesMetadata) (int64, error) {
	// don't insert a zeroed ID
	var id sql.NullInt64
	if seriesMeta.ID != 0 {
		id = sql.NullInt64{Int64: int64(seriesMeta.ID), Valid: true}
	}

	err := s.Queries.UpsertSeriesMetadata(ctx, sqlcdb.UpsertSeriesMetadataParams{
		ID:             id,
		TmdbID:         int64(seriesMeta.TmdbID),
		Title:          seriesMeta.Title,
		Overview:       metaStrPtr(seriesMeta.Overview),
		LastInfoSync:   metaTimePtr(seriesMeta.LastInfoSync),
		FirstAirDate:   metaTimePtr(seriesMeta.FirstAirDate),
		LastAirDate:    metaTimePtr(seriesMeta.LastAirDate),
		SeasonCount:    int64(seriesMeta.SeasonCount),
		EpisodeCount:   int64(seriesMeta.EpisodeCount),
		Status:         seriesMeta.Status,
		PosterPath:     metaStrPtr(seriesMeta.PosterPath),
		ExternalIds:    metaStrPtr(seriesMeta.ExternalIds),
		WatchProviders: metaStrPtr(seriesMeta.WatchProviders),
	})
	if err != nil {
		return 0, err
	}

	metadataID, err := s.Queries.GetSeriesMetadataIDByTmdbID(ctx, int64(seriesMeta.TmdbID))
	if err != nil {
		return 0, fmt.Errorf("failed to get series metadata ID after upsert: %w", err)
	}

	return metadataID, nil
}

// UpdateSeriesMetadata updates an existing series metadata record
func (s *SQLite) UpdateSeriesMetadata(ctx context.Context, metadata model.SeriesMetadata) error {
	now := time.Now()
	metadata.LastInfoSync = &now

	err := s.Queries.UpdateSeriesMetadata(ctx, sqlcdb.UpdateSeriesMetadataParams{
		Title:          metadata.Title,
		Overview:       metaStrPtr(metadata.Overview),
		LastInfoSync:   metaTimePtr(metadata.LastInfoSync),
		FirstAirDate:   metaTimePtr(metadata.FirstAirDate),
		LastAirDate:    metaTimePtr(metadata.LastAirDate),
		SeasonCount:    int64(metadata.SeasonCount),
		EpisodeCount:   int64(metadata.EpisodeCount),
		Status:         metadata.Status,
		PosterPath:     metaStrPtr(metadata.PosterPath),
		ExternalIds:    metaStrPtr(metadata.ExternalIds),
		WatchProviders: metaStrPtr(metadata.WatchProviders),
		ID:             int64(metadata.ID),
	})
	if err != nil {
		return fmt.Errorf("failed to update series metadata: %w", err)
	}

	return nil
}

// DeleteSeriesMetadata deletes a Series metadata by id
func (s *SQLite) DeleteSeriesMetadata(ctx context.Context, id int64) error {
	err := s.Queries.DeleteSeriesMetadata(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to delete Series metadata: %w", err)
	}

	return nil
}

// ListSeriesMetadata lists all Series metadata
func (s *SQLite) ListSeriesMetadata(ctx context.Context) ([]*model.SeriesMetadata, error) {
	rows, err := s.Queries.ListSeriesMetadata(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list Series metadata: %w", err)
	}

	seriesMetadata := make([]*model.SeriesMetadata, 0, len(rows))
	for _, row := range rows {
		seriesMetadata = append(seriesMetadata, seriesMetadataFromRow(row))
	}

	return seriesMetadata, nil
}

// ListSeriesMetadataByStatus lists all Series metadata with the given status
func (s *SQLite) ListSeriesMetadataByStatus(ctx context.Context, status string) ([]*model.SeriesMetadata, error) {
	rows, err := s.Queries.ListSeriesMetadataByStatus(ctx, status)
	if err != nil {
		return nil, fmt.Errorf("failed to list Series metadata: %w", err)
	}

	seriesMetadata := make([]*model.SeriesMetadata, 0, len(rows))
	for _, row := range rows {
		seriesMetadata = append(seriesMetadata, seriesMetadataFromRow(row))
	}

	return seriesMetadata, nil
}

// GetSeriesMetadataByID get a Series metadata by id
func (s *SQLite) GetSeriesMetadataByID(ctx context.Context, id int64) (*model.SeriesMetadata, error) {
	row, err := s.Queries.GetSeriesMetadataByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get Series metadata: %w", err)
	}

	return seriesMetadataFromRow(row), nil
}

// GetSeriesMetadataByTmdbID get a Series metadata by tmdb id
func (s *SQLite) GetSeriesMetadataByTmdbID(ctx context.Context, tmdbID int64) (*model.SeriesMetadata, error) {
	row, err := s.Queries.GetSeriesMetadataByTmdbID(ctx, tmdbID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get Series metadata: %w", err)
	}

	return seriesMetadataFromRow(row), nil
}

// UpdateSeriesState updates the transition state of a series
// Metadata is optional and can be nil
func (s *SQLite) UpdateSeriesState(ctx context.Context, id int64, state storage.SeriesState, metadata *storage.TransitionStateMetadata) error {
	series, err := s.GetSeriesByID(ctx, id)
	if err != nil {
		return err
	}

	err = series.Machine().ToState(state)
	if err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	// Note: SeriesTransition doesn't currently support DownloadClientID and DownloadID
	// These can be added to the schema if needed in the future
	_ = metadata

	previousSortKey, err := s.txQueries(tx).DemoteSeriesTransition(ctx, id)
	if err != nil {
		tx.Rollback()
		return err
	}

	err = s.txQueries(tx).InsertSeriesTransition(ctx, sqlcdb.InsertSeriesTransitionParams{
		SeriesID:   id,
		ToState:    string(state),
		FromState:  sql.NullString{},
		MostRecent: true,
		SortKey:    previousSortKey + 1,
	})
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

// UpdateSeriesByID updates fields on a series. Like the legacy statement it
// only ever writes the monitored column.
func (s *SQLite) UpdateSeriesByID(ctx context.Context, id int64, series model.Series) error {
	return s.Queries.UpdateSeriesByID(ctx, sqlcdb.UpdateSeriesByIDParams{
		Monitored: int64(series.Monitored),
		ID:        id,
	})
}

// LinkSeriesMetadata links a series with its metadata
func (s *SQLite) LinkSeriesMetadata(ctx context.Context, seriesID int64, metadataID int32) error {
	return s.Queries.LinkSeriesMetadata(ctx, sqlcdb.LinkSeriesMetadataParams{
		SeriesMetadataID: sql.NullInt64{Int64: int64(metadataID), Valid: true},
		ID:               seriesID,
	})
}

// seriesMetadataFromRow converts a sqlc series_metadata row into the domain
// model, which preserves the legacy Jet shapes (int32 IDs, pointer nullables).
func seriesMetadataFromRow(row sqlcdb.SeriesMetadatum) *model.SeriesMetadata {
	return &model.SeriesMetadata{
		ID:             int32(row.ID),
		TmdbID:         int32(row.TmdbID),
		Title:          row.Title,
		Overview:       metaNullStr(row.Overview),
		LastInfoSync:   metaNullTime(row.LastInfoSync),
		FirstAirDate:   metaNullTime(row.FirstAirDate),
		LastAirDate:    metaNullTime(row.LastAirDate),
		SeasonCount:    int32(row.SeasonCount),
		EpisodeCount:   int32(row.EpisodeCount),
		Status:         row.Status,
		PosterPath:     metaNullStr(row.PosterPath),
		ExternalIds:    metaNullStr(row.ExternalIds),
		WatchProviders: metaNullStr(row.WatchProviders),
	}
}

// The meta* helpers convert between sqlc's sql.Null* column shapes and the
// domain models' legacy pointer shapes. They are specific to the metadata/file
// implementation group; see MIGRATION_COORDINATION.md on helper naming.

// metaNullStr converts a nullable string column to the domain pointer shape.
func metaNullStr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	s := v.String
	return &s
}

// metaStrPtr converts a domain string pointer to a nullable string column.
func metaStrPtr(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

// metaNullTime converts a nullable time column to the domain pointer shape.
func metaNullTime(v sql.NullTime) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time
	return &t
}

// metaTimePtr converts a domain time pointer to a nullable time column.
func metaTimePtr(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *t, Valid: true}
}

// metaNullInt32 converts a nullable integer column to the domain int32 pointer.
func metaNullInt32(v sql.NullInt64) *int32 {
	if !v.Valid {
		return nil
	}
	n := int32(v.Int64)
	return &n
}

// metaInt32Ptr converts a domain int32 pointer to a nullable integer column.
func metaInt32Ptr(n *int32) sql.NullInt64 {
	if n == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*n), Valid: true}
}
