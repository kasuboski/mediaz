package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/kasuboski/mediaz/pkg/library"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/tmdb"
	"go.uber.org/zap"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

type GenericResponse struct {
	Error    *error `json:"error,omitempty"`
	Response any    `json:"response"`
}

type TMDBClientInterface tmdb.ClientInterface

// Server houses all dependencies for the media server to work such as loggers, clients, configurations, etc.
type Server struct {
	baseLogger *zap.SugaredLogger
	tmdb       TMDBClientInterface
	library    library.Library
}

// New creates a new media server
func New(tmbdClient *tmdb.Client, library library.Library, logger *zap.SugaredLogger) Server {
	return Server{
		tmdb:       tmbdClient,
		library:    library,
		baseLogger: logger,
	}
}

// Serve starts the http server and is a blocking call
func (s Server) Serve(port int) error {
	rtr := mux.NewRouter()
	rtr.Use(s.LogMiddleware())
	rtr.HandleFunc("/healthz", s.Healthz()).Methods(http.MethodGet)

	api := rtr.PathPrefix("/api").Subrouter()
	v1 := api.PathPrefix("/v1").Subrouter()
	v1.HandleFunc("/movies", s.ListMovies()).Methods(http.MethodGet)
	v1.HandleFunc("/tv", s.ListTVShows()).Methods(http.MethodGet)

	corsHandler := handlers.CORS(
		handlers.AllowedOrigins([]string{"*"}),
	)(rtr)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: corsHandler,
	}

	go func() {
		s.baseLogger.Info("serving... ", zap.Int("port", port))
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

		b, err := json.Marshal(response)
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("content-type", "application/json")
		_, _ = w.Write(b)
	}
}

// ListMovies lists movies on disk
func (s Server) ListMovies() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())
		movies, err := s.library.FindMovies(r.Context())
		if err != nil {
			log.Error("failed to list movies", zap.Error(err))
			http.Error(w, "failed to list movies", http.StatusInternalServerError)
			return
		}

		resp := GenericResponse{
			Response: movies,
		}

		b, err := json.Marshal(movies)
		if err != nil {
			log.Error("failed to marshal respponse", err, zap.Any("response", resp))
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		log.Info("returning movies", zap.Any("list", movies))
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write(b)
	}
}

// ListTVShows lists tv shows on disk
func (s Server) ListTVShows() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())
		episodes, err := s.library.FindEpisodes(r.Context())
		if err != nil {
			log.Error("failed to list shows", zap.Error(err))
			http.Error(w, "failed to list shows", http.StatusInternalServerError)
			return
		}

		resp := GenericResponse{
			Response: episodes,
		}

		b, err := json.Marshal(episodes)
		if err != nil {
			log.Error("failed to marshal respponse", err, zap.Any("response", resp))
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		log.Info("returning tv shows", zap.Any("list", episodes))
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write(b)
	}
}
