package sqlite

import (
	"context"
	"fmt"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
)

// CreateIndexer stores a new indexer in the database
func (s *SQLite) CreateIndexer(ctx context.Context, indexer model.Indexer) (int64, error) {
	stmt := table.Indexer.INSERT(table.Indexer.AllColumns.Except(table.Indexer.ID)).MODEL(indexer).ON_CONFLICT(table.Indexer.IndexerSourceID, table.Indexer.Name).DO_NOTHING().RETURNING(table.Indexer.AllColumns)
	result, err := s.handleInsert(ctx, stmt)
	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

func (s *SQLite) UpdateIndexer(ctx context.Context, id int64, indexer model.Indexer) error {
	if indexer.APIKey != nil {
		stmt := table.Indexer.UPDATE(
			table.Indexer.Name,
			table.Indexer.Priority,
			table.Indexer.URI,
			table.Indexer.APIKey,
		).MODEL(indexer).WHERE(table.Indexer.ID.EQ(sqlite.Int64(id)))
		_, err := stmt.ExecContext(ctx, s.db)
		return err
	}

	stmt := table.Indexer.UPDATE(
		table.Indexer.Name,
		table.Indexer.Priority,
		table.Indexer.URI,
	).MODEL(indexer).WHERE(table.Indexer.ID.EQ(sqlite.Int64(id)))
	_, err := stmt.ExecContext(ctx, s.db)
	return err
}

// DeleteIndexer deletes a stored indexer given the indexer ID
func (s *SQLite) DeleteIndexer(ctx context.Context, id int64) error {
	stmt := table.Indexer.DELETE().WHERE(table.Indexer.ID.EQ(sqlite.Int64(id))).RETURNING(table.Indexer.AllColumns)
	_, err := s.handleDelete(ctx, stmt)
	return err
}

// ListIndexers lists the stored indexers
func (s *SQLite) ListIndexers(ctx context.Context, where ...sqlite.BoolExpression) ([]*model.Indexer, error) {
	indexers := make([]*model.Indexer, 0)

	stmt := table.Indexer.SELECT(table.Indexer.AllColumns).FROM(table.Indexer)

	for _, w := range where {
		stmt = stmt.WHERE(w)
	}

	stmt = stmt.ORDER_BY(table.Indexer.Priority.DESC())

	err := stmt.QueryContext(ctx, s.db, &indexers)
	if err != nil {
		return nil, fmt.Errorf("failed to list indexers: %w", err)
	}

	return indexers, nil
}
