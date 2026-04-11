package server

import (
	"net/http"
)

// SearchMovie searches for movie metadata via tmdb
func (s Server) SearchMovie() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")

		result, err := s.manager.SearchMovie(r.Context(), query)
		if err != nil {
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		s.respond(r, w, http.StatusOK, result)
	}
}

// SearchTV searches for TV metadata via tmdb
func (s Server) SearchTV() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")

		result, err := s.manager.SearchTV(r.Context(), query)
		if err != nil {
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		s.respond(r, w, http.StatusOK, result)
	}
}

// SearchForMovie triggers a search for a movie by library ID
func (s Server) SearchForMovie() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.parseURLInt64(w, r, "id")
		if !ok {
			return
		}

		err := s.manager.SearchForMovie(r.Context(), id)
		if err != nil {
			if isNotFound(err) {
				s.respondError(r, w, http.StatusNotFound, err)
				return
			}
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}

		w.WriteHeader(http.StatusAccepted)
	}
}

// SearchForSeries triggers a search for a series by library ID
func (s Server) SearchForSeries() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.parseURLInt64(w, r, "id")
		if !ok {
			return
		}

		err := s.manager.SearchForSeries(r.Context(), id)
		if err != nil {
			if isNotFound(err) {
				s.respondError(r, w, http.StatusNotFound, err)
				return
			}
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}
}

// SearchForSeason triggers a search for a season by library ID
func (s Server) SearchForSeason() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.parseURLInt64(w, r, "id")
		if !ok {
			return
		}

		err := s.manager.SearchForSeason(r.Context(), id)
		if err != nil {
			if isNotFound(err) {
				s.respondError(r, w, http.StatusNotFound, err)
				return
			}
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}
}

// SearchForEpisode triggers a search for an episode by library ID
func (s Server) SearchForEpisode() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.parseURLInt64(w, r, "id")
		if !ok {
			return
		}

		err := s.manager.SearchForEpisode(r.Context(), id)
		if err != nil {
			if isNotFound(err) {
				s.respondError(r, w, http.StatusNotFound, err)
				return
			}
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}
}
