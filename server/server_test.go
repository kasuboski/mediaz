package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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
				ID:                1,
				MovieMetadataID:   ptr(int32(1)),
				QualityProfileID:  1,
				Monitored:         1,
				Path:              ptr("/movies/Test Movie (2023)"),
			},
			State: storage.MovieStateDownloaded,
		}

		store.EXPECT().GetMovieByMetadataID(gomock.Any(), 1).Return(expectedMovie, nil)

		// Create manager with mocked dependencies
		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{})

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

		movieDetail, ok := response.Response.(map[string]interface{})
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

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{})

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

		movieDetail, ok := response.Response.(map[string]interface{})
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

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{})

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

// Helper function for creating pointers
func ptr[T any](v T) *T {
	return &v
}
