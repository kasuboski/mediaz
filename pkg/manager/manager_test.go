package manager

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/library"
	mockLibrary "github.com/kasuboski/mediaz/pkg/library/mocks"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/ptr"
	"github.com/kasuboski/mediaz/pkg/storage"
	storageMocks "github.com/kasuboski/mediaz/pkg/storage/mocks"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	mediaSqlite "github.com/kasuboski/mediaz/pkg/storage/sqlite"
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

		assert.Equal(t, "Movie 1/movie1.mp4", movieFiles[0].RelativePath.String)
		assert.Equal(t, int64(1024), movieFiles[0].Size)
		assert.Equal(t, "Movie 2/movie2.mkv", movieFiles[1].RelativePath.String)
		assert.Equal(t, int64(2048), movieFiles[1].Size)

		assert.Equal(t, int64(1), movies[0].Movie.MovieFileID.Int64)
		assert.Equal(t, "Movie 1", movies[0].Movie.Path.String)
		assert.Equal(t, int64(2), movies[1].Movie.MovieFileID.Int64)
		assert.Equal(t, "Movie 2", movies[1].Movie.Path.String)
	})

	t.Run("create movie for already tracked file", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ctx := context.Background()

		store := newStore(t, ctx)
		mockLibrary := mockLibrary.NewMockLibrary(ctrl)

		_, err := store.CreateMovieFile(ctx, sqlcdb.MovieFile{
			Size:             1024,
			RelativePath:     sql.NullString{String: "Movie 1/movie1.mp4", Valid: true},
			OriginalFilePath: sql.NullString{String: "/movies/Movie 1/movie1.mp4", Valid: true},
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

		assert.Equal(t, int64(1), movies[0].Movie.MovieFileID.Int64)
		assert.Equal(t, "Movie 1", movies[0].Movie.Path.String)
	})

	t.Run("link new movie file to existing movie", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ctx := context.Background()

		store := newStore(t, ctx)
		mockLibrary := mockLibrary.NewMockLibrary(ctrl)

		_, err := store.CreateMovieFile(ctx, sqlcdb.MovieFile{
			Size:             1024,
			RelativePath:     sql.NullString{String: "Movie 1/movie1.mp4", Valid: true},
			OriginalFilePath: sql.NullString{String: "/movies/Movie 1/movie1.mp4", Valid: true},
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

		assert.Equal(t, int64(1), movies[0].Movie.MovieFileID.Int64)
		assert.Equal(t, "Movie 1", movies[0].Movie.Path.String)

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

		_, err := store.CreateMovieFile(ctx, sqlcdb.MovieFile{
			Size:             1024,
			RelativePath:     sql.NullString{String: "Movie 1/MOVIE1.MP4", Valid: true},
			OriginalFilePath: sql.NullString{String: "/movies/Movie 1/MOVIE1.MP4", Valid: true},
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

	t.Run("skips discovered files with absolute relative path", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ctx := context.Background()

		store := newStore(t, ctx)
		mockLibrary := mockLibrary.NewMockLibrary(ctrl)

		discoveredFiles := []library.MovieFile{
			{RelativePath: "/absolute/path/Movie 1/movie1.mp4", AbsolutePath: "/absolute/path/Movie 1/movie1.mp4", Size: 1024},
			{RelativePath: "Movie 2/movie2.mkv", AbsolutePath: "/movies/Movie 2/movie2.mkv", Size: 2048},
		}

		mockLibrary.EXPECT().FindMovies(ctx).Times(1).Return(discoveredFiles, nil)

		m := New(nil, nil, mockLibrary, store, nil, config.Manager{}, config.Config{})
		require.NotNil(t, m)

		err := m.IndexMovieLibrary(ctx)
		assert.NoError(t, err)

		movieFiles, err := store.ListMovieFiles(ctx)
		require.NoError(t, err)
		require.Len(t, movieFiles, 1)

		expected := sqlcdb.MovieFile{
			ID:               movieFiles[0].ID,
			DateAdded:        movieFiles[0].DateAdded,
			RelativePath:     sql.NullString{String: "Movie 2/movie2.mkv", Valid: true},
			OriginalFilePath: sql.NullString{String: "Movie 2/movie2.mkv", Valid: true},
			Size:             2048,
		}
		assert.Equal(t, expected, movieFiles[0])
	})
}

func TestListShowsInLibrary(t *testing.T) {
	ctx := context.Background()

	t.Run("no shows in library", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := storageMocks.NewMockStorage(ctrl)
		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})

		store.EXPECT().ListSeries(ctx, gomock.Any()).Return([]sqlcdb.ListSeriesRow{}, nil)

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
		series := sqlcdb.ListSeriesRow{
			Series: sqlcdb.Series{
				SeriesMetadataID: sql.NullInt64{Int64: int64(metadataID), Valid: true},
				Path:             sql.NullString{String: path, Valid: true},
			},
			State: string(storage.SeriesStateDiscovered),
		}

		seriesMetadata := sqlcdb.SeriesMetadatum{
			ID:             1,
			TmdbID:         321,
			Title:          "Test Series",
			Status:         "Continuing",
			PosterPath:     sql.NullString{String: "poster.jpg", Valid: true},
			ExternalIds:    sql.NullString{},
			WatchProviders: sql.NullString{},
		}

		store.EXPECT().ListSeries(ctx, gomock.Any()).Return([]sqlcdb.ListSeriesRow{series}, nil)
		store.EXPECT().GetSeriesMetadataByID(ctx, gomock.Any()).Return(seriesMetadata, nil)

		shows, err := m.ListShowsInLibrary(ctx)
		require.NoError(t, err)
		require.Len(t, shows, 1)

		expected := LibraryShow{
			Path:       path,
			TMDBID:     seriesMetadata.TmdbID,
			Title:      seriesMetadata.Title,
			PosterPath: seriesMetadata.PosterPath.String,
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
		series := sqlcdb.ListSeriesRow{
			Series: sqlcdb.Series{
				Path: sql.NullString{String: path, Valid: true},
			},
			State: string(storage.SeriesStateDiscovered),
		}

		store.EXPECT().ListSeries(ctx, gomock.Any()).Return([]sqlcdb.ListSeriesRow{series}, nil)

		shows, err := m.ListShowsInLibrary(ctx)
		require.NoError(t, err)
		assert.Empty(t, shows, "series without metadata should not be included in library")
	})

	t.Run("error listing series", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := storageMocks.NewMockStorage(ctrl)
		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})

		store.EXPECT().ListSeries(ctx, gomock.Any()).Return(nil, errors.New("db error"))

		shows, err := m.ListShowsInLibrary(ctx)
		assert.Error(t, err)
		assert.Nil(t, shows)
	})
}

