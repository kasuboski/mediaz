package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/manager"
	"go.uber.org/zap"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

type GenericResponse struct {
	Error    *error `json:"error,omitempty"`
	Response any    `json:"response"`
}

// Server houses all dependencies for the media server to work such as loggers, clients, configurations, etc.
type Server struct {
	baseLogger *zap.SugaredLogger
	manager    manager.MediaManager
}

// New creates a new media server
func New(logger *zap.SugaredLogger, manager manager.MediaManager) Server {
	return Server{
		baseLogger: logger,
		manager:    manager,
	}
}

func writeGenericResponse(w http.ResponseWriter, status int) error {
	return writeResponse(w, status, GenericResponse{})
}

func writeErrorResponse(w http.ResponseWriter, status int, err error) error {
	return writeResponse(w, status, GenericResponse{
		Error: &err,
	})
}

func writeResponse(w http.ResponseWriter, status int, body any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}

	w.Header().Set("content-type", "application/json")
	if status != http.StatusOK {
		w.WriteHeader(status)
	}

	w.Write(b)
	return nil
}

// Serve starts the http server and is a blocking call
func (s Server) Serve(port int) error {
	rtr := mux.NewRouter()
	rtr.Use(s.LogMiddleware())
	rtr.HandleFunc("/healthz", s.Healthz()).Methods(http.MethodGet)

	api := rtr.PathPrefix("/api").Subrouter()

	v1 := api.PathPrefix("/v1").Subrouter()

	v1.HandleFunc("/movies", s.ListMovies()).Methods(http.MethodGet)
	v1.HandleFunc("/movies", s.CreateMovie()).Methods(http.MethodPost)

	v1.HandleFunc("/tv", s.ListTVShows()).Methods(http.MethodGet)

	v1.HandleFunc("/tmdb", s.SearchMovie()).Methods(http.MethodGet)
	v1.HandleFunc("/indexers", s.ListIndexers()).Methods(http.MethodGet)

	corsHandler := handlers.CORS(
		handlers.AllowedOrigins([]string{"*"}),
	)(rtr)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: corsHandler,
	}

	go func() {
		s.baseLogger.Info("serving...", zap.Int("port", port))
		if err := srv.ListenAndServe(); err != nil {
			s.baseLogger.Error(err.Error())
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	return srv.Shutdown(ctx)
}

// Healthz is an endpoint that can be used for probes
func (s Server) Healthz() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := GenericResponse{
			Response: "ok",
		}
		writeResponse(w, http.StatusOK, response)
	}
}

// ListMovies lists movies on disk
func (s Server) ListMovies() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())
		movies, err := s.manager.ListMoviesOnDisk(r.Context())
		if err != nil {
			log.Error("failed to list movies", zap.Error(err))
			http.Error(w, "failed to list movies", http.StatusInternalServerError)
			return
		}

		resp := GenericResponse{
			Response: movies,
		}

		writeResponse(w, http.StatusOK, resp)
	}
}

// ListTVShows lists tv shows on disk
func (s Server) ListTVShows() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		log := logger.FromCtx(r.Context())
		episodes, err := s.manager.ListEpisodesOnDisk(r.Context())
		if err != nil {
			log.Error("failed to list shows", zap.Error(err))
			http.Error(w, "failed to list shows", http.StatusInternalServerError)
			return
		}

		resp := GenericResponse{
			Response: episodes,
		}

		writeResponse(w, http.StatusOK, resp)
	}
}

func (s Server) ListIndexers() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())
		result, err := s.manager.ListIndexers(r.Context())
		if err != nil {
			writeErrorResponse(w, http.StatusOK, err)
			return
		}

		err = writeResponse(w, http.StatusOK, GenericResponse{Response: result})
		if err != nil {
			log.Error("failed to write response", zap.Error(err))
			return
		}
	}
}

// SearchMovie searches for movie metadata via tmdb
func (s Server) SearchMovie() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())
		qps := r.URL.Query()
		query := qps.Get("query")

		result, err := s.manager.SearchMovie(r.Context(), query)
		if err != nil {
			writeErrorResponse(w, http.StatusOK, err)
			return
		}

		err = writeResponse(w, http.StatusOK, GenericResponse{Response: result})
		if err != nil {
			log.Error("failed to write response", zap.Error(err))
			return
		}
	}
}

func (s Server) CreateMovie() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())
		b, err := io.ReadAll(r.Body)
		if err != nil {
			log.Debug("invalid request body", zap.Error(err))
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		var request manager.AddMovieRequest
		err = json.Unmarshal(b, &request)
		if err != nil {
			log.Debug("invalid request body", zap.ByteString("body", b))
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		err = s.manager.AddMovie(r.Context(), request)
		if err != nil {
			writeErrorResponse(w, http.StatusInternalServerError, err)
			return
		}
	}
}
