package sqlite

import (
	"context"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
)

// CreateQualityDefinition store a new quality definition
func (s *SQLite) CreateQualityDefinition(ctx context.Context, definition model.QualityDefinition) (int64, error) {
	stmt := table.QualityDefinition.INSERT(table.QualityDefinition.AllColumns.Except(table.QualityDefinition.ID)).MODEL(definition).RETURNING(table.QualityDefinition.ID)
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

// GetQualityDefinition gets a quality definition
func (s *SQLite) GetQualityDefinition(ctx context.Context, id int64) (model.QualityDefinition, error) {
	stmt := table.QualityDefinition.SELECT(table.QualityDefinition.AllColumns).FROM(table.QualityDefinition).WHERE(table.QualityDefinition.ID.EQ(sqlite.Int64(id))).ORDER_BY(table.QualityDefinition.ID.ASC())
	var result model.QualityDefinition
	err := stmt.QueryContext(ctx, s.db, &result)
	return result, err
}

// ListQualityDefinitions lists all quality definitions
func (s *SQLite) ListQualityDefinitions(ctx context.Context) ([]*model.QualityDefinition, error) {
	definitions := make([]*model.QualityDefinition, 0)
	stmt := table.QualityDefinition.SELECT(table.QualityDefinition.AllColumns).FROM(table.QualityDefinition).ORDER_BY(table.QualityDefinition.ID.ASC())
	err := stmt.QueryContext(ctx, s.db, &definitions)
	return definitions, err
}

// UpdateQualityDefinition updates a quality definition
func (s *SQLite) UpdateQualityDefinition(ctx context.Context, id int64, definition model.QualityDefinition) error {
	stmt := table.QualityDefinition.UPDATE(
		table.QualityDefinition.Name,
		table.QualityDefinition.PreferredSize,
		table.QualityDefinition.MinSize,
		table.QualityDefinition.MaxSize,
		table.QualityDefinition.MediaType,
	).MODEL(definition).WHERE(table.QualityDefinition.ID.EQ(sqlite.Int64(id)))
	_, err := stmt.ExecContext(ctx, s.db)
	return err
}

func (s *SQLite) DeleteQualityDefinition(ctx context.Context, id int64) error {
	stmt := table.QualityDefinition.DELETE().WHERE(table.QualityDefinition.ID.EQ(sqlite.Int64(id))).RETURNING(table.QualityDefinition.AllColumns)
	_, err := s.handleDelete(ctx, stmt)
	return err
}
