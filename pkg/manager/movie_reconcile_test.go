package manager

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/download"
	downloadMock "github.com/kasuboski/mediaz/pkg/download/mocks"
	mhttpMock "github.com/kasuboski/mediaz/pkg/http/mocks"
	"github.com/kasuboski/mediaz/pkg/indexer"
	indexerMock "github.com/kasuboski/mediaz/pkg/indexer/mocks"
	mio "github.com/kasuboski/mediaz/pkg/io"
	"github.com/kasuboski/mediaz/pkg/library"
	mockLibrary "github.com/kasuboski/mediaz/pkg/library/mocks"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/storage"
	storageMocks "github.com/kasuboski/mediaz/pkg/storage/mocks"
	mediaSqlite "github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
	"github.com/kasuboski/mediaz/pkg/tmdb"
	"github.com/oapi-codegen/nullable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func Test_Manager_reconcileMissingMovie(t *testing.T) {
	ctrl := gomock.NewController(t)

	store, err := mediaSqlite.New(context.Background(), ":memory:")
	require.NoError(t, err)

	schemas, err := storage.ReadSchemaFiles("../storage/sqlite/schema/schema.sql", "../storage/sqlite/schema/defaults.sql")
	require.NoError(t, err)

	ctx := context.Background()
	err = store.Init(ctx, schemas...)
	require.NoError(t, err)

	indexers := []model.Indexer{{ID: 1, Name: "test", Priority: 1}, {ID: 3, Name: "test2", Priority: 10}}

	bigSeeders := nullable.NewNullNullable[int32]()
	bigSeeders.Set(23)

	smallerSeeders := nullable.NewNullNullable[int32]()
	smallerSeeders.Set(15)

	smallestSeeders := nullable.NewNullNullable[int32]()
	smallestSeeders.Set(10)

	torrentProto := ptr(prowlarr.DownloadProtocolTorrent)
	usenetProto := ptr(prowlarr.DownloadProtocolUsenet)

	wantRelease := &prowlarr.ReleaseResource{ID: ptr(int32(123)), Title: nullable.NewNullableWithValue("test movie"), Size: sizeGBToBytes(23), Seeders: bigSeeders, Protocol: torrentProto}
	doNotWantRelease := &prowlarr.ReleaseResource{ID: ptr(int32(124)), Title: nullable.NewNullableWithValue("test movie"), Size: sizeGBToBytes(23), Seeders: smallerSeeders, Protocol: torrentProto}
	smallMovie := &prowlarr.ReleaseResource{ID: ptr(int32(125)), Title: nullable.NewNullableWithValue("test movie - very small"), Size: sizeGBToBytes(1), Seeders: smallestSeeders, Protocol: torrentProto}
	nzbMovie := &prowlarr.ReleaseResource{ID: ptr(int32(1225)), Title: nullable.NewNullableWithValue("test movie - nzb"), Size: sizeGBToBytes(23), Seeders: smallestSeeders, Protocol: usenetProto}

	releases := []*prowlarr.ReleaseResource{doNotWantRelease, wantRelease, smallMovie, nzbMovie}

	mockIndexerSource := indexerMock.NewMockIndexerSource(ctrl)
	mockIndexerSource.EXPECT().Search(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(releases, nil).AnyTimes()

	indexerFactory := indexerMock.NewMockFactory(ctrl)
	indexerFactory.EXPECT().NewIndexerSource(gomock.Any()).Return(mockIndexerSource, nil).AnyTimes()

	releaseDate := time.Now().AddDate(0, 0, -1).Format(tmdb.ReleaseDateFormat)

	tmdbHttpMock := mhttpMock.NewMockHTTPClient(ctrl)
	tmdbHttpMock.EXPECT().Do(gomock.Any()).Return(mediaDetailsResponse("test movie", 120, releaseDate), nil).Times(1)

	tClient, err := tmdb.New("https://api.themoviedb.org", "1234", tmdb.WithHTTPClient(tmdbHttpMock))
	require.NoError(t, err)

	downloadClient := model.DownloadClient{
		Implementation: "transmission",
		Type:           "torrent",
		Port:           8080,
		Host:           "transmission",
		Scheme:         "http",
	}

	downloadClientID, err := store.CreateDownloadClient(ctx, downloadClient)
	require.NoError(t, err)

	downloadClient.ID = int32(downloadClientID)

	mockFactory := downloadMock.NewMockFactory(ctrl)
	mockDownloadClient := downloadMock.NewMockDownloadClient(ctrl)
	downloadStatus := download.Status{
		ID:   "123",
		Name: "test download",
	}
	mockDownloadClient.EXPECT().Add(ctx, download.AddRequest{Release: wantRelease}).Times(1).Return(downloadStatus, nil)

	mockFactory.EXPECT().NewDownloadClient(downloadClient).Times(1).Return(mockDownloadClient, nil)

	movieFS := fstest.MapFS{}
	tvFS := fstest.MapFS{}
	lib := library.New(
		library.FileSystem{
			FS: movieFS,
		},
		library.FileSystem{
			FS: tvFS,
		},
		&mio.MediaFileSystem{},
	)

	m := New(tClient, indexerFactory, lib, store, mockFactory, config.Manager{}, config.Config{})
	require.NotNil(t, m)

	sourceID, err := store.CreateIndexerSource(ctx, model.IndexerSource{
		Name:           "test-source",
		Implementation: "prowlarr",
		Scheme:         "http",
		Host:           "test",
		Enabled:        true,
	})
	require.NoError(t, err)

	sourceIndexers := []indexer.SourceIndexer{
		{ID: 1, Name: "test", Priority: 1},
		{ID: 3, Name: "test2", Priority: 10},
	}
	m.indexerCache.Set(sourceID, indexerCacheEntry{
		Indexers:   sourceIndexers,
		SourceName: "test-source",
		SourceURI:  "http://test",
	})

	req := AddMovieRequest{
		TMDBID:           1234,
		QualityProfileID: 1,
	}

	mov, err := m.AddMovieToLibrary(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, mov)

	snapshot := newReconcileSnapshot(indexers, []*model.DownloadClient{
		&downloadClient,
	})

	err = m.reconcileMissingMovie(ctx, mov, snapshot)
	require.NoError(t, err)

	mov, err = m.storage.GetMovie(ctx, int64(mov.ID))
	require.NoError(t, err)

	assert.Equal(t, mov.State, storage.MovieStateDownloading)
}

