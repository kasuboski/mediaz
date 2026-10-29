package server

import (
	"fmt"
	"net/http"
	"strconv"
)

// SearchMovie searches for movie metadata via tmdb
func (s Server) SearchMovie() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		page, ok := s.discoveryPage(w, r)
		if !ok {
			return
		}

		result, err := s.manager.SearchMovie(r.Context(), query, page)
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
		page, ok := s.discoveryPage(w, r)
		if !ok {
			return
		}

		result, err := s.manager.SearchTV(r.Context(), query, page)
		if err != nil {
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		s.respond(r, w, http.StatusOK, result)
	}
}

func (s Server) discoveryPage(w http.ResponseWriter, r *http.Request) (int32, bool) {
	values, present := r.URL.Query()["page"]
	if !present {
		return 1, true
	}
	if len(values) != 1 {
		s.respondError(r, w, http.StatusBadRequest, fmt.Errorf("page must be specified once"))
		return 0, false
	}
	value, err := strconv.ParseInt(values[0], 10, 32)
	if err != nil || value <= 0 {
		s.respondError(r, w, http.StatusBadRequest, fmt.Errorf("page must be a positive 32-bit integer"))
		return 0, false
	}
	return int32(value), true
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
