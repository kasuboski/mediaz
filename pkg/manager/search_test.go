package manager

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/kasuboski/mediaz/config"
	downloadMock "github.com/kasuboski/mediaz/pkg/download/mocks"
	"github.com/kasuboski/mediaz/pkg/indexer"
	indexerMock "github.com/kasuboski/mediaz/pkg/indexer/mocks"
	"github.com/kasuboski/mediaz/pkg/storage"
	storageMocks "github.com/kasuboski/mediaz/pkg/storage/mocks"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestMediaManager_SearchForMovie(t *testing.T) {
	t.Run("movie not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storageMocks.NewMockStorage(ctrl)
		store.EXPECT().GetMovie(gomock.Any(), int64(1)).Return(sqlcdb.GetMovieByIDRow{}, storage.ErrNotFound)

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		err := m.SearchForMovie(context.Background(), 1)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "movie not found")
	})

	t.Run("movie not monitored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storageMocks.NewMockStorage(ctrl)
		store.EXPECT().GetMovie(gomock.Any(), int64(1)).Return(sqlcdb.GetMovieByIDRow{
			Movie: sqlcdb.Movie{
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
		store.EXPECT().GetMovie(gomock.Any(), int64(1)).Return(sqlcdb.GetMovieByIDRow{
			Movie: sqlcdb.Movie{
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

		store.EXPECT().GetMovie(gomock.Any(), int64(1)).Return(sqlcdb.GetMovieByIDRow{
			Movie: sqlcdb.Movie{
				ID:        1,
				Monitored: 1,
			},
		}, nil)
		store.EXPECT().ListDownloadClients(gomock.Any()).Return([]sqlcdb.DownloadClient{
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
		store.EXPECT().GetSeriesByID(gomock.Any(), gomock.Any()).Return(sqlcdb.GetSeriesRow{}, storage.ErrNotFound)

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		err := m.SearchForSeries(context.Background(), 1)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "series not found")
	})

	t.Run("series not monitored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storageMocks.NewMockStorage(ctrl)
		store.EXPECT().GetSeriesByID(gomock.Any(), gomock.Any()).Return(sqlcdb.GetSeriesRow{
			Series: sqlcdb.Series{
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
		store.EXPECT().GetSeriesByID(gomock.Any(), gomock.Any()).Return(sqlcdb.GetSeriesRow{
			Series: sqlcdb.Series{
				ID:        1,
				Monitored: 1,
			},
		}, nil)
		store.EXPECT().ListSeasons(gomock.Any(), gomock.Any()).Return(nil, nil)
		store.EXPECT().UpdateSeriesByID(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		err := m.SearchForSeries(context.Background(), 1)

		assert.NoError(t, err)
	})

	t.Run("skips unmonitored seasons", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storageMocks.NewMockStorage(ctrl)
		store.EXPECT().GetSeriesByID(gomock.Any(), gomock.Any()).Return(sqlcdb.GetSeriesRow{
			Series: sqlcdb.Series{
				ID:        1,
				Monitored: 1,
			},
		}, nil)
		store.EXPECT().ListSeasons(gomock.Any(), gomock.Any()).Return([]sqlcdb.ListSeasonsRow{
			{Season: sqlcdb.Season{ID: 1, SeriesID: 1, SeasonNumber: 1, Monitored: 0}},
			{Season: sqlcdb.Season{ID: 2, SeriesID: 1, SeasonNumber: 2, Monitored: 0}},
		}, nil)
		store.EXPECT().UpdateSeriesByID(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

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
		store.EXPECT().GetSeasonByID(gomock.Any(), gomock.Any()).Return(sqlcdb.GetSeasonByIDRow{}, storage.ErrNotFound)

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		err := m.SearchForSeason(context.Background(), 1)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "season not found")
	})

	t.Run("season not monitored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storageMocks.NewMockStorage(ctrl)
		store.EXPECT().GetSeasonByID(gomock.Any(), gomock.Any()).Return(sqlcdb.GetSeasonByIDRow{
			Season: sqlcdb.Season{
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
		store.EXPECT().GetSeasonByID(gomock.Any(), gomock.Any()).Return(sqlcdb.GetSeasonByIDRow{
			Season: sqlcdb.Season{
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

		store.EXPECT().GetSeasonByID(gomock.Any(), gomock.Any()).Return(sqlcdb.GetSeasonByIDRow{
			Season: sqlcdb.Season{
				ID:           1,
				SeriesID:     1,
				SeasonNumber: 1,
				Monitored:    1,
			},
		}, nil)
		store.EXPECT().ListDownloadClients(gomock.Any()).Return([]sqlcdb.DownloadClient{
			{ID: 1, Type: "torrent", Implementation: "transmission"},
		}, nil)
		store.EXPECT().ListIndexers(gomock.Any()).Return(nil, nil)
		store.EXPECT().GetSeriesByID(gomock.Any(), int64(1)).Return(sqlcdb.GetSeriesRow{
			Series: sqlcdb.Series{
				ID:               1,
				QualityProfileID: 0,
			},
		}, nil)

		mockIndexerSource := indexerMock.NewMockIndexerSource(ctrl)
		store.EXPECT().GetIndexerSource(gomock.Any(), int64(1)).Return(sqlcdb.IndexerSource{
			ID: 1, Name: "test-source", Scheme: "http", Host: "test", Enabled: true,
		}, nil)
		indexerFactory.EXPECT().NewIndexerSource(gomock.Any()).Return(mockIndexerSource, nil)
		mockIndexerSource.EXPECT().ListIndexers(gomock.Any()).Return([]indexer.SourceIndexer{
			{ID: 1, Name: "test-indexer", Priority: 1},
		}, nil)
		store.EXPECT().CreateIndexer(gomock.Any(), sqlcdb.Indexer{
			Name: "test-indexer", Priority: 1, IndexerSourceID: sql.NullInt64{Int64: 1, Valid: true}, Categories: `{"movie":[],"tv":[]}`,
		}).Return(int64(1), nil)

		m := New(nil, indexerFactory, nil, store, downloadFactory, config.Manager{}, config.Config{})
		require.NoError(t, m.RefreshIndexerSource(context.Background(), 1))

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
		store.EXPECT().GetEpisodeByID(gomock.Any(), gomock.Any()).Return(sqlcdb.GetEpisodeByIDRow{}, storage.ErrNotFound)

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		err := m.SearchForEpisode(context.Background(), 1)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "episode not found")
	})

	t.Run("episode not monitored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storageMocks.NewMockStorage(ctrl)
		store.EXPECT().GetEpisodeByID(gomock.Any(), gomock.Any()).Return(sqlcdb.GetEpisodeByIDRow{
			Episode: sqlcdb.Episode{
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
		store.EXPECT().GetEpisodeByID(gomock.Any(), gomock.Any()).Return(sqlcdb.GetEpisodeByIDRow{
			Episode: sqlcdb.Episode{
				ID:        1,
				SeasonID:  1,
				Monitored: 1,
			},
		}, nil)
		store.EXPECT().GetSeasonByID(gomock.Any(), gomock.Any()).Return(sqlcdb.GetSeasonByIDRow{}, storage.ErrNotFound)

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		err := m.SearchForEpisode(context.Background(), 1)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "season not found")
	})

	t.Run("no download clients available", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storageMocks.NewMockStorage(ctrl)
		store.EXPECT().GetEpisodeByID(gomock.Any(), gomock.Any()).Return(sqlcdb.GetEpisodeByIDRow{
			Episode: sqlcdb.Episode{
				ID:        1,
				SeasonID:  1,
				Monitored: 1,
			},
		}, nil)
		store.EXPECT().GetSeasonByID(gomock.Any(), gomock.Any()).Return(sqlcdb.GetSeasonByIDRow{
			Season: sqlcdb.Season{
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
		store.EXPECT().ListDownloadClients(gomock.Any()).Return([]sqlcdb.DownloadClient{
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
		store.EXPECT().ListDownloadClients(gomock.Any()).Return([]sqlcdb.DownloadClient{
			{ID: 1, Type: "torrent", Implementation: "transmission"},
		}, nil)
		store.EXPECT().ListIndexers(gomock.Any()).Return(nil, nil)

		indexerFactory := indexerMock.NewMockFactory(ctrl)
		mockIndexerSource := indexerMock.NewMockIndexerSource(ctrl)
		store.EXPECT().GetIndexerSource(gomock.Any(), int64(1)).Return(sqlcdb.IndexerSource{
			ID: 1, Name: "test-source", Scheme: "http", Host: "test", Enabled: true,
		}, nil)
		indexerFactory.EXPECT().NewIndexerSource(gomock.Any()).Return(mockIndexerSource, nil)
		mockIndexerSource.EXPECT().ListIndexers(gomock.Any()).Return([]indexer.SourceIndexer{
			{ID: 1, Name: "test-indexer", Priority: 1},
		}, nil)
		store.EXPECT().CreateIndexer(gomock.Any(), sqlcdb.Indexer{
			Name: "test-indexer", Priority: 1, IndexerSourceID: sql.NullInt64{Int64: 1, Valid: true}, Categories: `{"movie":[],"tv":[]}`,
		}).Return(int64(1), nil)

		m := New(nil, indexerFactory, nil, store, nil, config.Manager{}, config.Config{})
		require.NoError(t, m.RefreshIndexerSource(context.Background(), 1))

		snapshot, err := m.prepareSearchSnapshot(context.Background())

		require.NoError(t, err)
		assert.NotNil(t, snapshot)
		assert.Equal(t, []int64{1}, snapshot.GetIndexerIDs())
	})
}
