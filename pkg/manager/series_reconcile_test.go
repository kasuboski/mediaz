package manager

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/download"
	downloadMock "github.com/kasuboski/mediaz/pkg/download/mocks"
	"github.com/kasuboski/mediaz/pkg/indexer"
	indexerMock "github.com/kasuboski/mediaz/pkg/indexer/mocks"
	"github.com/kasuboski/mediaz/pkg/library"
	libraryMocks "github.com/kasuboski/mediaz/pkg/library/mocks"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/ptr"
	"github.com/kasuboski/mediaz/pkg/storage"
	storageMocks "github.com/kasuboski/mediaz/pkg/storage/mocks"
	sqlcdb "github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	mediaSqlite "github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/kasuboski/mediaz/pkg/tmdb"
	tmdbMocks "github.com/kasuboski/mediaz/pkg/tmdb/mocks"
	"github.com/oapi-codegen/nullable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestMediaManager_updateEpisodeState(t *testing.T) {
	t.Run("update episode state", func(t *testing.T) {
		ctx := context.Background()
		store, err := mediaSqlite.New(ctx, ":memory:")
		require.NoError(t, err)

		seriesMetadataID, err := store.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{TmdbID: 1, Title: "Series"})
		require.NoError(t, err)
		seriesID, err := store.CreateSeries(ctx, sqlcdb.Series{QualityProfileID: 1, SeriesMetadataID: sql.NullInt64{Int64: seriesMetadataID, Valid: true}}, storage.SeriesStateMissing)
		require.NoError(t, err)
		seasonMetadataID, err := store.CreateSeasonMetadata(ctx, sqlcdb.SeasonMetadatum{SeriesMetadataID: seriesMetadataID, Title: "Season 1", Number: 1})
		require.NoError(t, err)
		seasonID, err := store.CreateSeason(ctx, sqlcdb.Season{SeriesID: seriesID, SeasonNumber: 1, SeasonMetadataID: sql.NullInt64{Int64: seasonMetadataID, Valid: true}}, storage.SeasonStateMissing)
		require.NoError(t, err)
		downloadClientID, err := store.CreateDownloadClient(ctx, sqlcdb.DownloadClient{Implementation: "transmission", Type: "torrent", Host: "localhost", Port: 9091, Scheme: "http"})
		require.NoError(t, err)
		episodeMetadataID, err := store.CreateEpisodeMetadata(ctx, sqlcdb.EpisodeMetadatum{TmdbID: 1, Title: "Episode 1", Number: 1, SeasonMetadataID: seasonMetadataID})
		require.NoError(t, err)
		episode := sqlcdb.Episode{SeasonID: seasonID, EpisodeNumber: 1, EpisodeMetadataID: sql.NullInt64{Int64: episodeMetadataID, Valid: true}}
		episodeID, err := store.CreateEpisode(ctx, episode, storage.EpisodeStateMissing)
		require.NoError(t, err)

		manager := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		err = manager.updateEpisodeState(ctx, sqlcdb.ListEpisodesRow{Episode: sqlcdb.Episode{ID: episodeID, SeasonID: seasonID, EpisodeNumber: 1}, State: string(storage.EpisodeStateMissing)}, storage.EpisodeStateDownloading, &storage.TransitionStateMetadata{
			DownloadID: ptr.To("123"), DownloadClientID: ptr.To(downloadClientID), IsEntireSeasonDownload: ptr.To(true),
		})
		require.NoError(t, err)

		foundEpisode, err := store.GetEpisodeByID(ctx, episodeID)
		require.NoError(t, err)
		assert.Equal(t, sql.NullString{String: string(storage.EpisodeStateDownloading), Valid: true}, foundEpisode.State)
		assert.Equal(t, sql.NullInt64{Int64: downloadClientID, Valid: true}, foundEpisode.DownloadClientID)
		assert.Equal(t, sql.NullString{String: "123", Valid: true}, foundEpisode.DownloadID)
		assert.Equal(t, seasonID, foundEpisode.Episode.SeasonID)
		assert.Equal(t, int64(1), foundEpisode.Episode.EpisodeNumber)
	})
}

