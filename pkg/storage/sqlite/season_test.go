package sqlite

import (
	"context"
	"database/sql"
	"testing"

	"github.com/kasuboski/mediaz/pkg/ptr"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tvTestSeries(t *testing.T, ctx context.Context, store storage.Storage) int64 {
	t.Helper()
	id, err := store.CreateSeries(ctx, sqlcdb.Series{Monitored: 1, QualityProfileID: 1}, storage.SeriesStateMissing)
	require.NoError(t, err)
	return id
}

func TestSeasonStorage(t *testing.T) {
	ctx := t.Context()
	store := initSqlite(t, ctx)
	seriesID := tvTestSeries(t, ctx, store)
	season := sqlcdb.Season{SeriesID: seriesID, SeasonNumber: 1, Monitored: 1}
	id, err := store.CreateSeason(ctx, season, storage.SeasonStateMissing)
	require.NoError(t, err)
	season.ID = id
	got, err := store.GetSeasonByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, sqlcdb.GetSeasonByIDRow{Season: season, State: sql.NullString{String: string(storage.SeasonStateMissing), Valid: true}}, got)
	list, err := store.ListSeasons(ctx, storage.SeasonFilter{SeriesID: &seriesID})
	require.NoError(t, err)
	assert.Equal(t, []sqlcdb.ListSeasonsRow{{Season: season, State: got.State.String}}, list)
	byNumber, err := store.GetSeasonBySeriesIDAndNumber(ctx, seriesID, 1)
	require.NoError(t, err)
	assert.Equal(t, season, byNumber.Season)
	_, err = store.GetSeasonBySeriesIDAndNumber(ctx, seriesID, 9)
	assert.ErrorIs(t, err, storage.ErrNotFound)
	_, err = store.GetSeasonBySeriesIDAndNumber(ctx, seriesID+1000, 1)
	assert.ErrorIs(t, err, storage.ErrNotFound)
	require.NoError(t, store.DeleteSeason(ctx, id))
	list, err = store.ListSeasons(ctx, storage.SeasonFilter{SeriesID: &seriesID})
	require.NoError(t, err)
	assert.Empty(t, list)
	_, err = store.GetSeasonByID(ctx, id)
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestSeasonStorage_ListFilters(t *testing.T) {
	ctx := t.Context()
	store := initSqlite(t, ctx)
	seriesID := tvTestSeries(t, ctx, store)
	otherSeries := tvTestSeries(t, ctx, store)
	missing, err := store.CreateSeason(ctx, sqlcdb.Season{SeriesID: seriesID, SeasonNumber: 1, Monitored: 1}, storage.SeasonStateMissing)
	require.NoError(t, err)
	downloading, err := store.CreateSeason(ctx, sqlcdb.Season{SeriesID: seriesID, SeasonNumber: 2, Monitored: 1}, storage.SeasonStateMissing)
	require.NoError(t, err)
	require.NoError(t, store.UpdateSeasonState(ctx, downloading, storage.SeasonStateDownloading, nil))
	other, err := store.CreateSeason(ctx, sqlcdb.Season{SeriesID: otherSeries, SeasonNumber: 1}, storage.SeasonStateMissing)
	require.NoError(t, err)
	for _, tc := range []struct {
		name   string
		filter storage.SeasonFilter
		ids    []int64
	}{
		{"all", storage.SeasonFilter{}, []int64{missing, downloading, other}},
		{"series", storage.SeasonFilter{SeriesID: &seriesID}, []int64{missing, downloading}},
		{"missing", storage.SeasonFilter{SeriesID: &seriesID, States: []storage.SeasonState{storage.SeasonStateMissing}}, []int64{missing}},
		{"empty", storage.SeasonFilter{States: []storage.SeasonState{storage.SeasonStateCompleted}}, nil},
		{"many states", storage.SeasonFilter{SeriesID: &seriesID, States: []storage.SeasonState{storage.SeasonStateNew, storage.SeasonStateCompleted, storage.SeasonStateContinuing, storage.SeasonStateDiscovered, storage.SeasonStateMissing, storage.SeasonStateDownloading}}, []int64{missing, downloading}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows, err := store.ListSeasons(ctx, tc.filter)
			require.NoError(t, err)
			var ids []int64
			for _, row := range rows {
				ids = append(ids, row.Season.ID)
			}
			assert.ElementsMatch(t, tc.ids, ids)
		})
	}
}

func TestSQLite_UpdateSeasonState(t *testing.T) {
	ctx := t.Context()
	store := initSqlite(t, ctx)
	id, err := store.CreateSeason(ctx, sqlcdb.Season{SeriesID: tvTestSeries(t, ctx, store), SeasonNumber: 1, Monitored: 1}, storage.SeasonStateMissing)
	require.NoError(t, err)
	clientID, err := store.CreateDownloadClient(ctx, sqlcdb.DownloadClient{Type: "torrent", Implementation: "transmission", Host: "localhost", Scheme: "http"})
	require.NoError(t, err)
	require.NoError(t, store.UpdateSeasonState(ctx, id, storage.SeasonStateDownloading, &storage.TransitionStateMetadata{DownloadID: ptr.To("bundle"), DownloadClientID: &clientID, IsEntireSeasonDownload: ptr.To(true)}))
	got, err := store.GetSeasonByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, sqlcdb.GetSeasonByIDRow{Season: got.Season, State: sql.NullString{String: string(storage.SeasonStateDownloading), Valid: true}, DownloadID: sql.NullString{String: "bundle", Valid: true}, DownloadClientID: sql.NullInt64{Int64: clientID, Valid: true}, IsEntireSeasonDownload: sql.NullBool{Bool: true, Valid: true}}, got)
	require.NoError(t, store.UpdateSeasonState(ctx, id, storage.SeasonStateCompleted, nil))
	require.NoError(t, store.UpdateSeasonState(ctx, id, storage.SeasonStateContinuing, nil))
	got, err = store.GetSeasonByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, sqlcdb.GetSeasonByIDRow{Season: got.Season, State: sql.NullString{String: string(storage.SeasonStateContinuing), Valid: true}}, got)
	require.NoError(t, store.UpdateSeasonState(ctx, id, storage.SeasonStateCompleted, nil))
	beforeRejected, err := store.GetSeasonByID(ctx, id)
	require.NoError(t, err)
	historyBeforeRejected := seasonTransitionRows(t, store.(*SQLite))
	require.Len(t, historyBeforeRejected, 5)
	for i, transition := range historyBeforeRejected {
		assert.Equal(t, i == len(historyBeforeRejected)-1, transition.MostRecent)
	}
	require.Error(t, store.UpdateSeasonState(ctx, id, storage.SeasonStateMissing, nil))
	afterRejected, err := store.GetSeasonByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, beforeRejected, afterRejected, "rejected update must preserve the complete season row")
	assert.Equal(t, historyBeforeRejected, seasonTransitionRows(t, store.(*SQLite)), "rejected update must preserve all history and latest markers")
	assert.ErrorIs(t, store.UpdateSeasonState(ctx, 999999, storage.SeasonStateMissing, nil), storage.ErrNotFound)
}
