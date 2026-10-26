package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

// maxTVFilterStates is the number of fixed nullable slots the series, season
// and episode list queries reserve for the States IN filter.
const maxTVFilterStates = 4

// tv conversion helpers translate between sqlc's default types (int64,
// sql.Null*) and the domain shapes in pkg/storage/model and pkg/storage.
// Names are prefixed to avoid collisions with other migration groups.

func tvNullString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

func tvStringPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	return &ns.String
}

func tvStringFromNull(ns sql.NullString) string {
	return ns.String
}

func tvNullInt64FromInt32Ptr(i *int32) sql.NullInt64 {
	if i == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*i), Valid: true}
}

func tvInt32Ptr(ni sql.NullInt64) *int32 {
	if !ni.Valid {
		return nil
	}
	i := int32(ni.Int64)
	return &i
}

func tvNullID(id int32) sql.NullInt64 {
	if id == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(id), Valid: true}
}

func tvInt32(ni sql.NullInt64) int32 {
	return int32(ni.Int64)
}

func tvNullTime(t *time.Time) sql.NullTime {
	if t == nil || t.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *t, Valid: true}
}

func tvTimePtr(nt sql.NullTime) *time.Time {
	if !nt.Valid {
		return nil
	}
	return &nt.Time
}

// tvMonitored converts an optional monitored flag to the nullable integer
// used by the list queries; nil means "no constraint".
func tvMonitored(monitored *bool) sql.NullInt64 {
	if monitored == nil {
		return sql.NullInt64{}
	}
	if *monitored {
		return sql.NullInt64{Int64: 1, Valid: true}
	}
	return sql.NullInt64{Int64: 0, Valid: true}
}

// tvStateStrings converts a typed state slice to strings.
func tvStateStrings[S ~string](states []S) []string {
	out := make([]string, len(states))
	for i, s := range states {
		out[i] = string(s)
	}
	return out
}

// tvStateSlots holds the fixed nullable slots that stand in for a states IN
// list. The first slot doubles as the guard: when it is NULL no state
// constraint is applied at all, so a zero-value filter matches everything.
type tvStateSlots struct {
	Any sql.NullString
	S2  sql.NullString
	S3  sql.NullString
	S4  sql.NullString
}

func tvStateSlotsFrom(states []string) (tvStateSlots, error) {
	var slots tvStateSlots
	if len(states) > maxTVFilterStates {
		return slots, fmt.Errorf("at most %d states supported per filter, got %d", maxTVFilterStates, len(states))
	}

	targets := []*sql.NullString{&slots.Any, &slots.S2, &slots.S3, &slots.S4}
	for i, s := range states {
		*targets[i] = sql.NullString{String: s, Valid: true}
	}
	return slots, nil
}

// tvSeriesFromRow converts a single-row series result into the domain shape.
func tvSeriesFromRow(r sqlcdb.GetSeriesRow) *storage.Series {
	return &storage.Series{
		Series: model.Series{
			ID:                int32(r.ID),
			Path:              tvStringPtr(r.Path),
			Monitored:         int32(r.Monitored),
			Added:             tvTimePtr(r.Added),
			QualityProfileID:  int32(r.QualityProfileID),
			SeriesMetadataID:  tvInt32Ptr(r.SeriesMetadataID),
			LastSearchTime:    tvTimePtr(r.LastSearchTime),
			MonitorNewSeasons: int32(r.MonitorNewSeasons),
		},
		State: storage.SeriesState(r.ToState),
	}
}

// tvSeriesFromListRow converts a list-series result into the domain shape.
func tvSeriesFromListRow(r sqlcdb.ListSeriesRow) *storage.Series {
	return &storage.Series{
		Series: model.Series{
			ID:                int32(r.ID),
			Path:              tvStringPtr(r.Path),
			Monitored:         int32(r.Monitored),
			Added:             tvTimePtr(r.Added),
			QualityProfileID:  int32(r.QualityProfileID),
			SeriesMetadataID:  tvInt32Ptr(r.SeriesMetadataID),
			LastSearchTime:    tvTimePtr(r.LastSearchTime),
			MonitorNewSeasons: int32(r.MonitorNewSeasons),
		},
		State: storage.SeriesState(r.ToState),
	}
}

