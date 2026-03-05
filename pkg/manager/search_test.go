package manager

import (
	"context"
	"errors"
	"testing"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/config"
	downloadMock "github.com/kasuboski/mediaz/pkg/download/mocks"
	"github.com/kasuboski/mediaz/pkg/indexer"
	indexerMock "github.com/kasuboski/mediaz/pkg/indexer/mocks"
	"github.com/kasuboski/mediaz/pkg/storage"
	storageMocks "github.com/kasuboski/mediaz/pkg/storage/mocks"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestMediaManager_SearchForMovie(t *testing.T) {
	t.Run("movie not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storageMocks.NewMockStorage(ctrl)
		store.EXPECT().GetMovie(gomock.Any(), int64(1)).Return(nil, storage.ErrNotFound)

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		err := m.SearchForMovie(context.Background(), 1)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "movie not found")
	})

	t.Run("movie not monitored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storageMocks.NewMockStorage(ctrl)
		store.EXPECT().GetMovie(gomock.Any(), int64(1)).Return(&storage.Movie{
			Movie: model.Movie{
				ID:        1,
				Monitored: 0,
			},
		}, nil)

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		err := m.SearchForMovie(context.Background(), 1)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not monitored")
	})

	t.Run("no download clients available", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storageMocks.NewMockStorage(ctrl)
		store.EXPECT().GetMovie(gomock.Any(), int64(1)).Return(&storage.Movie{
			Movie: model.Movie{
				ID:        1,
				Monitored: 1,
			},
		}, nil)
		store.EXPECT().ListDownloadClients(gomock.Any()).Return(nil, errors.New("no clients"))

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		err := m.SearchForMovie(context.Background(), 1)

		assert.Error(t, err)
	})

	t.Run("no indexers available", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storageMocks.NewMockStorage(ctrl)
		indexerFactory := indexerMock.NewMockFactory(ctrl)
		downloadFactory := downloadMock.NewMockFactory(ctrl)

		store.EXPECT().GetMovie(gomock.Any(), int64(1)).Return(&storage.Movie{
			Movie: model.Movie{
				ID:        1,
				Monitored: 1,
			},
		}, nil)
		store.EXPECT().ListDownloadClients(gomock.Any()).Return([]*model.DownloadClient{
			{ID: 1, Type: "torrent", Implementation: "transmission"},
		}, nil)
		store.EXPECT().ListIndexers(gomock.Any()).Return(nil, nil)

		m := New(nil, indexerFactory, nil, store, downloadFactory, config.Manager{}, config.Config{})
		err := m.SearchForMovie(context.Background(), 1)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no indexers available")
	})
}

func TestMediaManager_SearchForSeries(t *testing.T) {
	t.Run("series not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storageMocks.NewMockStorage(ctrl)
		store.EXPECT().GetSeries(gomock.Any(), gomock.Any()).Return(nil, storage.ErrNotFound)

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		err := m.SearchForSeries(context.Background(), 1)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "series not found")
	})

	t.Run("series not monitored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storageMocks.NewMockStorage(ctrl)
		store.EXPECT().GetSeries(gomock.Any(), gomock.Any()).Return(&storage.Series{
			Series: model.Series{
				ID:        1,
				Monitored: 0,
			},
		}, nil)

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		err := m.SearchForSeries(context.Background(), 1)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not monitored")
	})

	t.Run("no seasons to search", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storageMocks.NewMockStorage(ctrl)
		store.EXPECT().GetSeries(gomock.Any(), gomock.Any()).Return(&storage.Series{
			Series: model.Series{
				ID:        1,
				Monitored: 1,
			},
		}, nil)
		store.EXPECT().ListSeasons(gomock.Any(), gomock.Any()).Return(nil, nil)
		store.EXPECT().UpdateSeries(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		err := m.SearchForSeries(context.Background(), 1)

		assert.NoError(t, err)
	})

	t.Run("skips unmonitored seasons", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storageMocks.NewMockStorage(ctrl)
		store.EXPECT().GetSeries(gomock.Any(), gomock.Any()).Return(&storage.Series{
			Series: model.Series{
				ID:        1,
				Monitored: 1,
			},
		}, nil)
		store.EXPECT().ListSeasons(gomock.Any(), gomock.Any()).Return([]*storage.Season{
			{Season: model.Season{ID: 1, SeriesID: 1, SeasonNumber: 1, Monitored: 0}},
			{Season: model.Season{ID: 2, SeriesID: 1, SeasonNumber: 2, Monitored: 0}},
		}, nil)
		store.EXPECT().UpdateSeries(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		err := m.SearchForSeries(context.Background(), 1)

		assert.NoError(t, err)
	})
}

