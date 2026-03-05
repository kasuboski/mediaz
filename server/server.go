package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"time"

	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/manager"
	"github.com/kasuboski/mediaz/pkg/storage"
	"go.uber.org/zap"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

type GenericResponse struct {
	Error    string `json:"error,omitempty"`
	Response any    `json:"response"`
}

type RefreshRequest struct {
	TmdbIDs []int `json:"tmdbIds"`
}

// Server houses all dependencies for the media server to work such as loggers, clients, configurations, etc.
type Server struct {
	baseLogger *zap.SugaredLogger
	manager    manager.MediaManager
	config     config.Server
	fileServer http.Handler
}

// New creates a new media server
func New(logger *zap.SugaredLogger, manager manager.MediaManager, config config.Server) Server {
	return Server{
		baseLogger: logger,
		manager:    manager,
		config:     config,
	}
}

func writeErrorResponse(w http.ResponseWriter, status int, err error) error {
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	return writeResponse(w, status, GenericResponse{
		Error: errMsg,
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
func (s *Server) Serve(port int) error {
	if _, err := os.Stat(s.config.DistDir); os.IsNotExist(err) {
		return fmt.Errorf("static file directory does not exist: %s", s.config.DistDir)
	}
	s.fileServer = http.FileServer(http.Dir(s.config.DistDir))

	rtr := mux.NewRouter()
	rtr.Use(s.LogMiddleware())
	rtr.HandleFunc("/healthz", s.Healthz()).Methods(http.MethodGet)

	api := rtr.PathPrefix("/api").Subrouter()
	v1 := api.PathPrefix("/v1").Subrouter()

	v1.HandleFunc("/library/movies", s.ListMovies()).Methods(http.MethodGet)
	v1.HandleFunc("/library/movies", s.AddMovieToLibrary()).Methods(http.MethodPost)
	v1.HandleFunc("/library/movies/{id}", s.DeleteMovieFromLibrary()).Methods(http.MethodDelete)
	v1.HandleFunc("/library/movies/{id}/monitored", s.UpdateMovieMonitored()).Methods(http.MethodPatch)
	v1.HandleFunc("/library/movies/{id}/quality", s.UpdateMovieQualityProfile()).Methods(http.MethodPatch)
	v1.HandleFunc("/library/movies/{id}/search", s.SearchForMovie()).Methods(http.MethodPost)

	v1.HandleFunc("/movie/{tmdbID}", s.GetMovieDetailByTMDBID()).Methods(http.MethodGet)

	v1.HandleFunc("/tv/{tmdbID}", s.GetTVDetailByTMDBID()).Methods(http.MethodGet)

	v1.HandleFunc("/library/tv", s.ListTVShows()).Methods(http.MethodGet)
	v1.HandleFunc("/library/tv", s.AddSeriesToLibrary()).Methods(http.MethodPost)
	v1.HandleFunc("/library/tv/{id}", s.DeleteSeriesFromLibrary()).Methods(http.MethodDelete)
	v1.HandleFunc("/library/tv/{id}/monitored", s.UpdateSeriesMonitored()).Methods(http.MethodPatch)
	v1.HandleFunc("/library/tv/{id}/search", s.SearchForSeries()).Methods(http.MethodPost)
	v1.HandleFunc("/season/{id}/search", s.SearchForSeason()).Methods(http.MethodPost)
	v1.HandleFunc("/episode/{id}/search", s.SearchForEpisode()).Methods(http.MethodPost)

	v1.HandleFunc("/tv/refresh", s.RefreshSeriesMetadata()).Methods(http.MethodPost)
	v1.HandleFunc("/movies/refresh", s.RefreshMovieMetadata()).Methods(http.MethodPost)

	v1.HandleFunc("/discover/movie", s.SearchMovie()).Methods(http.MethodGet)
	v1.HandleFunc("/discover/tv", s.SearchTV()).Methods(http.MethodGet)

	v1.HandleFunc("/indexers", s.ListIndexers()).Methods(http.MethodGet)
	v1.HandleFunc("/indexers", s.CreateIndexer()).Methods(http.MethodPost)
	v1.HandleFunc("/indexers/{id}", s.UpdateIndexer()).Methods(http.MethodPut)
	v1.HandleFunc("/indexers", s.DeleteIndexer()).Methods(http.MethodDelete)

	v1.HandleFunc("/indexer-sources", s.ListIndexerSources()).Methods(http.MethodGet)
	v1.HandleFunc("/indexer-sources", s.CreateIndexerSource()).Methods(http.MethodPost)
	v1.HandleFunc("/indexer-sources/{id}", s.GetIndexerSource()).Methods(http.MethodGet)
	v1.HandleFunc("/indexer-sources/{id}", s.UpdateIndexerSource()).Methods(http.MethodPut)
	v1.HandleFunc("/indexer-sources/{id}", s.DeleteIndexerSource()).Methods(http.MethodDelete)
	v1.HandleFunc("/indexer-sources/test", s.TestIndexerSource()).Methods(http.MethodPost)
	v1.HandleFunc("/indexer-sources/{id}/refresh", s.RefreshIndexerSource()).Methods(http.MethodPost)

	v1.HandleFunc("/download/clients", s.ListDownloadClients()).Methods(http.MethodGet)
	v1.HandleFunc("/download/clients/{id}", s.GetDownloadClient()).Methods(http.MethodGet)
	v1.HandleFunc("/download/clients/test", s.TestDownloadClient()).Methods(http.MethodPost)
	v1.HandleFunc("/download/clients", s.CreateDownloadClient()).Methods(http.MethodPost)
	v1.HandleFunc("/download/clients/{id}", s.UpdateDownloadClient()).Methods(http.MethodPut)
	v1.HandleFunc("/download/clients/{id}", s.DeleteDownloadClient()).Methods(http.MethodDelete)

	v1.HandleFunc("/quality/definitions", s.ListQualityDefinitions()).Methods(http.MethodGet)
	v1.HandleFunc("/quality/definitions/{id}", s.GetQualityDefinition()).Methods(http.MethodGet)
	v1.HandleFunc("/quality/definitions", s.CreateQualityDefinition()).Methods(http.MethodPost)
	v1.HandleFunc("/quality/definitions/{id}", s.UpdateQualityDefinition()).Methods(http.MethodPut)
	v1.HandleFunc("/quality/definitions", s.DeleteQualityDefinition()).Methods(http.MethodDelete)

	v1.HandleFunc("/quality/profiles/{id}", s.GetQualityProfile()).Methods(http.MethodGet)
	v1.HandleFunc("/quality/profiles", s.ListQualityProfiles()).Methods(http.MethodGet)
	v1.HandleFunc("/quality/profiles", s.CreateQualityProfile()).Methods(http.MethodPost)
	v1.HandleFunc("/quality/profiles/{id}", s.UpdateQualityProfile()).Methods(http.MethodPut)
	v1.HandleFunc("/quality/profiles/{id}", s.DeleteQualityProfile()).Methods(http.MethodDelete)

	v1.HandleFunc("/config", s.GetConfig()).Methods(http.MethodGet)
	v1.HandleFunc("/library/stats", s.GetLibraryStats()).Methods(http.MethodGet)

	v1.HandleFunc("/jobs", s.ListJobs()).Methods(http.MethodGet)
	v1.HandleFunc("/jobs", s.CreateJob()).Methods(http.MethodPost)
	v1.HandleFunc("/jobs/{id}", s.GetJob()).Methods(http.MethodGet)
	v1.HandleFunc("/jobs/{id}/cancel", s.CancelJob()).Methods(http.MethodPost)

	v1.HandleFunc("/activity/active", s.GetActiveActivity()).Methods(http.MethodGet)
	v1.HandleFunc("/activity/failures", s.GetRecentFailures()).Methods(http.MethodGet)
	v1.HandleFunc("/activity/timeline", s.GetActivityTimeline()).Methods(http.MethodGet)
	v1.HandleFunc("/activity/history/{entityType}/{entityId}", s.GetEntityTransitionHistory()).Methods(http.MethodGet)

	rtr.PathPrefix("/static/").Handler(s.FileHandler()).Methods(http.MethodGet)
	rtr.PathPrefix("/").Handler(s.IndexHandler())

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

		resp := GenericResponse{Response: movies}

		writeResponse(w, http.StatusOK, resp)
	}
}

// GetMovieDetailByTMDBID retrieves detailed information for a single movie by TMDB ID
func (s Server) GetMovieDetailByTMDBID() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())
		vars := mux.Vars(r)
		tmdbIDVar := vars["tmdbID"]

		tmdbID, err := strconv.Atoi(tmdbIDVar)
		if err != nil {
			http.Error(w, "Invalid TMDB ID format", http.StatusBadRequest)
			return
		}

		movieDetail, err := s.manager.GetMovieDetailByTMDBID(r.Context(), tmdbID)
		if err != nil {
			log.Error("failed to get movie detail", zap.Error(err), zap.Int("tmdbID", tmdbID))
			writeErrorResponse(w, http.StatusInternalServerError, err)
			return
		}

		resp := GenericResponse{Response: movieDetail}
		writeResponse(w, http.StatusOK, resp)
	}
}

