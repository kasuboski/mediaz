package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/manager"
	"go.uber.org/zap"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

type GenericResponse struct {
	Error    error `json:"error,omitempty"`
	Response any   `json:"response"`
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

func writeErrorResponse(w http.ResponseWriter, status int, err error) error {
	return writeResponse(w, status, GenericResponse{
		Error: err,
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

	v1.HandleFunc("/library/movies", s.ListMovies()).Methods(http.MethodGet)
	v1.HandleFunc("/library/movies", s.AddMovieToLibrary()).Methods(http.MethodPost)

	v1.HandleFunc("/library/tv", s.ListTVShows()).Methods(http.MethodGet)
	// TODO: add method to add a show to your library
	// v1.HandleFunc("/library/tv", s.AddShowToLibrary()).Methods(http.MethodPost)

	v1.HandleFunc("/discover/movie", s.SearchMovie()).Methods(http.MethodGet)
	v1.HandleFunc("/discover/tv", s.SearchTV()).Methods(http.MethodGet)

	v1.HandleFunc("/indexers", s.ListIndexers()).Methods(http.MethodGet)
	v1.HandleFunc("/indexers", s.CreateIndexer()).Methods(http.MethodPost)
	v1.HandleFunc("/indexers", s.DeleteIndexer()).Methods(http.MethodDelete)

	v1.HandleFunc("/download/clients", s.ListDownloadClients()).Methods(http.MethodGet)
	v1.HandleFunc("/download/clients/{id}", s.GetDownloadClient()).Methods(http.MethodGet)
	v1.HandleFunc("/download/clients", s.CreateDownloadClient()).Methods(http.MethodPost)
	v1.HandleFunc("/download/clients/{id}", s.DeleteDownloadClient()).Methods(http.MethodDelete)

	v1.HandleFunc("/quality/definitions", s.ListIndexers()).Methods(http.MethodGet)
	v1.HandleFunc("/quality/definitions", s.CreateIndexer()).Methods(http.MethodPost)
	v1.HandleFunc("/quality/definitions", s.DeleteIndexer()).Methods(http.MethodDelete)

	v1.HandleFunc("/quality/profiles/{id}", s.GetQualityProfile()).Methods(http.MethodGet)
	v1.HandleFunc("/quality/profiles", s.ListQualityProfiles()).Methods(http.MethodGet)

	corsHandler := handlers.CORS(
		handlers.AllowedOrigins([]string{"*"}),
	)(rtr)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: corsHandler,
	}

	go func() {
		s.baseLogger.Infow("serving...", "port", port)
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
		movies, err := s.manager.ListMoviesInLibrary(r.Context())
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
		episodes, err := s.manager.ListShowsInLibrary(r.Context())
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

// ListIndexers lists all indexers
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

// CreateIndexer creates an indexer
func (s Server) CreateIndexer() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())
		b, err := io.ReadAll(r.Body)
		if err != nil {
			log.Debug("invalid request body", zap.Error(err))
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		var request manager.AddIndexerRequest
		err = json.Unmarshal(b, &request)
		if err != nil {
			log.Debug("invalid request body", zap.ByteString("body", b))
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		log.Debug("adding indexer", zap.Any("request", request))
		indexer, err := s.manager.AddIndexer(r.Context(), request)
		if err != nil {
			log.Debug("failed to create indexer", zap.Error(err))
			writeErrorResponse(w, http.StatusInternalServerError, err)
			return
		}

		log.Debug("succesfully added indexer")
		writeResponse(w, http.StatusCreated, GenericResponse{
			Response: indexer,
		})
	}
}

// DeleteIndexer deletes an indexer
func (s Server) DeleteIndexer() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())
		b, err := io.ReadAll(r.Body)
		if err != nil {
			log.Debug("invalid request body", zap.Error(err))
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		var request manager.DeleteIndexerRequest
		err = json.Unmarshal(b, &request)
		if err != nil {
			log.Debug("invalid request body", zap.ByteString("body", b))
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		log.Debug("adding indexer", zap.Any("request", request))
		err = s.manager.DeleteIndexer(r.Context(), request)
		if err != nil {
			log.Debug("failed to delete indexer", zap.Error(err))
			writeErrorResponse(w, http.StatusInternalServerError, err)
			return
		}

		log.Debug("succesfully deleted indexer")
		writeResponse(w, http.StatusOK, GenericResponse{
			Response: request,
		})
	}
}

// ListQualityDefinitions lists all stored quality definitions
func (s Server) ListQualityDefinitions() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())
		result, err := s.manager.ListQualityDefinitions(r.Context())
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

// CreateQualityDefinition creates a quality definition
func (s Server) CreateQualityDefinition() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())
		b, err := io.ReadAll(r.Body)
		if err != nil {
			log.Debug("invalid request body", zap.Error(err))
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		var request manager.AddQualityDefinitionRequest
		err = json.Unmarshal(b, &request)
		if err != nil {
			log.Debug("invalid request body", zap.ByteString("body", b))
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		log.Debug("adding indexer", zap.Any("request", request))
		definition, err := s.manager.AddQualityDefinition(r.Context(), request)
		if err != nil {
			log.Debug("failed to create quality definition", zap.Error(err))
			writeErrorResponse(w, http.StatusInternalServerError, err)
			return
		}

		log.Debug("succesfully added quality definition")
		writeResponse(w, http.StatusCreated, GenericResponse{
			Response: definition,
		})
	}
}