func TestMediaManager_SearchForSeason(t *testing.T) {
	t.Run("season not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storageMocks.NewMockStorage(ctrl)
		store.EXPECT().GetSeason(gomock.Any(), gomock.Any()).Return(nil, storage.ErrNotFound)

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		err := m.SearchForSeason(context.Background(), 1)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "season not found")
	})

	t.Run("season not monitored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storageMocks.NewMockStorage(ctrl)
		store.EXPECT().GetSeason(gomock.Any(), gomock.Any()).Return(&storage.Season{
			Season: model.Season{
				ID:        1,
				Monitored: 0,
			},
		}, nil)

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		err := m.SearchForSeason(context.Background(), 1)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not monitored")
	})

	t.Run("no download clients available", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storageMocks.NewMockStorage(ctrl)
		store.EXPECT().GetSeason(gomock.Any(), gomock.Any()).Return(&storage.Season{
			Season: model.Season{
				ID:        1,
				SeriesID:  1,
				Monitored: 1,
			},
		}, nil)
		store.EXPECT().ListDownloadClients(gomock.Any()).Return(nil, errors.New("no clients"))

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		err := m.SearchForSeason(context.Background(), 1)

		assert.Error(t, err)
	})

	t.Run("series has no quality profile", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storageMocks.NewMockStorage(ctrl)
		indexerFactory := indexerMock.NewMockFactory(ctrl)
		downloadFactory := downloadMock.NewMockFactory(ctrl)

		store.EXPECT().GetSeason(gomock.Any(), gomock.Any()).Return(&storage.Season{
			Season: model.Season{
				ID:           1,
				SeriesID:     1,
				SeasonNumber: 1,
				Monitored:    1,
			},
		}, nil)
		store.EXPECT().ListDownloadClients(gomock.Any()).Return([]*model.DownloadClient{
			{ID: 1, Type: "torrent", Implementation: "transmission"},
		}, nil)
		store.EXPECT().ListIndexers(gomock.Any()).Return(nil, nil)
		store.EXPECT().GetSeries(gomock.Any(), table.Series.ID.EQ(sqlite.Int32(1))).Return(&storage.Series{
			Series: model.Series{
				ID:               1,
				QualityProfileID: 0,
			},
		}, nil)

		m := New(nil, indexerFactory, nil, store, downloadFactory, config.Manager{}, config.Config{})

		// Set up the indexer cache with test indexers
		sourceID := int64(1)
		sourceIndexers := []indexer.SourceIndexer{
			{ID: 1, Name: "test-indexer", Priority: 1},
		}
		m.indexerCache.Set(sourceID, indexerCacheEntry{
			Indexers:   sourceIndexers,
			SourceName: "test-source",
			SourceURI:  "http://test",
		})

		err := m.SearchForSeason(context.Background(), 1)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no quality profile")
	})
}

func TestMediaManager_SearchForEpisode(t *testing.T) {
	t.Run("episode not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storageMocks.NewMockStorage(ctrl)
		store.EXPECT().GetEpisode(gomock.Any(), gomock.Any()).Return(nil, storage.ErrNotFound)

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		err := m.SearchForEpisode(context.Background(), 1)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "episode not found")
	})

	t.Run("episode not monitored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storageMocks.NewMockStorage(ctrl)
		store.EXPECT().GetEpisode(gomock.Any(), gomock.Any()).Return(&storage.Episode{
			Episode: model.Episode{
				ID:        1,
				Monitored: 0,
			},
		}, nil)

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		err := m.SearchForEpisode(context.Background(), 1)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not monitored")
	})

	t.Run("season not found for episode", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storageMocks.NewMockStorage(ctrl)
		store.EXPECT().GetEpisode(gomock.Any(), gomock.Any()).Return(&storage.Episode{
			Episode: model.Episode{
				ID:        1,
				SeasonID:  1,
				Monitored: 1,
			},
		}, nil)
		store.EXPECT().GetSeason(gomock.Any(), gomock.Any()).Return(nil, storage.ErrNotFound)

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		err := m.SearchForEpisode(context.Background(), 1)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "season not found")
	})

	t.Run("no download clients available", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storageMocks.NewMockStorage(ctrl)
		store.EXPECT().GetEpisode(gomock.Any(), gomock.Any()).Return(&storage.Episode{
			Episode: model.Episode{
				ID:        1,
				SeasonID:  1,
				Monitored: 1,
			},
		}, nil)
		store.EXPECT().GetSeason(gomock.Any(), gomock.Any()).Return(&storage.Season{
			Season: model.Season{
				ID:       1,
				SeriesID: 1,
			},
		}, nil)
		store.EXPECT().ListDownloadClients(gomock.Any()).Return(nil, errors.New("no clients"))

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		err := m.SearchForEpisode(context.Background(), 1)

		assert.Error(t, err)
	})
}

func TestMediaManager_prepareSearchSnapshot(t *testing.T) {
	t.Run("no download clients", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storageMocks.NewMockStorage(ctrl)
		store.EXPECT().ListDownloadClients(gomock.Any()).Return(nil, errors.New("no clients"))

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		snapshot, err := m.prepareSearchSnapshot(context.Background())

		assert.Error(t, err)
		assert.Nil(t, snapshot)
	})

	t.Run("no indexers from cache or db", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storageMocks.NewMockStorage(ctrl)
		store.EXPECT().ListDownloadClients(gomock.Any()).Return([]*model.DownloadClient{
			{ID: 1, Type: "torrent", Implementation: "transmission"},
		}, nil)
		store.EXPECT().ListIndexers(gomock.Any()).Return(nil, nil)

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		snapshot, err := m.prepareSearchSnapshot(context.Background())

		assert.Error(t, err)
		assert.Nil(t, snapshot)
		assert.Contains(t, err.Error(), "no indexers available")
	})

	t.Run("success with cached indexers", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storageMocks.NewMockStorage(ctrl)
		store.EXPECT().ListDownloadClients(gomock.Any()).Return([]*model.DownloadClient{
			{ID: 1, Type: "torrent", Implementation: "transmission"},
		}, nil)
		store.EXPECT().ListIndexers(gomock.Any()).Return(nil, nil)

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})

		// Set up the indexer cache with test indexers
		sourceID := int64(1)
		sourceIndexers := []indexer.SourceIndexer{
			{ID: 1, Name: "test-indexer", Priority: 1},
		}
		m.indexerCache.Set(sourceID, indexerCacheEntry{
			Indexers:   sourceIndexers,
			SourceName: "test-source",
			SourceURI:  "http://test",
		})

		snapshot, err := m.prepareSearchSnapshot(context.Background())

		require.NoError(t, err)
		assert.NotNil(t, snapshot)
		assert.Equal(t, []int32{1}, snapshot.GetIndexerIDs())
	})
}
