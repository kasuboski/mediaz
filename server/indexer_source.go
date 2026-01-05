package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/manager"
	"go.uber.org/zap"
)

func (s Server) ListIndexerSources() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())

		sources, err := s.manager.ListIndexerSources(r.Context())
		if err != nil {
			log.Error("failed to list indexer sources", zap.Error(err))
			writeErrorResponse(w, http.StatusInternalServerError, err)
			return
		}

		writeResponse(w, http.StatusOK, GenericResponse{Response: sources})
	}
}

func (s Server) CreateIndexerSource() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		var req manager.AddIndexerSourceRequest
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		source, err := s.manager.CreateIndexerSource(r.Context(), req)
		if err != nil {
			log.Error("failed to create indexer source", zap.Error(err))
			writeErrorResponse(w, http.StatusInternalServerError, err)
			return
		}

		writeResponse(w, http.StatusCreated, GenericResponse{Response: source})
	}
}

func (s Server) GetIndexerSource() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())
		vars := mux.Vars(r)

		id, err := strconv.ParseInt(vars["id"], 10, 64)
		if err != nil {
			http.Error(w, "invalid ID", http.StatusBadRequest)
			return
		}

		source, err := s.manager.GetIndexerSource(r.Context(), id)
		if err != nil {
			log.Error("failed to get indexer source", zap.Error(err))
			writeErrorResponse(w, http.StatusInternalServerError, err)
			return
		}

		writeResponse(w, http.StatusOK, GenericResponse{Response: source})
	}
}

func (s Server) UpdateIndexerSource() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())
		vars := mux.Vars(r)

		id, err := strconv.ParseInt(vars["id"], 10, 64)
		if err != nil {
			http.Error(w, "invalid ID", http.StatusBadRequest)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		var req manager.UpdateIndexerSourceRequest
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		source, err := s.manager.UpdateIndexerSource(r.Context(), id, req)
		if err != nil {
			log.Error("failed to update indexer source", zap.Error(err))
			writeErrorResponse(w, http.StatusInternalServerError, err)
			return
		}

		writeResponse(w, http.StatusOK, GenericResponse{Response: source})
	}
}

func (s Server) DeleteIndexerSource() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())
		vars := mux.Vars(r)

		id, err := strconv.ParseInt(vars["id"], 10, 64)
		if err != nil {
			http.Error(w, "invalid ID", http.StatusBadRequest)
			return
		}

		if err := s.manager.DeleteIndexerSource(r.Context(), id); err != nil {
			log.Error("failed to delete indexer source", zap.Error(err))
			writeErrorResponse(w, http.StatusInternalServerError, err)
			return
		}

		writeResponse(w, http.StatusOK, GenericResponse{Response: map[string]int64{"id": id}})
	}
}

func (s Server) TestIndexerSource() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		var req manager.AddIndexerSourceRequest
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if err := s.manager.TestIndexerSource(r.Context(), req); err != nil {
			log.Error("indexer source test failed", zap.Error(err))
			writeErrorResponse(w, http.StatusBadRequest, err)
			return
		}

		writeResponse(w, http.StatusOK, GenericResponse{
			Response: map[string]string{"message": "Connection successful"},
		})
	}
}

func (s Server) RefreshIndexerSource() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())
		vars := mux.Vars(r)

		id, err := strconv.ParseInt(vars["id"], 10, 64)
		if err != nil {
			http.Error(w, "invalid ID", http.StatusBadRequest)
			return
		}

		if err := s.manager.RefreshIndexerSource(r.Context(), id); err != nil {
			log.Error("failed to refresh indexer source", zap.Error(err))
			writeErrorResponse(w, http.StatusInternalServerError, err)
			return
		}

		writeResponse(w, http.StatusOK, GenericResponse{
			Response: map[string]string{"message": "Indexer source refreshed"},
		})
	}
}
