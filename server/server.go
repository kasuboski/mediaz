package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	tmdb "github.com/kasuboski/mediaz/pkg/client"

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
	tmdb TMDBClientInterface
}

// New creates a new media server
func New(tmbdClient *tmdb.Client) Server {
	return Server{
		tmdb: tmbdClient,
	}
}

// Serve starts the http server and is a blocking call
func (s Server) New(port int) *http.Server {
	rtr := mux.NewRouter()
	rtr.HandleFunc("/healthz", s.Healthz()).Methods(http.MethodGet)

	corsHandler := handlers.CORS(
		handlers.AllowedOrigins([]string{"*"}),
	)(rtr)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: corsHandler,
	}

	s.http = srv

	return srv.ListenAndServe()
}

func (s Server) Healthz() http.HandlerFunc {
	response := GenericResponse{
		Response: "ok",
	}
	return func(w http.ResponseWriter, r *http.Request) {
		b, err := json.Marshal(response)
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("content-type", "application/json")
		_, _ = w.Write(b)
	}
}
