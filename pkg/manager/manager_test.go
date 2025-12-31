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
	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/config"
	downloadMock "github.com/kasuboski/mediaz/pkg/download/mocks"
	mhttpMock "github.com/kasuboski/mediaz/pkg/http/mocks"
	mio "github.com/kasuboski/mediaz/pkg/io"
	"github.com/kasuboski/mediaz/pkg/library"
	mockLibrary "github.com/kasuboski/mediaz/pkg/library/mocks"
	"github.com/kasuboski/mediaz/pkg/pagination"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	prowlMock "github.com/kasuboski/mediaz/pkg/prowlarr/mocks"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/mocks"
	mediaSqlite "github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
	"github.com/kasuboski/mediaz/pkg/tmdb"
	tmdbMocks "github.com/kasuboski/mediaz/pkg/tmdb/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestAddMovietoLibrary(t *testing.T) {
	ctrl := gomock.NewController(t)

	store, err := mediaSqlite.New(context.Background(), ":memory:")
	require.NoError(t, err)

	schemas, err := storage.ReadSchemaFiles("../storage/sqlite/schema/schema.sql", "../storage/sqlite/schema/defaults.sql")
	require.NoError(t, err)

	ctx := context.Background()
	err = store.Init(ctx, schemas...)
	require.NoError(t, err)

	// create a date in the past
	releaseDate := time.Now().AddDate(0, 0, -1).Format(tmdb.ReleaseDateFormat)

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

	tmdbHttpMock := mhttpMock.NewMockHTTPClient(ctrl)
	tmdbHttpMock.EXPECT().Do(gomock.Any()).Return(mediaDetailsResponse("test movie", 120, releaseDate), nil).Times(1)

	tClient, err := tmdb.New("https://api.themoviedb.org", "1234", tmdb.WithHTTPClient(tmdbHttpMock))
	require.NoError(t, err)

	mockFactory := downloadMock.NewMockFactory(ctrl)

	m := New(tClient, pClient, lib, store, mockFactory, config.Manager{}, config.Config{})
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
	movie.Added = nil
	movieMetadataID := int32(1)

	assert.Equal(t, &storage.Movie{
		Movie: model.Movie{
			ID:               1,
			Monitored:        1,
			QualityProfileID: 1,
			MovieMetadataID:  &movieMetadataID,
			Path:             ptr("test movie"),
		},
		State: storage.MovieStateMissing,
	}, movie)
}

func TestListMoviesInLibrary(t *testing.T) {
	ctx := context.Background()

	t.Run("no movies in library", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := mocks.NewMockStorage(ctrl)
		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})

		store.EXPECT().ListMovies(ctx).Return([]*storage.Movie{}, nil)

		movies, err := m.ListMoviesInLibrary(ctx)
		require.NoError(t, err)
		assert.Empty(t, movies)
	})

	t.Run("movies with metadata", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := mocks.NewMockStorage(ctrl)
		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})

		metadataID := int32(1)
		path := "movie1"
		discoveredMovie := &storage.Movie{
			Movie: model.Movie{
				MovieMetadataID: &metadataID,
				Path:            &path,
			},
			State: storage.MovieStateDiscovered,
		}

		downloadedMovie := &storage.Movie{
			Movie: model.Movie{
				MovieMetadataID: &metadataID,
				Path:            &path,
			},
			State: storage.MovieStateDownloaded,
		}

		year := int32(2024)
		movieMetadata := &model.MovieMetadata{
			ID:     1,
			TmdbID: 123,
			Title:  "Test Movie",
			Images: "poster.jpg",
			Year:   &year,
		}

		store.EXPECT().ListMovies(ctx).Return([]*storage.Movie{discoveredMovie, downloadedMovie}, nil)
		store.EXPECT().GetMovieMetadata(ctx, gomock.Any()).Return(movieMetadata, nil).Times(2)

		movies, err := m.ListMoviesInLibrary(ctx)
		require.NoError(t, err)
		assert.Len(t, movies, 2)

		expectedMovie := LibraryMovie{
			Path:       path,
			TMDBID:     movieMetadata.TmdbID,
			Title:      movieMetadata.Title,
			PosterPath: movieMetadata.Images,
			Year:       *movieMetadata.Year,
		}

		for _, movie := range movies {
			assert.Equal(t, expectedMovie.Path, movie.Path)
			assert.Equal(t, expectedMovie.TMDBID, movie.TMDBID)
			assert.Equal(t, expectedMovie.Title, movie.Title)
			assert.Equal(t, expectedMovie.PosterPath, movie.PosterPath)
			assert.Equal(t, expectedMovie.Year, movie.Year)
		}
	})

	t.Run("movies without metadata", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := mocks.NewMockStorage(ctrl)
		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})

		path := "movie1"
		discoveredMovie := &storage.Movie{
			Movie: model.Movie{
				Path: &path,
			},
			State: storage.MovieStateDiscovered,
		}

		store.EXPECT().ListMovies(ctx).Return([]*storage.Movie{discoveredMovie}, nil)

		movies, err := m.ListMoviesInLibrary(ctx)
		require.NoError(t, err)
		assert.Len(t, movies, 1)

		expectedMovie := LibraryMovie{
			Path:  path,
			State: string(storage.MovieStateDiscovered),
		}
		assert.Equal(t, expectedMovie, movies[0])
	})

	t.Run("error listing movies", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := mocks.NewMockStorage(ctrl)
		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})

		store.EXPECT().ListMovies(ctx).Return(nil, errors.New("db error"))

		movies, err := m.ListMoviesInLibrary(ctx)
		assert.Error(t, err)
		assert.Nil(t, movies)
	})
}

func TestListShowsInLibrary(t *testing.T) {
	ctx := context.Background()

	t.Run("no shows in library", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := mocks.NewMockStorage(ctrl)
		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})

		store.EXPECT().ListSeries(ctx).Return([]*storage.Series{}, nil)

		shows, err := m.ListShowsInLibrary(ctx)
		require.NoError(t, err)
		assert.Empty(t, shows)
	})

	t.Run("shows with metadata", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := mocks.NewMockStorage(ctrl)
		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})

		metadataID := int32(1)
		path := "Show 1"
		series := &storage.Series{
			Series: model.Series{
				SeriesMetadataID: &metadataID,
				Path:             &path,
			},
			State: storage.SeriesStateDiscovered,
		}

		seriesMetadata := &model.SeriesMetadata{
			ID:             1,
			TmdbID:         321,
			Title:          "Test Series",
			Status:         "Continuing",
			PosterPath:     ptr("poster.jpg"),
			ExternalIds:    nil,
			WatchProviders: nil,
		}

		store.EXPECT().ListSeries(ctx).Return([]*storage.Series{series}, nil)
		store.EXPECT().GetSeriesMetadata(ctx, gomock.Any()).Return(seriesMetadata, nil)

		shows, err := m.ListShowsInLibrary(ctx)
		require.NoError(t, err)
		require.Len(t, shows, 1)

		expected := LibraryShow{
			Path:       path,
			TMDBID:     seriesMetadata.TmdbID,
			Title:      seriesMetadata.Title,
			PosterPath: *seriesMetadata.PosterPath,
		}
		assert.Equal(t, expected.Path, shows[0].Path)
		assert.Equal(t, expected.TMDBID, shows[0].TMDBID)
		assert.Equal(t, expected.Title, shows[0].Title)
		assert.Equal(t, expected.PosterPath, shows[0].PosterPath)
	})

	t.Run("shows without metadata", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := mocks.NewMockStorage(ctrl)
		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})

		path := "Show 1"
		series := &storage.Series{
			Series: model.Series{
				Path: &path,
			},
			State: storage.SeriesStateDiscovered,
		}

		store.EXPECT().ListSeries(ctx).Return([]*storage.Series{series}, nil)

		shows, err := m.ListShowsInLibrary(ctx)
		require.NoError(t, err)
		require.Len(t, shows, 1)

		expected := LibraryShow{
			Path:  path,
			State: string(storage.SeriesStateDiscovered),
		}
		assert.Equal(t, expected, shows[0])
	})

	t.Run("error listing series", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := mocks.NewMockStorage(ctrl)
		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})

		store.EXPECT().ListSeries(ctx).Return(nil, errors.New("db error"))

		shows, err := m.ListShowsInLibrary(ctx)
		assert.Error(t, err)
		assert.Nil(t, shows)
	})
}

