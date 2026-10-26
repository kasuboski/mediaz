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

// CreateEpisodeMetadata creates the given episodeMeta
func (s *SQLite) CreateEpisodeMetadata(ctx context.Context, episodeMeta model.EpisodeMetadata) (int64, error) {
	// don't insert a zeroed ID
	var id sql.NullInt64
	if episodeMeta.ID != 0 {
		id = sql.NullInt64{Int64: int64(episodeMeta.ID), Valid: true}
	}

	err := s.Queries.UpsertEpisodeMetadata(ctx, sqlcdb.UpsertEpisodeMetadataParams{
		ID:               id,
		SeasonMetadataID: int64(episodeMeta.SeasonMetadataID),
		Number:           int64(episodeMeta.Number),
		TmdbID:           int64(episodeMeta.TmdbID),
		Title:            episodeMeta.Title,
		Overview:         metaStrPtr(episodeMeta.Overview),
		AirDate:          metaTimePtr(episodeMeta.AirDate),
		Runtime:          metaInt32Ptr(episodeMeta.Runtime),
		StillPath:        metaStrPtr(episodeMeta.StillPath),
	})
	if err != nil {
		return 0, err
	}

	metadataID, err := s.Queries.GetEpisodeMetadataIDByTmdbID(ctx, int64(episodeMeta.TmdbID))
	if err != nil {
		return 0, fmt.Errorf("failed to get episode metadata ID after upsert: %w", err)
	}

	return metadataID, nil
}

// DeleteEpisodeMetadata deletes an episode metadata by id
func (s *SQLite) DeleteEpisodeMetadata(ctx context.Context, id int64) error {
	err := s.Queries.DeleteEpisodeMetadata(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to delete episode metadata: %w", err)
	}

	return nil
}

// ListEpisodeMetadataBySeasonMetadataID lists all episode metadata for the given season metadata
func (s *SQLite) ListEpisodeMetadataBySeasonMetadataID(ctx context.Context, seasonMetadataID int32) ([]*model.EpisodeMetadata, error) {
	rows, err := s.Queries.ListEpisodeMetadataBySeasonMetadataID(ctx, int64(seasonMetadataID))
	if err != nil {
		return nil, fmt.Errorf("failed to list episode metadata: %w", err)
	}

	episodeMetadata := make([]*model.EpisodeMetadata, 0, len(rows))
	for _, row := range rows {
		episodeMetadata = append(episodeMetadata, episodeMetadataFromRow(row))
	}

	return episodeMetadata, nil
}

// ListEpisodeMetadataByIDs lists all episode metadata for the given ids
func (s *SQLite) ListEpisodeMetadataByIDs(ctx context.Context, ids []int64) ([]*model.EpisodeMetadata, error) {
	rows, err := s.Queries.ListEpisodeMetadataByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("failed to list episode metadata: %w", err)
	}

	episodeMetadata := make([]*model.EpisodeMetadata, 0, len(rows))
	for _, row := range rows {
		episodeMetadata = append(episodeMetadata, episodeMetadataFromRow(row))
	}

	return episodeMetadata, nil
}

// GetEpisodeMetadataByID get an episode metadata by id
func (s *SQLite) GetEpisodeMetadataByID(ctx context.Context, id int64) (*model.EpisodeMetadata, error) {
	row, err := s.Queries.GetEpisodeMetadataByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get episode metadata: %w", err)
	}

	return episodeMetadataFromRow(row), nil
}

// GetEpisodeMetadataByTmdbID get an episode metadata by tmdb id
func (s *SQLite) GetEpisodeMetadataByTmdbID(ctx context.Context, tmdbID int64) (*model.EpisodeMetadata, error) {
	row, err := s.Queries.GetEpisodeMetadataByTmdbID(ctx, tmdbID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get episode metadata: %w", err)
	}

	return episodeMetadataFromRow(row), nil
}

// GetEpisodeMetadataBySeasonAndNumber get an episode metadata by season metadata id and episode number
func (s *SQLite) GetEpisodeMetadataBySeasonAndNumber(ctx context.Context, seasonMetadataID, number int32) (*model.EpisodeMetadata, error) {
	row, err := s.Queries.GetEpisodeMetadataBySeasonAndNumber(ctx, sqlcdb.GetEpisodeMetadataBySeasonAndNumberParams{
		SeasonMetadataID: int64(seasonMetadataID),
		Number:           int64(number),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get episode metadata: %w", err)
	}

	return episodeMetadataFromRow(row), nil
}

// episodeMetadataFromRow converts a sqlc episode_metadata row into the domain
// model, which preserves the legacy Jet shapes (int32 IDs, pointer nullables).
func episodeMetadataFromRow(row sqlcdb.EpisodeMetadatum) *model.EpisodeMetadata {
	return &model.EpisodeMetadata{
		ID:               int32(row.ID),
		SeasonMetadataID: int32(row.SeasonMetadataID),
		Number:           int32(row.Number),
		TmdbID:           int32(row.TmdbID),
		Title:            row.Title,
		Overview:         metaNullStr(row.Overview),
		AirDate:          metaNullTime(row.AirDate),
		Runtime:          metaNullInt32(row.Runtime),
		StillPath:        metaNullStr(row.StillPath),
	}
}
