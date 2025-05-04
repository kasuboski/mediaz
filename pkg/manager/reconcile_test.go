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
	mio "github.com/kasuboski/mediaz/pkg/io"
	"github.com/kasuboski/mediaz/pkg/library"
	mockLibrary "github.com/kasuboski/mediaz/pkg/library/mocks"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	prowlMock "github.com/kasuboski/mediaz/pkg/prowlarr/mocks"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/mocks"
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

	store, err := mediaSqlite.New(":memory:")
	require.NoError(t, err)

	schemas, err := storage.ReadSchemaFiles("../storage/sqlite/schema/schema.sql", "../storage/sqlite/schema/defaults.sql")
	require.NoError(t, err)

	ctx := context.Background()
	err = store.Init(ctx, schemas...)
	require.NoError(t, err)

	prowlarrMock := prowlMock.NewMockClientInterface(ctrl)
	indexers := []Indexer{{ID: 1, Name: "test", Priority: 1}, {ID: 3, Name: "test2", Priority: 10}}

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
	prowlarrMock.EXPECT().GetAPIV1Search(gomock.Any(), gomock.Any()).Return(searchIndexersResponse(t, releases), nil).Times(len(indexers))

	pClient, err := prowlarr.New(":", "1234")
	pClient.ClientInterface = prowlarrMock
	require.NoError(t, err)

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

	m := New(tClient, pClient, lib, store, mockFactory, config.Manager{})
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

	err = m.reconcileMissingMovie(ctx, mov, snapshot)
	require.NoError(t, err)

	mov, err = m.storage.GetMovie(ctx, int64(mov.ID))
	require.NoError(t, err)

	assert.Equal(t, mov.State, storage.MovieStateDownloading)
}