func TestIndexMovieLibrary(t *testing.T) {
	t.Run("error listing finding files in library", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ctx := context.Background()

		library := mockLibrary.NewMockLibrary(ctrl)
		library.EXPECT().FindMovies(ctx).Times(1).Return(nil, errors.New("expected tested error"))
		m := New(nil, nil, library, nil, nil, config.Manager{}, config.Config{})
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
		m := New(nil, nil, mockLibrary, nil, nil, config.Manager{}, config.Config{})
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

		m := New(nil, nil, mockLibrary, store, nil, config.Manager{}, config.Config{})
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
			{RelativePath: "Movie 1/movie1.mp4", AbsolutePath: "/movies/movie1.mp4", Size: 1024},
			{RelativePath: "Movie 2/movie2.mkv", AbsolutePath: "/movies/movie2.mkv", Size: 2048},
		}

		mockLibrary.EXPECT().FindMovies(ctx).Times(1).Return(discoveredFiles, nil)

		m := New(nil, nil, mockLibrary, store, nil, config.Manager{}, config.Config{})
		require.NotNil(t, m)

		err := m.IndexMovieLibrary(ctx)
		assert.NoError(t, err)

		movies, err := store.ListMovies(ctx)
		require.NoError(t, err)
		assert.Len(t, movies, 2)

		movieFiles, err := store.ListMovieFiles(ctx)
		require.NoError(t, err)
		assert.Len(t, movieFiles, 2)

		assert.Equal(t, "Movie 1/movie1.mp4", *movieFiles[0].RelativePath)
		assert.Equal(t, int64(1024), movieFiles[0].Size)
		assert.Equal(t, "Movie 2/movie2.mkv", *movieFiles[1].RelativePath)
		assert.Equal(t, int64(2048), movieFiles[1].Size)

		assert.Equal(t, int32(1), *movies[0].MovieFileID)
		assert.Equal(t, "Movie 1", *movies[0].Path)
		assert.Equal(t, int32(2), *movies[1].MovieFileID)
		assert.Equal(t, "Movie 2", *movies[1].Path)
	})

	t.Run("create movie for already tracked file", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ctx := context.Background()

		store := newStore(t, ctx)
		mockLibrary := mockLibrary.NewMockLibrary(ctrl)

		_, err := store.CreateMovieFile(ctx, model.MovieFile{
			Size:             1024,
			RelativePath:     ptr("Movie 1/movie1.mp4"),
			OriginalFilePath: ptr("/movies/Movie 1/movie1.mp4"),
		})
		require.NoError(t, err)

		discoveredFiles := []library.MovieFile{
			{RelativePath: "Movie 1/movie1.mp4", AbsolutePath: "/movies/Movie 1/movie1.mp4"},
		}

		mockLibrary.EXPECT().FindMovies(ctx).Times(1).Return(discoveredFiles, nil)

		m := New(nil, nil, mockLibrary, store, nil, config.Manager{}, config.Config{})
		require.NotNil(t, m)

		err = m.IndexMovieLibrary(ctx)
		assert.NoError(t, err)

		movieFiles, err := store.ListMovieFiles(ctx)
		require.NoError(t, err)
		assert.Len(t, movieFiles, 1)

		movies, err := store.ListMovies(ctx)
		require.NoError(t, err)
		require.Len(t, movies, 1)

		assert.Equal(t, int32(1), *movies[0].MovieFileID)
		assert.Equal(t, "Movie 1", *movies[0].Path)
	})

	t.Run("link new movie file to existing movie", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ctx := context.Background()

		store := newStore(t, ctx)
		mockLibrary := mockLibrary.NewMockLibrary(ctrl)

		_, err := store.CreateMovieFile(ctx, model.MovieFile{
			Size:             1024,
			RelativePath:     ptr("Movie 1/movie1.mp4"),
			OriginalFilePath: ptr("/movies/Movie 1/movie1.mp4"),
		})
		require.NoError(t, err)

		discoveredFiles := []library.MovieFile{
			{RelativePath: "Movie 1/movie1.mp4", AbsolutePath: "/movies/Movie 1/movie1.mp4"},
		}

		mockLibrary.EXPECT().FindMovies(ctx).Times(1).Return(discoveredFiles, nil)

		m := New(nil, nil, mockLibrary, store, nil, config.Manager{}, config.Config{})
		require.NotNil(t, m)

		err = m.IndexMovieLibrary(ctx)
		assert.NoError(t, err)

		movieFiles, err := store.ListMovieFiles(ctx)
		require.NoError(t, err)
		assert.Len(t, movieFiles, 1)

		movies, err := store.ListMovies(ctx)
		require.NoError(t, err)
		require.Len(t, movies, 1)

		assert.Equal(t, int32(1), *movies[0].MovieFileID)
		assert.Equal(t, "Movie 1", *movies[0].Path)

		discoveredFiles = []library.MovieFile{
			{RelativePath: "Movie 1/movie1.mp4", AbsolutePath: "/movies/Movie 1/movie1.mp4"},
			{RelativePath: "Movie 1/movie1.mkv", AbsolutePath: "/movies/Movie 1/movie1.mkv"},
		}

		mockLibrary.EXPECT().FindMovies(ctx).Times(1).Return(discoveredFiles, nil)

		err = m.IndexMovieLibrary(ctx)
		assert.NoError(t, err)

		movieFiles, err = store.ListMovieFiles(ctx)
		require.NoError(t, err)
		assert.Len(t, movieFiles, 2)

		movies, err = store.ListMovies(ctx)
		require.NoError(t, err)
		require.Len(t, movies, 1)
	})
}