// GetTVDetailByTMDBID retrieves detailed information for a single TV show by TMDB ID
func (s Server) GetTVDetailByTMDBID() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())
		vars := mux.Vars(r)
		tmdbIDVar := vars["tmdbID"]

		tmdbID, err := strconv.Atoi(tmdbIDVar)
		if err != nil {
			http.Error(w, "Invalid TMDB ID format", http.StatusBadRequest)
			return
		}

		tvDetail, err := s.manager.GetTVDetailByTMDBID(r.Context(), tmdbID)
		if err != nil {
			log.Error("failed to get TV detail", zap.Error(err), zap.Int("tmdbID", tmdbID))
			writeErrorResponse(w, http.StatusInternalServerError, err)
			return
		}

		resp := GenericResponse{Response: tvDetail}
		writeResponse(w, http.StatusOK, resp)
	}
}

// ListTVShows lists tv shows on disk
func (s Server) ListTVShows() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		log := logger.FromCtx(r.Context())
		shows, err := s.manager.ListShowsInLibrary(r.Context())
		if err != nil {
			log.Error("failed to list shows", zap.Error(err))
			http.Error(w, "failed to list shows", http.StatusInternalServerError)
			return
		}

		resp := GenericResponse{
			Response: shows,
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

func (s Server) UpdateIndexer() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())
		vars := mux.Vars(r)
		idStr := vars["id"]

		id, err := strconv.Atoi(idStr)
		if err != nil {
			http.Error(w, "Invalid ID format", http.StatusBadRequest)
			return
		}

		b, err := io.ReadAll(r.Body)
		if err != nil {
			log.Debug("invalid request body", zap.Error(err))
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		var request manager.UpdateIndexerRequest
		err = json.Unmarshal(b, &request)
		if err != nil {
			log.Debug("invalid request body", zap.ByteString("body", b))
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		log.Debug("updating indexer", zap.Int("id", id), zap.Any("request", request))
		indexer, err := s.manager.UpdateIndexer(r.Context(), int32(id), request)
		if err != nil {
			log.Debug("failed to update indexer", zap.Error(err))
			writeErrorResponse(w, http.StatusInternalServerError, err)
			return
		}

		log.Debug("successfully updated indexer")
		writeResponse(w, http.StatusOK, GenericResponse{
			Response: indexer,
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

// ListQualityDefinitions lists all stored quality definitions
func (s Server) GetQualityDefinition() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())
		vars := mux.Vars(r)
		idVar := vars["id"]

		id, err := strconv.ParseInt(idVar, 10, 64)
		if err != nil {
			http.Error(w, "Invalid ID format", http.StatusBadRequest)
			return
		}

		result, err := s.manager.GetQualityDefinition(r.Context(), id)
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

// DeleteMovieFromLibrary removes a movie from the library with optional file deletion
func (s Server) DeleteMovieFromLibrary() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())
		vars := mux.Vars(r)

		id, err := strconv.ParseInt(vars["id"], 10, 64)
		if err != nil {
			http.Error(w, "Invalid ID", http.StatusBadRequest)
			return
		}

		deleteFiles := r.URL.Query().Get("deleteFiles") == "true"

		if err := s.manager.DeleteMovie(r.Context(), id, deleteFiles); err != nil {
			log.Error("delete failed", zap.Error(err))
			if errors.Is(err, storage.ErrNotFound) {
				writeErrorResponse(w, http.StatusNotFound, err)
				return
			}
			writeErrorResponse(w, http.StatusInternalServerError, err)
			return
		}

		writeResponse(w, http.StatusOK, GenericResponse{
			Response: map[string]any{
				"id":           id,
				"message":      "Movie deleted",
				"filesDeleted": deleteFiles,
			},
		})
	}
}

