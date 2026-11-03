package manager

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/download"
	downloadMock "github.com/kasuboski/mediaz/pkg/download/mocks"
	"github.com/kasuboski/mediaz/pkg/indexer"
	indexerMock "github.com/kasuboski/mediaz/pkg/indexer/mocks"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	tmdbMocks "github.com/kasuboski/mediaz/pkg/tmdb/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func admissionFixture(t *testing.T) (context.Context, storage.Storage, sqlcdb.Series, sqlcdb.ListSeasonsRow, []sqlcdb.ListEpisodesRow) {
	t.Helper()
	ctx := context.Background()
	store := newStore(t, ctx)
	series, season, episodes := issue101TVFixtures(t, ctx, store, []struct {
		number    int64
		monitored int64
		override  sql.NullInt64
	}{{number: 1, monitored: 1}, {number: 2, monitored: 1}})
	return ctx, store, series, season, episodes
}

func TestTVAdmissionConcurrentAndCoverage(t *testing.T) {
	for _, pack := range []bool{false, true} {
		t.Run(map[bool]string{false: "manual_manual_scheduled", true: "pack_episode_overlap"}[pack], func(t *testing.T) {
			ctx, store, series, season, episodes := admissionFixture(t)
			ctrl := gomock.NewController(t)
			client := downloadMock.NewMockDownloadClient(ctrl)
			factory := downloadMock.NewMockFactory(ctrl)
			dc := sqlcdb.DownloadClient{ID: 2, Implementation: "transmission", Type: "torrent", Host: "transmission", Port: 8080, Scheme: "http"}
			// Losing contenders may perform local preflight, but cannot call Add.
			factory.EXPECT().NewDownloadClient(dc).Return(client, nil).AnyTimes()
			client.EXPECT().Add(gomock.Any(), gomock.Any()).Return(download.Status{ID: "job"}, nil).Times(1)
			m := New(nil, nil, nil, store, factory, config.Manager{}, config.Config{})
			snapshot := newReconcileSnapshot(nil, []sqlcdb.DownloadClient{dc})
			var wg sync.WaitGroup
			start := make(chan struct{})
			results := make(chan error, 3)
			for i := 0; i < 3; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					<-start
					scope := "episode"
					if i == 2 {
						scope = "scheduled"
					}
					target := episodes[:1]
					if pack && i == 0 {
						target = episodes
					}
					_, _, finish, err := m.admitTVRelease(withTVScope(ctx, scope), snapshot, issue101EpisodeRelease(1, "Series.S01E01"), series.QualityProfileID, season.Season.QualityProfileID, target)
					// Deliberately leave the successful claim uncertain, simulating a crash
					// before linkage. It must block every overlapping request thereafter.
					if err == nil {
						finish(false)
					}
					results <- err
				}(i)
			}
			close(start)
			wg.Wait()
			close(results)
			successes := 0
			for err := range results {
				if err == nil {
					successes++
				}
			}
			require.Equal(t, 1, successes)
		})
	}
}

func TestTVAdmissionDefiniteVersusUncertain(t *testing.T) {
	for _, definite := range []bool{false, true} {
		t.Run(map[bool]string{false: "uncertain_retained", true: "rejection_released"}[definite], func(t *testing.T) {
			ctx, store, series, season, episodes := admissionFixture(t)
			ctrl := gomock.NewController(t)
			client := downloadMock.NewMockDownloadClient(ctrl)
			factory := downloadMock.NewMockFactory(ctrl)
			dc := sqlcdb.DownloadClient{ID: 2, Implementation: "transmission", Type: "torrent", Host: "transmission", Port: 8080, Scheme: "http"}
			factory.EXPECT().NewDownloadClient(dc).Return(client, nil).AnyTimes()
			var submissionErr error = errors.New("connection lost after write")
			adds := 1
			if definite {
				submissionErr = &download.SubmissionRejectedError{Err: errors.New("rejected")}
				adds = 2
			}
			client.EXPECT().Add(gomock.Any(), gomock.Any()).Return(download.Status{}, submissionErr).Times(adds)
			m := New(nil, nil, nil, store, factory, config.Manager{}, config.Config{})
			snapshot := newReconcileSnapshot(nil, []sqlcdb.DownloadClient{dc})
			for i := 0; i < 2; i++ {
				_, _, _, err := m.admitTVRelease(ctx, snapshot, issue101EpisodeRelease(1, "Series.S01E01"), series.QualityProfileID, season.Season.QualityProfileID, episodes[:1])
				require.Error(t, err)
				if i == 0 && !definite {
					require.ErrorContains(t, err, "outcome uncertain")
				}
				if i == 1 && !definite {
					require.ErrorContains(t, err, "admission blocked")
				}
			}
		})
	}
}

