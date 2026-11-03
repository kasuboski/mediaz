package manager

import (
	"context"
	"database/sql"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/indexer"
	indexerMocks "github.com/kasuboski/mediaz/pkg/indexer/mocks"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	mediaSqlite "github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/kasuboski/mediaz/pkg/tmdb"
	tmdbMocks "github.com/kasuboski/mediaz/pkg/tmdb/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// Regression for CodeRabbit 4459881598: detailSeasons must keep local episodes and
// seasons that have no metadata match visible (file-discovered children).
func TestDetailSeasonsKeepsUnmatchedLocalChildren(t *testing.T) {
	ctx := context.Background()
	store, err := mediaSqlite.New(ctx, filepath.Join(t.TempDir(), "detail.sqlite"))
	require.NoError(t, err)

	mid, err := store.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{TmdbID: 555, Title: "Detail Show"})
	require.NoError(t, err)
	meta, err := store.GetSeriesMetadataByID(ctx, mid)
	require.NoError(t, err)

	seasonMetaID, err := store.CreateSeasonMetadata(ctx, sqlcdb.SeasonMetadatum{SeriesMetadataID: mid, Number: 1, TmdbID: 601, Title: "Season One"})
	require.NoError(t, err)
	episodeMetaID, err := store.CreateEpisodeMetadata(ctx, sqlcdb.EpisodeMetadatum{SeasonMetadataID: seasonMetaID, Number: 1, TmdbID: 701, Title: "Pilot", AirDate: sql.NullTime{Time: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true}})
	require.NoError(t, err)

	seriesID, err := store.CreateSeries(ctx, sqlcdb.Series{
		QualityProfileID: 4, Monitored: 1,
		SeriesMetadataID: sql.NullInt64{Int64: mid, Valid: true},
	}, storage.SeriesStateDiscovered)
	require.NoError(t, err)

	matchedSeasonID, err := store.CreateSeason(ctx, sqlcdb.Season{SeriesID: seriesID, SeasonNumber: 1, Monitored: 1, SeasonMetadataID: sql.NullInt64{Int64: seasonMetaID, Valid: true}}, storage.SeasonStateMissing)
	require.NoError(t, err)
	_, err = store.CreateEpisode(ctx, sqlcdb.Episode{SeasonID: matchedSeasonID, EpisodeNumber: 1, Monitored: 1, EpisodeMetadataID: sql.NullInt64{Int64: episodeMetaID, Valid: true}}, storage.EpisodeStateMissing)
	require.NoError(t, err)
	// Local episode 7 in the matched season has no metadata row (file discovery).
	unmatchedEpisodeID, err := store.CreateEpisode(ctx, sqlcdb.Episode{SeasonID: matchedSeasonID, EpisodeNumber: 7, Monitored: 1}, storage.EpisodeStateMissing)
	require.NoError(t, err)
	// A whole local season absent from the metadata tree.
	orphanSeasonID, err := store.CreateSeason(ctx, sqlcdb.Season{SeriesID: seriesID, SeasonNumber: 5, Monitored: 0}, storage.SeasonStateMissing)
	require.NoError(t, err)
	orphanEpisodeID, err := store.CreateEpisode(ctx, sqlcdb.Episode{SeasonID: orphanSeasonID, EpisodeNumber: 2, Monitored: 0}, storage.EpisodeStateMissing)
	require.NoError(t, err)

	svc := NewSeriesService(nil, nil, store, store, NewQualityService(store), &mockMetadataProvider{metadata: &meta})
	detail, err := svc.detailSeasons(ctx, mid, seriesID)
	require.NoError(t, err)
	require.Len(t, detail, 2, "metadata season plus unmatched local season must both be present")

	require.Equal(t, int64(1), detail[0].Number)
	require.Equal(t, int32(2), detail[0].EpisodeCount, "metadata episode plus unmatched local episode")
	require.Len(t, detail[0].Episodes, 2)
	first := detail[0].Episodes[0]
	require.NotNil(t, first.ID)
	require.Equal(t, int64(1), first.Number)
	require.Equal(t, "Pilot", first.Title)
	second := detail[0].Episodes[1]
	require.NotNil(t, second.ID, "unmatched local episode must keep its local ID")
	require.Equal(t, unmatchedEpisodeID, *second.ID)
	require.Equal(t, int64(7), second.Number)
	require.Equal(t, "Episode 7", second.Title)

	require.Equal(t, int64(5), detail[1].Number, "local season absent from metadata tree must be preserved")
	require.NotNil(t, detail[1].ID)
	require.Equal(t, orphanSeasonID, *detail[1].ID)
	require.Equal(t, int32(1), detail[1].EpisodeCount)
	require.Len(t, detail[1].Episodes, 1)
	require.NotNil(t, detail[1].Episodes[0].ID)
	require.Equal(t, orphanEpisodeID, *detail[1].Episodes[0].ID)
}

