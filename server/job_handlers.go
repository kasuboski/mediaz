package server

import (
	"net/http"

	"github.com/kasuboski/mediaz/pkg/manager"
)

// ListJobs lists jobs with optional pagination
func (s Server) ListJobs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		params, err := ParsePaginationParams(r)
		if err != nil {
			s.respondError(r, w, http.StatusBadRequest, err)
			return
		}

		jobs, err := s.manager.ListJobs(r.Context(), nil, nil, params)
		if err != nil {
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		s.respond(r, w, http.StatusOK, jobs)
	}
}

// GetJob gets a job by ID
func (s Server) GetJob() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.parseURLInt64(w, r, "id")
		if !ok {
			return
		}

		job, err := s.manager.GetJob(r.Context(), id)
		if err != nil {
			if isNotFound(err) {
				s.respondError(r, w, http.StatusNotFound, err)
				return
			}
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		s.respond(r, w, http.StatusOK, job)
	}
}

// CancelJob cancels a running job
func (s Server) CancelJob() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.parseURLInt64(w, r, "id")
		if !ok {
			return
		}

		job, err := s.manager.CancelJob(r.Context(), id)
		if err != nil {
			if isNotFound(err) {
				s.respondError(r, w, http.StatusNotFound, err)
				return
			}
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		s.respond(r, w, http.StatusOK, job)
	}
}

// CreateJob creates a new pending job
func (s Server) CreateJob() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req manager.TriggerJobRequest
		if !s.decodeJSON(w, r, &req) {
			return
		}

		job, err := s.manager.CreateJob(r.Context(), req)
		if err != nil {
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		s.respond(r, w, http.StatusCreated, job)
	}
}