// CreateSeries stores a Series in the database
func (s *SQLite) CreateSeries(ctx context.Context, series storage.Series, initialState storage.SeriesState) (int64, error) {
	if series.State == "" {
		series.State = storage.SeriesStateNew
	}

	err := series.Machine().ToState(initialState)
	if err != nil {
		return 0, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}

	// When Added is nil or zero the column is omitted so the column default
	// (CURRENT_TIMESTAMP) applies, matching the legacy statement.
	withAdded := series.Added != nil && !series.Added.IsZero()

	var inserted int64
	if withAdded {
		inserted, err = s.txQueries(tx).CreateSeriesWithAdded(ctx, sqlcdb.CreateSeriesWithAddedParams{
			Path:              tvNullString(series.Path),
			Monitored:         int64(series.Monitored),
			Added:             tvNullTime(series.Added),
			QualityProfileID:  int64(series.QualityProfileID),
			SeriesMetadataID:  tvNullInt64FromInt32Ptr(series.SeriesMetadataID),
			LastSearchTime:    tvNullTime(series.LastSearchTime),
			MonitorNewSeasons: int64(series.MonitorNewSeasons),
		})
	} else {
		inserted, err = s.txQueries(tx).CreateSeries(ctx, sqlcdb.CreateSeriesParams{
			Path:              tvNullString(series.Path),
			Monitored:         int64(series.Monitored),
			QualityProfileID:  int64(series.QualityProfileID),
			SeriesMetadataID:  tvNullInt64FromInt32Ptr(series.SeriesMetadataID),
			LastSearchTime:    tvNullTime(series.LastSearchTime),
			MonitorNewSeasons: int64(series.MonitorNewSeasons),
		})
	}
	if err != nil {
		tx.Rollback()
		return 0, err
	}

	err = s.txQueries(tx).InsertSeriesTransition(ctx, sqlcdb.InsertSeriesTransitionParams{
		SeriesID:   inserted,
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

// GetSeriesByID looks for a series given its id
func (s *SQLite) GetSeriesByID(ctx context.Context, id int64) (*storage.Series, error) {
	return s.getSeries(ctx, sqlcdb.GetSeriesParams{ID: sql.NullInt64{Int64: id, Valid: true}})
}

// GetSeriesByMetadataID looks for a series given its metadata id
func (s *SQLite) GetSeriesByMetadataID(ctx context.Context, metadataID int32) (*storage.Series, error) {
	return s.getSeries(ctx, sqlcdb.GetSeriesParams{SeriesMetadataID: sql.NullInt64{Int64: int64(metadataID), Valid: true}})
}

// GetSeriesByPath looks for a series given its path
func (s *SQLite) GetSeriesByPath(ctx context.Context, path string) (*storage.Series, error) {
	return s.getSeries(ctx, sqlcdb.GetSeriesParams{Path: sql.NullString{String: path, Valid: true}})
}

func (s *SQLite) getSeries(ctx context.Context, params sqlcdb.GetSeriesParams) (*storage.Series, error) {
	row, err := s.Queries.GetSeries(ctx, params)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get series: %w", err)
	}

	return tvSeriesFromRow(row), nil
}

// DeleteSeries removes a Series by id
func (s *SQLite) DeleteSeries(ctx context.Context, id int64) error {
	err := s.Queries.DeleteSeries(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to delete Series: %w", err)
	}

	return nil
}

// ListSeries lists all Series constrained by the filter; a zero-value filter
// lists everything.
func (s *SQLite) ListSeries(ctx context.Context, filter storage.SeriesFilter) ([]*storage.Series, error) {
	params := sqlcdb.ListSeriesParams{
		Monitored: tvMonitored(filter.Monitored),
	}

	slots, err := tvStateSlotsFrom(tvStateStrings(filter.States))
	if err != nil {
		return nil, fmt.Errorf("failed to list series: %w", err)
	}
	params.StatesAny = slots.Any
	params.State2 = slots.S2
	params.State3 = slots.S3
	params.State4 = slots.S4

	rows, err := s.Queries.ListSeries(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to list series: %w", err)
	}

	series := make([]*storage.Series, 0, len(rows))
	for _, r := range rows {
		series = append(series, tvSeriesFromListRow(r))
	}

	return series, nil
}
