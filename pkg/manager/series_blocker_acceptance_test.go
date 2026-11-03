package manager

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/download"
	downloadMock "github.com/kasuboski/mediaz/pkg/download/mocks"
	"github.com/kasuboski/mediaz/pkg/indexer"
	indexerMock "github.com/kasuboski/mediaz/pkg/indexer/mocks"
	"github.com/kasuboski/mediaz/pkg/storage"
	sqlcdb "github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// blockerAcceptanceTVFixtures builds an in-memory library (default seeded
// profiles, series profile 4 = episode scope) with season 1 containing the
// given episodes. Each episode is aired unless future=true.
func blockerAcceptanceTVFixtures(t *testing.T, ctx context.Context, store storage.Storage, seriesState storage.SeriesState, seasonState storage.SeasonState, episodes []struct {
	number    int64
	monitored int64
	future    bool
}) (int64, int64) {
	t.Helper()

	for i := 0; i < 2; i++ {
		_, err := store.CreateDownloadClient(ctx, sqlcdb.DownloadClient{ID: int64(i + 1), Implementation: "transmission", Type: "torrent", Host: "transmission", Port: 8080, Scheme: "http"})
		require.NoError(t, err)
	}

	seriesMetadataID, err := store.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{TmdbID: 1, Title: "Series", Status: "Continuing"})
	require.NoError(t, err)
	seriesID, err := store.CreateSeries(ctx, sqlcdb.Series{QualityProfileID: 4, Monitored: 1, SeriesMetadataID: sql.NullInt64{Int64: seriesMetadataID, Valid: true}}, storage.SeriesStateMissing)
	require.NoError(t, err)
	seasonMetadataID, err := store.CreateSeasonMetadata(ctx, sqlcdb.SeasonMetadatum{SeriesMetadataID: seriesMetadataID, Title: "Season 1", Number: 1})
	require.NoError(t, err)
	seasonID, err := store.CreateSeason(ctx, sqlcdb.Season{SeriesID: seriesID, SeasonNumber: 1, SeasonMetadataID: sql.NullInt64{Int64: seasonMetadataID, Valid: true}, Monitored: 1}, storage.SeasonStateMissing)
	require.NoError(t, err)

	// Walk the aggregate state machines to the parent states under test,
	// as prior acquisition history would have.
	if seriesState != storage.SeriesStateMissing {
		require.NoError(t, store.UpdateSeriesState(ctx, seriesID, storage.SeriesStateDownloading, nil))
		require.NoError(t, store.UpdateSeriesState(ctx, seriesID, seriesState, nil))
	}
	if seasonState != storage.SeasonStateMissing {
		require.NoError(t, store.UpdateSeasonState(ctx, seasonID, storage.SeasonStateDownloading, nil))
		require.NoError(t, store.UpdateSeasonState(ctx, seasonID, seasonState, nil))
	}

	for _, e := range episodes {
		airDate := time.Now().Add(-2 * time.Hour)
		if e.future {
			airDate = time.Now().Add(48 * time.Hour)
		}
		metaID, err := store.CreateEpisodeMetadata(ctx, sqlcdb.EpisodeMetadatum{
			TmdbID:           e.number,
			Title:            "Episode",
			Number:           e.number,
			SeasonMetadataID: seasonMetadataID,
			AirDate:          sqlNullTime(airDate),
			Runtime:          sql.NullInt64{Int64: 42, Valid: true},
		})
		require.NoError(t, err)
		_, err = store.CreateEpisode(ctx, sqlcdb.Episode{
			SeasonID:          seasonID,
			EpisodeNumber:     e.number,
			Monitored:         e.monitored,
			EpisodeMetadataID: sql.NullInt64{Int64: metaID, Valid: true},
		}, storage.EpisodeStateMissing)
		require.NoError(t, err)
	}

	return seriesID, seasonID
}

func sqlNullTime(t time.Time) sql.NullTime {
	return sql.NullTime{Time: t, Valid: true}
}

