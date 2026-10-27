package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

// GetMovieFilesByMovieName gets all movie files given a movie name.
// It assumes the movie name is the prefix of the relative path for a movie file.
func (s *SQLite) GetMovieFilesByMovieName(ctx context.Context, name string) ([]sqlcdb.MovieFile, error) {
	rows, err := s.Queries.GetMovieFilesByMovieName(ctx, sql.NullString{String: name, Valid: true})
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		return nil, storage.ErrNotFound
	}

	return rows, nil
}

// CreateMovieFile stores a movie file. The id and date_added columns are
// omitted from the insert so the autoincrement key and the database default
// timestamp (UTC) are used.
func (s *SQLite) CreateMovieFile(ctx context.Context, file sqlcdb.MovieFile) (int64, error) {
	return s.Queries.CreateMovieFile(ctx, sqlcdb.CreateMovieFileParams{
		Quality:          file.Quality,
		Size:             file.Size,
		SceneName:        file.SceneName,
		MediaInfo:        file.MediaInfo,
		ReleaseGroup:     file.ReleaseGroup,
		RelativePath:     file.RelativePath,
		Edition:          file.Edition,
		Languages:        file.Languages,
		IndexerFlags:     file.IndexerFlags,
		OriginalFilePath: file.OriginalFilePath,
	})
}

// DeleteMovieFile removes a movie file by id
func (s *SQLite) DeleteMovieFile(ctx context.Context, id int64) error {
	return s.Queries.DeleteMovieFile(ctx, id)
}

// ListMovieFiles lists all movie files
func (s *SQLite) ListMovieFiles(ctx context.Context) ([]sqlcdb.MovieFile, error) {
	rows, err := s.Queries.ListMovieFiles(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list movie files: %w", err)
	}

	return rows, nil
}