func TestMovieRejectRelease(t *testing.T) {
	t.Run("prefix match only", func(t *testing.T) {
		ctx := context.Background()
		det := &model.MovieMetadata{Title: "Brothers", Runtime: 60}
		profile := storage.QualityProfile{
			Name: "test",
			Qualities: []storage.QualityDefinition{{
				Name:          "WEBDL-1080p",
				MinSize:       17.0,
				MaxSize:       2000,
				PreferredSize: 1999,
			}},
		}
		protocols := map[string]struct{}{"usenet": {}, "torrent": {}}
		rejectFunc := RejectMovieReleaseFunc(ctx, det.Title, det.Runtime, profile, protocols)

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
		rejectFunc := RejectMovieReleaseFunc(ctx, det.Title, det.Runtime, profile, protocolsAvailable)

		// Test case where the release protocol is not available
		r2 := &prowlarr.ReleaseResource{Protocol: ptr(prowlarr.DownloadProtocolUsenet), Size: ptr(int64(500))}
		if !rejectFunc(r2) {
			t.Errorf("Expected rejection for protocol 'usenet' since it's unavailable")
		}
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

func newStore(t *testing.T, ctx context.Context) storage.Storage {
	store, err := mediaSqlite.New(context.Background(), ":memory:")
	require.NoError(t, err)

	schemas, err := storage.ReadSchemaFiles("../storage/sqlite/schema/schema.sql", "../storage/sqlite/schema/defaults.sql")
	require.NoError(t, err)

	err = store.Init(ctx, schemas...)
	require.NoError(t, err)

	return store
}

func TestGetMovieDetailByTMDBID(t *testing.T) {
	ctx := context.Background()

	t.Run("success - movie not in library", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := mocks.NewMockStorage(ctrl)
		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})

		releaseDate := time.Date(2023, 1, 15, 0, 0, 0, 0, time.UTC)
		year := int32(2023)
		metadata := &model.MovieMetadata{
			ID:               1,
			TmdbID:           123,
			Title:            "Test Movie",
			OriginalTitle:    ptr("Original Test Movie"),
			Overview:         ptr("Test movie overview"),
			Images:           "poster.jpg",
			Runtime:          120,
			Genres:           ptr("Action, Drama"),
			Studio:           ptr("Test Studio"),
			Website:          ptr("https://test.com"),
			CollectionTmdbID: ptr(int32(456)),
			CollectionTitle:  ptr("Test Collection"),
			Popularity:       ptr(8.5),
			Year:             &year,
			ReleaseDate:      &releaseDate,
		}

		store.EXPECT().GetMovieMetadata(ctx, gomock.Any()).Return(metadata, nil)
		store.EXPECT().GetMovieByMetadataID(ctx, 1).Return(nil, storage.ErrNotFound)

		result, err := m.GetMovieDetailByTMDBID(ctx, 123)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.Equal(t, int32(123), result.TMDBID)
		assert.Equal(t, ptr("Original Test Movie"), result.OriginalTitle)
		assert.Equal(t, "Test Movie", result.Title)
		assert.Equal(t, ptr("Test movie overview"), result.Overview)
		assert.Equal(t, "poster.jpg", result.PosterPath)
		assert.Equal(t, ptr(int32(120)), result.Runtime)
		assert.Equal(t, ptr("Action, Drama"), result.Genres)
		assert.Equal(t, ptr("Test Studio"), result.Studio)
		assert.Equal(t, ptr("https://test.com"), result.Website)
		assert.Equal(t, ptr(int32(456)), result.CollectionTmdbID)
		assert.Equal(t, ptr("Test Collection"), result.CollectionTitle)
		assert.Equal(t, ptr(8.5), result.Popularity)
		assert.Equal(t, &year, result.Year)
		assert.Equal(t, ptr("2023-01-15"), result.ReleaseDate)
		assert.Equal(t, "Not In Library", result.LibraryStatus)
		assert.Nil(t, result.Path)
		assert.Nil(t, result.QualityProfileID)
		assert.Nil(t, result.Monitored)
	})

	t.Run("success - movie in library", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := mocks.NewMockStorage(ctrl)
		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})

		metadata := &model.MovieMetadata{
			ID:      1,
			TmdbID:  123,
			Title:   "Test Movie",
			Images:  "poster.jpg",
			Runtime: 120,
		}

		path := "/movies/test-movie"
		qualityProfileID := int32(2)
		movie := &storage.Movie{
			Movie: model.Movie{
				ID:               1,
				MovieMetadataID:  ptr(int32(1)),
				Path:             &path,
				QualityProfileID: qualityProfileID,
				Monitored:        1,
			},
			State: storage.MovieStateDownloaded,
		}

		store.EXPECT().GetMovieMetadata(ctx, gomock.Any()).Return(metadata, nil)
		store.EXPECT().GetMovieByMetadataID(ctx, 1).Return(movie, nil)

		result, err := m.GetMovieDetailByTMDBID(ctx, 123)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.Equal(t, int32(123), result.TMDBID)
		assert.Equal(t, "Test Movie", result.Title)
		assert.Equal(t, string(storage.MovieStateDownloaded), result.LibraryStatus)
		assert.Equal(t, &path, result.Path)
		assert.Equal(t, &qualityProfileID, result.QualityProfileID)
		monitored := true
		assert.Equal(t, &monitored, result.Monitored)
	})

	t.Run("success - movie with nil release date", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := mocks.NewMockStorage(ctrl)
		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})

		metadata := &model.MovieMetadata{
			ID:          1,
			TmdbID:      123,
			Title:       "Test Movie",
			Images:      "poster.jpg",
			Runtime:     120,
			ReleaseDate: nil,
		}

		store.EXPECT().GetMovieMetadata(ctx, gomock.Any()).Return(metadata, nil)
		store.EXPECT().GetMovieByMetadataID(ctx, 1).Return(nil, storage.ErrNotFound)

		result, err := m.GetMovieDetailByTMDBID(ctx, 123)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.Equal(t, int32(123), result.TMDBID)
		assert.Nil(t, result.ReleaseDate)
		assert.Equal(t, "Not In Library", result.LibraryStatus)
	})

	t.Run("success - movie with unmonitored status", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := mocks.NewMockStorage(ctrl)
		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})

		metadata := &model.MovieMetadata{
			ID:      1,
			TmdbID:  123,
			Title:   "Test Movie",
			Images:  "poster.jpg",
			Runtime: 120,
		}

		movie := &storage.Movie{
			Movie: model.Movie{
				ID:               1,
				MovieMetadataID:  ptr(int32(1)),
				QualityProfileID: 1,
				Monitored:        0, // Unmonitored
			},
			State: storage.MovieStateMissing,
		}

		store.EXPECT().GetMovieMetadata(ctx, gomock.Any()).Return(metadata, nil)
		store.EXPECT().GetMovieByMetadataID(ctx, 1).Return(movie, nil)

		result, err := m.GetMovieDetailByTMDBID(ctx, 123)
		require.NoError(t, err)
		require.NotNil(t, result)

		monitored := false
		assert.Equal(t, &monitored, result.Monitored)
		assert.Equal(t, string(storage.MovieStateMissing), result.LibraryStatus)
	})

	t.Run("error - GetMovieMetadata fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := mocks.NewMockStorage(ctrl)
		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})

		expectedErr := errors.New("metadata fetch error")
		store.EXPECT().GetMovieMetadata(ctx, gomock.Any()).Return(nil, expectedErr)

		result, err := m.GetMovieDetailByTMDBID(ctx, 123)
		assert.Error(t, err)
		assert.Equal(t, expectedErr, err)
		assert.Nil(t, result)
	})

	t.Run("success - storage error non-NotFound is logged but not returned", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := mocks.NewMockStorage(ctrl)
		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})

		metadata := &model.MovieMetadata{
			ID:      1,
			TmdbID:  123,
			Title:   "Test Movie",
			Images:  "poster.jpg",
			Runtime: 120,
		}

		dbErr := errors.New("database connection error")
		store.EXPECT().GetMovieMetadata(ctx, gomock.Any()).Return(metadata, nil)
		store.EXPECT().GetMovieByMetadataID(ctx, 1).Return(nil, dbErr)

		result, err := m.GetMovieDetailByTMDBID(ctx, 123)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.Equal(t, int32(123), result.TMDBID)
		assert.Equal(t, "Not In Library", result.LibraryStatus)
		assert.Nil(t, result.Path)
		assert.Nil(t, result.QualityProfileID)
		assert.Nil(t, result.Monitored)
	})
}

