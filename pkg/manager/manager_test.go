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
	"time"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/library"
	mockLibrary "github.com/kasuboski/mediaz/pkg/library/mocks"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/ptr"
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
			Overview:       ptr.To("Test series overview"),
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
				SeriesMetadataID: ptr.To(int32(1)),
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
		assert.Equal(t, ptr.To("backdrop.jpg"), result.BackdropPath)
		assert.Equal(t, ptr.To("2023-01-15"), result.FirstAirDate)
		assert.Equal(t, ptr.To("2023-12-15"), result.LastAirDate)
		assert.Equal(t, int32(2), result.SeasonCount)
		assert.Equal(t, int32(20), result.EpisodeCount)
		require.Len(t, result.Networks, 2)
		assert.Equal(t, "HBO", result.Networks[0].Name)
		assert.Equal(t, "Netflix", result.Networks[1].Name)
		assert.Equal(t, []string{"Drama", "Thriller"}, result.Genres)
		assert.Equal(t, ptr.To(true), result.Adult)
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
				SeriesMetadataID: ptr.To(int32(1)),
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
		store := storageMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		m := New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		firstAirDate := time.Date(2023, 1, 15, 0, 0, 0, 0, time.UTC)
		externalIDsJSON := `{"imdb_id":"tt7654321","tvdb_id":54321}`
		watchProvidersJSON := `{"US":{"flatrate":[{"provider_id":8,"provider_name":"Netflix","logo_path":"/net.png"}]}}`
		metadata := &model.SeriesMetadata{
			ID:             1,
			TmdbID:         123,
			Title:          "Test Series",
			Overview:       ptr.To("Test series overview"),
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
		assert.Equal(t, ptr.To("backdrop.jpg"), result.BackdropPath)
		assert.Equal(t, ptr.To("2023-01-15"), result.FirstAirDate)
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
		assert.Equal(t, ptr.To("tt7654321"), result.ExternalIDs.ImdbID)
		tvdbID := 54321
		assert.Equal(t, &tvdbID, result.ExternalIDs.TvdbID)
		// Check watch providers from stored data
		require.Len(t, result.WatchProviders, 1)
		assert.Equal(t, 8, result.WatchProviders[0].ProviderID)
		assert.Equal(t, "Netflix", result.WatchProviders[0].Name)
		assert.Equal(t, ptr.To("/net.png"), result.WatchProviders[0].LogoPath)
	})

	t.Run("success - TV show in library", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := storageMocks.NewMockStorage(ctrl)
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
				SeriesMetadataID: ptr.To(int32(1)),
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
		store := storageMocks.NewMockStorage(ctrl)
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
		store := storageMocks.NewMockStorage(ctrl)
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
		store := storageMocks.NewMockStorage(ctrl)
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
		store := storageMocks.NewMockStorage(ctrl)
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

		store := storageMocks.NewMockStorage(ctrl)
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
				SeriesMetadataID: ptr.To(int32(metadataID)),
				Monitored:        1,
				QualityProfileID: 1,
				Added:            ptr.To(time.Now()),
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
		assert.Equal(t, series.SeriesMetadataID, ptr.To(int32(metadataID)))
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
			FirstAirDate:   ptr.To(time.Now().Add(time.Hour * 24 * 7)),
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
		assert.Equal(t, series.SeriesMetadataID, ptr.To(int32(metadataID)))
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
		assert.Equal(t, ptr.To(int32(1)), series.SeriesMetadataID)
		assert.Equal(t, int32(1), series.Monitored)
		assert.Equal(t, int32(1), series.QualityProfileID)
		assert.Equal(t, storage.SeriesStateMissing, series.State)

		seasons, err := m.seriesStorage.ListSeasons(ctx, table.Season.SeriesID.EQ(sqlite.Int32(series.ID)))
		assert.Nil(t, err)
		require.Len(t, seasons, 1)
		assert.Equal(t, int32(series.ID), seasons[0].SeriesID)
		assert.Equal(t, seasons[0].State, storage.SeasonStateMissing)
		assert.Equal(t, int32(1), seasons[0].Monitored)

		episodes, err := m.seriesStorage.ListEpisodes(ctx, table.Episode.SeasonID.EQ(sqlite.Int32(seasons[0].ID)))
		assert.Nil(t, err)
		require.Len(t, episodes, 1)
		assert.Equal(t, int32(1), episodes[0].EpisodeNumber)
		assert.Equal(t, int32(1), episodes[0].Monitored)
		assert.Equal(t, storage.EpisodeStateMissing, episodes[0].State)
	})
}
