package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/manager"
	"github.com/kasuboski/mediaz/pkg/storage"
	storeMocks "github.com/kasuboski/mediaz/pkg/storage/mocks"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	tmdbMocks "github.com/kasuboski/mediaz/pkg/tmdb/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestServer_GetTVDetailByTMDBID(t *testing.T) {
	t.Run("success - TV show exists in library", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// Mock storage
		store := storeMocks.NewMockStorage(ctrl)

		// Mock TMDB client
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		// Setup expectations for GetSeriesMetadata call
		externalIDsJSON := `{"imdb_id":"tt1234567","tvdb_id":12345}`
		watchProvidersJSON := `{"US":{"flatrate":[{"provider_id":8,"provider_name":"Netflix","logo_path":"/net.png"}]}}`
		expectedMetadata := sqlcdb.SeriesMetadatum{
			ID:             1,
			TmdbID:         12345,
			Title:          "Test TV Show",
			Overview:       sql.NullString{String: "A test TV show overview", Valid: true},
			FirstAirDate:   sql.NullTime{Time: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true},
			LastAirDate:    sql.NullTime{Time: time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC), Valid: true},
			SeasonCount:    3,
			EpisodeCount:   30,
			Status:         "Continuing",
			ExternalIds:    sql.NullString{String: externalIDsJSON, Valid: true},
			WatchProviders: sql.NullString{String: watchProvidersJSON, Valid: true},
		}

		store.EXPECT().GetSeriesMetadataByTmdbID(gomock.Any(), gomock.Any()).Return(expectedMetadata, nil)

		// Setup expectations for TvSeriesDetails call
		responseBody := `{
			"poster_path": "/test-poster.jpg",
			"backdrop_path": "/test-backdrop.jpg",
			"adult": true,
			"popularity": 85.5,
			"networks": [{"name": "HBO"}, {"name": "Netflix"}],
			"genres": [{"name": "Drama"}, {"name": "Thriller"}]
		}`
		resp := &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(responseBody)),
		}
		tmdbMock.EXPECT().TvSeriesDetails(gomock.Any(), int32(12345), nil).Return(resp, nil)

		// Setup expectations for GetSeries call
		expectedSeries := sqlcdb.GetSeriesRow{
			Series: sqlcdb.Series{
				ID:               1,
				SeriesMetadataID: sql.NullInt64{Int64: 1, Valid: true},
				QualityProfileID: 1,
				Monitored:        1,
				Path:             sql.NullString{String: "/tv/Test TV Show (2023)", Valid: true},
			},
			State: string(storage.SeriesStateDiscovered),
		}

		store.EXPECT().GetSeriesByMetadataID(gomock.Any(), gomock.Any()).Return(expectedSeries, nil)

		// Setup expectations for seasons and episodes data
		seasonMetadataID := int64(10)
		season := sqlcdb.GetSeasonByIDRow{
			State: sql.NullString{String: string(storage.SeasonStateMissing), Valid: true},
			Season: sqlcdb.Season{
				ID:               1,
				SeriesID:         1,
				SeasonMetadataID: sql.NullInt64{Int64: seasonMetadataID, Valid: true},
				Monitored:        1,
			},
		}
		seasonMetadata := sqlcdb.SeasonMetadatum{
			ID:     10,
			Number: 1,
			TmdbID: 67890,
			Title:  "Season 1",
		}
		episodeMetadataID := int64(200)
		episode := sqlcdb.GetEpisodeByIDRow{
			Episode: sqlcdb.Episode{
				ID:                100,
				SeasonID:          1,
				EpisodeMetadataID: sql.NullInt64{Int64: episodeMetadataID, Valid: true},
				Monitored:         1,
				EpisodeNumber:     1,
			},
			State: sql.NullString{String: string(storage.EpisodeStateDownloaded), Valid: true},
		}

		store.EXPECT().ListSeasons(gomock.Any(), gomock.Any()).Return([]sqlcdb.ListSeasonsRow{{Season: season.Season, State: season.State.String}}, nil)
		store.EXPECT().ListSeasonMetadataByIDs(gomock.Any(), []int64{seasonMetadataID}).Return([]sqlcdb.SeasonMetadatum{seasonMetadata}, nil)
		store.EXPECT().ListEpisodesBySeasonIDs(gomock.Any(), []int64{season.Season.ID}).Return([]sqlcdb.ListEpisodesBySeasonIDsRow{{Episode: episode.Episode, State: episode.State.String}}, nil)
		episodeMetadata := sqlcdb.EpisodeMetadatum{
			ID:     200,
			Number: 1,
			TmdbID: 54321,
			Title:  "Episode 1",
		}
		store.EXPECT().ListEpisodeMetadataByIDs(gomock.Any(), []int64{episodeMetadataID}).Return([]sqlcdb.EpisodeMetadatum{episodeMetadata}, nil)

		// Create manager with mocked dependencies
		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := newTestServer(withManager(mgr))

		req, err := http.NewRequest("GET", "/tv/12345", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/tv/{tmdbID}", s.GetTVDetailByTMDBID()).Methods("GET")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("content-type"))

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		tvDetail, ok := response.Response.(map[string]any)
		require.True(t, ok, "Response should be a map")

		assert.Equal(t, float64(12345), tvDetail["tmdbID"])
		assert.Equal(t, "Test TV Show", tvDetail["title"])
		assert.Equal(t, "A test TV show overview", tvDetail["overview"])
		assert.Equal(t, "/test-poster.jpg", tvDetail["posterPath"])
		assert.Equal(t, "/test-backdrop.jpg", tvDetail["backdropPath"])
		assert.Equal(t, "2023-01-01", tvDetail["firstAirDate"])
		assert.Equal(t, "2023-12-31", tvDetail["lastAirDate"])
		assert.Equal(t, float64(3), tvDetail["seasonCount"])
		assert.Equal(t, float64(30), tvDetail["episodeCount"])

		// Check networks array (objects with name/logoPath)
		networks, ok := tvDetail["networks"].([]any)
		require.True(t, ok)
		require.Len(t, networks, 2)
		first := networks[0].(map[string]any)
		second := networks[1].(map[string]any)
		assert.Equal(t, "HBO", first["name"])
		assert.Equal(t, "Netflix", second["name"])

		// Check genres array
		genres, ok := tvDetail["genres"].([]any)
		require.True(t, ok)
		assert.Equal(t, []any{"Drama", "Thriller"}, genres)

		assert.Equal(t, "discovered", tvDetail["libraryStatus"])
		assert.Equal(t, "/tv/Test TV Show (2023)", tvDetail["path"])
		assert.Equal(t, true, tvDetail["monitored"])

		// Check seasons array
		seasons, ok := tvDetail["seasons"].([]any)
		require.True(t, ok, "Response should contain seasons array")
		require.Len(t, seasons, 1)

		season0 := seasons[0].(map[string]any)
		assert.Equal(t, float64(67890), season0["tmdbID"])
		assert.Equal(t, float64(1), season0["seriesID"])
		assert.Equal(t, float64(1), season0["seasonNumber"])
		assert.Equal(t, "Season 1", season0["title"])
		assert.Equal(t, float64(1), season0["episodeCount"])
		assert.Equal(t, true, season0["monitored"])

		// Check episodes within season
		episodes, ok := season0["episodes"].([]any)
		require.True(t, ok, "Season should contain episodes array")
		require.Len(t, episodes, 1)

		episode0 := episodes[0].(map[string]any)
		assert.Equal(t, float64(54321), episode0["tmdbID"])
		assert.Equal(t, float64(1), episode0["seriesID"])
		assert.Equal(t, float64(1), episode0["seasonNumber"])
		assert.Equal(t, float64(1), episode0["episodeNumber"])
		assert.Equal(t, "Episode 1", episode0["title"])
		assert.Equal(t, true, episode0["monitored"])
		assert.Equal(t, true, episode0["downloaded"])
	})

	t.Run("success - TV show not in library", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		emptyExternalIDsJSON := `{"imdb_id":null,"tvdb_id":null}`
		emptyWatchProvidersJSON := `{"US":{"flatrate":[]}}`
		expectedMetadata := sqlcdb.SeriesMetadatum{
			ID:             1,
			TmdbID:         12345,
			Title:          "Test TV Show",
			SeasonCount:    2,
			EpisodeCount:   20,
			Status:         "Continuing",
			ExternalIds:    sql.NullString{String: emptyExternalIDsJSON, Valid: true},
			WatchProviders: sql.NullString{String: emptyWatchProvidersJSON, Valid: true},
		}

		store.EXPECT().GetSeriesMetadataByTmdbID(gomock.Any(), gomock.Any()).Return(expectedMetadata, nil)

		responseBody := `{"poster_path": "/test-poster.jpg"}`
		resp := &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(responseBody)),
		}
		tmdbMock.EXPECT().TvSeriesDetails(gomock.Any(), int32(12345), nil).Return(resp, nil)

		store.EXPECT().GetSeriesByMetadataID(gomock.Any(), gomock.Any()).Return(sqlcdb.GetSeriesRow{}, storage.ErrNotFound)

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := newTestServer(withManager(mgr))

		req, err := http.NewRequest("GET", "/tv/12345", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/tv/{tmdbID}", s.GetTVDetailByTMDBID()).Methods("GET")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		tvDetail, ok := response.Response.(map[string]any)
		require.True(t, ok, "Response should be a map")

		assert.Equal(t, "Not In Library", tvDetail["libraryStatus"])
		assert.Nil(t, tvDetail["path"])
		assert.Nil(t, tvDetail["monitored"])
	})

	t.Run("invalid tmdb id format", func(t *testing.T) {
		s := newTestServer()

		req, err := http.NewRequest("GET", "/tv/invalid", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/tv/{tmdbID}", s.GetTVDetailByTMDBID()).Methods("GET")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid tmdbID")
	})

	t.Run("manager error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		store.EXPECT().GetSeriesMetadataByTmdbID(gomock.Any(), gomock.Any()).Return(sqlcdb.SeriesMetadatum{}, errors.New("metadata not found"))

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := newTestServer(withManager(mgr))

		req, err := http.NewRequest("GET", "/tv/12345", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/tv/{tmdbID}", s.GetTVDetailByTMDBID()).Methods("GET")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("content-type"))

		responseBody := rr.Body.String()
		assert.Contains(t, responseBody, "error")
		assert.Contains(t, responseBody, "null") // response should be null when there's an error
	})
}