func TestBuildTVDetailResult(t *testing.T) {
	t.Run("builds complete TV detail result", func(t *testing.T) {
		m := New(nil, nil, nil, nil, nil, config.Manager{}, config.Config{})

		// Mock series metadata
		firstAirDate := time.Date(2023, 1, 15, 0, 0, 0, 0, time.UTC)
		lastAirDate := time.Date(2023, 12, 15, 0, 0, 0, 0, time.UTC)
		metadata := &model.SeriesMetadata{
			ID:             1,
			TmdbID:         123,
			Title:          "Test Series",
			Status:         "Continuing",
			Overview:       ptr("Test series overview"),
			FirstAirDate:   &firstAirDate,
			LastAirDate:    &lastAirDate,
			SeasonCount:    2,
			EpisodeCount:   20,
			ExternalIds:    nil,
			WatchProviders: nil,
		}

		// Mock TMDB details response
		details := &tmdb.SeriesDetailsResponse{
			PosterPath:   "poster.jpg",
			BackdropPath: "backdrop.jpg",
			Adult:        true,
			Popularity:   8.5,
			Networks: []tmdb.Network{
				{Name: "HBO"},
				{Name: "Netflix"},
			},
			Genres: []tmdb.Genre{
				{Name: "Drama"},
				{Name: "Thriller"},
			},
		}

		// Mock library series - in library and monitored
		path := "/tv/test-series"
		qualityProfileID := int32(2)
		series := &storage.Series{
			Series: model.Series{
				ID:               1,
				SeriesMetadataID: ptr(int32(1)),
				Path:             &path,
				QualityProfileID: qualityProfileID,
				Monitored:        1,
			},
			State: storage.SeriesStateDiscovered,
		}

		result := m.buildTVDetailResult(metadata, details, series, []SeasonResult{})

		assert.Equal(t, int32(123), result.TMDBID)
		assert.Equal(t, "Test Series", result.Title)
		assert.Equal(t, "Test series overview", result.Overview)
		assert.Equal(t, "poster.jpg", result.PosterPath)
		assert.Equal(t, ptr("backdrop.jpg"), result.BackdropPath)
		assert.Equal(t, ptr("2023-01-15"), result.FirstAirDate)
		assert.Equal(t, ptr("2023-12-15"), result.LastAirDate)
		assert.Equal(t, int32(2), result.SeasonCount)
		assert.Equal(t, int32(20), result.EpisodeCount)
		require.Len(t, result.Networks, 2)
		assert.Equal(t, "HBO", result.Networks[0].Name)
		assert.Equal(t, "Netflix", result.Networks[1].Name)
		assert.Equal(t, []string{"Drama", "Thriller"}, result.Genres)
		assert.Equal(t, ptr(true), result.Adult)
		pop := float64(8.5)
		assert.Equal(t, &pop, result.Popularity)
		assert.Equal(t, string(storage.SeriesStateDiscovered), result.LibraryStatus)
		assert.Equal(t, &path, result.Path)
		assert.Equal(t, &qualityProfileID, result.QualityProfileID)
		monitored := true
		assert.Equal(t, &monitored, result.Monitored)
	})

	t.Run("builds TV detail result not in library", func(t *testing.T) {
		m := New(nil, nil, nil, nil, nil, config.Manager{}, config.Config{})

		metadata := &model.SeriesMetadata{
			ID:             1,
			TmdbID:         123,
			Title:          "Test Series",
			Status:         "Continuing",
			SeasonCount:    1,
			EpisodeCount:   10,
			ExternalIds:    nil,
			WatchProviders: nil,
		}

		details := &tmdb.SeriesDetailsResponse{
			PosterPath:   "poster.jpg",
			BackdropPath: "backdrop.jpg",
		}

		// No series in library (nil)
		result := m.buildTVDetailResult(metadata, details, nil, []SeasonResult{})

		assert.Equal(t, int32(123), result.TMDBID)
		assert.Equal(t, "Test Series", result.Title)
		assert.Equal(t, "Not In Library", result.LibraryStatus)
		assert.Nil(t, result.Path)
		assert.Nil(t, result.QualityProfileID)
		assert.Nil(t, result.Monitored)
	})

	t.Run("builds TV detail result with minimal data", func(t *testing.T) {
		m := New(nil, nil, nil, nil, nil, config.Manager{}, config.Config{})

		metadata := &model.SeriesMetadata{
			ID:             1,
			TmdbID:         123,
			Title:          "Test Series",
			Status:         "Continuing",
			SeasonCount:    1,
			EpisodeCount:   5,
			ExternalIds:    nil,
			WatchProviders: nil,
		}

		details := &tmdb.SeriesDetailsResponse{
			PosterPath: "poster.jpg",
			// No backdrop, networks, genres, etc.
		}

		result := m.buildTVDetailResult(metadata, details, nil, []SeasonResult{})

		assert.Equal(t, int32(123), result.TMDBID)
		assert.Equal(t, "Test Series", result.Title)
		assert.Equal(t, "poster.jpg", result.PosterPath)
		assert.Nil(t, result.BackdropPath)
		assert.Nil(t, result.FirstAirDate)
		assert.Nil(t, result.LastAirDate)
		assert.Empty(t, result.Networks)
		assert.Empty(t, result.Genres)
		assert.Nil(t, result.Adult)
		assert.Nil(t, result.Popularity)
		assert.Equal(t, "Not In Library", result.LibraryStatus)
	})

	t.Run("builds TV detail result with unmonitored series", func(t *testing.T) {
		m := New(nil, nil, nil, nil, nil, config.Manager{}, config.Config{})

		metadata := &model.SeriesMetadata{
			ID:             1,
			TmdbID:         123,
			Title:          "Test Series",
			Status:         "Continuing",
			SeasonCount:    1,
			EpisodeCount:   5,
			ExternalIds:    nil,
			WatchProviders: nil,
		}

		details := &tmdb.SeriesDetailsResponse{
			PosterPath: "poster.jpg",
		}

		// Unmonitored series
		series := &storage.Series{
			Series: model.Series{
				ID:               1,
				SeriesMetadataID: ptr(int32(1)),
				QualityProfileID: 1,
				Monitored:        0, // Unmonitored
			},
			State: storage.SeriesStateMissing,
		}

		result := m.buildTVDetailResult(metadata, details, series, []SeasonResult{})

		monitored := false
		assert.Equal(t, &monitored, result.Monitored)
		assert.Equal(t, string(storage.SeriesStateMissing), result.LibraryStatus)
	})
}

