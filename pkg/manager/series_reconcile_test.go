package manager

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/download"
	downloadMock "github.com/kasuboski/mediaz/pkg/download/mocks"
	"github.com/kasuboski/mediaz/pkg/indexer"
	indexerMock "github.com/kasuboski/mediaz/pkg/indexer/mocks"
	"github.com/kasuboski/mediaz/pkg/library"
	libraryMocks "github.com/kasuboski/mediaz/pkg/library/mocks"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/storage"
	storageMocks "github.com/kasuboski/mediaz/pkg/storage/mocks"
	mediaSqlite "github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
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
		store, err := mediaSqlite.New(context.Background(), ":memory:")
		require.NoError(t, err)

		schemas, err := storage.ReadSchemaFiles("../storage/sqlite/schema/schema.sql", "../storage/sqlite/schema/defaults.sql")
		require.NoError(t, err)
		require.NotNil(t, store)

		err = store.Init(ctx, schemas...)
		require.NoError(t, err)

		manager := MediaManager{
			storage: store,
		}

		episode := storage.Episode{
			Episode: model.Episode{
				SeasonID:          1,
				EpisodeNumber:     1,
				EpisodeMetadataID: ptr(int32(1)),
			},
		}

		episodeID, err := manager.storage.CreateEpisode(ctx, episode, storage.EpisodeStateMissing)
		require.NoError(t, err)
		assert.Equal(t, int64(1), episodeID)

		episode.ID = int32(episodeID)
		err = manager.updateEpisodeState(ctx, episode, storage.EpisodeStateDownloading, &storage.TransitionStateMetadata{
			DownloadID:             ptr("123"),
			DownloadClientID:       ptr(int32(2)),
			IsEntireSeasonDownload: ptr(true),
		})
		require.NoError(t, err)

		foundEpisode, err := store.GetEpisode(ctx, table.Episode.ID.EQ(sqlite.Int32(episode.ID)))
		require.NoError(t, err)
		require.NotNil(t, foundEpisode)

		assert.Equal(t, storage.EpisodeStateDownloading, foundEpisode.State)
		assert.Equal(t, int32(2), foundEpisode.DownloadClientID)
		assert.Equal(t, "123", foundEpisode.DownloadID)
		assert.Equal(t, int32(1), foundEpisode.SeasonID)
		assert.Equal(t, int32(1), foundEpisode.EpisodeNumber)
	})
}

