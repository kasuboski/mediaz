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
	storeMocks "github.com/kasuboski/mediaz/pkg/storage/mocks"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	tmdbMocks "github.com/kasuboski/mediaz/pkg/tmdb/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestServer_ListIndexers(t *testing.T) {
	t.Run("success - returns empty list", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		store.EXPECT().ListIndexers(gomock.Any(), gomock.Any()).Return([]*model.Indexer{}, nil)

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := newTestServer(withManager(mgr))

		req, err := http.NewRequest("GET", "/indexers", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.ListIndexers()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("content-type"))

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		// When nil is marshalled to JSON it becomes null, which unmarshals to nil
		assert.Nil(t, response.Response)
	})

	t.Run("success - returns indexers", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		indexers := []*model.Indexer{
			{ID: 1, Name: "test-indexer", Priority: 10, URI: "http://example.com"},
			{ID: 2, Name: "another-indexer", Priority: 20, URI: "http://example.org"},
		}

		store.EXPECT().ListIndexers(gomock.Any(), gomock.Any()).Return(indexers, nil)

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := newTestServer(withManager(mgr))

		req, err := http.NewRequest("GET", "/indexers", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.ListIndexers()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		indexerList, ok := response.Response.([]any)
		require.True(t, ok, "Response should be an array")
		assert.Len(t, indexerList, 2)

		first := indexerList[0].(map[string]any)
		assert.Equal(t, float64(1), first["id"])
		assert.Equal(t, "test-indexer", first["name"])
		assert.Equal(t, "Internal", first["source"])
	})

	t.Run("error - storage error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		store.EXPECT().ListIndexers(gomock.Any(), gomock.Any()).Return(nil, errors.New("storage error"))

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := newTestServer(withManager(mgr))

		req, err := http.NewRequest("GET", "/indexers", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.ListIndexers()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("content-type"))
	})
}

func TestServer_CreateIndexer(t *testing.T) {
	t.Run("success - creates an indexer", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		store.EXPECT().CreateIndexer(gomock.Any(), gomock.Any()).Return(int64(1), nil)

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := newTestServer(withManager(mgr))

		requestBody := `{"name":"test-indexer","priority":10,"uri":"http://example.com"}`
		req, err := http.NewRequest("POST", "/indexers", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.CreateIndexer()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("content-type"))

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		indexer, ok := response.Response.(map[string]any)
		require.True(t, ok, "Response should be a map")
		assert.Equal(t, float64(1), indexer["id"])
		assert.Equal(t, "test-indexer", indexer["name"])
	})

	t.Run("invalid request body - malformed json", func(t *testing.T) {
		s := newTestServer()

		requestBody := `{"invalid json`
		req, err := http.NewRequest("POST", "/indexers", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.CreateIndexer()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid request body")
	})

	t.Run("error - validation error (empty name)", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := newTestServer(withManager(mgr))

		requestBody := `{"name":"","priority":10,"uri":"http://example.com"}`
		req, err := http.NewRequest("POST", "/indexers", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.CreateIndexer()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("error - storage error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		store.EXPECT().CreateIndexer(gomock.Any(), gomock.Any()).Return(int64(0), errors.New("storage error"))

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := newTestServer(withManager(mgr))

		requestBody := `{"name":"test-indexer","priority":10,"uri":"http://example.com"}`
		req, err := http.NewRequest("POST", "/indexers", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.CreateIndexer()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestServer_DeleteIndexer(t *testing.T) {
	t.Run("success - deletes an indexer", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		store.EXPECT().DeleteIndexer(gomock.Any(), int64(1)).Return(nil)

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := newTestServer(withManager(mgr))

		req, err := http.NewRequest("DELETE", "/indexers/1", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/indexers/{id}", s.DeleteIndexer()).Methods("DELETE")
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

		req, err := http.NewRequest("DELETE", "/indexers/invalid", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/indexers/{id}", s.DeleteIndexer()).Methods("DELETE")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid id")
	})

	t.Run("error - storage error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		store.EXPECT().DeleteIndexer(gomock.Any(), int64(1)).Return(errors.New("storage error"))

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := newTestServer(withManager(mgr))

		req, err := http.NewRequest("DELETE", "/indexers/1", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/indexers/{id}", s.DeleteIndexer()).Methods("DELETE")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestServer_UpdateIndexer(t *testing.T) {
	t.Run("success - updates an indexer", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		store.EXPECT().UpdateIndexer(gomock.Any(), int64(1), gomock.Any()).Return(nil)

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := newTestServer(withManager(mgr))

		requestBody := `{"name":"updated-indexer","priority":20,"uri":"http://updated.com"}`
		req, err := http.NewRequest("PUT", "/indexers/1", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/indexers/{id}", s.UpdateIndexer()).Methods("PUT")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("content-type"))

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		indexer, ok := response.Response.(map[string]any)
		require.True(t, ok, "Response should be a map")
		assert.Equal(t, float64(1), indexer["id"])
		assert.Equal(t, "updated-indexer", indexer["name"])
		assert.Equal(t, float64(20), indexer["priority"])
	})

	t.Run("invalid id format", func(t *testing.T) {
		s := newTestServer()

		requestBody := `{"name":"updated-indexer","priority":20,"uri":"http://updated.com"}`
		req, err := http.NewRequest("PUT", "/indexers/invalid", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/indexers/{id}", s.UpdateIndexer()).Methods("PUT")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid id")
	})

	t.Run("invalid request body - malformed json", func(t *testing.T) {
		s := newTestServer()

		requestBody := `{"invalid json`
		req, err := http.NewRequest("PUT", "/indexers/1", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/indexers/{id}", s.UpdateIndexer()).Methods("PUT")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid request body")
	})

	t.Run("error - validation error (empty name)", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := newTestServer(withManager(mgr))

		requestBody := `{"name":"","priority":20,"uri":"http://updated.com"}`
		req, err := http.NewRequest("PUT", "/indexers/1", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/indexers/{id}", s.UpdateIndexer()).Methods("PUT")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("error - storage error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		store.EXPECT().UpdateIndexer(gomock.Any(), int64(1), gomock.Any()).Return(errors.New("storage error"))

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := newTestServer(withManager(mgr))

		requestBody := `{"name":"updated-indexer","priority":20,"uri":"http://updated.com"}`
		req, err := http.NewRequest("PUT", "/indexers/1", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/indexers/{id}", s.UpdateIndexer()).Methods("PUT")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}