func TestMediaManager_reconcileMissingEpisodes(t *testing.T) {
	t.Run("reconcile missing episodes - not all released", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		ctx := context.Background()
		store := newTVReconcileStore(t, ctx, true)

		mockDownloadClient := downloadMock.NewMockDownloadClient(ctrl)
		mockFactory := downloadMock.NewMockFactory(ctrl)
		downloadClientModel := sqlcdb.DownloadClient{
			ID:             2,
			Implementation: "transmission",
			Type:           "torrent",
			Port:           8080,
			Host:           "transmission",
			Scheme:         "http",
		}
		_, errClient := store.CreateDownloadClient(ctx, downloadClientModel)
		require.NoError(t, errClient)
		_, errClient = store.CreateDownloadClient(ctx, downloadClientModel)
		require.NoError(t, errClient)

		snapshot := newReconcileSnapshot([]sqlcdb.Indexer{{ID: 1}}, []sqlcdb.DownloadClient{downloadClientModel})

		mockFactory.EXPECT().NewDownloadClient(downloadClientModel).Return(mockDownloadClient, nil)
		mockFactory.EXPECT().NewDownloadClient(downloadClientModel).Return(mockDownloadClient, nil)
		mockDownloadClient.EXPECT().Add(ctx, gomock.Any()).Return(download.Status{
			ID:   "123",
			Name: "test download",
		}, nil)
		mockDownloadClient.EXPECT().Add(ctx, gomock.Any()).Return(download.Status{
			ID:   "124",
			Name: "test download",
		}, nil)

		episodeMetadata := sqlcdb.EpisodeMetadatum{SeasonMetadataID: 1,
			TmdbID: 1,
			Title:  "Test Episode",
			Number: 1,
			// time in the past
			AirDate: sql.NullTime{Time: snapshot.time.Add(time.Hour * -2), Valid: true},
			Runtime: sql.NullInt64{Int64: 42, Valid: true},
		}

		metadataID1, err := store.CreateEpisodeMetadata(ctx, episodeMetadata)
		require.NoError(t, err)

		episode1 := sqlcdb.ListEpisodesRow{
			Episode: sqlcdb.Episode{
				SeasonID:          1,
				EpisodeNumber:     1,
				EpisodeMetadataID: sql.NullInt64{Int64: metadataID1, Valid: true},
			},
		}

		_, err = store.CreateEpisode(ctx, episode1.Episode, storage.EpisodeStateMissing)
		require.NoError(t, err)

		episodeMetadata = sqlcdb.EpisodeMetadatum{SeasonMetadataID: 1,
			TmdbID: 2,
			Title:  "Test Episode 2",
			Number: 2,
			// time in the past
			AirDate: sql.NullTime{Time: snapshot.time.Add(time.Hour * -2), Valid: true},
			Runtime: sql.NullInt64{Int64: 42, Valid: true},
		}

		metadataID2, err := store.CreateEpisodeMetadata(ctx, episodeMetadata)
		require.NoError(t, err)

		episode2 := sqlcdb.ListEpisodesRow{
			Episode: sqlcdb.Episode{
				SeasonID:          1,
				EpisodeNumber:     2,
				EpisodeMetadataID: sql.NullInt64{Int64: metadataID2, Valid: true},
			},
		}
		_, err = store.CreateEpisode(ctx, episode2.Episode, storage.EpisodeStateMissing)
		require.NoError(t, err)

		episodeMetadata = sqlcdb.EpisodeMetadatum{SeasonMetadataID: 1,
			TmdbID: 3,
			Title:  "Test Episode 3",
			Number: 3,
			// future time
			AirDate: sql.NullTime{Time: snapshot.time.Add(time.Hour * 2), Valid: true},
			Runtime: sql.NullInt64{Int64: 42, Valid: true},
		}

		metadataID3, err := store.CreateEpisodeMetadata(ctx, episodeMetadata)
		require.NoError(t, err)

		episode3 := sqlcdb.ListEpisodesRow{
			Episode: sqlcdb.Episode{
				SeasonID:          1,
				EpisodeNumber:     3,
				EpisodeMetadataID: sql.NullInt64{Int64: metadataID3, Valid: true},
			},
		}

		_, err = store.CreateEpisode(ctx, episode3.Episode, storage.EpisodeStateMissing)
		require.NoError(t, err)

		releases := []*prowlarr.ReleaseResource{
			{
				ID:       ptr.To(int32(1)),
				Title:    nullable.NewNullableWithValue("Series.S01E01.1080p.WEB-DL.AAC2.0.x264-GROUP"),
				Size:     sizeGBToBytes(2),
				Protocol: ptr.To(prowlarr.DownloadProtocolTorrent),
			},
			{
				ID:       ptr.To(int32(2)),
				Title:    nullable.NewNullableWithValue("Series.S01E02.1080p.WEB-DL.AAC2.0.x264-GROUP"),
				Size:     sizeGBToBytes(2),
				Protocol: ptr.To(prowlarr.DownloadProtocolTorrent),
			},
		}

		qualityProfile := []sqlcdb.ListQualityProfileRowsRow{{QualityDefinition: sqlcdb.QualityDefinition{
			Name:          "HD",
			MinSize:       0,
			MaxSize:       2000,
			PreferredSize: 1000,
			MediaType:     "tv",
		}}}

		episodes, err := store.ListEpisodes(ctx, storage.EpisodeFilter{})
		require.NoError(t, err)
		require.Len(t, episodes, 3)

		m := New(nil, nil, nil, store, mockFactory, config.Manager{}, config.Config{})

		err = m.reconcileMissingEpisodes(ctx, "Series", 1, 1, episodes, snapshot, qualityProfile, releases)
		require.NoError(t, err)

		episodes, err = store.ListEpisodes(ctx, storage.EpisodeFilter{})
		require.NoError(t, err)
		require.Len(t, episodes, 3)

		slices.SortFunc(episodes, func(a, b sqlcdb.ListEpisodesRow) int {
			if a.Episode.ID < b.Episode.ID {
				return -1
			}
			if a.Episode.ID > b.Episode.ID {
				return 1
			}
			return 0
		})

		assert.Equal(t, string(storage.EpisodeStateDownloading), episodes[0].State)
		assert.Equal(t, sql.NullString{String: "123", Valid: true}, episodes[0].DownloadID)
		assert.Equal(t, sql.NullInt64{Int64: 2, Valid: true}, episodes[0].DownloadClientID)

		assert.Equal(t, string(storage.EpisodeStateDownloading), episodes[1].State)
		assert.Equal(t, sql.NullString{String: "124", Valid: true}, episodes[1].DownloadID)
		assert.Equal(t, sql.NullInt64{Int64: 2, Valid: true}, episodes[1].DownloadClientID)

		assert.Equal(t, string(storage.EpisodeStateMissing), episodes[2].State)
		assert.Equal(t, sql.NullString{}, episodes[2].DownloadID)
		assert.Equal(t, sql.NullInt64{Int64: 0, Valid: false}, episodes[2].DownloadClientID)
	})

	t.Run("reconcile missing episodes - all released", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		ctx := context.Background()
		store := newTVReconcileStore(t, ctx, false)

		mockDownloadClient := downloadMock.NewMockDownloadClient(ctrl)
		mockFactory := downloadMock.NewMockFactory(ctrl)
		downloadClientModel := sqlcdb.DownloadClient{
			ID:             2,
			Implementation: "transmission",
			Type:           "torrent",
			Port:           8080,
			Host:           "transmission",
			Scheme:         "http",
		}
		_, errClient := store.CreateDownloadClient(ctx, downloadClientModel)
		require.NoError(t, errClient)
		_, errClient = store.CreateDownloadClient(ctx, downloadClientModel)
		require.NoError(t, errClient)

		snapshot := newReconcileSnapshot([]sqlcdb.Indexer{{ID: 1}}, []sqlcdb.DownloadClient{downloadClientModel})

		mockFactory.EXPECT().NewDownloadClient(downloadClientModel).Return(mockDownloadClient, nil)
		mockFactory.EXPECT().NewDownloadClient(downloadClientModel).Return(mockDownloadClient, nil)
		mockDownloadClient.EXPECT().Add(ctx, gomock.Any()).Return(download.Status{
			ID:   "123",
			Name: "test download",
		}, nil)
		mockDownloadClient.EXPECT().Add(ctx, gomock.Any()).Return(download.Status{
			ID:   "124",
			Name: "test download",
		}, nil)

		seasonID, err := store.CreateSeason(ctx, sqlcdb.Season{
			SeriesID:         1,
			SeasonMetadataID: sql.NullInt64{Int64: 1, Valid: true},
			Monitored:        1,
		}, storage.SeasonStateMissing)
		require.NoError(t, err)

		episodeMetadata := sqlcdb.EpisodeMetadatum{SeasonMetadataID: 1,
			TmdbID: 1,
			Title:  "Test Episode",
			Number: 1,
			// time in the past
			AirDate: sql.NullTime{Time: snapshot.time.Add(time.Hour * -2), Valid: true},
			Runtime: sql.NullInt64{Int64: 42, Valid: true},
		}

		metadataID1, err := store.CreateEpisodeMetadata(ctx, episodeMetadata)
		require.NoError(t, err)

		episode1 := sqlcdb.ListEpisodesRow{
			Episode: sqlcdb.Episode{
				SeasonID:          seasonID,
				EpisodeNumber:     1,
				EpisodeMetadataID: sql.NullInt64{Int64: metadataID1, Valid: true},
			},
		}

		_, err = store.CreateEpisode(ctx, episode1.Episode, storage.EpisodeStateMissing)
		require.NoError(t, err)

		episodeMetadata = sqlcdb.EpisodeMetadatum{SeasonMetadataID: 1,
			TmdbID: 2,
			Title:  "Test Episode 2",
			Number: 2,
			// time in the past
			AirDate: sql.NullTime{Time: snapshot.time.Add(time.Hour * -2), Valid: true},
			Runtime: sql.NullInt64{Int64: 42, Valid: true},
		}

		metadataID2, err := store.CreateEpisodeMetadata(ctx, episodeMetadata)
		require.NoError(t, err)

		episode2 := sqlcdb.ListEpisodesRow{
			Episode: sqlcdb.Episode{
				SeasonID:          seasonID,
				EpisodeNumber:     2,
				EpisodeMetadataID: sql.NullInt64{Int64: metadataID2, Valid: true},
			},
		}
		_, err = store.CreateEpisode(ctx, episode2.Episode, storage.EpisodeStateMissing)
		require.NoError(t, err)

		releases := []*prowlarr.ReleaseResource{
			{
				ID:       ptr.To(int32(1)),
				Title:    nullable.NewNullableWithValue("Series.S01E01.1080p.WEB-DL.AAC2.0.x264-GROUP"),
				Size:     sizeGBToBytes(2),
				Protocol: ptr.To(prowlarr.DownloadProtocolTorrent),
			},
			{
				ID:       ptr.To(int32(2)),
				Title:    nullable.NewNullableWithValue("Series.S01E02.1080p.WEB-DL.AAC2.0.x264-GROUP"),
				Size:     sizeGBToBytes(2),
				Protocol: ptr.To(prowlarr.DownloadProtocolTorrent),
			},
		}

		qualityProfile := []sqlcdb.ListQualityProfileRowsRow{{QualityDefinition: sqlcdb.QualityDefinition{
			Name:          "HD",
			MinSize:       0,
			MaxSize:       2000,
			PreferredSize: 1000,
			MediaType:     "tv",
		}}}

		episodes, err := store.ListEpisodes(ctx, storage.EpisodeFilter{})
		require.NoError(t, err)
		require.Len(t, episodes, 2)

		m := New(nil, nil, nil, store, mockFactory, config.Manager{}, config.Config{})

		err = m.reconcileMissingEpisodes(ctx, "Series", seasonID, 1, episodes, snapshot, qualityProfile, releases)
		require.NoError(t, err)

		episodes, err = store.ListEpisodes(ctx, storage.EpisodeFilter{})
		require.NoError(t, err)
		require.Len(t, episodes, 2)

		slices.SortFunc(episodes, func(a, b sqlcdb.ListEpisodesRow) int {
			if a.Episode.ID < b.Episode.ID {
				return -1
			}
			if a.Episode.ID > b.Episode.ID {
				return 1
			}
			return 0
		})

		assert.Equal(t, string(storage.EpisodeStateDownloading), episodes[0].State)
		assert.Equal(t, sql.NullString{String: "123", Valid: true}, episodes[0].DownloadID)
		assert.Equal(t, sql.NullInt64{Int64: 2, Valid: true}, episodes[0].DownloadClientID)

		assert.Equal(t, string(storage.EpisodeStateDownloading), episodes[1].State)
		assert.Equal(t, sql.NullString{String: "124", Valid: true}, episodes[1].DownloadID)
		assert.Equal(t, sql.NullInt64{Int64: 2, Valid: true}, episodes[1].DownloadClientID)

		seasons, err := store.ListSeasons(ctx, storage.SeasonFilter{})
		require.NoError(t, err)
		require.Len(t, seasons, 1)
		assert.Equal(t, string(storage.SeasonStateDownloading), seasons[0].State)
	})

	t.Run("empty episode with nil snapshot", func(t *testing.T) {
		m := New(nil, nil, nil, nil, nil, config.Manager{}, config.Config{})
		err := m.reconcileMissingEpisodes(context.Background(), "Series", 1, 1, []sqlcdb.ListEpisodesRow{{}}, nil, nil, nil)
		require.NoError(t, err)
	})

	t.Run("nil snapshot", func(t *testing.T) {
		m := New(nil, nil, nil, nil, nil, config.Manager{}, config.Config{})
		episode := sqlcdb.ListEpisodesRow{}
		err := m.reconcileMissingEpisodes(context.Background(), "Series", 1, 1, []sqlcdb.ListEpisodesRow{episode}, nil, nil, nil)
		require.NoError(t, err)
	})

	t.Run("nil runtime", func(t *testing.T) {
		ctx := context.Background()
		store := newTVReconcileStore(t, ctx, true)

		episodeMetadata := sqlcdb.EpisodeMetadatum{SeasonMetadataID: 1,
			ID:      1,
			Title:   "Test Episode",
			Number:  1,
			Runtime: sql.NullInt64{},
		}

		_, err := store.CreateEpisodeMetadata(ctx, episodeMetadata)
		require.NoError(t, err)

		episode := sqlcdb.ListEpisodesRow{
			Episode: sqlcdb.Episode{
				EpisodeMetadataID: sql.NullInt64{Int64: 1, Valid: true},
			},
		}

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		err = m.reconcileMissingEpisodes(ctx, "Series", 1, 1, []sqlcdb.ListEpisodesRow{episode}, &ReconcileSnapshot{}, nil, nil)
		require.NoError(t, err)
	})

	t.Run("no matching releases", func(t *testing.T) {
		ctx := context.Background()
		store := newTVReconcileStore(t, ctx, true)

		episodeMetadata := sqlcdb.EpisodeMetadatum{SeasonMetadataID: 1,
			ID:     1,
			Title:  "Test Episode",
			Number: 1,
		}

		_, err := store.CreateEpisodeMetadata(ctx, episodeMetadata)
		require.NoError(t, err)

		episode := sqlcdb.ListEpisodesRow{
			Episode: sqlcdb.Episode{
				EpisodeMetadataID: sql.NullInt64{Int64: 1, Valid: true},
			},
		}

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		err = m.reconcileMissingEpisodes(ctx, "Series", 1, 1, []sqlcdb.ListEpisodesRow{episode}, &ReconcileSnapshot{}, nil, nil)
		require.NoError(t, err)
	})
}
func TestMediaManager_reconcileMissingSeason(t *testing.T) {
	t.Run("season without metadata", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)
		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		err := m.reconcileMissingSeason(ctx, "Series", sqlcdb.ListSeasonsRow{}, nil, nil, nil)
		require.ErrorIs(t, err, storage.ErrNotFound)
	})

	t.Run("missing season metadata", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)

		season := sqlcdb.ListSeasonsRow{
			Season: sqlcdb.Season{
				ID:               1,
				SeriesID:         1,
				SeasonMetadataID: sql.NullInt64{Int64: 999, Valid: true}, // Non-existent metadata ID
			},
		}

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		err := m.reconcileMissingSeason(ctx, "Series", season, nil, nil, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found in storage")
	})

	t.Run("no episodes", func(t *testing.T) {
		ctx := context.Background()
		store := newTVReconcileStore(t, ctx, false)

		season := sqlcdb.ListSeasonsRow{
			Season: sqlcdb.Season{
				ID:               1,
				SeriesID:         1,
				SeasonMetadataID: sql.NullInt64{Int64: 1, Valid: true},
			},
		}

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		err := m.reconcileMissingSeason(ctx, "Series", season, nil, nil, nil)
		require.NoError(t, err)
	})

	t.Run("successful season pack reconciliation", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		ctx := context.Background()
		store := newStore(t, ctx)

		mockDownloadClient := downloadMock.NewMockDownloadClient(ctrl)
		mockFactory := downloadMock.NewMockFactory(ctrl)

		downloadClientModel := sqlcdb.DownloadClient{
			ID:             2,
			Implementation: "transmission",
			Type:           "torrent",
			Port:           8080,
			Host:           "transmission",
			Scheme:         "http",
		}
		_, errClient := store.CreateDownloadClient(ctx, downloadClientModel)
		require.NoError(t, errClient)
		_, errClient = store.CreateDownloadClient(ctx, downloadClientModel)
		require.NoError(t, errClient)

		mockFactory.EXPECT().NewDownloadClient(downloadClientModel).Return(mockDownloadClient, nil)
		mockDownloadClient.EXPECT().Add(ctx, gomock.Any()).Return(download.Status{
			ID:   "123",
			Name: "test download",
		}, nil)

		seriesMetadataID, err := store.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{
			TmdbID:         1,
			Title:          "Series",
			Status:         "Continuing",
			EpisodeCount:   10,
			ExternalIds:    sql.NullString{},
			WatchProviders: sql.NullString{},
		})
		require.NoError(t, err)

		seriesID, err := store.CreateSeries(ctx, sqlcdb.Series{QualityProfileID: 1,
			ID:               1,
			SeriesMetadataID: sql.NullInt64{Int64: seriesMetadataID, Valid: true},
		}, storage.SeriesStateMissing)
		require.NoError(t, err)

		_, err = store.GetSeriesByID(ctx, seriesID)
		require.NoError(t, err)

		seasonMetadataID, err := store.CreateSeasonMetadata(ctx, sqlcdb.SeasonMetadatum{
			SeriesMetadataID: seriesMetadataID,
			Title:            "Season 1",
			Number:           1,
		})
		require.NoError(t, err)

		seasonID, err := store.CreateSeason(ctx, sqlcdb.Season{
			SeriesID:         1,
			SeasonMetadataID: sql.NullInt64{Int64: 1, Valid: true},
		}, storage.SeasonStateMissing)
		require.NoError(t, err)

		season, err := store.GetSeasonByID(ctx, seasonID)
		require.NoError(t, err)
		assert.Equal(t, int64(1), season.Season.SeriesID)
		assert.Equal(t, sql.NullInt64{Int64: 1, Valid: true}, season.Season.SeasonMetadataID)

		_, err = store.GetSeasonMetadataByID(ctx, int64(seasonMetadataID))
		require.NoError(t, err)

		episodeMetadataID1, err := store.CreateEpisodeMetadata(ctx, sqlcdb.EpisodeMetadatum{
			TmdbID:           1,
			Title:            "Hello",
			Number:           1,
			SeasonMetadataID: seasonMetadataID,
			AirDate:          sql.NullTime{Time: time.Now().Add(time.Hour * -2), Valid: true},
			Runtime:          sql.NullInt64{Int64: 42, Valid: true},
		})
		require.NoError(t, err)

		_, err = store.CreateEpisode(ctx, sqlcdb.Episode{
			SeasonID:          seasonID,
			EpisodeNumber:     1,
			EpisodeMetadataID: sql.NullInt64{Int64: episodeMetadataID1, Valid: true},
		}, storage.EpisodeStateMissing)
		require.NoError(t, err)

		episodeMetadataID2, err := store.CreateEpisodeMetadata(ctx, sqlcdb.EpisodeMetadatum{
			TmdbID:           2,
			Title:            "There",
			Number:           2,
			SeasonMetadataID: seasonMetadataID,
			AirDate:          sql.NullTime{Time: time.Now().Add(time.Hour * -2), Valid: true},
			Runtime:          sql.NullInt64{Int64: 42, Valid: true},
		})
		require.NoError(t, err)

		_, err = store.CreateEpisode(ctx, sqlcdb.Episode{
			SeasonID:          seasonID,
			EpisodeNumber:     2,
			EpisodeMetadataID: sql.NullInt64{Int64: episodeMetadataID2, Valid: true},
		}, storage.EpisodeStateMissing)
		require.NoError(t, err)

		releases := []*prowlarr.ReleaseResource{
			{
				ID:       ptr.To(int32(1)),
				Title:    nullable.NewNullableWithValue("series.S01.1080p.WEB-DL.HEVC.x265"),
				Size:     sizeGBToBytes(2),
				Protocol: ptr.To(prowlarr.DownloadProtocolTorrent),
			},
		}

		qualityProfile := []sqlcdb.ListQualityProfileRowsRow{{QualityDefinition: sqlcdb.QualityDefinition{
			Name:          "HD",
			MinSize:       15,
			MaxSize:       1000,
			PreferredSize: 995,
			MediaType:     "tv",
		}}}

		snapshot := newReconcileSnapshot([]sqlcdb.Indexer{{ID: 1}}, []sqlcdb.DownloadClient{downloadClientModel})

		m := New(nil, nil, nil, store, mockFactory, config.Manager{}, config.Config{})

		err = m.reconcileMissingSeason(ctx, "Series", seasonListRow(season), snapshot, qualityProfile, releases)
		require.NoError(t, err)

		episodes, err := store.ListEpisodes(ctx, storage.EpisodeFilter{})
		require.NoError(t, err)
		require.Len(t, episodes, 2)

		assert.Equal(t, string(storage.EpisodeStateDownloading), episodes[0].State)
		assert.Equal(t, sql.NullString{String: "123", Valid: true}, episodes[0].DownloadID)
		assert.Equal(t, sql.NullInt64{Int64: 2, Valid: true}, episodes[0].DownloadClientID)

		assert.Equal(t, string(storage.EpisodeStateDownloading), episodes[1].State)
		assert.Equal(t, sql.NullString{String: "123", Valid: true}, episodes[1].DownloadID)
		assert.Equal(t, sql.NullInt64{Int64: 2, Valid: true}, episodes[1].DownloadClientID)
	})

	t.Run("successful individual episode reconciliation", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		ctx := context.Background()
		store := newStore(t, ctx)

		mockDownloadClient := downloadMock.NewMockDownloadClient(ctrl)
		mockFactory := downloadMock.NewMockFactory(ctrl)

		downloadClientModel := sqlcdb.DownloadClient{
			ID:             2,
			Implementation: "transmission",
			Type:           "torrent",
			Port:           8080,
			Host:           "transmission",
			Scheme:         "http",
		}
		_, errClient := store.CreateDownloadClient(ctx, downloadClientModel)
		require.NoError(t, errClient)
		_, errClient = store.CreateDownloadClient(ctx, downloadClientModel)
		require.NoError(t, errClient)

		snapshot := newReconcileSnapshot([]sqlcdb.Indexer{{ID: 1}}, []sqlcdb.DownloadClient{downloadClientModel})

		mockFactory.EXPECT().NewDownloadClient(downloadClientModel).Return(mockDownloadClient, nil)
		mockFactory.EXPECT().NewDownloadClient(downloadClientModel).Return(mockDownloadClient, nil)
		mockDownloadClient.EXPECT().Add(ctx, gomock.Any()).Return(download.Status{
			ID:   "123",
			Name: "test download",
		}, nil)
		mockDownloadClient.EXPECT().Add(ctx, gomock.Any()).Return(download.Status{
			ID:   "124",
			Name: "test download 2",
		}, nil)

		seriesMetadataID, err := store.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{
			TmdbID:         1,
			Title:          "Series",
			Status:         "Continuing",
			EpisodeCount:   10,
			ExternalIds:    sql.NullString{},
			WatchProviders: sql.NullString{},
		})
		require.NoError(t, err)

		seriesID, err := store.CreateSeries(ctx, sqlcdb.Series{QualityProfileID: 1,
			ID:               1,
			SeriesMetadataID: sql.NullInt64{Int64: seriesMetadataID, Valid: true},
		}, storage.SeriesStateMissing)
		require.NoError(t, err)

		_, err = store.GetSeriesByID(ctx, seriesID)
		require.NoError(t, err)

		seasonMetadataID, err := store.CreateSeasonMetadata(ctx, sqlcdb.SeasonMetadatum{
			SeriesMetadataID: seriesMetadataID,
			Title:            "Season 1",
			Number:           1,
		})
		require.NoError(t, err)

		seasonID, err := store.CreateSeason(ctx, sqlcdb.Season{
			SeriesID:         1,
			SeasonMetadataID: sql.NullInt64{Int64: 1, Valid: true},
		}, storage.SeasonStateMissing)
		require.NoError(t, err)

		season, err := store.GetSeasonByID(ctx, seasonID)
		require.NoError(t, err)
		assert.Equal(t, int64(1), season.Season.SeriesID)
		assert.Equal(t, sql.NullInt64{Int64: 1, Valid: true}, season.Season.SeasonMetadataID)

		_, err = store.GetSeasonMetadataByID(ctx, int64(seasonMetadataID))
		require.NoError(t, err)

		episodeMetadataID1, err := store.CreateEpisodeMetadata(ctx, sqlcdb.EpisodeMetadatum{
			TmdbID:           1,
			Title:            "Test",
			Number:           1,
			SeasonMetadataID: seasonMetadataID,
			AirDate:          sql.NullTime{Time: snapshot.time.Add(time.Hour * -2), Valid: true},
			Runtime:          sql.NullInt64{Int64: 42, Valid: true},
		})
		require.NoError(t, err)

		_, err = store.CreateEpisode(ctx, sqlcdb.Episode{
			SeasonID:          seasonID,
			EpisodeNumber:     1,
			EpisodeMetadataID: sql.NullInt64{Int64: episodeMetadataID1, Valid: true},
		}, storage.EpisodeStateMissing)
		require.NoError(t, err)

		episodeMetadataID2, err := store.CreateEpisodeMetadata(ctx, sqlcdb.EpisodeMetadatum{
			TmdbID:           2,
			Title:            "Testing",
			Number:           2,
			SeasonMetadataID: seasonMetadataID,
			AirDate:          sql.NullTime{Time: snapshot.time.Add(time.Hour * -2), Valid: true},
			Runtime:          sql.NullInt64{Int64: 42, Valid: true},
		})
		require.NoError(t, err)

		_, err = store.CreateEpisode(ctx, sqlcdb.Episode{
			SeasonID:          seasonID,
			EpisodeNumber:     2,
			EpisodeMetadataID: sql.NullInt64{Int64: episodeMetadataID2, Valid: true},
		}, storage.EpisodeStateMissing)
		require.NoError(t, err)

		releases := []*prowlarr.ReleaseResource{
			{
				ID:       ptr.To(int32(1)),
				Title:    nullable.NewNullableWithValue("Series.S01E01.1080p.WEB-DL.AAC2.0.x264-GROUP"),
				Size:     sizeGBToBytes(2),
				Protocol: ptr.To(prowlarr.DownloadProtocolTorrent),
			},
			{
				ID:       ptr.To(int32(2)),
				Title:    nullable.NewNullableWithValue("Series.S01E02.1080p.WEB-DL.AAC2.0.x264-GROUP"),
				Size:     sizeGBToBytes(2),
				Protocol: ptr.To(prowlarr.DownloadProtocolTorrent),
			},
		}

		qualityProfile := []sqlcdb.ListQualityProfileRowsRow{{QualityDefinition: sqlcdb.QualityDefinition{
			Name:          "HD",
			MinSize:       15,
			MaxSize:       1000,
			PreferredSize: 995,
			MediaType:     "tv",
		}}}

		m := New(nil, nil, nil, store, mockFactory, config.Manager{}, config.Config{})

		err = m.reconcileMissingSeason(ctx, "Series", seasonListRow(season), snapshot, qualityProfile, releases)
		require.NoError(t, err)

		episodes, err := store.ListEpisodes(ctx, storage.EpisodeFilter{})
		require.NoError(t, err)
		require.Len(t, episodes, 2)

		assert.Equal(t, string(storage.EpisodeStateDownloading), episodes[0].State)
		assert.Equal(t, sql.NullString{String: "123", Valid: true}, episodes[0].DownloadID)
		assert.Equal(t, sql.NullInt64{Int64: 2, Valid: true}, episodes[0].DownloadClientID)

		assert.Equal(t, string(storage.EpisodeStateDownloading), episodes[1].State)
		assert.Equal(t, sql.NullString{String: "124", Valid: true}, episodes[1].DownloadID)
		assert.Equal(t, sql.NullInt64{Int64: 2, Valid: true}, episodes[1].DownloadClientID)
	})
}
func TestMediaManager_ReconcileMissingSeries(t *testing.T) {
	t.Run("nil snapshot", func(t *testing.T) {
		m := New(nil, nil, nil, nil, nil, config.Manager{}, config.Config{})
		err := m.ReconcileMissingSeries(context.Background(), nil)
		require.NoError(t, err)
	})

	t.Run("no missing series", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		ctx := context.Background()
		store := storageMocks.NewMockStorage(ctrl)

		store.EXPECT().ListIndexers(ctx).Return([]sqlcdb.Indexer{}, nil)

		indexerFactory := indexerMock.NewMockFactory(ctrl)

		m := New(nil, indexerFactory, nil, store, nil, config.Manager{}, config.Config{})
		snapshot := newReconcileSnapshot([]sqlcdb.Indexer{}, nil)
		err := m.ReconcileMissingSeries(ctx, snapshot)
		require.NoError(t, err)
	})

	t.Run("error listing series", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		ctx := context.Background()
		store := storageMocks.NewMockStorage(ctrl)

		monitored := true
		seriesFilter := storage.SeriesFilter{
			States:    []storage.SeriesState{storage.SeriesStateMissing},
			Monitored: &monitored,
		}

		indexer := &sqlcdb.Indexer{ID: 1, Name: "test", Priority: 1}
		store.EXPECT().ListIndexers(ctx).Return([]sqlcdb.Indexer{*indexer}, nil)

		expectedErr := errors.New("database error")
		store.EXPECT().ListSeries(ctx, seriesFilter).Return(nil, expectedErr)

		indexerFactory := indexerMock.NewMockFactory(ctrl)

		m := New(nil, indexerFactory, nil, store, nil, config.Manager{}, config.Config{})
		snapshot := newReconcileSnapshot([]sqlcdb.Indexer{}, nil)
		err := m.ReconcileMissingSeries(ctx, snapshot)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "couldn't list missing series")
	})

	t.Run("successful reconciliation", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		ctx := context.Background()
		store := newStore(t, ctx)

		mockDownloadClient := downloadMock.NewMockDownloadClient(ctrl)
		mockFactory := downloadMock.NewMockFactory(ctrl)

		downloadClientModel := sqlcdb.DownloadClient{
			ID:             2,
			Implementation: "transmission",
			Type:           "torrent",
			Port:           8080,
			Host:           "transmission",
			Scheme:         "http",
		}
		_, errClient := store.CreateDownloadClient(ctx, downloadClientModel)
		require.NoError(t, errClient)
		_, errClient = store.CreateDownloadClient(ctx, downloadClientModel)
		require.NoError(t, errClient)

		snapshot := newReconcileSnapshot([]sqlcdb.Indexer{{ID: 1}}, []sqlcdb.DownloadClient{downloadClientModel})

		mockFactory.EXPECT().NewDownloadClient(downloadClientModel).Return(mockDownloadClient, nil)
		mockFactory.EXPECT().NewDownloadClient(downloadClientModel).Return(mockDownloadClient, nil)
		mockDownloadClient.EXPECT().Add(ctx, gomock.Any()).Return(download.Status{
			ID:   "123",
			Name: "test download",
		}, nil)
		mockDownloadClient.EXPECT().Add(ctx, gomock.Any()).Return(download.Status{
			ID:   "124",
			Name: "test download",
		}, nil)

		seriesMetadataID, err := store.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{
			TmdbID:       1,
			Title:        "Series",
			EpisodeCount: 10,
		})
		require.NoError(t, err)

		seriesID, err := store.CreateSeries(ctx, sqlcdb.Series{
			ID:               1,
			SeriesMetadataID: sql.NullInt64{Int64: seriesMetadataID, Valid: true},
			Monitored:        1,
			QualityProfileID: 4,
		}, storage.SeriesStateMissing)
		require.NoError(t, err)

		_, err = store.GetSeriesByID(ctx, seriesID)
		require.NoError(t, err)

		seasonMetadataID, err := store.CreateSeasonMetadata(ctx, sqlcdb.SeasonMetadatum{
			SeriesMetadataID: seriesMetadataID,
			Title:            "Season 1",
			Number:           1,
		})
		require.NoError(t, err)

		seasonID, err := store.CreateSeason(ctx, sqlcdb.Season{
			SeriesID:         1,
			SeasonMetadataID: sql.NullInt64{Int64: 1, Valid: true},
			Monitored:        1,
		}, storage.SeasonStateMissing)
		require.NoError(t, err)

		season, err := store.GetSeasonByID(ctx, seasonID)
		require.NoError(t, err)
		assert.Equal(t, int64(1), season.Season.SeriesID)
		assert.Equal(t, sql.NullInt64{Int64: 1, Valid: true}, season.Season.SeasonMetadataID)

		episodeMetadataID1, err := store.CreateEpisodeMetadata(ctx, sqlcdb.EpisodeMetadatum{
			TmdbID:           1,
			Title:            "Hello",
			Number:           1,
			SeasonMetadataID: seasonMetadataID,
			AirDate:          sql.NullTime{Time: snapshot.time.Add(time.Hour * -2), Valid: true},
			Runtime:          sql.NullInt64{Int64: 42, Valid: true},
		})
		require.NoError(t, err)

		_, err = store.CreateEpisode(ctx, sqlcdb.Episode{
			SeasonID:          seasonID,
			EpisodeNumber:     1,
			EpisodeMetadataID: sql.NullInt64{Int64: episodeMetadataID1, Valid: true},
			Monitored:         1,
		}, storage.EpisodeStateMissing)
		require.NoError(t, err)

		episodeMetadataID2, err := store.CreateEpisodeMetadata(ctx, sqlcdb.EpisodeMetadatum{
			TmdbID:           2,
			Title:            "There",
			Number:           2,
			SeasonMetadataID: seasonMetadataID,
			AirDate:          sql.NullTime{Time: snapshot.time.Add(time.Hour * -2), Valid: true},
			Runtime:          sql.NullInt64{Int64: 42, Valid: true},
		})
		require.NoError(t, err)

		_, err = store.CreateEpisode(ctx, sqlcdb.Episode{
			SeasonID:          seasonID,
			EpisodeNumber:     2,
			EpisodeMetadataID: sql.NullInt64{Int64: episodeMetadataID2, Valid: true},
		}, storage.EpisodeStateMissing)
		require.NoError(t, err)

		releases := []*prowlarr.ReleaseResource{
			{
				ID:       ptr.To(int32(1)),
				Title:    nullable.NewNullableWithValue("Series.S01E01.1080p.WEB-DL.AAC2.0.x264-GROUP"),
				Size:     sizeGBToBytes(2),
				Protocol: ptr.To(prowlarr.DownloadProtocolTorrent),
			},
			{
				ID:       ptr.To(int32(2)),
				Title:    nullable.NewNullableWithValue("Series.S01E02.1080p.WEB-DL.AAC2.0.x264-GROUP"),
				Size:     sizeGBToBytes(2),
				Protocol: ptr.To(prowlarr.DownloadProtocolTorrent),
			},
		}

		mockIndexerSource := indexerMock.NewMockIndexerSource(ctrl)
		mockIndexerSource.EXPECT().Search(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(releases, nil).AnyTimes()

		indexerFactory := indexerMock.NewMockFactory(ctrl)
		indexerFactory.EXPECT().NewIndexerSource(gomock.Any()).Return(mockIndexerSource, nil).AnyTimes()

		// Create an indexer source in the database
		sourceID, err := store.CreateIndexerSource(ctx, sqlcdb.IndexerSource{
			Name:           "test-source",
			Implementation: "prowlarr",
			Scheme:         "http",
			Host:           "test",
			Enabled:        true,
		})
		require.NoError(t, err)

		// Create indexers linked to the source
		_, err = store.CreateIndexer(ctx, sqlcdb.Indexer{
			IndexerSourceID: sql.NullInt64{Int64: sourceID, Valid: true},
			Name:            "test",
			Priority:        1,
			Uri:             "http://test/indexer1",
		})
		require.NoError(t, err)

		mockIndexerSource.EXPECT().ListIndexers(ctx).Return([]indexer.SourceIndexer{
			{ID: 1, Name: "test", Priority: 1},
		}, nil)

		m := New(nil, indexerFactory, nil, store, mockFactory, config.Manager{}, config.Config{})
		err = m.RefreshIndexerSource(ctx, sourceID)
		require.NoError(t, err)

		err = m.ReconcileMissingSeries(ctx, snapshot)
		require.NoError(t, err)

		episodes, err := store.ListEpisodes(ctx, storage.EpisodeFilter{})
		require.NoError(t, err)
		require.Len(t, episodes, 2)

		assert.Equal(t, string(storage.EpisodeStateDownloading), episodes[0].State)
		assert.Equal(t, sql.NullString{String: "123", Valid: true}, episodes[0].DownloadID)
		assert.Equal(t, sql.NullInt64{Int64: 2, Valid: true}, episodes[0].DownloadClientID)

		assert.Equal(t, string(storage.EpisodeStateDownloading), episodes[1].State)
		assert.Equal(t, sql.NullString{String: "124", Valid: true}, episodes[1].DownloadID)
		assert.Equal(t, sql.NullInt64{Int64: 2, Valid: true}, episodes[1].DownloadClientID)
	})
}

