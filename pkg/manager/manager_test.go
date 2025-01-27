package manager

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"testing"
	"testing/fstest"
	"time"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/kasuboski/mediaz/pkg/download"
	downloadMock "github.com/kasuboski/mediaz/pkg/download/mocks"
	mio "github.com/kasuboski/mediaz/pkg/io"
	"github.com/kasuboski/mediaz/pkg/library"
	mockLibrary "github.com/kasuboski/mediaz/pkg/library/mocks"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	prowlMock "github.com/kasuboski/mediaz/pkg/prowlarr/mocks"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/tmdb"
	"github.com/kasuboski/mediaz/pkg/tmdb/mocks"
	"github.com/oapi-codegen/nullable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestAddMovietoLibrary(t *testing.T) {
	ctrl := gomock.NewController(t)
	tmdbMock := mocks.NewMockClientInterface(ctrl)

	store, err := sqlite.New(":memory:")
	require.NoError(t, err)

	schemas, err := storage.ReadSchemaFiles("../storage/sqlite/schema/schema.sql", "../storage/sqlite/schema/defaults.sql")
	require.NoError(t, err)

	ctx := context.Background()
	err = store.Init(ctx, schemas...)
	require.NoError(t, err)

	// create a date in the past
	releaseDate := time.Now().AddDate(0, 0, -1).Format(tmdb.ReleaseDateFormat)
	tmdbMock.EXPECT().MovieDetails(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mediaDetailsResponse("test movie", 120, releaseDate), nil).Times(1)

	prowlarrMock := prowlMock.NewMockClientInterface(ctrl)

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
	pClient, err := prowlarr.New(":", "1234")
	pClient.ClientInterface = prowlarrMock
	require.NoError(t, err)

	tClient, err := tmdb.New(":", "1234")
	tClient.ClientInterface = tmdbMock
	require.NoError(t, err)

	mockFactory := downloadMock.NewMockFactory(ctrl)

	m := New(tClient, pClient, lib, store, mockFactory)
	require.NotNil(t, m)

	req := AddMovieRequest{
		TMDBID:           1234,
		QualityProfileID: 1,
	}

	mov, err := m.AddMovieToLibrary(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, mov)

	assert.Equal(t, int32(1), mov.ID)
	assert.Equal(t, storage.MovieStateMissing, mov.State)

	movie, err := m.storage.GetMovie(ctx, int64(mov.ID))
	require.Nil(t, err)

	movieMetadataID := int32(1)

	assert.Equal(t, &storage.Movie{
		Movie: model.Movie{
			ID:               1,
			Monitored:        1,
			QualityProfileID: 1,
			MovieMetadataID:  &movieMetadataID,
		},
		State: storage.MovieStateMissing,
	}, movie)
}