func TestMovieRejectRelease(t *testing.T) {
	t.Run("prefix match only", func(t *testing.T) {
		ctx := context.Background()
		det := &sqlcdb.MovieMetadatum{Title: "Brothers", Runtime: 60}
		profile := []sqlcdb.ListQualityProfileRowsRow{{QualityDefinition: sqlcdb.QualityDefinition{
			Name:          "WEBDL-1080p",
			MinSize:       17.0,
			MaxSize:       2000,
			PreferredSize: 1999,
		}}}
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
		det := &sqlcdb.MovieMetadatum{Title: "Interstellar"}
		profile := []sqlcdb.ListQualityProfileRowsRow{{QualityDefinition: sqlcdb.QualityDefinition{
			MinSize:       0,
			MaxSize:       1000,
			PreferredSize: 1999,
		}}}
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
		profile := []sqlcdb.ListQualityProfileRowsRow{{QualityDefinition: sqlcdb.QualityDefinition{
			MinSize:       0,
			MaxSize:       10000,
			PreferredSize: 1999,
		}}}
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

	return store
}

func TestMediaManager_AddSeriesToLibrary(t *testing.T) {
	t.Run("error getting quality profile", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ctx := context.Background()

		store := storageMocks.NewMockStorage(ctrl)
		store.EXPECT().GetQualityProfile(gomock.Any(), int64(1)).Return(sqlcdb.QualityProfile{}, errors.New("expected testing error"))

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
		store.CreateQualityProfile(ctx, sqlcdb.QualityProfile{
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
		_, err := store.CreateQualityProfile(ctx, sqlcdb.QualityProfile{
			ID:   1,
			Name: "test-profile",
		})
		require.NoError(t, err)

		metadataID, err := store.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{
			ID:             1,
			TmdbID:         1234,
			Title:          "Test Series",
			SeasonCount:    1,
			EpisodeCount:   1,
			Status:         "Continuing",
			ExternalIds:    sql.NullString{}, // Optional field
			WatchProviders: sql.NullString{}, // Optional field
		})
		require.NoError(t, err)

		seriesID, err := store.CreateSeries(ctx, sqlcdb.Series{
			ID:               1,
			SeriesMetadataID: sql.NullInt64{Int64: metadataID, Valid: true},
			Monitored:        1,
			QualityProfileID: 1,
			Added:            sql.NullTime{Time: time.Now(), Valid: true},
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

		assert.Equal(t, series.Series.ID, int64(seriesID))
		assert.Equal(t, sql.NullInt64{Int64: metadataID, Valid: true}, series.Series.SeriesMetadataID)
		assert.Equal(t, series.Series.Monitored, int64(1))
		assert.Equal(t, series.Series.QualityProfileID, int64(1))
		assert.Equal(t, string(storage.SeriesStateMissing), series.State)
	})

	t.Run("series metadata exists in db - series does not exist - unreleased", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)

		store := newStore(t, ctx)
		_, err := store.CreateQualityProfile(ctx, sqlcdb.QualityProfile{
			ID:   1,
			Name: "test-profile",
		})
		require.NoError(t, err)

		metadataID, err := store.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{
			ID:             1,
			TmdbID:         1234,
			Title:          "Test Series",
			SeasonCount:    1,
			EpisodeCount:   1,
			Status:         "Continuing",
			FirstAirDate:   sql.NullTime{Time: time.Now().Add(time.Hour * 24 * 7), Valid: true},
			ExternalIds:    sql.NullString{}, // Optional field
			WatchProviders: sql.NullString{}, // Optional field
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

		assert.Equal(t, series.Series.ID, int64(1))
		assert.Equal(t, sql.NullInt64{Int64: metadataID, Valid: true}, series.Series.SeriesMetadataID)
		assert.Equal(t, series.Series.Monitored, int64(1))
		assert.Equal(t, series.Series.QualityProfileID, int64(1))
		assert.Equal(t, string(storage.SeriesStateUnreleased), series.State)
	})

	t.Run("series metadata exists in db - series does not exist - released", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)

		store := newStore(t, ctx)
		_, err := store.CreateQualityProfile(ctx, sqlcdb.QualityProfile{
			ID:   1,
			Name: "test-profile",
		})
		require.NoError(t, err)

		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)
		details := &tmdb.SeriesDetails{
			ID:               1,
			Name:             "test series",
			FirstAirDate:     time.Now().UTC().AddDate(0, 0, -1).Format(tmdb.ReleaseDateFormat),
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

		assert.Equal(t, int64(1), series.Series.ID)
		assert.Equal(t, sql.NullInt64{Int64: 1, Valid: true}, series.Series.SeriesMetadataID)
		assert.Equal(t, int64(1), series.Series.Monitored)
		assert.Equal(t, int64(1), series.Series.QualityProfileID)
		assert.Equal(t, string(storage.SeriesStateMissing), series.State)

		seasons, err := m.seriesStorage.ListSeasons(ctx, storage.SeasonFilter{SeriesID: ptr.To(int64(series.Series.ID))})
		assert.Nil(t, err)
		require.Len(t, seasons, 1)
		assert.Equal(t, int64(series.Series.ID), seasons[0].Season.SeriesID)
		assert.Equal(t, string(storage.SeasonStateMissing), seasons[0].State)
		assert.Equal(t, int64(1), seasons[0].Season.Monitored)

		episodes, err := m.seriesStorage.ListEpisodes(ctx, storage.EpisodeFilter{SeasonID: ptr.To(int64(seasons[0].Season.ID))})
		assert.Nil(t, err)
		require.Len(t, episodes, 1)
		assert.Equal(t, int64(1), episodes[0].Episode.EpisodeNumber)
		assert.Equal(t, int64(1), episodes[0].Episode.Monitored)
		assert.Equal(t, string(storage.EpisodeStateMissing), episodes[0].State)
	})
}
