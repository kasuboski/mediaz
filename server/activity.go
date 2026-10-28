package server

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/kasuboski/mediaz/pkg/manager"
	"github.com/kasuboski/mediaz/pkg/pagination"
)

func parseActivePaginationParams(r *http.Request) (manager.ActiveActivityParams, error) {
	query := r.URL.Query()
	shared := r.Clone(r.Context())
	shared.URL.RawQuery = query.Encode()
	base, err := ParsePaginationParams(shared)
	if err != nil {
		return manager.ActiveActivityParams{}, err
	}
	parseNamed := func(name string) (pagination.Params, error) {
		values, ok := query[name+"Page"]
		if !ok {
			return base, nil
		}
		clone := r.Clone(r.Context())
		q := clone.URL.Query()
		q.Del("page")
		q["page"] = append([]string(nil), values...)
		clone.URL.RawQuery = q.Encode()
		return ParsePaginationParams(clone)
	}
	movies, err := parseNamed("movies")
	if err != nil {
		return manager.ActiveActivityParams{}, err
	}
	series, err := parseNamed("series")
	if err != nil {
		return manager.ActiveActivityParams{}, err
	}
	jobs, err := parseNamed("jobs")
	if err != nil {
		return manager.ActiveActivityParams{}, err
	}
	return manager.ActiveActivityParams{Movies: movies, Series: series, Jobs: jobs}, nil
}

func (s Server) GetActiveActivity() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		params, err := parseActivePaginationParams(r)
		if err != nil {
			s.respondError(r, w, http.StatusBadRequest, err)
			return
		}
		response, err := s.manager.GetActiveActivity(r.Context(), params)
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

		params, err := ParsePaginationParams(r)
		if err != nil {
			s.respondError(r, w, http.StatusBadRequest, err)
			return
		}
		response, err := s.manager.GetRecentFailures(r.Context(), hours, params)
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

		if entityType != "movie" && entityType != "series" && entityType != "season" && entityType != "episode" && entityType != "job" {
			s.respondError(r, w, http.StatusBadRequest, fmt.Errorf("unsupported entity type: %s", entityType))
			return
		}
		params, err := ParsePaginationParams(r)
		if err != nil {
			s.respondError(r, w, http.StatusBadRequest, err)
			return
		}

		entityID, ok := s.parseURLInt64(w, r, "entityId")
		if !ok {
			return
		}

		response, err := s.manager.GetEntityTransitionHistory(r.Context(), entityType, entityID, params)
		if err != nil {
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		s.respond(r, w, http.StatusOK, response)
	}
}