// UpdateMovieMonitored updates the monitoring status of a movie
func (s Server) UpdateMovieMonitored() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())
		vars := mux.Vars(r)

		id, err := strconv.ParseInt(vars["id"], 10, 64)
		if err != nil {
			http.Error(w, "Invalid ID", http.StatusBadRequest)
			return
		}

		var req struct {
			Monitored bool `json:"monitored"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid body", http.StatusBadRequest)
			return
		}

		movie, err := s.manager.UpdateMovieMonitored(r.Context(), id, req.Monitored)
		if err != nil {
			log.Error("update failed", zap.Error(err))
			if errors.Is(err, storage.ErrNotFound) {
				writeErrorResponse(w, http.StatusNotFound, err)
				return
			}
			writeErrorResponse(w, http.StatusInternalServerError, err)
			return
		}

		writeResponse(w, http.StatusOK, GenericResponse{Response: movie})
	}
}

// UpdateMovieQualityProfile updates the quality profile of a movie
func (s Server) UpdateMovieQualityProfile() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())
		vars := mux.Vars(r)

		id, err := strconv.ParseInt(vars["id"], 10, 64)
		if err != nil {
			http.Error(w, "Invalid ID", http.StatusBadRequest)
			return
		}

		var req struct {
			QualityProfileID int32 `json:"qualityProfileId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid body", http.StatusBadRequest)
			return
		}

		movie, err := s.manager.UpdateMovieQualityProfile(r.Context(), id, req.QualityProfileID)
		if err != nil {
			log.Error("update failed", zap.Error(err))
			if errors.Is(err, storage.ErrNotFound) {
				writeErrorResponse(w, http.StatusNotFound, err)
				return
			}
			writeErrorResponse(w, http.StatusInternalServerError, err)
			return
		}

		writeResponse(w, http.StatusOK, GenericResponse{Response: movie})
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

		mediaType := r.URL.Query().Get("type")

		var profile []*storage.QualityProfile
		var err error

		switch mediaType {
		case "movie":
			profile, err = s.manager.ListMovieQualityProfiles(r.Context())
		case "series":
			profile, err = s.manager.ListEpisodeQualityProfiles(r.Context())
		default:
			profile, err = s.manager.ListQualityProfiles(r.Context())
		}

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