func Test_Manager_reconcileDiscoveredMovie(t *testing.T) {
	t.Run("single result", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		store := storageMocks.NewMockStorage(ctrl)
		tmdbHttpMock := mhttpMock.NewMockHTTPClient(ctrl)

		// Mock search response
		searchResp := &http.Response{
			Body: io.NopCloser(strings.NewReader(`{
				"results": [{
					"id": 1234,
					"title": "test movie",
					"overview": "test overview",
					"poster_path": "/test.jpg"
				}]
			}`)),
			StatusCode: http.StatusOK,
		}
		tmdbHttpMock.EXPECT().Do(gomock.Any()).Return(searchResp, nil).Times(1)

		tClient, err := tmdb.New("https://api.themoviedb.org", "1234", tmdb.WithHTTPClient(tmdbHttpMock))
		require.NoError(t, err)

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		m.tmdb = tClient

		movie := storage.Movie{
			Movie: model.Movie{
				ID:   1,
				Path: func() *string { s := "test movie"; return &s }(),
			},
		}

		ctx := context.Background()

		store.EXPECT().GetMovieMetadata(ctx, table.MovieMetadata.TmdbID.EQ(sqlite.Int(1234))).Return(&model.MovieMetadata{ID: 120}, nil)
		store.EXPECT().GetMovieByMetadataID(ctx, 120).Return(nil, storage.ErrNotFound)
		store.EXPECT().LinkMovieMetadata(ctx, int64(1), int32(120)).Return(nil)

		// Execute reconciliation
		err = m.reconcileDiscoveredMovie(ctx, &movie)
		require.NoError(t, err)

		// Verify movie properties
		require.Equal(t, int32(1), movie.ID, "ID should remain unchanged")
		require.Equal(t, "test movie", *movie.Path, "Path should remain unchanged")

		// Verify that metadata was linked (ID 120 from the mock response)
		require.NotNil(t, movie.MovieMetadataID, "MovieMetadataID should be set")
		require.Equal(t, int32(120), *movie.MovieMetadataID, "MovieMetadataID should be set to the ID from GetMovieMetadata")
		// We don't verify MovieMetadataID here since it's managed by the mock expectations
	})

	t.Run("no results", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		store := storageMocks.NewMockStorage(ctrl)
		tmdbHttpMock := mhttpMock.NewMockHTTPClient(ctrl)

		// Mock search response with no results
		searchResp := &http.Response{
			Body: io.NopCloser(strings.NewReader(`{
				"results": []
			}`)),
			StatusCode: http.StatusOK,
		}
		tmdbHttpMock.EXPECT().Do(gomock.Any()).Return(searchResp, nil).Times(1)

		tClient, err := tmdb.New("https://api.themoviedb.org", "1234", tmdb.WithHTTPClient(tmdbHttpMock))
		require.NoError(t, err)

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		m.tmdb = tClient

		movie := storage.Movie{
			Movie: model.Movie{
				ID:   1,
				Path: func() *string { s := "test movie"; return &s }(),
			},
		}

		ctx := context.Background()

		// Execute reconciliation
		err = m.reconcileDiscoveredMovie(ctx, &movie)
		require.NoError(t, err)

		// Verify movie properties
		require.Equal(t, int32(1), movie.ID, "ID should remain unchanged")
		require.Equal(t, "test movie", *movie.Path, "Path should remain unchanged")
	})

	t.Run("multiple results", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		store := storageMocks.NewMockStorage(ctrl)
		tmdbHttpMock := mhttpMock.NewMockHTTPClient(ctrl)

		// Mock search response with multiple results
		searchResp := &http.Response{
			Body: io.NopCloser(strings.NewReader(`{
				"results": [{
					"id": 1234,
					"title": "test movie 1",
					"overview": "test overview",
					"poster_path": "/test.jpg"
				}, {
					"id": 5678,
					"title": "test movie 2",
					"overview": "test overview 2",
					"poster_path": "/test2.jpg"
				}]
			}`)),
			StatusCode: http.StatusOK,
		}
		tmdbHttpMock.EXPECT().Do(gomock.Any()).Return(searchResp, nil).Times(1)

		tClient, err := tmdb.New("https://api.themoviedb.org", "1234", tmdb.WithHTTPClient(tmdbHttpMock))
		require.NoError(t, err)

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		m.tmdb = tClient

		movie := storage.Movie{
			Movie: model.Movie{
				ID:   1,
				Path: func() *string { s := "test movie"; return &s }(),
			},
		}

		ctx := context.Background()

		store.EXPECT().GetMovieMetadata(ctx, table.MovieMetadata.TmdbID.EQ(sqlite.Int(1234))).Return(&model.MovieMetadata{ID: 120}, nil)
		store.EXPECT().GetMovieByMetadataID(ctx, 120).Return(nil, storage.ErrNotFound)
		store.EXPECT().LinkMovieMetadata(ctx, int64(1), int32(120)).Return(nil)

		// Execute reconciliation
		err = m.reconcileDiscoveredMovie(ctx, &movie)
		require.NoError(t, err)

		// Verify movie properties
		require.Equal(t, int32(1), movie.ID, "ID should remain unchanged")
		require.Equal(t, "test movie", *movie.Path, "Path should remain unchanged")

		// Verify that metadata was linked (ID 120 from the mock response)
		require.NotNil(t, movie.MovieMetadataID, "MovieMetadataID should be set")
		require.Equal(t, int32(120), *movie.MovieMetadataID, "MovieMetadataID should be set to the ID from GetMovieMetadata")
	})

	t.Run("already linked", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		store := storageMocks.NewMockStorage(ctrl)
		tmdbHttpMock := mhttpMock.NewMockHTTPClient(ctrl)

		// Mock search response
		searchResp := &http.Response{
			Body: io.NopCloser(strings.NewReader(`{
				"results": [{
					"id": 1234,
					"title": "test movie",
					"overview": "test overview",
					"poster_path": "/test.jpg"
				}]
			}`)),
			StatusCode: http.StatusOK,
		}
		tmdbHttpMock.EXPECT().Do(gomock.Any()).Return(searchResp, nil).Times(1)

		tClient, err := tmdb.New("https://api.themoviedb.org", "1234", tmdb.WithHTTPClient(tmdbHttpMock))
		require.NoError(t, err)

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		m.tmdb = tClient

		movie := storage.Movie{
			Movie: model.Movie{
				ID:   2,
				Path: func() *string { s := "test movie duplicate"; return &s }(),
			},
		}

		ctx := context.Background()

		store.EXPECT().GetMovieMetadata(ctx, table.MovieMetadata.TmdbID.EQ(sqlite.Int(1234))).Return(&model.MovieMetadata{ID: 120}, nil)
		// Metadata is already linked to movie ID 1
		existingMovie := &storage.Movie{Movie: model.Movie{ID: 1}}
		store.EXPECT().GetMovieByMetadataID(ctx, 120).Return(existingMovie, nil)
		// LinkMovieMetadata should NOT be called

		// Execute reconciliation
		err = m.reconcileDiscoveredMovie(ctx, &movie)
		require.NoError(t, err)

		// Verify movie properties - metadata should NOT be linked
		require.Equal(t, int32(2), movie.ID, "ID should remain unchanged")
		require.Equal(t, "test movie duplicate", *movie.Path, "Path should remain unchanged")
		require.Nil(t, movie.MovieMetadataID, "MovieMetadataID should NOT be set because metadata is already linked to another movie")
	})

	t.Run("has metadata", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		store := storageMocks.NewMockStorage(ctrl)
		tmdbHttpMock := mhttpMock.NewMockHTTPClient(ctrl)

		tClient, err := tmdb.New("https://api.themoviedb.org", "1234", tmdb.WithHTTPClient(tmdbHttpMock))
		require.NoError(t, err)

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		m.tmdb = tClient

		metadataID := int32(120)
		movie := storage.Movie{
			Movie: model.Movie{
				ID:              1,
				Path:            func() *string { s := "test movie"; return &s }(),
				MovieMetadataID: &metadataID,
			},
		}

		ctx := context.Background()

		// Execute reconciliation
		err = m.reconcileDiscoveredMovie(ctx, &movie)
		require.NoError(t, err)

		// Verify movie properties
		require.Equal(t, int32(1), movie.ID, "ID should remain unchanged")
		require.Equal(t, "test movie", *movie.Path, "Path should remain unchanged")

		// Verify that metadata was linked (ID 120 from the mock response)
		require.NotNil(t, movie.MovieMetadataID, "MovieMetadataID should be set")
		require.Equal(t, int32(120), *movie.MovieMetadataID, "MovieMetadataID should be set to the ID from GetMovieMetadata")
	})
}

