package sqlite

import (
	"context"
	"database/sql"

	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

func (s *SQLite) CreateQualityProfile(ctx context.Context, profile sqlcdb.QualityProfile) (int64, error) {
	return s.Queries.CreateQualityProfile(ctx, sqlcdb.CreateQualityProfileParams{Name: profile.Name, CutoffQualityID: profile.CutoffQualityID, UpgradeAllowed: profile.UpgradeAllowed})
}

func (s *SQLite) GetQualityProfile(ctx context.Context, id int64) (sqlcdb.QualityProfile, error) {
	row, err := s.Queries.GetQualityProfile(ctx, id)
	if err != nil {
		return sqlcdb.QualityProfile{}, mapQueryErr(err, "failed to get quality profile")
	}
	return row, nil
}

func (s *SQLite) ListQualityProfiles(ctx context.Context) ([]sqlcdb.QualityProfile, error) {
	return s.Queries.ListQualityProfiles(ctx)
}

func (s *SQLite) ListQualityProfilesByMediaType(ctx context.Context, mediaType string) ([]sqlcdb.QualityProfile, error) {
	return s.Queries.ListQualityProfilesByMediaType(ctx, mediaType)
}

func (s *SQLite) ListQualityProfileRows(ctx context.Context, profileID int64) ([]sqlcdb.ListQualityProfileRowsRow, error) {
	return s.Queries.ListQualityProfileRows(ctx, sqlcdb.ListQualityProfileRowsParams{ProfileID: sql.NullInt64{Int64: profileID, Valid: profileID != 0}})
}

func (s *SQLite) UpdateQualityProfile(ctx context.Context, id int64, profile sqlcdb.QualityProfile) error {
	_, err := s.GetQualityProfile(ctx, id)
	if err != nil {
		return err
	}
	return s.Queries.UpdateQualityProfile(ctx, sqlcdb.UpdateQualityProfileParams{Name: profile.Name, CutoffQualityID: profile.CutoffQualityID, UpgradeAllowed: profile.UpgradeAllowed, ID: id})
}

func (s *SQLite) DeleteQualityProfile(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	q := s.txQueries(tx)
	if _, err := q.GetQualityProfile(ctx, id); err != nil {
		return mapQueryErr(err, "failed to get quality profile")
	}
	if err := q.DeleteQualityProfileItemsByProfileID(ctx, id); err != nil {
		return err
	}
	if err := q.DeleteQualityProfile(ctx, id); err != nil {
		return err
	}
	return tx.Commit()
}
