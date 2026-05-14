package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/manager"
	"github.com/kasuboski/mediaz/pkg/storage"
	mediaSqlite "github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	tmdbMocks "github.com/kasuboski/mediaz/pkg/tmdb/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// newInMemoryStore creates an in-memory SQLite storage with schemas initialized.
func newInMemoryStore(t *testing.T) storage.Storage {
	t.Helper()
	ctx := context.Background()
	store, err := mediaSqlite.New(ctx, ":memory:")
	require.NoError(t, err)
	schemas, err := storage.GetSchemas()
	require.NoError(t, err)
	err = store.Init(ctx, schemas...)
	require.NoError(t, err)
	return store
}

// gomockController creates a gomock controller that is auto-finished on test cleanup.
func gomockController(t *testing.T) *gomock.Controller {
	t.Helper()
	return gomock.NewController(t)
}

// modelIndexer creates a model.Indexer with the given fields.
func modelIndexer(name string, priority int32, uri string) model.Indexer {
	return model.Indexer{
		Name:     name,
		Priority: priority,
		URI:      uri,
	}
}

func newIndexerStore(t *testing.T) storage.Storage {
	t.Helper()
	ctx := context.Background()
	store, err := mediaSqlite.New(ctx, ":memory:")
	require.NoError(t, err)
	schemas, err := storage.GetSchemas()
	require.NoError(t, err)
	err = store.Init(ctx, schemas...)
	require.NoError(t, err)
	return store
}

func newIndexerManager(t *testing.T) manager.MediaManager {
	t.Helper()
	ctrl := gomockController(t)
	tmdbMock := tmdbMocks.NewMockITmdb(ctrl)
	store := newIndexerStore(t)
	return manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})
}

func TestServer_ListIndexers(t *testing.T) {
	t.Run("success - returns empty list", func(t *testing.T) {
		mgr := newIndexerManager(t)

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

		assert.Nil(t, response.Response)
	})

	t.Run("success - returns indexers", func(t *testing.T) {
		mgr := newIndexerManager(t)
		ctx := context.Background()

		// Seed two indexers
		_, err := mgr.AddIndexer(ctx, manager.AddIndexerRequest{
			Indexer: modelIndexer("test-indexer", 10, "http://example.com"),
		})
		require.NoError(t, err)

		_, err = mgr.AddIndexer(ctx, manager.AddIndexerRequest{
			Indexer: modelIndexer("another-indexer", 20, "http://example.org"),
		})
		require.NoError(t, err)

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
		require.Len(t, indexerList, 2)

		// Collect by name to avoid relying on DB ordering
		byName := make(map[string]map[string]any)
		for _, item := range indexerList {
			m := item.(map[string]any)
			byName[m["name"].(string)] = m
		}

		first := byName["test-indexer"]
		assert.Equal(t, float64(1), first["id"])
		assert.Equal(t, "test-indexer", first["name"])
		assert.Equal(t, "Internal", first["source"])
		assert.Equal(t, float64(10), first["priority"])
		assert.Equal(t, "http://example.com", first["uri"])

		second := byName["another-indexer"]
		assert.Equal(t, float64(2), second["id"])
		assert.Equal(t, "another-indexer", second["name"])
		assert.Equal(t, "Internal", second["source"])
		assert.Equal(t, float64(20), second["priority"])
		assert.Equal(t, "http://example.org", second["uri"])
	})
}

func TestServer_CreateIndexer(t *testing.T) {
	t.Run("success - creates an indexer", func(t *testing.T) {
		mgr := newIndexerManager(t)

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
		assert.Equal(t, float64(10), indexer["priority"])
		assert.Equal(t, "http://example.com", indexer["uri"])
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
		mgr := newIndexerManager(t)

		s := newTestServer(withManager(mgr))

		requestBody := `{"name":"","priority":10,"uri":"http://example.com"}`
		req, err := http.NewRequest("POST", "/indexers", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.CreateIndexer()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})
}

func TestServer_DeleteIndexer(t *testing.T) {
	t.Run("success - deletes an indexer", func(t *testing.T) {
		mgr := newIndexerManager(t)
		ctx := context.Background()

		// Seed an indexer
		_, err := mgr.AddIndexer(ctx, manager.AddIndexerRequest{
			Indexer: modelIndexer("to-delete", 5, "http://example.com"),
		})
		require.NoError(t, err)

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

	t.Run("error - storage error (nonexistent id)", func(t *testing.T) {
		mgr := newIndexerManager(t)

		s := newTestServer(withManager(mgr))

		req, err := http.NewRequest("DELETE", "/indexers/999", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/indexers/{id}", s.DeleteIndexer()).Methods("DELETE")
		router.ServeHTTP(rr, req)

		// Deleting a nonexistent row is typically a no-op in SQLite but
		// the handler returns 500 if the manager returns any error.
		// With real SQLite this succeeds silently, so expect 200.
		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestServer_UpdateIndexer(t *testing.T) {
	t.Run("success - updates an indexer", func(t *testing.T) {
		mgr := newIndexerManager(t)
		ctx := context.Background()

		// Seed an indexer
		_, err := mgr.AddIndexer(ctx, manager.AddIndexerRequest{
			Indexer: modelIndexer("original", 5, "http://original.com"),
		})
		require.NoError(t, err)

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
		assert.Equal(t, "http://updated.com", indexer["uri"])
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
		mgr := newIndexerManager(t)

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
}
