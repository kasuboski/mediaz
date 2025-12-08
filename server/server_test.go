package server

import (
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
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	tmdbMocks "github.com/kasuboski/mediaz/pkg/tmdb/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func TestServer_Healthz(t *testing.T) {
	t.Run("healthz", func(t *testing.T) {
		s := Server{baseLogger: zap.NewNop().Sugar()}

		req, err := http.NewRequest("GET", "/healthz", nil)
		assert.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.Healthz()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		assert.Equal(t, "application/json", rr.Header().Get("content-type"))

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)

		assert.NoError(t, err)
		assert.Equal(t, "ok", response.Response)
	})
}

func TestServer_GetMovieDetailByTMDBID(t *testing.T) {
	t.Run("success - movie exists in library", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// Mock storage
		store := storeMocks.NewMockStorage(ctrl)

		// Mock TMDB client
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		// Setup expectations for GetMovieMetadata call
		expectedMetadata := &model.MovieMetadata{
			ID:               1,
			TmdbID:           12345,
			Title:            "Test Movie",
			OriginalTitle:    ptr("Original Test Movie"),
			Overview:         ptr("A test movie overview"),
			Images:           "/test-poster.jpg",
			Runtime:          120,
			Year:             ptr(int32(2023)),
			ReleaseDate:      ptr(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)),
			Genres:           ptr("Action,Adventure"),
			Studio:           ptr("Test Studio"),
			Website:          ptr("https://testmovie.com"),
			Popularity:       ptr(85.5),
			CollectionTmdbID: ptr(int32(5000)),
			CollectionTitle:  ptr("Test Collection"),
		}

		store.EXPECT().GetMovieMetadata(gomock.Any(), gomock.Any()).Return(expectedMetadata, nil)

		// Setup expectations for GetMovieByMetadataID call
		expectedMovie := &storage.Movie{
			Movie: model.Movie{
				ID:               1,
				MovieMetadataID:  ptr(int32(1)),
				QualityProfileID: 1,
				Monitored:        1,
				Path:             ptr("/movies/Test Movie (2023)"),
			},
			State: storage.MovieStateDownloaded,
		}

		store.EXPECT().GetMovieByMetadataID(gomock.Any(), 1).Return(expectedMovie, nil)

		// Create manager with mocked dependencies
		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := Server{
			baseLogger: zap.NewNop().Sugar(),
			manager:    mgr,
		}

		req, err := http.NewRequest("GET", "/movie/12345", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/movie/{tmdbID}", s.GetMovieDetailByTMDBID()).Methods("GET")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("content-type"))

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		movieDetail, ok := response.Response.(map[string]any)
		require.True(t, ok, "Response should be a map")

		assert.Equal(t, float64(12345), movieDetail["tmdbID"])
		assert.Equal(t, "Test Movie", movieDetail["title"])
		assert.Equal(t, "Original Test Movie", movieDetail["originalTitle"])
		assert.Equal(t, "A test movie overview", movieDetail["overview"])
		assert.Equal(t, "/test-poster.jpg", movieDetail["posterPath"])
		assert.Equal(t, "2023-01-01", movieDetail["releaseDate"])
		assert.Equal(t, float64(2023), movieDetail["year"])
		assert.Equal(t, float64(120), movieDetail["runtime"])
		assert.Equal(t, "downloaded", movieDetail["libraryStatus"])
		assert.Equal(t, "/movies/Test Movie (2023)", movieDetail["path"])
		assert.Equal(t, true, movieDetail["monitored"])
	})

	t.Run("success - movie not in library", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		expectedMetadata := &model.MovieMetadata{
			ID:      1,
			TmdbID:  12345,
			Title:   "Test Movie",
			Images:  "/test-poster.jpg",
			Runtime: 120,
		}

		store.EXPECT().GetMovieMetadata(gomock.Any(), gomock.Any()).Return(expectedMetadata, nil)
		store.EXPECT().GetMovieByMetadataID(gomock.Any(), 1).Return(nil, storage.ErrNotFound)

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := Server{
			baseLogger: zap.NewNop().Sugar(),
			manager:    mgr,
		}

		req, err := http.NewRequest("GET", "/movie/12345", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/movie/{tmdbID}", s.GetMovieDetailByTMDBID()).Methods("GET")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		movieDetail, ok := response.Response.(map[string]any)
		require.True(t, ok, "Response should be a map")

		assert.Equal(t, "Not In Library", movieDetail["libraryStatus"])
		assert.Nil(t, movieDetail["path"])
		assert.Nil(t, movieDetail["monitored"])
	})

	t.Run("invalid tmdb id format", func(t *testing.T) {
		s := Server{baseLogger: zap.NewNop().Sugar()}

		req, err := http.NewRequest("GET", "/movie/invalid", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/movie/{tmdbID}", s.GetMovieDetailByTMDBID()).Methods("GET")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Invalid TMDB ID format")
	})

	t.Run("manager error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		store.EXPECT().GetMovieMetadata(gomock.Any(), gomock.Any()).Return(nil, errors.New("metadata not found"))

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := Server{
			baseLogger: zap.NewNop().Sugar(),
			manager:    mgr,
		}

		req, err := http.NewRequest("GET", "/movie/12345", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/movie/{tmdbID}", s.GetMovieDetailByTMDBID()).Methods("GET")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("content-type"))

		// For error responses, we just check that it's a proper error response
		responseBody := rr.Body.String()
		assert.Contains(t, responseBody, "error")
		assert.Contains(t, responseBody, "null") // response should be null when there's an error
	})
}

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
		expectedMetadata := &model.SeriesMetadata{
			ID:             1,
			TmdbID:         12345,
			Title:          "Test TV Show",
			Overview:       ptr("A test TV show overview"),
			FirstAirDate:   ptr(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)),
			LastAirDate:    ptr(time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC)),
			SeasonCount:    3,
			EpisodeCount:   30,
			Status:         "Continuing",
			ExternalIds:    &externalIDsJSON,
			WatchProviders: &watchProvidersJSON,
		}

		store.EXPECT().GetSeriesMetadata(gomock.Any(), gomock.Any()).Return(expectedMetadata, nil)

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
		expectedSeries := &storage.Series{
			Series: model.Series{
				ID:               1,
				SeriesMetadataID: ptr(int32(1)),
				QualityProfileID: 1,
				Monitored:        1,
				Path:             ptr("/tv/Test TV Show (2023)"),
			},
			State: storage.SeriesStateDiscovered,
		}

		store.EXPECT().GetSeries(gomock.Any(), gomock.Any()).Return(expectedSeries, nil)

		// Setup expectations for seasons and episodes data
		seasonMetadataID := int32(10)
		season := &storage.Season{
			Season: model.Season{
				ID:               1,
				SeriesID:         1,
				SeasonMetadataID: &seasonMetadataID,
				Monitored:        1,
			},
		}
		seasonMetadata := &model.SeasonMetadata{
			ID:     10,
			Number: 1,
			TmdbID: 67890,
			Title:  "Season 1",
		}
		episodeMetadataID := int32(200)
		episode := &storage.Episode{
			Episode: model.Episode{
				ID:                100,
				SeasonID:          1,
				EpisodeMetadataID: &episodeMetadataID,
				Monitored:         1,
				EpisodeNumber:     1,
			},
			State: storage.EpisodeStateDownloaded,
		}

		store.EXPECT().ListSeasons(gomock.Any(), gomock.Any()).Return([]*storage.Season{season}, nil)
		store.EXPECT().GetSeasonMetadata(gomock.Any(), gomock.Any()).Return(seasonMetadata, nil)
		store.EXPECT().ListEpisodes(gomock.Any(), gomock.Any()).Return([]*storage.Episode{episode}, nil)
		episodeMetadata := &model.EpisodeMetadata{
			ID:     200,
			Number: 1,
			TmdbID: 54321,
			Title:  "Episode 1",
		}
		store.EXPECT().GetEpisodeMetadata(gomock.Any(), gomock.Any()).Return(episodeMetadata, nil)

		// Create manager with mocked dependencies
		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := Server{
			baseLogger: zap.NewNop().Sugar(),
			manager:    mgr,
		}

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
		expectedMetadata := &model.SeriesMetadata{
			ID:             1,
			TmdbID:         12345,
			Title:          "Test TV Show",
			SeasonCount:    2,
			EpisodeCount:   20,
			Status:         "Continuing",
			ExternalIds:    &emptyExternalIDsJSON,
			WatchProviders: &emptyWatchProvidersJSON,
		}

		store.EXPECT().GetSeriesMetadata(gomock.Any(), gomock.Any()).Return(expectedMetadata, nil)

		responseBody := `{"poster_path": "/test-poster.jpg"}`
		resp := &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(responseBody)),
		}
		tmdbMock.EXPECT().TvSeriesDetails(gomock.Any(), int32(12345), nil).Return(resp, nil)

		store.EXPECT().GetSeries(gomock.Any(), gomock.Any()).Return(nil, storage.ErrNotFound)

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := Server{
			baseLogger: zap.NewNop().Sugar(),
			manager:    mgr,
		}

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
		s := Server{baseLogger: zap.NewNop().Sugar()}

		req, err := http.NewRequest("GET", "/tv/invalid", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/tv/{tmdbID}", s.GetTVDetailByTMDBID()).Methods("GET")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Invalid TMDB ID format")
	})

	t.Run("manager error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		store.EXPECT().GetSeriesMetadata(gomock.Any(), gomock.Any()).Return(nil, errors.New("metadata not found"))

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := Server{
			baseLogger: zap.NewNop().Sugar(),
			manager:    mgr,
		}

		req, err := http.NewRequest("GET", "/tv/12345", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/tv/{tmdbID}", s.GetTVDetailByTMDBID()).Methods("GET")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("content-type"))

		// For error responses, we just check that it's a proper error response
		responseBody := rr.Body.String()
		assert.Contains(t, responseBody, "error")
		assert.Contains(t, responseBody, "null") // response should be null when there's an error
	})
}