// DeleteQualityDefinition deletes a quality definition
func (s Server) DeleteQualityDefinition() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())
		b, err := io.ReadAll(r.Body)
		if err != nil {
			log.Debug("invalid request body", zap.Error(err))
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		var request manager.DeleteQualityDefinitionRequest
		err = json.Unmarshal(b, &request)
		if err != nil {
			log.Debug("invalid request body", zap.ByteString("body", b))
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		log.Debug("adding indexer", zap.Any("request", request))
		err = s.manager.DeleteQualityDefinition(r.Context(), request)
		if err != nil {
			log.Debug("failed to delete quality definition", zap.Error(err))
			writeErrorResponse(w, http.StatusInternalServerError, err)
			return
		}

		log.Debug("succesfully deleted quality definition")
		writeResponse(w, http.StatusOK, GenericResponse{
			Response: request,
		})
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

// SearchTV searches for movie metadata via tmdb
func (s Server) SearchTV() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())
		qps := r.URL.Query()
		query := qps.Get("query")

		result, err := s.manager.SearchTV(r.Context(), query)
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

// AddMovieToLibrary adds a movie to the library
func (s Server) AddMovieToLibrary() http.HandlerFunc {
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
			log.Debug("invalid request body", zap.String("body", string(b)))
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		release, err := s.manager.AddMovieToLibrary(r.Context(), request)
		if err != nil {
			log.Error("couldn't add a movie", zap.Error(err))
			writeErrorResponse(w, http.StatusInternalServerError, err)
			return
		}

		err = writeResponse(w, http.StatusOK, GenericResponse{Response: release})
		if err != nil {
			log.Error("failed to write response", zap.Error(err))
			return
		}
	}
}

// GetQualityProfile gets a quality profile given an id
func (s Server) GetQualityProfile() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())
		vars := mux.Vars(r)
		idVar := vars["id"]

		id, err := strconv.ParseInt(idVar, 10, 64)
		if err != nil {
			http.Error(w, "Invalid ID format", http.StatusBadRequest)
			return
		}

		profile, err := s.manager.GetQualityProfile(r.Context(), id)
		if err != nil {
			log.Error("failed to get quality profile", zap.Error(err))
			http.Error(w, "failed to get quality profile", http.StatusInternalServerError)
			return
		}

		resp := GenericResponse{
			Response: profile,
		}

		writeResponse(w, http.StatusOK, resp)
	}
}

// ListQualityProfiles lists all quality profiles
func (s Server) ListQualityProfiles() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())
		profile, err := s.manager.ListQualityProfiles(r.Context())
		if err != nil {
			log.Errorw("failed to list quality profile", zap.Error(err))
			http.Error(w, "failed to list quality profile", http.StatusInternalServerError)
			return
		}

		resp := GenericResponse{
			Response: profile,
		}

		writeResponse(w, http.StatusOK, resp)
	}
}

// CreateDownloadClient stores a download client
func (s Server) CreateDownloadClient() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())

		b, err := io.ReadAll(r.Body)
		if err != nil {
			log.Debug("invalid request body", zap.Error(err))
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		var request manager.AddDownloadClientRequest
		err = json.Unmarshal(b, &request)
		if err != nil {
			log.Debug("invalid request body", zap.String("body", string(b)))
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		result, err := s.manager.CreateDownloadClient(r.Context(), request)
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

// GetDownloadClient gets a download client
func (s Server) GetDownloadClient() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())
		vars := mux.Vars(r)
		idVar := vars["id"]

		id, err := strconv.ParseInt(idVar, 10, 64)
		if err != nil {
			log.Debug("invalid id provided", zap.Error(err), zap.Any("id", id))
			http.Error(w, "Invalid ID format", http.StatusBadRequest)
			return
		}

		downloadClient, err := s.manager.GetDownloadClient(r.Context(), id)
		if err != nil {
			log.Debug("failed to get download client", zap.Error(err))
			writeErrorResponse(w, http.StatusInternalServerError, err)
			return
		}

		writeResponse(w, http.StatusCreated, GenericResponse{
			Response: downloadClient,
		})
	}
}

// DeleDownloadClient deletes a download client from storage
func (s Server) DeleteDownloadClient() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())
		vars := mux.Vars(r)
		idVar := vars["id"]

		id, err := strconv.ParseInt(idVar, 10, 64)
		if err != nil {
			log.Debug("invalid id provided", zap.Error(err), zap.Any("id", id))
			http.Error(w, "Invalid ID format", http.StatusBadRequest)
			return
		}

		err = s.manager.DeleteDownloadClient(r.Context(), id)
		if err != nil {
			log.Debug("failed to get download client", zap.Error(err))
			writeErrorResponse(w, http.StatusInternalServerError, err)
			return
		}

		writeResponse(w, http.StatusOK, GenericResponse{
			Response: id,
		})
	}
}

// ListDownloadClients lists all download client
func (s Server) ListDownloadClients() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())

		downloadClient, err := s.manager.ListDownloadClients(r.Context())
		if err != nil {
			log.Debug("failed to get download client", zap.Error(err))
			writeErrorResponse(w, http.StatusInternalServerError, err)
			return
		}

		writeResponse(w, http.StatusCreated, GenericResponse{
			Response: downloadClient,
		})
	}
}