func Test_Manager_reconcileMissingMovie(t *testing.T) {
	ctrl := gomock.NewController(t)
	tmdbMock := mocks.NewMockClientInterface(ctrl)

	store, err := sqlite.New(":memory:")
	require.NoError(t, err)

	schemas, err := storage.ReadSchemaFiles("../storage/sqlite/schema/schema.sql", "../storage/sqlite/schema/defaults.sql")
	require.NoError(t, err)

	ctx := context.Background()
	err = store.Init(ctx, schemas...)
	require.NoError(t, err)

	releaseDate := time.Now().AddDate(0, 0, -1).Format(tmdb.ReleaseDateFormat)
	tmdbMock.EXPECT().MovieDetails(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mediaDetailsResponse("test movie", 120, releaseDate), nil).Times(1)

	prowlarrMock := prowlMock.NewMockClientInterface(ctrl)
	indexers := []Indexer{{ID: 1, Name: "test", Priority: 1}, {ID: 3, Name: "test2", Priority: 10}}
	// prowlarrMock.EXPECT().GetAPIV1Indexer(gomock.Any()).Return(indexersResponse(t, indexers), nil).Times(1)

	bigSeeders := nullable.NewNullNullable[int32]()
	bigSeeders.Set(23)

	smallerSeeders := nullable.NewNullNullable[int32]()
	smallerSeeders.Set(15)

	smallestSeeders := nullable.NewNullNullable[int32]()
	smallestSeeders.Set(10)

	torrentProto := ptr(prowlarr.DownloadProtocolTorrent)
	usenetProto := ptr(prowlarr.DownloadProtocolUsenet)

	wantRelease := &prowlarr.ReleaseResource{ID: intPtr(123), Title: nullable.NewNullableWithValue("test movie"), Size: sizeGBToBytes(23), Seeders: bigSeeders, Protocol: torrentProto}
	doNotWantRelease := &prowlarr.ReleaseResource{ID: intPtr(124), Title: nullable.NewNullableWithValue("test movie"), Size: sizeGBToBytes(23), Seeders: smallerSeeders, Protocol: torrentProto}
	smallMovie := &prowlarr.ReleaseResource{ID: intPtr(125), Title: nullable.NewNullableWithValue("test movie - very small"), Size: sizeGBToBytes(1), Seeders: smallestSeeders, Protocol: torrentProto}
	nzbMovie := &prowlarr.ReleaseResource{ID: intPtr(1225), Title: nullable.NewNullableWithValue("test movie - nzb"), Size: sizeGBToBytes(23), Seeders: smallestSeeders, Protocol: usenetProto}

	releases := []*prowlarr.ReleaseResource{doNotWantRelease, wantRelease, smallMovie, nzbMovie}
	prowlarrMock.EXPECT().GetAPIV1Search(gomock.Any(), gomock.Any()).Return(searchIndexersResponse(t, releases), nil).Times(len(indexers))

	pClient, err := prowlarr.New(":", "1234")
	pClient.ClientInterface = prowlarrMock
	require.NoError(t, err)

	tClient, err := tmdb.New(":", "1234")
	tClient.ClientInterface = tmdbMock
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

	m := New(tClient, pClient, lib, store, mockFactory)
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

func Test_Manager_reconcileUnreleasedMovie(t *testing.T) {
	ctrl := gomock.NewController(t)
	tmdbMock := mocks.NewMockClientInterface(ctrl)

	store, err := sqlite.New(":memory:")
	require.NoError(t, err)

	schemas, err := storage.ReadSchemaFiles("../storage/sqlite/schema/schema.sql", "../storage/sqlite/schema/defaults.sql")
	require.NoError(t, err)

	ctx := context.Background()
	err = store.Init(ctx, schemas...)
	require.NoError(t, err)

	releaseDate := time.Now().AddDate(0, 0, +5).Format(tmdb.ReleaseDateFormat)
	tmdbMock.EXPECT().MovieDetails(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mediaDetailsResponse("test movie", 120, releaseDate), nil).Times(1)

	prowlarrMock := prowlMock.NewMockClientInterface(ctrl)
	indexers := []Indexer{{ID: 1, Name: "test", Priority: 1}, {ID: 3, Name: "test2", Priority: 10}}

	pClient, err := prowlarr.New(":", "1234")
	pClient.ClientInterface = prowlarrMock
	require.NoError(t, err)

	tClient, err := tmdb.New(":", "1234")
	tClient.ClientInterface = tmdbMock
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

	m := New(tClient, pClient, lib, store, mockFactory)
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

func TestIndexMovieLibrary(t *testing.T) {
	t.Run("error listing finding files in library", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ctx := context.Background()

		library := mockLibrary.NewMockLibrary(ctrl)
		library.EXPECT().FindMovies(ctx).Times(1).Return(nil, errors.New("expected tested error"))
		m := New(nil, nil, library, nil, nil)
		require.NotNil(t, m)

		err := m.IndexMovieLibrary(ctx)
		assert.Error(t, err)
		assert.EqualError(t, err, "failed to index movie library: expected tested error")
	})

	t.Run("no files discovered", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ctx := context.Background()

		mockLibrary := mockLibrary.NewMockLibrary(ctrl)
		mockLibrary.EXPECT().FindMovies(ctx).Times(1).Return([]library.MovieFile{}, nil)
		m := New(nil, nil, mockLibrary, nil, nil)
		require.NotNil(t, m)

		err := m.IndexMovieLibrary(ctx)
		assert.NoError(t, err)
	})

	t.Run("error listing movie files from storage", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ctx := context.Background()

		store := newStore(t, ctx)
		mockLibrary := mockLibrary.NewMockLibrary(ctrl)

		discoveredFiles := []library.MovieFile{
			{RelativePath: "movie1.mp4", AbsolutePath: "/movies/movie1.mp4"},
		}

		mockLibrary.EXPECT().FindMovies(ctx).Times(1).Return(discoveredFiles, nil)

		m := New(nil, nil, mockLibrary, store, nil)
		require.NotNil(t, m)

		err := m.IndexMovieLibrary(ctx)
		assert.NoError(t, err)
	})

	t.Run("successfully indexes new movie files", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ctx := context.Background()

		store := newStore(t, ctx)
		mockLibrary := mockLibrary.NewMockLibrary(ctrl)

		discoveredFiles := []library.MovieFile{
			{RelativePath: "movie1.mp4", AbsolutePath: "/movies/movie1.mp4", Size: 1024},
			{RelativePath: "movie2.mkv", AbsolutePath: "/movies/movie2.mkv", Size: 2048},
			{RelativePath: "movie3.txt", AbsolutePath: "/movies/movie3.txt", Size: 10},
		}

		mockLibrary.EXPECT().FindMovies(ctx).Times(1).Return(discoveredFiles, nil)

		m := New(nil, nil, mockLibrary, store, nil)
		require.NotNil(t, m)

		err := m.IndexMovieLibrary(ctx)
		assert.NoError(t, err)

		movies, err := store.ListMovies(ctx)
		require.NoError(t, err)
		assert.Len(t, movies, 2)

		movieFiles, err := store.ListMovieFiles(ctx)
		require.NoError(t, err)
		assert.Len(t, movieFiles, 2)

		assert.Equal(t, "movie1.mp4", *movieFiles[0].RelativePath)
		assert.Equal(t, int64(1024), movieFiles[0].Size)
		assert.Equal(t, "movie2.mkv", *movieFiles[1].RelativePath)
		assert.Equal(t, int64(2048), movieFiles[1].Size)
	})

	t.Run("skips already tracked files", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ctx := context.Background()

		store := newStore(t, ctx)
		mockLibrary := mockLibrary.NewMockLibrary(ctrl)

		existingMovie := storage.Movie{Movie: model.Movie{Path: ptr("movie1.mp4")}}
		movieID, err := store.CreateMovie(ctx, existingMovie, storage.MovieStateDiscovered)
		require.NoError(t, err)

		_, err = store.CreateMovieFile(ctx, model.MovieFile{
			MovieID:          int32(movieID),
			RelativePath:     ptr("movie1.mp4"),
			OriginalFilePath: ptr("/movies/movie1.mp4"),
		})
		require.NoError(t, err)

		discoveredFiles := []library.MovieFile{
			{RelativePath: "movie1.mp4", AbsolutePath: "/movies/movie1.mp4"},
			{RelativePath: "movie2.mkv", AbsolutePath: "/movies/movie2.mkv"},
		}

		mockLibrary.EXPECT().FindMovies(ctx).Times(1).Return(discoveredFiles, nil)

		m := New(nil, nil, mockLibrary, store, nil)
		require.NotNil(t, m)

		err = m.IndexMovieLibrary(ctx)
		assert.NoError(t, err)

		movies, err := store.ListMovies(ctx)
		require.NoError(t, err)
		assert.Len(t, movies, 2)

		movieFiles, err := store.ListMovieFiles(ctx)
		require.NoError(t, err)
		assert.Len(t, movieFiles, 2)
	})
}

