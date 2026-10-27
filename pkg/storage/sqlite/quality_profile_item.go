package sqlite

import (
	"context"

	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

// CreateQualityProfileItem stores a quality profile item
func (s *SQLite) CreateQualityProfileItem(ctx context.Context, item sqlcdb.QualityProfileItem) (int64, error) {
	return s.Queries.CreateQualityProfileItem(ctx, sqlcdb.CreateQualityProfileItemParams{
		ProfileID: item.ProfileID,
		QualityID: item.QualityID,
	})
}

// CreateQualityProfileItems stores multiple quality profile items atomically
func (s *SQLite) CreateQualityProfileItems(ctx context.Context, items []sqlcdb.QualityProfileItem) error {
	if len(items) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	q := s.txQueries(tx)
	for _, item := range items {
		if _, err := q.CreateQualityProfileItem(ctx, sqlcdb.CreateQualityProfileItemParams{
			ProfileID: item.ProfileID,
			QualityID: item.QualityID,
		}); err != nil {
			_ = tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}

// GetQualityProfileItem gets a quality item that belongs to a profile
func (s *SQLite) GetQualityProfileItem(ctx context.Context, id int64) (sqlcdb.QualityProfileItem, error) {
	item, err := s.Queries.GetQualityProfileItem(ctx, id)
	if err != nil {
		return sqlcdb.QualityProfileItem{}, mapQueryErr(err, "failed to get quality profile item")
	}
	return item, nil
}

// ListQualityProfileItems lists all items in a quality profile ordered by id
func (s *SQLite) ListQualityProfileItems(ctx context.Context) ([]sqlcdb.QualityProfileItem, error) {
	return s.Queries.ListQualityProfileItems(ctx)
}

// DeleteQualityProfileItem deletes a quality profile item
func (s *SQLite) DeleteQualityProfileItem(ctx context.Context, id int64) error {
	return s.Queries.DeleteQualityProfileItem(ctx, id)
}

// DeleteQualityProfileItemsByProfileID deletes all items for a given profile
func (s *SQLite) DeleteQualityProfileItemsByProfileID(ctx context.Context, profileID int64) error {
	return s.Queries.DeleteQualityProfileItemsByProfileID(ctx, profileID)
}