func Test_Manager_reconcileUnreleasedMovie(t *testing.T) {
	ctrl := gomock.NewController(t)

	store, err := mediaSqlite.New(context.Background(), ":memory:")
	require.NoError(t, err)

	schemas, err := storage.ReadSchemaFiles("../storage/sqlite/schema/schema.sql", "../storage/sqlite/schema/defaults.sql")
	require.NoError(t, err)

	ctx := context.Background()
	err = store.Init(ctx, schemas...)
	require.NoError(t, err)

	indexers := []model.Indexer{{ID: 1, Name: "test", Priority: 1}, {ID: 3, Name: "test2", Priority: 10}}

	indexerFactory := indexerMock.NewMockFactory(ctrl)

	releaseDate := time.Now().AddDate(0, 0, +5).Format(tmdb.ReleaseDateFormat)

	tmdbHttpMock := mhttpMock.NewMockHTTPClient(ctrl)
	tmdbHttpMock.EXPECT().Do(gomock.Any()).Return(mediaDetailsResponse("test movie", 120, releaseDate), nil).Times(1)
	tClient, err := tmdb.New("https://api.themoviedb.org", "1234", tmdb.WithHTTPClient(tmdbHttpMock))
	require.NoError(t, err)

	downloadClient := model.DownloadClient{
		Implementation: "transmission",
		Type:           "torrent",
		Port:           8080,
		Host:           "transmission",
		Scheme:         "http",
	}

	downloadClientID, err := store.CreateDownloadClient(ctx, downloadClient)
	require.NoError(t, err)

	downloadClient.ID = int32(downloadClientID)

	mockFactory := downloadMock.NewMockFactory(ctrl)

	movieFS := fstest.MapFS{}
	tvFS := fstest.MapFS{}
	lib := library.New(
		library.FileSystem{
			FS: movieFS,
		},
		library.FileSystem{
			FS: tvFS,
		},
		&mio.MediaFileSystem{},
	)

	m := New(tClient, indexerFactory, lib, store, mockFactory, config.Manager{}, config.Config{})
	require.NotNil(t, m)

	req := AddMovieRequest{
		TMDBID:           1234,
		QualityProfileID: 1,
	}

	mov, err := m.AddMovieToLibrary(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, mov)

	snapshot := newReconcileSnapshot(indexers, []*model.DownloadClient{
		&downloadClient,
	})

	err = m.reconcileUnreleasedMovie(ctx, mov, snapshot)
	require.NoError(t, err)

	mov, err = m.storage.GetMovie(ctx, int64(mov.ID))
	require.NoError(t, err)

	assert.Equal(t, mov.State, storage.MovieStateUnreleased)
}