func TestMediaManager_ReconcileContinuingSeries(t *testing.T) {
	t.Run("nil snapshot", func(t *testing.T) {
		m := New(nil, nil, nil, nil, nil, config.Manager{}, config.Config{})
		err := m.ReconcileContinuingSeries(context.Background(), nil)
		require.NoError(t, err)
	})

	t.Run("no continuing series", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		ctx := context.Background()
		store := storageMocks.NewMockStorage(ctrl)

		continuingFilter := storage.SeriesFilter{
			States: []storage.SeriesState{storage.SeriesStateContinuing, storage.SeriesStateDownloading},
		}

		store.EXPECT().ListSeries(ctx, continuingFilter).Return(nil, storage.ErrNotFound)

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		err := m.ReconcileContinuingSeries(ctx, &ReconcileSnapshot{})
		require.NoError(t, err)
	})

	t.Run("error listing series", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		ctx := context.Background()
		store := storageMocks.NewMockStorage(ctrl)

		continuingFilter := storage.SeriesFilter{
			States: []storage.SeriesState{storage.SeriesStateContinuing, storage.SeriesStateDownloading},
		}

		expectedErr := errors.New("database error")
		store.EXPECT().ListSeries(ctx, continuingFilter).Return(nil, expectedErr)

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		err := m.ReconcileContinuingSeries(ctx, &ReconcileSnapshot{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "couldn't list continuing series")
	})

	t.Run("successful reconciliation with missing episodes", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		ctx := context.Background()
		store := newStore(t, ctx)

		// Add TMDB mock for the refresh functionality
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)
		tmdbMock.EXPECT().GetSeriesDetails(ctx, 1).Return(&tmdb.SeriesDetails{
			ID:   1,
			Name: "Continuing Series",
		}, nil).AnyTimes()

		// Mock external IDs and watch providers calls during metadata creation
		extIDsResp := &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewBufferString(`{"imdb_id":null,"tvdb_id":null}`))}
		tmdbMock.EXPECT().TvSeriesExternalIds(gomock.Any(), int32(1)).Return(extIDsResp, nil).AnyTimes()
		wpResp := &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewBufferString(`{"results":{"US":{"flatrate":[]}}}`))}
		tmdbMock.EXPECT().TvSeriesWatchProviders(gomock.Any(), int32(1)).Return(wpResp, nil).AnyTimes()

		mockDownloadClient := downloadMock.NewMockDownloadClient(ctrl)
		mockFactory := downloadMock.NewMockFactory(ctrl)

		downloadClientModel := sqlcdb.DownloadClient{
			ID:             1,
			Implementation: "transmission",
			Type:           "torrent",
			Port:           8080,
			Host:           "transmission",
			Scheme:         "http",
		}
		_, errClient := store.CreateDownloadClient(ctx, downloadClientModel)
		require.NoError(t, errClient)
		_, errClient = store.CreateDownloadClient(ctx, downloadClientModel)
		require.NoError(t, errClient)

		snapshot := newReconcileSnapshot([]sqlcdb.Indexer{{ID: 1}}, []sqlcdb.DownloadClient{downloadClientModel})

		mockFactory.EXPECT().NewDownloadClient(gomock.Any()).Return(mockDownloadClient, nil).AnyTimes()
		mockDownloadClient.EXPECT().Add(ctx, gomock.Any()).Return(download.Status{
			ID:   "download-123",
			Name: "test download continuing",
		}, nil).AnyTimes()

		seriesMetadataID, err := store.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{
			TmdbID:         1,
			Title:          "Continuing Series",
			Status:         "Continuing",
			EpisodeCount:   10,
			ExternalIds:    sql.NullString{},
			WatchProviders: sql.NullString{},
		})
		require.NoError(t, err)

		seriesID, err := store.CreateSeries(ctx, sqlcdb.Series{
			SeriesMetadataID: sql.NullInt64{Int64: seriesMetadataID, Valid: true},
			Monitored:        1,
			QualityProfileID: 4, // Episode profile from defaults.sql
		}, storage.SeriesStateMissing)
		require.NoError(t, err)

		// Transition to downloading state
		err = store.UpdateSeriesState(ctx, seriesID, storage.SeriesStateDownloading, nil)
		require.NoError(t, err)

		seasonMetadataID, err := store.CreateSeasonMetadata(ctx, sqlcdb.SeasonMetadatum{
			SeriesMetadataID: seriesMetadataID,
			Title:            "Season 1",
			Number:           1,
		})
		require.NoError(t, err)

		seasonID, err := store.CreateSeason(ctx, sqlcdb.Season{
			SeriesID:         seriesID,
			SeasonMetadataID: sql.NullInt64{Int64: seasonMetadataID, Valid: true},
			Monitored:        1,
		}, storage.SeasonStateMissing)
		require.NoError(t, err)

		// Transition to downloading state
		err = store.UpdateSeasonState(ctx, seasonID, storage.SeasonStateDownloading, nil)
		require.NoError(t, err)

		episodeMetadataID, err := store.CreateEpisodeMetadata(ctx, sqlcdb.EpisodeMetadatum{
			TmdbID:           1,
			Title:            "New Episode",
			Number:           3,
			SeasonMetadataID: seasonMetadataID,
			AirDate:          sql.NullTime{Time: snapshot.time.Add(time.Hour * -2), Valid: true},
			Runtime:          sql.NullInt64{Int64: 42, Valid: true},
		})
		require.NoError(t, err)

		_, err = store.CreateEpisode(ctx, sqlcdb.Episode{
			SeasonID:          seasonID,
			EpisodeNumber:     3,
			EpisodeMetadataID: sql.NullInt64{Int64: episodeMetadataID, Valid: true},
			Monitored:         1,
		}, storage.EpisodeStateMissing)
		require.NoError(t, err)

		releases := []*prowlarr.ReleaseResource{
			{
				ID:       ptr.To(int32(1)),
				Title:    nullable.NewNullableWithValue("Continuing.Series.S01E03.1080p.WEB-DL.AAC2.0.x264-GROUP"),
				Size:     sizeGBToBytes(2),
				Protocol: ptr.To(prowlarr.DownloadProtocolTorrent),
			},
		}

		mockIndexerSource := indexerMock.NewMockIndexerSource(ctrl)
		mockIndexerSource.EXPECT().Search(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(releases, nil).AnyTimes()

		indexerFactory := indexerMock.NewMockFactory(ctrl)
		indexerFactory.EXPECT().NewIndexerSource(gomock.Any()).Return(mockIndexerSource, nil).AnyTimes()

		m := New(tmdbMock, indexerFactory, nil, store, mockFactory, config.Manager{}, config.Config{})

		// Just test that the function runs without error
		err = m.ReconcileContinuingSeries(ctx, snapshot)
		require.NoError(t, err)
	})

	t.Run("continuing series with no missing episodes", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)

		snapshot := newReconcileSnapshot([]sqlcdb.Indexer{{ID: 1}}, []sqlcdb.DownloadClient{})

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})

		err := m.ReconcileContinuingSeries(ctx, snapshot)
		require.NoError(t, err)
	})

	t.Run("discover new episodes for continuing series", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		ctx := context.Background()
		store := newStore(t, ctx)

		// Set up series and season data
		seriesMetadataID, err := store.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{
			TmdbID:         100,
			Title:          "Weekly Series",
			SeasonCount:    1,
			EpisodeCount:   3,
			Status:         "Returning Series",
			ExternalIds:    sql.NullString{},
			WatchProviders: sql.NullString{},
		})
		require.NoError(t, err)

		seriesID, err := store.CreateSeries(ctx, sqlcdb.Series{
			SeriesMetadataID: sql.NullInt64{Int64: seriesMetadataID, Valid: true},
			Monitored:        1,
			QualityProfileID: 4, // Episode profile from defaults.sql
		}, storage.SeriesStateMissing)
		require.NoError(t, err)

		// Transition to downloading state
		err = store.UpdateSeriesState(ctx, seriesID, storage.SeriesStateDownloading, nil)
		require.NoError(t, err)

		// Transition to continuing state
		err = store.UpdateSeriesState(ctx, seriesID, storage.SeriesStateContinuing, nil)
		require.NoError(t, err)

		seasonMetadataID, err := store.CreateSeasonMetadata(ctx, sqlcdb.SeasonMetadatum{
			SeriesMetadataID: seriesMetadataID, // Should reference the series metadata ID, not storage series ID
			Title:            "Season 1",
			Number:           1,
			TmdbID:           1001, // Match the TMDB ID from our mock
		})
		require.NoError(t, err)

		seasonID, err := store.CreateSeason(ctx, sqlcdb.Season{
			SeriesID:         seriesID,
			SeasonMetadataID: sql.NullInt64{Int64: seasonMetadataID, Valid: true},
			Monitored:        1,
		}, storage.SeasonStateMissing)
		require.NoError(t, err)

		// Transition to downloading state
		err = store.UpdateSeasonState(ctx, seasonID, storage.SeasonStateDownloading, nil)
		require.NoError(t, err)

		// Transition to continuing state
		err = store.UpdateSeasonState(ctx, seasonID, storage.SeasonStateContinuing, nil)
		require.NoError(t, err)

		// Create existing episode metadata (episodes 1-2 exist, episode 3 will be discovered)
		for i := 1; i <= 2; i++ {
			// Episodes 1-2 aired in the past
			pastDate := time.Now().Add(-time.Hour * 24 * time.Duration(i))
			airDate := &pastDate

			episodeMetadataID, err := store.CreateEpisodeMetadata(ctx, sqlcdb.EpisodeMetadatum{
				TmdbID:           int64(1001000 + i), // Match the TMDB IDs from our mock (1001001, 1001002)
				Title:            fmt.Sprintf("Episode %d", i),
				Number:           int64(i),
				SeasonMetadataID: seasonMetadataID, // This should be the season metadata ID
				AirDate:          sql.NullTime{Time: *airDate, Valid: true},
			})
			require.NoError(t, err)

			// Create episode records for episodes 1-2
			episodeID, err := store.CreateEpisode(ctx, sqlcdb.Episode{
				SeasonID:          seasonID,
				EpisodeNumber:     int64(i),
				EpisodeMetadataID: sql.NullInt64{Int64: episodeMetadataID, Valid: true},
				Monitored:         1,
			}, storage.EpisodeStateMissing)
			require.NoError(t, err)

			// Transition episodes 1-2 to downloaded state via downloading to avoid them being searched for
			err = store.UpdateEpisodeState(ctx, episodeID, storage.EpisodeStateDownloading, nil)
			require.NoError(t, err)
			err = store.UpdateEpisodeState(ctx, episodeID, storage.EpisodeStateDownloaded, nil)
			require.NoError(t, err)
		}

		snapshot := newReconcileSnapshot([]sqlcdb.Indexer{{ID: 1}}, []sqlcdb.DownloadClient{})

		// Set up TMDB mock for metadata refresh
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)
		tmdbMock.EXPECT().GetSeriesDetails(ctx, 100).Return(&tmdb.SeriesDetails{
			ID:               100, // Match the series TMDB ID
			Name:             "Weekly Series",
			NumberOfSeasons:  1,
			NumberOfEpisodes: 3,
			FirstAirDate:     "2024-01-01",
			Seasons: []tmdb.Season{
				{
					ID:           1001, // Set explicit TMDB ID for season
					SeasonNumber: 1,
					Name:         "Season 1",
					Overview:     "First season",
					Episodes: []tmdb.Episode{
						{ID: 1001001, EpisodeNumber: 1, Name: "Episode 1", Overview: "First episode", Runtime: 45, AirDate: "2024-01-01"},
						{ID: 1001002, EpisodeNumber: 2, Name: "Episode 2", Overview: "Second episode", Runtime: 45, AirDate: "2024-01-02"},
						{ID: 1001003, EpisodeNumber: 3, Name: "Episode 3", Overview: "Third episode", Runtime: 45, AirDate: "2099-12-31"},
					},
				},
			},
		}, nil).AnyTimes()

		// Mock external IDs and watch providers calls during metadata creation
		extIDsResp := &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewBufferString(`{"imdb_id":null,"tvdb_id":null}`))}
		tmdbMock.EXPECT().TvSeriesExternalIds(gomock.Any(), int32(100)).Return(extIDsResp, nil).AnyTimes()
		wpResp := &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewBufferString(`{"results":{"US":{"flatrate":[]}}}`))}
		tmdbMock.EXPECT().TvSeriesWatchProviders(gomock.Any(), int32(100)).Return(wpResp, nil).AnyTimes()

		mockIndexerSource := indexerMock.NewMockIndexerSource(ctrl)
		mockIndexerSource.EXPECT().Search(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*prowlarr.ReleaseResource{}, nil).AnyTimes()

		indexerFactory := indexerMock.NewMockFactory(ctrl)
		indexerFactory.EXPECT().NewIndexerSource(gomock.Any()).Return(mockIndexerSource, nil).AnyTimes()

		m := New(tmdbMock, indexerFactory, nil, store, nil, config.Manager{}, config.Config{})

		// Before reconciliation, we should only have 2 episode records
		episodesBefore, err := store.ListEpisodes(ctx, storage.EpisodeFilter{SeasonID: &seasonID})
		require.NoError(t, err)
		assert.Len(t, episodesBefore, 2, "Should have 2 existing episodes before reconciliation")

		err = m.ReconcileContinuingSeries(ctx, snapshot)
		require.NoError(t, err)

		episodesAfter, err := store.ListEpisodes(ctx, storage.EpisodeFilter{SeasonID: &seasonID})
		require.NoError(t, err)
		assert.Len(t, episodesAfter, 3, "Should have 3 episodes after reconciliation (episode 3 should be discovered)")

		// Check that episode 3 was created with correct state
		var episode3 *sqlcdb.ListEpisodesRow
		for _, ep := range episodesAfter {
			if ep.Episode.EpisodeNumber == 3 {
				episode3 = &ep
				break
			}
		}
		require.NotNil(t, episode3, "Episode 3 should have been created")
		assert.Equal(t, string(storage.EpisodeStateUnreleased), episode3.State, "Episode 3 should be in unreleased state")
	})

}

