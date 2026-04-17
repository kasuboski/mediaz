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
	"github.com/kasuboski/mediaz/config"
	downloadMock "github.com/kasuboski/mediaz/pkg/download/mocks"
	mhttpMock "github.com/kasuboski/mediaz/pkg/http/mocks"
	"github.com/kasuboski/mediaz/pkg/indexer/mocks"
	mio "github.com/kasuboski/mediaz/pkg/io"
	"github.com/kasuboski/mediaz/pkg/library"
	mockLibrary "github.com/kasuboski/mediaz/pkg/library/mocks"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/ptr"
	"github.com/kasuboski/mediaz/pkg/storage"
	storageMocks "github.com/kasuboski/mediaz/pkg/storage/mocks"
	mediaSqlite "github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/tmdb"
	"github.com/oapi-codegen/nullable"
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
		true,
	)

	tmdbHttpMock := mhttpMock.NewMockHTTPClient(ctrl)
	tmdbHttpMock.EXPECT().Do(gomock.Any()).Return(mediaDetailsResponse("test movie", 120, releaseDate), nil).Times(1)

	tClient, err := tmdb.New("https://api.themoviedb.org", "1234", tmdb.WithHTTPClient(tmdbHttpMock))
	require.NoError(t, err)

	indexerFactory := mocks.NewMockFactory(ctrl)
	mockFactory := downloadMock.NewMockFactory(ctrl)

	m := New(tClient, indexerFactory, lib, store, mockFactory, config.Manager{}, config.Config{})
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

	movie, err := m.movieStorage.GetMovie(ctx, int64(mov.ID))
	require.Nil(t, err)
	movie.Added = nil
	movieMetadataID := int32(1)

	assert.Equal(t, &storage.Movie{
		Movie: model.Movie{
			ID:               1,
			Monitored:        1,
			QualityProfileID: 1,
			MovieMetadataID:  &movieMetadataID,
			Path:             ptr.To("test movie"),
		},
		State: storage.MovieStateMissing,
	}, movie)
}

