package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/kasuboski/mediaz/pkg/storage"
)

// parseURLInt64 extracts an int64 from gorilla/mux URL variables.
// Returns false after writing an error response if parsing fails.
func (s Server) parseURLInt64(w http.ResponseWriter, r *http.Request, key string) (int64, bool) {
	vars := mux.Vars(r)
	val, err := strconv.ParseInt(vars[key], 10, 64)
	if err != nil {
		s.respondError(r, w, http.StatusBadRequest, fmt.Errorf("invalid %s: must be integer", key))
		return 0, false
	}
	return val, true
}

// decodeJSON reads the request body, unmarshals into v, and validates struct tags.
// Returns false after writing an error response if reading, unmarshaling, or validation fails.
func (s Server) decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	b, err := io.ReadAll(r.Body)
	if err != nil {
		s.respondError(r, w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return false
	}

	if err := json.Unmarshal(b, v); err != nil {
		s.respondError(r, w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return false
	}

	if s.validate != nil {
		if err := s.validate.Struct(v); err != nil {
			s.respondError(r, w, http.StatusBadRequest, err)
			return false
		}
	}

	return true
}

// respond writes a JSON response. Logs any write errors using the request-scoped logger.
func (s Server) respond(r *http.Request, w http.ResponseWriter, status int, data any) {
	s.logWriteError(r, s.writeResponse(w, status, GenericResponse{Response: data}))
}

// respondError writes a JSON error response. Logs any write errors using the request-scoped logger.
func (s Server) respondError(r *http.Request, w http.ResponseWriter, status int, err error) {
	s.logWriteError(r, s.writeErrorResponse(w, status, err))
}

// isNotFound returns true if the error is a storage not-found error.
func isNotFound(err error) bool {
	return errors.Is(err, storage.ErrNotFound)
}