func TestMediaManager_reconcileMissingEpisodes(t *testing.T) {
	t.Run("reconcile missing episodes - not all released", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		ctx := context.Background()
		store := newStore(t, ctx)

		mockDownloadClient := downloadMock.NewMockDownloadClient(ctrl)
		mockFactory := downloadMock.NewMockFactory(ctrl)
		downloadClientModel := model.DownloadClient{
			ID:             2,
			Implementation: "transmission",
			Type:           "torrent",
			Port:           8080,
			Host:           "transmission",
			Scheme:         "http",
		}

		snapshot := newReconcileSnapshot([]model.Indexer{{ID: 1}}, []*model.DownloadClient{&downloadClientModel})

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

		episodeMetadata := model.EpisodeMetadata{
			TmdbID: 1,
			Title:  "Test Episode",
			Number: 1,
			// time in the past
			AirDate: ptr(snapshot.time.Add(time.Hour * -2)),
			Runtime: ptr(int32(42)),
		}

		metadataID1, err := store.CreateEpisodeMetadata(ctx, episodeMetadata)
		require.NoError(t, err)

		episode1 := storage.Episode{
			Episode: model.Episode{
				SeasonID:          1,
				EpisodeNumber:     1,
				EpisodeMetadataID: ptr(int32(metadataID1)),
			},
		}

		_, err = store.CreateEpisode(ctx, episode1, storage.EpisodeStateMissing)
		require.NoError(t, err)

		episodeMetadata = model.EpisodeMetadata{
			TmdbID: 2,
			Title:  "Test Episode 2",
			Number: 2,
			// time in the past
			AirDate: ptr(snapshot.time.Add(time.Hour * -2)),
			Runtime: ptr(int32(42)),
		}

		metadataID2, err := store.CreateEpisodeMetadata(ctx, episodeMetadata)
		require.NoError(t, err)

		episode2 := storage.Episode{
			Episode: model.Episode{
				SeasonID:          1,
				EpisodeNumber:     2,
				EpisodeMetadataID: ptr(int32(metadataID2)),
			},
		}
		_, err = store.CreateEpisode(ctx, episode2, storage.EpisodeStateMissing)
		require.NoError(t, err)

		episodeMetadata = model.EpisodeMetadata{
			TmdbID: 3,
			Title:  "Test Episode 3",
			Number: 3,
			// future time
			AirDate: ptr(snapshot.time.Add(time.Hour * 2)),
			Runtime: ptr(int32(42)),
		}

		metadataID3, err := store.CreateEpisodeMetadata(ctx, episodeMetadata)
		require.NoError(t, err)

		episode3 := storage.Episode{
			Episode: model.Episode{
				SeasonID:          1,
				EpisodeNumber:     3,
				EpisodeMetadataID: ptr(int32(metadataID3)),
			},
		}

		_, err = store.CreateEpisode(ctx, episode3, storage.EpisodeStateMissing)
		require.NoError(t, err)

		releases := []*prowlarr.ReleaseResource{
			{
				ID:       ptr(int32(1)),
				Title:    nullable.NewNullableWithValue("Series.S01E01.1080p.WEB-DL.AAC2.0.x264-GROUP"),
				Size:     sizeGBToBytes(2),
				Protocol: ptr(prowlarr.DownloadProtocolTorrent),
			},
			{
				ID:       ptr(int32(2)),
				Title:    nullable.NewNullableWithValue("Series.S01E02.1080p.WEB-DL.AAC2.0.x264-GROUP"),
				Size:     sizeGBToBytes(2),
				Protocol: ptr(prowlarr.DownloadProtocolTorrent),
			},
		}

		qualityProfile := storage.QualityProfile{
			Name: "Default",
			Qualities: []storage.QualityDefinition{
				{
					Name:          "HD",
					MinSize:       0,
					MaxSize:       2000,
					PreferredSize: 1000,
					MediaType:     "tv",
				},
			},
		}

		episodes, err := store.ListEpisodes(ctx)
		require.NoError(t, err)
		require.Len(t, episodes, 3)

		m := New(nil, nil, nil, store, mockFactory, config.Manager{}, config.Config{})

		err = m.reconcileMissingEpisodes(ctx, "Series", 1, 1, episodes, snapshot, qualityProfile, releases)
		require.NoError(t, err)

		episodes, err = store.ListEpisodes(ctx)
		require.NoError(t, err)
		require.Len(t, episodes, 3)

		slices.SortFunc(episodes, func(a, b *storage.Episode) int {
			if a.ID < b.ID {
				return -1
			}
			if a.ID > b.ID {
				return 1
			}
			return 0
		})

		assert.Equal(t, storage.EpisodeStateDownloading, episodes[0].State)
		assert.Equal(t, "123", episodes[0].DownloadID)
		assert.Equal(t, int32(2), episodes[0].DownloadClientID)

		assert.Equal(t, storage.EpisodeStateDownloading, episodes[1].State)
		assert.Equal(t, "124", episodes[1].DownloadID)
		assert.Equal(t, int32(2), episodes[1].DownloadClientID)

		assert.Equal(t, storage.EpisodeStateMissing, episodes[2].State)
		assert.Equal(t, "", episodes[2].DownloadID)
		assert.Equal(t, int32(0), episodes[2].DownloadClientID)
	})

	t.Run("reconcile missing episodes - all released", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		ctx := context.Background()
		store := newStore(t, ctx)

		mockDownloadClient := downloadMock.NewMockDownloadClient(ctrl)
		mockFactory := downloadMock.NewMockFactory(ctrl)
		downloadClientModel := model.DownloadClient{
			ID:             2,
			Implementation: "transmission",
			Type:           "torrent",
			Port:           8080,
			Host:           "transmission",
			Scheme:         "http",
		}

		snapshot := newReconcileSnapshot([]model.Indexer{{ID: 1}}, []*model.DownloadClient{&downloadClientModel})

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

		seasonID, err := store.CreateSeason(ctx, storage.Season{
			Season: model.Season{
				SeriesID:         1,
				SeasonMetadataID: ptr(int32(1)),
				Monitored:        1,
			},
		}, storage.SeasonStateMissing)
		require.NoError(t, err)

		episodeMetadata := model.EpisodeMetadata{
			TmdbID: 1,
			Title:  "Test Episode",
			Number: 1,
			// time in the past
			AirDate: ptr(snapshot.time.Add(time.Hour * -2)),
			Runtime: ptr(int32(42)),
		}

		metadataID1, err := store.CreateEpisodeMetadata(ctx, episodeMetadata)
		require.NoError(t, err)

		episode1 := storage.Episode{
			Episode: model.Episode{
				SeasonID:          int32(seasonID),
				EpisodeNumber:     1,
				EpisodeMetadataID: ptr(int32(metadataID1)),
			},
		}

		_, err = store.CreateEpisode(ctx, episode1, storage.EpisodeStateMissing)
		require.NoError(t, err)

		episodeMetadata = model.EpisodeMetadata{
			TmdbID: 2,
			Title:  "Test Episode 2",
			Number: 2,
			// time in the past
			AirDate: ptr(snapshot.time.Add(time.Hour * -2)),
			Runtime: ptr(int32(42)),
		}

		metadataID2, err := store.CreateEpisodeMetadata(ctx, episodeMetadata)
		require.NoError(t, err)

		episode2 := storage.Episode{
			Episode: model.Episode{
				SeasonID:          int32(seasonID),
				EpisodeNumber:     2,
				EpisodeMetadataID: ptr(int32(metadataID2)),
			},
		}
		_, err = store.CreateEpisode(ctx, episode2, storage.EpisodeStateMissing)
		require.NoError(t, err)

		releases := []*prowlarr.ReleaseResource{
			{
				ID:       ptr(int32(1)),
				Title:    nullable.NewNullableWithValue("Series.S01E01.1080p.WEB-DL.AAC2.0.x264-GROUP"),
				Size:     sizeGBToBytes(2),
				Protocol: ptr(prowlarr.DownloadProtocolTorrent),
			},
			{
				ID:       ptr(int32(2)),
				Title:    nullable.NewNullableWithValue("Series.S01E02.1080p.WEB-DL.AAC2.0.x264-GROUP"),
				Size:     sizeGBToBytes(2),
				Protocol: ptr(prowlarr.DownloadProtocolTorrent),
			},
		}

		qualityProfile := storage.QualityProfile{
			Name: "Default",
			Qualities: []storage.QualityDefinition{
				{
					Name:          "HD",
					MinSize:       0,
					MaxSize:       2000,
					PreferredSize: 1000,
					MediaType:     "tv",
				},
			},
		}

		episodes, err := store.ListEpisodes(ctx)
		require.NoError(t, err)
		require.Len(t, episodes, 2)

		m := New(nil, nil, nil, store, mockFactory, config.Manager{}, config.Config{})

		err = m.reconcileMissingEpisodes(ctx, "Series", int32(seasonID), 1, episodes, snapshot, qualityProfile, releases)
		require.NoError(t, err)

		episodes, err = store.ListEpisodes(ctx)
		require.NoError(t, err)
		require.Len(t, episodes, 2)

		slices.SortFunc(episodes, func(a, b *storage.Episode) int {
			if a.ID < b.ID {
				return -1
			}
			if a.ID > b.ID {
				return 1
			}
			return 0
		})

		assert.Equal(t, storage.EpisodeStateDownloading, episodes[0].State)
		assert.Equal(t, "123", episodes[0].DownloadID)
		assert.Equal(t, int32(2), episodes[0].DownloadClientID)

		assert.Equal(t, storage.EpisodeStateDownloading, episodes[1].State)
		assert.Equal(t, "124", episodes[1].DownloadID)
		assert.Equal(t, int32(2), episodes[1].DownloadClientID)

		seasons, err := store.ListSeasons(ctx)
		require.NoError(t, err)
		require.Len(t, seasons, 1)
		assert.Equal(t, storage.SeasonStateDownloading, seasons[0].State)
	})

	t.Run("nil episode", func(t *testing.T) {
		m := New(nil, nil, nil, nil, nil, config.Manager{}, config.Config{})
		err := m.reconcileMissingEpisodes(context.Background(), "Series", 1, 1, []*storage.Episode{nil}, nil, storage.QualityProfile{}, nil)
		require.NoError(t, err)
	})

	t.Run("nil snapshot", func(t *testing.T) {
		m := New(nil, nil, nil, nil, nil, config.Manager{}, config.Config{})
		episode := &storage.Episode{}
		err := m.reconcileMissingEpisodes(context.Background(), "Series", 1, 1, []*storage.Episode{episode}, nil, storage.QualityProfile{}, nil)
		require.NoError(t, err)
	})

	t.Run("nil runtime", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)

		episodeMetadata := model.EpisodeMetadata{
			ID:      1,
			Title:   "Test Episode",
			Number:  1,
			Runtime: nil,
		}

		_, err := store.CreateEpisodeMetadata(ctx, episodeMetadata)
		require.NoError(t, err)

		episode := storage.Episode{
			Episode: model.Episode{
				EpisodeMetadataID: ptr(int32(1)),
			},
		}

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		err = m.reconcileMissingEpisodes(ctx, "Series", 1, 1, []*storage.Episode{&episode}, &ReconcileSnapshot{}, storage.QualityProfile{}, nil)
		require.NoError(t, err)
	})

	t.Run("no matching releases", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)

		episodeMetadata := model.EpisodeMetadata{
			ID:     1,
			Title:  "Test Episode",
			Number: 1,
		}

		_, err := store.CreateEpisodeMetadata(ctx, episodeMetadata)
		require.NoError(t, err)

		episode := storage.Episode{
			Episode: model.Episode{
				EpisodeMetadataID: ptr(int32(1)),
			},
		}

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		err = m.reconcileMissingEpisodes(ctx, "Series", 1, 1, []*storage.Episode{&episode}, &ReconcileSnapshot{}, storage.QualityProfile{}, nil)
		require.NoError(t, err)
	})
}
func TestMediaManager_reconcileMissingSeason(t *testing.T) {
	t.Run("nil season", func(t *testing.T) {
		m := New(nil, nil, nil, nil, nil, config.Manager{}, config.Config{})
		err := m.reconcileMissingSeason(context.Background(), "Series", nil, nil, storage.QualityProfile{}, nil)
		require.Error(t, err)
		assert.Equal(t, "season is nil", err.Error())
	})

	t.Run("missing season metadata", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)

		season := &storage.Season{
			Season: model.Season{
				ID:               1,
				SeriesID:         1,
				SeasonMetadataID: ptr(int32(999)), // Non-existent metadata ID
			},
		}

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		err := m.reconcileMissingSeason(ctx, "Series", season, nil, storage.QualityProfile{}, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found in storage")
	})

	t.Run("no episodes", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)

		seasonMetadata := model.SeasonMetadata{
			ID:     1,
			Title:  "Test Season",
			Number: 1,
		}
		_, err := store.CreateSeasonMetadata(ctx, seasonMetadata)
		require.NoError(t, err)

		season := &storage.Season{
			Season: model.Season{
				ID:               1,
				SeriesID:         1,
				SeasonMetadataID: ptr(int32(1)),
			},
		}

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		err = m.reconcileMissingSeason(ctx, "Series", season, nil, storage.QualityProfile{}, nil)
		require.NoError(t, err)
	})

	t.Run("successful season pack reconciliation", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		ctx := context.Background()
		store := newStore(t, ctx)

		mockDownloadClient := downloadMock.NewMockDownloadClient(ctrl)
		mockFactory := downloadMock.NewMockFactory(ctrl)

		downloadClientModel := model.DownloadClient{
			ID:             2,
			Implementation: "transmission",
			Type:           "torrent",
			Port:           8080,
			Host:           "transmission",
			Scheme:         "http",
		}

		mockFactory.EXPECT().NewDownloadClient(downloadClientModel).Return(mockDownloadClient, nil)
		mockDownloadClient.EXPECT().Add(ctx, gomock.Any()).Return(download.Status{
			ID:   "123",
			Name: "test download",
		}, nil)

		seriesMetadataID, err := store.CreateSeriesMetadata(ctx, model.SeriesMetadata{
			TmdbID:         1,
			Title:          "Series",
			Status:         "Continuing",
			EpisodeCount:   10,
			ExternalIds:    nil,
			WatchProviders: nil,
		})
		require.NoError(t, err)

		seriesID, err := store.CreateSeries(ctx, storage.Series{
			Series: model.Series{
				ID:               1,
				SeriesMetadataID: ptr(int32(seriesMetadataID)),
			},
		}, storage.SeriesStateMissing)
		require.NoError(t, err)

		_, err = store.GetSeries(ctx, table.Series.ID.EQ(sqlite.Int64(seriesID)))
		require.NoError(t, err)

		seasonID, err := store.CreateSeason(ctx, storage.Season{
			Season: model.Season{
				SeriesID:         1,
				SeasonMetadataID: ptr(int32(1)),
			},
		}, storage.SeasonStateMissing)
		require.NoError(t, err)

		season, err := store.GetSeason(ctx, table.Season.ID.EQ(sqlite.Int64(seasonID)))
		require.NoError(t, err)
		assert.Equal(t, int32(1), season.SeriesID)
		assert.Equal(t, ptr(int32(1)), season.SeasonMetadataID)

		seasonMetadataID, err := store.CreateSeasonMetadata(ctx, model.SeasonMetadata{
			SeriesMetadataID: int32(seriesID),
			Title:            "Season 1",
			Number:           1,
		})
		require.NoError(t, err)

		_, err = store.GetSeasonMetadata(ctx, table.SeasonMetadata.ID.EQ(sqlite.Int64(seasonMetadataID)))
		require.NoError(t, err)

		episodeMetadataID1, err := store.CreateEpisodeMetadata(ctx, model.EpisodeMetadata{
			TmdbID:           1,
			Title:            "Hello",
			Number:           1,
			SeasonMetadataID: int32(seasonID),
			AirDate:          ptr(time.Now().Add(time.Hour * -2)),
			Runtime:          ptr(int32(42)),
		})
		require.NoError(t, err)

		_, err = store.CreateEpisode(ctx, storage.Episode{
			Episode: model.Episode{
				SeasonID:          int32(seasonID),
				EpisodeNumber:     1,
				EpisodeMetadataID: ptr(int32(episodeMetadataID1)),
			},
		}, storage.EpisodeStateMissing)
		require.NoError(t, err)

		episodeMetadataID2, err := store.CreateEpisodeMetadata(ctx, model.EpisodeMetadata{
			TmdbID:           2,
			Title:            "There",
			Number:           2,
			SeasonMetadataID: int32(seasonID),
			AirDate:          ptr(time.Now().Add(time.Hour * -2)),
			Runtime:          ptr(int32(42)),
		})
		require.NoError(t, err)

		_, err = store.CreateEpisode(ctx, storage.Episode{
			Episode: model.Episode{
				SeasonID:          int32(seasonID),
				EpisodeNumber:     2,
				EpisodeMetadataID: ptr(int32(episodeMetadataID2)),
			},
		}, storage.EpisodeStateMissing)
		require.NoError(t, err)

		releases := []*prowlarr.ReleaseResource{
			{
				ID:       ptr(int32(1)),
				Title:    nullable.NewNullableWithValue("series.S01.1080p.WEB-DL.HEVC.x265"),
				Size:     sizeGBToBytes(2),
				Protocol: ptr(prowlarr.DownloadProtocolTorrent),
			},
		}

		qualityProfile := storage.QualityProfile{
			Name: "Default",
			Qualities: []storage.QualityDefinition{
				{
					Name:          "HD",
					MinSize:       15,
					MaxSize:       1000,
					PreferredSize: 995,
					MediaType:     "tv",
				},
			},
		}

		snapshot := newReconcileSnapshot([]model.Indexer{{ID: 1}}, []*model.DownloadClient{&downloadClientModel})

		m := New(nil, nil, nil, store, mockFactory, config.Manager{}, config.Config{})

		err = m.reconcileMissingSeason(ctx, "Series", season, snapshot, qualityProfile, releases)
		require.NoError(t, err)

		episodes, err := store.ListEpisodes(ctx)
		require.NoError(t, err)
		require.Len(t, episodes, 2)

		assert.Equal(t, storage.EpisodeStateDownloading, episodes[0].State)
		assert.Equal(t, "123", episodes[0].DownloadID)
		assert.Equal(t, int32(2), episodes[0].DownloadClientID)

		assert.Equal(t, storage.EpisodeStateDownloading, episodes[1].State)
		assert.Equal(t, "123", episodes[1].DownloadID)
		assert.Equal(t, int32(2), episodes[1].DownloadClientID)
	})

	t.Run("successful individual episode reconciliation", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		ctx := context.Background()
		store := newStore(t, ctx)

		mockDownloadClient := downloadMock.NewMockDownloadClient(ctrl)
		mockFactory := downloadMock.NewMockFactory(ctrl)

		downloadClientModel := model.DownloadClient{
			ID:             2,
			Implementation: "transmission",
			Type:           "torrent",
			Port:           8080,
			Host:           "transmission",
			Scheme:         "http",
		}

		snapshot := newReconcileSnapshot([]model.Indexer{{ID: 1}}, []*model.DownloadClient{&downloadClientModel})

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

		seriesMetadataID, err := store.CreateSeriesMetadata(ctx, model.SeriesMetadata{
			TmdbID:         1,
			Title:          "Series",
			Status:         "Continuing",
			EpisodeCount:   10,
			ExternalIds:    nil,
			WatchProviders: nil,
		})
		require.NoError(t, err)

		seriesID, err := store.CreateSeries(ctx, storage.Series{
			Series: model.Series{
				ID:               1,
				SeriesMetadataID: ptr(int32(seriesMetadataID)),
			},
		}, storage.SeriesStateMissing)
		require.NoError(t, err)

		_, err = store.GetSeries(ctx, table.Series.ID.EQ(sqlite.Int64(seriesID)))
		require.NoError(t, err)

		seasonID, err := store.CreateSeason(ctx, storage.Season{
			Season: model.Season{
				SeriesID:         1,
				SeasonMetadataID: ptr(int32(1)),
			},
		}, storage.SeasonStateMissing)
		require.NoError(t, err)

		season, err := store.GetSeason(ctx, table.Season.ID.EQ(sqlite.Int64(seasonID)))
		require.NoError(t, err)
		assert.Equal(t, int32(1), season.SeriesID)
		assert.Equal(t, ptr(int32(1)), season.SeasonMetadataID)

		seasonMetadataID, err := store.CreateSeasonMetadata(ctx, model.SeasonMetadata{
			SeriesMetadataID: int32(seriesID),
			Title:            "Season 1",
			Number:           1,
		})
		require.NoError(t, err)

		_, err = store.GetSeasonMetadata(ctx, table.SeasonMetadata.ID.EQ(sqlite.Int64(seasonMetadataID)))
		require.NoError(t, err)

		episodeMetadataID1, err := store.CreateEpisodeMetadata(ctx, model.EpisodeMetadata{
			TmdbID:           1,
			Title:            "Test",
			Number:           1,
			SeasonMetadataID: int32(seasonID),
			AirDate:          ptr(snapshot.time.Add(time.Hour * -2)),
			Runtime:          ptr(int32(42)),
		})
		require.NoError(t, err)

		_, err = store.CreateEpisode(ctx, storage.Episode{
			Episode: model.Episode{
				SeasonID:          int32(seasonID),
				EpisodeNumber:     1,
				EpisodeMetadataID: ptr(int32(episodeMetadataID1)),
			},
		}, storage.EpisodeStateMissing)
		require.NoError(t, err)

		episodeMetadataID2, err := store.CreateEpisodeMetadata(ctx, model.EpisodeMetadata{
			TmdbID:           2,
			Title:            "Testing",
			Number:           2,
			SeasonMetadataID: int32(seasonID),
			AirDate:          ptr(snapshot.time.Add(time.Hour * -2)),
			Runtime:          ptr(int32(42)),
		})
		require.NoError(t, err)

		_, err = store.CreateEpisode(ctx, storage.Episode{
			Episode: model.Episode{
				SeasonID:          int32(seasonID),
				EpisodeNumber:     2,
				EpisodeMetadataID: ptr(int32(episodeMetadataID2)),
			},
		}, storage.EpisodeStateMissing)
		require.NoError(t, err)

		releases := []*prowlarr.ReleaseResource{
			{
				ID:       ptr(int32(1)),
				Title:    nullable.NewNullableWithValue("Series.S01E01.1080p.WEB-DL.AAC2.0.x264-GROUP"),
				Size:     sizeGBToBytes(2),
				Protocol: ptr(prowlarr.DownloadProtocolTorrent),
			},
			{
				ID:       ptr(int32(2)),
				Title:    nullable.NewNullableWithValue("Series.S01E02.1080p.WEB-DL.AAC2.0.x264-GROUP"),
				Size:     sizeGBToBytes(2),
				Protocol: ptr(prowlarr.DownloadProtocolTorrent),
			},
		}

		qualityProfile := storage.QualityProfile{
			Name: "Default",
			Qualities: []storage.QualityDefinition{
				{
					Name:          "HD",
					MinSize:       15,
					MaxSize:       1000,
					PreferredSize: 995,
					MediaType:     "tv",
				},
			},
		}

		m := New(nil, nil, nil, store, mockFactory, config.Manager{}, config.Config{})

		err = m.reconcileMissingSeason(ctx, "Series", season, snapshot, qualityProfile, releases)
		require.NoError(t, err)

		episodes, err := store.ListEpisodes(ctx)
		require.NoError(t, err)
		require.Len(t, episodes, 2)

		assert.Equal(t, storage.EpisodeStateDownloading, episodes[0].State)
		assert.Equal(t, "123", episodes[0].DownloadID)
		assert.Equal(t, int32(2), episodes[0].DownloadClientID)

		assert.Equal(t, storage.EpisodeStateDownloading, episodes[1].State)
		assert.Equal(t, "124", episodes[1].DownloadID)
		assert.Equal(t, int32(2), episodes[1].DownloadClientID)
	})
}
func Test_getSeasonRuntime(t *testing.T) {
	tests := []struct {
		name                string
		episodeMetadata     []*model.EpisodeMetadata
		totalSeasonEpisodes int
		want                int32
	}{
		{
			name: "all episodes have runtime",
			episodeMetadata: []*model.EpisodeMetadata{
				{Runtime: ptr(int32(30))},
				{Runtime: ptr(int32(30))},
				{Runtime: ptr(int32(30))},
			},
			totalSeasonEpisodes: 3,
			want:                90,
		},
		{
			name: "some episodes missing runtime",
			episodeMetadata: []*model.EpisodeMetadata{
				{Runtime: ptr(int32(30))},
				{Runtime: nil},
				{Runtime: ptr(int32(30))},
			},
			totalSeasonEpisodes: 3,
			want:                90, // Average of 30 mins applied to missing episode
		},
		{
			name: "all episodes missing runtime",
			episodeMetadata: []*model.EpisodeMetadata{
				{Runtime: nil},
				{Runtime: nil},
				{Runtime: nil},
			},
			totalSeasonEpisodes: 3,
			want:                0,
		},
		{
			name:                "empty episode list",
			episodeMetadata:     []*model.EpisodeMetadata{},
			totalSeasonEpisodes: 0,
			want:                0,
		},
		{
			name: "more total episodes than provided",
			episodeMetadata: []*model.EpisodeMetadata{
				{Runtime: ptr(int32(30))},
				{Runtime: ptr(int32(30))},
			},
			totalSeasonEpisodes: 4,
			want:                120, // (30+30) + (30*2) for missing episodes
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getSeasonRuntime(tt.episodeMetadata, tt.totalSeasonEpisodes)
			assert.Equal(t, tt.want, got)
		})
	}
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

		store.EXPECT().ListIndexers(ctx).Return([]*model.Indexer{}, nil)

		indexerFactory := indexerMock.NewMockFactory(ctrl)

		m := New(nil, indexerFactory, nil, store, nil, config.Manager{}, config.Config{})
		snapshot := newReconcileSnapshot([]model.Indexer{}, nil)
		err := m.ReconcileMissingSeries(ctx, snapshot)
		require.NoError(t, err)
	})

	t.Run("error listing series", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		ctx := context.Background()
		store := storageMocks.NewMockStorage(ctrl)

		where := table.SeriesTransition.ToState.EQ(sqlite.String(string(storage.SeriesStateMissing))).
			AND(table.SeriesTransition.MostRecent.EQ(sqlite.Bool(true))).
			AND(table.Series.Monitored.EQ(sqlite.Int(1)))

		indexer := &model.Indexer{ID: 1, Name: "test", Priority: 1}
		store.EXPECT().ListIndexers(ctx).Return([]*model.Indexer{indexer}, nil)

		expectedErr := errors.New("database error")
		store.EXPECT().ListSeries(ctx, where).Return(nil, expectedErr)

		indexerFactory := indexerMock.NewMockFactory(ctrl)

		m := New(nil, indexerFactory, nil, store, nil, config.Manager{}, config.Config{})
		snapshot := newReconcileSnapshot([]model.Indexer{}, nil)
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

		downloadClientModel := model.DownloadClient{
			ID:             2,
			Implementation: "transmission",
			Type:           "torrent",
			Port:           8080,
			Host:           "transmission",
			Scheme:         "http",
		}

		snapshot := newReconcileSnapshot([]model.Indexer{{ID: 1}}, []*model.DownloadClient{&downloadClientModel})

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

		seriesMetadataID, err := store.CreateSeriesMetadata(ctx, model.SeriesMetadata{
			TmdbID:       1,
			Title:        "Series",
			EpisodeCount: 10,
		})
		require.NoError(t, err)

		seriesID, err := store.CreateSeries(ctx, storage.Series{
			Series: model.Series{
				ID:               1,
				SeriesMetadataID: ptr(int32(seriesMetadataID)),
				Monitored:        1,
				QualityProfileID: 4,
			},
		}, storage.SeriesStateMissing)
		require.NoError(t, err)

		_, err = store.GetSeries(ctx, table.Series.ID.EQ(sqlite.Int64(seriesID)))
		require.NoError(t, err)

		seasonID, err := store.CreateSeason(ctx, storage.Season{
			Season: model.Season{
				SeriesID:         1,
				SeasonMetadataID: ptr(int32(1)),
				Monitored:        1,
			},
		}, storage.SeasonStateMissing)
		require.NoError(t, err)

		season, err := store.GetSeason(ctx, table.Season.ID.EQ(sqlite.Int64(seasonID)))
		require.NoError(t, err)
		assert.Equal(t, int32(1), season.SeriesID)
		assert.Equal(t, ptr(int32(1)), season.SeasonMetadataID)

		_, err = store.CreateSeasonMetadata(ctx, model.SeasonMetadata{
			SeriesMetadataID: int32(seriesID),
			Title:            "Season 1",
			Number:           1,
		})
		require.NoError(t, err)

		episodeMetadataID1, err := store.CreateEpisodeMetadata(ctx, model.EpisodeMetadata{
			TmdbID:           1,
			Title:            "Hello",
			Number:           1,
			SeasonMetadataID: int32(seasonID),
			AirDate:          ptr(snapshot.time.Add(time.Hour * -2)),
			Runtime:          ptr(int32(42)),
		})
		require.NoError(t, err)

		_, err = store.CreateEpisode(ctx, storage.Episode{
			Episode: model.Episode{
				SeasonID:          int32(seasonID),
				EpisodeNumber:     1,
				EpisodeMetadataID: ptr(int32(episodeMetadataID1)),
				Monitored:         1,
			},
		}, storage.EpisodeStateMissing)
		require.NoError(t, err)

		episodeMetadataID2, err := store.CreateEpisodeMetadata(ctx, model.EpisodeMetadata{
			TmdbID:           2,
			Title:            "There",
			Number:           2,
			SeasonMetadataID: int32(seasonID),
			AirDate:          ptr(snapshot.time.Add(time.Hour * -2)),
			Runtime:          ptr(int32(42)),
		})
		require.NoError(t, err)

		_, err = store.CreateEpisode(ctx, storage.Episode{
			Episode: model.Episode{
				SeasonID:          int32(seasonID),
				EpisodeNumber:     2,
				EpisodeMetadataID: ptr(int32(episodeMetadataID2)),
			},
		}, storage.EpisodeStateMissing)
		require.NoError(t, err)

		releases := []*prowlarr.ReleaseResource{
			{
				ID:       ptr(int32(1)),
				Title:    nullable.NewNullableWithValue("Series.S01E01.1080p.WEB-DL.AAC2.0.x264-GROUP"),
				Size:     sizeGBToBytes(2),
				Protocol: ptr(prowlarr.DownloadProtocolTorrent),
			},
			{
				ID:       ptr(int32(2)),
				Title:    nullable.NewNullableWithValue("Series.S01E02.1080p.WEB-DL.AAC2.0.x264-GROUP"),
				Size:     sizeGBToBytes(2),
				Protocol: ptr(prowlarr.DownloadProtocolTorrent),
			},
		}

		mockIndexerSource := indexerMock.NewMockIndexerSource(ctrl)
		mockIndexerSource.EXPECT().Search(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(releases, nil).AnyTimes()

		indexerFactory := indexerMock.NewMockFactory(ctrl)
		indexerFactory.EXPECT().NewIndexerSource(gomock.Any()).Return(mockIndexerSource, nil).AnyTimes()

		// Create an indexer source in the database
		sourceID, err := store.CreateIndexerSource(ctx, model.IndexerSource{
			Name:           "test-source",
			Implementation: "prowlarr",
			Scheme:         "http",
			Host:           "test",
			Enabled:        true,
		})
		require.NoError(t, err)

		// Create indexers linked to the source
		_, err = store.CreateIndexer(ctx, model.Indexer{
			IndexerSourceID: ptr(int32(sourceID)),
			Name:            "test",
			Priority:        1,
			URI:             "http://test/indexer1",
		})
		require.NoError(t, err)

		m := New(nil, indexerFactory, nil, store, mockFactory, config.Manager{}, config.Config{})

		sourceIndexers := []indexer.SourceIndexer{
			{ID: 1, Name: "test", Priority: 1},
		}
		m.indexerCache.Set(sourceID, indexerCacheEntry{
			Indexers:   sourceIndexers,
			SourceName: "test-source",
			SourceURI:  "http://test",
		})

		err = m.ReconcileMissingSeries(ctx, snapshot)
		require.NoError(t, err)

		episodes, err := store.ListEpisodes(ctx)
		require.NoError(t, err)
		require.Len(t, episodes, 2)

		assert.Equal(t, storage.EpisodeStateDownloading, episodes[0].State)
		assert.Equal(t, "123", episodes[0].DownloadID)
		assert.Equal(t, int32(2), episodes[0].DownloadClientID)

		assert.Equal(t, storage.EpisodeStateDownloading, episodes[1].State)
		assert.Equal(t, "124", episodes[1].DownloadID)
		assert.Equal(t, int32(2), episodes[1].DownloadClientID)
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

		where := table.SeriesTransition.ToState.IN(
			sqlite.String(string(storage.SeriesStateContinuing)),
			sqlite.String(string(storage.SeriesStateDownloading)),
		).AND(table.SeriesTransition.MostRecent.EQ(sqlite.Bool(true))).
			AND(table.Series.Monitored.EQ(sqlite.Int(1)))

		store.EXPECT().ListSeries(ctx, where).Return(nil, storage.ErrNotFound)

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		err := m.ReconcileContinuingSeries(ctx, &ReconcileSnapshot{})
		require.NoError(t, err)
	})

	t.Run("error listing series", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		ctx := context.Background()
		store := storageMocks.NewMockStorage(ctrl)

		where := table.SeriesTransition.ToState.IN(
			sqlite.String(string(storage.SeriesStateContinuing)),
			sqlite.String(string(storage.SeriesStateDownloading)),
		).AND(table.SeriesTransition.MostRecent.EQ(sqlite.Bool(true))).
			AND(table.Series.Monitored.EQ(sqlite.Int(1)))

		expectedErr := errors.New("database error")
		store.EXPECT().ListSeries(ctx, where).Return(nil, expectedErr)

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

		downloadClientModel := model.DownloadClient{
			ID:             1,
			Implementation: "transmission",
			Type:           "torrent",
			Port:           8080,
			Host:           "transmission",
			Scheme:         "http",
		}

		snapshot := newReconcileSnapshot([]model.Indexer{{ID: 1}}, []*model.DownloadClient{&downloadClientModel})

		mockFactory.EXPECT().NewDownloadClient(gomock.Any()).Return(mockDownloadClient, nil).AnyTimes()
		mockDownloadClient.EXPECT().Add(ctx, gomock.Any()).Return(download.Status{
			ID:   "download-123",
			Name: "test download continuing",
		}, nil).AnyTimes()

		seriesMetadataID, err := store.CreateSeriesMetadata(ctx, model.SeriesMetadata{
			TmdbID:         1,
			Title:          "Continuing Series",
			Status:         "Continuing",
			EpisodeCount:   10,
			ExternalIds:    nil,
			WatchProviders: nil,
		})
		require.NoError(t, err)

		seriesID, err := store.CreateSeries(ctx, storage.Series{
			Series: model.Series{
				SeriesMetadataID: ptr(int32(seriesMetadataID)),
				Monitored:        1,
				QualityProfileID: 4, // Episode profile from defaults.sql
			},
		}, storage.SeriesStateMissing)
		require.NoError(t, err)

		// Transition to downloading state
		err = store.UpdateSeriesState(ctx, seriesID, storage.SeriesStateDownloading, nil)
		require.NoError(t, err)

		seasonMetadataID, err := store.CreateSeasonMetadata(ctx, model.SeasonMetadata{
			SeriesMetadataID: int32(seriesID),
			Title:            "Season 1",
			Number:           1,
		})
		require.NoError(t, err)

		seasonID, err := store.CreateSeason(ctx, storage.Season{
			Season: model.Season{
				SeriesID:         int32(seriesID),
				SeasonMetadataID: ptr(int32(seasonMetadataID)),
				Monitored:        1,
			},
		}, storage.SeasonStateMissing)
		require.NoError(t, err)

		// Transition to downloading state
		err = store.UpdateSeasonState(ctx, seasonID, storage.SeasonStateDownloading, nil)
		require.NoError(t, err)

		episodeMetadataID, err := store.CreateEpisodeMetadata(ctx, model.EpisodeMetadata{
			TmdbID:           1,
			Title:            "New Episode",
			Number:           3,
			SeasonMetadataID: int32(seasonID),
			AirDate:          ptr(snapshot.time.Add(time.Hour * -2)),
			Runtime:          ptr(int32(42)),
		})
		require.NoError(t, err)

		_, err = store.CreateEpisode(ctx, storage.Episode{
			Episode: model.Episode{
				SeasonID:          int32(seasonID),
				EpisodeNumber:     3,
				EpisodeMetadataID: ptr(int32(episodeMetadataID)),
				Monitored:         1,
			},
		}, storage.EpisodeStateMissing)
		require.NoError(t, err)

		releases := []*prowlarr.ReleaseResource{
			{
				ID:       ptr(int32(1)),
				Title:    nullable.NewNullableWithValue("Continuing.Series.S01E03.1080p.WEB-DL.AAC2.0.x264-GROUP"),
				Size:     sizeGBToBytes(2),
				Protocol: ptr(prowlarr.DownloadProtocolTorrent),
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

		snapshot := newReconcileSnapshot([]model.Indexer{{ID: 1}}, []*model.DownloadClient{})

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
		seriesMetadataID, err := store.CreateSeriesMetadata(ctx, model.SeriesMetadata{
			TmdbID:         100,
			Title:          "Weekly Series",
			SeasonCount:    1,
			EpisodeCount:   3,
			Status:         "Returning Series",
			ExternalIds:    nil,
			WatchProviders: nil,
		})
		require.NoError(t, err)

		seriesID, err := store.CreateSeries(ctx, storage.Series{
			Series: model.Series{
				SeriesMetadataID: ptr(int32(seriesMetadataID)),
				Monitored:        1,
				QualityProfileID: 4, // Episode profile from defaults.sql
			},
		}, storage.SeriesStateMissing)
		require.NoError(t, err)

		// Transition to downloading state
		err = store.UpdateSeriesState(ctx, seriesID, storage.SeriesStateDownloading, nil)
		require.NoError(t, err)

		// Transition to continuing state
		err = store.UpdateSeriesState(ctx, seriesID, storage.SeriesStateContinuing, nil)
		require.NoError(t, err)

		seasonMetadataID, err := store.CreateSeasonMetadata(ctx, model.SeasonMetadata{
			SeriesMetadataID: int32(seriesMetadataID), // Should reference the series metadata ID, not storage series ID
			Title:            "Season 1",
			Number:           1,
			TmdbID:           1001, // Match the TMDB ID from our mock
		})
		require.NoError(t, err)

		seasonID, err := store.CreateSeason(ctx, storage.Season{
			Season: model.Season{
				SeriesID:         int32(seriesID),
				SeasonMetadataID: ptr(int32(seasonMetadataID)),
				Monitored:        1,
			},
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

			episodeMetadataID, err := store.CreateEpisodeMetadata(ctx, model.EpisodeMetadata{
				TmdbID:           int32(1001000 + i), // Match the TMDB IDs from our mock (1001001, 1001002)
				Title:            fmt.Sprintf("Episode %d", i),
				Number:           int32(i),
				SeasonMetadataID: int32(seasonMetadataID), // This should be the season metadata ID
				AirDate:          airDate,
			})
			require.NoError(t, err)

			// Create episode records for episodes 1-2
			episodeID, err := store.CreateEpisode(ctx, storage.Episode{
				Episode: model.Episode{
					SeasonID:          int32(seasonID),
					EpisodeNumber:     int32(i),
					EpisodeMetadataID: ptr(int32(episodeMetadataID)),
					Monitored:         1,
				},
			}, storage.EpisodeStateMissing)
			require.NoError(t, err)

			// Transition episodes 1-2 to downloaded state via downloading to avoid them being searched for
			err = store.UpdateEpisodeState(ctx, episodeID, storage.EpisodeStateDownloading, nil)
			require.NoError(t, err)
			err = store.UpdateEpisodeState(ctx, episodeID, storage.EpisodeStateDownloaded, nil)
			require.NoError(t, err)
		}

		snapshot := newReconcileSnapshot([]model.Indexer{{ID: 1}}, []*model.DownloadClient{})

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
		episodesBefore, err := store.ListEpisodes(ctx, table.Episode.SeasonID.EQ(sqlite.Int64(seasonID)))
		require.NoError(t, err)
		assert.Len(t, episodesBefore, 2, "Should have 2 existing episodes before reconciliation")

		err = m.ReconcileContinuingSeries(ctx, snapshot)
		require.NoError(t, err)

		episodesAfter, err := store.ListEpisodes(ctx, table.Episode.SeasonID.EQ(sqlite.Int64(seasonID)))
		require.NoError(t, err)
		assert.Len(t, episodesAfter, 3, "Should have 3 episodes after reconciliation (episode 3 should be discovered)")

		// Check that episode 3 was created with correct state
		var episode3 *storage.Episode
		for _, ep := range episodesAfter {
			if ep.EpisodeNumber == 3 {
				episode3 = ep
				break
			}
		}
		require.NotNil(t, episode3, "Episode 3 should have been created")
		assert.Equal(t, storage.EpisodeStateUnreleased, episode3.State, "Episode 3 should be in unreleased state")
	})

}

func TestDetermineSeasonState(t *testing.T) {
	tests := []struct {
		name     string
		episodes []*storage.Episode
		expected storage.SeasonState
	}{
		{
			name:     "empty episodes",
			episodes: []*storage.Episode{},
			expected: storage.SeasonStateMissing,
		},
		{
			name: "all episodes downloaded",
			episodes: []*storage.Episode{
				{Episode: model.Episode{ID: 1}, State: storage.EpisodeStateDownloaded},
				{Episode: model.Episode{ID: 2}, State: storage.EpisodeStateDownloaded},
				{Episode: model.Episode{ID: 3}, State: storage.EpisodeStateDownloaded},
			},
			expected: storage.SeasonStateCompleted,
		},
		{
			name: "some episodes downloading",
			episodes: []*storage.Episode{
				{Episode: model.Episode{ID: 1}, State: storage.EpisodeStateDownloaded},
				{Episode: model.Episode{ID: 2}, State: storage.EpisodeStateDownloading},
				{Episode: model.Episode{ID: 3}, State: storage.EpisodeStateMissing},
			},
			expected: storage.SeasonStateDownloading,
		},
		{
			name: "one episode downloading others downloaded",
			episodes: []*storage.Episode{
				{Episode: model.Episode{ID: 1}, State: storage.EpisodeStateDownloaded},
				{Episode: model.Episode{ID: 2}, State: storage.EpisodeStateDownloaded},
				{Episode: model.Episode{ID: 3}, State: storage.EpisodeStateDownloading},
			},
			expected: storage.SeasonStateDownloading,
		},
		{
			name: "continuing - downloaded and unreleased",
			episodes: []*storage.Episode{
				{Episode: model.Episode{ID: 1}, State: storage.EpisodeStateDownloaded},
				{Episode: model.Episode{ID: 2}, State: storage.EpisodeStateDownloaded},
				{Episode: model.Episode{ID: 3}, State: storage.EpisodeStateUnreleased},
				{Episode: model.Episode{ID: 4}, State: storage.EpisodeStateUnreleased},
			},
			expected: storage.SeasonStateContinuing,
		},
		{
			name: "continuing - missing and unreleased",
			episodes: []*storage.Episode{
				{Episode: model.Episode{ID: 1}, State: storage.EpisodeStateMissing},
				{Episode: model.Episode{ID: 2}, State: storage.EpisodeStateMissing},
				{Episode: model.Episode{ID: 3}, State: storage.EpisodeStateUnreleased},
				{Episode: model.Episode{ID: 4}, State: storage.EpisodeStateUnreleased},
			},
			expected: storage.SeasonStateContinuing,
		},
		{
			name: "continuing - mix of downloaded, missing and unreleased",
			episodes: []*storage.Episode{
				{Episode: model.Episode{ID: 1}, State: storage.EpisodeStateDownloaded},
				{Episode: model.Episode{ID: 2}, State: storage.EpisodeStateMissing},
				{Episode: model.Episode{ID: 3}, State: storage.EpisodeStateUnreleased},
			},
			expected: storage.SeasonStateContinuing,
		},
		{
			name: "missing - all aired episodes missing, no unreleased",
			episodes: []*storage.Episode{
				{Episode: model.Episode{ID: 1}, State: storage.EpisodeStateMissing},
				{Episode: model.Episode{ID: 2}, State: storage.EpisodeStateMissing},
				{Episode: model.Episode{ID: 3}, State: storage.EpisodeStateMissing},
			},
			expected: storage.SeasonStateMissing,
		},
		{
			name: "missing - mix of downloaded and missing, no unreleased",
			episodes: []*storage.Episode{
				{Episode: model.Episode{ID: 1}, State: storage.EpisodeStateDownloaded},
				{Episode: model.Episode{ID: 2}, State: storage.EpisodeStateMissing},
				{Episode: model.Episode{ID: 3}, State: storage.EpisodeStateMissing},
			},
			expected: storage.SeasonStateMissing,
		},
		{
			name: "unreleased - all episodes unreleased",
			episodes: []*storage.Episode{
				{Episode: model.Episode{ID: 1}, State: storage.EpisodeStateUnreleased},
				{Episode: model.Episode{ID: 2}, State: storage.EpisodeStateUnreleased},
				{Episode: model.Episode{ID: 3}, State: storage.EpisodeStateUnreleased},
			},
			expected: storage.SeasonStateUnreleased,
		},
		{
			name: "downloading takes priority over continuing",
			episodes: []*storage.Episode{
				{Episode: model.Episode{ID: 1}, State: storage.EpisodeStateDownloaded},
				{Episode: model.Episode{ID: 2}, State: storage.EpisodeStateDownloading},
				{Episode: model.Episode{ID: 3}, State: storage.EpisodeStateUnreleased},
			},
			expected: storage.SeasonStateDownloading,
		},
		{
			name: "downloading takes priority over all other states",
			episodes: []*storage.Episode{
				{Episode: model.Episode{ID: 1}, State: storage.EpisodeStateDownloaded},
				{Episode: model.Episode{ID: 2}, State: storage.EpisodeStateDownloading},
				{Episode: model.Episode{ID: 3}, State: storage.EpisodeStateMissing},
				{Episode: model.Episode{ID: 4}, State: storage.EpisodeStateUnreleased},
			},
			expected: storage.SeasonStateDownloading,
		},
		{
			name: "single downloaded episode",
			episodes: []*storage.Episode{
				{Episode: model.Episode{ID: 1}, State: storage.EpisodeStateDownloaded},
			},
			expected: storage.SeasonStateCompleted,
		},
		{
			name: "single missing episode",
			episodes: []*storage.Episode{
				{Episode: model.Episode{ID: 1}, State: storage.EpisodeStateMissing},
			},
			expected: storage.SeasonStateMissing,
		},
		{
			name: "single unreleased episode",
			episodes: []*storage.Episode{
				{Episode: model.Episode{ID: 1}, State: storage.EpisodeStateUnreleased},
			},
			expected: storage.SeasonStateUnreleased,
		},
		{
			name: "single downloading episode",
			episodes: []*storage.Episode{
				{Episode: model.Episode{ID: 1}, State: storage.EpisodeStateDownloading},
			},
			expected: storage.SeasonStateDownloading,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, result := determineSeasonState(tt.episodes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDetermineSeasonStateWithCounts(t *testing.T) {
	tests := []struct {
		name           string
		episodes       []*storage.Episode
		expectedState  storage.SeasonState
		expectedCounts map[string]int
	}{
		{
			name:          "empty episodes",
			episodes:      []*storage.Episode{},
			expectedState: storage.SeasonStateMissing,
			expectedCounts: map[string]int{
				"done":        0,
				"downloading": 0,
				"missing":     0,
				"unreleased":  0,
				"discovered":  0,
			},
		},
		{
			name: "mix of all states",
			episodes: []*storage.Episode{
				{Episode: model.Episode{ID: 1}, State: storage.EpisodeStateDownloaded},
				{Episode: model.Episode{ID: 2}, State: storage.EpisodeStateDownloaded},
				{Episode: model.Episode{ID: 3}, State: storage.EpisodeStateDownloading},
				{Episode: model.Episode{ID: 4}, State: storage.EpisodeStateMissing},
				{Episode: model.Episode{ID: 5}, State: storage.EpisodeStateMissing},
				{Episode: model.Episode{ID: 6}, State: storage.EpisodeStateUnreleased},
			},
			expectedState: storage.SeasonStateDownloading,
			expectedCounts: map[string]int{
				"done":        2,
				"downloading": 1,
				"missing":     2,
				"unreleased":  1,
				"discovered":  0,
			},
		},
		{
			name: "discovered with completed episodes",
			episodes: []*storage.Episode{
				{Episode: model.Episode{ID: 1}, State: storage.EpisodeStateDiscovered},
				{Episode: model.Episode{ID: 2}, State: storage.EpisodeStateCompleted},
				{Episode: model.Episode{ID: 3}, State: storage.EpisodeStateDiscovered},
			},
			expectedState: storage.SeasonStateContinuing,
			expectedCounts: map[string]int{
				"done":        1,
				"downloading": 0,
				"missing":     0,
				"unreleased":  0,
				"discovered":  2,
			},
		},
		{
			name: "all discovered episodes",
			episodes: []*storage.Episode{
				{Episode: model.Episode{ID: 1}, State: storage.EpisodeStateDiscovered},
				{Episode: model.Episode{ID: 2}, State: storage.EpisodeStateDiscovered},
			},
			expectedState: storage.SeasonStateDiscovered,
			expectedCounts: map[string]int{
				"done":        0,
				"downloading": 0,
				"missing":     0,
				"unreleased":  0,
				"discovered":  2,
			},
		},
		{
			name: "continuing season counts",
			episodes: []*storage.Episode{
				{Episode: model.Episode{ID: 1}, State: storage.EpisodeStateDownloaded},
				{Episode: model.Episode{ID: 2}, State: storage.EpisodeStateMissing},
				{Episode: model.Episode{ID: 3}, State: storage.EpisodeStateUnreleased},
				{Episode: model.Episode{ID: 4}, State: storage.EpisodeStateUnreleased},
			},
			expectedState: storage.SeasonStateContinuing,
			expectedCounts: map[string]int{
				"done":        1,
				"downloading": 0,
				"missing":     1,
				"unreleased":  2,
				"discovered":  0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			counts, state := determineSeasonState(tt.episodes)
			assert.Equal(t, tt.expectedState, state)
			assert.Equal(t, tt.expectedCounts, counts)
		})
	}
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

		where := table.EpisodeTransition.ToState.EQ(sqlite.String(string(storage.EpisodeStateDiscovered))).
			AND(table.EpisodeTransition.MostRecent.EQ(sqlite.Bool(true))).
			AND(table.Episode.Monitored.EQ(sqlite.Int(1)))

		store.EXPECT().ListEpisodes(ctx, where).Return(nil, storage.ErrNotFound)

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

		where := table.EpisodeTransition.ToState.EQ(sqlite.String(string(storage.EpisodeStateDiscovered))).
			AND(table.EpisodeTransition.MostRecent.EQ(sqlite.Bool(true))).
			AND(table.Episode.Monitored.EQ(sqlite.Int(1)))

		expectedErr := errors.New("database error")
		store.EXPECT().ListEpisodes(ctx, where).Return(nil, expectedErr)

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
		seriesMetadataID, err := store.CreateSeriesMetadata(ctx, model.SeriesMetadata{
			TmdbID:       1,
			Title:        "Test Series",
			EpisodeCount: 5,
		})
		require.NoError(t, err)

		// Create a series with path and linked metadata
		seriesID, err := store.CreateSeries(ctx, storage.Series{
			Series: model.Series{
				SeriesMetadataID: ptr(int32(seriesMetadataID)),
				Path:             ptr("test-series"),
				Monitored:        1,
			},
		}, storage.SeriesStateMissing)
		require.NoError(t, err)

		// Create season metadata
		seasonMetadataID, err := store.CreateSeasonMetadata(ctx, model.SeasonMetadata{
			SeriesMetadataID: int32(seriesMetadataID),
			Title:            "Season 1",
			Number:           1,
		})
		require.NoError(t, err)

		// Create season with metadata linked
		seasonID, err := store.CreateSeason(ctx, storage.Season{
			Season: model.Season{
				SeriesID:         int32(seriesID),
				SeasonNumber:     1,
				SeasonMetadataID: ptr(int32(seasonMetadataID)),
				Monitored:        1,
			},
		}, storage.SeasonStateMissing)
		require.NoError(t, err)

		// Create episode metadata
		episodeMetadataID, err := store.CreateEpisodeMetadata(ctx, model.EpisodeMetadata{
			SeasonMetadataID: int32(seasonMetadataID),
			Title:            "Episode 1",
			Number:           1,
		})
		require.NoError(t, err)

		// Create a discovered episode with metadata linked
		episodeID, err := store.CreateEpisode(ctx, storage.Episode{
			Episode: model.Episode{
				SeasonID:          int32(seasonID),
				EpisodeNumber:     1,
				EpisodeMetadataID: ptr(int32(episodeMetadataID)),
				Monitored:         1,
			},
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
		episode, err := store.GetEpisode(ctx, table.Episode.ID.EQ(sqlite.Int64(episodeID)))
		require.NoError(t, err)
		assert.Equal(t, storage.EpisodeStateCompleted, episode.State)
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
		seriesID, err := store.CreateSeries(ctx, storage.Series{
			Series: model.Series{
				Path:      nil, // This will cause the reconcile to skip
				Monitored: 1,
			},
		}, storage.SeriesStateMissing)
		require.NoError(t, err)

		seasonID, err := store.CreateSeason(ctx, storage.Season{
			Season: model.Season{
				SeriesID:  int32(seriesID),
				Monitored: 1,
			},
		}, storage.SeasonStateMissing)
		require.NoError(t, err)

		_, err = store.CreateEpisode(ctx, storage.Episode{
			Episode: model.Episode{
				SeasonID:      int32(seasonID),
				EpisodeNumber: 1,
				Monitored:     1,
			},
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
		store := newStore(t, ctx)

		// Create season metadata first
		seasonMetadataID, err := store.CreateSeasonMetadata(ctx, model.SeasonMetadata{
			TmdbID: 1,
			Title:  "Season 1",
			Number: 1,
		})
		require.NoError(t, err)

		// Create episode metadata for episodes 1,2,3
		episodeMetadataID1, err := store.CreateEpisodeMetadata(ctx, model.EpisodeMetadata{TmdbID: 1, Title: "Episode 1", Number: 1, SeasonMetadataID: int32(seasonMetadataID)})
		require.NoError(t, err)
		episodeMetadataID2, err := store.CreateEpisodeMetadata(ctx, model.EpisodeMetadata{TmdbID: 2, Title: "Episode 2", Number: 2, SeasonMetadataID: int32(seasonMetadataID)})
		require.NoError(t, err)
		episodeMetadataID3, err := store.CreateEpisodeMetadata(ctx, model.EpisodeMetadata{TmdbID: 3, Title: "Episode 3", Number: 3, SeasonMetadataID: int32(seasonMetadataID)})
		require.NoError(t, err)

		// Create episodes
		episodes := []*storage.Episode{
			{Episode: model.Episode{ID: 1, EpisodeMetadataID: ptr(int32(episodeMetadataID1))}},
			{Episode: model.Episode{ID: 2, EpisodeMetadataID: ptr(int32(episodeMetadataID2))}},
			{Episode: model.Episode{ID: 3, EpisodeMetadataID: ptr(int32(episodeMetadataID3))}},
		}

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		result := m.matchEpisodeFileToEpisode(ctx, "/downloads/Series.Name.S01E03.1080p.WEB-DL.x264-GROUP.mkv", episodes)
		require.NotNil(t, result)
		assert.Equal(t, int32(3), result.ID)
	})

	t.Run("matches 1x05 format", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)

		seasonMetadataID, err := store.CreateSeasonMetadata(ctx, model.SeasonMetadata{TmdbID: 2, Title: "Season 1", Number: 1})
		require.NoError(t, err)

		episodeMetadataID4, err := store.CreateEpisodeMetadata(ctx, model.EpisodeMetadata{TmdbID: 4, Title: "Episode 4", Number: 4, SeasonMetadataID: int32(seasonMetadataID)})
		require.NoError(t, err)
		episodeMetadataID5, err := store.CreateEpisodeMetadata(ctx, model.EpisodeMetadata{TmdbID: 5, Title: "Episode 5", Number: 5, SeasonMetadataID: int32(seasonMetadataID)})
		require.NoError(t, err)

		episodes := []*storage.Episode{
			{Episode: model.Episode{ID: 4, EpisodeMetadataID: ptr(int32(episodeMetadataID4))}},
			{Episode: model.Episode{ID: 5, EpisodeMetadataID: ptr(int32(episodeMetadataID5))}},
		}

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		result := m.matchEpisodeFileToEpisode(ctx, "/downloads/Series.Name.1x05.720p.HDTV.x264-GROUP.mkv", episodes)
		require.NotNil(t, result)
		assert.Equal(t, int32(5), result.ID)
	})

	t.Run("no match when episode number not found", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)

		episodeMetadataID6, err := store.CreateEpisodeMetadata(ctx, model.EpisodeMetadata{TmdbID: 6, Title: "Episode 1", Number: 1, SeasonMetadataID: 1})
		require.NoError(t, err)

		episodes := []*storage.Episode{{Episode: model.Episode{ID: 6, EpisodeMetadataID: ptr(int32(episodeMetadataID6))}}}

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		result := m.matchEpisodeFileToEpisode(ctx, "/downloads/Series.Name.Some.Random.File.mkv", episodes)
		assert.Nil(t, result)
	})

	t.Run("no match when episode number doesn't exist in episodes", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)

		episodeMetadataID7, err := store.CreateEpisodeMetadata(ctx, model.EpisodeMetadata{TmdbID: 7, Title: "Episode 1", Number: 1, SeasonMetadataID: 1})
		require.NoError(t, err)
		episodeMetadataID8, err := store.CreateEpisodeMetadata(ctx, model.EpisodeMetadata{TmdbID: 8, Title: "Episode 2", Number: 2, SeasonMetadataID: 1})
		require.NoError(t, err)

		episodes := []*storage.Episode{
			{Episode: model.Episode{ID: 7, EpisodeMetadataID: ptr(int32(episodeMetadataID7))}},
			{Episode: model.Episode{ID: 8, EpisodeMetadataID: ptr(int32(episodeMetadataID8))}},
		}

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		result := m.matchEpisodeFileToEpisode(ctx, "/downloads/Series.Name.S01E10.1080p.WEB-DL.x264-GROUP.mkv", episodes)
		assert.Nil(t, result)
	})
}