func (s Server) CreateQualityProfile() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())
		b, err := io.ReadAll(r.Body)
		if err != nil {
			log.Debug("invalid request body", zap.Error(err))
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		var request manager.AddQualityProfileRequest
		err = json.Unmarshal(b, &request)
		if err != nil {
			log.Debug("invalid request body", zap.ByteString("body", b))
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		profile, err := s.manager.AddQualityProfile(r.Context(), request)
		if err != nil {
			writeErrorResponse(w, http.StatusInternalServerError, err)
			return
		}

		writeResponse(w, http.StatusCreated, GenericResponse{Response: profile})
	}
}

func (s Server) UpdateQualityProfile() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())
		vars := mux.Vars(r)
		idStr := vars["id"]

		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid ID format", http.StatusBadRequest)
			return
		}

		b, err := io.ReadAll(r.Body)
		if err != nil {
			log.Debug("invalid request body", zap.Error(err))
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		var request manager.UpdateQualityProfileRequest
		err = json.Unmarshal(b, &request)
		if err != nil {
			log.Debug("invalid request body", zap.ByteString("body", b))
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		profile, err := s.manager.UpdateQualityProfile(r.Context(), id, request)
		if err != nil {
			writeErrorResponse(w, http.StatusInternalServerError, err)
			return
		}

		writeResponse(w, http.StatusOK, GenericResponse{Response: profile})
	}
}

