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

// GetEpisodeFileByID gets all episode files for an episode
func (s *SQLite) GetEpisodeFileByID(ctx context.Context, id int64) ([]*model.EpisodeFile, error) {
	rows, err := s.Queries.GetEpisodeFileByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		return nil, storage.ErrNotFound
	}

	result := make([]*model.EpisodeFile, 0, len(rows))
	for _, row := range rows {
		result = append(result, episodeFileFromRow(row))
	}

	return result, nil
}

// GetEpisodeFile gets an episode file by id
func (s *SQLite) GetEpisodeFile(ctx context.Context, id int32) (*model.EpisodeFile, error) {
	row, err := s.Queries.GetEpisodeFile(ctx, int64(id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, err
	}

	return episodeFileFromRow(row), nil
}

// UpdateEpisodeFile updates an episode file
func (s *SQLite) UpdateEpisodeFile(ctx context.Context, id int32, file model.EpisodeFile) error {
	return s.Queries.UpdateEpisodeFile(ctx, sqlcdb.UpdateEpisodeFileParams{
		Quality:          file.Quality,
		Size:             file.Size,
		Added:            file.Added,
		RelativePath:     metaStrPtr(file.RelativePath),
		OriginalFilePath: metaStrPtr(file.OriginalFilePath),
		ID:               int64(id),
	})
}

// CreateEpisodeFile stores an episode file
func (s *SQLite) CreateEpisodeFile(ctx context.Context, file model.EpisodeFile) (int64, error) {
	// don't insert a zeroed ID: when an explicit id is given the added column
	// is inserted too, otherwise AUTOINCREMENT and the column default
	// CURRENT_TIMESTAMP apply, matching the legacy statement branches
	if file.ID != 0 {
		return s.Queries.CreateEpisodeFileWithID(ctx, sqlcdb.CreateEpisodeFileWithIDParams{
			ID:               int64(file.ID),
			Quality:          file.Quality,
			Size:             file.Size,
			Added:            file.Added,
			RelativePath:     metaStrPtr(file.RelativePath),
			OriginalFilePath: metaStrPtr(file.OriginalFilePath),
		})
	}

	return s.Queries.CreateEpisodeFile(ctx, sqlcdb.CreateEpisodeFileParams{
		Quality:          file.Quality,
		Size:             file.Size,
		RelativePath:     metaStrPtr(file.RelativePath),
		OriginalFilePath: metaStrPtr(file.OriginalFilePath),
	})
}

// DeleteEpisodeFile removes an episode file by id
func (s *SQLite) DeleteEpisodeFile(ctx context.Context, id int64) error {
	return s.Queries.DeleteEpisodeFile(ctx, id)
}

// ListEpisodeFiles lists all episode files
func (s *SQLite) ListEpisodeFiles(ctx context.Context) ([]*model.EpisodeFile, error) {
	rows, err := s.Queries.ListEpisodeFiles(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list episode files: %w", err)
	}

	episodeFiles := make([]*model.EpisodeFile, 0, len(rows))
	for _, row := range rows {
		episodeFiles = append(episodeFiles, episodeFileFromRow(row))
	}

	return episodeFiles, nil
}

// episodeFileFromRow converts a sqlc episode_file row into the domain model,
// which preserves the legacy Jet shapes (int32 IDs, pointer nullables).
func episodeFileFromRow(row sqlcdb.EpisodeFile) *model.EpisodeFile {
	return &model.EpisodeFile{
		ID:               int32(row.ID),
		Quality:          row.Quality,
		Size:             row.Size,
		Added:            row.Added,
		RelativePath:     metaNullStr(row.RelativePath),
		OriginalFilePath: metaNullStr(row.OriginalFilePath),
	}
}
