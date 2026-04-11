package server

import (
	"github.com/gorilla/mux"
)

// registerRoutes sets up all API routes on the given router.
func (s *Server) registerRoutes(rtr *mux.Router) {
	rtr.HandleFunc("/healthz", s.Healthz()).Methods("GET")

	api := rtr.PathPrefix("/api").Subrouter()
	v1 := api.PathPrefix("/v1").Subrouter()

	// Movies
	v1.HandleFunc("/library/movies", s.ListMovies()).Methods("GET")
	v1.HandleFunc("/library/movies", s.AddMovieToLibrary()).Methods("POST")
	v1.HandleFunc("/library/movies/{id}", s.DeleteMovieFromLibrary()).Methods("DELETE")
	v1.HandleFunc("/library/movies/{id}/monitored", s.UpdateMovieMonitored()).Methods("PATCH")
	v1.HandleFunc("/library/movies/{id}/quality", s.UpdateMovieQualityProfile()).Methods("PATCH")
	v1.HandleFunc("/library/movies/{id}/search", s.SearchForMovie()).Methods("POST")

	// Movie details
	v1.HandleFunc("/movie/{tmdbID}", s.GetMovieDetailByTMDBID()).Methods("GET")

	// TV details
	v1.HandleFunc("/tv/{tmdbID}", s.GetTVDetailByTMDBID()).Methods("GET")

	// Series
	v1.HandleFunc("/library/tv", s.ListTVShows()).Methods("GET")
	v1.HandleFunc("/library/tv", s.AddSeriesToLibrary()).Methods("POST")
	v1.HandleFunc("/library/tv/{id}", s.DeleteSeriesFromLibrary()).Methods("DELETE")
	v1.HandleFunc("/library/tv/{id}/monitored", s.UpdateSeriesMonitored()).Methods("PATCH")
	v1.HandleFunc("/library/tv/{id}/search", s.SearchForSeries()).Methods("POST")
	v1.HandleFunc("/season/{id}/search", s.SearchForSeason()).Methods("POST")
	v1.HandleFunc("/episode/{id}/search", s.SearchForEpisode()).Methods("POST")

	// Refresh
	v1.HandleFunc("/tv/refresh", s.RefreshSeriesMetadata()).Methods("POST")
	v1.HandleFunc("/movies/refresh", s.RefreshMovieMetadata()).Methods("POST")

	// Discover / search
	v1.HandleFunc("/discover/movie", s.SearchMovie()).Methods("GET")
	v1.HandleFunc("/discover/tv", s.SearchTV()).Methods("GET")

	// Indexers
	v1.HandleFunc("/indexers", s.ListIndexers()).Methods("GET")
	v1.HandleFunc("/indexers", s.CreateIndexer()).Methods("POST")
	v1.HandleFunc("/indexers/{id}", s.UpdateIndexer()).Methods("PUT")
	v1.HandleFunc("/indexers/{id}", s.DeleteIndexer()).Methods("DELETE")

	// Indexer sources
	v1.HandleFunc("/indexer-sources", s.ListIndexerSources()).Methods("GET")
	v1.HandleFunc("/indexer-sources", s.CreateIndexerSource()).Methods("POST")
	v1.HandleFunc("/indexer-sources/{id}", s.GetIndexerSource()).Methods("GET")
	v1.HandleFunc("/indexer-sources/{id}", s.UpdateIndexerSource()).Methods("PUT")
	v1.HandleFunc("/indexer-sources/{id}", s.DeleteIndexerSource()).Methods("DELETE")
	v1.HandleFunc("/indexer-sources/test", s.TestIndexerSource()).Methods("POST")
	v1.HandleFunc("/indexer-sources/{id}/refresh", s.RefreshIndexerSource()).Methods("POST")

	// Download clients
	v1.HandleFunc("/download/clients", s.ListDownloadClients()).Methods("GET")
	v1.HandleFunc("/download/clients/{id}", s.GetDownloadClient()).Methods("GET")
	v1.HandleFunc("/download/clients/test", s.TestDownloadClient()).Methods("POST")
	v1.HandleFunc("/download/clients", s.CreateDownloadClient()).Methods("POST")
	v1.HandleFunc("/download/clients/{id}", s.UpdateDownloadClient()).Methods("PUT")
	v1.HandleFunc("/download/clients/{id}", s.DeleteDownloadClient()).Methods("DELETE")

	// Quality definitions
	v1.HandleFunc("/quality/definitions", s.ListQualityDefinitions()).Methods("GET")
	v1.HandleFunc("/quality/definitions/{id}", s.GetQualityDefinition()).Methods("GET")
	v1.HandleFunc("/quality/definitions", s.CreateQualityDefinition()).Methods("POST")
	v1.HandleFunc("/quality/definitions/{id}", s.UpdateQualityDefinition()).Methods("PUT")
	v1.HandleFunc("/quality/definitions/{id}", s.DeleteQualityDefinition()).Methods("DELETE")

	// Quality profiles
	v1.HandleFunc("/quality/profiles/{id}", s.GetQualityProfile()).Methods("GET")
	v1.HandleFunc("/quality/profiles", s.ListQualityProfiles()).Methods("GET")
	v1.HandleFunc("/quality/profiles", s.CreateQualityProfile()).Methods("POST")
	v1.HandleFunc("/quality/profiles/{id}", s.UpdateQualityProfile()).Methods("PUT")
	v1.HandleFunc("/quality/profiles/{id}", s.DeleteQualityProfile()).Methods("DELETE")

	// Config & stats
	v1.HandleFunc("/config", s.GetConfig()).Methods("GET")
	v1.HandleFunc("/library/stats", s.GetLibraryStats()).Methods("GET")

	// Jobs
	v1.HandleFunc("/jobs", s.ListJobs()).Methods("GET")
	v1.HandleFunc("/jobs", s.CreateJob()).Methods("POST")
	v1.HandleFunc("/jobs/{id}", s.GetJob()).Methods("GET")
	v1.HandleFunc("/jobs/{id}/cancel", s.CancelJob()).Methods("POST")

	// Activity
	v1.HandleFunc("/activity/active", s.GetActiveActivity()).Methods("GET")
	v1.HandleFunc("/activity/failures", s.GetRecentFailures()).Methods("GET")
	v1.HandleFunc("/activity/timeline", s.GetActivityTimeline()).Methods("GET")
	v1.HandleFunc("/activity/history/{entityType}/{entityId}", s.GetEntityTransitionHistory()).Methods("GET")

	// Static files
	rtr.PathPrefix("/static/").Handler(s.FileHandler()).Methods("GET")
	rtr.PathPrefix("/").Handler(s.IndexHandler())
}