func TestGetTVDetailByTMDBID(t *testing.T) {
	ctx := context.Background()

	t.Run("success - TV show not in library", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := mocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		m := New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		firstAirDate := time.Date(2023, 1, 15, 0, 0, 0, 0, time.UTC)
		externalIDsJSON := `{"imdb_id":"tt7654321","tvdb_id":54321}`
		watchProvidersJSON := `{"US":{"flatrate":[{"provider_id":8,"provider_name":"Netflix","logo_path":"/net.png"}]}}`
		metadata := &model.SeriesMetadata{
			ID:             1,
			TmdbID:         123,
			Title:          "Test Series",
			Overview:       ptr("Test series overview"),
			FirstAirDate:   &firstAirDate,
			SeasonCount:    2,
			EpisodeCount:   20,
			Status:         "Continuing",
			ExternalIds:    &externalIDsJSON,
			WatchProviders: &watchProvidersJSON,
		}

		// Mock series details response will be returned via HTTP response

		// Mock GetSeriesMetadata call
		store.EXPECT().GetSeriesMetadata(ctx, gomock.Any()).Return(metadata, nil)

		// Mock TvSeriesDetails call
		responseBody := `{"poster_path":"poster.jpg","backdrop_path":"backdrop.jpg","networks":[{"name":"HBO"}],"genres":[{"name":"Drama"}]}`
		resp := &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewBufferString(responseBody)),
		}
		tmdbMock.EXPECT().TvSeriesDetails(ctx, int32(123), nil).Return(resp, nil)

		// Mock GetSeries call - not found
		store.EXPECT().GetSeries(ctx, gomock.Any()).Return(nil, storage.ErrNotFound)

		result, err := m.GetTVDetailByTMDBID(ctx, 123)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.Equal(t, int32(123), result.TMDBID)
		assert.Equal(t, "Test Series", result.Title)
		assert.Equal(t, "Test series overview", result.Overview)
		assert.Equal(t, "poster.jpg", result.PosterPath)
		assert.Equal(t, ptr("backdrop.jpg"), result.BackdropPath)
		assert.Equal(t, ptr("2023-01-15"), result.FirstAirDate)
		assert.Equal(t, int32(2), result.SeasonCount)
		assert.Equal(t, int32(20), result.EpisodeCount)
		require.Len(t, result.Networks, 1)
		assert.Equal(t, "HBO", result.Networks[0].Name)
		assert.Equal(t, []string{"Drama"}, result.Genres)
		assert.Equal(t, "Not In Library", result.LibraryStatus)
		assert.Nil(t, result.Path)
		assert.Nil(t, result.QualityProfileID)
		assert.Nil(t, result.Monitored)
		// Check external IDs from stored data
		require.NotNil(t, result.ExternalIDs)
		assert.Equal(t, ptr("tt7654321"), result.ExternalIDs.ImdbID)
		tvdbID := 54321
		assert.Equal(t, &tvdbID, result.ExternalIDs.TvdbID)
		// Check watch providers from stored data
		require.Len(t, result.WatchProviders, 1)
		assert.Equal(t, 8, result.WatchProviders[0].ProviderID)
		assert.Equal(t, "Netflix", result.WatchProviders[0].Name)
		assert.Equal(t, ptr("/net.png"), result.WatchProviders[0].LogoPath)
	})

	t.Run("success - TV show in library", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := mocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		m := New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		emptyExternalIDsJSON := `{"imdb_id":null,"tvdb_id":null}`
		emptyWatchProvidersJSON := `{"US":{"flatrate":[]}}`
		metadata := &model.SeriesMetadata{
			ID:             1,
			TmdbID:         123,
			Title:          "Test Series",
			SeasonCount:    1,
			EpisodeCount:   10,
			Status:         "Continuing",
			ExternalIds:    &emptyExternalIDsJSON,
			WatchProviders: &emptyWatchProvidersJSON,
		}

		// Mock series details response will be returned via HTTP response

		path := "/tv/test-series"
		qualityProfileID := int32(2)
		series := &storage.Series{
			Series: model.Series{
				ID:               1,
				SeriesMetadataID: ptr(int32(1)),
				Path:             &path,
				QualityProfileID: qualityProfileID,
				Monitored:        1,
			},
			State: storage.SeriesStateDiscovered,
		}

		store.EXPECT().GetSeriesMetadata(ctx, gomock.Any()).Return(metadata, nil)

		responseBody := `{"poster_path":"poster.jpg","backdrop_path":"backdrop.jpg"}`
		resp := &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewBufferString(responseBody)),
		}
		tmdbMock.EXPECT().TvSeriesDetails(ctx, int32(123), nil).Return(resp, nil)

		store.EXPECT().GetSeries(ctx, gomock.Any()).Return(series, nil)

		// Mock calls for seasons and episodes (empty list since this test doesn't focus on seasons)
		store.EXPECT().ListSeasons(ctx, gomock.Any()).Return([]*storage.Season{}, nil)

		result, err := m.GetTVDetailByTMDBID(ctx, 123)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.Equal(t, int32(123), result.TMDBID)
		assert.Equal(t, "Test Series", result.Title)
		assert.Equal(t, string(storage.SeriesStateDiscovered), result.LibraryStatus)
		assert.Equal(t, &path, result.Path)
		assert.Equal(t, &qualityProfileID, result.QualityProfileID)
		monitored := true
		assert.Equal(t, &monitored, result.Monitored)
	})

	t.Run("error - GetSeriesMetadata fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := mocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		m := New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		expectedErr := errors.New("metadata fetch error")
		store.EXPECT().GetSeriesMetadata(ctx, gomock.Any()).Return(nil, expectedErr)

		result, err := m.GetTVDetailByTMDBID(ctx, 123)
		assert.Error(t, err)
		assert.Equal(t, expectedErr, err)
		assert.Nil(t, result)
	})

	t.Run("error - TMDB API call fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := mocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		m := New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		metadata := &model.SeriesMetadata{
			ID:             1,
			TmdbID:         123,
			Title:          "Test Series",
			Status:         "Continuing",
			ExternalIds:    nil,
			WatchProviders: nil,
		}

		store.EXPECT().GetSeriesMetadata(ctx, gomock.Any()).Return(metadata, nil)

		expectedErr := errors.New("TMDB API error")
		tmdbMock.EXPECT().TvSeriesDetails(ctx, int32(123), nil).Return(nil, expectedErr)

		result, err := m.GetTVDetailByTMDBID(ctx, 123)
		assert.Error(t, err)
		assert.Equal(t, expectedErr, err)
		assert.Nil(t, result)
	})

	t.Run("success - storage error non-NotFound is logged but not returned", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := mocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		m := New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		metadata := &model.SeriesMetadata{
			ID:             1,
			TmdbID:         123,
			Title:          "Test Series",
			Status:         "Continuing",
			ExternalIds:    nil,
			WatchProviders: nil,
		}

		// Mock series details response will be returned via HTTP response

		store.EXPECT().GetSeriesMetadata(ctx, gomock.Any()).Return(metadata, nil)

		responseBody := `{"poster_path":"poster.jpg"}`
		resp := &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewBufferString(responseBody)),
		}
		tmdbMock.EXPECT().TvSeriesDetails(ctx, int32(123), nil).Return(resp, nil)

		dbErr := errors.New("database connection error")
		store.EXPECT().GetSeries(ctx, gomock.Any()).Return(nil, dbErr)

		result, err := m.GetTVDetailByTMDBID(ctx, 123)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.Equal(t, int32(123), result.TMDBID)
		assert.Equal(t, "Not In Library", result.LibraryStatus)
		assert.Nil(t, result.Path)
		assert.Nil(t, result.QualityProfileID)
		assert.Nil(t, result.Monitored)
	})

	t.Run("error - TMDB returns empty FirstAirDate", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := mocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		m := New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		// This should trigger the creation of metadata which calls FromSeriesDetails with empty FirstAirDate
		store.EXPECT().GetSeriesMetadata(ctx, gomock.Any()).Return(nil, storage.ErrNotFound)

		// Mock GetSeriesDetails to return series with empty FirstAirDate
		tmdbMock.EXPECT().GetSeriesDetails(ctx, 123).Return(&tmdb.SeriesDetails{
			ID:               123,
			Name:             "Test Series",
			FirstAirDate:     "", // Empty date that causes the parsing error
			NumberOfSeasons:  1,
			NumberOfEpisodes: 10,
			Seasons:          []tmdb.Season{}, // Empty to avoid creating season metadata
		}, nil)

		// Mock external IDs and watch providers calls during metadata creation
		extIDsResp := &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewBufferString(`{"imdb_id":null,"tvdb_id":null}`))}
		tmdbMock.EXPECT().TvSeriesExternalIds(ctx, int32(123)).Return(extIDsResp, nil)
		wpResp := &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewBufferString(`{"results":{"US":{"flatrate":[]}}}`))}
		tmdbMock.EXPECT().TvSeriesWatchProviders(ctx, int32(123)).Return(wpResp, nil)

		// Mock CreateSeriesMetadata call
		store.EXPECT().CreateSeriesMetadata(ctx, gomock.Any()).Return(int64(1), nil)

		// Mock GetSeriesMetadata call after creation
		createdMetadata := &model.SeriesMetadata{
			ID:             1,
			TmdbID:         123,
			Title:          "Test Series",
			Status:         "Continuing",
			FirstAirDate:   nil, // Should be nil due to empty date
			SeasonCount:    1,
			EpisodeCount:   10,
			ExternalIds:    nil,
			WatchProviders: nil,
		}
		store.EXPECT().GetSeriesMetadata(ctx, gomock.Any()).Return(createdMetadata, nil)

		// Mock TvSeriesDetails call for the main flow
		responseBody := `{"poster_path":"poster.jpg"}`
		resp := &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewBufferString(responseBody)),
		}
		tmdbMock.EXPECT().TvSeriesDetails(ctx, int32(123), nil).Return(resp, nil)

		// Mock GetSeries call - not found
		store.EXPECT().GetSeries(ctx, gomock.Any()).Return(nil, storage.ErrNotFound)

		result, err := m.GetTVDetailByTMDBID(ctx, 123)
		require.NoError(t, err, "Should handle empty FirstAirDate gracefully")
		require.NotNil(t, result)

		assert.Equal(t, int32(123), result.TMDBID)
		assert.Equal(t, "Test Series", result.Title)
		assert.Nil(t, result.FirstAirDate) // Should be nil when date is empty
		assert.Equal(t, "Not In Library", result.LibraryStatus)
	})
}