func TestMediaManager_ReconcileDiscoveredEpisodes(t *testing.T) {
	t.Run("no discovered episodes", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		snapshot := &ReconcileSnapshot{
			time: time.Now(),
		}

		ctx := context.Background()
		store := storageMocks.NewMockStorage(ctrl)

		discoveredFilter := storage.EpisodeFilter{
			States: []storage.EpisodeState{storage.EpisodeStateDiscovered},
		}

		store.EXPECT().ListEpisodes(ctx, discoveredFilter).Return(nil, storage.ErrNotFound)

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		err := m.ReconcileDiscoveredEpisodes(ctx, snapshot)
		require.NoError(t, err)
	})

	t.Run("error listing episodes", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		snapshot := &ReconcileSnapshot{
			time: time.Now(),
		}

		ctx := context.Background()
		store := storageMocks.NewMockStorage(ctrl)

		discoveredFilter := storage.EpisodeFilter{
			States: []storage.EpisodeState{storage.EpisodeStateDiscovered},
		}

		expectedErr := errors.New("database error")
		store.EXPECT().ListEpisodes(ctx, discoveredFilter).Return(nil, expectedErr)

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		err := m.ReconcileDiscoveredEpisodes(ctx, snapshot)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "couldn't list discovered episodes")
	})

	t.Run("successful reconciliation", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		ctx := context.Background()
		store := newStore(t, ctx)

		snapshot := &ReconcileSnapshot{
			time: time.Now(),
		}

		// Create series metadata for TMDB search functionality
		seriesMetadataID, err := store.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{
			TmdbID:       1,
			Title:        "Test Series",
			EpisodeCount: 5,
		})
		require.NoError(t, err)

		// Create a series with path and linked metadata
		seriesID, err := store.CreateSeries(ctx, sqlcdb.Series{QualityProfileID: 1,
			SeriesMetadataID: sql.NullInt64{Int64: seriesMetadataID, Valid: true},
			Path:             sql.NullString{String: "test-series", Valid: true},
			Monitored:        1,
		}, storage.SeriesStateMissing)
		require.NoError(t, err)

		// Create season metadata
		seasonMetadataID, err := store.CreateSeasonMetadata(ctx, sqlcdb.SeasonMetadatum{
			SeriesMetadataID: seriesMetadataID,
			Title:            "Season 1",
			Number:           1,
		})
		require.NoError(t, err)

		// Create season with metadata linked
		seasonID, err := store.CreateSeason(ctx, sqlcdb.Season{
			SeriesID:         seriesID,
			SeasonNumber:     1,
			SeasonMetadataID: sql.NullInt64{Int64: seasonMetadataID, Valid: true},
			Monitored:        1,
		}, storage.SeasonStateMissing)
		require.NoError(t, err)

		// Create episode metadata
		episodeMetadataID, err := store.CreateEpisodeMetadata(ctx, sqlcdb.EpisodeMetadatum{
			SeasonMetadataID: seasonMetadataID,
			Title:            "Episode 1",
			Number:           1,
		})
		require.NoError(t, err)

		// Create a discovered episode with metadata linked
		episodeID, err := store.CreateEpisode(ctx, sqlcdb.Episode{
			SeasonID:          seasonID,
			EpisodeNumber:     1,
			EpisodeMetadataID: sql.NullInt64{Int64: episodeMetadataID, Valid: true},
			Monitored:         1,
		}, storage.EpisodeStateDiscovered)
		require.NoError(t, err)

		// Mock the library interface for discovered files
		libraryMock := libraryMocks.NewMockLibrary(ctrl)
		libraryMock.EXPECT().FindEpisodes(ctx).Return([]library.EpisodeFile{
			{
				SeriesName:    "test-series",
				SeasonNumber:  1,
				EpisodeNumber: 1,
				Name:          "test-episode-file.mkv",
			},
		}, nil).AnyTimes()

		// Mock the TMDB client (not needed for this test but prevents nil pointer)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		m := New(tmdbMock, nil, libraryMock, store, nil, config.Manager{}, config.Config{})
		err = m.ReconcileDiscoveredEpisodes(ctx, snapshot)
		require.NoError(t, err)

		// Verify episode was transitioned to completed state
		episode, err := store.GetEpisodeByID(ctx, episodeID)
		require.NoError(t, err)
		assert.Equal(t, sql.NullString{String: string(storage.EpisodeStateCompleted), Valid: true}, episode.State)
	})

	t.Run("error during reconcile individual episode", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		ctx := context.Background()
		store := newStore(t, ctx)

		snapshot := &ReconcileSnapshot{
			time: time.Now(),
		}

		// Create series without path (will cause error in reconcileDiscoveredEpisode)
		seriesID, err := store.CreateSeries(ctx, sqlcdb.Series{QualityProfileID: 1,
			Path:      sql.NullString{}, // This will cause the reconcile to skip
			Monitored: 1,
		}, storage.SeriesStateMissing)
		require.NoError(t, err)

		seasonID, err := store.CreateSeason(ctx, sqlcdb.Season{
			SeriesID:  seriesID,
			Monitored: 1,
		}, storage.SeasonStateMissing)
		require.NoError(t, err)

		_, err = store.CreateEpisode(ctx, sqlcdb.Episode{
			SeasonID:      seasonID,
			EpisodeNumber: 1,
			Monitored:     1,
		}, storage.EpisodeStateDiscovered)
		require.NoError(t, err)

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		err = m.ReconcileDiscoveredEpisodes(ctx, snapshot)
		require.NoError(t, err) // Function should not fail even if individual episodes fail to reconcile
	})
}

