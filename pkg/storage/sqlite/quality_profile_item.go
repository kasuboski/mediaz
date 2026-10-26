package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

// CreateQualityProfileItem stores a quality profile item
func (s *SQLite) CreateQualityProfileItem(ctx context.Context, item model.QualityProfileItem) (int64, error) {
	return s.Queries.CreateQualityProfileItem(ctx, sqlcdb.CreateQualityProfileItemParams{
		ProfileID: int64(item.ProfileID),
		QualityID: int64(item.QualityID),
	})
}

// CreateQualityProfileItems stores multiple quality profile items atomically
func (s *SQLite) CreateQualityProfileItems(ctx context.Context, items []model.QualityProfileItem) error {
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
			ProfileID: int64(item.ProfileID),
			QualityID: int64(item.QualityID),
		}); err != nil {
			_ = tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}

// GetQualityProfileItem gets a quality item that belongs to a profile
func (s *SQLite) GetQualityProfileItem(ctx context.Context, id int64) (model.QualityProfileItem, error) {
	r, err := s.Queries.GetQualityProfileItem(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.QualityProfileItem{}, storage.ErrNotFound
		}
		return model.QualityProfileItem{}, err
	}

	itemID := int32(r.ID)
	return model.QualityProfileItem{
		ID:        &itemID,
		ProfileID: int32(r.ProfileID),
		QualityID: int32(r.QualityID),
	}, nil
}

// ListQualityProfileItems lists all items in a quality profile ordered by id
func (s *SQLite) ListQualityProfileItems(ctx context.Context) ([]*model.QualityProfileItem, error) {
	rows, err := s.Queries.ListQualityProfileItems(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]*model.QualityProfileItem, 0, len(rows))
	for _, r := range rows {
		itemID := int32(r.ID)
		items = append(items, &model.QualityProfileItem{
			ID:        &itemID,
			ProfileID: int32(r.ProfileID),
			QualityID: int32(r.QualityID),
		})
	}

	return items, nil
}

// DeleteQualityProfileItem deletes a quality profile item
func (s *SQLite) DeleteQualityProfileItem(ctx context.Context, id int64) error {
	return s.Queries.DeleteQualityProfileItem(ctx, id)
}

// DeleteQualityProfileItemsByProfileID deletes all items for a given profile
func (s *SQLite) DeleteQualityProfileItemsByProfileID(ctx context.Context, profileID int64) error {
	return s.Queries.DeleteQualityProfileItemsByProfileID(ctx, profileID)
}
