package server

import (
	"errors"
	"net/http"

	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/manager"
	"github.com/kasuboski/mediaz/pkg/storage"
	"go.uber.org/zap"
)

// ListTVShows lists tv shows on disk
func (s Server) ListTVShows() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		shows, err := s.manager.ListShowsInLibrary(r.Context())
		if err != nil {
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		s.respond(r, w, http.StatusOK, shows)
	}
}

// GetTVDetailByTMDBID retrieves detailed information for a single TV show by TMDB ID
func (s Server) GetTVDetailByTMDBID() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.parseURLInt64(w, r, "tmdbID")
		if !ok {
			return
		}

		tvDetail, err := s.manager.GetTVDetailByTMDBID(r.Context(), int(id))
		if err != nil {
			if isNotFound(err) {
				s.respondError(r, w, http.StatusNotFound, err)
				return
			}
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		s.respond(r, w, http.StatusOK, tvDetail)
	}
}

// AddSeriesToLibrary adds a series to the library
func (s Server) AddSeriesToLibrary() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req manager.AddSeriesRequest
		if !s.decodeJSON(w, r, &req) {
			return
		}

		release, err := s.manager.AddSeriesToLibrary(r.Context(), req)
		if err != nil {
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		s.respond(r, w, http.StatusOK, release)
	}
}

// DeleteSeriesFromLibrary removes a series from the library with optional file deletion
func (s Server) DeleteSeriesFromLibrary() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.parseURLInt64(w, r, "id")
		if !ok {
			return
		}

		deleteDirectory := r.URL.Query().Get("deleteDirectory") == "true"

		if err := s.manager.DeleteSeries(r.Context(), id, deleteDirectory); err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				s.respondError(r, w, http.StatusNotFound, err)
				return
			}
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}

		s.respond(r, w, http.StatusOK, map[string]any{
			"id":               id,
			"message":          "Series deleted",
			"directoryDeleted": deleteDirectory,
		})
	}
}

// UpdateSeriesMonitored updates the monitoring status of a series
func (s Server) UpdateSeriesMonitored() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.parseURLInt64(w, r, "id")
		if !ok {
			return
		}

		var req struct {
			Monitored bool `json:"monitored"`
		}
		if !s.decodeJSON(w, r, &req) {
			return
		}

		series, err := s.manager.UpdateSeriesMonitored(r.Context(), id, req.Monitored)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				s.respondError(r, w, http.StatusNotFound, err)
				return
			}
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		s.respond(r, w, http.StatusOK, series)
	}
}

// RefreshSeriesMetadata refreshes metadata for the given TMDB IDs
func (s Server) RefreshSeriesMetadata() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())

		var req RefreshRequest
		if !s.decodeJSON(w, r, &req) {
			return
		}

		if err := s.manager.RefreshSeriesMetadata(r.Context(), req.TmdbIDs...); err != nil {
			log.Error("failed to refresh series metadata", zap.Error(err))
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}

		s.respond(r, w, http.StatusOK, "Series metadata refresh completed")
	}
}