func TestMediaManager_matchEpisodeFileToEpisode(t *testing.T) {
	t.Run("matches S01E03 format", func(t *testing.T) {
		ctx := context.Background()
		store := newTVReconcileStore(t, ctx, true)

		// Create season metadata first
		seasonMetadataID := int64(1)

		// Create episode metadata for episodes 1,2,3
		episodeMetadataID1, err := store.CreateEpisodeMetadata(ctx, sqlcdb.EpisodeMetadatum{TmdbID: 1, Title: "Episode 1", Number: 1, SeasonMetadataID: seasonMetadataID})
		require.NoError(t, err)
		episodeMetadataID2, err := store.CreateEpisodeMetadata(ctx, sqlcdb.EpisodeMetadatum{TmdbID: 2, Title: "Episode 2", Number: 2, SeasonMetadataID: seasonMetadataID})
		require.NoError(t, err)
		episodeMetadataID3, err := store.CreateEpisodeMetadata(ctx, sqlcdb.EpisodeMetadatum{TmdbID: 3, Title: "Episode 3", Number: 3, SeasonMetadataID: seasonMetadataID})
		require.NoError(t, err)

		// Create episodes
		episodes := []sqlcdb.ListEpisodesRow{
			{Episode: sqlcdb.Episode{ID: 1, EpisodeMetadataID: sql.NullInt64{Int64: episodeMetadataID1, Valid: true}}},
			{Episode: sqlcdb.Episode{ID: 2, EpisodeMetadataID: sql.NullInt64{Int64: episodeMetadataID2, Valid: true}}},
			{Episode: sqlcdb.Episode{ID: 3, EpisodeMetadataID: sql.NullInt64{Int64: episodeMetadataID3, Valid: true}}},
		}

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		result := m.matchEpisodeFileToEpisode(ctx, "/downloads/Series.Name.S01E03.1080p.WEB-DL.x264-GROUP.mkv", episodes)
		require.NotNil(t, result)
		assert.Equal(t, int64(3), result.Episode.ID)
	})

	t.Run("matches 1x05 format", func(t *testing.T) {
		ctx := context.Background()
		store := newTVReconcileStore(t, ctx, true)

		seasonMetadataID := int64(1)

		episodeMetadataID4, err := store.CreateEpisodeMetadata(ctx, sqlcdb.EpisodeMetadatum{TmdbID: 4, Title: "Episode 4", Number: 4, SeasonMetadataID: seasonMetadataID})
		require.NoError(t, err)
		episodeMetadataID5, err := store.CreateEpisodeMetadata(ctx, sqlcdb.EpisodeMetadatum{TmdbID: 5, Title: "Episode 5", Number: 5, SeasonMetadataID: seasonMetadataID})
		require.NoError(t, err)

		episodes := []sqlcdb.ListEpisodesRow{
			{Episode: sqlcdb.Episode{ID: 4, EpisodeMetadataID: sql.NullInt64{Int64: episodeMetadataID4, Valid: true}}},
			{Episode: sqlcdb.Episode{ID: 5, EpisodeMetadataID: sql.NullInt64{Int64: episodeMetadataID5, Valid: true}}},
		}

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		result := m.matchEpisodeFileToEpisode(ctx, "/downloads/Series.Name.1x05.720p.HDTV.x264-GROUP.mkv", episodes)
		require.NotNil(t, result)
		assert.Equal(t, int64(5), result.Episode.ID)
	})

	t.Run("no match when episode number not found", func(t *testing.T) {
		ctx := context.Background()
		store := newTVReconcileStore(t, ctx, true)

		episodeMetadataID6, err := store.CreateEpisodeMetadata(ctx, sqlcdb.EpisodeMetadatum{TmdbID: 6, Title: "Episode 1", Number: 1, SeasonMetadataID: 1})
		require.NoError(t, err)

		episodes := []sqlcdb.ListEpisodesRow{{Episode: sqlcdb.Episode{ID: 6, EpisodeMetadataID: sql.NullInt64{Int64: episodeMetadataID6, Valid: true}}}}

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		result := m.matchEpisodeFileToEpisode(ctx, "/downloads/Series.Name.Some.Random.File.mkv", episodes)
		assert.Nil(t, result)
	})

	t.Run("no match when episode number doesn't exist in episodes", func(t *testing.T) {
		ctx := context.Background()
		store := newTVReconcileStore(t, ctx, true)

		episodeMetadataID7, err := store.CreateEpisodeMetadata(ctx, sqlcdb.EpisodeMetadatum{TmdbID: 7, Title: "Episode 1", Number: 1, SeasonMetadataID: 1})
		require.NoError(t, err)
		episodeMetadataID8, err := store.CreateEpisodeMetadata(ctx, sqlcdb.EpisodeMetadatum{TmdbID: 8, Title: "Episode 2", Number: 2, SeasonMetadataID: 1})
		require.NoError(t, err)

		episodes := []sqlcdb.ListEpisodesRow{
			{Episode: sqlcdb.Episode{ID: 7, EpisodeMetadataID: sql.NullInt64{Int64: episodeMetadataID7, Valid: true}}},
			{Episode: sqlcdb.Episode{ID: 8, EpisodeMetadataID: sql.NullInt64{Int64: episodeMetadataID8, Valid: true}}},
		}

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		result := m.matchEpisodeFileToEpisode(ctx, "/downloads/Series.Name.S01E10.1080p.WEB-DL.x264-GROUP.mkv", episodes)
		assert.Nil(t, result)
	})
}

