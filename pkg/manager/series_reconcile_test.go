package manager

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/download"
	downloadMock "github.com/kasuboski/mediaz/pkg/download/mocks"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	prowlMock "github.com/kasuboski/mediaz/pkg/prowlarr/mocks"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/mocks"
	mediaSqlite "github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
	"github.com/oapi-codegen/nullable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestMediaManager_updateEpisodeState(t *testing.T) {
	t.Run("update episode state", func(t *testing.T) {
		ctx := context.Background()
		store, err := mediaSqlite.New(":memory:")
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
				Runtime:           ptr(int32(100)),
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
		assert.Equal(t, ptr(int32(100)), foundEpisode.Runtime)
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

		snapshot := newReconcileSnapshot([]Indexer{{ID: 1}}, []*model.DownloadClient{&downloadClientModel})

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
			TmdbID:  1,
			Title:   "Test Episode",
			Number:  1,
			Runtime: ptr(int32(45)),
			// time in the past
			AirDate: ptr(snapshot.time.Add(time.Hour * -2)),
		}

		metadataID1, err := store.CreateEpisodeMetadata(ctx, episodeMetadata)
		require.NoError(t, err)

		episode1 := storage.Episode{
			Episode: model.Episode{
				SeasonID:          1,
				EpisodeNumber:     1,
				EpisodeMetadataID: ptr(int32(metadataID1)),
				Runtime:           ptr(int32(45)),
			},
		}

		_, err = store.CreateEpisode(ctx, episode1, storage.EpisodeStateMissing)
		require.NoError(t, err)

		episodeMetadata = model.EpisodeMetadata{
			TmdbID:  2,
			Title:   "Test Episode 2",
			Number:  2,
			Runtime: ptr(int32(47)),
			// time in the past
			AirDate: ptr(snapshot.time.Add(time.Hour * -2)),
		}

		metadataID2, err := store.CreateEpisodeMetadata(ctx, episodeMetadata)
		require.NoError(t, err)

		episode2 := storage.Episode{
			Episode: model.Episode{
				SeasonID:          1,
				EpisodeNumber:     2,
				EpisodeMetadataID: ptr(int32(metadataID2)),
				Runtime:           ptr(int32(45)),
			},
		}
		_, err = store.CreateEpisode(ctx, episode2, storage.EpisodeStateMissing)
		require.NoError(t, err)

		episodeMetadata = model.EpisodeMetadata{
			TmdbID:  3,
			Title:   "Test Episode 3",
			Number:  3,
			Runtime: ptr(int32(47)),
			// future time
			AirDate: ptr(snapshot.time.Add(time.Hour * 2)),
		}

		metadataID3, err := store.CreateEpisodeMetadata(ctx, episodeMetadata)
		require.NoError(t, err)

		episode3 := storage.Episode{
			Episode: model.Episode{
				SeasonID:          1,
				EpisodeNumber:     3,
				EpisodeMetadataID: ptr(int32(metadataID3)),
				Runtime:           ptr(int32(45)),
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

		m := New(nil, nil, nil, store, mockFactory, config.Manager{})

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

		snapshot := newReconcileSnapshot([]Indexer{{ID: 1}}, []*model.DownloadClient{&downloadClientModel})

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
			TmdbID:  1,
			Title:   "Test Episode",
			Number:  1,
			Runtime: ptr(int32(45)),
			// time in the past
			AirDate: ptr(snapshot.time.Add(time.Hour * -2)),
		}

		metadataID1, err := store.CreateEpisodeMetadata(ctx, episodeMetadata)
		require.NoError(t, err)

		episode1 := storage.Episode{
			Episode: model.Episode{
				SeasonID:          int32(seasonID),
				EpisodeNumber:     1,
				EpisodeMetadataID: ptr(int32(metadataID1)),
				Runtime:           ptr(int32(45)),
			},
		}

		_, err = store.CreateEpisode(ctx, episode1, storage.EpisodeStateMissing)
		require.NoError(t, err)

		episodeMetadata = model.EpisodeMetadata{
			TmdbID:  2,
			Title:   "Test Episode 2",
			Number:  2,
			Runtime: ptr(int32(47)),
			// time in the past
			AirDate: ptr(snapshot.time.Add(time.Hour * -2)),
		}

		metadataID2, err := store.CreateEpisodeMetadata(ctx, episodeMetadata)
		require.NoError(t, err)

		episode2 := storage.Episode{
			Episode: model.Episode{
				SeasonID:          int32(seasonID),
				EpisodeNumber:     2,
				EpisodeMetadataID: ptr(int32(metadataID2)),
				Runtime:           ptr(int32(45)),
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

		m := New(nil, nil, nil, store, mockFactory, config.Manager{})

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
		m := New(nil, nil, nil, nil, nil, config.Manager{})
		err := m.reconcileMissingEpisodes(context.Background(), "Series", 1, 1, []*storage.Episode{nil}, nil, storage.QualityProfile{}, nil)
		require.NoError(t, err)
	})

	t.Run("nil snapshot", func(t *testing.T) {
		m := New(nil, nil, nil, nil, nil, config.Manager{})
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

		m := New(nil, nil, nil, store, nil, config.Manager{})
		err = m.reconcileMissingEpisodes(ctx, "Series", 1, 1, []*storage.Episode{&episode}, &ReconcileSnapshot{}, storage.QualityProfile{}, nil)
		require.NoError(t, err)
	})

	t.Run("no matching releases", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)

		episodeMetadata := model.EpisodeMetadata{
			ID:      1,
			Title:   "Test Episode",
			Number:  1,
			Runtime: ptr(int32(45)),
		}

		_, err := store.CreateEpisodeMetadata(ctx, episodeMetadata)
		require.NoError(t, err)

		episode := storage.Episode{
			Episode: model.Episode{
				EpisodeMetadataID: ptr(int32(1)),
			},
		}

		m := New(nil, nil, nil, store, nil, config.Manager{})
		err = m.reconcileMissingEpisodes(ctx, "Series", 1, 1, []*storage.Episode{&episode}, &ReconcileSnapshot{}, storage.QualityProfile{}, nil)
		require.NoError(t, err)
	})
}
func TestMediaManager_reconcileMissingSeason(t *testing.T) {
	t.Run("nil season", func(t *testing.T) {
		m := New(nil, nil, nil, nil, nil, config.Manager{})
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

		m := New(nil, nil, nil, store, nil, config.Manager{})
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

		m := New(nil, nil, nil, store, nil, config.Manager{})
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
			TmdbID:       1,
			Title:        "Series",
			EpisodeCount: 10,
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
			SeriesID: int32(seriesID),
			Title:    "Season 1",
			Number:   1,
		})
		require.NoError(t, err)

		_, err = store.GetSeasonMetadata(ctx, table.SeasonMetadata.ID.EQ(sqlite.Int64(seasonMetadataID)))
		require.NoError(t, err)

		episodeMetadataID1, err := store.CreateEpisodeMetadata(ctx, model.EpisodeMetadata{
			TmdbID:   1,
			Title:    "Hello",
			Number:   1,
			SeasonID: int32(seasonID),
			Runtime:  ptr(int32(45)),
		})
		require.NoError(t, err)

		_, err = store.CreateEpisode(ctx, storage.Episode{
			Episode: model.Episode{
				SeasonID:          int32(seasonID),
				EpisodeNumber:     1,
				EpisodeMetadataID: ptr(int32(episodeMetadataID1)),
				Runtime:           ptr(int32(45)),
			},
		}, storage.EpisodeStateMissing)
		require.NoError(t, err)

		episodeMetadataID2, err := store.CreateEpisodeMetadata(ctx, model.EpisodeMetadata{
			TmdbID:   2,
			Title:    "There",
			Number:   2,
			SeasonID: int32(seasonID),
			Runtime:  ptr(int32(45)),
		})
		require.NoError(t, err)

		_, err = store.CreateEpisode(ctx, storage.Episode{
			Episode: model.Episode{
				SeasonID:          int32(seasonID),
				EpisodeNumber:     2,
				EpisodeMetadataID: ptr(int32(episodeMetadataID2)),
				Runtime:           ptr(int32(45)),
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

		snapshot := newReconcileSnapshot([]Indexer{{ID: 1}}, []*model.DownloadClient{&downloadClientModel})

		m := New(nil, nil, nil, store, mockFactory, config.Manager{})

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

		snapshot := newReconcileSnapshot([]Indexer{{ID: 1}}, []*model.DownloadClient{&downloadClientModel})

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
			TmdbID:       1,
			Title:        "Series",
			EpisodeCount: 10,
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
			SeriesID: int32(seriesID),
			Title:    "Season 1",
			Number:   1,
		})
		require.NoError(t, err)

		_, err = store.GetSeasonMetadata(ctx, table.SeasonMetadata.ID.EQ(sqlite.Int64(seasonMetadataID)))
		require.NoError(t, err)

		episodeMetadataID1, err := store.CreateEpisodeMetadata(ctx, model.EpisodeMetadata{
			TmdbID:   1,
			Title:    "Test",
			Number:   1,
			SeasonID: int32(seasonID),
			Runtime:  ptr(int32(45)),
			AirDate:  ptr(snapshot.time.Add(time.Hour * -2)),
		})
		require.NoError(t, err)

		_, err = store.CreateEpisode(ctx, storage.Episode{
			Episode: model.Episode{
				SeasonID:          int32(seasonID),
				EpisodeNumber:     1,
				EpisodeMetadataID: ptr(int32(episodeMetadataID1)),
				Runtime:           ptr(int32(45)),
			},
		}, storage.EpisodeStateMissing)
		require.NoError(t, err)

		episodeMetadataID2, err := store.CreateEpisodeMetadata(ctx, model.EpisodeMetadata{
			TmdbID:   2,
			Title:    "Testing",
			Number:   2,
			SeasonID: int32(seasonID),
			Runtime:  ptr(int32(45)),
			AirDate:  ptr(snapshot.time.Add(time.Hour * -2)),
		})
		require.NoError(t, err)

		_, err = store.CreateEpisode(ctx, storage.Episode{
			Episode: model.Episode{
				SeasonID:          int32(seasonID),
				EpisodeNumber:     2,
				EpisodeMetadataID: ptr(int32(episodeMetadataID2)),
				Runtime:           ptr(int32(45)),
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

		m := New(nil, nil, nil, store, mockFactory, config.Manager{})

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
		episodes            []*storage.Episode
		totalSeasonEpisodes int
		want                int32
	}{
		{
			name: "all episodes have runtime",
			episodes: []*storage.Episode{
				{Episode: model.Episode{Runtime: ptr(int32(30))}},
				{Episode: model.Episode{Runtime: ptr(int32(30))}},
				{Episode: model.Episode{Runtime: ptr(int32(30))}},
			},
			totalSeasonEpisodes: 3,
			want:                90,
		},
		{
			name: "some episodes missing runtime",
			episodes: []*storage.Episode{
				{Episode: model.Episode{Runtime: ptr(int32(30))}},
				{Episode: model.Episode{Runtime: nil}},
				{Episode: model.Episode{Runtime: ptr(int32(30))}},
			},
			totalSeasonEpisodes: 3,
			want:                90, // Average of 30 mins applied to missing episode
		},
		{
			name: "all episodes missing runtime",
			episodes: []*storage.Episode{
				{Episode: model.Episode{Runtime: nil}},
				{Episode: model.Episode{Runtime: nil}},
				{Episode: model.Episode{Runtime: nil}},
			},
			totalSeasonEpisodes: 3,
			want:                0,
		},
		{
			name:                "empty episode list",
			episodes:            []*storage.Episode{},
			totalSeasonEpisodes: 0,
			want:                0,
		},
		{
			name: "more total episodes than provided",
			episodes: []*storage.Episode{
				{Episode: model.Episode{Runtime: ptr(int32(30))}},
				{Episode: model.Episode{Runtime: ptr(int32(30))}},
			},
			totalSeasonEpisodes: 4,
			want:                120, // (30+30) + (30*2) for missing episodes
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getSeasonRuntime(tt.episodes, tt.totalSeasonEpisodes)
			assert.Equal(t, tt.want, got)
		})
	}
}
func TestMediaManager_ReconcileMissingSeries(t *testing.T) {
	t.Run("nil snapshot", func(t *testing.T) {
		m := New(nil, nil, nil, nil, nil, config.Manager{})
		err := m.ReconcileMissingSeries(context.Background(), nil)
		require.NoError(t, err)
	})

	t.Run("no missing series", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		ctx := context.Background()
		store := mocks.NewMockStorage(ctrl)

		where := table.SeriesTransition.ToState.EQ(sqlite.String(string(storage.SeriesStateMissing))).
			AND(table.SeriesTransition.MostRecent.EQ(sqlite.Bool(true))).
			AND(table.Series.Monitored.EQ(sqlite.Int(1)))

		store.EXPECT().ListSeries(ctx, where).Return(nil, storage.ErrNotFound)

		m := New(nil, nil, nil, store, nil, config.Manager{})
		err := m.ReconcileMissingSeries(ctx, &ReconcileSnapshot{})
		require.NoError(t, err)
	})

	t.Run("error listing series", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		ctx := context.Background()
		store := mocks.NewMockStorage(ctrl)

		where := table.SeriesTransition.ToState.EQ(sqlite.String(string(storage.SeriesStateMissing))).
			AND(table.SeriesTransition.MostRecent.EQ(sqlite.Bool(true))).
			AND(table.Series.Monitored.EQ(sqlite.Int(1)))

		expectedErr := errors.New("database error")
		store.EXPECT().ListSeries(ctx, where).Return(nil, expectedErr)

		m := New(nil, nil, nil, store, nil, config.Manager{})
		err := m.ReconcileMissingSeries(ctx, &ReconcileSnapshot{})
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

		snapshot := newReconcileSnapshot([]Indexer{{ID: 1}}, []*model.DownloadClient{&downloadClientModel})

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
			SeriesID: int32(seriesID),
			Title:    "Season 1",
			Number:   1,
		})
		require.NoError(t, err)

		episodeMetadataID1, err := store.CreateEpisodeMetadata(ctx, model.EpisodeMetadata{
			TmdbID:   1,
			Title:    "Hello",
			Number:   1,
			SeasonID: int32(seasonID),
			Runtime:  ptr(int32(45)),
			AirDate:  ptr(snapshot.time.Add(time.Hour * -2)),
		})
		require.NoError(t, err)

		_, err = store.CreateEpisode(ctx, storage.Episode{
			Episode: model.Episode{
				SeasonID:          int32(seasonID),
				EpisodeNumber:     1,
				EpisodeMetadataID: ptr(int32(episodeMetadataID1)),
				Runtime:           ptr(int32(45)),
				Monitored:         1,
			},
		}, storage.EpisodeStateMissing)
		require.NoError(t, err)

		episodeMetadataID2, err := store.CreateEpisodeMetadata(ctx, model.EpisodeMetadata{
			TmdbID:   2,
			Title:    "There",
			Number:   2,
			SeasonID: int32(seasonID),
			Runtime:  ptr(int32(45)),
			AirDate:  ptr(snapshot.time.Add(time.Hour * -2)),
		})
		require.NoError(t, err)

		_, err = store.CreateEpisode(ctx, storage.Episode{
			Episode: model.Episode{
				SeasonID:          int32(seasonID),
				EpisodeNumber:     2,
				EpisodeMetadataID: ptr(int32(episodeMetadataID2)),
				Runtime:           ptr(int32(45)),
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

		prowlarrMock := prowlMock.NewMockClientInterface(ctrl)
		prowlarrMock.EXPECT().GetAPIV1Search(gomock.Any(), gomock.Any()).Return(searchIndexersResponse(t, releases), nil).Times(1)

		pClient, err := prowlarr.New(":", "1234")
		require.NoError(t, err)
		pClient.ClientInterface = prowlarrMock

		m := New(nil, pClient, nil, store, mockFactory, config.Manager{})

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