func TestServer_ListJobs(t *testing.T) {
	t.Run("success - list all jobs", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		// Setup test jobs
		createdAt := time.Now().Add(-1 * time.Hour)
		updatedAt := time.Now()
		errorMsg := "test error"
		jobs := []*storage.Job{
			{
				Job: model.Job{
					ID:        1,
					Type:      "MovieIndex",
					CreatedAt: &createdAt,
				},
				State:     storage.JobStateDone,
				UpdatedAt: &updatedAt,
				Error:     nil,
			},
			{
				Job: model.Job{
					ID:        2,
					Type:      "SeriesReconcile",
					CreatedAt: &createdAt,
				},
				State:     storage.JobStateError,
				UpdatedAt: &updatedAt,
				Error:     &errorMsg,
			},
		}

		store.EXPECT().ListJobs(gomock.Any(), gomock.Any()).Return(jobs, nil)

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := Server{
			baseLogger: zap.NewNop().Sugar(),
			manager:    mgr,
		}

		req, err := http.NewRequest("GET", "/jobs", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.ListJobs()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("content-type"))

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		jobList, ok := response.Response.(map[string]any)
		require.True(t, ok, "Response should be a map")

		jobsArray, ok := jobList["jobs"].([]any)
		require.True(t, ok, "jobs should be an array")
		assert.Len(t, jobsArray, 2)

		assert.Equal(t, float64(2), jobList["count"])

		// Check first job
		job1 := jobsArray[0].(map[string]any)
		assert.Equal(t, float64(1), job1["id"])
		assert.Equal(t, "MovieIndex", job1["type"])
		assert.Equal(t, "done", job1["state"])

		// Check second job with error
		job2 := jobsArray[1].(map[string]any)
		assert.Equal(t, float64(2), job2["id"])
		assert.Equal(t, "SeriesReconcile", job2["type"])
		assert.Equal(t, "error", job2["state"])
		assert.Equal(t, "test error", job2["error"])
	})

	t.Run("success - empty job list", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		store.EXPECT().ListJobs(gomock.Any(), gomock.Any()).Return([]*storage.Job{}, nil)

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := Server{
			baseLogger: zap.NewNop().Sugar(),
			manager:    mgr,
		}

		req, err := http.NewRequest("GET", "/jobs", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.ListJobs()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		jobList, ok := response.Response.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, float64(0), jobList["count"])
	})

	t.Run("error - storage error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		store.EXPECT().ListJobs(gomock.Any(), gomock.Any()).Return(nil, errors.New("storage error"))

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := Server{
			baseLogger: zap.NewNop().Sugar(),
			manager:    mgr,
		}

		req, err := http.NewRequest("GET", "/jobs", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.ListJobs()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("content-type"))
	})
}

