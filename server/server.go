package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	tmdb "github.com/kasuboski/mediaz/pkg/client"
	"github.com/kasuboski/mediaz/pkg/logger"
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
}

// New creates a new media server
func New(tmbdClient *tmdb.Client, logger *zap.SugaredLogger) Server {
	return Server{
		baseLogger: logger,
		tmdb:       tmbdClient,
	}
}

// Serve starts the http server and is a blocking call
func (s Server) Serve(port int) error {
	rtr := mux.NewRouter()
	rtr.Use(s.LogMiddleware())
	rtr.HandleFunc("/healthz", s.Healthz()).Methods(http.MethodGet)

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

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	return srv.Shutdown(ctx)
}

func (s Server) Healthz() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())
		log.Info("health check")

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
