package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

// CreateSeries stores a Series in the database. When series.Added is zero the
// column default (CURRENT_TIMESTAMP, UTC) applies.
func (s *SQLite) CreateSeries(ctx context.Context, series sqlcdb.Series, initialState storage.SeriesState) (int64, error) {
	if err := storage.SeriesStateNew.Machine().ToState(initialState); err != nil {
		return 0, err
	}

	var inserted int64
	err := s.withTx(ctx, func(q *sqlcdb.Queries) error {
		var err error
		if series.Added.Valid && !series.Added.Time.IsZero() {
			inserted, err = q.CreateSeriesWithAdded(ctx, sqlcdb.CreateSeriesWithAddedParams{
				Path:              series.Path,
				Monitored:         series.Monitored,
				Added:             series.Added,
				QualityProfileID:  series.QualityProfileID,
				SeriesMetadataID:  series.SeriesMetadataID,
				LastSearchTime:    series.LastSearchTime,
				MonitorNewSeasons: series.MonitorNewSeasons,
			})
		} else {
			inserted, err = q.CreateSeries(ctx, sqlcdb.CreateSeriesParams{
				Path:              series.Path,
				Monitored:         series.Monitored,
				QualityProfileID:  series.QualityProfileID,
				SeriesMetadataID:  series.SeriesMetadataID,
				LastSearchTime:    series.LastSearchTime,
				MonitorNewSeasons: series.MonitorNewSeasons,
			})
		}
		if err != nil {
			return err
		}

		return q.InsertSeriesTransition(ctx, sqlcdb.InsertSeriesTransitionParams{
			SeriesID:   inserted,
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

// GetSeriesByID looks for a series given its id
func (s *SQLite) GetSeriesByID(ctx context.Context, id int64) (sqlcdb.GetSeriesRow, error) {
	return s.getSeries(ctx, sqlcdb.GetSeriesParams{ID: sql.NullInt64{Int64: id, Valid: true}})
}

// GetSeriesByMetadataID looks for a series given its metadata id
func (s *SQLite) GetSeriesByMetadataID(ctx context.Context, metadataID int64) (sqlcdb.GetSeriesRow, error) {
	return s.getSeries(ctx, sqlcdb.GetSeriesParams{SeriesMetadataID: sql.NullInt64{Int64: metadataID, Valid: true}})
}

// GetSeriesByPath looks for a series given its path
func (s *SQLite) GetSeriesByPath(ctx context.Context, path string) (sqlcdb.GetSeriesRow, error) {
	return s.getSeries(ctx, sqlcdb.GetSeriesParams{Path: sql.NullString{String: path, Valid: true}})
}

func (s *SQLite) getSeries(ctx context.Context, params sqlcdb.GetSeriesParams) (sqlcdb.GetSeriesRow, error) {
	row, err := s.Queries.GetSeries(ctx, params)
	if err != nil {
		return sqlcdb.GetSeriesRow{}, mapQueryErr(err, "failed to get series")
	}
	return row, nil
}

// DeleteSeries removes a Series by id
func (s *SQLite) DeleteSeries(ctx context.Context, id int64) error {
	if err := s.Queries.DeleteSeries(ctx, id); err != nil {
		return err
	}
	return nil
}

// ListSeries lists all Series constrained by the filter; a zero-value filter
// lists everything. The States filter accepts any number of states.
func (s *SQLite) ListSeries(ctx context.Context, filter storage.SeriesFilter) ([]sqlcdb.ListSeriesRow, error) {
	params := sqlcdb.ListSeriesParams{
		States: make([]string, 0, len(filter.States)),
	}
	if filter.Monitored != nil {
		if *filter.Monitored {
			params.Monitored = sql.NullInt64{Int64: 1, Valid: true}
		} else {
			params.Monitored = sql.NullInt64{Int64: 0, Valid: true}
		}
	}
	for _, state := range filter.States {
		params.States = append(params.States, string(state))
	}
	if len(params.States) > 0 {
		params.StatesAny = sql.NullString{String: params.States[0], Valid: true}
	}

	rows, err := s.Queries.ListSeries(ctx, params)
	if err != nil {
		return nil, mapQueryErr(err, "failed to list series")
	}
	return rows, nil
}

// ListLibraryShows lists metadata-backed series in stable title/id order.
func (s *SQLite) ListLibraryShows(ctx context.Context, offset, limit int, states []storage.SeriesState) ([]sqlcdb.ListLibraryShowsRow, error) {
	if offset < 0 || limit <= 0 {
		return nil, fmt.Errorf("invalid library pagination: offset %d, limit %d", offset, limit)
	}
	stateStrings := make([]string, 0, len(states))
	for _, state := range states {
		stateStrings = append(stateStrings, string(state))
	}
	encoded, err := json.Marshal(stateStrings)
	if err != nil {
		return nil, err
	}
	return s.Queries.ListLibraryShows(ctx, sqlcdb.ListLibraryShowsParams{Offset: int64(offset), Limit: int64(limit), States: string(encoded)})
}
func (s *SQLite) CountLibraryShows(ctx context.Context, states []storage.SeriesState) (int64, error) {
	stateStrings := make([]string, 0, len(states))
	for _, state := range states {
		stateStrings = append(stateStrings, string(state))
	}
	encoded, err := json.Marshal(stateStrings)
	if err != nil {
		return 0, err
	}
	return s.Queries.CountLibraryShows(ctx, string(encoded))
}
func (s *SQLite) CountLibraryShowsByState(ctx context.Context) ([]sqlcdb.CountLibraryShowsByStateRow, error) {
	return s.Queries.CountLibraryShowsByState(ctx)
}

// UpdateSeriesState updates the transition state of a series. The demote and
// insert statements run in one transaction; on any failure nothing is written.
func (s *SQLite) UpdateSeriesState(ctx context.Context, id int64, state storage.SeriesState, metadata *storage.TransitionStateMetadata) error {
	return s.withTx(ctx, func(q *sqlcdb.Queries) error {
		series, err := q.GetSeries(ctx, sqlcdb.GetSeriesParams{ID: sql.NullInt64{Int64: id, Valid: true}})
		if err != nil {
			return mapQueryErr(err, "failed to get series")
		}
		if err := storage.SeriesState(series.State).Machine().ToState(state); err != nil {
			return err
		}

		previousSortKey, err := q.DemoteSeriesTransition(ctx, id)
		if err != nil {
			return err
		}

		return q.InsertSeriesTransition(ctx, sqlcdb.InsertSeriesTransitionParams{
			SeriesID:   id,
			FromState:  sql.NullString{String: series.State, Valid: true},
			ToState:    string(state),
			MostRecent: true,
			SortKey:    previousSortKey + 1,
		})
	})
}

// LinkSeriesMetadata links a series with its metadata
func (s *SQLite) LinkSeriesMetadata(ctx context.Context, seriesID int64, metadataID int64) error {
	return s.Queries.LinkSeriesMetadata(ctx, sqlcdb.LinkSeriesMetadataParams{
		SeriesMetadataID: sql.NullInt64{Int64: metadataID, Valid: true},
		ID:               seriesID,
	})
}

// UpdateSeriesByID updates fields on a series identified by id.
// Legacy parity: only the monitored column is persisted.
func (s *SQLite) UpdateSeriesByID(ctx context.Context, id int64, series sqlcdb.Series) error {
	return s.Queries.UpdateSeriesByID(ctx, sqlcdb.UpdateSeriesByIDParams{
		Monitored: series.Monitored,
		ID:        id,
	})
}
