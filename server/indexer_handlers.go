package server

import (
	"errors"
	"net/http"
	"sort"

	"github.com/kasuboski/mediaz/pkg/indexer"
	"github.com/kasuboski/mediaz/pkg/manager"
	"github.com/kasuboski/mediaz/pkg/pagination"
	"github.com/kasuboski/mediaz/pkg/storage"
)

// ListIndexers lists all indexers
func (s Server) ListIndexers() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		params, err := ParsePaginationParams(r)
		if err != nil {
			s.respondError(r, w, http.StatusBadRequest, err)
			return
		}
		result, err := s.manager.ListIndexers(r.Context())
		if err != nil {
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		sort.Slice(result, func(i, j int) bool {
			if result[i].Name != result[j].Name {
				return result[i].Name < result[j].Name
			}
			return result[i].ID < result[j].ID
		})
		s.respond(r, w, http.StatusOK, pagination.Page(result, params))
	}
}

// CreateIndexer creates an indexer
func (s Server) CreateIndexer() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req indexerRequest
		if !s.decodeJSON(w, r, &req) {
			return
		}

		indexer, err := s.manager.AddIndexer(r.Context(), manager.AddIndexerRequest{Indexer: req.row()})
		if err != nil {
			if errors.Is(err, manager.ErrValidation) {
				s.respondError(r, w, http.StatusBadRequest, err)
				return
			}
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		s.respond(r, w, http.StatusCreated, indexer)
	}
}

// DeleteIndexer deletes an indexer by ID from the URL
func (s Server) DeleteIndexer() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.parseURLInt64(w, r, "id")
		if !ok {
			return
		}

		rowID := indexer.RowID(id)
		if err := s.manager.DeleteIndexer(r.Context(), manager.DeleteIndexerRequest{ID: &rowID}); err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				s.respondError(r, w, http.StatusNotFound, err)
				return
			}
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		s.respond(r, w, http.StatusOK, map[string]any{"id": id})
	}
}

// UpdateIndexer updates an indexer by ID from the URL
func (s Server) UpdateIndexer() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.parseURLInt64(w, r, "id")
		if !ok {
			return
		}

		var req manager.UpdateIndexerRequest
		if !s.decodeJSON(w, r, &req) {
			return
		}

		indexer, err := s.manager.UpdateIndexer(r.Context(), indexer.RowID(id), req)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				s.respondError(r, w, http.StatusNotFound, err)
				return
			}
			if errors.Is(err, manager.ErrValidation) {
				s.respondError(r, w, http.StatusBadRequest, err)
				return
			}
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		s.respond(r, w, http.StatusOK, indexer)
	}
}
