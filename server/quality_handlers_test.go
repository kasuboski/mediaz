package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
)

// --- Quality Definition tests ---

func TestServer_ListQualityDefinitions(t *testing.T) {
	t.Run("success - returns definitions", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		defs := []*model.QualityDefinition{
			{ID: 1, Name: "Bluray-1080p", MediaType: "movie", PreferredSize: 40, MinSize: 20, MaxSize: 60},
			{ID: 2, Name: "HDTV-720p", MediaType: "episode", PreferredSize: 10, MinSize: 5, MaxSize: 15},
		}

		store.EXPECT().ListQualityDefinitions(gomock.Any()).Return(defs, nil)

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := newTestServer(withManager(mgr))

		req, err := http.NewRequest("GET", "/quality/definitions", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.ListQualityDefinitions()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("content-type"))

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		defList, ok := response.Response.([]any)
		require.True(t, ok, "Response should be an array")
		assert.Len(t, defList, 2)

		first := defList[0].(map[string]any)
		assert.Equal(t, float64(1), first["ID"])
		assert.Equal(t, "Bluray-1080p", first["Name"])
	})

	t.Run("error - storage error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		store.EXPECT().ListQualityDefinitions(gomock.Any()).Return(nil, errors.New("storage error"))

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := newTestServer(withManager(mgr))

		req, err := http.NewRequest("GET", "/quality/definitions", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.ListQualityDefinitions()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestServer_GetQualityDefinition(t *testing.T) {
	t.Run("success - returns definition by id", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		store.EXPECT().GetQualityDefinition(gomock.Any(), int64(1)).Return(model.QualityDefinition{
			ID: 1, Name: "Bluray-1080p", MediaType: "movie", PreferredSize: 40, MinSize: 20, MaxSize: 60,
		}, nil)

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := newTestServer(withManager(mgr))

		req, err := http.NewRequest("GET", "/quality/definitions/1", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/quality/definitions/{id}", s.GetQualityDefinition()).Methods("GET")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("content-type"))

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		def, ok := response.Response.(map[string]any)
		require.True(t, ok, "Response should be a map")
		assert.Equal(t, float64(1), def["ID"])
		assert.Equal(t, "Bluray-1080p", def["Name"])
	})

	t.Run("invalid id format", func(t *testing.T) {
		s := newTestServer()

		req, err := http.NewRequest("GET", "/quality/definitions/invalid", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/quality/definitions/{id}", s.GetQualityDefinition()).Methods("GET")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid id")
	})

	t.Run("error - storage error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		store.EXPECT().GetQualityDefinition(gomock.Any(), int64(999)).Return(model.QualityDefinition{}, errors.New("not found"))

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := newTestServer(withManager(mgr))

		req, err := http.NewRequest("GET", "/quality/definitions/999", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/quality/definitions/{id}", s.GetQualityDefinition()).Methods("GET")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestServer_CreateQualityDefinition(t *testing.T) {
	t.Run("success - creates a definition", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		store.EXPECT().CreateQualityDefinition(gomock.Any(), gomock.Any()).Return(int64(1), nil)

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := newTestServer(withManager(mgr))

		requestBody := `{"name":"Bluray-1080p","type":"movie","preferredSize":40,"minSize":20,"maxSize":60}`
		req, err := http.NewRequest("POST", "/quality/definitions", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.CreateQualityDefinition()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("content-type"))

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		def, ok := response.Response.(map[string]any)
		require.True(t, ok, "Response should be a map")
		assert.Equal(t, float64(1), def["ID"])
		assert.Equal(t, "Bluray-1080p", def["Name"])
		assert.Equal(t, "movie", def["MediaType"])
	})

	t.Run("invalid request body - malformed json", func(t *testing.T) {
		s := newTestServer()

		requestBody := `{"invalid json`
		req, err := http.NewRequest("POST", "/quality/definitions", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.CreateQualityDefinition()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid request body")
	})

	t.Run("error - storage error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		store.EXPECT().CreateQualityDefinition(gomock.Any(), gomock.Any()).Return(int64(0), errors.New("storage error"))

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := newTestServer(withManager(mgr))

		requestBody := `{"name":"Bluray-1080p","type":"movie","preferredSize":40,"minSize":20,"maxSize":60}`
		req, err := http.NewRequest("POST", "/quality/definitions", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.CreateQualityDefinition()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestServer_UpdateQualityDefinition(t *testing.T) {
	t.Run("success - updates a definition", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		store.EXPECT().UpdateQualityDefinition(gomock.Any(), int64(1), gomock.Any()).Return(nil)
		store.EXPECT().GetQualityDefinition(gomock.Any(), int64(1)).Return(model.QualityDefinition{
			ID: 1, Name: "Bluray-720p", MediaType: "movie", PreferredSize: 25, MinSize: 10, MaxSize: 40,
		}, nil)

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := newTestServer(withManager(mgr))

		requestBody := `{"name":"Bluray-720p","type":"movie","preferredSize":25,"minSize":10,"maxSize":40}`
		req, err := http.NewRequest("PUT", "/quality/definitions/1", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/quality/definitions/{id}", s.UpdateQualityDefinition()).Methods("PUT")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("content-type"))

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		def, ok := response.Response.(map[string]any)
		require.True(t, ok, "Response should be a map")
		assert.Equal(t, float64(1), def["ID"])
		assert.Equal(t, "Bluray-720p", def["Name"])
	})

	t.Run("invalid id format", func(t *testing.T) {
		s := newTestServer()

		requestBody := `{"name":"Bluray-720p","type":"movie"}`
		req, err := http.NewRequest("PUT", "/quality/definitions/invalid", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/quality/definitions/{id}", s.UpdateQualityDefinition()).Methods("PUT")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid id")
	})

	t.Run("invalid request body - malformed json", func(t *testing.T) {
		s := newTestServer()

		requestBody := `{"invalid json`
		req, err := http.NewRequest("PUT", "/quality/definitions/1", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/quality/definitions/{id}", s.UpdateQualityDefinition()).Methods("PUT")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid request body")
	})

	t.Run("error - storage error on update", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		store.EXPECT().UpdateQualityDefinition(gomock.Any(), int64(1), gomock.Any()).Return(errors.New("storage error"))

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := newTestServer(withManager(mgr))

		requestBody := `{"name":"Bluray-720p","type":"movie","preferredSize":25,"minSize":10,"maxSize":40}`
		req, err := http.NewRequest("PUT", "/quality/definitions/1", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/quality/definitions/{id}", s.UpdateQualityDefinition()).Methods("PUT")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestServer_DeleteQualityDefinition(t *testing.T) {
	t.Run("success - deletes a definition", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		store.EXPECT().DeleteQualityDefinition(gomock.Any(), int64(1)).Return(nil)

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := newTestServer(withManager(mgr))

		req, err := http.NewRequest("DELETE", "/quality/definitions/1", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/quality/definitions/{id}", s.DeleteQualityDefinition()).Methods("DELETE")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("content-type"))

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		respMap, ok := response.Response.(map[string]any)
		require.True(t, ok, "Response should be a map")
		assert.Equal(t, float64(1), respMap["id"])
	})

	t.Run("invalid id format", func(t *testing.T) {
		s := newTestServer()

		req, err := http.NewRequest("DELETE", "/quality/definitions/invalid", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/quality/definitions/{id}", s.DeleteQualityDefinition()).Methods("DELETE")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid id")
	})

	t.Run("error - storage error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		store.EXPECT().DeleteQualityDefinition(gomock.Any(), int64(1)).Return(errors.New("storage error"))

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := newTestServer(withManager(mgr))

		req, err := http.NewRequest("DELETE", "/quality/definitions/1", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/quality/definitions/{id}", s.DeleteQualityDefinition()).Methods("DELETE")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

// --- Quality Profile tests ---

func TestServer_GetQualityProfile(t *testing.T) {
	t.Run("success - returns profile by id", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		cutoffID := int32(3)
		profile := storage.QualityProfile{
			ID:              1,
			Name:            "HD Profile",
			CutoffQualityID: &cutoffID,
			UpgradeAllowed:  true,
			Qualities: []storage.QualityDefinition{
				{ID: 1, Name: "HDTV-720p", MediaType: "movie"},
				{ID: 3, Name: "Bluray-1080p", MediaType: "movie"},
			},
		}

		store.EXPECT().GetQualityProfile(gomock.Any(), int64(1)).Return(profile, nil)

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := newTestServer(withManager(mgr))

		req, err := http.NewRequest("GET", "/quality/profiles/1", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/quality/profiles/{id}", s.GetQualityProfile()).Methods("GET")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("content-type"))

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		prof, ok := response.Response.(map[string]any)
		require.True(t, ok, "Response should be a map")
		assert.Equal(t, float64(1), prof["id"])
		assert.Equal(t, "HD Profile", prof["name"])
		assert.Equal(t, true, prof["upgradeAllowed"])
	})

	t.Run("invalid id format", func(t *testing.T) {
		s := newTestServer()

		req, err := http.NewRequest("GET", "/quality/profiles/invalid", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/quality/profiles/{id}", s.GetQualityProfile()).Methods("GET")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid id")
	})

	t.Run("error - storage error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		store.EXPECT().GetQualityProfile(gomock.Any(), int64(999)).Return(storage.QualityProfile{}, errors.New("not found"))

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := newTestServer(withManager(mgr))

		req, err := http.NewRequest("GET", "/quality/profiles/999", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/quality/profiles/{id}", s.GetQualityProfile()).Methods("GET")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestServer_ListQualityProfiles(t *testing.T) {
	t.Run("success - returns all profiles when no type filter", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		profiles := []*storage.QualityProfile{
			{ID: 1, Name: "Any HD", UpgradeAllowed: false},
			{ID: 2, Name: "Movie 4K", UpgradeAllowed: true},
		}

		store.EXPECT().ListQualityProfiles(gomock.Any(), gomock.Any()).Return(profiles, nil)

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := newTestServer(withManager(mgr))

		req, err := http.NewRequest("GET", "/quality/profiles", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.ListQualityProfiles()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("content-type"))

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		profileList, ok := response.Response.([]*storage.QualityProfile)
		// Profiles may come back as []any due to JSON round-trip
		if !ok {
			profileAny, ok := response.Response.([]any)
			require.True(t, ok, "Response should be an array")
			assert.Len(t, profileAny, 2)
		} else {
			assert.Len(t, profileList, 2)
		}
	})

	t.Run("success - filters by movie type", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		profiles := []*storage.QualityProfile{
			{ID: 2, Name: "Movie 4K", UpgradeAllowed: true},
		}

		store.EXPECT().ListQualityProfiles(gomock.Any(), gomock.Any()).Return(profiles, nil)

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := newTestServer(withManager(mgr))

		req, err := http.NewRequest("GET", "/quality/profiles?type=movie", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.ListQualityProfiles()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("success - filters by series type", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		profiles := []*storage.QualityProfile{
			{ID: 3, Name: "Series HD", UpgradeAllowed: false},
		}

		store.EXPECT().ListQualityProfiles(gomock.Any(), gomock.Any()).Return(profiles, nil)

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := newTestServer(withManager(mgr))

		req, err := http.NewRequest("GET", "/quality/profiles?type=series", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.ListQualityProfiles()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("error - storage error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		store.EXPECT().ListQualityProfiles(gomock.Any(), gomock.Any()).Return(nil, errors.New("storage error"))

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := newTestServer(withManager(mgr))

		req, err := http.NewRequest("GET", "/quality/profiles", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.ListQualityProfiles()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestServer_CreateQualityProfile(t *testing.T) {
	t.Run("success - creates a profile", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		cutoffID := int32(2)

		store.EXPECT().CreateQualityProfile(gomock.Any(), gomock.Any()).Return(int64(1), nil)
		store.EXPECT().CreateQualityProfileItems(gomock.Any(), gomock.Any()).Return(nil)
		store.EXPECT().GetQualityProfile(gomock.Any(), int64(1)).Return(storage.QualityProfile{
			ID:              1,
			Name:            "Test Profile",
			CutoffQualityID: &cutoffID,
			UpgradeAllowed:  true,
			Qualities: []storage.QualityDefinition{
				{ID: 1, Name: "HDTV-720p", MediaType: "movie"},
				{ID: 2, Name: "Bluray-1080p", MediaType: "movie"},
			},
		}, nil)

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := newTestServer(withManager(mgr))

		requestBody := `{"name":"Test Profile","cutoffQualityId":2,"upgradeAllowed":true,"qualityIds":[1,2]}`
		req, err := http.NewRequest("POST", "/quality/profiles", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.CreateQualityProfile()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("content-type"))

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		prof, ok := response.Response.(map[string]any)
		require.True(t, ok, "Response should be a map")
		assert.Equal(t, float64(1), prof["id"])
		assert.Equal(t, "Test Profile", prof["name"])
	})

	t.Run("invalid request body - malformed json", func(t *testing.T) {
		s := newTestServer()

		requestBody := `{"invalid json`
		req, err := http.NewRequest("POST", "/quality/profiles", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.CreateQualityProfile()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid request body")
	})

	t.Run("error - storage error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		store.EXPECT().CreateQualityProfile(gomock.Any(), gomock.Any()).Return(int64(0), errors.New("storage error"))

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := newTestServer(withManager(mgr))

		requestBody := `{"name":"Test Profile","qualityIds":[1]}`
		req, err := http.NewRequest("POST", "/quality/profiles", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.CreateQualityProfile()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestServer_UpdateQualityProfile(t *testing.T) {
	t.Run("success - updates a profile", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		cutoffID := int32(2)
		existingProfile := storage.QualityProfile{
			ID:              1,
			Name:            "Old Name",
			CutoffQualityID: &cutoffID,
			UpgradeAllowed:  false,
			Qualities: []storage.QualityDefinition{
				{ID: 1, Name: "HDTV-720p"},
			},
		}

		newCutoffID := int32(2)
		updatedProfile := storage.QualityProfile{
			ID:              1,
			Name:            "Updated Profile",
			CutoffQualityID: &newCutoffID,
			UpgradeAllowed:  true,
			Qualities: []storage.QualityDefinition{
				{ID: 1, Name: "HDTV-720p"},
				{ID: 2, Name: "Bluray-1080p"},
			},
		}

		store.EXPECT().GetQualityProfile(gomock.Any(), int64(1)).Return(existingProfile, nil)
		store.EXPECT().UpdateQualityProfile(gomock.Any(), int64(1), gomock.Any()).Return(nil)
		store.EXPECT().DeleteQualityProfileItemsByProfileID(gomock.Any(), int64(1)).Return(nil)
		store.EXPECT().CreateQualityProfileItems(gomock.Any(), gomock.Any()).Return(nil)
		store.EXPECT().GetQualityProfile(gomock.Any(), int64(1)).Return(updatedProfile, nil)

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := newTestServer(withManager(mgr))

		requestBody := `{"name":"Updated Profile","cutoffQualityId":2,"upgradeAllowed":true,"qualityIds":[1,2]}`
		req, err := http.NewRequest("PUT", "/quality/profiles/1", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/quality/profiles/{id}", s.UpdateQualityProfile()).Methods("PUT")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("content-type"))

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		prof, ok := response.Response.(map[string]any)
		require.True(t, ok, "Response should be a map")
		assert.Equal(t, float64(1), prof["id"])
		assert.Equal(t, "Updated Profile", prof["name"])
		assert.Equal(t, true, prof["upgradeAllowed"])
	})

	t.Run("invalid id format", func(t *testing.T) {
		s := newTestServer()

		requestBody := `{"name":"Updated Profile","qualityIds":[1]}`
		req, err := http.NewRequest("PUT", "/quality/profiles/invalid", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/quality/profiles/{id}", s.UpdateQualityProfile()).Methods("PUT")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid id")
	})

	t.Run("invalid request body - malformed json", func(t *testing.T) {
		s := newTestServer()

		requestBody := `{"invalid json`
		req, err := http.NewRequest("PUT", "/quality/profiles/1", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/quality/profiles/{id}", s.UpdateQualityProfile()).Methods("PUT")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid request body")
	})

	t.Run("error - storage error on get existing", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		store.EXPECT().GetQualityProfile(gomock.Any(), int64(1)).Return(storage.QualityProfile{}, errors.New("storage error"))

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := newTestServer(withManager(mgr))

		requestBody := `{"name":"Updated Profile","qualityIds":[1]}`
		req, err := http.NewRequest("PUT", "/quality/profiles/1", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/quality/profiles/{id}", s.UpdateQualityProfile()).Methods("PUT")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestServer_DeleteQualityProfile(t *testing.T) {
	t.Run("success - deletes a profile", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		store.EXPECT().DeleteQualityProfile(gomock.Any(), int64(1)).Return(nil)

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := newTestServer(withManager(mgr))

		req, err := http.NewRequest("DELETE", "/quality/profiles/1", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/quality/profiles/{id}", s.DeleteQualityProfile()).Methods("DELETE")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("content-type"))
	})

	t.Run("invalid id format", func(t *testing.T) {
		s := newTestServer()

		req, err := http.NewRequest("DELETE", "/quality/profiles/invalid", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/quality/profiles/{id}", s.DeleteQualityProfile()).Methods("DELETE")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid id")
	})

	t.Run("error - storage error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		store.EXPECT().DeleteQualityProfile(gomock.Any(), int64(1)).Return(errors.New("storage error"))

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := newTestServer(withManager(mgr))

		req, err := http.NewRequest("DELETE", "/quality/profiles/1", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/quality/profiles/{id}", s.DeleteQualityProfile()).Methods("DELETE")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}