func TestServer_GetJob(t *testing.T) {
	t.Run("success - get job by id", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		createdAt := time.Now().Add(-1 * time.Hour)
		updatedAt := time.Now()
		job := &storage.Job{
			Job: model.Job{
				ID:        42,
				Type:      "MovieIndex",
				CreatedAt: &createdAt,
			},
			State:     storage.JobStateRunning,
			UpdatedAt: &updatedAt,
			Error:     nil,
		}

		store.EXPECT().GetJob(gomock.Any(), int64(42)).Return(job, nil)

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := Server{
			baseLogger: zap.NewNop().Sugar(),
			manager:    mgr,
		}

		req, err := http.NewRequest("GET", "/jobs/42", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/jobs/{id}", s.GetJob()).Methods("GET")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("content-type"))

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		jobResp, ok := response.Response.(map[string]any)
		require.True(t, ok, "Response should be a map")

		assert.Equal(t, float64(42), jobResp["id"])
		assert.Equal(t, "MovieIndex", jobResp["type"])
		assert.Equal(t, "running", jobResp["state"])
		assert.Nil(t, jobResp["error"])
	})

	t.Run("invalid job id format", func(t *testing.T) {
		s := Server{baseLogger: zap.NewNop().Sugar()}

		req, err := http.NewRequest("GET", "/jobs/invalid", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/jobs/{id}", s.GetJob()).Methods("GET")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Invalid ID format")
	})

	t.Run("error - job not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		store.EXPECT().GetJob(gomock.Any(), int64(999)).Return(nil, storage.ErrNotFound)

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := Server{
			baseLogger: zap.NewNop().Sugar(),
			manager:    mgr,
		}

		req, err := http.NewRequest("GET", "/jobs/999", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/jobs/{id}", s.GetJob()).Methods("GET")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestServer_CancelJob(t *testing.T) {
	t.Run("success - cancel running job", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		createdAt := time.Now().Add(-1 * time.Hour)
		updatedAt := time.Now()

		// Expect GetJob call to check current state
		store.EXPECT().GetJob(gomock.Any(), int64(42)).Return(&storage.Job{
			Job: model.Job{
				ID:        42,
				Type:      "MovieIndex",
				CreatedAt: &createdAt,
			},
			State:     storage.JobStateRunning,
			UpdatedAt: &updatedAt,
		}, nil)

		// After cancellation, return cancelled job
		store.EXPECT().GetJob(gomock.Any(), int64(42)).Return(&storage.Job{
			Job: model.Job{
				ID:        42,
				Type:      "MovieIndex",
				CreatedAt: &createdAt,
			},
			State:     storage.JobStateCancelled,
			UpdatedAt: &updatedAt,
		}, nil)

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := Server{
			baseLogger: zap.NewNop().Sugar(),
			manager:    mgr,
		}

		req, err := http.NewRequest("POST", "/jobs/42/cancel", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/jobs/{id}/cancel", s.CancelJob()).Methods("POST")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("content-type"))

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		jobResp, ok := response.Response.(map[string]any)
		require.True(t, ok, "Response should be a map")

		assert.Equal(t, float64(42), jobResp["id"])
		assert.Equal(t, "cancelled", jobResp["state"])
	})

	t.Run("invalid job id format", func(t *testing.T) {
		s := Server{baseLogger: zap.NewNop().Sugar()}

		req, err := http.NewRequest("POST", "/jobs/invalid/cancel", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/jobs/{id}/cancel", s.CancelJob()).Methods("POST")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Invalid ID format")
	})

	t.Run("error - job not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		store.EXPECT().GetJob(gomock.Any(), int64(999)).Return(nil, storage.ErrNotFound)

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := Server{
			baseLogger: zap.NewNop().Sugar(),
			manager:    mgr,
		}

		req, err := http.NewRequest("POST", "/jobs/999/cancel", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/jobs/{id}/cancel", s.CancelJob()).Methods("POST")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestServer_CreateJob(t *testing.T) {
	t.Run("success - create new job", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		createdAt := time.Now()
		updatedAt := time.Now()

		// Expect CreateJob to be called
		store.EXPECT().CreateJob(gomock.Any(), gomock.Any(), storage.JobStatePending).Return(int64(1), nil)
		store.EXPECT().GetJob(gomock.Any(), int64(1)).Return(&storage.Job{
			Job: model.Job{
				ID:        1,
				Type:      "MovieIndex",
				CreatedAt: &createdAt,
			},
			State:     storage.JobStatePending,
			UpdatedAt: &updatedAt,
		}, nil)

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := Server{
			baseLogger: zap.NewNop().Sugar(),
			manager:    mgr,
		}

		requestBody := `{"type":"MovieIndex"}`
		req, err := http.NewRequest("POST", "/jobs", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.CreateJob()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("content-type"))

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		jobResp, ok := response.Response.(map[string]any)
		require.True(t, ok, "Response should be a map")

		assert.Equal(t, float64(1), jobResp["id"])
		assert.Equal(t, "MovieIndex", jobResp["type"])
		assert.Equal(t, "pending", jobResp["state"])
	})

	t.Run("success - job already pending", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		createdAt := time.Now()
		updatedAt := time.Now()

		existingJob := &storage.Job{
			Job: model.Job{
				ID:        5,
				Type:      "MovieIndex",
				CreatedAt: &createdAt,
			},
			State:     storage.JobStatePending,
			UpdatedAt: &updatedAt,
		}

		// Simulate job already pending
		store.EXPECT().CreateJob(gomock.Any(), gomock.Any(), storage.JobStatePending).Return(int64(0), storage.ErrJobAlreadyPending)
		store.EXPECT().ListJobs(gomock.Any(), gomock.Any()).Return([]*storage.Job{existingJob}, nil)

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := Server{
			baseLogger: zap.NewNop().Sugar(),
			manager:    mgr,
		}

		requestBody := `{"type":"MovieIndex"}`
		req, err := http.NewRequest("POST", "/jobs", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.CreateJob()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		jobResp, ok := response.Response.(map[string]any)
		require.True(t, ok, "Response should be a map")

		assert.Equal(t, float64(5), jobResp["id"])
		assert.Equal(t, "pending", jobResp["state"])
	})

	t.Run("invalid request body", func(t *testing.T) {
		s := Server{baseLogger: zap.NewNop().Sugar()}

		requestBody := `{"invalid json`
		req, err := http.NewRequest("POST", "/jobs", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.CreateJob()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid request body")
	})

	t.Run("error - storage error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		store.EXPECT().CreateJob(gomock.Any(), gomock.Any(), storage.JobStatePending).Return(int64(0), errors.New("storage error"))

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := Server{
			baseLogger: zap.NewNop().Sugar(),
			manager:    mgr,
		}

		requestBody := `{"type":"MovieIndex"}`
		req, err := http.NewRequest("POST", "/jobs", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.CreateJob()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

// Helper function for creating pointers
func ptr[T any](v T) *T {
	return &v
}