func (s Server) DeleteQualityProfile() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		idStr := vars["id"]

		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid ID format", http.StatusBadRequest)
			return
		}

		request := manager.DeleteQualityProfileRequest{
			ID: func() *int { i := int(id); return &i }(),
		}

		err = s.manager.DeleteQualityProfile(r.Context(), request)
		if err != nil {
			writeErrorResponse(w, http.StatusInternalServerError, err)
			return
		}

		writeResponse(w, http.StatusOK, GenericResponse{Response: request})
	}
}

func (s Server) UpdateQualityDefinition() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())
		vars := mux.Vars(r)
		idStr := vars["id"]

		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid ID format", http.StatusBadRequest)
			return
		}

		b, err := io.ReadAll(r.Body)
		if err != nil {
			log.Debug("invalid request body", zap.Error(err))
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		var request manager.UpdateQualityDefinitionRequest
		err = json.Unmarshal(b, &request)
		if err != nil {
			log.Debug("invalid request body", zap.ByteString("body", b))
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		definition, err := s.manager.UpdateQualityDefinition(r.Context(), id, request)
		if err != nil {
			writeErrorResponse(w, http.StatusInternalServerError, err)
			return
		}

		writeResponse(w, http.StatusOK, GenericResponse{Response: definition})
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

		err = writeResponse(w, http.StatusCreated, GenericResponse{Response: result})
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

		writeResponse(w, http.StatusOK, GenericResponse{
			Response: downloadClient,
		})
	}
}

func (s Server) UpdateDownloadClient() http.HandlerFunc {
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

		b, err := io.ReadAll(r.Body)
		if err != nil {
			log.Debug("invalid request body", zap.Error(err))
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		var request manager.UpdateDownloadClientRequest
		err = json.Unmarshal(b, &request)
		if err != nil {
			log.Debug("invalid request body", zap.String("body", string(b)))
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		result, err := s.manager.UpdateDownloadClient(r.Context(), id, request)
		if err != nil {
			writeErrorResponse(w, http.StatusInternalServerError, err)
			return
		}

		err = writeResponse(w, http.StatusOK, GenericResponse{Response: result})
		if err != nil {
			log.Error("failed to write response", zap.Error(err))
			return
		}
	}
}

func (s Server) TestDownloadClient() http.HandlerFunc {
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

		err = s.manager.TestDownloadClient(r.Context(), request)
		if err != nil {
			writeErrorResponse(w, http.StatusBadRequest, err)
			return
		}

		writeResponse(w, http.StatusOK, GenericResponse{
			Response: map[string]string{"message": "Connection successful"},
		})
	}
}

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

		writeResponse(w, http.StatusOK, GenericResponse{
			Response: downloadClient,
		})
	}
}

