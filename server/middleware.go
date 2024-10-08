package server

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/kasuboski/mediaz/pkg/logger"
	"go.uber.org/zap"
)

// LogMiddleware adds a logger to the request context with logging fields specific to the request
func (s Server) LogMiddleware() mux.MiddlewareFunc {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log := s.baseLogger.With(zap.String("path", r.URL.Path)).With(zap.String("id", uuid.New().String()))
			h.ServeHTTP(w, r.WithContext(logger.WithCtx(r.Context(), log)))
		})
	}
}
