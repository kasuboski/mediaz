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

// CreateEpisodeMetadata creates the given episodeMeta
func (s *SQLite) CreateEpisodeMetadata(ctx context.Context, episodeMeta model.EpisodeMetadata) (int64, error) {
	// don't insert a zeroed ID
	insertColumns := table.EpisodeMetadata.MutableColumns
	if episodeMeta.ID != 0 {
		insertColumns = table.EpisodeMetadata.AllColumns
	}

	stmt := table.EpisodeMetadata.
		INSERT(insertColumns).
		MODEL(episodeMeta).
		ON_CONFLICT(table.EpisodeMetadata.TmdbID).
		DO_UPDATE(sqlite.SET(
			table.EpisodeMetadata.Title.SET(sqlite.String(episodeMeta.Title)),
		))

	_, err := s.handleInsert(ctx, stmt)
	if err != nil {
		return 0, err
	}

	var existing model.EpisodeMetadata
	getStmt := table.EpisodeMetadata.
		SELECT(table.EpisodeMetadata.ID).
		FROM(table.EpisodeMetadata).
		WHERE(table.EpisodeMetadata.TmdbID.EQ(sqlite.Int32(episodeMeta.TmdbID)))

	err = getStmt.QueryContext(ctx, s.db, &existing)
	if err != nil {
		return 0, fmt.Errorf("failed to get episode metadata ID after upsert: %w", err)
	}

	return int64(existing.ID), nil
}

// DeleteEpisodeMetadata deletes an episode metadata by id
func (s *SQLite) DeleteEpisodeMetadata(ctx context.Context, id int64) error {
	stmt := table.EpisodeMetadata.
		DELETE().
		WHERE(table.EpisodeMetadata.ID.EQ(sqlite.Int64(id)))

	_, err := s.handleDelete(ctx, stmt)
	if err != nil {
		return fmt.Errorf("failed to delete episode metadata: %w", err)
	}

	return nil
}

// ListEpisodeMetadata lists all episode metadata
func (s *SQLite) ListEpisodeMetadata(ctx context.Context, where ...sqlite.BoolExpression) ([]*model.EpisodeMetadata, error) {
	stmt := table.EpisodeMetadata.
		SELECT(table.EpisodeMetadata.AllColumns).
		FROM(table.EpisodeMetadata)

	for _, w := range where {
		stmt = stmt.WHERE(w)
	}

	var episodeMetadata []*model.EpisodeMetadata
	err := stmt.QueryContext(ctx, s.db, &episodeMetadata)
	if err != nil {
		return nil, fmt.Errorf("failed to list episode metadata: %w", err)
	}

	return episodeMetadata, nil
}

// GetEpisodeMetadata get an episode metadata for the given where
func (s *SQLite) GetEpisodeMetadata(ctx context.Context, where sqlite.BoolExpression) (*model.EpisodeMetadata, error) {
	stmt := table.EpisodeMetadata.
		SELECT(table.EpisodeMetadata.AllColumns).
		FROM(table.EpisodeMetadata).
		WHERE(where)

	var episodeMetadata model.EpisodeMetadata
	err := stmt.QueryContext(ctx, s.db, &episodeMetadata)
	if err != nil {
		if errors.Is(err, qrm.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get episode metadata: %w", err)
	}

	return &episodeMetadata, nil
}