func Test_Manager_ReconcileSeries(t *testing.T) {
	t.Run("returns nil when all sub-reconcilers succeed", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		require.NotNil(t, m)

		err := m.ReconcileSeries(ctx)
		assert.NoError(t, err)
	})

	t.Run("returns error when ListDownloadClients fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ctx := context.Background()

		store := storageMocks.NewMockStorage(ctrl)
		store.EXPECT().ListDownloadClients(ctx).Return(nil, errors.New("download client error"))

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		require.NotNil(t, m)

		err := m.ReconcileSeries(ctx)
		assert.Error(t, err)
		assert.Equal(t, "download client error", err.Error())
	})

	t.Run("aggregates errors from multiple sub-reconcilers", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ctx := context.Background()

		store := storageMocks.NewMockStorage(ctrl)

		// ListDownloadClients succeeds
		store.EXPECT().ListDownloadClients(ctx).Return(nil, nil)

		// ReconcileMissingSeries: listIndexersInternal → ListIndexers fails
		store.EXPECT().ListIndexers(gomock.Any()).Return(nil, errors.New("indexer error")).AnyTimes()

		// ReconcileDownloadingSeries: ListEpisodes fails
		store.EXPECT().ListEpisodes(gomock.Any(), gomock.Any()).Return(nil, errors.New("episodes error")).AnyTimes()

		// ReconcileContinuingSeries: ListSeries succeeds with empty
		store.EXPECT().ListSeries(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()

		// ReconcileCompletedSeasons: ListSeasons succeeds with empty
		store.EXPECT().ListSeasons(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		require.NotNil(t, m)

		err := m.ReconcileSeries(ctx)
		assert.Error(t, err)
		// errors.Join wraps multiple errors; both messages should be present
		errStr := err.Error()
		assert.Contains(t, errStr, "indexer error")
		assert.Contains(t, errStr, "episodes error")
	})
}

func newTVReconcileStore(t *testing.T, ctx context.Context, withSeason bool) storage.Storage {
	t.Helper()
	store := newStore(t, ctx)
	metadataID, err := store.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{TmdbID: 999999, Title: "Series", Status: "Continuing"})
	require.NoError(t, err)
	seasonMetadataID, err := store.CreateSeasonMetadata(ctx, sqlcdb.SeasonMetadatum{SeriesMetadataID: metadataID, TmdbID: 999999, Number: 1, Title: "Season 1"})
	require.NoError(t, err)
	seriesID, err := store.CreateSeries(ctx, sqlcdb.Series{SeriesMetadataID: sql.NullInt64{Int64: metadataID, Valid: true}, QualityProfileID: 1, Monitored: 1}, storage.SeriesStateMissing)
	require.NoError(t, err)
	if withSeason {
		_, err = store.CreateSeason(ctx, sqlcdb.Season{SeriesID: seriesID, SeasonMetadataID: sql.NullInt64{Int64: seasonMetadataID, Valid: true}, SeasonNumber: 1, Monitored: 1}, storage.SeasonStateMissing)
		require.NoError(t, err)
	}
	return store
}
