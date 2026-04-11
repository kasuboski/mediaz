package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestServer_Healthz(t *testing.T) {
	t.Run("healthz", func(t *testing.T) {
		s := Server{baseLogger: zap.NewNop().Sugar()}

		req, err := http.NewRequest("GET", "/healthz", nil)
		assert.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.Healthz()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		assert.Equal(t, "application/json", rr.Header().Get("content-type"))

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)

		assert.NoError(t, err)
		assert.Equal(t, "ok", response.Response)
	})
}