func TestTVAdmissionRevalidatesSelectionAndProfile(t *testing.T) {
	for _, quality := range []bool{false, true} {
		t.Run(map[bool]string{false: "selection", true: "quality"}[quality], func(t *testing.T) {
			ctx, store, series, season, episodes := admissionFixture(t)
			patch := storage.SeriesSelectionPatch{}
			if quality {
				id := int64(5)
				patch.QualityProfileID = &id
			} else {
				off := false
				patch.Episodes = []storage.EpisodeSelectionPatch{{ID: episodes[0].Episode.ID, Monitored: &off}}
			}
			// Represents a completed save while an external search was in flight.
			require.NoError(t, store.ApplySeriesSelectionPatch(ctx, series.ID, patch))
			m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
			_, _, _, err := m.admitTVRelease(ctx, newReconcileSnapshot(nil, nil), issue101EpisodeRelease(1, "Series.S01E01"), series.QualityProfileID, season.Season.QualityProfileID, episodes[:1])
			require.ErrorContains(t, err, "changed during search")
		})
	}
}

func TestTVReservationAtomicRollbackAndActiveSeason(t *testing.T) {
	ctx, store, _, season, episodes := admissionFixture(t)
	a, b := episodes[0].Episode.ID, episodes[1].Episode.ID
	require.NoError(t, store.ReserveTVEpisodes(ctx, []int64{b}))
	require.Error(t, store.ReserveTVEpisodes(ctx, []int64{a, b}))
	// A failed pack claim must not leave its nonconflicting first episode held.
	require.NoError(t, store.ReserveTVEpisodes(ctx, []int64{a}))
	require.NoError(t, store.ReleaseTVEpisodes(ctx, []int64{a, b}))
	id := "pack"
	client := int64(2)
	require.NoError(t, store.UpdateSeasonState(ctx, season.Season.ID, storage.SeasonStateDownloading, &storage.TransitionStateMetadata{DownloadID: &id, DownloadClientID: &client}))
	require.ErrorContains(t, store.ReserveTVEpisodes(ctx, []int64{a}), "active season job")
}

func TestTVActiveDownloadSurvivesAutomationAndQualitySaves(t *testing.T) {
	ctx, store, series, season, episodes := admissionFixture(t)
	id := "existing"
	clientID := int64(2)
	require.NoError(t, store.UpdateEpisodeState(ctx, episodes[0].Episode.ID, storage.EpisodeStateDownloading, &storage.TransitionStateMetadata{DownloadID: &id, DownloadClientID: &clientID}))
	before, err := store.GetEpisodeByID(ctx, episodes[0].Episode.ID)
	require.NoError(t, err)
	ctrl := gomock.NewController(t)
	client := downloadMock.NewMockDownloadClient(ctrl)
	factory := downloadMock.NewMockFactory(ctrl)
	dc := sqlcdb.DownloadClient{ID: 2, Implementation: "transmission", Type: "torrent", Host: "transmission", Port: 8080, Scheme: "http"}
	factory.EXPECT().NewDownloadClient(dc).Return(client, nil)
	client.EXPECT().Get(gomock.Any(), download.GetRequest{ID: id}).Return(download.Status{ID: id, Done: false}, nil)
	m := New(nil, nil, nil, store, factory, config.Manager{}, config.Config{})
	_, err = m.UpdateSeriesMonitored(ctx, series.ID, false)
	require.NoError(t, err)
	quality := int64(5)
	require.NoError(t, store.ApplySeriesSelectionPatch(ctx, series.ID, storage.SeriesSelectionPatch{Seasons: []storage.SeasonSelectionPatch{{ID: season.Season.ID, QualityProfileID: &quality}}}))
	require.NoError(t, m.ReconcileDownloadingSeries(ctx, newReconcileSnapshot(nil, []sqlcdb.DownloadClient{dc})))
	after, err := store.GetEpisodeByID(ctx, episodes[0].Episode.ID)
	require.NoError(t, err)
	require.Equal(t, before, after)
}

