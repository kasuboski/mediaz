package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

// tvSeasonFromRow converts a single-row season result into the domain shape.
// The transition join is a LEFT JOIN, so transition columns may be NULL when
// a season has no transitions yet.
func tvSeasonFromRow(r sqlcdb.GetSeasonRow) *storage.Season {
	return &storage.Season{
		Season: model.Season{
			ID:               int32(r.ID),
			SeriesID:         int32(r.SeriesID),
			SeasonNumber:     int32(r.SeasonNumber),
			SeasonMetadataID: tvInt32Ptr(r.SeasonMetadataID),
			Monitored:        int32(r.Monitored),
		},
		DownloadID:       tvStringFromNull(r.DownloadID),
		DownloadClientID: tvInt32(r.DownloadClientID),
		State:            storage.SeasonState(tvStringFromNull(r.ToState)),
	}
}

// tvSeasonFromListRow converts a list-season result into the domain shape.
// The list query uses an INNER JOIN, so the state is always present.
func tvSeasonFromListRow(r sqlcdb.ListSeasonsRow) *storage.Season {
	return &storage.Season{
		Season: model.Season{
			ID:               int32(r.ID),
			SeriesID:         int32(r.SeriesID),
			SeasonNumber:     int32(r.SeasonNumber),
			SeasonMetadataID: tvInt32Ptr(r.SeasonMetadataID),
			Monitored:        int32(r.Monitored),
		},
		DownloadID:       tvStringFromNull(r.DownloadID),
		DownloadClientID: tvInt32(r.DownloadClientID),
		State:            storage.SeasonState(r.ToState),
	}
}

// LinkSeasonMetadata links a season with its metadata
func (s *SQLite) LinkSeasonMetadata(ctx context.Context, seasonID int64, metadataID int32) error {
	return s.Queries.LinkSeasonMetadata(ctx, sqlcdb.LinkSeasonMetadataParams{
		SeasonMetadataID: sql.NullInt64{Int64: int64(metadataID), Valid: true},
		ID:               seasonID,
	})
}

// CreateSeason stores a season in the database
func (s *SQLite) CreateSeason(ctx context.Context, season storage.Season, initialState storage.SeasonState) (int64, error) {
	if season.State == "" {
		season.State = storage.SeasonStateNew
	}

	err := season.Machine().ToState(initialState)
	if err != nil {
		return 0, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}

	// A NULL id lets SQLite assign a fresh AUTOINCREMENT rowid; a non-zero id
	// inserts explicitly or lands in the ON CONFLICT branch, matching the two
	// legacy insert column lists.
	inserted, err := s.txQueries(tx).CreateSeason(ctx, sqlcdb.CreateSeasonParams{
		ID:               tvNullID(season.ID),
		SeriesID:         int64(season.SeriesID),
		SeasonNumber:     int64(season.SeasonNumber),
		SeasonMetadataID: tvNullInt64FromInt32Ptr(season.SeasonMetadataID),
		Monitored:        int64(season.Monitored),
	})
	if err != nil {
		tx.Rollback()
		return 0, err
	}

	err = s.txQueries(tx).InsertSeasonTransition(ctx, sqlcdb.InsertSeasonTransitionParams{
		SeasonID:   inserted,
		ToState:    string(initialState),
		FromState:  sql.NullString{},
		MostRecent: true,
		SortKey:    1,
	})
	if err != nil {
		tx.Rollback()
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		tx.Rollback()
		return 0, err
	}

	return inserted, nil
}

// GetSeasonByID gets a season by id
func (s *SQLite) GetSeasonByID(ctx context.Context, id int64) (*storage.Season, error) {
	return s.getSeason(ctx, sqlcdb.GetSeasonParams{ID: sql.NullInt64{Int64: id, Valid: true}})
}

// GetSeasonBySeriesIDAndNumber gets a season given its series id and season number
func (s *SQLite) GetSeasonBySeriesIDAndNumber(ctx context.Context, seriesID int64, seasonNumber int32) (*storage.Season, error) {
	return s.getSeason(ctx, sqlcdb.GetSeasonParams{
		SeriesID:     sql.NullInt64{Int64: seriesID, Valid: true},
		SeasonNumber: sql.NullInt64{Int64: int64(seasonNumber), Valid: true},
	})
}

func (s *SQLite) getSeason(ctx context.Context, params sqlcdb.GetSeasonParams) (*storage.Season, error) {
	row, err := s.Queries.GetSeason(ctx, params)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get season: %w", err)
	}

	return tvSeasonFromRow(row), nil
}

// DeleteSeason removes a season by id
func (s *SQLite) DeleteSeason(ctx context.Context, id int64) error {
	err := s.Queries.DeleteSeason(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to delete season: %w", err)
	}

	return nil
}

// ListSeasons lists seasons constrained by the filter; a zero-value filter
// lists everything.
func (s *SQLite) ListSeasons(ctx context.Context, filter storage.SeasonFilter) ([]*storage.Season, error) {
	params := sqlcdb.ListSeasonsParams{}
	if filter.SeriesID != nil {
		params.SeriesID = sql.NullInt64{Int64: *filter.SeriesID, Valid: true}
	}

	slots, err := tvStateSlotsFrom(tvStateStrings(filter.States))
	if err != nil {
		return nil, fmt.Errorf("failed to list seasons: %w", err)
	}
	params.StatesAny = slots.Any
	params.State2 = slots.S2
	params.State3 = slots.S3
	params.State4 = slots.S4

	rows, err := s.Queries.ListSeasons(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to list seasons: %w", err)
	}

	seasons := make([]*storage.Season, 0, len(rows))
	for _, r := range rows {
		seasons = append(seasons, tvSeasonFromListRow(r))
	}

	return seasons, nil
}

// UpdateSeasonState updates the transition state of a season
// Metadata is optional and can be nil
func (s *SQLite) UpdateSeasonState(ctx context.Context, id int64, state storage.SeasonState, metadata *storage.TransitionStateMetadata) error {
	season, err := s.GetSeasonByID(ctx, id)
	if err != nil {
		return err
	}

	err = season.Machine().ToState(state)
	if err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	// Demote the current most recent transition; the query errors when the
	// season has no transition, matching the legacy behavior.
	previousSortKey, err := s.txQueries(tx).DemoteSeasonTransition(ctx, id)
	if err != nil {
		tx.Rollback()
		return err
	}

	transition := sqlcdb.InsertSeasonTransitionParams{
		SeasonID:   id,
		ToState:    string(state),
		FromState:  sql.NullString{},
		MostRecent: true,
		SortKey:    int64(int32(previousSortKey) + 1),
	}

	if metadata != nil {
		if metadata.DownloadClientID != nil {
			transition.DownloadClientID = sql.NullInt64{Int64: int64(*metadata.DownloadClientID), Valid: true}
		}
		if metadata.DownloadID != nil {
			transition.DownloadID = sql.NullString{String: *metadata.DownloadID, Valid: true}
		}
		if metadata.IsEntireSeasonDownload != nil {
			transition.IsEntireSeasonDownload = sql.NullBool{Bool: *metadata.IsEntireSeasonDownload, Valid: true}
		}
	}

	err = s.txQueries(tx).InsertSeasonTransition(ctx, transition)
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}