func TestParseTMDBDate(t *testing.T) {
	t.Run("parses valid date", func(t *testing.T) {
		result, err := parseTMDBDate("2023-01-15")
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, 2023, result.Year())
		assert.Equal(t, time.January, result.Month())
		assert.Equal(t, 15, result.Day())
	})

	t.Run("handles empty string", func(t *testing.T) {
		result, err := parseTMDBDate("")
		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("returns error for invalid date", func(t *testing.T) {
		result, err := parseTMDBDate("invalid-date")
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestMediaManager_AddSeriesToLibrary(t *testing.T) {
	t.Run("error getting quality profile", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ctx := context.Background()

		store := mocks.NewMockStorage(ctrl)
		store.EXPECT().GetQualityProfile(gomock.Any(), int64(1)).Return(storage.QualityProfile{}, errors.New("expected testing error"))

		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		require.NotNil(t, m)

		req := AddSeriesRequest{
			TMDBID:           1234,
			QualityProfileID: 1,
		}

		_, err := m.AddSeriesToLibrary(ctx, req)
		assert.Error(t, err)
		assert.Equal(t, "expected testing error", err.Error())
	})

	t.Run("error getting series metadata from tdmb", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)

		store := newStore(t, ctx)
		store.CreateQualityProfile(ctx, model.QualityProfile{
			ID:   1,
			Name: "test-profile",
		})

		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)
		tmdbMock.EXPECT().GetSeriesDetails(gomock.Any(), gomock.Any()).Return(nil, errors.New("expected testing error"))

		m := New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})
		require.NotNil(t, m)

		req := AddSeriesRequest{
			TMDBID:           1234,
			QualityProfileID: 1,
		}

		_, err := m.AddSeriesToLibrary(ctx, req)
		assert.Error(t, err)
		assert.Equal(t, "expected testing error", err.Error())
	})

	t.Run("series metadata exists in db - series exists", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)

		store := newStore(t, ctx)
		_, err := store.CreateQualityProfile(ctx, model.QualityProfile{
			ID:   1,
			Name: "test-profile",
		})
		require.NoError(t, err)

		metadataID, err := store.CreateSeriesMetadata(ctx, model.SeriesMetadata{
			ID:             1,
			TmdbID:         1234,
			Title:          "Test Series",
			SeasonCount:    1,
			EpisodeCount:   1,
			Status:         "Continuing",
			ExternalIds:    nil, // Optional field
			WatchProviders: nil, // Optional field
		})
		require.NoError(t, err)

		seriesID, err := store.CreateSeries(ctx, storage.Series{
			Series: model.Series{
				ID:               1,
				SeriesMetadataID: ptr(int32(metadataID)),
				Monitored:        1,
				QualityProfileID: 1,
				Added:            ptr(time.Now()),
			},
		}, storage.SeriesStateMissing)
		require.NoError(t, err)

		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		m := New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})
		require.NotNil(t, m)

		req := AddSeriesRequest{
			TMDBID:           1234,
			QualityProfileID: 1,
		}

		series, err := m.AddSeriesToLibrary(ctx, req)
		require.NoError(t, err)

		assert.Equal(t, series.ID, int32(seriesID))
		assert.Equal(t, series.SeriesMetadataID, ptr(int32(metadataID)))
		assert.Equal(t, series.Monitored, int32(1))
		assert.Equal(t, series.QualityProfileID, int32(1))
		assert.Equal(t, storage.SeriesStateMissing, series.State)
	})

	t.Run("series metadata exists in db - series does not exist - unreleased", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)

		store := newStore(t, ctx)
		_, err := store.CreateQualityProfile(ctx, model.QualityProfile{
			ID:   1,
			Name: "test-profile",
		})
		require.NoError(t, err)

		metadataID, err := store.CreateSeriesMetadata(ctx, model.SeriesMetadata{
			ID:             1,
			TmdbID:         1234,
			Title:          "Test Series",
			SeasonCount:    1,
			EpisodeCount:   1,
			Status:         "Continuing",
			FirstAirDate:   ptr(time.Now().Add(time.Hour * 24 * 7)),
			ExternalIds:    nil, // Optional field
			WatchProviders: nil, // Optional field
		})
		require.NoError(t, err)

		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		m := New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})
		require.NotNil(t, m)

		req := AddSeriesRequest{
			TMDBID:           1234,
			QualityProfileID: 1,
		}

		series, err := m.AddSeriesToLibrary(ctx, req)
		require.NoError(t, err)

		assert.Equal(t, series.ID, int32(1))
		assert.Equal(t, series.SeriesMetadataID, ptr(int32(metadataID)))
		assert.Equal(t, series.Monitored, int32(1))
		assert.Equal(t, series.QualityProfileID, int32(1))
		assert.Equal(t, storage.SeriesStateUnreleased, series.State)
	})

	t.Run("series metadata exists in db - series does not exist - released", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)

		store := newStore(t, ctx)
		_, err := store.CreateQualityProfile(ctx, model.QualityProfile{
			ID:   1,
			Name: "test-profile",
		})
		require.NoError(t, err)

		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)
		details := &tmdb.SeriesDetails{
			ID:               1,
			Name:             "test series",
			FirstAirDate:     time.Now().Add(-time.Hour * 2).Format(tmdb.ReleaseDateFormat),
			NumberOfSeasons:  1,
			NumberOfEpisodes: 1,
			Seasons: []tmdb.Season{
				{
					ID:           1,
					Name:         "test season",
					SeasonNumber: 1,
					Episodes: []tmdb.Episode{
						{
							ID:            1,
							Name:          "test episode",
							EpisodeNumber: 1,
						},
					},
				},
			},
		}

		tmdbMock.EXPECT().GetSeriesDetails(gomock.Any(), gomock.Any()).Return(details, nil)

		// Mock external IDs and watch providers calls during metadata creation
		extIDsResp := &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewBufferString(`{"imdb_id":null,"tvdb_id":null}`))}
		tmdbMock.EXPECT().TvSeriesExternalIds(gomock.Any(), int32(1234)).Return(extIDsResp, nil)
		wpResp := &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewBufferString(`{"results":{"US":{"flatrate":[]}}}`))}
		tmdbMock.EXPECT().TvSeriesWatchProviders(gomock.Any(), int32(1234)).Return(wpResp, nil)

		m := New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})
		require.NotNil(t, m)

		req := AddSeriesRequest{
			TMDBID:           1234,
			QualityProfileID: 1,
		}

		series, err := m.AddSeriesToLibrary(ctx, req)
		require.NoError(t, err)

		assert.Equal(t, int32(1), series.ID)
		assert.Equal(t, ptr(int32(1)), series.SeriesMetadataID)
		assert.Equal(t, int32(1), series.Monitored)
		assert.Equal(t, int32(1), series.QualityProfileID)
		assert.Equal(t, storage.SeriesStateMissing, series.State)

		seasons, err := m.storage.ListSeasons(ctx, table.Season.SeriesID.EQ(sqlite.Int32(series.ID)))
		assert.Nil(t, err)
		require.Len(t, seasons, 1)
		assert.Equal(t, int32(series.ID), seasons[0].SeriesID)
		assert.Equal(t, seasons[0].State, storage.SeasonStateMissing)
		assert.Equal(t, int32(1), seasons[0].Monitored)

		episodes, err := m.storage.ListEpisodes(ctx, table.Episode.SeasonID.EQ(sqlite.Int32(seasons[0].ID)))
		assert.Nil(t, err)
		require.Len(t, episodes, 1)
		assert.Equal(t, int32(1), episodes[0].EpisodeNumber)
		assert.Equal(t, int32(1), episodes[0].Monitored)
		assert.Equal(t, storage.EpisodeStateMissing, episodes[0].State)
	})
}

