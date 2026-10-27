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
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestServer_SearchForMovie(t *testing.T) {
	t.Run("invalid id", func(t *testing.T) {
		s := newTestServer()

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
		store.EXPECT().GetMovie(gomock.Any(), int64(1)).Return(sqlcdb.GetMovieByIDRow{}, storage.ErrNotFound)

		mgr := manager.New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		s := newTestServer(withManager(mgr))

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
		store.EXPECT().GetMovie(gomock.Any(), int64(1)).Return(sqlcdb.GetMovieByIDRow{Movie: sqlcdb.Movie{ID: 1, Monitored: 0}}, nil)

		mgr := manager.New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		s := newTestServer(withManager(mgr))

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
		s := newTestServer()

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
		store.EXPECT().GetSeriesByID(gomock.Any(), gomock.Any()).Return(sqlcdb.GetSeriesRow{}, storage.ErrNotFound)

		mgr := manager.New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		s := newTestServer(withManager(mgr))

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
		store.EXPECT().GetSeriesByID(gomock.Any(), gomock.Any()).Return(sqlcdb.GetSeriesRow{Series: sqlcdb.Series{ID: 1, Monitored: 0}}, nil)

		mgr := manager.New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		s := newTestServer(withManager(mgr))

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
		s := newTestServer()

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
		store.EXPECT().GetSeasonByID(gomock.Any(), gomock.Any()).Return(sqlcdb.GetSeasonByIDRow{}, storage.ErrNotFound)

		mgr := manager.New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		s := newTestServer(withManager(mgr))

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
		store.EXPECT().GetSeasonByID(gomock.Any(), gomock.Any()).Return(sqlcdb.GetSeasonByIDRow{Season: sqlcdb.Season{ID: 1, Monitored: 0}}, nil)

		mgr := manager.New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		s := newTestServer(withManager(mgr))

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
		s := newTestServer()

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
		store.EXPECT().GetEpisodeByID(gomock.Any(), gomock.Any()).Return(sqlcdb.GetEpisodeByIDRow{}, storage.ErrNotFound)

		mgr := manager.New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		s := newTestServer(withManager(mgr))

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
		store.EXPECT().GetEpisodeByID(gomock.Any(), gomock.Any()).Return(sqlcdb.GetEpisodeByIDRow{Episode: sqlcdb.Episode{ID: 1, Monitored: 0}}, nil)

		mgr := manager.New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		s := newTestServer(withManager(mgr))

		req, err := http.NewRequest("POST", "/episode/1/search", nil)
		require.NoError(t, err)

		req = mux.SetURLVars(req, map[string]string{"id": "1"})

		rr := httptest.NewRecorder()
		handler := s.SearchForEpisode()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}