func Test_Manager_reconcileDiscoveredMovie(t *testing.T) {
	t.Run("single result", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		store := mocks.NewMockStorage(ctrl)
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

		m := New(nil, nil, nil, store, nil, config.Manager{})
		m.tmdb = tClient

		movie := storage.Movie{
			Movie: model.Movie{
				ID:   1,
				Path: func() *string { s := "test movie"; return &s }(),
			},
		}

		ctx := context.Background()

		store.EXPECT().GetMovieMetadata(ctx, table.MovieMetadata.TmdbID.EQ(sqlite.Int(1234))).Return(&model.MovieMetadata{ID: 120}, nil)
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

		store := mocks.NewMockStorage(ctrl)
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

		m := New(nil, nil, nil, store, nil, config.Manager{})
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

		store := mocks.NewMockStorage(ctrl)
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

		m := New(nil, nil, nil, store, nil, config.Manager{})
		m.tmdb = tClient

		movie := storage.Movie{
			Movie: model.Movie{
				ID:   1,
				Path: func() *string { s := "test movie"; return &s }(),
			},
		}

		ctx := context.Background()

		store.EXPECT().GetMovieMetadata(ctx, table.MovieMetadata.TmdbID.EQ(sqlite.Int(1234))).Return(&model.MovieMetadata{ID: 120}, nil)
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

	t.Run("has metadata", func(t *testing.T) {
		ctrl := gomock.NewController(t)

		store := mocks.NewMockStorage(ctrl)
		tmdbHttpMock := mhttpMock.NewMockHTTPClient(ctrl)

		tClient, err := tmdb.New("https://api.themoviedb.org", "1234", tmdb.WithHTTPClient(tmdbHttpMock))
		require.NoError(t, err)

		m := New(nil, nil, nil, store, nil, config.Manager{})
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

	store, err := mediaSqlite.New(":memory:")
	require.NoError(t, err)

	schemas, err := storage.ReadSchemaFiles("../storage/sqlite/schema/schema.sql", "../storage/sqlite/schema/defaults.sql")
	require.NoError(t, err)

	ctx := context.Background()
	err = store.Init(ctx, schemas...)
	require.NoError(t, err)

	prowlarrMock := prowlMock.NewMockClientInterface(ctrl)
	indexers := []Indexer{{ID: 1, Name: "test", Priority: 1}, {ID: 3, Name: "test2", Priority: 10}}

	pClient, err := prowlarr.New(":", "1234")
	pClient.ClientInterface = prowlarrMock
	require.NoError(t, err)

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

	m := New(tClient, pClient, lib, store, mockFactory, config.Manager{})
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
		m := New(nil, nil, nil, nil, nil, config.Manager{})
		require.NotNil(t, m)

		movie := &storage.Movie{Movie: model.Movie{ID: 1}}
		err := m.reconcileDownloadingMovie(ctx, movie, nil)
		assert.NoError(t, err)
	})

	t.Run("movie is not monitored", func(t *testing.T) {
		m := New(nil, nil, nil, nil, nil, config.Manager{})
		require.NotNil(t, m)

		movie := &storage.Movie{Movie: model.Movie{ID: 1, Monitored: 0}}
		snapshot := newReconcileSnapshot(nil, nil)
		err := m.reconcileDownloadingMovie(ctx, movie, snapshot)
		assert.NoError(t, err)
	})

	t.Run("movie file is already tracked", func(t *testing.T) {
		store := newStore(t, ctx)
		m := New(nil, nil, nil, store, nil, config.Manager{})
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
		m := New(nil, nil, nil, store, nil, config.Manager{})
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

		m := New(nil, nil, nil, store, mockFactory, config.Manager{})
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

		m := New(nil, nil, nil, store, mockFactory, config.Manager{})
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

		m := New(nil, nil, nil, store, mockFactory, config.Manager{})
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

		m := New(nil, nil, nil, store, mockFactory, config.Manager{})
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

		m := New(nil, nil, mockLibrary, store, mockFactory, config.Manager{})
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

		m := New(nil, nil, mockLibrary, store, mockFactory, config.Manager{})
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
	t.Run("reconcile missing episodes", func(t *testing.T) {
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

		snapshot := newReconcileSnapshot([]Indexer{{ID: 1}}, []*model.DownloadClient{&downloadClientModel})

		m := New(nil, nil, nil, store, mockFactory, config.Manager{})

		err = m.reconcileMissingEpisodes(ctx, "Series", 1, episodes, snapshot, qualityProfile, releases)
		require.NoError(t, err)

		updatedEpisode, err := store.ListEpisodes(ctx)
		require.NoError(t, err)
		require.Len(t, updatedEpisode, 2)

		assert.Equal(t, storage.EpisodeStateDownloading, updatedEpisode[0].State)
		assert.Equal(t, "123", updatedEpisode[0].DownloadID)
		assert.Equal(t, int32(2), updatedEpisode[0].DownloadClientID)

		assert.Equal(t, storage.EpisodeStateDownloading, updatedEpisode[1].State)
		assert.Equal(t, "124", updatedEpisode[1].DownloadID)
		assert.Equal(t, int32(2), updatedEpisode[1].DownloadClientID)
	})

	t.Run("nil episode", func(t *testing.T) {
		m := New(nil, nil, nil, nil, nil, config.Manager{})
		err := m.reconcileMissingEpisodes(context.Background(), "Series", 1, []*storage.Episode{nil}, nil, storage.QualityProfile{}, nil)
		require.NoError(t, err)
	})

	t.Run("nil snapshot", func(t *testing.T) {
		m := New(nil, nil, nil, nil, nil, config.Manager{})
		episode := &storage.Episode{}
		err := m.reconcileMissingEpisodes(context.Background(), "Series", 1, []*storage.Episode{episode}, nil, storage.QualityProfile{}, nil)
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
		err = m.reconcileMissingEpisodes(ctx, "Series", 1, []*storage.Episode{&episode}, &ReconcileSnapshot{}, storage.QualityProfile{}, nil)
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
		err = m.reconcileMissingEpisodes(ctx, "Series", 1, []*storage.Episode{&episode}, &ReconcileSnapshot{}, storage.QualityProfile{}, nil)
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

		snapshot := newReconcileSnapshot([]Indexer{{ID: 1}}, []*model.DownloadClient{&downloadClientModel})

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
