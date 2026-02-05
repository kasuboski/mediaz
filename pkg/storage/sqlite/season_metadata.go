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

// CreateSeasonMetadata creates the given seasonMeta
func (s *SQLite) CreateSeasonMetadata(ctx context.Context, seasonMeta model.SeasonMetadata) (int64, error) {
	// don't insert a zeroed ID
	insertColumns := table.SeasonMetadata.MutableColumns
	if seasonMeta.ID != 0 {
		insertColumns = table.SeasonMetadata.AllColumns
	}

	stmt := table.SeasonMetadata.
		INSERT(insertColumns).
		MODEL(seasonMeta).
		ON_CONFLICT(table.SeasonMetadata.TmdbID).
		DO_NOTHING()

	_, err := s.handleInsert(ctx, stmt)
	if err != nil {
		return 0, err
	}

	var existing model.SeasonMetadata
	getStmt := table.SeasonMetadata.
		SELECT(table.SeasonMetadata.ID).
		FROM(table.SeasonMetadata).
		WHERE(table.SeasonMetadata.TmdbID.EQ(sqlite.Int32(seasonMeta.TmdbID)))

	err = getStmt.QueryContext(ctx, s.db, &existing)
	if err != nil {
		return 0, fmt.Errorf("failed to get season metadata ID after upsert: %w", err)
	}

	return int64(existing.ID), nil
}

// DeleteSeasonMetadata deletes a season metadata by id
func (s *SQLite) DeleteSeasonMetadata(ctx context.Context, id int64) error {
	stmt := table.SeasonMetadata.
		DELETE().
		WHERE(table.SeasonMetadata.ID.EQ(sqlite.Int64(id)))

	_, err := s.handleDelete(ctx, stmt)
	if err != nil {
		return fmt.Errorf("failed to delete season metadata: %w", err)
	}

	return nil
}

// ListSeasonMetadata lists all season metadata
func (s *SQLite) ListSeasonMetadata(ctx context.Context, where ...sqlite.BoolExpression) ([]*model.SeasonMetadata, error) {
	stmt := table.SeasonMetadata.
		SELECT(table.SeasonMetadata.AllColumns).
		FROM(table.SeasonMetadata)

	for _, w := range where {
		stmt = stmt.WHERE(w)
	}

	var seasonMetadata []*model.SeasonMetadata
	err := stmt.QueryContext(ctx, s.db, &seasonMetadata)
	if err != nil {
		return nil, fmt.Errorf("failed to list season metadata: %w", err)
	}

	return seasonMetadata, nil
}

// GetSeasonMetadata get a season metadata for the given where
func (s *SQLite) GetSeasonMetadata(ctx context.Context, where sqlite.BoolExpression) (*model.SeasonMetadata, error) {
	stmt := table.SeasonMetadata.
		SELECT(table.SeasonMetadata.AllColumns).
		FROM(table.SeasonMetadata).
		WHERE(where)

	var seasonMetadata model.SeasonMetadata
	err := stmt.QueryContext(ctx, s.db, &seasonMetadata)
	if err != nil {
		if errors.Is(err, qrm.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get season metadata: %w", err)
	}

	return &seasonMetadata, nil
}