func Test_Manager_reconcileDownloadingMovie(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()

	t.Run("snapshot is nil", func(t *testing.T) {
		m := New(nil, nil, nil, nil, nil, config.Manager{}, config.Config{})
		require.NotNil(t, m)

		movie := &storage.Movie{Movie: model.Movie{ID: 1}}
		err := m.reconcileDownloadingMovie(ctx, movie, nil)
		assert.NoError(t, err)
	})

	t.Run("movie is not monitored", func(t *testing.T) {
		m := New(nil, nil, nil, nil, nil, config.Manager{}, config.Config{})
		require.NotNil(t, m)

		movie := &storage.Movie{Movie: model.Movie{ID: 1, Monitored: 0}}
		snapshot := newReconcileSnapshot(nil, nil)
		err := m.reconcileDownloadingMovie(ctx, movie, snapshot)
		assert.NoError(t, err)
	})

	t.Run("movie file is already tracked", func(t *testing.T) {
		store := newStore(t, ctx)
		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		require.NotNil(t, m)

		movie := storage.Movie{Movie: model.Movie{ID: 1, Monitored: 1, Path: ptr("Movie 1")}}
		_, err := store.CreateMovie(ctx, movie, storage.MovieStateMissing)
		require.NoError(t, err)

		err = m.updateMovieState(ctx, &movie, storage.MovieStateDownloading, nil)
		require.NoError(t, err)

		_, err = store.CreateMovieFile(ctx, model.MovieFile{RelativePath: ptr("Movie 1/movie1.mp4")})
		require.NoError(t, err)

		snapshot := newReconcileSnapshot(nil, nil)
		err = m.reconcileDownloadingMovie(ctx, &movie, snapshot)
		assert.NoError(t, err)

		mov, err := store.GetMovie(ctx, 1)
		require.NoError(t, err)
		assert.Equal(t, storage.MovieStateDownloaded, mov.State)
	})

	t.Run("download client not found in snapshot", func(t *testing.T) {
		store := newStore(t, ctx)
		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		require.NotNil(t, m)

		movie := storage.Movie{Movie: model.Movie{ID: 1, Monitored: 1}}

		_, err := store.CreateMovie(ctx, movie, storage.MovieStateMissing)
		require.NoError(t, err)

		err = m.updateMovieState(ctx, &movie, storage.MovieStateDownloading, nil)
		require.NoError(t, err)

		snapshot := newReconcileSnapshot(nil, nil)
		err = m.reconcileDownloadingMovie(ctx, &movie, snapshot)
		assert.NoError(t, err)
	})

	t.Run("failed to create download client from model", func(t *testing.T) {
		store := newStore(t, ctx)

		downloadClientModel := model.DownloadClient{
			Implementation: "transmission",
			Type:           "torrent",
			Port:           8080,
			Host:           "transmission",
			Scheme:         "http",
			APIKey:         nil,
		}

		downloadClientID, err := store.CreateDownloadClient(ctx, downloadClientModel)
		require.NoError(t, err)

		downloadClientModel.ID = int32(downloadClientID)

		mockFactory := downloadMock.NewMockFactory(ctrl)
		mockFactory.EXPECT().NewDownloadClient(downloadClientModel).Return(nil, errors.New("failed to create download client"))

		m := New(nil, nil, nil, store, mockFactory, config.Manager{}, config.Config{})
		require.NotNil(t, m)

		movieID, err := m.storage.CreateMovie(ctx, storage.Movie{Movie: model.Movie{ID: 1, Monitored: 1, QualityProfileID: 1, Path: ptr("Movie 1")}}, storage.MovieStateMissing)
		require.NoError(t, err)

		movie, err := m.storage.GetMovie(ctx, movieID)
		require.NoError(t, err)

		downloadID := "123"
		err = m.updateMovieState(ctx, movie, storage.MovieStateDownloading, &storage.TransitionStateMetadata{
			DownloadID:       &downloadID,
			DownloadClientID: &downloadClientModel.ID,
		})
		require.NoError(t, err)

		movie, err = m.storage.GetMovie(ctx, movieID)
		require.NoError(t, err)

		snapshot := newReconcileSnapshot(nil, []*model.DownloadClient{&downloadClientModel})

		err = m.reconcileDownloadingMovie(ctx, movie, snapshot)
		require.Error(t, err)
		assert.Equal(t, err.Error(), "failed to create download client")

		mov, err := store.GetMovie(ctx, 1)
		require.NoError(t, err)
		assert.Equal(t, storage.MovieStateDownloading, mov.State)
	})

	t.Run("failed to get download status", func(t *testing.T) {
		store := newStore(t, ctx)

		downloadClientModel := model.DownloadClient{
			Implementation: "transmission",
			Type:           "torrent",
			Port:           8080,
			Host:           "transmission",
			Scheme:         "http",
		}

		downloadClientID, err := store.CreateDownloadClient(ctx, downloadClientModel)
		require.NoError(t, err)

		downloadClientModel.ID = int32(downloadClientID)

		mockDownloadClient := downloadMock.NewMockDownloadClient(ctrl)
		mockFactory := downloadMock.NewMockFactory(ctrl)
		mockFactory.EXPECT().NewDownloadClient(downloadClientModel).Return(mockDownloadClient, nil)
		mockDownloadClient.EXPECT().Get(ctx, download.GetRequest{ID: "123"}).Return(download.Status{}, errors.New("failed to get download status"))

		m := New(nil, nil, nil, store, mockFactory, config.Manager{}, config.Config{})
		require.NotNil(t, m)

		movieID, err := m.storage.CreateMovie(ctx, storage.Movie{Movie: model.Movie{ID: 1, Monitored: 1, QualityProfileID: 1, Path: ptr("Movie 1")}}, storage.MovieStateMissing)
		require.NoError(t, err)

		movie, err := m.storage.GetMovie(ctx, movieID)
		require.NoError(t, err)

		downloadID := "123"
		err = m.updateMovieState(ctx, movie, storage.MovieStateDownloading, &storage.TransitionStateMetadata{
			DownloadID:       &downloadID,
			DownloadClientID: &downloadClientModel.ID,
		})
		require.NoError(t, err)

		movie, err = m.storage.GetMovie(ctx, movieID)
		require.NoError(t, err)

		snapshot := newReconcileSnapshot(nil, []*model.DownloadClient{&downloadClientModel})

		err = m.reconcileDownloadingMovie(ctx, movie, snapshot)
		assert.Error(t, err)
		assert.Equal(t, err.Error(), "failed to get download status")

		mov, err := store.GetMovie(ctx, 1)
		require.NoError(t, err)
		assert.Equal(t, storage.MovieStateDownloading, mov.State)
	})

	t.Run("download not finished", func(t *testing.T) {
		store := newStore(t, ctx)
		mockDownloadClient := downloadMock.NewMockDownloadClient(ctrl)
		downloadClientModel := model.DownloadClient{
			Implementation: "transmission",
			Type:           "torrent",
			Port:           8080,
			Host:           "transmission",
			Scheme:         "http",
		}

		downloadClientID, err := store.CreateDownloadClient(ctx, downloadClientModel)
		require.NoError(t, err)

		downloadClientModel.ID = int32(downloadClientID)

		mockFactory := downloadMock.NewMockFactory(ctrl)
		mockFactory.EXPECT().NewDownloadClient(downloadClientModel).Return(mockDownloadClient, nil)

		mockDownloadClient.EXPECT().Get(ctx, download.GetRequest{ID: "123"}).Return(download.Status{
			ID:   "123",
			Done: false,
		}, nil)

		m := New(nil, nil, nil, store, mockFactory, config.Manager{}, config.Config{})
		require.NotNil(t, m)

		movieID, err := m.storage.CreateMovie(ctx, storage.Movie{Movie: model.Movie{ID: 1, Monitored: 1, QualityProfileID: 1, Path: ptr("my-movie")}}, storage.MovieStateMissing)
		require.NoError(t, err)

		movie, err := m.storage.GetMovie(ctx, movieID)
		require.NoError(t, err)

		downloadID := "123"
		err = m.updateMovieState(ctx, movie, storage.MovieStateDownloading, &storage.TransitionStateMetadata{
			DownloadID:       &downloadID,
			DownloadClientID: &downloadClientModel.ID,
		})
		require.NoError(t, err)

		movie, err = m.storage.GetMovie(ctx, movieID)
		require.NoError(t, err)

		snapshot := newReconcileSnapshot(nil, []*model.DownloadClient{&downloadClientModel})
		err = m.reconcileDownloadingMovie(ctx, movie, snapshot)
		assert.NoError(t, err)

		mov, err := store.GetMovie(ctx, 1)
		require.NoError(t, err)
		assert.Equal(t, storage.MovieStateDownloading, mov.State)
	})

	t.Run("failed to get movie metadata", func(t *testing.T) {
		store := newStore(t, ctx)

		downloadClientModel := model.DownloadClient{
			Implementation: "transmission",
			Type:           "torrent",
			Port:           8080,
			Host:           "transmission",
			Scheme:         "http",
		}

		downloadClientID, err := store.CreateDownloadClient(ctx, downloadClientModel)
		require.NoError(t, err)

		downloadClientModel.ID = int32(downloadClientID)

		mockDownloadClient := downloadMock.NewMockDownloadClient(ctrl)
		mockFactory := downloadMock.NewMockFactory(ctrl)
		mockFactory.EXPECT().NewDownloadClient(downloadClientModel).Return(mockDownloadClient, nil)
		mockDownloadClient.EXPECT().Get(ctx, download.GetRequest{ID: "123"}).Return(download.Status{
			ID:   "123",
			Done: true,
		}, nil)

		m := New(nil, nil, nil, store, mockFactory, config.Manager{}, config.Config{})
		require.NotNil(t, m)

		movieID, err := m.storage.CreateMovie(ctx, storage.Movie{Movie: model.Movie{ID: 1, Monitored: 1, QualityProfileID: 1, MovieMetadataID: ptr(int32(1)), Path: ptr("Movie 1")}}, storage.MovieStateMissing)
		require.NoError(t, err)

		movie, err := m.storage.GetMovie(ctx, movieID)
		require.NoError(t, err)

		downloadID := "123"
		err = m.updateMovieState(ctx, movie, storage.MovieStateDownloading, &storage.TransitionStateMetadata{
			DownloadID:       &downloadID,
			DownloadClientID: &downloadClientModel.ID,
		})
		require.NoError(t, err)

		movie, err = m.storage.GetMovie(ctx, movieID)
		require.NoError(t, err)

		snapshot := newReconcileSnapshot(nil, []*model.DownloadClient{&downloadClientModel})

		err = m.reconcileDownloadingMovie(ctx, movie, snapshot)
		assert.Error(t, err)
		assert.Equal(t, "not found in storage", err.Error())

		mov, err := store.GetMovie(ctx, 1)
		assert.NoError(t, err)
		assert.Equal(t, storage.MovieStateDownloading, mov.State)
	})

	t.Run("failed to add movie file to library", func(t *testing.T) {
		store := newStore(t, ctx)
		mockLibrary := mockLibrary.NewMockLibrary(ctrl)
		mockLibrary.EXPECT().AddMovie(gomock.Any(), "my-movie", "test path").Return(library.MovieFile{}, errors.New("expected testing error"))

		downloadClientModel := model.DownloadClient{
			Implementation: "transmission",
			Type:           "torrent",
			Port:           8080,
			Host:           "transmission",
			Scheme:         "http",
		}

		downloadClientID, err := store.CreateDownloadClient(ctx, downloadClientModel)
		require.NoError(t, err)

		downloadClientModel.ID = int32(downloadClientID)

		mockDownloadClient := downloadMock.NewMockDownloadClient(ctrl)
		mockFactory := downloadMock.NewMockFactory(ctrl)
		mockFactory.EXPECT().NewDownloadClient(downloadClientModel).Return(mockDownloadClient, nil)
		mockDownloadClient.EXPECT().Get(ctx, download.GetRequest{ID: "123"}).Return(download.Status{
			ID:        "123",
			Done:      true,
			FilePaths: []string{"test path"},
		}, nil)

		m := New(nil, nil, mockLibrary, store, mockFactory, config.Manager{}, config.Config{})
		require.NotNil(t, m)

		movieID, err := m.storage.CreateMovie(ctx, storage.Movie{Movie: model.Movie{ID: 1, Monitored: 1, QualityProfileID: 1, MovieMetadataID: ptr(int32(1)), Path: ptr("Movie 1")}}, storage.MovieStateMissing)
		require.NoError(t, err)

		movie, err := m.storage.GetMovie(ctx, movieID)
		require.NoError(t, err)

		downloadID := "123"
		err = m.updateMovieState(ctx, movie, storage.MovieStateDownloading, &storage.TransitionStateMetadata{
			DownloadID:       &downloadID,
			DownloadClientID: &downloadClientModel.ID,
		})
		require.NoError(t, err)

		movie, err = m.storage.GetMovie(ctx, movieID)
		require.NoError(t, err)

		movieMetadata := model.MovieMetadata{Title: "my-movie", TmdbID: 1234}
		_, err = store.CreateMovieMetadata(ctx, movieMetadata)
		require.NoError(t, err)

		snapshot := newReconcileSnapshot(nil, []*model.DownloadClient{&downloadClientModel})

		err = m.reconcileDownloadingMovie(ctx, movie, snapshot)
		assert.Error(t, err)
		assert.Equal(t, "failed to add movie to library: expected testing error", err.Error())

		mov, err := store.GetMovie(ctx, 1)
		assert.NoError(t, err)
		assert.Equal(t, storage.MovieStateDownloading, mov.State)
	})

	t.Run("successfully reconciled downloading movie", func(t *testing.T) {
		store := newStore(t, ctx)
		mockLibrary := mockLibrary.NewMockLibrary(ctrl)
		mockLibrary.EXPECT().AddMovie(gomock.Any(), "my-movie", "/downloads/movie.mp4").Return(library.MovieFile{
			Name:         "my-movie",
			RelativePath: "my-movie/movie.mp4",
			AbsolutePath: "/movies/my-movie/movie.mp4",
			Size:         1024,
		}, nil)

		downloadClientModel := model.DownloadClient{
			Implementation: "transmission",
			Type:           "torrent",
			Port:           8080,
			Host:           "transmission",
			Scheme:         "http",
		}

		downloadClientID, err := store.CreateDownloadClient(ctx, downloadClientModel)
		require.NoError(t, err)

		downloadClientModel.ID = int32(downloadClientID)

		mockDownloadClient := downloadMock.NewMockDownloadClient(ctrl)
		mockFactory := downloadMock.NewMockFactory(ctrl)
		mockFactory.EXPECT().NewDownloadClient(downloadClientModel).Return(mockDownloadClient, nil)
		mockDownloadClient.EXPECT().Get(ctx, download.GetRequest{ID: "123"}).Return(download.Status{
			ID:        "123",
			Done:      true,
			FilePaths: []string{"/downloads/movie.mp4"},
		}, nil)

		m := New(nil, nil, mockLibrary, store, mockFactory, config.Manager{}, config.Config{})
		require.NotNil(t, m)

		movieID, err := m.storage.CreateMovie(ctx, storage.Movie{Movie: model.Movie{ID: 1, Monitored: 1, QualityProfileID: 1, MovieMetadataID: ptr(int32(1)), Path: ptr("my-movie")}}, storage.MovieStateMissing)
		require.NoError(t, err)

		movie, err := m.storage.GetMovie(ctx, movieID)
		require.NoError(t, err)

		downloadID := "123"
		err = m.updateMovieState(ctx, movie, storage.MovieStateDownloading, &storage.TransitionStateMetadata{
			DownloadID:       &downloadID,
			DownloadClientID: &downloadClientModel.ID,
		})
		require.NoError(t, err)

		movie, err = m.storage.GetMovie(ctx, movieID)
		require.NoError(t, err)

		movieMetadata := model.MovieMetadata{Title: "my-movie", TmdbID: 1234}
		_, err = store.CreateMovieMetadata(ctx, movieMetadata)
		require.NoError(t, err)

		snapshot := newReconcileSnapshot(nil, []*model.DownloadClient{&downloadClientModel})

		err = m.reconcileDownloadingMovie(ctx, movie, snapshot)
		assert.NoError(t, err)

		mov, err := store.GetMovie(ctx, 1)
		assert.NoError(t, err)
		assert.Equal(t, storage.MovieStateDownloaded, mov.State)

		dmfs, err := store.ListMovieFiles(ctx)
		require.NoError(t, err)
		require.Len(t, dmfs, 1)

		mfs, err := store.GetMovieFilesByMovieName(ctx, "my-movie")
		require.NoError(t, err)
		require.Len(t, mfs, 1)
		mf := mfs[0]
		assert.Equal(t, "my-movie/movie.mp4", *mf.RelativePath)
		assert.Equal(t, "/downloads/movie.mp4", *mf.OriginalFilePath)
		assert.Equal(t, int64(1024), mf.Size)
	})
}
