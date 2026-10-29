package sqlite

import (
	"context"
	"database/sql"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

// LinkSeasonMetadata links a season with its metadata
func (s *SQLite) LinkSeasonMetadata(ctx context.Context, seasonID int64, metadataID int64) error {
	return s.Queries.LinkSeasonMetadata(ctx, sqlcdb.LinkSeasonMetadataParams{
		SeasonMetadataID: sql.NullInt64{Int64: metadataID, Valid: true},
		ID:               seasonID,
	})
}

// CreateSeason stores a season in the database. A zero season.ID lets SQLite
// assign a fresh autoincrement id; a non-zero id upserts by id.
func (s *SQLite) CreateSeason(ctx context.Context, season sqlcdb.Season, initialState storage.SeasonState) (int64, error) {
	if err := storage.SeasonStateNew.Machine().ToState(initialState); err != nil {
		return 0, err
	}

	var inserted int64
	err := s.withTx(ctx, func(q *sqlcdb.Queries) error {
		var err error
		inserted, err = q.CreateSeason(ctx, sqlcdb.CreateSeasonParams{
			ID:               sql.NullInt64{Int64: season.ID, Valid: season.ID != 0},
			SeriesID:         season.SeriesID,
			SeasonNumber:     season.SeasonNumber,
			SeasonMetadataID: season.SeasonMetadataID,
			Monitored:        season.Monitored,
		})
		if err != nil {
			return err
		}

		current, err := q.GetSeasonByID(ctx, inserted)
		if err != nil {
			return err
		}
		// Upserts update the entity, never reset its transition history.
		if current.State.Valid {
			return nil
		}

		return q.InsertSeasonTransition(ctx, sqlcdb.InsertSeasonTransitionParams{
			SeasonID:   inserted,
			ToState:    string(initialState),
			MostRecent: true,
			SortKey:    1,
		})
	})
	if err != nil {
		return 0, err
	}

	return inserted, nil
}

// GetSeasonByID gets a season by id
func (s *SQLite) GetSeasonByID(ctx context.Context, id int64) (sqlcdb.GetSeasonByIDRow, error) {
	row, err := s.Queries.GetSeasonByID(ctx, id)
	if err != nil {
		return sqlcdb.GetSeasonByIDRow{}, mapQueryErr(err, "failed to get season")
	}
	return row, nil
}

// GetSeasonBySeriesIDAndNumber gets a season given its series id and season number
func (s *SQLite) GetSeasonBySeriesIDAndNumber(ctx context.Context, seriesID int64, seasonNumber int64) (sqlcdb.GetSeasonBySeriesIDAndNumberRow, error) {
	row, err := s.Queries.GetSeasonBySeriesIDAndNumber(ctx, sqlcdb.GetSeasonBySeriesIDAndNumberParams{
		SeriesID:     seriesID,
		SeasonNumber: seasonNumber,
	})
	if err != nil {
		return sqlcdb.GetSeasonBySeriesIDAndNumberRow{}, mapQueryErr(err, "failed to get season")
	}
	return row, nil
}

// DeleteSeason removes a season by id
func (s *SQLite) DeleteSeason(ctx context.Context, id int64) error {
	if err := s.Queries.DeleteSeason(ctx, id); err != nil {
		return err
	}
	return nil
}

// ListSeasons lists seasons constrained by the filter; a zero-value filter
// lists everything. The States filter accepts any number of states.
func (s *SQLite) ListSeasons(ctx context.Context, filter storage.SeasonFilter) ([]sqlcdb.ListSeasonsRow, error) {
	params := sqlcdb.ListSeasonsParams{
		States: make([]string, 0, len(filter.States)),
	}
	if filter.SeriesID != nil {
		params.SeriesID = sql.NullInt64{Int64: *filter.SeriesID, Valid: true}
	}
	for _, state := range filter.States {
		params.States = append(params.States, string(state))
	}
	if len(params.States) > 0 {
		params.StatesAny = sql.NullString{String: params.States[0], Valid: true}
	}

	rows, err := s.Queries.ListSeasons(ctx, params)
	if err != nil {
		return nil, mapQueryErr(err, "failed to list seasons")
	}
	return rows, nil
}

// UpdateSeasonState updates the transition state of a season. Metadata is
// optional and can be nil. The demote+insert pair runs in one transaction; on
// any failure nothing is written.
func (s *SQLite) UpdateSeasonState(ctx context.Context, id int64, state storage.SeasonState, metadata *storage.TransitionStateMetadata) error {
	return s.withTx(ctx, func(q *sqlcdb.Queries) error {
		season, err := q.GetSeasonByID(ctx, id)
		if err != nil {
			return mapQueryErr(err, "failed to get season")
		}
		if err := storage.SeasonState(season.State.String).Machine().ToState(state); err != nil {
			return err
		}

		// Demote the current most recent transition; the query errors when the
		// season has no transition, matching the legacy behavior.
		previousSortKey, err := q.DemoteSeasonTransition(ctx, id)
		if err != nil {
			return err
		}

		transition := sqlcdb.InsertSeasonTransitionParams{
			SeasonID:   id,
			FromState:  season.State,
			ToState:    string(state),
			MostRecent: true,
			SortKey:    previousSortKey + 1,
		}
		if metadata != nil {
			if metadata.DownloadClientID != nil {
				transition.DownloadClientID = sql.NullInt64{Int64: *metadata.DownloadClientID, Valid: true}
			}
			if metadata.DownloadID != nil {
				transition.DownloadID = sql.NullString{String: *metadata.DownloadID, Valid: true}
			}
			if metadata.IsEntireSeasonDownload != nil {
				transition.IsEntireSeasonDownload = sql.NullBool{Bool: *metadata.IsEntireSeasonDownload, Valid: true}
			}
		}

		return q.InsertSeasonTransition(ctx, transition)
	})
}
