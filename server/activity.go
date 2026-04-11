package server

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

func (s Server) GetActiveActivity() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response, err := s.manager.GetActiveActivity(r.Context())
		if err != nil {
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		s.respond(r, w, http.StatusOK, response)
	}
}

func (s Server) GetRecentFailures() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hours := 24
		if hoursStr := r.URL.Query().Get("hours"); hoursStr != "" {
			parsed, err := strconv.Atoi(hoursStr)
			if err != nil || parsed < 1 {
				s.respondError(r, w, http.StatusBadRequest, fmt.Errorf("invalid hours parameter: must be positive integer"))
				return
			}
			hours = parsed
		}

		response, err := s.manager.GetRecentFailures(r.Context(), hours)
		if err != nil {
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		s.respond(r, w, http.StatusOK, response)
	}
}

func (s Server) GetActivityTimeline() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		days := 1
		if daysStr := r.URL.Query().Get("days"); daysStr != "" {
			parsed, err := strconv.Atoi(daysStr)
			if err != nil || parsed < 1 {
				s.respondError(r, w, http.StatusBadRequest, fmt.Errorf("invalid days parameter: must be positive integer"))
				return
			}
			days = parsed
		}

		params, err := ParsePaginationParams(r)
		if err != nil {
			s.respondError(r, w, http.StatusBadRequest, err)
			return
		}

		response, err := s.manager.GetActivityTimeline(r.Context(), days, params)
		if err != nil {
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		s.respond(r, w, http.StatusOK, response)
	}
}

func (s Server) GetEntityTransitionHistory() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)

		entityType := vars["entityType"]
		if entityType == "" {
			s.respondError(r, w, http.StatusBadRequest, fmt.Errorf("entityType is required"))
			return
		}

		entityID, ok := s.parseURLInt64(w, r, "entityId")
		if !ok {
			return
		}

		response, err := s.manager.GetEntityTransitionHistory(r.Context(), entityType, entityID)
		if err != nil {
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		s.respond(r, w, http.StatusOK, response)
	}
}
