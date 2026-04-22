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
	"github.com/kasuboski/mediaz/pkg/download"
	downloadMocks "github.com/kasuboski/mediaz/pkg/download/mocks"
	"github.com/kasuboski/mediaz/pkg/manager"
	storeMocks "github.com/kasuboski/mediaz/pkg/storage/mocks"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	tmdbMocks "github.com/kasuboski/mediaz/pkg/tmdb/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestServer_UpdateDownloadClient(t *testing.T) {
	t.Run("success - update download client", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		apiKey := "test-api-key"
		existingClient := model.DownloadClient{
			ID:             1,
			Type:           "usenet",
			Implementation: "sabnzbd",
			Scheme:         "http",
			Host:           "localhost",
			Port:           8080,
			APIKey:         &apiKey,
		}
		store.EXPECT().GetDownloadClient(gomock.Any(), int64(1)).Return(existingClient, nil)
		store.EXPECT().UpdateDownloadClient(gomock.Any(), int64(1), gomock.Any()).Return(nil)

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := newTestServer(withManager(mgr))

		requestBody := `{"type":"usenet","implementation":"sabnzbd","scheme":"https","host":"sabnzbd.example.com","port":443}`
		req, err := http.NewRequest("PUT", "/download/clients/1", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/download/clients/{id}", s.UpdateDownloadClient()).Methods("PUT")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("content-type"))

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		clientResp, ok := response.Response.(map[string]any)
		require.True(t, ok, "Response should be a map")

		assert.Equal(t, float64(1), clientResp["ID"])
		assert.Equal(t, "usenet", clientResp["Type"])
		assert.Equal(t, "sabnzbd", clientResp["Implementation"])
		assert.Equal(t, "https", clientResp["Scheme"])
		assert.Equal(t, "sabnzbd.example.com", clientResp["Host"])
		assert.Equal(t, float64(443), clientResp["Port"])
	})

	t.Run("invalid id format", func(t *testing.T) {
		s := newTestServer()

		requestBody := `{"type":"usenet","implementation":"sabnzbd"}`
		req, err := http.NewRequest("PUT", "/download/clients/invalid", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/download/clients/{id}", s.UpdateDownloadClient()).Methods("PUT")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid id")
	})

	t.Run("invalid request body - malformed json", func(t *testing.T) {
		s := newTestServer()

		requestBody := `{"invalid json`
		req, err := http.NewRequest("PUT", "/download/clients/1", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/download/clients/{id}", s.UpdateDownloadClient()).Methods("PUT")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid request body")
	})

	t.Run("error - manager returns error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		store.EXPECT().GetDownloadClient(gomock.Any(), int64(1)).Return(model.DownloadClient{}, errors.New("client not found"))

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := newTestServer(withManager(mgr))

		requestBody := `{"type":"usenet","implementation":"sabnzbd"}`
		req, err := http.NewRequest("PUT", "/download/clients/1", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/download/clients/{id}", s.UpdateDownloadClient()).Methods("PUT")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("content-type"))

		responseBody := rr.Body.String()
		assert.Contains(t, responseBody, "error")
	})
}

func TestServer_TestDownloadClient(t *testing.T) {
	t.Run("success - connection test succeeds", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)
		factory := downloadMocks.NewMockFactory(ctrl)
		client := downloadMocks.NewMockDownloadClient(ctrl)

		testClient := model.DownloadClient{
			Type:           "torrent",
			Implementation: "transmission",
			Scheme:         "http",
			Host:           "localhost",
			Port:           9091,
		}

		factory.EXPECT().NewDownloadClient(testClient).Return(client, nil)
		client.EXPECT().List(gomock.Any()).Return([]download.Status{}, nil)

		mgr := manager.New(tmdbMock, nil, nil, store, factory, config.Manager{}, config.Config{})

		s := newTestServer(withManager(mgr))

		requestBody := `{"type":"torrent","implementation":"transmission","scheme":"http","host":"localhost","port":9091}`
		req, err := http.NewRequest("POST", "/download/clients/test", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.TestDownloadClient()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("content-type"))

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		respMap, ok := response.Response.(map[string]any)
		require.True(t, ok, "Response should be a map")
		assert.Equal(t, "Connection successful", respMap["message"])
	})

	t.Run("invalid request body - malformed json", func(t *testing.T) {
		s := newTestServer()

		requestBody := `{"invalid json`
		req, err := http.NewRequest("POST", "/download/clients/test", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.TestDownloadClient()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid request body")
	})

	t.Run("error - connection test fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)
		factory := downloadMocks.NewMockFactory(ctrl)
		client := downloadMocks.NewMockDownloadClient(ctrl)

		testClient := model.DownloadClient{
			Type:           "torrent",
			Implementation: "transmission",
			Scheme:         "http",
			Host:           "invalid-host",
			Port:           9091,
		}

		factory.EXPECT().NewDownloadClient(testClient).Return(client, nil)
		client.EXPECT().List(gomock.Any()).Return(nil, errors.New("connection failed"))

		mgr := manager.New(tmdbMock, nil, nil, store, factory, config.Manager{}, config.Config{})

		s := newTestServer(withManager(mgr))

		requestBody := `{"type":"torrent","implementation":"transmission","scheme":"http","host":"invalid-host","port":9091}`
		req, err := http.NewRequest("POST", "/download/clients/test", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.TestDownloadClient()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("content-type"))

		responseBody := rr.Body.String()
		assert.Contains(t, responseBody, "error")
	})
}
