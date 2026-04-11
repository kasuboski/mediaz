package server

import (
	"net/http"

	"github.com/kasuboski/mediaz/pkg/manager"
)

func (s Server) ListIndexerSources() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sources, err := s.manager.ListIndexerSources(r.Context())
		if err != nil {
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		s.respond(r, w, http.StatusOK, sources)
	}
}

func (s Server) CreateIndexerSource() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req manager.AddIndexerSourceRequest
		if !s.decodeJSON(w, r, &req) {
			return
		}

		source, err := s.manager.CreateIndexerSource(r.Context(), req)
		if err != nil {
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		s.respond(r, w, http.StatusCreated, source)
	}
}

func (s Server) GetIndexerSource() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.parseURLInt64(w, r, "id")
		if !ok {
			return
		}

		source, err := s.manager.GetIndexerSource(r.Context(), id)
		if err != nil {
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		s.respond(r, w, http.StatusOK, source)
	}
}

func (s Server) UpdateIndexerSource() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.parseURLInt64(w, r, "id")
		if !ok {
			return
		}

		var req manager.UpdateIndexerSourceRequest
		if !s.decodeJSON(w, r, &req) {
			return
		}

		source, err := s.manager.UpdateIndexerSource(r.Context(), id, req)
		if err != nil {
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		s.respond(r, w, http.StatusOK, source)
	}
}

func (s Server) DeleteIndexerSource() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.parseURLInt64(w, r, "id")
		if !ok {
			return
		}

		if err := s.manager.DeleteIndexerSource(r.Context(), id); err != nil {
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		s.respond(r, w, http.StatusOK, map[string]int64{"id": id})
	}
}

func (s Server) TestIndexerSource() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req manager.AddIndexerSourceRequest
		if !s.decodeJSON(w, r, &req) {
			return
		}

		if err := s.manager.TestIndexerSource(r.Context(), req); err != nil {
			s.respondError(r, w, http.StatusBadRequest, err)
			return
		}

		s.respond(r, w, http.StatusOK, map[string]string{"message": "Connection successful"})
	}
}

func (s Server) RefreshIndexerSource() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.parseURLInt64(w, r, "id")
		if !ok {
			return
		}

		if err := s.manager.RefreshIndexerSource(r.Context(), id); err != nil {
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}

		s.respond(r, w, http.StatusOK, map[string]string{"message": "Indexer source refreshed"})
	}
}