// blockerAcceptanceIndexer wires a mock indexer source into the store and
// returns a manager whose scheduled/manual searches see the given releases.
func blockerAcceptanceIndexer(t *testing.T, ctx context.Context, ctrl *gomock.Controller, store storage.Storage, releases []*indexer.Release) (MediaManager, *downloadMock.MockDownloadClient) {
	t.Helper()

	mockDownloadClient := downloadMock.NewMockDownloadClient(ctrl)
	mockFactory := downloadMock.NewMockFactory(ctrl)
	mockFactory.EXPECT().NewDownloadClient(gomock.Any()).Return(mockDownloadClient, nil).AnyTimes()

	mockIndexerSource := indexerMock.NewMockIndexerSource(ctrl)
	mockIndexerSource.EXPECT().Search(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(releases, nil).AnyTimes()
	mockIndexerSource.EXPECT().ListIndexers(gomock.Any()).Return([]indexer.SourceIndexer{{ID: 1, Name: "test", Priority: 1}}, nil).AnyTimes()

	indexerFactory := indexerMock.NewMockFactory(ctrl)
	indexerFactory.EXPECT().NewIndexerSource(gomock.Any()).Return(mockIndexerSource, nil).AnyTimes()

	sourceID, err := store.CreateIndexerSource(ctx, sqlcdb.IndexerSource{
		Name:           "test-source",
		Implementation: "prowlarr",
		Scheme:         "http",
		Host:           "test",
		Enabled:        true,
	})
	require.NoError(t, err)
	_, err = store.CreateIndexer(ctx, sqlcdb.Indexer{
		IndexerSourceID: sql.NullInt64{Int64: sourceID, Valid: true},
		Name:            "test",
		Priority:        1,
		Uri:             "http://test/indexer1",
	})
	require.NoError(t, err)

	m := New(nil, indexerFactory, nil, store, mockFactory, config.Manager{}, config.Config{})
	require.NoError(t, m.RefreshIndexerSource(ctx, sourceID))

	return m, mockDownloadClient
}

// Scheduled acquisition must find a monitored, aired, missing episode of a
// continuing or downloaded series independent of the parent aggregate state,
// including children created by a metadata refresh cycle. Unmonitored
// counterparts stay visible but are never enqueued.
func TestScheduledReconcileAcquiresAiredMonitoredEpisodeIndependentOfAggregateState(t *testing.T) {
	states := []struct {
		name        string
		seriesState storage.SeriesState
		seasonState storage.SeasonState
	}{
		{name: "downloaded parent", seriesState: storage.SeriesStateCompleted, seasonState: storage.SeasonStateCompleted},
		{name: "continuing parent", seriesState: storage.SeriesStateContinuing, seasonState: storage.SeasonStateContinuing},
	}

	for _, tc := range states {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()
			store := newStore(t, ctx)

			// Two newly refreshed children of an existing monitored season:
			// the aired one is monitored (inherits the season choice), the
			// aired sibling is unmonitored.
			_, seasonID := blockerAcceptanceTVFixtures(t, ctx, store, tc.seriesState, tc.seasonState, []struct {
				number    int64
				monitored int64
				future    bool
			}{
				{number: 1, monitored: 1},
				{number: 2, monitored: 0},
			})

			releases := []*indexer.Release{
				issue101EpisodeRelease(1, "Series.S01E01.1080p.WEB-DL-GROUP"),
				issue101EpisodeRelease(2, "Series.S01E02.1080p.WEB-DL-GROUP"),
			}
			m, client := blockerAcceptanceIndexer(t, ctx, ctrl, store, releases)
			// Exactly one external submission: the monitored aired episode.
			client.EXPECT().Add(gomock.Any(), gomock.Any()).Times(1).Return(download.Status{ID: "d-1", Name: "test download"}, nil)

			// One scheduled missing-series pass acquires the eligible child
			// with no manual state change and no aggregate dependency.
			clientModel := sqlcdb.DownloadClient{ID: 1, Implementation: "transmission", Type: "torrent", Host: "transmission", Port: 8080, Scheme: "http"}
			snapshot := newReconcileSnapshot(nil, []sqlcdb.DownloadClient{clientModel})
			require.NoError(t, m.ReconcileMissingSeries(ctx, snapshot))

			episodes, err := store.ListEpisodes(ctx, storage.EpisodeFilter{})
			require.NoError(t, err)
			require.Len(t, episodes, 2)
			byNumber := map[int64]sqlcdb.ListEpisodesRow{}
			for _, e := range episodes {
				byNumber[e.Episode.EpisodeNumber] = e
			}
			assert.Equal(t, string(storage.EpisodeStateDownloading), byNumber[1].State, "aired monitored episode must be acquired")
			assert.Equal(t, sql.NullString{String: "d-1", Valid: true}, byNumber[1].DownloadID)
			assert.Equal(t, string(storage.EpisodeStateMissing), byNumber[2].State, "unmonitored sibling must stay missing and never enqueue")
			assert.Equal(t, sql.NullString{}, byNumber[2].DownloadID)

			// The unmonitored sibling remains visible for the user.
			assert.Equal(t, int64(2), byNumber[2].Episode.EpisodeNumber)
			assert.Equal(t, int64(0), byNumber[2].Episode.Monitored)
			assert.NotZero(t, byNumber[2].Episode.ID)

			// A follow-up scheduled pass is a no-op for the acquired state.
			require.NoError(t, m.ReconcileMissingSeries(ctx, newReconcileSnapshot(nil, []sqlcdb.DownloadClient{clientModel})))
			episodes, err = store.ListEpisodes(ctx, storage.EpisodeFilter{})
			require.NoError(t, err)
			for _, e := range episodes {
				if e.Episode.EpisodeNumber == 1 {
					assert.Equal(t, string(storage.EpisodeStateDownloading), e.State)
				} else {
					assert.Equal(t, string(storage.EpisodeStateMissing), e.State)
				}
			}
			assert.NotZero(t, seasonID)
		})
	}
}