// Regression for CodeRabbit 4459881643: the aggregate revert to continuing must only
// apply to series currently in the missing state; downloading/completed aggregates stay.
func TestReconcileMissingSeriesStateRevertScopedToMissing(t *testing.T) {
	ctx := context.Background()
	build := func(t *testing.T, initial storage.SeriesState) (MediaManager, *sqlcdb.Series, sqlcdb.ListSeriesRow) {
		db, err := mediaSqlite.New(ctx, filepath.Join(t.TempDir(), "reconcile.sqlite"))
		require.NoError(t, err)
		seriesMetadataID, err := db.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{TmdbID: 321, Title: "Scoped Show"})
		require.NoError(t, err)
		seasonMetadataID, err := db.CreateSeasonMetadata(ctx, sqlcdb.SeasonMetadatum{SeriesMetadataID: seriesMetadataID, Number: 1, TmdbID: 611, Title: "Season"})
		require.NoError(t, err)
		episodeMetaID, err := db.CreateEpisodeMetadata(ctx, sqlcdb.EpisodeMetadatum{
			SeasonMetadataID: seasonMetadataID, Number: 1, TmdbID: 711, Title: "Episode",
			AirDate: sql.NullTime{Time: time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true},
		})
		require.NoError(t, err)
		seriesID, err := db.CreateSeries(ctx, sqlcdb.Series{
			QualityProfileID: 1, Monitored: 1,
			SeriesMetadataID: sql.NullInt64{Int64: seriesMetadataID, Valid: true},
		}, storage.SeriesStateMissing)
		require.NoError(t, err)
		// Move to the requested state through valid machine transitions.
		switch initial {
		case storage.SeriesStateDownloading:
			require.NoError(t, db.UpdateSeriesState(ctx, seriesID, storage.SeriesStateDownloading, nil))
		case storage.SeriesStateCompleted:
			require.NoError(t, db.UpdateSeriesState(ctx, seriesID, storage.SeriesStateDiscovered, nil))
			require.NoError(t, db.UpdateSeriesState(ctx, seriesID, storage.SeriesStateCompleted, nil))
		}
		seasonID, err := db.CreateSeason(ctx, sqlcdb.Season{SeriesID: seriesID, SeasonNumber: 1, Monitored: 1, SeasonMetadataID: sql.NullInt64{Int64: seasonMetadataID, Valid: true}}, storage.SeasonStateMissing)
		require.NoError(t, err)
		_, err = db.CreateEpisode(ctx, sqlcdb.Episode{SeasonID: seasonID, EpisodeNumber: 1, Monitored: 1, EpisodeMetadataID: sql.NullInt64{Int64: episodeMetaID, Valid: true}}, storage.EpisodeStateMissing)
		require.NoError(t, err)

		ctrl := gomock.NewController(t)
		indexerSource := indexerMocks.NewMockIndexerSource(ctrl)
		indexerSource.EXPECT().Search(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
		indexerSource.EXPECT().ListIndexers(ctx).Return([]indexer.SourceIndexer{{ID: 1, Name: "test", Priority: 1}}, nil).AnyTimes()
		indexerFactory := indexerMocks.NewMockFactory(ctrl)
		indexerFactory.EXPECT().NewIndexerSource(gomock.Any()).Return(indexerSource, nil).AnyTimes()
		sourceID, err := db.CreateIndexerSource(ctx, sqlcdb.IndexerSource{Name: "test-source", Implementation: "prowlarr", Host: "localhost", Scheme: "http", Enabled: true})
		require.NoError(t, err)
		_, err = db.CreateIndexer(ctx, sqlcdb.Indexer{IndexerSourceID: sql.NullInt64{Int64: sourceID, Valid: true}, Name: "test", Priority: 1, Uri: "http://test/indexer", ProwlarrID: sql.NullInt64{Int64: 1, Valid: true}})
		require.NoError(t, err)
		manager := New(nil, indexerFactory, nil, db, nil, config.Manager{}, config.Config{})
		require.NoError(t, manager.RefreshIndexerSource(ctx, sourceID))
		seriesRow, err := db.GetSeriesByID(ctx, seriesID)
		require.NoError(t, err)
		return manager, &seriesRow.Series, sqlcdb.ListSeriesRow{Series: seriesRow.Series, State: seriesRow.State}
	}
	snapshot := newReconcileSnapshot([]sqlcdb.Indexer{{ID: 1}}, nil)
	snapshot.time = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	t.Run("downloading aggregate is untouched when no eligible missing aired child", func(t *testing.T) {
		manager, series, row := build(t, storage.SeriesStateDownloading)
		require.NoError(t, manager.reconcileMissingSeries(ctx, row, snapshot))
		after, err := manager.seriesStorage.GetSeriesByID(ctx, series.ID)
		require.NoError(t, err)
		require.Equal(t, string(storage.SeriesStateDownloading), after.State)
	})
	t.Run("completed aggregate is untouched when no eligible missing aired child", func(t *testing.T) {
		manager, series, row := build(t, storage.SeriesStateCompleted)
		require.NoError(t, manager.reconcileMissingSeries(ctx, row, snapshot))
		after, err := manager.seriesStorage.GetSeriesByID(ctx, series.ID)
		require.NoError(t, err)
		require.Equal(t, string(storage.SeriesStateCompleted), after.State)
	})
	t.Run("missing aggregate with only unreleased seasons reverts to continuing", func(t *testing.T) {
		manager, series, row := build(t, storage.SeriesStateMissing)
		require.NoError(t, manager.reconcileMissingSeries(ctx, row, snapshot))
		after, err := manager.seriesStorage.GetSeriesByID(ctx, series.ID)
		require.NoError(t, err)
		require.Equal(t, string(storage.SeriesStateContinuing), after.State)
	})
}