func TestListMoviesInLibrary(t *testing.T) {
	ctx := context.Background()

	t.Run("no movies in library", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := storageMocks.NewMockStorage(ctrl)
		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})

		store.EXPECT().ListMovies(ctx).Return([]*storage.Movie{}, nil)

		movies, err := m.ListMoviesInLibrary(ctx)
		require.NoError(t, err)
		assert.Empty(t, movies)
	})

	t.Run("movies with metadata", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := storageMocks.NewMockStorage(ctrl)
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

	t.Run("movies without metadata are filtered out", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := storageMocks.NewMockStorage(ctrl)
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
		assert.Empty(t, movies, "movies without metadata should not be included in library")
	})

	t.Run("error listing movies", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := storageMocks.NewMockStorage(ctrl)
		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})

		store.EXPECT().ListMovies(ctx).Return(nil, errors.New("db error"))

		movies, err := m.ListMoviesInLibrary(ctx)
		assert.Error(t, err)
		assert.Nil(t, movies)
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
			RelativePath:     ptr.To("Movie 1/movie1.mp4"),
			OriginalFilePath: ptr.To("/movies/Movie 1/movie1.mp4"),
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
			RelativePath:     ptr.To("Movie 1/movie1.mp4"),
			OriginalFilePath: ptr.To("/movies/Movie 1/movie1.mp4"),
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

	t.Run("skips files tracked with different casing", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ctx := context.Background()

		store := newStore(t, ctx)
		mockLibrary := mockLibrary.NewMockLibrary(ctrl)

		_, err := store.CreateMovieFile(ctx, model.MovieFile{
			Size:             1024,
			RelativePath:     ptr.To("Movie 1/MOVIE1.MP4"),
			OriginalFilePath: ptr.To("/movies/Movie 1/MOVIE1.MP4"),
		})
		require.NoError(t, err)

		discoveredFiles := []library.MovieFile{
			{RelativePath: "movie 1/movie1.mp4", AbsolutePath: "/movies/movie 1/movie1.mp4"},
		}

		mockLibrary.EXPECT().FindMovies(ctx).Times(1).Return(discoveredFiles, nil)

		m := New(nil, nil, mockLibrary, store, nil, config.Manager{}, config.Config{})
		require.NotNil(t, m)

		err = m.IndexMovieLibrary(ctx)
		assert.NoError(t, err)

		// Should not create a new movie file since it's already tracked (case-insensitive)
		movieFiles, err := store.ListMovieFiles(ctx)
		require.NoError(t, err)
		assert.Len(t, movieFiles, 1)
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
		params := ReleaseFilterParams{
			Title:   det.Title,
			Runtime: det.Runtime,
		}
		rejectFunc := RejectMovieReleaseFunc(ctx, params, profile, protocols)

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
		params := ReleaseFilterParams{
			Title:   det.Title,
			Runtime: det.Runtime,
		}
		rejectFunc := RejectMovieReleaseFunc(ctx, params, profile, protocolsAvailable)

		// Test case where the release protocol is not available
		r2 := &prowlarr.ReleaseResource{Protocol: ptr.To(prowlarr.DownloadProtocolUsenet), Size: ptr.To(int64(500))}
		if !rejectFunc(r2) {
			t.Errorf("Expected rejection for protocol 'usenet' since it's unavailable")
		}
	})

	t.Run("year matching", func(t *testing.T) {
		ctx := context.Background()
		profile := storage.QualityProfile{
			Name: "test",
			Qualities: []storage.QualityDefinition{{
				MinSize:       0,
				MaxSize:       10000,
				PreferredSize: 1999,
			}},
		}
		protocols := map[string]struct{}{"torrent": {}}
		year2024 := int32(2024)
		params := ReleaseFilterParams{
			Title:   "Brothers",
			Year:    &year2024,
			Runtime: 60,
		}
		rejectFunc := RejectMovieReleaseFunc(ctx, params, profile, protocols)

		tests := []struct {
			title       string
			shouldMatch bool
		}{
			{"Brothers 2024 1080p AMZN WEB DLip ExKinoRay", true},
			{"Brothers.2024.1080p.AMZN.WEBRip.1400MB.DD5.1.x264-GalaxyRG", true},
			{"Brothers (2009) 720p BrRip x264 - 700MB -YIFY", false},
			{"Step Brothers 2008 UNRATED 1080p BluRay HEVC x265 5.1 BONE", false},
		}

		for _, tt := range tests {
			r := &prowlarr.ReleaseResource{
				Title:    nullable.NewNullableWithValue(tt.title),
				Protocol: ptr.To(prowlarr.DownloadProtocolTorrent),
				Size:     ptr.To(int64(500 << 20)),
			}
			got := rejectFunc(r)
			if got != !tt.shouldMatch {
				t.Errorf("(%s): expected match=%v, got rejected=%v", tt.title, tt.shouldMatch, got)
			}
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
		store := storageMocks.NewMockStorage(ctrl)
		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})

		releaseDate := time.Date(2023, 1, 15, 0, 0, 0, 0, time.UTC)
		year := int32(2023)
		metadata := &model.MovieMetadata{
			ID:               1,
			TmdbID:           123,
			Title:            "Test Movie",
			OriginalTitle:    ptr.To("Original Test Movie"),
			Overview:         ptr.To("Test movie overview"),
			Images:           "poster.jpg",
			Runtime:          120,
			Genres:           ptr.To("Action, Drama"),
			Studio:           ptr.To("Test Studio"),
			Website:          ptr.To("https://test.com"),
			CollectionTmdbID: ptr.To(int32(456)),
			CollectionTitle:  ptr.To("Test Collection"),
			Popularity:       ptr.To(8.5),
			Year:             &year,
			ReleaseDate:      &releaseDate,
		}

		store.EXPECT().GetMovieMetadata(ctx, gomock.Any()).Return(metadata, nil)
		store.EXPECT().GetMovieByMetadataID(ctx, 1).Return(nil, storage.ErrNotFound)

		result, err := m.GetMovieDetailByTMDBID(ctx, 123)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.Equal(t, int32(123), result.TMDBID)
		assert.Equal(t, ptr.To("Original Test Movie"), result.OriginalTitle)
		assert.Equal(t, "Test Movie", result.Title)
		assert.Equal(t, ptr.To("Test movie overview"), result.Overview)
		assert.Equal(t, "poster.jpg", result.PosterPath)
		assert.Equal(t, ptr.To(int32(120)), result.Runtime)
		assert.Equal(t, ptr.To("Action, Drama"), result.Genres)
		assert.Equal(t, ptr.To("Test Studio"), result.Studio)
		assert.Equal(t, ptr.To("https://test.com"), result.Website)
		assert.Equal(t, ptr.To(int32(456)), result.CollectionTmdbID)
		assert.Equal(t, ptr.To("Test Collection"), result.CollectionTitle)
		assert.Equal(t, ptr.To(8.5), result.Popularity)
		assert.Equal(t, &year, result.Year)
		assert.Equal(t, ptr.To("2023-01-15"), result.ReleaseDate)
		assert.Equal(t, "Not In Library", result.LibraryStatus)
		assert.Nil(t, result.Path)
		assert.Nil(t, result.QualityProfileID)
		assert.Nil(t, result.Monitored)
	})

	t.Run("success - movie in library", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := storageMocks.NewMockStorage(ctrl)
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
				MovieMetadataID:  ptr.To(int32(1)),
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
		store := storageMocks.NewMockStorage(ctrl)
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
		store := storageMocks.NewMockStorage(ctrl)
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
				MovieMetadataID:  ptr.To(int32(1)),
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
		store := storageMocks.NewMockStorage(ctrl)
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
		store := storageMocks.NewMockStorage(ctrl)
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

func TestUpdateMovieQualityProfile(t *testing.T) {
	t.Run("successfully updates quality profile", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store, err := mediaSqlite.New(context.Background(), ":memory:")
		require.NoError(t, err)

		schemas, err := storage.ReadSchemaFiles("../storage/sqlite/schema/schema.sql", "../storage/sqlite/schema/defaults.sql")
		require.NoError(t, err)

		ctx := context.Background()
		err = store.Init(ctx, schemas...)
		require.NoError(t, err)

		releaseDate := time.Now().AddDate(0, 0, -1).Format(tmdb.ReleaseDateFormat)
		tmdbHttpMock := mhttpMock.NewMockHTTPClient(ctrl)
		tmdbHttpMock.EXPECT().Do(gomock.Any()).Return(mediaDetailsResponse("test movie", 120, releaseDate), nil).Times(1)

		tClient, err := tmdb.New("https://api.themoviedb.org", "1234", tmdb.WithHTTPClient(tmdbHttpMock))
		require.NoError(t, err)

		mockFactory := downloadMock.NewMockFactory(ctrl)
		m := New(tClient, nil, nil, store, mockFactory, config.Manager{}, config.Config{})

		req := AddMovieRequest{
			TMDBID:           1234,
			QualityProfileID: 1,
		}
		movie, err := m.AddMovieToLibrary(ctx, req)
		require.NoError(t, err)

		result, err := m.UpdateMovieQualityProfile(ctx, int64(movie.ID), 3)
		require.NoError(t, err)
		assert.Equal(t, int32(3), result.QualityProfileID)
	})

	t.Run("returns error for non-existent movie", func(t *testing.T) {
		store, err := mediaSqlite.New(context.Background(), ":memory:")
		require.NoError(t, err)

		schemas, err := storage.ReadSchemaFiles("../storage/sqlite/schema/schema.sql", "../storage/sqlite/schema/defaults.sql")
		require.NoError(t, err)

		ctx := context.Background()
		err = store.Init(ctx, schemas...)
		require.NoError(t, err)

		m := MediaManager{movieStorage: store}
		_, err = m.UpdateMovieQualityProfile(ctx, 999, 3)
		require.Error(t, err)
		assert.ErrorContains(t, err, "rows")
	})
}
