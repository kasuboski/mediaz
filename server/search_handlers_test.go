package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/manager"
	"github.com/kasuboski/mediaz/pkg/storage"
	storeMocks "github.com/kasuboski/mediaz/pkg/storage/mocks"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func TestServer_SearchForMovie(t *testing.T) {
	t.Run("invalid id", func(t *testing.T) {
		s := Server{baseLogger: zap.NewNop().Sugar()}

		req, err := http.NewRequest("POST", "/library/movies/invalid/search", nil)
		require.NoError(t, err)

		req = mux.SetURLVars(req, map[string]string{"id": "invalid"})

		rr := httptest.NewRecorder()
		handler := s.SearchForMovie()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("movie not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		store.EXPECT().GetMovie(gomock.Any(), int64(1)).Return(nil, storage.ErrNotFound)

		mgr := manager.New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		s := Server{
			baseLogger: zap.NewNop().Sugar(),
			manager:    mgr,
		}

		req, err := http.NewRequest("POST", "/library/movies/1/search", nil)
		require.NoError(t, err)

		req = mux.SetURLVars(req, map[string]string{"id": "1"})

		rr := httptest.NewRecorder()
		handler := s.SearchForMovie()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("movie not monitored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		store.EXPECT().GetMovie(gomock.Any(), int64(1)).Return(&storage.Movie{
			Movie: model.Movie{ID: 1, Monitored: 0},
		}, nil)

		mgr := manager.New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		s := Server{
			baseLogger: zap.NewNop().Sugar(),
			manager:    mgr,
		}

		req, err := http.NewRequest("POST", "/library/movies/1/search", nil)
		require.NoError(t, err)

		req = mux.SetURLVars(req, map[string]string{"id": "1"})

		rr := httptest.NewRecorder()
		handler := s.SearchForMovie()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestServer_SearchForSeries(t *testing.T) {
	t.Run("invalid id", func(t *testing.T) {
		s := Server{baseLogger: zap.NewNop().Sugar()}

		req, err := http.NewRequest("POST", "/library/tv/invalid/search", nil)
		require.NoError(t, err)

		req = mux.SetURLVars(req, map[string]string{"id": "invalid"})

		rr := httptest.NewRecorder()
		handler := s.SearchForSeries()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("series not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		store.EXPECT().GetSeries(gomock.Any(), gomock.Any()).Return(nil, storage.ErrNotFound)

		mgr := manager.New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		s := Server{
			baseLogger: zap.NewNop().Sugar(),
			manager:    mgr,
		}

		req, err := http.NewRequest("POST", "/library/tv/1/search", nil)
		require.NoError(t, err)

		req = mux.SetURLVars(req, map[string]string{"id": "1"})

		rr := httptest.NewRecorder()
		handler := s.SearchForSeries()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("series not monitored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		store.EXPECT().GetSeries(gomock.Any(), gomock.Any()).Return(&storage.Series{
			Series: model.Series{ID: 1, Monitored: 0},
		}, nil)

		mgr := manager.New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		s := Server{
			baseLogger: zap.NewNop().Sugar(),
			manager:    mgr,
		}

		req, err := http.NewRequest("POST", "/library/tv/1/search", nil)
		require.NoError(t, err)

		req = mux.SetURLVars(req, map[string]string{"id": "1"})

		rr := httptest.NewRecorder()
		handler := s.SearchForSeries()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestServer_SearchForSeason(t *testing.T) {
	t.Run("invalid id", func(t *testing.T) {
		s := Server{baseLogger: zap.NewNop().Sugar()}

		req, err := http.NewRequest("POST", "/season/invalid/search", nil)
		require.NoError(t, err)

		req = mux.SetURLVars(req, map[string]string{"id": "invalid"})

		rr := httptest.NewRecorder()
		handler := s.SearchForSeason()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("season not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		store.EXPECT().GetSeason(gomock.Any(), gomock.Any()).Return(nil, storage.ErrNotFound)

		mgr := manager.New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		s := Server{
			baseLogger: zap.NewNop().Sugar(),
			manager:    mgr,
		}

		req, err := http.NewRequest("POST", "/season/1/search", nil)
		require.NoError(t, err)

		req = mux.SetURLVars(req, map[string]string{"id": "1"})

		rr := httptest.NewRecorder()
		handler := s.SearchForSeason()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("season not monitored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		store.EXPECT().GetSeason(gomock.Any(), gomock.Any()).Return(&storage.Season{
			Season: model.Season{ID: 1, Monitored: 0},
		}, nil)

		mgr := manager.New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		s := Server{
			baseLogger: zap.NewNop().Sugar(),
			manager:    mgr,
		}

		req, err := http.NewRequest("POST", "/season/1/search", nil)
		require.NoError(t, err)

		req = mux.SetURLVars(req, map[string]string{"id": "1"})

		rr := httptest.NewRecorder()
		handler := s.SearchForSeason()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestServer_SearchForEpisode(t *testing.T) {
	t.Run("invalid id", func(t *testing.T) {
		s := Server{baseLogger: zap.NewNop().Sugar()}

		req, err := http.NewRequest("POST", "/episode/invalid/search", nil)
		require.NoError(t, err)

		req = mux.SetURLVars(req, map[string]string{"id": "invalid"})

		rr := httptest.NewRecorder()
		handler := s.SearchForEpisode()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("episode not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		store.EXPECT().GetEpisode(gomock.Any(), gomock.Any()).Return(nil, storage.ErrNotFound)

		mgr := manager.New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		s := Server{
			baseLogger: zap.NewNop().Sugar(),
			manager:    mgr,
		}

		req, err := http.NewRequest("POST", "/episode/1/search", nil)
		require.NoError(t, err)

		req = mux.SetURLVars(req, map[string]string{"id": "1"})

		rr := httptest.NewRecorder()
		handler := s.SearchForEpisode()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("episode not monitored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		store.EXPECT().GetEpisode(gomock.Any(), gomock.Any()).Return(&storage.Episode{
			Episode: model.Episode{ID: 1, Monitored: 0},
		}, nil)

		mgr := manager.New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		s := Server{
			baseLogger: zap.NewNop().Sugar(),
			manager:    mgr,
		}

		req, err := http.NewRequest("POST", "/episode/1/search", nil)
		require.NoError(t, err)

		req = mux.SetURLVars(req, map[string]string{"id": "1"})

		rr := httptest.NewRecorder()
		handler := s.SearchForEpisode()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}
