package sqlite

import (
	"context"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
)

func (s *SQLite) CreateQualityProfileItem(ctx context.Context, item model.QualityProfileItem) (int64, error) {
	stmt := table.QualityProfileItem.INSERT(table.QualityProfileItem.AllColumns.Except(table.QualityProfileItem.ID)).RETURNING(table.QualityProfileItem.ID).MODEL(item)
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

func (s *SQLite) CreateQualityProfileItems(ctx context.Context, items []model.QualityProfileItem) error {
	if len(items) == 0 {
		return nil
	}
	stmt := table.QualityProfileItem.INSERT(table.QualityProfileItem.AllColumns.Except(table.QualityProfileItem.ID)).MODELS(items)
	_, err := stmt.ExecContext(ctx, s.db)
	return err
}

// GetQualityProfileItem gets a quality item that belongs to a profile
func (s *SQLite) GetQualityProfileItem(ctx context.Context, id int64) (model.QualityProfileItem, error) {
	stmt := table.QualityProfileItem.SELECT(table.QualityProfileItem.AllColumns).FROM(table.QualityProfileItem).WHERE(table.QualityProfileItem.ID.EQ(sqlite.Int64(id)))
	var result model.QualityProfileItem
	err := stmt.QueryContext(ctx, s.db, &result)
	return result, err
}

// ListQualityProfileItems lists all items in a quality profile
func (s *SQLite) ListQualityProfileItems(ctx context.Context) ([]*model.QualityProfileItem, error) {
	items := make([]*model.QualityProfileItem, 0)
	stmt := table.QualityProfileItem.SELECT(table.QualityProfileItem.AllColumns).FROM(table.QualityProfileItem).ORDER_BY(table.QualityProfileItem.ID.ASC())
	err := stmt.QueryContext(ctx, s.db, &items)
	return items, err
}

// DeleteQualityDefinition deletes a quality
func (s *SQLite) DeleteQualityProfileItem(ctx context.Context, id int64) error {
	stmt := table.QualityProfileItem.DELETE().WHERE(table.QualityProfileItem.ID.EQ(sqlite.Int64(id))).RETURNING(table.QualityProfileItem.AllColumns)
	_, err := s.handleDelete(ctx, stmt)
	return err
}

// DeleteQualityProfileItemsByProfileID deletes all items for a given profile
func (s *SQLite) DeleteQualityProfileItemsByProfileID(ctx context.Context, profileID int64) error {
	stmt := table.QualityProfileItem.DELETE().WHERE(table.QualityProfileItem.ProfileID.EQ(sqlite.Int64(profileID)))
	_, err := s.handleDelete(ctx, stmt)
	return err
}
