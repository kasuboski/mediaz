package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/manager"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/ptr"
	storeMocks "github.com/kasuboski/mediaz/pkg/storage/mocks"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	tmdbMocks "github.com/kasuboski/mediaz/pkg/tmdb/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

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
			OriginalTitle:    ptr.To("Original Test Movie"),
			Overview:         ptr.To("A test movie overview"),
			Images:           "/test-poster.jpg",
			Runtime:          120,
			Year:             ptr.To(int32(2023)),
			ReleaseDate:      ptr.To(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)),
			Genres:           ptr.To("Action,Adventure"),
			Studio:           ptr.To("Test Studio"),
			Website:          ptr.To("https://testmovie.com"),
			Popularity:       ptr.To(85.5),
			CollectionTmdbID: ptr.To(int32(5000)),
			CollectionTitle:  ptr.To("Test Collection"),
		}

		store.EXPECT().GetMovieMetadata(gomock.Any(), gomock.Any()).Return(expectedMetadata, nil)

		// Setup expectations for GetMovieByMetadataID call
		expectedMovie := &storage.Movie{
			Movie: model.Movie{
				ID:               1,
				MovieMetadataID:  ptr.To(int32(1)),
				QualityProfileID: 1,
				Monitored:        1,
				Path:             ptr.To("/movies/Test Movie (2023)"),
			},
			State: storage.MovieStateDownloaded,
		}

		store.EXPECT().GetMovieByMetadataID(gomock.Any(), 1).Return(expectedMovie, nil)

		// Create manager with mocked dependencies
		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := newTestServer(withManager(mgr))

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

		s := newTestServer(withManager(mgr))

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
		s := newTestServer()

		req, err := http.NewRequest("GET", "/movie/invalid", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/movie/{tmdbID}", s.GetMovieDetailByTMDBID()).Methods("GET")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid tmdbID")
	})

	t.Run("manager error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		store.EXPECT().GetMovieMetadata(gomock.Any(), gomock.Any()).Return(nil, errors.New("metadata not found"))

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := newTestServer(withManager(mgr))

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

func TestServer_AddMovieToLibrary(t *testing.T) {
	t.Run("success - new movie", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		releaseDate := time.Now().AddDate(0, 0, -1)
		metadata := &model.MovieMetadata{
			ID:          1,
			TmdbID:      1234,
			Title:       "Test Movie",
			ReleaseDate: &releaseDate,
		}

		store.EXPECT().GetQualityProfile(gomock.Any(), int64(1)).Return(storage.QualityProfile{ID: 1}, nil)
		store.EXPECT().GetMovieMetadata(gomock.Any(), gomock.Any()).Return(metadata, nil)
		store.EXPECT().GetMovieByMetadataID(gomock.Any(), 1).Return(nil, storage.ErrNotFound)
		store.EXPECT().CreateMovie(gomock.Any(), gomock.Any(), storage.MovieStateMissing).Return(int64(1), nil)
		store.EXPECT().GetMovie(gomock.Any(), int64(1)).Return(&storage.Movie{
			Movie: model.Movie{
				ID:               1,
				Monitored:        1,
				QualityProfileID: 1,
				MovieMetadataID:  ptr.To(int32(1)),
				Path:             ptr.To("Test Movie"),
			},
			State: storage.MovieStateMissing,
		}, nil)

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})
		s := newTestServer(withManager(mgr))

		body := `{"tmdbId": 1234, "qualityProfileId": 1}`
		req, err := http.NewRequest("POST", "/library/movies", strings.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		router := mux.NewRouter()
		router.HandleFunc("/library/movies", s.AddMovieToLibrary()).Methods("POST")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		movieData, ok := response.Response.(map[string]any)
		require.True(t, ok, "Response should contain movie data")
		assert.Equal(t, float64(1), movieData["ID"])
		assert.Equal(t, "missing", movieData["state"])
		assert.Equal(t, float64(1), movieData["Monitored"])
		assert.Equal(t, float64(1), movieData["QualityProfileID"])
	})

	t.Run("conflict - movie already downloaded", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		metadata := &model.MovieMetadata{
			ID:     1,
			TmdbID: 1234,
			Title:  "Existing Movie",
		}

		store.EXPECT().GetQualityProfile(gomock.Any(), int64(1)).Return(storage.QualityProfile{ID: 1}, nil)
		store.EXPECT().GetMovieMetadata(gomock.Any(), gomock.Any()).Return(metadata, nil)
		store.EXPECT().GetMovieByMetadataID(gomock.Any(), 1).Return(&storage.Movie{
			Movie: model.Movie{
				ID:               5,
				MovieMetadataID:  ptr.To(int32(1)),
				Monitored:        1,
				QualityProfileID: 1,
			},
			State: storage.MovieStateDownloaded,
		}, nil)

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})
		s := newTestServer(withManager(mgr))

		body := `{"tmdbId": 1234, "qualityProfileId": 1}`
		req, err := http.NewRequest("POST", "/library/movies", strings.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		router := mux.NewRouter()
		router.HandleFunc("/library/movies", s.AddMovieToLibrary()).Methods("POST")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusConflict, rr.Code)

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		movieData, ok := response.Response.(map[string]any)
		require.True(t, ok, "Response should contain the existing movie")
		assert.Equal(t, float64(5), movieData["ID"])
		assert.Equal(t, "downloaded", movieData["state"])
		assert.Equal(t, float64(1), movieData["Monitored"])
		assert.Equal(t, float64(1), movieData["QualityProfileID"])
	})

	t.Run("conflict - movie downloading", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		metadata := &model.MovieMetadata{
			ID:     1,
			TmdbID: 1234,
			Title:  "Downloading Movie",
		}

		store.EXPECT().GetQualityProfile(gomock.Any(), int64(1)).Return(storage.QualityProfile{ID: 1}, nil)
		store.EXPECT().GetMovieMetadata(gomock.Any(), gomock.Any()).Return(metadata, nil)
		store.EXPECT().GetMovieByMetadataID(gomock.Any(), 1).Return(&storage.Movie{
			Movie: model.Movie{
				ID:               6,
				MovieMetadataID:  ptr.To(int32(1)),
				Monitored:        1,
				QualityProfileID: 1,
			},
			State: storage.MovieStateDownloading,
		}, nil)

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})
		s := newTestServer(withManager(mgr))

		body := `{"tmdbId": 1234, "qualityProfileId": 1}`
		req, err := http.NewRequest("POST", "/library/movies", strings.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		router := mux.NewRouter()
		router.HandleFunc("/library/movies", s.AddMovieToLibrary()).Methods("POST")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusConflict, rr.Code)

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		movieData, ok := response.Response.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, float64(6), movieData["ID"])
		assert.Equal(t, "downloading", movieData["state"])
		assert.Equal(t, float64(1), movieData["Monitored"])
		assert.Equal(t, float64(1), movieData["QualityProfileID"])
	})

	t.Run("ok - movie exists in discovered state", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		metadata := &model.MovieMetadata{
			ID:     1,
			TmdbID: 1234,
			Title:  "Discovered Movie",
		}

		store.EXPECT().GetQualityProfile(gomock.Any(), int64(1)).Return(storage.QualityProfile{ID: 1}, nil)
		store.EXPECT().GetMovieMetadata(gomock.Any(), gomock.Any()).Return(metadata, nil)
		store.EXPECT().GetMovieByMetadataID(gomock.Any(), 1).Return(&storage.Movie{
			Movie: model.Movie{
				ID:               7,
				MovieMetadataID:  ptr.To(int32(1)),
				Monitored:        1,
				QualityProfileID: 1,
			},
			State: storage.MovieStateDiscovered,
		}, nil)

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})
		s := newTestServer(withManager(mgr))

		body := `{"tmdbId": 1234, "qualityProfileId": 1}`
		req, err := http.NewRequest("POST", "/library/movies", strings.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		router := mux.NewRouter()
		router.HandleFunc("/library/movies", s.AddMovieToLibrary()).Methods("POST")
		router.ServeHTTP(rr, req)

		// Discovered movies still need reconciliation, so 200 is correct
		assert.Equal(t, http.StatusOK, rr.Code)

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		movieData, ok := response.Response.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, float64(7), movieData["ID"])
		assert.Equal(t, "discovered", movieData["state"])
		assert.Equal(t, float64(1), movieData["Monitored"])
		assert.Equal(t, float64(1), movieData["QualityProfileID"])
	})

	t.Run("bad request - invalid body", func(t *testing.T) {
		s := newTestServer()

		req, err := http.NewRequest("POST", "/library/movies", strings.NewReader("not json"))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		router := mux.NewRouter()
		router.HandleFunc("/library/movies", s.AddMovieToLibrary()).Methods("POST")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("internal error - quality profile not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		store.EXPECT().GetQualityProfile(gomock.Any(), int64(99)).Return(storage.QualityProfile{}, errors.New("not found"))

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})
		s := newTestServer(withManager(mgr))

		body := `{"tmdbId": 1234, "qualityProfileId": 99}`
		req, err := http.NewRequest("POST", "/library/movies", strings.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		router := mux.NewRouter()
		router.HandleFunc("/library/movies", s.AddMovieToLibrary()).Methods("POST")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Nil(t, response.Response)
		assert.NotEmpty(t, response.Error)
	})
}
