package sqlite

import (
	"context"
	"fmt"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
)

// GetMovieFilesByMovieName gets all movie files given a movie name
// It assumes the movie name is the prefix of the relative path for a movie file
func (s *SQLite) GetMovieFilesByMovieName(ctx context.Context, name string) ([]*model.MovieFile, error) {
	stmt := table.MovieFile.
		SELECT(table.MovieFile.AllColumns).
		FROM(table.MovieFile.INNER_JOIN(table.Movie,
			table.MovieFile.RelativePath.LIKE(table.Movie.Path.CONCAT(sqlite.String("%"))),
		),
		).
		WHERE(table.Movie.Path.EQ(sqlite.String(name)))

	var result []*model.MovieFile
	err := stmt.QueryContext(ctx, s.db, &result)
	if err != nil {
		return result, err
	}

	if len(result) == 0 {
		return nil, storage.ErrNotFound
	}

	return result, err
}

// CreateMovieFile stores a movie file
func (s *SQLite) CreateMovieFile(ctx context.Context, file model.MovieFile) (int64, error) {
	// Exclude DateAdded so that the default is used
	stmt := table.MovieFile.INSERT(table.MovieFile.MutableColumns.Except(table.MovieFile.DateAdded).Except(table.MovieFile.ID)).RETURNING(table.MovieFile.ID).MODEL(file)
	result, err := s.handleInsert(ctx, stmt)
	if err != nil {
		return 0, err
	}

	inserted, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return inserted, nil
}

// DeleteMovieFile removes a movie file by id
func (s *SQLite) DeleteMovieFile(ctx context.Context, id int64) error {
	stmt := table.MovieFile.DELETE().WHERE(table.MovieFile.ID.EQ(sqlite.Int64(id))).RETURNING(table.MovieFile.ID)
	_, err := s.handleDelete(ctx, stmt)
	if err != nil {
		return err
	}
	return nil
}

// ListMovieFiles lists all movie files
func (s *SQLite) ListMovieFiles(ctx context.Context) ([]*model.MovieFile, error) {
	var movieFiles []*model.MovieFile
	stmt := table.MovieFile.SELECT(table.MovieFile.AllColumns).FROM(table.MovieFile).ORDER_BY(table.MovieFile.ID.ASC())
	err := stmt.QueryContext(ctx, s.db, &movieFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to list movie files: %w", err)
	}

	return movieFiles, nil
}
