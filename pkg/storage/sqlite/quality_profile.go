package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

// CreateQualityProfile stores a new quality profile
func (s *SQLite) CreateQualityProfile(ctx context.Context, profile model.QualityProfile) (int64, error) {
	return s.Queries.CreateQualityProfile(ctx, sqlcdb.CreateQualityProfileParams{
		Name:            profile.Name,
		CutoffQualityID: cfgNullInt64FromInt32(profile.CutoffQualityID),
		UpgradeAllowed:  profile.UpgradeAllowed,
	})
}

// GetQualityProfile gets a quality profile and all associated quality items
// given a quality profile id. Qualities are ordered by minimum size descending.
func (s *SQLite) GetQualityProfile(ctx context.Context, id int64) (storage.QualityProfile, error) {
	rows, err := s.Queries.ListQualityProfileRows(ctx, sqlcdb.ListQualityProfileRowsParams{
		ProfileID: sql.NullInt64{Int64: id, Valid: true},
	})
	if err != nil {
		return storage.QualityProfile{}, fmt.Errorf("failed to get quality profile: %w", err)
	}
	if len(rows) == 0 {
		return storage.QualityProfile{}, storage.ErrNotFound
	}

	return qualityProfileFromRows(rows), nil
}

// ListQualityProfiles lists all quality profiles and their associated quality
// items. Profiles appear in the order of their largest quality minimum size,
// matching the original joined-query ordering.
func (s *SQLite) ListQualityProfiles(ctx context.Context) ([]*storage.QualityProfile, error) {
	rows, err := s.Queries.ListQualityProfileRows(ctx, sqlcdb.ListQualityProfileRowsParams{})
	if err != nil {
		return nil, fmt.Errorf("failed to list quality profiles: %w", err)
	}

	return groupQualityProfileRows(rows), nil
}

// ListQualityProfilesByMediaType lists quality profiles that contain at least
// one quality definition of the given media type ("movie" or "episode").
func (s *SQLite) ListQualityProfilesByMediaType(ctx context.Context, mediaType string) ([]*storage.QualityProfile, error) {
	rows, err := s.Queries.ListQualityProfileRows(ctx, sqlcdb.ListQualityProfileRowsParams{
		MediaType: sql.NullString{String: mediaType, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list quality profiles by media type: %w", err)
	}

	return groupQualityProfileRows(rows), nil
}

// UpdateQualityProfile updates a quality profile
func (s *SQLite) UpdateQualityProfile(ctx context.Context, id int64, profile storage.QualityProfile) error {
	return s.Queries.UpdateQualityProfile(ctx, sqlcdb.UpdateQualityProfileParams{
		Name:            profile.Name,
		CutoffQualityID: cfgNullInt64FromInt32(profile.CutoffQualityID),
		UpgradeAllowed:  profile.UpgradeAllowed,
		ID:              id,
	})
}

// DeleteQualityProfile deletes a quality profile and all of its items in a
// single transaction
func (s *SQLite) DeleteQualityProfile(ctx context.Context, id int64) error {
	log := logger.FromCtx(ctx)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		log.Errorw("failed to begin transaction", "id", id, "error", err)
		return err
	}

	q := s.txQueries(tx)

	if err := q.DeleteQualityProfileItemsByProfileID(ctx, id); err != nil {
		log.Errorw("failed to delete profile items", "id", id, "error", err)
		if rbErr := tx.Rollback(); rbErr != nil {
			log.Errorw("failed to rollback", "id", id, "error", rbErr)
		}
		return err
	}

	if err := q.DeleteQualityProfile(ctx, id); err != nil {
		log.Errorw("failed to delete profile", "id", id, "error", err)
		if rbErr := tx.Rollback(); rbErr != nil {
			log.Errorw("failed to rollback", "id", id, "error", rbErr)
		}
		return err
	}

	log.Debugw("deleted quality profile", "id", id)
	return tx.Commit()
}

// qualityProfileFromRows assembles a single domain quality profile from the
// ordered rows of ListQualityProfileRows.
func qualityProfileFromRows(rows []sqlcdb.ListQualityProfileRowsRow) storage.QualityProfile {
	first := rows[0]
	profile := storage.QualityProfile{
		ID:              int32(first.ProfileID),
		Name:            first.ProfileName,
		CutoffQualityID: cfgInt32Ptr(first.ProfileCutoffQualityID),
		UpgradeAllowed:  first.ProfileUpgradeAllowed,
		Qualities:       make([]storage.QualityDefinition, 0, len(rows)),
	}

	for _, r := range rows {
		profile.Qualities = append(profile.Qualities, qualityDefinitionFromJoinRow(r))
	}

	return profile
}

// groupQualityProfileRows groups joined rows into distinct quality profiles,
// preserving row order so profile ordering matches the query's ordering.
func groupQualityProfileRows(rows []sqlcdb.ListQualityProfileRowsRow) []*storage.QualityProfile {
	profiles := make([]*storage.QualityProfile, 0)
	byID := make(map[int64]*storage.QualityProfile, len(rows))

	for _, r := range rows {
		profile, ok := byID[r.ProfileID]
		if !ok {
			profile = &storage.QualityProfile{
				ID:              int32(r.ProfileID),
				Name:            r.ProfileName,
				CutoffQualityID: cfgInt32Ptr(r.ProfileCutoffQualityID),
				UpgradeAllowed:  r.ProfileUpgradeAllowed,
				Qualities:       make([]storage.QualityDefinition, 0),
			}
			byID[r.ProfileID] = profile
			profiles = append(profiles, profile)
		}

		profile.Qualities = append(profile.Qualities, qualityDefinitionFromJoinRow(r))
	}

	return profiles
}

// qualityDefinitionFromJoinRow extracts the quality definition columns from a
// joined ListQualityProfileRows row.
func qualityDefinitionFromJoinRow(r sqlcdb.ListQualityProfileRowsRow) storage.QualityDefinition {
	return storage.QualityDefinition{
		ID:            int32(r.QualityID),
		Name:          r.QualityName,
		MediaType:     r.MediaType,
		PreferredSize: r.PreferredSize,
		MinSize:       r.MinSize,
		MaxSize:       r.MaxSize,
	}
}