func TestMediaManager_GetJob(t *testing.T) {
	t.Run("get job", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)
		scheduler := NewScheduler(store, config.Manager{}, make(map[JobType]JobExecutor))

		id, err := scheduler.createPendingJob(ctx, MovieIndex)
		require.Nil(t, err)
		require.NotEqual(t, id, int64(0))

		manager := MediaManager{
			storage:   store,
			scheduler: scheduler,
		}

		job, err := manager.GetJob(ctx, id)
		require.Nil(t, err)

		job.CreatedAt = time.Time{}
		job.UpdatedAt = time.Time{}

		assert.Equal(t, JobResponse{
			ID:    1,
			Type:  string(MovieIndex),
			State: string(storage.JobStatePending),
			Error: nil,
		}, job)
	})

	t.Run("job not found", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)
		scheduler := NewScheduler(store, config.Manager{}, make(map[JobType]JobExecutor))

		manager := MediaManager{
			storage:   store,
			scheduler: scheduler,
		}

		_, err := manager.GetJob(ctx, 999)
		require.Error(t, err)
		assert.Equal(t, storage.ErrNotFound, err)
	})
}

func TestMediaManager_CreateJob(t *testing.T) {
	t.Run("create job successfully", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)
		scheduler := NewScheduler(store, config.Manager{}, make(map[JobType]JobExecutor))

		manager := MediaManager{
			storage:   store,
			scheduler: scheduler,
		}

		req := TriggerJobRequest{Type: string(MovieIndex)}
		job, err := manager.CreateJob(ctx, req)
		require.NoError(t, err)

		assert.Equal(t, int64(1), job.ID)
		assert.Equal(t, string(MovieIndex), job.Type)
		assert.Equal(t, string(storage.JobStatePending), job.State)
		assert.Nil(t, job.Error)

		retrievedJob, err := manager.GetJob(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, job.ID, retrievedJob.ID)
		assert.Equal(t, job.Type, retrievedJob.Type)
		assert.Equal(t, job.State, retrievedJob.State)
	})

	t.Run("create job when pending already exists", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)
		scheduler := NewScheduler(store, config.Manager{}, make(map[JobType]JobExecutor))

		manager := MediaManager{
			storage:   store,
			scheduler: scheduler,
		}

		req := TriggerJobRequest{Type: string(MovieReconcile)}
		job1, err := manager.CreateJob(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, int64(1), job1.ID)

		job2, err := manager.CreateJob(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, job1.ID, job2.ID)
		assert.Equal(t, string(MovieReconcile), job2.Type)

		listResult, err := manager.ListJobs(ctx, nil, nil, pagination.Params{Page: 1, PageSize: 0})
		require.NoError(t, err)
		assert.Equal(t, 1, listResult.Count)
		assert.Equal(t, job1.ID, listResult.Jobs[0].ID)
	})

	t.Run("create multiple different job types", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)
		scheduler := NewScheduler(store, config.Manager{}, make(map[JobType]JobExecutor))

		manager := MediaManager{
			storage:   store,
			scheduler: scheduler,
		}

		movieIndexReq := TriggerJobRequest{Type: string(MovieIndex)}
		seriesIndexReq := TriggerJobRequest{Type: string(SeriesIndex)}

		movieJob, err := manager.CreateJob(ctx, movieIndexReq)
		require.NoError(t, err)
		assert.Equal(t, string(MovieIndex), movieJob.Type)

		seriesJob, err := manager.CreateJob(ctx, seriesIndexReq)
		require.NoError(t, err)
		assert.Equal(t, string(SeriesIndex), seriesJob.Type)
		assert.NotEqual(t, movieJob.ID, seriesJob.ID)

		listResult, err := manager.ListJobs(ctx, nil, nil, pagination.Params{Page: 1, PageSize: 0})
		require.NoError(t, err)
		assert.Equal(t, 2, listResult.Count)

		movieJobFound := false
		seriesJobFound := false
		for _, job := range listResult.Jobs {
			if job.ID == movieJob.ID {
				movieJobFound = true
				assert.Equal(t, string(MovieIndex), job.Type)
			}
			if job.ID == seriesJob.ID {
				seriesJobFound = true
				assert.Equal(t, string(SeriesIndex), job.Type)
			}
		}
		assert.True(t, movieJobFound, "movie job should be in list")
		assert.True(t, seriesJobFound, "series job should be in list")
	})
}