func (s Server) AddSeriesToLibrary() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())
		b, err := io.ReadAll(r.Body)
		if err != nil {
			log.Debug("invalid request body", zap.Error(err))
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		var request manager.AddSeriesRequest
		err = json.Unmarshal(b, &request)
		if err != nil {
			log.Debug("invalid request body", zap.String("body", string(b)))
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		release, err := s.manager.AddSeriesToLibrary(r.Context(), request)
		if err != nil {
			log.Error("couldn't add a series", zap.Error(err))
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

// DeleteSeriesFromLibrary removes a series from the library with optional file deletion
func (s Server) DeleteSeriesFromLibrary() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())
		vars := mux.Vars(r)

		id, err := strconv.ParseInt(vars["id"], 10, 64)
		if err != nil {
			http.Error(w, "Invalid ID", http.StatusBadRequest)
			return
		}

		deleteDirectory := r.URL.Query().Get("deleteDirectory") == "true"

		if err := s.manager.DeleteSeries(r.Context(), id, deleteDirectory); err != nil {
			log.Error("delete failed", zap.Error(err))
			if errors.Is(err, storage.ErrNotFound) {
				writeErrorResponse(w, http.StatusNotFound, err)
				return
			}
			writeErrorResponse(w, http.StatusInternalServerError, err)
			return
		}

		writeResponse(w, http.StatusOK, GenericResponse{
			Response: map[string]any{
				"id":               id,
				"message":          "Series deleted",
				"directoryDeleted": deleteDirectory,
			},
		})
	}
}

