package sqlite

import (
	"context"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

// GetEpisodeFileByID gets all episode files matching an id. The id column is
// the primary key so this returns at most one row; an empty result maps to
// storage.ErrNotFound.
func (s *SQLite) GetEpisodeFileByID(ctx context.Context, id int64) ([]sqlcdb.EpisodeFile, error) {
	rows, err := s.Queries.GetEpisodeFileByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		return nil, storage.ErrNotFound
	}

	return rows, nil
}

// GetEpisodeFile gets an episode file by id
func (s *SQLite) GetEpisodeFile(ctx context.Context, id int64) (sqlcdb.EpisodeFile, error) {
	row, err := s.Queries.GetEpisodeFile(ctx, id)
	if err != nil {
		return sqlcdb.EpisodeFile{}, mapQueryErr(err, "failed to get episode file")
	}
	return row, nil
}

// UpdateEpisodeFile updates an episode file
func (s *SQLite) UpdateEpisodeFile(ctx context.Context, id int64, file sqlcdb.EpisodeFile) error {
	return s.Queries.UpdateEpisodeFile(ctx, sqlcdb.UpdateEpisodeFileParams{
		Quality:          file.Quality,
		Size:             file.Size,
		Added:            file.Added,
		RelativePath:     file.RelativePath,
		OriginalFilePath: file.OriginalFilePath,
		ID:               id,
	})
}

// CreateEpisodeFile stores an episode file. When file.ID is zero the added
// column default (CURRENT_TIMESTAMP, UTC) applies; an explicit ID upserts by
// id (a conflicting insert updates its mutable fields).
func (s *SQLite) CreateEpisodeFile(ctx context.Context, file sqlcdb.EpisodeFile) (int64, error) {
	if file.ID != 0 {
		return s.Queries.CreateEpisodeFileWithID(ctx, sqlcdb.CreateEpisodeFileWithIDParams{
			ID:               file.ID,
			Quality:          file.Quality,
			Size:             file.Size,
			Added:            file.Added,
			RelativePath:     file.RelativePath,
			OriginalFilePath: file.OriginalFilePath,
		})
	}

	return s.Queries.CreateEpisodeFile(ctx, sqlcdb.CreateEpisodeFileParams{
		Quality:          file.Quality,
		Size:             file.Size,
		RelativePath:     file.RelativePath,
		OriginalFilePath: file.OriginalFilePath,
	})
}

// DeleteEpisodeFile removes an episode file by id
func (s *SQLite) DeleteEpisodeFile(ctx context.Context, id int64) error {
	return s.Queries.DeleteEpisodeFile(ctx, id)
}

// ListEpisodeFiles lists all episode files
func (s *SQLite) ListEpisodeFiles(ctx context.Context) ([]sqlcdb.EpisodeFile, error) {
	rows, err := s.Queries.ListEpisodeFiles(ctx)
	if err != nil {
		return nil, err
	}
	return rows, nil
}