// Explicit request-once for a season containing both a released and a future
// monitored child must submit only the released child; the future child stays
// unqueued even on the individual fallback path, and a stale pack release
// must not sneak it in.
func TestSearchForSeasonOnlySubmitsReleasedMonitoredChildren(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	store := newStore(t, ctx)

	_, seasonID := blockerAcceptanceTVFixtures(t, ctx, store, storage.SeriesStateMissing, storage.SeasonStateMissing, []struct {
		number    int64
		monitored int64
		future    bool
	}{
		{number: 1, monitored: 1},
		{number: 2, monitored: 1, future: true},
	})

	releases := []*indexer.Release{
		// Season pack that would cover the future child too: must be rejected.
		issue101EpisodeRelease(1, "Series.S01.1080p.WEB-DL-GROUP"),
		issue101EpisodeRelease(2, "Series.S01E01.1080p.WEB-DL-GROUP"),
	}
	m, client := blockerAcceptanceIndexer(t, ctx, ctrl, store, releases)
	client.EXPECT().Add(gomock.Any(), gomock.Any()).Times(1).Return(download.Status{ID: "d-2", Name: "test download"}, nil)

	require.NoError(t, m.SearchForSeason(ctx, seasonID))

	episodes, err := store.ListEpisodes(ctx, storage.EpisodeFilter{})
	require.NoError(t, err)
	require.Len(t, episodes, 2)
	byNumber := map[int64]sqlcdb.ListEpisodesRow{}
	for _, e := range episodes {
		byNumber[e.Episode.EpisodeNumber] = e
	}
	assert.Equal(t, string(storage.EpisodeStateDownloading), byNumber[1].State, "released monitored child is submitted")
	assert.Equal(t, string(storage.EpisodeStateMissing), byNumber[2].State, "future monitored child must remain unqueued")
	assert.Equal(t, sql.NullString{}, byNumber[2].DownloadID)

	// Monitoring flags are never mutated by a request-once search.
	assert.Equal(t, int64(1), byNumber[2].Episode.Monitored)
}
