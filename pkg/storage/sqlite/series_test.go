package sqlite

import (
	"database/sql"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/pkg/ptr"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeriesStorage(t *testing.T) {
	ctx := t.Context()
	store := initSqlite(t, ctx)
	list, err := store.ListSeries(ctx, storage.SeriesFilter{})
	require.NoError(t, err)
	assert.Empty(t, list)
	series := sqlcdb.Series{Monitored: 1, QualityProfileID: 1, Path: sql.NullString{String: "/library/tv/test", Valid: true}, MonitorNewSeasons: 1}
	id, err := store.CreateSeries(ctx, series, storage.SeriesStateMissing)
	require.NoError(t, err)
	series.ID = id
	got, err := store.GetSeriesByID(ctx, id)
	require.NoError(t, err)
	require.True(t, got.Series.Added.Valid)
	require.False(t, got.Series.Added.Time.IsZero())
	series.Added = got.Series.Added
	assert.Equal(t, sqlcdb.GetSeriesRow{Series: series, State: string(storage.SeriesStateMissing)}, got)
	list, err = store.ListSeries(ctx, storage.SeriesFilter{})
	require.NoError(t, err)
	assert.Equal(t, []sqlcdb.ListSeriesRow{{Series: series, State: got.State}}, list)
	byPath, err := store.GetSeriesByPath(ctx, series.Path.String)
	require.NoError(t, err)
	assert.Equal(t, got, byPath)
	require.NoError(t, store.UpdateSeriesByID(ctx, id, sqlcdb.Series{Monitored: 0, QualityProfileID: 3}))
	series.Monitored = 0
	got, err = store.GetSeriesByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, sqlcdb.GetSeriesRow{Series: series, State: string(storage.SeriesStateMissing)}, got)
	require.NoError(t, store.DeleteSeries(ctx, id))
	list, err = store.ListSeries(ctx, storage.SeriesFilter{})
	require.NoError(t, err)
	assert.Empty(t, list)
	_, err = store.GetSeriesByID(ctx, id)
	assert.ErrorIs(t, err, storage.ErrNotFound)
	_, err = store.GetSeriesByPath(ctx, "missing")
	assert.ErrorIs(t, err, storage.ErrNotFound)
	_, err = store.GetSeriesByMetadataID(ctx, 9999)
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestSeriesStorage_CreateAddedDefault(t *testing.T) {
	for _, added := range []sql.NullTime{{}, {Valid: true}, {Time: time.Date(2024, 5, 1, 12, 30, 0, 0, time.UTC), Valid: true}} {
		t.Run(added.Time.String(), func(t *testing.T) {
			ctx := t.Context()
			store := initSqlite(t, ctx)
			id, err := store.CreateSeries(ctx, sqlcdb.Series{QualityProfileID: 1, Added: added}, storage.SeriesStateMissing)
			require.NoError(t, err)
			got, err := store.GetSeriesByID(ctx, id)
			require.NoError(t, err)
			require.True(t, got.Series.Added.Valid)
			if !added.Time.IsZero() {
				assert.True(t, added.Time.Equal(got.Series.Added.Time))
			} else {
				assert.WithinDuration(t, time.Now(), got.Series.Added.Time, 5*time.Second)
			}
		})
	}
}

func TestSeriesStorage_ListFilters(t *testing.T) {
	ctx := t.Context()
	store := initSqlite(t, ctx)
	missing, err := store.CreateSeries(ctx, sqlcdb.Series{Monitored: 1, QualityProfileID: 1}, storage.SeriesStateMissing)
	require.NoError(t, err)
	discovered, err := store.CreateSeries(ctx, sqlcdb.Series{QualityProfileID: 1}, storage.SeriesStateDiscovered)
	require.NoError(t, err)
	downloading, err := store.CreateSeries(ctx, sqlcdb.Series{Monitored: 1, QualityProfileID: 1}, storage.SeriesStateMissing)
	require.NoError(t, err)
	require.NoError(t, store.UpdateSeriesState(ctx, downloading, storage.SeriesStateDownloading, nil))
	for _, tc := range []struct {
		name   string
		filter storage.SeriesFilter
		ids    []int64
	}{
		{"all", storage.SeriesFilter{}, []int64{missing, discovered, downloading}},
		{"monitored", storage.SeriesFilter{Monitored: ptr.To(true)}, []int64{missing, downloading}},
		{"unmonitored", storage.SeriesFilter{Monitored: ptr.To(false)}, []int64{discovered}},
		{"missing", storage.SeriesFilter{States: []storage.SeriesState{storage.SeriesStateMissing}}, []int64{missing}},
		{"empty", storage.SeriesFilter{States: []storage.SeriesState{storage.SeriesStateCompleted}}, nil},
		{"many states", storage.SeriesFilter{States: []storage.SeriesState{storage.SeriesStateCompleted, storage.SeriesStateContinuing, storage.SeriesStateUnreleased, storage.SeriesStateNew, storage.SeriesStateMissing, storage.SeriesStateDownloading}}, []int64{missing, downloading}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows, err := store.ListSeries(ctx, tc.filter)
			require.NoError(t, err)
			var ids []int64
			for _, row := range rows {
				ids = append(ids, row.Series.ID)
			}
			assert.ElementsMatch(t, tc.ids, ids)
		})
	}
}

func TestSeriesStorage_DefaultState(t *testing.T) {
	ctx := t.Context()
	store := initSqlite(t, ctx)
	id, err := store.CreateSeries(ctx, sqlcdb.Series{Monitored: 1, QualityProfileID: 1}, storage.SeriesStateNew)
	require.Error(t, err)
	assert.Zero(t, id)
	list, err := store.ListSeries(ctx, storage.SeriesFilter{})
	require.NoError(t, err)
	assert.Empty(t, list, "invalid initial state must not write a series")
}

func TestSQLite_UpdateSeriesState(t *testing.T) {
	ctx := t.Context()
	store := initSqlite(t, ctx)
	id, err := store.CreateSeries(ctx, sqlcdb.Series{QualityProfileID: 1}, storage.SeriesStateUnreleased)
	require.NoError(t, err)
	for _, state := range []storage.SeriesState{storage.SeriesStateMissing, storage.SeriesStateDownloading, storage.SeriesStateCompleted, storage.SeriesStateContinuing, storage.SeriesStateCompleted} {
		require.NoError(t, store.UpdateSeriesState(ctx, id, state, nil))
		got, err := store.GetSeriesByID(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, string(state), got.State)
	}
	before, err := store.GetSeriesByID(ctx, id)
	require.NoError(t, err)
	require.Error(t, store.UpdateSeriesState(ctx, id, storage.SeriesStateMissing, nil))
	after, err := store.GetSeriesByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, before, after)
	assert.ErrorIs(t, store.UpdateSeriesState(ctx, 999999, storage.SeriesStateMissing, nil), storage.ErrNotFound)
}