// UpdateSeriesMonitored updates the monitoring status of a series
func (s Server) UpdateSeriesMonitored() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())
		vars := mux.Vars(r)

		id, err := strconv.ParseInt(vars["id"], 10, 64)
		if err != nil {
			http.Error(w, "Invalid ID", http.StatusBadRequest)
			return
		}

		var req struct {
			Monitored bool `json:"monitored"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid body", http.StatusBadRequest)
			return
		}

		series, err := s.manager.UpdateSeriesMonitored(r.Context(), id, req.Monitored)
		if err != nil {
			log.Error("update failed", zap.Error(err))
			if errors.Is(err, storage.ErrNotFound) {
				writeErrorResponse(w, http.StatusNotFound, err)
				return
			}
			writeErrorResponse(w, http.StatusInternalServerError, err)
			return
		}

		writeResponse(w, http.StatusOK, GenericResponse{Response: series})
	}
}

func (s Server) FileHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.StripPrefix("/static", s.fileServer).ServeHTTP(w, r)
	}
}

func (s Server) IndexHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(s.config.DistDir, "index.html"))
	}
}

// GetConfig returns the library configuration (non-sensitive data only)
func (s Server) GetConfig() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		result := s.manager.GetConfigSummary()
		writeResponse(w, http.StatusOK, GenericResponse{Response: result})
	}
}

// ListJobs lists jobs with optional pagination
func (s Server) ListJobs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		params, err := ParsePaginationParams(r)
		if err != nil {
			writeErrorResponse(w, http.StatusBadRequest, err)
			return
		}

		jobs, err := s.manager.ListJobs(r.Context(), nil, nil, params)
		if err != nil {
			writeErrorResponse(w, http.StatusInternalServerError, err)
			return
		}

		writeResponse(w, http.StatusOK, GenericResponse{Response: jobs})
	}
}

// GetJob gets a job
func (s Server) GetJob() http.HandlerFunc {
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

		jobs, err := s.manager.GetJob(r.Context(), id)
		if err != nil {
			writeErrorResponse(w, http.StatusInternalServerError, err)
			return
		}

		writeResponse(w, http.StatusOK, GenericResponse{Response: jobs})
	}
}

// CancelJob cancels a running job
func (s Server) CancelJob() http.HandlerFunc {
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

		jobs, err := s.manager.CancelJob(r.Context(), id)
		if err != nil {
			writeErrorResponse(w, http.StatusInternalServerError, err)
			return
		}

		writeResponse(w, http.StatusOK, GenericResponse{Response: jobs})
	}
}

// CreateJob creates a new pending job
func (s Server) CreateJob() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())
		b, err := io.ReadAll(r.Body)
		if err != nil {
			log.Debug("invalid request body", zap.Error(err))
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		var request manager.TriggerJobRequest
		err = json.Unmarshal(b, &request)
		if err != nil {
			log.Debug("invalid request body", zap.String("body", string(b)))
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		jobs, err := s.manager.CreateJob(r.Context(), request)
		if err != nil {
			writeErrorResponse(w, http.StatusInternalServerError, err)
			return
		}

		writeResponse(w, http.StatusCreated, GenericResponse{Response: jobs})
	}
}

// GetLibraryStats returns library statistics
func (s Server) GetLibraryStats() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		stats, err := s.manager.GetLibraryStats(ctx)
		if err != nil {
			writeErrorResponse(w, http.StatusInternalServerError, err)
			return
		}
		writeResponse(w, http.StatusOK, GenericResponse{Response: stats})
	}
}

func (s Server) RefreshSeriesMetadata() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())

		var req RefreshRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Debug("invalid request body", zap.Error(err))
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		err := s.manager.RefreshSeriesMetadata(r.Context(), req.TmdbIDs...)
		if err != nil {
			log.Error("failed to refresh series metadata", zap.Error(err))
			writeErrorResponse(w, http.StatusInternalServerError, err)
			return
		}

		writeResponse(w, http.StatusOK, GenericResponse{
			Response: "Series metadata refresh completed",
		})
	}
}

func (s Server) RefreshMovieMetadata() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())

		var req RefreshRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Debug("invalid request body", zap.Error(err))
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		err := s.manager.RefreshMovieMetadata(r.Context(), req.TmdbIDs...)
		if err != nil {
			log.Error("failed to refresh movie metadata", zap.Error(err))
			writeErrorResponse(w, http.StatusInternalServerError, err)
			return
		}

		writeResponse(w, http.StatusOK, GenericResponse{
			Response: "Movie metadata refresh completed",
		})
	}
}

func (s Server) SearchForMovie() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())
		vars := mux.Vars(r)

		id, err := strconv.ParseInt(vars["id"], 10, 64)
		if err != nil {
			http.Error(w, "Invalid ID", http.StatusBadRequest)
			return
		}

		err = s.manager.SearchForMovie(r.Context(), id)
		if err != nil {
			log.Error("search failed", zap.Error(err))
			if errors.Is(err, storage.ErrNotFound) {
				writeErrorResponse(w, http.StatusNotFound, err)
				return
			}
			writeErrorResponse(w, http.StatusBadRequest, err)
			return
		}

		w.WriteHeader(http.StatusAccepted)
	}
}

func (s Server) SearchForSeries() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())
		vars := mux.Vars(r)

		id, err := strconv.ParseInt(vars["id"], 10, 64)
		if err != nil {
			http.Error(w, "Invalid ID", http.StatusBadRequest)
			return
		}

		err = s.manager.SearchForSeries(r.Context(), id)
		if err != nil {
			log.Error("search failed", zap.Error(err))
			if errors.Is(err, storage.ErrNotFound) {
				writeErrorResponse(w, http.StatusNotFound, err)
				return
			}
			writeErrorResponse(w, http.StatusBadRequest, err)
			return
		}

		w.WriteHeader(http.StatusAccepted)
	}
}

func (s Server) SearchForSeason() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())
		vars := mux.Vars(r)

		id, err := strconv.ParseInt(vars["id"], 10, 64)
		if err != nil {
			http.Error(w, "Invalid ID", http.StatusBadRequest)
			return
		}

		err = s.manager.SearchForSeason(r.Context(), id)
		if err != nil {
			log.Error("search failed", zap.Error(err))
			if errors.Is(err, storage.ErrNotFound) {
				writeErrorResponse(w, http.StatusNotFound, err)
				return
			}
			writeErrorResponse(w, http.StatusBadRequest, err)
			return
		}

		w.WriteHeader(http.StatusAccepted)
	}
}

func (s Server) SearchForEpisode() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromCtx(r.Context())
		vars := mux.Vars(r)

		id, err := strconv.ParseInt(vars["id"], 10, 64)
		if err != nil {
			http.Error(w, "Invalid ID", http.StatusBadRequest)
			return
		}

		err = s.manager.SearchForEpisode(r.Context(), id)
		if err != nil {
			log.Error("search failed", zap.Error(err))
			if errors.Is(err, storage.ErrNotFound) {
				writeErrorResponse(w, http.StatusNotFound, err)
				return
			}
			writeErrorResponse(w, http.StatusBadRequest, err)
			return
		}

		w.WriteHeader(http.StatusAccepted)
	}
}
