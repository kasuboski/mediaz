package server

import (
	"errors"
	"net/http"

	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/manager"
	"github.com/kasuboski/mediaz/pkg/storage"
	"go.uber.org/zap"
)

// ListMovies lists movies on disk
func (s Server) ListMovies() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		movies, err := s.manager.ListMoviesInLibrary(r.Context())
		if err != nil {
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		s.respond(r, w, http.StatusOK, movies)
	}
}

// GetMovieDetailByTMDBID retrieves detailed information for a single movie by TMDB ID
func (s Server) GetMovieDetailByTMDBID() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.parseURLInt64(w, r, "tmdbID")
		if !ok {
			return
		}

		movieDetail, err := s.manager.GetMovieDetailByTMDBID(r.Context(), int(id))
		if err != nil {
			if isNotFound(err) {
				s.respondError(r, w, http.StatusNotFound, err)
				return
			}
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		s.respond(r, w, http.StatusOK, movieDetail)
	}
}

// AddMovieToLibrary adds a movie to the library
func (s Server) AddMovieToLibrary() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req manager.AddMovieRequest
		if !s.decodeJSON(w, r, &req) {
			return
		}

		release, err := s.manager.AddMovieToLibrary(r.Context(), req)
		if err != nil {
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		s.respond(r, w, http.StatusOK, release)
	}
}

// DeleteMovieFromLibrary removes a movie from the library with optional file deletion
func (s Server) DeleteMovieFromLibrary() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.parseURLInt64(w, r, "id")
		if !ok {
			return
		}

		deleteFiles := r.URL.Query().Get("deleteFiles") == "true"

		if err := s.manager.DeleteMovie(r.Context(), id, deleteFiles); err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				s.respondError(r, w, http.StatusNotFound, err)
				return
			}
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}

		s.respond(r, w, http.StatusOK, map[string]any{
			"id":           id,
			"message":      "Movie deleted",
			"filesDeleted": deleteFiles,
		})
	}
}

// UpdateMovieMonitored updates the monitoring status of a movie
func (s Server) UpdateMovieMonitored() http.HandlerFunc {
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

		movie, err := s.manager.UpdateMovieMonitored(r.Context(), id, req.Monitored)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				s.respondError(r, w, http.StatusNotFound, err)
				return
			}
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		s.respond(r, w, http.StatusOK, movie)
	}
}

// UpdateMovieQualityProfile updates the quality profile of a movie
func (s Server) UpdateMovieQualityProfile() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.parseURLInt64(w, r, "id")
		if !ok {
			return
		}

		var req struct {
			QualityProfileID int32 `json:"qualityProfileId"`
		}
		if !s.decodeJSON(w, r, &req) {
			return
		}

		movie, err := s.manager.UpdateMovieQualityProfile(r.Context(), id, req.QualityProfileID)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				s.respondError(r, w, http.StatusNotFound, err)
				return
			}
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		s.respond(r, w, http.StatusOK, movie)
	}
}

// RefreshMovieMetadata refreshes metadata for the given TMDB IDs
func (s Server) RefreshMovieMetadata() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())

		var req RefreshRequest
		if !s.decodeJSON(w, r, &req) {
			return
		}

		if err := s.manager.RefreshMovieMetadata(r.Context(), req.TmdbIDs...); err != nil {
			log.Error("failed to refresh movie metadata", zap.Error(err))
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}

		s.respond(r, w, http.StatusOK, "Movie metadata refresh completed")
	}
}
