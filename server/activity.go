package server

import (
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/kasuboski/mediaz/pkg/logger"
	"go.uber.org/zap"
)

func (s Server) GetActiveActivity() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())

		response, err := s.manager.GetActiveActivity(r.Context())
		if err != nil {
			log.Error("failed to get active activity", zap.Error(err))
			writeErrorResponse(w, http.StatusInternalServerError, err)
			return
		}

		writeResponse(w, http.StatusOK, GenericResponse{Response: response})
	}
}

func (s Server) GetRecentFailures() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())

		hours := 24
		if hoursStr := r.URL.Query().Get("hours"); hoursStr != "" {
			parsed, err := strconv.Atoi(hoursStr)
			if err != nil || parsed < 1 {
				http.Error(w, "invalid hours parameter: must be positive integer", http.StatusBadRequest)
				return
			}
			hours = parsed
		}

		response, err := s.manager.GetRecentFailures(r.Context(), hours)
		if err != nil {
			log.Error("failed to get recent failures", zap.Error(err), zap.Int("hours", hours))
			writeErrorResponse(w, http.StatusInternalServerError, err)
			return
		}

		writeResponse(w, http.StatusOK, GenericResponse{Response: response})
	}
}

func (s Server) GetActivityTimeline() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())

		days := 1
		if daysStr := r.URL.Query().Get("days"); daysStr != "" {
			parsed, err := strconv.Atoi(daysStr)
			if err != nil || parsed < 1 {
				http.Error(w, "invalid days parameter: must be positive integer", http.StatusBadRequest)
				return
			}
			days = parsed
		}

		response, err := s.manager.GetActivityTimeline(r.Context(), days)
		if err != nil {
			log.Error("failed to get activity timeline", zap.Error(err), zap.Int("days", days))
			writeErrorResponse(w, http.StatusInternalServerError, err)
			return
		}

		writeResponse(w, http.StatusOK, GenericResponse{Response: response})
	}
}

func (s Server) GetEntityTransitionHistory() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())
		vars := mux.Vars(r)

		entityType := vars["entityType"]
		if entityType == "" {
			http.Error(w, "entityType is required", http.StatusBadRequest)
			return
		}

		entityIDStr := vars["entityId"]
		entityID, err := strconv.ParseInt(entityIDStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid entityId format", http.StatusBadRequest)
			return
		}

		response, err := s.manager.GetEntityTransitionHistory(r.Context(), entityType, entityID)
		if err != nil {
			log.Error("failed to get entity transition history", zap.Error(err), zap.String("entityType", entityType), zap.Int64("entityID", entityID))
			writeErrorResponse(w, http.StatusInternalServerError, err)
			return
		}

		writeResponse(w, http.StatusOK, GenericResponse{Response: response})
	}
}
