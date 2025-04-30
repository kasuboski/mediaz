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

	store, err := mediaSqlite.New(":memory:")
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

	m := New(tClient, pClient, lib, store, mockFactory, config.Manager{})
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
		m := New(nil, nil, nil, store, nil, config.Manager{})

		store.EXPECT().ListMoviesByState(ctx, storage.MovieStateDiscovered).Return([]*storage.Movie{}, nil)
		store.EXPECT().ListMoviesByState(ctx, storage.MovieStateDownloaded).Return([]*storage.Movie{}, nil)

		movies, err := m.ListMoviesInLibrary(ctx)
		require.NoError(t, err)
		assert.Empty(t, movies)
	})

	t.Run("movies with metadata", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := mocks.NewMockStorage(ctrl)
		m := New(nil, nil, nil, store, nil, config.Manager{})

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
			ID:      1,
			TmdbID:  123,
			Title:   "Test Movie",
			Images:  "poster.jpg",
			Year:    &year,
		}

		store.EXPECT().ListMoviesByState(ctx, storage.MovieStateDiscovered).Return([]*storage.Movie{discoveredMovie}, nil)
		store.EXPECT().ListMoviesByState(ctx, storage.MovieStateDownloaded).Return([]*storage.Movie{downloadedMovie}, nil)
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
		m := New(nil, nil, nil, store, nil, config.Manager{})

		path := "movie1"
		discoveredMovie := &storage.Movie{
			Movie: model.Movie{
				Path: &path,
			},
			State: storage.MovieStateDiscovered,
		}

		store.EXPECT().ListMoviesByState(ctx, storage.MovieStateDiscovered).Return([]*storage.Movie{discoveredMovie}, nil)
		store.EXPECT().ListMoviesByState(ctx, storage.MovieStateDownloaded).Return([]*storage.Movie{}, nil)

		movies, err := m.ListMoviesInLibrary(ctx)
		require.NoError(t, err)
		assert.Len(t, movies, 1)

		expectedMovie := LibraryMovie{
			Path:  path,
			State: string(storage.MovieStateDiscovered),
		}
		assert.Equal(t, expectedMovie, movies[0])
	})

	t.Run("error listing discovered movies", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := mocks.NewMockStorage(ctrl)
		m := New(nil, nil, nil, store, nil, config.Manager{})

		store.EXPECT().ListMoviesByState(ctx, storage.MovieStateDiscovered).Return(nil, errors.New("db error"))

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
		m := New(nil, nil, library, nil, nil, config.Manager{})
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
		m := New(nil, nil, mockLibrary, nil, nil, config.Manager{})
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

		m := New(nil, nil, mockLibrary, store, nil, config.Manager{})
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

		m := New(nil, nil, mockLibrary, store, nil, config.Manager{})
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

		m := New(nil, nil, mockLibrary, store, nil, config.Manager{})
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

		m := New(nil, nil, mockLibrary, store, nil, config.Manager{})
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

func TestRun(t *testing.T) {
	ctrl := gomock.NewController(t)
	prowlarrMock := prowlMock.NewMockClientInterface(ctrl)
	store, err := mediaSqlite.New(":memory:")
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

	require.NoError(t, err)

	mockFactory := downloadMock.NewMockFactory(ctrl)
	m := New(nil, pClient, lib, store, mockFactory, config.Manager{
		Jobs: config.Jobs{
			MovieReconcile: time.Minute * 1,
			MovieIndex:     time.Minute * 1,
		},
	})
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
				Name:          "WEBDL-1080p",
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
	store, err := mediaSqlite.New(":memory:")
	require.NoError(t, err)

	schemas, err := storage.ReadSchemaFiles("../storage/sqlite/schema/schema.sql", "../storage/sqlite/schema/defaults.sql")
	require.NoError(t, err)

	err = store.Init(ctx, schemas...)
	require.NoError(t, err)

	return store
}

func TestMediaManager_AddSeriesToLibrary(t *testing.T) {
	t.Run("error getting quality profile", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ctx := context.Background()

		store := mocks.NewMockStorage(ctrl)
		store.EXPECT().GetQualityProfile(gomock.Any(), int64(1)).Return(storage.QualityProfile{}, errors.New("expected testing error"))

		m := New(nil, nil, nil, store, nil, config.Manager{})
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

		m := New(tmdbMock, nil, nil, store, nil, config.Manager{})
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
			ID:     1,
			TmdbID: 1234,
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

		m := New(tmdbMock, nil, nil, store, nil, config.Manager{})
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
			ID:           1,
			TmdbID:       1234,
			FirstAirDate: ptr(time.Now().Add(time.Hour * 24 * 7)),
		})
		require.NoError(t, err)

		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		m := New(tmdbMock, nil, nil, store, nil, config.Manager{})
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

		m := New(tmdbMock, nil, nil, store, nil, config.Manager{})
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
