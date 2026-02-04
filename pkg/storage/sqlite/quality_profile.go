package sqlite

import (
	"context"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
)

func (s *SQLite) CreateQualityProfile(ctx context.Context, profile model.QualityProfile) (int64, error) {
	stmt := table.QualityProfile.INSERT(table.QualityProfile.AllColumns.Except(table.QualityProfile.ID)).MODEL(profile).RETURNING(table.QualityProfile.ID)
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

// GetQualityProfile gets a quality profile and all associated quality items given a quality profile id
func (s *SQLite) GetQualityProfile(ctx context.Context, id int64) (storage.QualityProfile, error) {
	stmt := sqlite.
		SELECT(
			table.QualityProfile.AllColumns,
			table.QualityProfileItem.AllColumns,
			table.QualityDefinition.AllColumns,
		).
		FROM(
			table.QualityProfile.INNER_JOIN(
				table.QualityProfileItem, table.QualityProfileItem.ProfileID.EQ(table.QualityProfile.ID)).INNER_JOIN(
				table.QualityDefinition, table.QualityProfileItem.QualityID.EQ(table.QualityDefinition.ID)),
		).
		WHERE(table.QualityProfile.ID.EQ(sqlite.Int(id))).
		ORDER_BY(table.QualityDefinition.MinSize.DESC())

	var result storage.QualityProfile
	err := stmt.QueryContext(ctx, s.db, &result)
	return result, err
}

// ListQualityProfiles lists all quality profiles and their associated quality items
func (s *SQLite) ListQualityProfiles(ctx context.Context, where ...sqlite.BoolExpression) ([]*storage.QualityProfile, error) {
	stmt := sqlite.
		SELECT(
			table.QualityProfile.AllColumns,
			table.QualityDefinition.AllColumns,
		).
		FROM(
			table.QualityProfile.INNER_JOIN(
				table.QualityProfileItem, table.QualityProfileItem.ProfileID.EQ(table.QualityProfile.ID)).INNER_JOIN(
				table.QualityDefinition, table.QualityProfileItem.QualityID.EQ(table.QualityDefinition.ID)),
		)

	var conds []sqlite.BoolExpression
	for _, w := range where {
		conds = append(conds, w)
	}
	if len(conds) > 0 {
		stmt = stmt.WHERE(sqlite.AND(conds...))
	}

	stmt = stmt.ORDER_BY(table.QualityDefinition.MinSize.DESC())

	result := make([]*storage.QualityProfile, 0)
	err := stmt.QueryContext(ctx, s.db, &result)
	return result, err
}

// UpdateQualityProfile updates a quality profile
func (s *SQLite) UpdateQualityProfile(ctx context.Context, id int64, profile model.QualityProfile) error {
	stmt := table.QualityProfile.UPDATE(
		table.QualityProfile.Name,
		table.QualityProfile.CutoffQualityID,
		table.QualityProfile.UpgradeAllowed,
	).MODEL(profile).WHERE(table.QualityProfile.ID.EQ(sqlite.Int64(id)))
	_, err := stmt.ExecContext(ctx, s.db)
	return err
}

// DeleteQualityProfile delete a quality profile
func (s *SQLite) DeleteQualityProfile(ctx context.Context, id int64) error {
	stmt := table.QualityProfile.DELETE().WHERE(table.QualityProfile.ID.EQ(sqlite.Int64(id))).RETURNING(table.QualityProfile.AllColumns)
	_, err := s.handleDelete(ctx, stmt)
	return err
}
