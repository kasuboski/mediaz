package server

import (
	"net/http"
	"path/filepath"
)

// Healthz is an endpoint that can be used for probes
func (s Server) Healthz() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.respond(r, w, http.StatusOK, "ok")
	}
}

func (s Server) FileHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.StripPrefix("/static", s.fileServer).ServeHTTP(w, r)
	}
}

func (s Server) IndexHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(s.config.DistDir, "index.html"))
	}
}

// GetConfig returns the library configuration (non-sensitive data only)
func (s Server) GetConfig() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.respond(r, w, http.StatusOK, s.manager.GetConfigSummary())
	}
}

// GetLibraryStats returns library statistics
func (s Server) GetLibraryStats() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stats, err := s.manager.GetLibraryStats(r.Context())
		if err != nil {
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		s.respond(r, w, http.StatusOK, stats)
	}
}