func TestMediaManager_ListJobs(t *testing.T) {
	t.Run("list all jobs", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)
		scheduler := NewScheduler(store, config.Manager{}, make(map[JobType]JobExecutor))

		manager := MediaManager{
			storage:   store,
			scheduler: scheduler,
		}

		_, err := manager.CreateJob(ctx, TriggerJobRequest{Type: string(MovieIndex)})
		require.NoError(t, err)
		_, err = manager.CreateJob(ctx, TriggerJobRequest{Type: string(SeriesIndex)})
		require.NoError(t, err)

		result, err := manager.ListJobs(ctx, nil, nil, pagination.Params{Page: 1, PageSize: 0})
		require.NoError(t, err)
		assert.Equal(t, 2, result.Count)
		assert.Len(t, result.Jobs, 2)
	})

	t.Run("list jobs filtered by type", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)
		scheduler := NewScheduler(store, config.Manager{}, make(map[JobType]JobExecutor))

		manager := MediaManager{
			storage:   store,
			scheduler: scheduler,
		}

		_, err := manager.CreateJob(ctx, TriggerJobRequest{Type: string(MovieIndex)})
		require.NoError(t, err)
		_, err = manager.CreateJob(ctx, TriggerJobRequest{Type: string(SeriesIndex)})
		require.NoError(t, err)
		_, err = manager.CreateJob(ctx, TriggerJobRequest{Type: string(MovieReconcile)})
		require.NoError(t, err)

		jobType := string(MovieIndex)
		result, err := manager.ListJobs(ctx, &jobType, nil, pagination.Params{Page: 1, PageSize: 0})
		require.NoError(t, err)
		assert.Equal(t, 1, result.Count)
		assert.Len(t, result.Jobs, 1)
		assert.Equal(t, string(MovieIndex), result.Jobs[0].Type)
	})

	t.Run("list jobs filtered by state", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)
		scheduler := NewScheduler(store, config.Manager{}, make(map[JobType]JobExecutor))

		manager := MediaManager{
			storage:   store,
			scheduler: scheduler,
		}

		id1, err := manager.CreateJob(ctx, TriggerJobRequest{Type: string(MovieIndex)})
		require.NoError(t, err)
		_, err = manager.CreateJob(ctx, TriggerJobRequest{Type: string(SeriesIndex)})
		require.NoError(t, err)

		err = store.UpdateJobState(ctx, id1.ID, storage.JobStateRunning, nil)
		require.NoError(t, err)
		err = store.UpdateJobState(ctx, id1.ID, storage.JobStateDone, nil)
		require.NoError(t, err)

		state := string(storage.JobStatePending)
		result, err := manager.ListJobs(ctx, nil, &state, pagination.Params{Page: 1, PageSize: 0})
		require.NoError(t, err)
		assert.Equal(t, 1, result.Count)
		assert.Len(t, result.Jobs, 1)
		assert.Equal(t, string(storage.JobStatePending), result.Jobs[0].State)
	})

	t.Run("list jobs filtered by type and state", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)
		scheduler := NewScheduler(store, config.Manager{}, make(map[JobType]JobExecutor))

		manager := MediaManager{
			storage:   store,
			scheduler: scheduler,
		}

		id1, err := manager.CreateJob(ctx, TriggerJobRequest{Type: string(MovieIndex)})
		require.NoError(t, err)
		_, err = manager.CreateJob(ctx, TriggerJobRequest{Type: string(SeriesIndex)})
		require.NoError(t, err)
		_, err = manager.CreateJob(ctx, TriggerJobRequest{Type: string(MovieReconcile)})
		require.NoError(t, err)

		err = store.UpdateJobState(ctx, id1.ID, storage.JobStateRunning, nil)
		require.NoError(t, err)
		err = store.UpdateJobState(ctx, id1.ID, storage.JobStateDone, nil)
		require.NoError(t, err)

		jobType := string(MovieIndex)
		state := string(storage.JobStateDone)
		result, err := manager.ListJobs(ctx, &jobType, &state, pagination.Params{Page: 1, PageSize: 0})
		require.NoError(t, err)
		assert.Equal(t, 1, result.Count)
		assert.Len(t, result.Jobs, 1)
		assert.Equal(t, string(MovieIndex), result.Jobs[0].Type)
		assert.Equal(t, string(storage.JobStateDone), result.Jobs[0].State)
	})

	t.Run("list jobs with no results", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)
		scheduler := NewScheduler(store, config.Manager{}, make(map[JobType]JobExecutor))

		manager := MediaManager{
			storage:   store,
			scheduler: scheduler,
		}

		result, err := manager.ListJobs(ctx, nil, nil, pagination.Params{Page: 1, PageSize: 0})
		require.NoError(t, err)
		assert.Equal(t, 0, result.Count)
		assert.Empty(t, result.Jobs)
	})

	t.Run("invalid job type", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)
		scheduler := NewScheduler(store, config.Manager{}, make(map[JobType]JobExecutor))

		manager := MediaManager{
			storage:   store,
			scheduler: scheduler,
		}

		invalidType := "InvalidJobType"
		_, err := manager.ListJobs(ctx, &invalidType, nil, pagination.Params{Page: 1, PageSize: 0})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid job type")
	})

	t.Run("invalid job state", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)
		scheduler := NewScheduler(store, config.Manager{}, make(map[JobType]JobExecutor))

		manager := MediaManager{
			storage:   store,
			scheduler: scheduler,
		}

		invalidState := "InvalidState"
		_, err := manager.ListJobs(ctx, nil, &invalidState, pagination.Params{Page: 1, PageSize: 0})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid job state")
	})
}

func TestMediaManager_CancelJob(t *testing.T) {
	t.Run("cancel running job", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)

		jobExecuted := false
		jobCancelled := false
		executors := map[JobType]JobExecutor{
			MovieIndex: func(ctx context.Context, jobID int64) error {
				jobExecuted = true
				<-ctx.Done()
				jobCancelled = true
				return ctx.Err()
			},
		}
		scheduler := NewScheduler(store, config.Manager{}, executors)

		manager := MediaManager{
			storage:   store,
			scheduler: scheduler,
		}

		job, err := manager.CreateJob(ctx, TriggerJobRequest{Type: string(MovieIndex)})
		require.NoError(t, err)

		go scheduler.executeJob(ctx, &storage.Job{
			Job: model.Job{
				ID:   int32(job.ID),
				Type: job.Type,
			},
			State: storage.JobStatePending,
		})

		time.Sleep(100 * time.Millisecond)
		require.True(t, jobExecuted, "job should have started executing")

		result, err := manager.CancelJob(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, job.ID, result.ID)

		time.Sleep(200 * time.Millisecond)
		require.True(t, jobCancelled, "job should have been cancelled")

		cancelledJob, err := store.GetJob(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, storage.JobStateCancelled, cancelledJob.State)

		retrievedJob, err := manager.GetJob(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, string(storage.JobStateCancelled), retrievedJob.State)
	})

	t.Run("cancel pending job", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)
		scheduler := NewScheduler(store, config.Manager{}, make(map[JobType]JobExecutor))

		manager := MediaManager{
			storage:   store,
			scheduler: scheduler,
		}

		job, err := manager.CreateJob(ctx, TriggerJobRequest{Type: string(MovieIndex)})
		require.NoError(t, err)

		result, err := manager.CancelJob(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, job.ID, result.ID)

		cancelledJob, err := store.GetJob(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, storage.JobStateCancelled, cancelledJob.State)

		retrievedJob, err := manager.GetJob(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, string(storage.JobStateCancelled), retrievedJob.State)
	})

	t.Run("cancel completed job", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)
		scheduler := NewScheduler(store, config.Manager{}, make(map[JobType]JobExecutor))

		manager := MediaManager{
			storage:   store,
			scheduler: scheduler,
		}

		job, err := manager.CreateJob(ctx, TriggerJobRequest{Type: string(MovieIndex)})
		require.NoError(t, err)

		err = store.UpdateJobState(ctx, job.ID, storage.JobStateRunning, nil)
		require.NoError(t, err)
		err = store.UpdateJobState(ctx, job.ID, storage.JobStateDone, nil)
		require.NoError(t, err)

		result, err := manager.CancelJob(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, job.ID, result.ID)

		doneJob, err := store.GetJob(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, storage.JobStateDone, doneJob.State)

		retrievedJob, err := manager.GetJob(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, string(storage.JobStateDone), retrievedJob.State)
	})

	t.Run("cancel non-existent job", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)
		scheduler := NewScheduler(store, config.Manager{}, make(map[JobType]JobExecutor))

		manager := MediaManager{
			storage:   store,
			scheduler: scheduler,
		}

		_, err := manager.CancelJob(ctx, 999)
		require.Error(t, err)
		assert.Equal(t, storage.ErrNotFound, err)
	})
}