func TestTVConcurrentManualAndSchedulerSearch(t *testing.T) {
	for _, change := range []string{"none", "selection", "quality"} {
		t.Run(change, func(t *testing.T) {
			ctx, store, series, _, episodes := admissionFixture(t)
			profile := int64(5)
			require.NoError(t, store.ApplySeriesSelectionPatch(ctx, series.ID, storage.SeriesSelectionPatch{QualityProfileID: &profile}))
			sourceID, err := store.CreateIndexerSource(ctx, sqlcdb.IndexerSource{Name: "fixture", Implementation: "prowlarr", Host: "fixture.invalid", Scheme: "http", Enabled: true})
			require.NoError(t, err)
			_, err = store.CreateIndexer(ctx, sqlcdb.Indexer{Name: "fixture", IndexerSourceID: sql.NullInt64{Int64: sourceID, Valid: true}, ProwlarrID: sql.NullInt64{Int64: 1, Valid: true}, Uri: "http://fixture.invalid/indexer"})
			require.NoError(t, err)
			ctrl := gomock.NewController(t)
			source := indexerMock.NewMockIndexerSource(ctrl)
			factory := indexerMock.NewMockFactory(ctrl)
			factory.EXPECT().NewIndexerSource(gomock.Any()).Return(source, nil).AnyTimes()
			source.EXPECT().ListIndexers(gomock.Any()).Return([]indexer.SourceIndexer{{ID: 1, Name: "fixture", URI: "http://fixture.invalid/indexer"}}, nil)
			arrived := make(chan struct{}, 3)
			release := make(chan struct{})
			source.EXPECT().Search(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, indexer.SourceID, []int32, indexer.SearchOptions) ([]*indexer.Release, error) {
				arrived <- struct{}{}
				<-release
				return []*indexer.Release{issue101EpisodeRelease(1, "Series.S01E01.1080p.WEB-DL-GROUP")}, nil
			}).Times(3)
			client := downloadMock.NewMockDownloadClient(ctrl)
			downloadFactory := downloadMock.NewMockFactory(ctrl)
			if change == "none" {
				downloadFactory.EXPECT().NewDownloadClient(gomock.Any()).Return(client, nil).Times(1)
				client.EXPECT().Add(gomock.Any(), gomock.Any()).Return(download.Status{ID: "one-job"}, nil).Times(1)
			}
			tmdbClient := tmdbMocks.NewMockITmdb(ctrl)
			if change != "none" {
				tmdbClient.EXPECT().TvSeriesDetails(gomock.Any(), int32(1), nil).Return(&http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{}`))}, nil)
			}
			m := New(tmdbClient, factory, nil, store, downloadFactory, config.Manager{}, config.Config{})
			require.NoError(t, m.RefreshIndexerSource(ctx, sourceID))
			var wg sync.WaitGroup
			for i := 0; i < 3; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					if i < 2 {
						_ = m.SearchForEpisode(ctx, episodes[0].Episode.ID)
					} else {
						dcs, err := store.ListDownloadClients(ctx)
						if err != nil {
							return
						}
						_ = m.ReconcileMissingSeries(ctx, newReconcileSnapshot(nil, dcs))
					}
				}(i)
			}
			for i := 0; i < 3; i++ {
				select {
				case <-arrived:
				case <-time.After(5 * time.Second):
					close(release)
					wg.Wait()
					t.Fatal("search did not reach provider barrier")
				}
			}
			if change != "none" {
				patch := SeriesSelectionPatch{}
				if change == "quality" {
					id := int64(6)
					patch.QualityProfileID = &id
				} else {
					off := false
					patch.Episodes = []EpisodeSelectionUpdate{{ID: episodes[0].Episode.ID, Monitored: &off}}
				}
				_, err := m.UpdateSeriesSelection(ctx, series.ID, patch)
				require.NoError(t, err)
			}
			close(release)
			wg.Wait()
			current, err := store.GetEpisodeByID(ctx, episodes[0].Episode.ID)
			require.NoError(t, err)
			expected := sqlcdb.GetEpisodeByIDRow{Episode: episodes[0].Episode, State: sql.NullString{String: "missing", Valid: true}}
			if change == "selection" {
				expected.Episode.Monitored = 0
			}
			if change == "none" {
				expected.State.String = "downloading"
				expected.DownloadID = sql.NullString{String: "one-job", Valid: true}
				expected.DownloadClientID = sql.NullInt64{Int64: 1, Valid: true}
			}
			require.Equal(t, expected, current)
		})
	}
}
