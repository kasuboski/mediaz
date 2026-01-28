package sqlite

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-jet/jet/v2/qrm"
	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
)

// CreateMovieMetadata creates the given movieMeta
func (s *SQLite) CreateMovieMetadata(ctx context.Context, movieMeta model.MovieMetadata) (int64, error) {
	stmt := table.MovieMetadata.INSERT(table.MovieMetadata.MutableColumns).MODEL(movieMeta).ON_CONFLICT(table.MovieMetadata.ID).DO_NOTHING()
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

// UpdateMovieMetadata updates an existing movie metadata record
func (s *SQLite) UpdateMovieMetadata(ctx context.Context, metadata model.MovieMetadata) error {
	stmt := table.MovieMetadata.
		UPDATE(table.MovieMetadata.AllColumns.Except(table.MovieMetadata.ID, table.MovieMetadata.TmdbID, table.MovieMetadata.LastInfoSync)).
		MODEL(metadata).
		WHERE(table.MovieMetadata.ID.EQ(sqlite.Int32(metadata.ID)))

	_, err := s.handleStatement(ctx, stmt)
	if err != nil {
		return fmt.Errorf("failed to update movie metadata: %w", err)
	}

	return nil
}

// DeleteMovieMetadata deletes a movie metadata by id
func (s *SQLite) DeleteMovieMetadata(ctx context.Context, id int64) error {
	stmt := table.MovieMetadata.DELETE().WHERE(table.MovieMetadata.ID.EQ(sqlite.Int64(id))).RETURNING(table.MovieMetadata.ID)
	_, err := s.handleDelete(ctx, stmt)
	if err != nil {
		return err
	}
	return nil
}

// ListMovieMetadata lists all movie metadata
func (s *SQLite) ListMovieMetadata(ctx context.Context) ([]*model.MovieMetadata, error) {
	movies := make([]*model.MovieMetadata, 0)
	stmt := table.Movie.SELECT(table.MovieMetadata.AllColumns).FROM(table.MovieMetadata).ORDER_BY(table.MovieMetadata.LastInfoSync.ASC())
	err := stmt.QueryContext(ctx, s.db, &movies)
	if err != nil {
		return nil, fmt.Errorf("failed to list movies: %w", err)
	}

	return movies, nil
}

// GetMovieMetadata get a movie metadata for the given where
func (s *SQLite) GetMovieMetadata(ctx context.Context, where sqlite.BoolExpression) (*model.MovieMetadata, error) {
	meta := &model.MovieMetadata{}
	stmt := table.Movie.SELECT(table.MovieMetadata.AllColumns).FROM(table.MovieMetadata).WHERE(where).LIMIT(1)
	err := stmt.QueryContext(ctx, s.db, meta)
	if err != nil {
		if errors.Is(err, qrm.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("failed to list movies: %w", err)
	}

	return meta, nil
}
