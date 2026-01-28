package sqlite

import (
	"context"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
)

// CreateIndexerSource stores a new indexer source in the database
func (s *SQLite) CreateIndexerSource(ctx context.Context, source model.IndexerSource) (int64, error) {
	stmt := table.IndexerSource.INSERT(table.IndexerSource.AllColumns.Except(table.IndexerSource.ID)).MODEL(source).RETURNING(table.IndexerSource.ID)
	result, err := s.handleInsert(ctx, stmt)
	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

// GetIndexerSource gets a stored indexer source given an id
func (s *SQLite) GetIndexerSource(ctx context.Context, id int64) (model.IndexerSource, error) {
	stmt := table.IndexerSource.SELECT(table.IndexerSource.AllColumns).FROM(table.IndexerSource).WHERE(table.IndexerSource.ID.EQ(sqlite.Int64(id)))
	var result model.IndexerSource
	err := stmt.QueryContext(ctx, s.db, &result)
	return result, err
}

// ListIndexerSources lists all stored indexer sources
func (s *SQLite) ListIndexerSources(ctx context.Context, where ...sqlite.BoolExpression) ([]*model.IndexerSource, error) {
	items := make([]*model.IndexerSource, 0)

	stmt := table.IndexerSource.SELECT(table.IndexerSource.AllColumns).FROM(table.IndexerSource)

	var conds []sqlite.BoolExpression
	for _, w := range where {
		conds = append(conds, w)
	}
	if len(conds) > 0 {
		stmt = stmt.WHERE(sqlite.AND(conds...))
	}

	stmt = stmt.ORDER_BY(table.IndexerSource.Name.ASC())

	err := stmt.QueryContext(ctx, s.db, &items)
	return items, err
}

// UpdateIndexerSource updates an existing indexer source
func (s *SQLite) UpdateIndexerSource(ctx context.Context, id int64, source model.IndexerSource) error {
	if source.APIKey != nil {
		stmt := table.IndexerSource.UPDATE(
			table.IndexerSource.Name,
			table.IndexerSource.Implementation,
			table.IndexerSource.Scheme,
			table.IndexerSource.Host,
			table.IndexerSource.Port,
			table.IndexerSource.APIKey,
			table.IndexerSource.Enabled,
		).MODEL(source).WHERE(table.IndexerSource.ID.EQ(sqlite.Int64(id)))
		_, err := stmt.ExecContext(ctx, s.db)
		return err
	}

	stmt := table.IndexerSource.UPDATE(
		table.IndexerSource.Name,
		table.IndexerSource.Implementation,
		table.IndexerSource.Scheme,
		table.IndexerSource.Host,
		table.IndexerSource.Port,
		table.IndexerSource.Enabled,
	).MODEL(source).WHERE(table.IndexerSource.ID.EQ(sqlite.Int64(id)))
	_, err := stmt.ExecContext(ctx, s.db)
	return err
}

// DeleteIndexerSource deletes an indexer source given an id
func (s *SQLite) DeleteIndexerSource(ctx context.Context, id int64) error {
	stmt := table.IndexerSource.DELETE().WHERE(table.IndexerSource.ID.EQ(sqlite.Int64(id)))
	_, err := s.handleDelete(ctx, stmt)
	return err
}
