package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/manager"
	"go.uber.org/zap"
)

// GenericResponse is the standard JSON envelope for all API responses.
type GenericResponse struct {
	Error    string `json:"error,omitempty"`
	Response any    `json:"response"`
}

// RefreshRequest is the request body for refresh endpoints.
type RefreshRequest struct {
	TmdbIDs []int `json:"tmdbIds"`
}

// Server houses all dependencies for the media server to work such as loggers, clients, configurations, etc.
type Server struct {
	baseLogger *zap.SugaredLogger
	manager    manager.MediaManager
	config     config.Server
	fileServer http.Handler
	validate   *validator.Validate
}

// New creates a new media server
func New(logger *zap.SugaredLogger, manager manager.MediaManager, config config.Server) Server {
	return Server{
		baseLogger: logger,
		manager:    manager,
		config:     config,
		validate:   validator.New(validator.WithRequiredStructEnabled()),
	}
}

func (s Server) writeErrorResponse(w http.ResponseWriter, status int, err error) error {
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	return s.writeResponse(w, status, GenericResponse{
		Error: errMsg,
	})
}

// requestLogger returns the request-scoped logger if available, otherwise the base logger.
func (s Server) requestLogger(r *http.Request) *zap.SugaredLogger {
	if r != nil {
		if l := logger.FromCtx(r.Context()); l != nil {
			return l
		}
	}
	return s.baseLogger
}

// NOTE: writeResponse uses baseLogger directly because the caller (logWriteError)
// already handles logging with request context. Keep this method simple.
func (s Server) writeResponse(w http.ResponseWriter, status int, body any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}

	w.Header().Set("content-type", "application/json")
	if status != http.StatusOK {
		w.WriteHeader(status)
	}

	_, err = w.Write(b)
	return err
}

func (s Server) logWriteError(r *http.Request, err error) {
	if err != nil {
		s.requestLogger(r).Errorw("failed to write response", zap.Error(err))
	}
}

// Serve starts the http server and is a blocking call.
// It returns immediately if the server fails to bind the port.
func (s *Server) Serve(port int) error {
	if _, err := os.Stat(s.config.DistDir); os.IsNotExist(err) {
		return fmt.Errorf("static file directory does not exist: %s", s.config.DistDir)
	}
	s.fileServer = http.FileServer(http.Dir(s.config.DistDir))

	rtr := mux.NewRouter()
	rtr.Use(s.LogMiddleware())
	s.registerRoutes(rtr)

	corsHandler := handlers.CORS(
		handlers.AllowedOrigins([]string{"*"}),
		handlers.AllowedMethods([]string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}),
		handlers.AllowedHeaders([]string{"Content-Type", "Authorization"}),
		handlers.ExposedHeaders([]string{"Content-Length"}),
		handlers.AllowCredentials(),
		handlers.MaxAge(3600),
	)(rtr)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: corsHandler,
	}

	serveErr := make(chan error, 1)
	go func() {
		s.baseLogger.Infow("serving...", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serveErr <- err
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	select {
	case err := <-serveErr:
		return fmt.Errorf("server failed to start: %w", err)
	case <-c:
		// graceful shutdown
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	return srv.Shutdown(ctx)
}
