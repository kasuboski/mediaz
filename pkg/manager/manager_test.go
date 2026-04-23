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

func TestListShowsInLibrary(t *testing.T) {
	ctx := context.Background()

	t.Run("no shows in library", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := storageMocks.NewMockStorage(ctrl)
		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})

		store.EXPECT().ListSeries(ctx).Return([]*storage.Series{}, nil)

		shows, err := m.ListShowsInLibrary(ctx)
		require.NoError(t, err)
		assert.Empty(t, shows)
	})

	t.Run("shows with metadata", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := storageMocks.NewMockStorage(ctrl)
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
			PosterPath:     ptr.To("poster.jpg"),
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

	t.Run("shows without metadata are filtered out", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := storageMocks.NewMockStorage(ctrl)
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
		assert.Empty(t, shows, "series without metadata should not be included in library")
	})

	t.Run("error listing series", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := storageMocks.NewMockStorage(ctrl)
		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})

		store.EXPECT().ListSeries(ctx).Return(nil, errors.New("db error"))

		shows, err := m.ListShowsInLibrary(ctx)
		assert.Error(t, err)
		assert.Nil(t, shows)
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