func TestRun(t *testing.T) {
	ctrl := gomock.NewController(t)
	tmdbMock := mocks.NewMockClientInterface(ctrl)
	prowlarrMock := prowlMock.NewMockClientInterface(ctrl)
	store, err := sqlite.New(":memory:")
	require.Nil(t, err)

	schemas, err := storage.ReadSchemaFiles("../storage/sqlite/schema/schema.sql")
	require.Nil(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()
	err = store.Init(ctx, schemas...)
	require.Nil(t, err)

	movieFS, expectedMovies := library.MovieFSFromFile(t, "../library/testing/test_movies.txt")
	require.NotEmpty(t, expectedMovies)
	tvFS, expectedEpisodes := library.TVFSFromFile(t, "../library/testing/test_episodes.txt")
	require.NotEmpty(t, expectedEpisodes)

	lib := library.New(
		library.FileSystem{
			FS: movieFS,
		},
		library.FileSystem{
			FS: tvFS,
		},
		&mio.MediaFileSystem{},
	)
	pClient, err := prowlarr.New(":", "1234")
	pClient.ClientInterface = prowlarrMock
	require.NoError(t, err)

	tClient, err := tmdb.New(":", "1234")
	tClient.ClientInterface = tmdbMock
	require.NoError(t, err)

	mockFactory := downloadMock.NewMockFactory(ctrl)
	m := New(tClient, pClient, lib, store, mockFactory)
	require.NotNil(t, m)

	err = m.Run(ctx)
	assert.ErrorIs(t, err, context.DeadlineExceeded)

}

func TestRejectRelease(t *testing.T) {
	t.Run("prefix match only", func(t *testing.T) {
		ctx := context.Background()
		det := &model.MovieMetadata{Title: "Brothers", Runtime: 60}
		profile := storage.QualityProfile{
			Name: "test",
			Qualities: []storage.QualityDefinition{{
				MinSize:       17.0,
				MaxSize:       2000,
				PreferredSize: 1999,
			}},
		}
		rejectFunc := rejectReleaseFunc(ctx, det, profile, map[string]struct{}{"usenet": {}, "torrent": {}})
		releases := getReleasesFromFile(t, "./testing/brother-releases.json")
		for _, r := range releases {
			got := rejectFunc(r)
			snaps.MatchSnapshot(t, []string{r.Title.MustGet(), strconv.FormatBool(got)})
		}
	})

	t.Run("reject by unavailable protocol", func(t *testing.T) {
		ctx := context.Background()
		det := &model.MovieMetadata{Title: "Interstellar"}
		profile := storage.QualityProfile{
			Name: "test",
			Qualities: []storage.QualityDefinition{{
				MinSize:       0,
				MaxSize:       1000,
				PreferredSize: 1999,
			}},
		}
		protocolsAvailable := map[string]struct{}{"torrent": {}, "ftp": {}}
		rejectFunc := rejectReleaseFunc(ctx, det, profile, protocolsAvailable)

		// Test case where the release protocol is not available
		r2 := &prowlarr.ReleaseResource{Protocol: ptr(prowlarr.DownloadProtocolUsenet), Size: ptr(int64(500))}
		if !rejectFunc(r2) {
			t.Errorf("Expected rejection for protocol 'usenet' since it's unavailable")
		}
	})
}

func Test_Manager_reconcileDownloadingMovie(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()

	t.Run("snapshot is nil", func(t *testing.T) {
		m := New(nil, nil, nil, nil, nil)
		require.NotNil(t, m)

		movie := &storage.Movie{Movie: model.Movie{ID: 1}}
		err := m.reconcileDownloadingMovie(ctx, movie, nil)
		assert.NoError(t, err)
	})

	t.Run("movie is not monitored", func(t *testing.T) {
		m := New(nil, nil, nil, nil, nil)
		require.NotNil(t, m)

		movie := &storage.Movie{Movie: model.Movie{ID: 1, Monitored: 0}}
		snapshot := newReconcileSnapshot(nil, nil)
		err := m.reconcileDownloadingMovie(ctx, movie, snapshot)
		assert.NoError(t, err)
	})

	t.Run("movie file is already tracked", func(t *testing.T) {
		store := newStore(t, ctx)
		m := New(nil, nil, nil, store, nil)
		require.NotNil(t, m)

		movie := storage.Movie{Movie: model.Movie{ID: 1, Monitored: 1}}
		_, err := store.CreateMovie(ctx, movie, storage.MovieStateMissing)
		require.NoError(t, err)

		err = m.updateMovieState(ctx, &movie, storage.MovieStateDownloading, nil)
		require.NoError(t, err)

		_, err = store.CreateMovieFile(ctx, model.MovieFile{MovieID: 1})
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
		m := New(nil, nil, nil, store, nil)
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

		m := New(nil, nil, nil, store, mockFactory)
		require.NotNil(t, m)

		movieID, err := m.storage.CreateMovie(ctx, storage.Movie{Movie: model.Movie{ID: 1, Monitored: 1, QualityProfileID: 1}}, storage.MovieStateMissing)
		require.NoError(t, err)

		movie, err := m.storage.GetMovie(ctx, movieID)
		require.NoError(t, err)

		downloadID := "123"
		err = m.updateMovieState(ctx, movie, storage.MovieStateDownloading, &storage.MovieStateMetadata{
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

		m := New(nil, nil, nil, store, mockFactory)
		require.NotNil(t, m)

		movieID, err := m.storage.CreateMovie(ctx, storage.Movie{Movie: model.Movie{ID: 1, Monitored: 1, QualityProfileID: 1}}, storage.MovieStateMissing)
		require.NoError(t, err)

		movie, err := m.storage.GetMovie(ctx, movieID)
		require.NoError(t, err)

		downloadID := "123"
		err = m.updateMovieState(ctx, movie, storage.MovieStateDownloading, &storage.MovieStateMetadata{
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

		m := New(nil, nil, nil, store, mockFactory)
		require.NotNil(t, m)

		movieID, err := m.storage.CreateMovie(ctx, storage.Movie{Movie: model.Movie{ID: 1, Monitored: 1, QualityProfileID: 1}}, storage.MovieStateMissing)
		require.NoError(t, err)

		movie, err := m.storage.GetMovie(ctx, movieID)
		require.NoError(t, err)

		downloadID := "123"
		err = m.updateMovieState(ctx, movie, storage.MovieStateDownloading, &storage.MovieStateMetadata{
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

		m := New(nil, nil, nil, store, mockFactory)
		require.NotNil(t, m)

		movieID, err := m.storage.CreateMovie(ctx, storage.Movie{Movie: model.Movie{ID: 1, Monitored: 1, QualityProfileID: 1, MovieMetadataID: intPtr(1)}}, storage.MovieStateMissing)
		require.NoError(t, err)

		movie, err := m.storage.GetMovie(ctx, movieID)
		require.NoError(t, err)

		downloadID := "123"
		err = m.updateMovieState(ctx, movie, storage.MovieStateDownloading, &storage.MovieStateMetadata{
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

		m := New(nil, nil, mockLibrary, store, mockFactory)
		require.NotNil(t, m)

		movieID, err := m.storage.CreateMovie(ctx, storage.Movie{Movie: model.Movie{ID: 1, Monitored: 1, QualityProfileID: 1, MovieMetadataID: intPtr(1)}}, storage.MovieStateMissing)
		require.NoError(t, err)

		movie, err := m.storage.GetMovie(ctx, movieID)
		require.NoError(t, err)

		downloadID := "123"
		err = m.updateMovieState(ctx, movie, storage.MovieStateDownloading, &storage.MovieStateMetadata{
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
			Name:         "test movie",
			RelativePath: "/movies/my-movie/movie.mp4",
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

		m := New(nil, nil, mockLibrary, store, mockFactory)
		require.NotNil(t, m)

		movieID, err := m.storage.CreateMovie(ctx, storage.Movie{Movie: model.Movie{ID: 1, Monitored: 1, QualityProfileID: 1, MovieMetadataID: intPtr(1)}}, storage.MovieStateMissing)
		require.NoError(t, err)

		movie, err := m.storage.GetMovie(ctx, movieID)
		require.NoError(t, err)

		downloadID := "123"
		err = m.updateMovieState(ctx, movie, storage.MovieStateDownloading, &storage.MovieStateMetadata{
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

		mfs, err := store.GetMovieFiles(ctx, 1)
		require.NoError(t, err)
		require.Len(t, mfs, 1)
		mf := mfs[0]
		assert.Equal(t, "/movies/my-movie/movie.mp4", *mf.RelativePath)
		assert.Equal(t, "/downloads/movie.mp4", *mf.OriginalFilePath)
		assert.Equal(t, int64(1024), mf.Size)
	})
}

func getReleasesFromFile(t *testing.T, path string) []*prowlarr.ReleaseResource {
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	releases := make([]*prowlarr.ReleaseResource, 0)
	err = json.Unmarshal(b, &releases)
	if err != nil {
		t.Fatal(err)
	}
	return releases
}

func searchIndexersResponse(t *testing.T, releases []*prowlarr.ReleaseResource) *http.Response {
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(map[string][]string),
	}

	out, err := json.Marshal(releases)
	assert.Nil(t, err)

	resp.Body = io.NopCloser(bytes.NewBuffer(out))

	return resp
}

// mediaDetailsResponse returns an http.Response that represents a MediaDetails with the given title and runtime
func mediaDetailsResponse(title string, runtime int, releaseDate string) *http.Response {
	details := &tmdb.MediaDetails{
		ID:          1,
		Title:       &title,
		Runtime:     &runtime,
		ReleaseDate: &releaseDate,
	}

	b, _ := json.Marshal(details)

	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBuffer(b)),
	}
}

func sizeGBToBytes(gb int) *int64 {
	b := int64(gb * 1024 * 1024 * 1024)
	return &b
}

func ptr[A any](thing A) *A {
	return &thing
}

func newStore(t *testing.T, ctx context.Context) storage.Storage {
	store, err := sqlite.New(":memory:")
	require.NoError(t, err)

	schemas, err := storage.ReadSchemaFiles("../storage/sqlite/schema/schema.sql", "../storage/sqlite/schema/defaults.sql")
	require.NoError(t, err)

	err = store.Init(ctx, schemas...)
	require.NoError(t, err)

	return store
}