// Regression for CodeRabbit 4459881666: UpdateSelection must release the admission
// stripe before the remote TMDB detail fetch.
func TestUpdateSelectionReleasesAdmissionLockBeforeRemoteFetch(t *testing.T) {
	ctx := context.Background()
	store, err := mediaSqlite.New(ctx, filepath.Join(t.TempDir(), "selection-lock.sqlite"))
	require.NoError(t, err)
	mid, err := store.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{TmdbID: 777, Title: "Lock Show"})
	require.NoError(t, err)
	meta, err := store.GetSeriesMetadataByID(ctx, mid)
	require.NoError(t, err)
	seriesID, err := store.CreateSeries(ctx, sqlcdb.Series{
		QualityProfileID: 4, Monitored: 1,
		SeriesMetadataID: sql.NullInt64{Int64: mid, Valid: true},
	}, storage.SeriesStateDiscovered)
	require.NoError(t, err)

	ctrl := gomock.NewController(t)
	tmdbClient := tmdbMocks.NewMockITmdb(ctrl)
	started := make(chan struct{})
	release := make(chan struct{})
	tmdbClient.EXPECT().TvSeriesDetails(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, _ int32, _ *tmdb.TvSeriesDetailsParams, _ ...tmdb.RequestEditorFn) (*http.Response, error) {
		close(started)
		<-release
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
	}).AnyTimes()

	svc := NewSeriesService(tmdbClient, nil, store, store, NewQualityService(store), &mockMetadataProvider{metadata: &meta})
	done := make(chan error, 1)
	go func() {
		monitored := true
		_, err := svc.UpdateSelection(ctx, seriesID, SeriesSelectionPatch{Monitored: &monitored})
		done <- err
	}()

	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("remote TMDB fetch never started")
	}

	// The admission stripe must be free while the remote fetch is in flight.
	acquired := make(chan struct{})
	go func() {
		unlock := storage.LockTVAdmission(seriesID)
		close(acquired)
		unlock()
	}()
	select {
	case <-acquired:
	case <-time.After(5 * time.Second):
		t.Fatal("admission stripe still held during remote TMDB fetch")
	}

	close(release)
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("UpdateSelection did not finish")
	}
}
