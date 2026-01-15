package manager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-jet/jet/v2/qrm"
	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/cache"
	"github.com/kasuboski/mediaz/pkg/download"
	"github.com/kasuboski/mediaz/pkg/indexer"
	"github.com/kasuboski/mediaz/pkg/library"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/pagination"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
	"github.com/kasuboski/mediaz/pkg/tmdb"
	"github.com/oapi-codegen/nullable"
	"go.uber.org/zap"
)

type MediaManager struct {
	tmdb           tmdb.ITmdb
	indexerFactory indexer.Factory
	indexerCache   *cache.Cache[int64, indexerCacheEntry]
	library        library.Library
	storage        storage.Storage
	factory        download.Factory
	config         config.Config
	configs        config.Manager
	scheduler      *Scheduler
}

func New(tmbdClient tmdb.ITmdb, indexerFactory indexer.Factory, library library.Library, storage storage.Storage, factory download.Factory, managerConfigs config.Manager, fullConfig config.Config) MediaManager {

	m := MediaManager{
		tmdb:           tmbdClient,
		indexerFactory: indexerFactory,
		indexerCache:   cache.New[int64, indexerCacheEntry](),
		library:        library,
		storage:        storage,
		factory:        factory,
		config:         fullConfig,
		configs:        managerConfigs,
	}

	executors := map[JobType]JobExecutor{
		MovieReconcile: func(ctx context.Context, jobID int64) error {
			return m.ReconcileMovies(ctx)
		},
		MovieIndex: func(ctx context.Context, jobID int64) error {
			return m.IndexMovieLibrary(ctx)
		},
		SeriesReconcile: func(ctx context.Context, jobID int64) error {
			return m.ReconcileSeries(ctx)
		},
		SeriesIndex: func(ctx context.Context, jobID int64) error {
			return m.IndexSeriesLibrary(ctx)
		},
		IndexerSync: func(ctx context.Context, jobID int64) error {
			return m.RefreshAllIndexerSources(ctx)
		},
	}

	scheduler := NewScheduler(storage, managerConfigs, executors)
	m.scheduler = scheduler

	return m
}

func now() time.Time {
	return time.Now()
}

// SearchMovie querie tmdb for a movie
func (m MediaManager) SearchMovie(ctx context.Context, query string) (*SearchMediaResponse, error) {
	log := logger.FromCtx(ctx)
	if query == "" {
		log.Debug("search movie query is empty", zap.String("query", query))
		return nil, errors.New("query is empty")
	}

	res, err := m.tmdb.SearchMovie(ctx, &tmdb.SearchMovieParams{Query: query})
	if err != nil {
		log.Error("search movie failed request", zap.Error(err))
		return nil, err
	}
	defer res.Body.Close()

	log.Debug("search movie response", zap.Any("status", res.Status))
	result, err := parseMediaResult(res)
	if err != nil {
		log.Debug("error parsing movie query result", zap.Error(err))
		return nil, err
	}

	return result, nil
}

// GetMovieDetailByTMDBID retrieves detailed information for a single movie by TMDB ID
func (m MediaManager) GetMovieDetailByTMDBID(ctx context.Context, tmdbID int) (*MovieDetailResult, error) {
	log := logger.FromCtx(ctx)

	// Get movie metadata from TMDB (creates if not exists)
	metadata, err := m.GetMovieMetadata(ctx, tmdbID)
	if err != nil {
		log.Error("failed to get movie metadata", zap.Error(err), zap.Int("tmdbID", tmdbID))
		return nil, err
	}

	// Create the detailed result from metadata
	result := &MovieDetailResult{
		TMDBID:           metadata.TmdbID,
		ImdbID:           metadata.ImdbID,
		Title:            metadata.Title,
		OriginalTitle:    metadata.OriginalTitle,
		Overview:         metadata.Overview,
		PosterPath:       metadata.Images,
		Runtime:          &metadata.Runtime,
		Genres:           metadata.Genres,
		Studio:           metadata.Studio,
		Website:          metadata.Website,
		CollectionTmdbID: metadata.CollectionTmdbID,
		CollectionTitle:  metadata.CollectionTitle,
		Popularity:       metadata.Popularity,
		Year:             metadata.Year,
		LibraryStatus:    "Not In Library", // Default status
	}

	// Format release date as string if available
	if metadata.ReleaseDate != nil {
		releaseDateStr := metadata.ReleaseDate.Format("2006-01-02")
		result.ReleaseDate = &releaseDateStr
	}

	// Try to get library information (movie record)
	movie, err := m.storage.GetMovieByMetadataID(ctx, int(metadata.ID))
	if err == nil && movie != nil {
		result.ID = &movie.ID
		result.LibraryStatus = string(movie.State)
		result.Path = movie.Path
		result.QualityProfileID = &movie.QualityProfileID
		monitored := movie.Monitored == 1
		result.Monitored = &monitored
	} else if !errors.Is(err, storage.ErrNotFound) {
		log.Debug("error checking movie library status", zap.Error(err), zap.Int32("metadataID", metadata.ID))
	}

	return result, nil
}

// GetTVDetailByTMDBID retrieves detailed information for a single TV show by TMDB ID
func (m MediaManager) GetTVDetailByTMDBID(ctx context.Context, tmdbID int) (*TVDetailResult, error) {
	log := logger.FromCtx(ctx)

	// Get data from various sources
	metadata, seriesDetailsResponse, err := m.getTVMetadataAndDetails(ctx, tmdbID)
	if err != nil {
		log.Error("failed to get TV metadata and details", zap.Error(err), zap.Int("tmdbID", tmdbID))
		return nil, err
	}

	// Get library information
	series, err := m.storage.GetSeries(ctx, table.Series.SeriesMetadataID.EQ(sqlite.Int32(metadata.ID)))
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		log.Debug("error checking series library status", zap.Error(err), zap.Int32("metadataID", metadata.ID))
	}

	var seasons []SeasonResult
	if series != nil {
		seasonsData, err := m.getSeasonsWithEpisodes(ctx, series.ID)
		if err != nil {
			log.Debug("failed to get seasons and episodes", zap.Error(err), zap.Int32("metadataID", metadata.ID))
			seasons = []SeasonResult{}
		}
		if err == nil {
			seasons = seasonsData
		}
	}

	if series == nil {
		where := table.SeasonMetadata.SeriesMetadataID.EQ(sqlite.Int32(metadata.ID))
		seasonMetadata, err := m.storage.ListSeasonMetadata(ctx, where)
		if err != nil {
			log.Debug("failed to get season metadata", zap.Error(err))
			seasons = []SeasonResult{}
		}
		if err == nil {
			for _, sm := range seasonMetadata {
				seasonResult := m.buildSeasonResultWithEpisodesFromMetadata(ctx, sm)
				seasons = append(seasons, seasonResult)
			}
		}
	}

	// Transform data into result
	result := m.buildTVDetailResult(metadata, seriesDetailsResponse, series, seasons)

	// Add stored external IDs from database
	if extIDs, err := DeserializeExternalIDs(metadata.ExternalIds); err == nil && extIDs != nil {
		result.ExternalIDs = &ExternalIDs{ImdbID: extIDs.ImdbID, TvdbID: extIDs.TvdbID}
	}

	// Add stored watch providers from database
	if wpData, err := DeserializeWatchProviders(metadata.WatchProviders); err == nil && wpData != nil {
		providers := make([]WatchProvider, 0, len(wpData.US.Flatrate))
		for _, p := range wpData.US.Flatrate {
			providers = append(providers, WatchProvider{
				ProviderID: p.ProviderID,
				Name:       p.Name,
				LogoPath:   p.LogoPath,
			})
		}
		result.WatchProviders = providers
	}

	return result, nil
}

// buildTVDetailResult transforms metadata and TMDB details into TVDetailResult
func (m MediaManager) buildTVDetailResult(metadata *model.SeriesMetadata, details *tmdb.SeriesDetailsResponse, series *storage.Series, seasons []SeasonResult) *TVDetailResult {
	result := &TVDetailResult{
		TMDBID:        metadata.TmdbID,
		Title:         metadata.Title,
		SeasonCount:   metadata.SeasonCount,
		EpisodeCount:  metadata.EpisodeCount,
		LibraryStatus: "Not In Library", // Default status
	}

	if metadata.Overview != nil {
		result.Overview = *metadata.Overview
	}

	// Set poster path - prefer database over TMDB API to avoid unnecessary API calls
	if metadata.PosterPath != nil && *metadata.PosterPath != "" {
		result.PosterPath = *metadata.PosterPath
	} else if details.PosterPath != "" {
		result.PosterPath = details.PosterPath
	}

	// Set backdrop path from TMDB API (not stored in database)
	if details.BackdropPath != "" {
		result.BackdropPath = &details.BackdropPath
	}

	// Format dates
	if metadata.FirstAirDate != nil {
		firstAirDateStr := metadata.FirstAirDate.Format("2006-01-02")
		result.FirstAirDate = &firstAirDateStr
	}
	if metadata.LastAirDate != nil {
		lastAirDateStr := metadata.LastAirDate.Format("2006-01-02")
		result.LastAirDate = &lastAirDateStr
	}
	// Next air date from TMDB next episode
	if details.NextEpisodeToAir.AirDate != "" {
		nextAir := details.NextEpisodeToAir.AirDate
		result.NextAirDate = &nextAir
	}

	// Status when available
	if details.Status != "" {
		status := details.Status
		result.Status = &status
	}

	// Original language
	if details.OriginalLanguage != "" {
		ol := details.OriginalLanguage
		result.OriginalLanguage = &ol
	}

	// Production countries (names when available)
	if len(details.ProductionCountries) > 0 {
		pcs := make([]string, 0, len(details.ProductionCountries))
		for _, pc := range details.ProductionCountries {
			if pc.Name != nil && *pc.Name != "" {
				pcs = append(pcs, *pc.Name)
			} else if pc.Iso31661 != nil && *pc.Iso31661 != "" {
				pcs = append(pcs, *pc.Iso31661)
			}
		}
		result.ProductionCountries = pcs
	}

	// Networks with optional logos
	if len(details.Networks) > 0 {
		networks := make([]NetworkInfo, 0, len(details.Networks))
		for _, n := range details.Networks {
			ni := NetworkInfo{Name: n.Name}
			if n.LogoPath != "" {
				lp := n.LogoPath
				ni.LogoPath = &lp
			}
			networks = append(networks, ni)
		}
		result.Networks = networks
	}

	// Extract genre names
	if len(details.Genres) > 0 {
		var genres []string
		for _, genre := range details.Genres {
			genres = append(genres, genre.Name)
		}
		result.Genres = genres
	}

	// Set additional fields from TMDB response
	if details.Adult {
		result.Adult = &details.Adult
	}
	if details.Popularity > 0 {
		pop := float64(details.Popularity)
		result.Popularity = &pop
	}
	// Map ratings when available
	if details.VoteAverage > 0 {
		va := float32(details.VoteAverage)
		result.VoteAverage = &va
	}
	if details.VoteCount > 0 {
		vc := int(details.VoteCount)
		result.VoteCount = &vc
	}

	// Set library status information if series exists
	if series != nil {
		result.ID = &series.ID
		result.LibraryStatus = string(series.State)
		result.Path = series.Path
		result.QualityProfileID = &series.QualityProfileID
		monitored := series.Monitored == 1
		result.Monitored = &monitored
	}

	// Add seasons information if available
	if len(seasons) > 0 {
		result.Seasons = seasons
	}

	return result
}

// getSeasonsWithEpisodes retrieves seasons and their episodes for a series
func (m MediaManager) getSeasonsWithEpisodes(ctx context.Context, seriesID int32) ([]SeasonResult, error) {
	log := logger.FromCtx(ctx)

	// Query seasons for this series
	seasons, err := m.storage.ListSeasons(ctx,
		table.Season.SeriesID.EQ(sqlite.Int32(seriesID)))
	if err != nil {
		log.Error("failed to list seasons", zap.Error(err), zap.Int32("seriesID", seriesID))
		return nil, err
	}

	// Transform to response format with metadata and episodes
	var results []SeasonResult
	for _, season := range seasons {
		// Get season metadata for rich data
		if season.SeasonMetadataID == nil {
			log.Debug("season has no metadata ID, skipping", zap.Int32("seasonID", season.ID))
			continue
		}

		seasonMeta, err := m.storage.GetSeasonMetadata(ctx,
			table.SeasonMetadata.ID.EQ(sqlite.Int32(*season.SeasonMetadataID)))
		if err != nil {
			log.Error("failed to get season metadata", zap.Error(err), zap.Int32("seasonMetadataID", *season.SeasonMetadataID))
			continue
		}

		// Get episodes for this season
		episodes, err := m.getEpisodesForSeason(ctx, season.ID, season.SeriesID, seasonMeta.Number)
		if err != nil {
			log.Debug("failed to get episodes for season", zap.Error(err), zap.Int32("seasonID", season.ID))
			// Continue with empty episodes array
			episodes = []EpisodeResult{}
		}

		result := SeasonResult{
			ID:           season.ID,
			SeriesID:     season.SeriesID,
			Number:       seasonMeta.Number,
			Title:        seasonMeta.Title,
			TMDBID:       seasonMeta.TmdbID,
			Monitored:    season.Monitored == 1,
			EpisodeCount: int32(len(episodes)),
			Episodes:     episodes,
		}

		// Add optional fields
		if seasonMeta.Overview != nil {
			result.Overview = seasonMeta.Overview
		}
		if seasonMeta.AirDate != nil {
			airDateStr := seasonMeta.AirDate.Format("2006-01-02")
			result.AirDate = &airDateStr
		}

		results = append(results, result)
	}

	return results, nil
}

// getEpisodesForSeason retrieves episodes for a specific season
func (m MediaManager) getEpisodesForSeason(ctx context.Context, seasonID int32, seriesID int32, seasonNumber int32) ([]EpisodeResult, error) {
	log := logger.FromCtx(ctx)

	// Query episodes for this season
	episodes, err := m.storage.ListEpisodes(ctx,
		table.Episode.SeasonID.EQ(sqlite.Int32(seasonID)))
	if err != nil {
		log.Error("failed to list episodes", zap.Error(err), zap.Int32("seasonID", seasonID))
		return nil, err
	}

	// Transform to response format with metadata lookup
	results := make([]EpisodeResult, 0)
	for _, episode := range episodes {
		var episodeMeta *model.EpisodeMetadata

		// Try to get episode metadata if available
		if episode.EpisodeMetadataID != nil {
			meta, err := m.storage.GetEpisodeMetadata(ctx,
				table.EpisodeMetadata.ID.EQ(sqlite.Int32(*episode.EpisodeMetadataID)))
			if err != nil {
				log.Error("failed to get episode metadata", zap.Error(err), zap.Int32("episodeMetadataID", *episode.EpisodeMetadataID))
				// Continue without metadata
			} else {
				episodeMeta = meta
			}
		}

		// Build result with available data
		result := EpisodeResult{
			ID:           episode.ID,
			SeriesID:     seriesID,
			SeasonNumber: seasonNumber,
			Monitored:    episode.Monitored == 1,
			Downloaded:   episode.State == storage.EpisodeStateDownloaded || episode.State == storage.EpisodeStateCompleted,
		}

		// Fill in metadata if available
		if episodeMeta != nil {
			result.TMDBID = episodeMeta.TmdbID
			result.Number = episodeMeta.Number
			result.Title = episodeMeta.Title

			// Add optional fields
			if episodeMeta.Overview != nil {
				result.Overview = episodeMeta.Overview
			}
			if episodeMeta.AirDate != nil {
				airDateStr := episodeMeta.AirDate.Format("2006-01-02")
				result.AirDate = &airDateStr
			}
			if episodeMeta.Runtime != nil {
				result.Runtime = episodeMeta.Runtime
			}
			if episodeMeta.StillPath != nil {
				result.StillPath = episodeMeta.StillPath
			}

		} else {
			// Fallback values for episodes without metadata
			result.TMDBID = 0
			result.Number = episode.EpisodeNumber
			result.Title = fmt.Sprintf("Episode %d", episode.EpisodeNumber)
		}

		results = append(results, result)
	}

	return results, nil
}

// GetConfigSummary returns a readonly summary of the library configuration
func (m MediaManager) GetConfigSummary() ConfigSummary {
	return ConfigSummary{
		Library: LibraryConfig{
			MovieDir:         m.config.Library.MovieDir,
			TVDir:            m.config.Library.TVDir,
			DownloadMountDir: m.config.Library.DownloadMountDir,
		},
		Server: ServerConfig{
			Port: m.config.Server.Port,
		},
		Jobs: JobsConfig{
			MovieReconcile:  m.config.Manager.Jobs.MovieReconcile.String(),
			MovieIndex:      m.config.Manager.Jobs.MovieIndex.String(),
			SeriesReconcile: m.config.Manager.Jobs.SeriesReconcile.String(),
			SeriesIndex:     m.config.Manager.Jobs.SeriesIndex.String(),
		},
	}
}

// GetLibraryStats returns aggregate statistics about the library using optimized queries
func (m MediaManager) buildSeasonResultWithEpisodesFromMetadata(ctx context.Context, sm *model.SeasonMetadata) SeasonResult {
	log := logger.FromCtx(ctx)

	where := table.EpisodeMetadata.SeasonMetadataID.EQ(sqlite.Int32(sm.ID))
	episodeMetadata, err := m.storage.ListEpisodeMetadata(ctx, where)
	if err != nil {
		log.Debug("failed to get episode metadata for season", zap.Error(err))
		return m.buildBasicSeasonResult(sm, []EpisodeResult{})
	}

	var episodes []EpisodeResult
	for _, em := range episodeMetadata {
		episode := m.buildEpisodeResultFromMetadata(em, sm.Number)
		episodes = append(episodes, episode)
	}

	return m.buildBasicSeasonResult(sm, episodes)
}

func (m MediaManager) buildEpisodeResultFromMetadata(em *model.EpisodeMetadata, seasonNum int32) EpisodeResult {
	episode := EpisodeResult{
		ID:           0,
		TMDBID:       em.TmdbID,
		Number:       em.Number,
		Title:        em.Title,
		SeasonNumber: seasonNum,
		SeriesID:     0,
		Monitored:    false,
		Downloaded:   false,
	}

	if em.Overview != nil {
		episode.Overview = em.Overview
	}
	if em.AirDate != nil {
		airDateStr := em.AirDate.Format("2006-01-02")
		episode.AirDate = &airDateStr
	}
	if em.StillPath != nil {
		episode.StillPath = em.StillPath
	}
	if em.Runtime != nil {
		episode.Runtime = em.Runtime
	}

	return episode
}

func (m MediaManager) buildBasicSeasonResult(sm *model.SeasonMetadata, episodes []EpisodeResult) SeasonResult {
	result := SeasonResult{
		ID:           0,
		SeriesID:     0,
		Number:       sm.Number,
		Title:        sm.Title,
		TMDBID:       sm.TmdbID,
		Monitored:    false,
		EpisodeCount: int32(len(episodes)),
		Episodes:     episodes,
	}

	if sm.Overview != nil {
		result.Overview = sm.Overview
	}
	if sm.AirDate != nil {
		airDateStr := sm.AirDate.Format("2006-01-02")
		result.AirDate = &airDateStr
	}

	return result
}

func (m MediaManager) GetLibraryStats(ctx context.Context) (*storage.LibraryStats, error) {
	// Use the new optimized storage method that aggregates in the database
	return m.storage.GetLibraryStats(ctx)
}

// SearchMovie query tmdb for tv shows
func (m MediaManager) SearchTV(ctx context.Context, query string) (*SearchMediaResponse, error) {
	log := logger.FromCtx(ctx)
	if query == "" {
		log.Debug("search tv query is empty", zap.String("query", query))
		return nil, errors.New("query is empty")
	}

	res, err := m.tmdb.SearchTv(ctx, &tmdb.SearchTvParams{Query: query})
	if err != nil {
		log.Error("search tv failed request", zap.Error(err))
		return nil, err
	}
	defer res.Body.Close()

	log.Debug("search tv response", zap.Any("status", res.Status))
	result, err := parseMediaResult(res)
	if err != nil {
		log.Debug("error parsing tv show query result", zap.Error(err))
		return nil, err
	}

	return result, nil
}

func (m MediaManager) GetSeriesDetails(ctx context.Context, tmdbID int) (model.SeriesMetadata, error) {
	var model model.SeriesMetadata
	det, err := m.tmdb.GetSeriesDetails(ctx, tmdbID)
	if err != nil {
		return model, err
	}

	model, err = FromSeriesDetails(*det)
	return model, err
}

func parseMediaResult(res *http.Response) (*SearchMediaResponse, error) {
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected media query status status: %s", res.Status)
	}

	b, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	results := new(SearchMediaResponse)
	err = json.Unmarshal(b, results)
	return results, err
}

func (m MediaManager) listIndexersInternal(ctx context.Context) ([]model.Indexer, error) {
	var all []model.Indexer

	keys := m.indexerCache.Keys()
	for _, sourceID := range keys {
		cached, ok := m.indexerCache.Get(sourceID)
		if !ok {
			continue
		}

		if !cached.Enabled {
			continue
		}

		for _, idx := range cached.Indexers {
			all = append(all, model.Indexer{
				ID:              idx.ID,
				IndexerSourceID: ptr(int32(sourceID)),
				Name:            idx.Name,
				Priority:        idx.Priority,
				URI:             idx.URI,
				APIKey:          nil,
			})
		}
	}

	dbIndexers, err := m.storage.ListIndexers(ctx)
	if err != nil {
		return nil, err
	}

	for _, idx := range dbIndexers {
		if idx.IndexerSourceID == nil {
			all = append(all, *idx)
		}
	}

	return all, nil
}

func (m MediaManager) ListIndexers(ctx context.Context) ([]IndexerResponse, error) {
	var all []IndexerResponse

	keys := m.indexerCache.Keys()
	for _, sourceID := range keys {
		cached, ok := m.indexerCache.Get(sourceID)
		if !ok {
			continue
		}

		for _, idx := range cached.Indexers {
			all = append(all, IndexerResponse{
				ID:       idx.ID,
				Name:     idx.Name,
				Source:   cached.SourceName,
				Priority: idx.Priority,
				URI:      idx.URI,
			})
		}
	}

	dbIndexers, err := m.storage.ListIndexers(ctx)
	if err != nil {
		return nil, err
	}

	for _, idx := range dbIndexers {
		if idx.IndexerSourceID == nil {
			all = append(all, IndexerResponse{
				ID:       idx.ID,
				Name:     idx.Name,
				Source:   "Internal",
				Priority: idx.Priority,
				URI:      idx.URI,
			})
		}
	}

	return all, nil
}

func (m MediaManager) ListShowsInLibrary(ctx context.Context) ([]LibraryShow, error) {
	series, err := m.storage.ListSeries(ctx)
	if err != nil {
		return nil, err
	}
	var shows []LibraryShow
	for _, sp := range series {
		srec := *sp
		// Skip series without metadata - they haven't been reconciled yet
		if srec.SeriesMetadataID == nil {
			continue
		}

		ls := LibraryShow{State: string(srec.State)}
		if srec.Path != nil {
			ls.Path = *srec.Path
		}

		meta, err := m.storage.GetSeriesMetadata(ctx, table.SeriesMetadata.ID.EQ(sqlite.Int(int64(*srec.SeriesMetadataID))))
		if err == nil && meta != nil {
			ls.TMDBID = meta.TmdbID
			ls.Title = meta.Title
			if meta.PosterPath != nil {
				ls.PosterPath = *meta.PosterPath
			}
			if meta.FirstAirDate != nil {
				ls.Year = int32(meta.FirstAirDate.Year())
			}
		}

		shows = append(shows, ls)
	}
	return shows, nil
}

// ListMoviesInLibrary returns library movies enriched with metadata
func (m MediaManager) ListMoviesInLibrary(ctx context.Context) ([]LibraryMovie, error) {
	all, err := m.storage.ListMovies(ctx)
	if err != nil {
		return nil, err
	}
	var movies []LibraryMovie
	for _, mp := range all {
		mrec := *mp
		// Skip movies without metadata - they haven't been reconciled yet
		if mrec.MovieMetadataID == nil {
			continue
		}

		lm := LibraryMovie{State: string(mrec.State)}
		if mrec.Path != nil {
			lm.Path = *mrec.Path
		}

		meta, err := m.GetMovieMetadataByID(ctx, *mrec.MovieMetadataID)
		if err == nil && meta != nil {
			lm.TMDBID = meta.TmdbID
			lm.Title = meta.Title
			lm.PosterPath = meta.Images
			if meta.Year != nil {
				lm.Year = *meta.Year
			}
		}

		movies = append(movies, lm)
	}
	return movies, nil
}

func (m MediaManager) Run(ctx context.Context) error {
	log := logger.FromCtx(ctx)

	if err := m.RefreshAllIndexerSources(ctx); err != nil {
		log.Warn("failed to refresh indexer sources on startup", zap.Error(err))
	}

	go m.scheduler.Run(ctx)

	for range ctx.Done() {
		log.Info("shutting down manager")
		return ctx.Err()
	}

	return nil
}

// IndexMovieLibrary indexes the movie library directory for new files that are not yet monitored. The movies are then stored with a state of discovered.
func (m MediaManager) IndexMovieLibrary(ctx context.Context) error {
	log := logger.FromCtx(ctx)

	discoveredFiles, err := m.library.FindMovies(ctx)
	if err != nil {
		return fmt.Errorf("failed to index movie library: %w", err)
	}

	if len(discoveredFiles) == 0 {
		log.Debug("no files discovered")
		return nil
	}

	movieFiles, err := m.storage.ListMovieFiles(ctx)
	if err != nil {
		return fmt.Errorf("failed to list movie files: %w", err)
	}

	for _, discoveredFile := range discoveredFiles {
		// need to check if the file is already tracked, and if not, add it
		isTracked := false
		for _, mf := range movieFiles {
			if mf == nil {
				continue
			}

			if strings.EqualFold(*mf.RelativePath, discoveredFile.RelativePath) {
				log.Debug("discovered file relative path matches monitored movie file relative path",
					zap.String("discovered file relative path", discoveredFile.RelativePath),
					zap.String("monitored file relative path", *mf.RelativePath))
				isTracked = true
				break
			}

			if strings.EqualFold(*mf.OriginalFilePath, discoveredFile.AbsolutePath) {
				log.Debug("discovered file absolute path matches monitored movie file original path",
					zap.String("discovered file absolute path", discoveredFile.AbsolutePath),
					zap.String("monitored file original path", *mf.OriginalFilePath))
				isTracked = true
				break
			}
		}

		if isTracked {
			continue
		}

		mf := model.MovieFile{
			OriginalFilePath: &discoveredFile.RelativePath, // this should always be relative if we discovered it in the library.. surely
			RelativePath:     &discoveredFile.RelativePath, // TODO: make sure it's actually relative
			Size:             discoveredFile.Size,
		}

		log.Debug("discovered new movie file", zap.String("path", discoveredFile.RelativePath))

		id, err := m.storage.CreateMovieFile(ctx, mf)
		if err != nil {
			log.Errorf("couldn't store movie file: %w", err)
			continue
		}

		log.Debug("created new movie file in storage", zap.Int64("movie file id", id))
	}

	// pull the updated movie file list in case we added anything above
	movieFiles, err = m.storage.ListMovieFiles(ctx)
	if err != nil {
		return fmt.Errorf("failed to list movie files: %w", err)
	}

	for _, f := range movieFiles {
		movieName := library.MovieNameFromFilepath(*f.RelativePath)
		foundMovie, err := m.storage.GetMovieByPath(ctx, movieName)
		if err == nil {
			log.Debug("movie file associated with movie already", zap.Any("movie id", foundMovie.ID))
			continue
		}
		if !errors.Is(err, qrm.ErrNoRows) {
			log.Debug("error fetching movie", zap.Error(err))
			continue
		}

		log.Debug("movie file does not have associated movie")

		movie := storage.Movie{
			Movie: model.Movie{
				MovieFileID: &f.ID,
				Path:        &movieName,
				Monitored:   0,
			},
		}

		_, err = m.storage.CreateMovie(ctx, movie, storage.MovieStateDiscovered)
		if err != nil {
			log.Errorf("couldn't create new movie for discovered file: %w", err)
			continue
		}

		log.Debug("successfully created movie for discovered movie file")
	}

	return nil
}

// AddMovieToLibrary adds a movie to be managed by mediaz
// TODO: check status of movie before doing anything else.. do we already have it tracked? is it downloaded or already discovered? error state?
func (m MediaManager) AddMovieToLibrary(ctx context.Context, request AddMovieRequest) (*storage.Movie, error) {
	log := logger.FromCtx(ctx)

	profile, err := m.GetQualityProfile(ctx, int64(request.QualityProfileID))
	if err != nil {
		log.Debug("failed to get quality profile", zap.Int32("id", request.QualityProfileID), zap.Error(err))
		return nil, err
	}

	det, err := m.GetMovieMetadata(ctx, request.TMDBID)
	if err != nil {
		log.Debug("failed to get movie metadata", zap.Error(err))
		return nil, err
	}

	movie, err := m.storage.GetMovieByMetadataID(ctx, int(det.ID))
	// if we find the movie we're done
	if err == nil {
		return movie, err
	}

	// anything other than a not found error is an internal error
	if !errors.Is(err, storage.ErrNotFound) {
		log.Warnw("couldn't find movie by metadata", "meta_id", det.ID, "err", err)
		return nil, err
	}

	// need to add the movie if it does not exist
	movie = &storage.Movie{
		Movie: model.Movie{
			MovieMetadataID:  &det.ID,
			QualityProfileID: profile.ID,
			Monitored:        1,
			Path:             &det.Title,
		},
	}

	state := storage.MovieStateMissing
	if !isReleased(now(), det.ReleaseDate) {
		state = storage.MovieStateUnreleased
	}

	id, err := m.storage.CreateMovie(ctx, *movie, state)
	if err != nil {
		log.Warnw("failed to create movie", "err", err)
		return nil, err
	}

	log.Debug("created movie", zap.Any("movie", movie))

	movie, err = m.storage.GetMovie(ctx, id)
	if err != nil {
		log.Warnw("failed to get created movie", "err", err)
	}

	return movie, nil
}

// AddSeriesToLibrary adds a series to be managed by mediaz
func (m MediaManager) AddSeriesToLibrary(ctx context.Context, request AddSeriesRequest) (*storage.Series, error) {
	log := logger.FromCtx(ctx)

	qualityProfile, err := m.GetQualityProfile(ctx, int64(request.QualityProfileID))
	if err != nil {
		log.Debug("failed to get quality profile", zap.Int32("id", request.QualityProfileID), zap.Error(err))
		return nil, err
	}

	seriesMetadata, err := m.GetSeriesMetadata(ctx, request.TMDBID)
	if err != nil {
		log.Debug("failed to get series metadata", zap.Error(err))
		return nil, err
	}

	series, err := m.storage.GetSeries(ctx, table.Series.SeriesMetadataID.EQ(sqlite.Int32(seriesMetadata.ID)))
	// if we find the series we dont need to add it
	if err == nil {
		return series, err
	}
	if !errors.Is(err, storage.ErrNotFound) {
		log.Warnw("couldn't find series by metadata", "meta_id", seriesMetadata.ID, "err", err)
		return nil, err
	}

	series = &storage.Series{
		Series: model.Series{
			SeriesMetadataID: &seriesMetadata.ID,
			QualityProfileID: qualityProfile.ID,
			Monitored:        0,
			Path:             &seriesMetadata.Title,
		},
	}

	state := storage.SeriesStateMissing
	if !isReleased(now(), seriesMetadata.FirstAirDate) {
		state = storage.SeriesStateUnreleased
	}

	seriesID, err := m.storage.CreateSeries(ctx, *series, state)
	if err != nil {
		log.Error("failed to create new missing series", zap.Error(err))
		return nil, err
	}

	log.Debug("created new missing series", zap.Any("series", series))

	// Get series to access its metadata ID
	seriesEntity, err := m.storage.GetSeries(ctx, table.Series.ID.EQ(sqlite.Int(seriesID)))
	if err != nil || seriesEntity.SeriesMetadataID == nil {
		log.Error("failed to get series or series has no metadata")
		return nil, fmt.Errorf("series has no metadata")
	}

	where := table.SeasonMetadata.SeriesMetadataID.EQ(sqlite.Int32(*seriesEntity.SeriesMetadataID))
	seasonMetadata, err := m.storage.ListSeasonMetadata(ctx, where)
	if err != nil {
		return nil, err
	}

	monitoredEpisodeSet := make(map[int32]bool)
	for _, episodeTMDBID := range request.MonitoredEpisodes {
		monitoredEpisodeSet[episodeTMDBID] = true
	}

	seriesHasMonitoredEpisodes := false
	for _, s := range seasonMetadata {
		season := storage.Season{
			Season: model.Season{
				SeriesID:         int32(seriesID),
				SeasonMetadataID: ptr(s.ID),
				SeasonNumber:     s.Number,
				Monitored:        0,
			},
		}

		seasonID, err := m.storage.CreateSeason(ctx, season, storage.SeasonStateMissing)
		if err != nil {
			log.Error("failed to create season", zap.Error(err))
			return nil, err
		}

		log.Debug("created new missing season", zap.Any("season", season))

		seasonEntity, err := m.storage.GetSeason(ctx, table.Season.ID.EQ(sqlite.Int64(seasonID)))
		if err != nil || seasonEntity.SeasonMetadataID == nil {
			log.Error("failed to get season or season has no metadata linked")
			return nil, fmt.Errorf("season has no metadata")
		}

		where := table.EpisodeMetadata.SeasonMetadataID.EQ(sqlite.Int32(*seasonEntity.SeasonMetadataID))

		episodesMetadata, err := m.storage.ListEpisodeMetadata(ctx, where)
		if err != nil {
			log.Error("failed to list episode metadata", zap.Error(err))
			return nil, err
		}

		seasonHasMonitoredEpisodes := false
		for _, e := range episodesMetadata {
			episodeMonitored := int32(0)
			if monitoredEpisodeSet[e.TmdbID] {
				episodeMonitored = 1
				seasonHasMonitoredEpisodes = true
				seriesHasMonitoredEpisodes = true
			}

			episode := storage.Episode{
				Episode: model.Episode{
					EpisodeMetadataID: ptr(e.ID),
					SeasonID:          int32(seasonID),
					Monitored:         episodeMonitored,
					EpisodeNumber:     e.Number,
				},
			}

			_, err := m.storage.CreateEpisode(ctx, episode, storage.EpisodeStateMissing)
			if err != nil {
				log.Error("failed to create episode", zap.Error(err))
				return nil, err
			}

			log.Debug("created new missing episode", zap.Any("episode", episode))
		}

		if seasonHasMonitoredEpisodes {
			seasonUpdate := model.Season{Monitored: 1}
			err = m.storage.UpdateSeason(ctx, seasonUpdate, table.Season.ID.EQ(sqlite.Int64(seasonID)))
			if err != nil {
				log.Warn("failed to update season monitoring", zap.Error(err))
			}
		}
	}

	if seriesHasMonitoredEpisodes {
		seriesUpdate := model.Series{Monitored: 1}
		err = m.storage.UpdateSeries(ctx, seriesUpdate, table.Series.ID.EQ(sqlite.Int64(seriesID)))
		if err != nil {
			log.Warn("failed to update series monitoring", zap.Error(err))
		}
	}

	series, err = m.storage.GetSeries(ctx, table.Series.ID.EQ(sqlite.Int64(seriesID)))
	if err != nil {
		log.Warnw("failed to get created series", "err", err)
	}

	return series, err
}

func (m MediaManager) DeleteMovie(ctx context.Context, movieID int64, deleteFiles bool) error {
	log := logger.FromCtx(ctx)

	movie, err := m.storage.GetMovie(ctx, movieID)
	if err != nil {
		return fmt.Errorf("failed to get movie: %w", err)
	}

	if deleteFiles {
		if movie.Path == nil {
			return fmt.Errorf("cannot delete files: movie path is nil")
		}

		if err := m.library.DeleteMovieDirectory(ctx, *movie.Path); err != nil {
			return fmt.Errorf("failed to delete movie directory %s: %w", *movie.Path, err)
		}
	}

	if err := m.storage.DeleteMovie(ctx, movieID); err != nil {
		return fmt.Errorf("failed to delete movie: %w", err)
	}

	log.Info("deleted movie", zap.Int64("id", movieID), zap.Bool("files_deleted", deleteFiles))
	return nil
}

func (m MediaManager) DeleteSeries(ctx context.Context, seriesID int64, deleteDirectory bool) error {
	log := logger.FromCtx(ctx)

	series, err := m.storage.GetSeries(ctx, table.Series.ID.EQ(sqlite.Int64(seriesID)))
	if err != nil {
		return fmt.Errorf("failed to get series: %w", err)
	}

	seasons, err := m.storage.ListSeasons(ctx, table.Season.SeriesID.EQ(sqlite.Int64(seriesID)))
	if err != nil {
		log.Warn("failed to get seasons for cleanup", zap.Error(err))
	}

	for _, season := range seasons {
		episodes, err := m.storage.ListEpisodes(ctx, table.Episode.SeasonID.EQ(sqlite.Int64(int64(season.ID))))
		if err != nil {
			log.Warn("failed to get episodes for cleanup", zap.Int32("season_id", season.ID), zap.Error(err))
			continue
		}
		for _, episode := range episodes {
			if episode.EpisodeFileID == nil {
				continue
			}
			if err := m.storage.DeleteEpisodeFile(ctx, int64(*episode.EpisodeFileID)); err != nil {
				log.Warn("failed to delete episode file", zap.Int32("episode_file_id", *episode.EpisodeFileID), zap.Error(err))
			}
		}
	}

	if deleteDirectory {
		if series.Path == nil {
			return fmt.Errorf("cannot delete directory: series path is nil")
		}

		if err := m.library.DeleteSeriesDirectory(ctx, *series.Path); err != nil {
			return fmt.Errorf("failed to delete series directory %s: %w", *series.Path, err)
		}
	}

	if err := m.storage.DeleteSeries(ctx, seriesID); err != nil {
		return fmt.Errorf("failed to delete series: %w", err)
	}

	log.Info("deleted series", zap.Int64("id", seriesID), zap.Bool("directory_deleted", deleteDirectory))
	return nil
}

func (m MediaManager) UpdateMovieMonitored(ctx context.Context, movieID int64, monitored bool) (*storage.Movie, error) {
	monitoredInt := int32(0)
	if monitored {
		monitoredInt = 1
	}

	movieUpdate := model.Movie{Monitored: monitoredInt}
	err := m.storage.UpdateMovie(ctx, movieUpdate, table.Movie.ID.EQ(sqlite.Int64(movieID)))
	if err != nil {
		return nil, err
	}

	movie, err := m.storage.GetMovie(ctx, movieID)
	if err != nil {
		return nil, err
	}

	logger.FromCtx(ctx).Info("updated monitoring", zap.Int64("movie_id", movieID), zap.Bool("monitored", monitored))
	return movie, nil
}

func (m MediaManager) UpdateMovieQualityProfile(ctx context.Context, movieID int64, qualityProfileID int32) (*storage.Movie, error) {
	err := m.storage.UpdateMovieQualityProfile(ctx, movieID, qualityProfileID)
	if err != nil {
		return nil, err
	}

	movie, err := m.storage.GetMovie(ctx, movieID)
	if err != nil {
		return nil, err
	}

	logger.FromCtx(ctx).Info("updated quality profile", zap.Int64("movie_id", movieID), zap.Int32("quality_profile_id", qualityProfileID))
	return movie, nil
}

func (m MediaManager) UpdateSeriesMonitored(ctx context.Context, seriesID int64, monitored bool) (*storage.Series, error) {
	monitoredInt := int32(0)
	if monitored {
		monitoredInt = 1
	}

	seriesUpdate := model.Series{Monitored: monitoredInt}
	err := m.storage.UpdateSeries(ctx, seriesUpdate, table.Series.ID.EQ(sqlite.Int64(seriesID)))
	if err != nil {
		return nil, err
	}

	series, err := m.storage.GetSeries(ctx, table.Series.ID.EQ(sqlite.Int64(seriesID)))
	if err != nil {
		return nil, err
	}

	logger.FromCtx(ctx).Info("updated monitoring", zap.Int64("series_id", seriesID), zap.Bool("monitored", monitored))
	return series, nil
}

func (m MediaManager) UpdateSeasonMonitoring(ctx context.Context, seasonID int64, monitored bool) (*storage.Season, error) {
	log := logger.FromCtx(ctx)
	monitoredInt := int32(0)
	if monitored {
		monitoredInt = 1
	}

	seasonUpdate := model.Season{Monitored: monitoredInt}
	err := m.storage.UpdateSeason(ctx, seasonUpdate, table.Season.ID.EQ(sqlite.Int64(seasonID)))
	if err != nil {
		return nil, err
	}

	episodes, err := m.storage.ListEpisodes(ctx, table.Episode.SeasonID.EQ(sqlite.Int32(int32(seasonID))))
	if err != nil {
		log.Warn("failed to list episodes for season", zap.Error(err))
	}

	for _, episode := range episodes {
		episodeUpdate := model.Episode{Monitored: monitoredInt}
		err = m.storage.UpdateEpisode(ctx, episodeUpdate, table.Episode.ID.EQ(sqlite.Int64(int64(episode.ID))))
		if err != nil {
			log.Warn("failed to update episode monitoring", zap.Error(err), zap.Int32("episode_id", episode.ID))
		}
	}

	season, err := m.storage.GetSeason(ctx, table.Season.ID.EQ(sqlite.Int64(seasonID)))
	if err != nil {
		return nil, err
	}

	log.Info("updated season monitoring", zap.Int64("season_id", seasonID), zap.Bool("monitored", monitored), zap.Int("episodes_updated", len(episodes)))
	return season, nil
}

func (m MediaManager) UpdateSeriesMonitoring(ctx context.Context, seriesID int64, request UpdateSeriesMonitoringRequest) (*storage.Series, error) {
	log := logger.FromCtx(ctx)

	monitoredEpisodeSet := make(map[int32]bool)
	for _, episodeTMDBID := range request.MonitoredEpisodes {
		monitoredEpisodeSet[episodeTMDBID] = true
	}

	seasons, err := m.storage.ListSeasons(ctx, table.Season.SeriesID.EQ(sqlite.Int32(int32(seriesID))))
	if err != nil {
		return nil, fmt.Errorf("failed to list seasons: %w", err)
	}

	episodesUpdated := 0
	seriesHasMonitoredEpisodes := false
	for _, season := range seasons {
		if season.SeasonMetadataID == nil {
			continue
		}

		where := table.EpisodeMetadata.SeasonMetadataID.EQ(sqlite.Int32(*season.SeasonMetadataID))
		episodesMetadata, err := m.storage.ListEpisodeMetadata(ctx, where)
		if err != nil {
			log.Warn("failed to list episode metadata", zap.Error(err), zap.Int32("season_id", season.ID))
			continue
		}

		episodeMetadataMap := make(map[int32]int32)
		for _, em := range episodesMetadata {
			episodeMetadataMap[em.ID] = em.TmdbID
		}

		episodes, err := m.storage.ListEpisodes(ctx, table.Episode.SeasonID.EQ(sqlite.Int32(season.ID)))
		if err != nil {
			log.Warn("failed to list episodes for season", zap.Error(err), zap.Int32("season_id", season.ID))
			continue
		}

		seasonHasMonitoredEpisodes := false
		for _, episode := range episodes {
			if episode.EpisodeMetadataID == nil {
				continue
			}

			episodeTMDBID, found := episodeMetadataMap[*episode.EpisodeMetadataID]
			if !found {
				continue
			}

			monitored := int32(0)
			if monitoredEpisodeSet[episodeTMDBID] {
				monitored = 1
				seasonHasMonitoredEpisodes = true
				seriesHasMonitoredEpisodes = true
			}

			episodeUpdate := model.Episode{Monitored: monitored}
			err = m.storage.UpdateEpisode(ctx, episodeUpdate, table.Episode.ID.EQ(sqlite.Int64(int64(episode.ID))))
			if err != nil {
				log.Warn("failed to update episode monitoring", zap.Error(err), zap.Int32("episode_id", episode.ID))
				continue
			}
			episodesUpdated++
		}

		seasonMonitored := int32(0)
		if seasonHasMonitoredEpisodes {
			seasonMonitored = 1
		}

		seasonUpdate := model.Season{Monitored: seasonMonitored}
		err = m.storage.UpdateSeason(ctx, seasonUpdate, table.Season.ID.EQ(sqlite.Int64(int64(season.ID))))
		if err != nil {
			log.Warn("failed to update season monitoring", zap.Error(err), zap.Int32("season_id", season.ID))
		}
	}

	seriesMonitored := int32(0)
	if seriesHasMonitoredEpisodes {
		seriesMonitored = 1
	}

	seriesUpdate := model.Series{Monitored: seriesMonitored}
	if request.QualityProfileID != nil {
		seriesUpdate.QualityProfileID = *request.QualityProfileID
	}
	err = m.storage.UpdateSeries(ctx, seriesUpdate, table.Series.ID.EQ(sqlite.Int64(seriesID)))
	if err != nil {
		log.Warn("failed to update series monitoring", zap.Error(err))
	}

	series, err := m.storage.GetSeries(ctx, table.Series.ID.EQ(sqlite.Int64(seriesID)))
	if err != nil {
		return nil, err
	}

	log.Info("updated series monitoring", zap.Int64("series_id", seriesID), zap.Int("episodes_updated", episodesUpdated))
	return series, nil
}

func (m MediaManager) SearchIndexers(ctx context.Context, indexers, categories []int32, opts indexer.SearchOptions) ([]*prowlarr.ReleaseResource, error) {
	log := logger.FromCtx(ctx)

	sourceIndexers := make(map[int64][]int32)

	keys := m.indexerCache.Keys()
	for _, sourceID := range keys {
		cached, ok := m.indexerCache.Get(sourceID)
		if !ok {
			continue
		}

		if !cached.Enabled {
			continue
		}

		for _, idx := range cached.Indexers {
			for _, id := range indexers {
				if idx.ID == id {
					sourceIndexers[sourceID] = append(sourceIndexers[sourceID], id)
				}
			}
		}
	}

	if len(sourceIndexers) == 0 {
		return nil, fmt.Errorf("no indexers available")
	}

	type result struct {
		releases []*prowlarr.ReleaseResource
		err      error
	}

	resultChan := make(chan result, len(sourceIndexers))
	var wg sync.WaitGroup

	for sourceID, idxIDs := range sourceIndexers {
		wg.Add(1)
		go func(srcID int64, indexerIDs []int32) {
			defer wg.Done()
			releases, err := m.searchIndexerSource(ctx, srcID, indexerIDs, categories, opts)
			resultChan <- result{releases: releases, err: err}
		}(sourceID, idxIDs)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	var allReleases []*prowlarr.ReleaseResource
	errorCount := 0
	successCount := 0
	totalSources := len(sourceIndexers)

	for res := range resultChan {
		if res.err != nil {
			log.Error("source search failed", zap.Error(res.err))
			errorCount++
			continue
		}
		successCount++
		allReleases = append(allReleases, res.releases...)
	}

	log.Debug("SearchIndexers result",
		zap.Int("error_count", errorCount),
		zap.Int("success_count", successCount),
		zap.Int("total_sources", totalSources),
		zap.Int("releases", len(allReleases)))

	if errorCount == totalSources && totalSources > 0 {
		log.Error("all sources failed, returning error")
		return nil, fmt.Errorf("all indexers unavailable")
	}

	if successCount > 0 && len(allReleases) == 0 {
		log.Error("searches succeeded but no results, returning error")
		return nil, fmt.Errorf("no results found")
	}

	if len(allReleases) == 0 {
		log.Error("no releases and no clear error, returning error")
		return nil, fmt.Errorf("no results found")
	}

	return allReleases, nil
}

func (m MediaManager) searchIndexerSource(ctx context.Context, sourceID int64, indexerIDs, categories []int32, opts indexer.SearchOptions) ([]*prowlarr.ReleaseResource, error) {
	log := logger.FromCtx(ctx)

	sourceConfig, err := m.storage.GetIndexerSource(ctx, sourceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get indexer source")
	}

	source, err := m.indexerFactory.NewIndexerSource(sourceConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create indexer source")
	}

	var sourceReleases []*prowlarr.ReleaseResource
	errorCount := 0

	for _, indexerID := range indexerIDs {
		releases, err := source.Search(ctx, indexerID, categories, opts)
		if err != nil {
			log.Error("indexer search failed",
				zap.Int32("indexerID", indexerID),
				zap.Error(err))
			errorCount++
			continue
		}
		sourceReleases = append(sourceReleases, releases...)
	}

	log.Debug("searchIndexerSource result",
		zap.Int("error_count", errorCount),
		zap.Int("total_indexers", len(indexerIDs)),
		zap.Int("releases", len(sourceReleases)))

	if errorCount == len(indexerIDs) && len(indexerIDs) > 0 {
		log.Error("all indexers in source failed, returning error")
		return nil, fmt.Errorf("all indexers unavailable")
	}

	if len(sourceReleases) == 0 {
		log.Error("no releases found, returning error")
		return nil, fmt.Errorf("no results found")
	}

	return sourceReleases, nil
}

func (m MediaManager) AddIndexer(ctx context.Context, request AddIndexerRequest) (IndexerResponse, error) {
	indexer := request.Indexer

	if indexer.Name == "" {
		return IndexerResponse{}, fmt.Errorf("indexer name is required")
	}

	id, err := m.storage.CreateIndexer(ctx, indexer)
	if err != nil {
		return IndexerResponse{}, err
	}

	indexer.ID = int32(id)

	return toIndexerResponse(indexer), nil
}

func (m MediaManager) UpdateIndexer(ctx context.Context, id int32, request UpdateIndexerRequest) (IndexerResponse, error) {
	if request.Name == "" {
		return IndexerResponse{}, fmt.Errorf("indexer name is required")
	}

	indexer := model.Indexer{
		ID:       id,
		Name:     request.Name,
		Priority: request.Priority,
		URI:      request.URI,
		APIKey:   request.APIKey,
	}

	err := m.storage.UpdateIndexer(ctx, int64(id), indexer)
	if err != nil {
		return IndexerResponse{}, err
	}

	return toIndexerResponse(indexer), nil
}

// AddIndexer stores a new indexer in the database
func (m MediaManager) DeleteIndexer(ctx context.Context, request DeleteIndexerRequest) error {
	if request.ID == nil {
		return fmt.Errorf("indexer id is required")
	}

	return m.storage.DeleteIndexer(ctx, int64(*request.ID))
}

func nullableDefault[T any](n nullable.Nullable[T]) T {
	var def T
	if n.IsSpecified() {
		v, _ := n.Get()
		return v
	}

	return def
}

func isReleased(now time.Time, releaseDate *time.Time) bool {
	if releaseDate == nil {
		return false
	}
	if releaseDate.IsZero() {
		return false
	}
	return now.After(*releaseDate)
}

func toIndexerResponse(indexer model.Indexer) IndexerResponse {
	return IndexerResponse{
		ID:       indexer.ID,
		Name:     indexer.Name,
		Priority: indexer.Priority,
		URI:      indexer.URI,
	}
}

func ptr[A any](thing A) *A {
	return &thing
}

// GetMovieMetadataByID retrieves movie metadata by its primary key
func (m MediaManager) GetMovieMetadataByID(ctx context.Context, metadataID int32) (*model.MovieMetadata, error) {
	// fetch metadata record
	return m.storage.GetMovieMetadata(ctx, table.MovieMetadata.ID.EQ(sqlite.Int(int64(metadataID))))
}

// ListSeasonsForSeries retrieves all seasons for a TV series by TMDB ID
func (m MediaManager) ListSeasonsForSeries(ctx context.Context, tmdbID int) ([]SeasonResult, error) {
	log := logger.FromCtx(ctx)

	// Ensure series metadata exists
	metadata, err := m.GetSeriesMetadata(ctx, tmdbID)
	if err != nil {
		log.Error("failed to get series metadata", zap.Error(err), zap.Int("tmdbID", tmdbID))
		return nil, err
	}

	// Find the series record that uses this metadata
	series, err := m.storage.GetSeries(ctx,
		table.Series.SeriesMetadataID.EQ(sqlite.Int32(metadata.ID)))
	if err != nil {
		log.Error("failed to get series", zap.Error(err), zap.Int32("metadataID", metadata.ID))
		return nil, err
	}

	// Query seasons with metadata join
	seasons, err := m.storage.ListSeasons(ctx,
		table.Season.SeriesID.EQ(sqlite.Int32(series.ID)))
	if err != nil {
		log.Error("failed to list seasons", zap.Error(err), zap.Int32("seriesID", series.ID))
		return nil, err
	}

	// Transform to response format with metadata lookup
	var results []SeasonResult
	for _, season := range seasons {
		// Get season metadata for rich data
		if season.SeasonMetadataID == nil {
			log.Debug("season has no metadata ID, skipping", zap.Int32("seasonID", season.ID))
			continue
		}

		seasonMeta, err := m.storage.GetSeasonMetadata(ctx,
			table.SeasonMetadata.ID.EQ(sqlite.Int32(*season.SeasonMetadataID)))
		if err != nil {
			log.Error("failed to get season metadata", zap.Error(err), zap.Int32("seasonMetadataID", *season.SeasonMetadataID))
			continue
		}

		// Count episodes for this season
		episodes, err := m.storage.ListEpisodes(ctx,
			table.Episode.SeasonID.EQ(sqlite.Int32(season.ID)))
		if err != nil {
			log.Debug("failed to count episodes for season", zap.Error(err), zap.Int32("seasonID", season.ID))
		}

		result := SeasonResult{
			SeriesID:     season.SeriesID,
			Number:       seasonMeta.Number,
			Title:        seasonMeta.Title,
			TMDBID:       seasonMeta.TmdbID,
			Monitored:    season.Monitored == 1,
			EpisodeCount: int32(len(episodes)),
		}

		// Add optional fields
		if seasonMeta.Overview != nil {
			result.Overview = seasonMeta.Overview
		}
		if seasonMeta.AirDate != nil {
			airDateStr := seasonMeta.AirDate.Format("2006-01-02")
			result.AirDate = &airDateStr
		}

		results = append(results, result)
	}

	return results, nil
}

// ListEpisodesForSeason retrieves all episodes for a season by TMDB ID and season number
func (m MediaManager) ListEpisodesForSeason(ctx context.Context, tmdbID int, seasonNumber int) ([]EpisodeResult, error) {
	log := logger.FromCtx(ctx)

	// Ensure series metadata exists
	seriesMetadata, err := m.GetSeriesMetadata(ctx, tmdbID)
	if err != nil {
		log.Error("failed to get series metadata", zap.Error(err), zap.Int("tmdbID", tmdbID))
		return nil, err
	}

	// Find the series record that uses this metadata
	series, err := m.storage.GetSeries(ctx,
		table.Series.SeriesMetadataID.EQ(sqlite.Int32(seriesMetadata.ID)))
	if err != nil {
		log.Error("failed to get series", zap.Error(err), zap.Int32("metadataID", seriesMetadata.ID))
		return nil, err
	}

	// Find all seasons for this series
	seasons, err := m.storage.ListSeasons(ctx,
		table.Season.SeriesID.EQ(sqlite.Int32(series.ID)))
	if err != nil {
		log.Error("failed to list seasons", zap.Error(err), zap.Int32("seriesID", series.ID))
		return nil, err
	}

	// Find the season that matches the requested season number
	// Priority: 1) Season with metadata matching number, 2) Season with matching season_number
	var candidateSeasons []*storage.Season
	var candidateMetas []*model.SeasonMetadata

	for _, season := range seasons {
		// Check if season has metadata with the right number
		if season.SeasonMetadataID != nil {
			meta, err := m.storage.GetSeasonMetadata(ctx,
				table.SeasonMetadata.ID.EQ(sqlite.Int32(*season.SeasonMetadataID)))
			if err == nil && meta.Number == int32(seasonNumber) {
				candidateSeasons = append(candidateSeasons, season)
				candidateMetas = append(candidateMetas, meta)
			}
		}
	}

	// If no seasons found with metadata, fall back to season_number matching
	if len(candidateSeasons) == 0 {
		for _, season := range seasons {
			if season.SeasonNumber == int32(seasonNumber) {
				candidateSeasons = append(candidateSeasons, season)
				candidateMetas = append(candidateMetas, nil)
			}
		}
	}

	if len(candidateSeasons) == 0 {
		log.Error("season not found", zap.Int32("seriesID", seriesMetadata.ID), zap.Int("seasonNumber", seasonNumber))
		return nil, fmt.Errorf("season %d not found for series %d", seasonNumber, tmdbID)
	}

	// If we have multiple candidates, prefer the one with episodes
	var targetSeason *storage.Season
	var seasonMeta *model.SeasonMetadata

	if len(candidateSeasons) == 1 {
		targetSeason = candidateSeasons[0]
		seasonMeta = candidateMetas[0]
	} else {
		// Multiple candidates - check which one has episodes
		for i, season := range candidateSeasons {
			episodes, err := m.storage.ListEpisodes(ctx,
				table.Episode.SeasonID.EQ(sqlite.Int32(season.ID)))
			if err == nil && len(episodes) > 0 {
				targetSeason = season
				seasonMeta = candidateMetas[i]
				break
			}
		}

		// If no season has episodes, just use the first one
		if targetSeason == nil {
			targetSeason = candidateSeasons[0]
			seasonMeta = candidateMetas[0]
		}
	}

	// Query episodes for this season
	episodes, err := m.storage.ListEpisodes(ctx,
		table.Episode.SeasonID.EQ(sqlite.Int32(targetSeason.ID)))
	if err != nil {
		log.Error("failed to list episodes", zap.Error(err), zap.Int32("seasonID", targetSeason.ID))
		return nil, err
	}

	// Transform to response format with metadata lookup
	results := make([]EpisodeResult, 0)
	for _, episode := range episodes {
		var episodeMeta *model.EpisodeMetadata

		// Try to get episode metadata if available
		if episode.EpisodeMetadataID != nil {
			meta, err := m.storage.GetEpisodeMetadata(ctx,
				table.EpisodeMetadata.ID.EQ(sqlite.Int32(*episode.EpisodeMetadataID)))
			if err != nil {
				log.Error("failed to get episode metadata", zap.Error(err), zap.Int32("episodeMetadataID", *episode.EpisodeMetadataID))
				// Continue without metadata
			} else {
				episodeMeta = meta
			}
		}

		// Determine season number for response
		seasonNum := int32(seasonNumber)
		if seasonMeta != nil {
			seasonNum = seasonMeta.Number
		}

		// Build result with available data
		result := EpisodeResult{
			ID:           episode.ID,
			SeriesID:     series.ID,
			SeasonNumber: seasonNum,
			Monitored:    episode.Monitored == 1,
			Downloaded:   episode.State == storage.EpisodeStateDownloaded || episode.State == storage.EpisodeStateCompleted,
		}

		// Fill in metadata if available
		if episodeMeta != nil {
			result.TMDBID = episodeMeta.TmdbID
			result.Number = episodeMeta.Number
			result.Title = episodeMeta.Title

			// Add optional fields
			if episodeMeta.Overview != nil {
				result.Overview = episodeMeta.Overview
			}
			if episodeMeta.AirDate != nil {
				airDateStr := episodeMeta.AirDate.Format("2006-01-02")
				result.AirDate = &airDateStr
			}
			if episodeMeta.Runtime != nil {
				result.Runtime = episodeMeta.Runtime
			}
			if episodeMeta.StillPath != nil {
				result.StillPath = episodeMeta.StillPath
			}

		} else {
			// Fallback values for episodes without metadata
			result.TMDBID = 0
			result.Number = episode.EpisodeNumber
			result.Title = fmt.Sprintf("Episode %d", episode.EpisodeNumber)
		}

		results = append(results, result)
	}

	return results, nil
}

func (m MediaManager) CreateJob(ctx context.Context, request TriggerJobRequest) (JobResponse, error) {
	log := logger.FromCtx(ctx)

	jobID, err := m.scheduler.createPendingJob(ctx, JobType(request.Type))
	if err == storage.ErrJobAlreadyPending {
		log.Debug("job already pending, returning existing job")
		jobs, err := m.scheduler.listPendingJobsByType(ctx, JobType(request.Type))
		if err != nil {
			return JobResponse{}, err
		}
		if len(jobs) == 0 {
			return JobResponse{}, fmt.Errorf("pending job not found after conflict")
		}
		return toJobResponse(jobs[0]), nil
	}
	if err != nil {
		return JobResponse{}, err
	}

	job, err := m.storage.GetJob(ctx, jobID)
	if err != nil {
		return JobResponse{}, err
	}

	return toJobResponse(job), nil
}

func (m MediaManager) GetJob(ctx context.Context, id int64) (JobResponse, error) {
	job, err := m.storage.GetJob(ctx, id)
	if err != nil {
		return JobResponse{}, err
	}

	return toJobResponse(job), nil
}

func (m MediaManager) ListJobs(ctx context.Context, jobType *string, state *string, params pagination.Params) (JobListResponse, error) {
	var conditions []sqlite.BoolExpression

	if jobType != nil {
		if !isValidJobType(*jobType) {
			return JobListResponse{}, fmt.Errorf("invalid job type: %s", *jobType)
		}
		conditions = append(conditions, table.Job.Type.EQ(sqlite.String(*jobType)))
	}

	if state != nil {
		switch storage.JobState(*state) {
		case storage.JobStatePending, storage.JobStateRunning, storage.JobStateDone,
			storage.JobStateError, storage.JobStateCancelled:
			conditions = append(conditions, table.JobTransition.ToState.EQ(sqlite.String(*state)))
		default:
			return JobListResponse{}, fmt.Errorf("invalid job state: %s", *state)
		}
	}

	conditions = append(conditions, table.JobTransition.MostRecent.EQ(sqlite.Bool(true)))

	where := sqlite.AND(conditions...)

	totalCount, err := m.storage.CountJobs(ctx, where)
	if err != nil {
		return JobListResponse{}, err
	}

	offset, limit := params.CalculateOffsetLimit()

	jobs, err := m.storage.ListJobs(ctx, offset, limit, where)
	if err != nil {
		return JobListResponse{}, err
	}

	responses := make([]JobResponse, len(jobs))
	for i, job := range jobs {
		responses[i] = toJobResponse(job)
	}

	if params.PageSize == 0 {
		return JobListResponse{
			Jobs:  responses,
			Count: totalCount,
		}, nil
	}

	meta := params.BuildMeta(totalCount)
	return JobListResponse{
		Jobs:       responses,
		Count:      totalCount,
		Pagination: &meta,
	}, nil
}

func (m MediaManager) CancelJob(ctx context.Context, id int64) (JobResponse, error) {
	log := logger.FromCtx(ctx)

	err := m.scheduler.CancelJob(ctx, id)
	if err != nil {
		log.Error("failed to cancel job", zap.Error(err), zap.Int64("job_id", id))
		return JobResponse{}, err
	}

	job, err := m.storage.GetJob(ctx, id)
	if err != nil {
		return JobResponse{}, err
	}

	return toJobResponse(job), nil
}

func (m MediaManager) prepareSearchSnapshot(ctx context.Context) (*ReconcileSnapshot, error) {
	log := logger.FromCtx(ctx)

	dcs, err := m.ListDownloadClients(ctx)
	if err != nil {
		log.Error("failed to list download clients", zap.Error(err))
		return nil, err
	}

	indexers, err := m.listIndexersInternal(ctx)
	if err != nil {
		log.Error("failed to list indexers", zap.Error(err))
		return nil, err
	}

	if len(indexers) == 0 {
		log.Warn("no indexers available for search")
		return nil, fmt.Errorf("no indexers available")
	}

	return newReconcileSnapshot(indexers, dcs), nil
}

func (m MediaManager) executeSearch(ctx context.Context, snapshot *ReconcileSnapshot, categories []int32, opts indexer.SearchOptions) ([]*prowlarr.ReleaseResource, error) {
	log := logger.FromCtx(ctx)

	indexerIDs := snapshot.GetIndexerIDs()
	log.Debug("searching indexers",
		zap.Int32s("indexer_ids", indexerIDs),
		zap.Int32s("categories", categories))

	releases, err := m.SearchIndexers(ctx, indexerIDs, categories, opts)
	if err != nil {
		log.Error("failed to search indexers", zap.Error(err))
		return nil, err
	}

	return releases, nil
}

func (m MediaManager) SearchForMovie(ctx context.Context, movieID int64) error {
	log := logger.FromCtx(ctx).With("movie_id", movieID)
	log.Debug("starting manual search for movie")

	movie, err := m.storage.GetMovie(ctx, movieID)
	if err != nil {
		log.Error("failed to get movie", zap.Error(err))
		return fmt.Errorf("movie not found: %w", err)
	}

	if movie.Monitored == 0 {
		log.Debug("movie is not monitored, cannot search")
		return fmt.Errorf("movie is not monitored")
	}

	snapshot, err := m.prepareSearchSnapshot(ctx)
	if err != nil {
		return err
	}

	err = m.reconcileMissingMovie(ctx, movie, snapshot)
	if err != nil {
		log.Error("failed to search for movie", zap.Error(err))
		return err
	}

	searchTime := now()
	err = m.storage.UpdateMovie(ctx, model.Movie{
		ID:             movie.ID,
		Monitored:      movie.Monitored,
		LastSearchTime: &searchTime,
	}, table.Movie.ID.EQ(sqlite.Int64(movieID)))
	if err != nil {
		log.Warn("failed to update last search time", zap.Error(err))
	}

	log.Debug("manual search completed for movie")
	return nil
}

func (m MediaManager) SearchForSeries(ctx context.Context, seriesID int64) error {
	log := logger.FromCtx(ctx).With("series_id", seriesID)
	log.Debug("starting manual search for series")

	series, err := m.storage.GetSeries(ctx, table.Series.ID.EQ(sqlite.Int32(int32(seriesID))))
	if err != nil {
		log.Error("failed to get series", zap.Error(err))
		return fmt.Errorf("series not found: %w", err)
	}

	if series.Monitored == 0 {
		log.Debug("series is not monitored, cannot search")
		return fmt.Errorf("series is not monitored")
	}

	seasons, err := m.storage.ListSeasons(ctx, table.Season.SeriesID.EQ(sqlite.Int32(series.ID)))
	if err != nil {
		log.Error("failed to list seasons", zap.Error(err))
		return fmt.Errorf("failed to list seasons: %w", err)
	}

	var searchErrors []error
	for _, season := range seasons {
		if season.Monitored == 0 {
			log.Debug("skipping unmonitored season", zap.Int32("season_id", season.ID))
			continue
		}

		err = m.SearchForSeason(ctx, int64(season.ID))
		if err != nil {
			log.Warn("failed to search for season", zap.Error(err), zap.Int32("season_id", season.ID))
			searchErrors = append(searchErrors, err)
		}
	}

	searchTime := now()
	err = m.storage.UpdateSeries(ctx, model.Series{
		ID:             series.ID,
		Monitored:      series.Monitored,
		LastSearchTime: &searchTime,
	}, table.Series.ID.EQ(sqlite.Int32(series.ID)))
	if err != nil {
		log.Warn("failed to update series last search time", zap.Error(err))
	}

	if len(searchErrors) > 0 {
		log.Warn("some season searches failed", zap.Int("failed_count", len(searchErrors)))
		return fmt.Errorf("%d season searches failed", len(searchErrors))
	}

	log.Debug("manual search completed for series")
	return nil
}

func (m MediaManager) SearchForEpisode(ctx context.Context, episodeID int64) error {
	log := logger.FromCtx(ctx).With("episode_id", episodeID)
	log.Debug("starting manual search for episode")

	episode, err := m.storage.GetEpisode(ctx, table.Episode.ID.EQ(sqlite.Int32(int32(episodeID))))
	if err != nil {
		log.Error("failed to get episode", zap.Error(err))
		return fmt.Errorf("episode not found: %w", err)
	}

	if episode.Monitored == 0 {
		log.Debug("episode is not monitored, cannot search")
		return fmt.Errorf("episode is not monitored")
	}

	season, err := m.storage.GetSeason(ctx, table.Season.ID.EQ(sqlite.Int32(episode.SeasonID)))
	if err != nil {
		log.Error("failed to get season for episode", zap.Error(err))
		return fmt.Errorf("season not found: %w", err)
	}

	snapshot, err := m.prepareSearchSnapshot(ctx)
	if err != nil {
		return err
	}

	series, err := m.storage.GetSeries(ctx, table.Series.ID.EQ(sqlite.Int32(season.SeriesID)))
	if err != nil {
		log.Error("failed to get series for episode", zap.Error(err))
		return fmt.Errorf("failed to get series: %w", err)
	}

	if series.QualityProfileID == 0 {
		log.Warn("series quality profile id is nil, skipping search")
		return fmt.Errorf("series has no quality profile")
	}

	qualityProfile, err := m.storage.GetQualityProfile(ctx, int64(series.QualityProfileID))
	if err != nil {
		log.Error("failed to get quality profile", zap.Error(err))
		return fmt.Errorf("failed to get quality profile: %w", err)
	}

	seriesMetadata, err := m.storage.GetSeriesMetadata(ctx, table.SeriesMetadata.ID.EQ(sqlite.Int32(*series.SeriesMetadataID)))
	if err != nil {
		log.Error("failed to get series metadata", zap.Error(err))
		return fmt.Errorf("failed to get series metadata: %w", err)
	}

	if seriesMetadata.Title == "" {
		log.Error("series metadata has empty title")
		return fmt.Errorf("series has no title")
	}

	searchType := indexer.TypeTV
	releases, err := m.executeSearch(ctx, snapshot, TV_CATEGORIES, indexer.SearchOptions{
		Query:   seriesMetadata.Title,
		Season:  &season.SeasonNumber,
		Episode: &episode.EpisodeNumber,
		Type:    &searchType,
	})
	if err != nil {
		return err
	}

	_, err = m.reconcileMissingEpisode(ctx, seriesMetadata.Title, season.SeasonNumber, episode, snapshot, qualityProfile, releases)
	if err != nil {
		log.Error("failed to reconcile episode", zap.Error(err))
		return err
	}

	log.Debug("manual search completed for episode")
	return nil
}

func (m MediaManager) SearchForSeason(ctx context.Context, seasonID int64) error {
	log := logger.FromCtx(ctx).With("season_id", seasonID)
	log.Debug("starting manual search for season")

	season, err := m.storage.GetSeason(ctx, table.Season.ID.EQ(sqlite.Int32(int32(seasonID))))
	if err != nil {
		log.Error("failed to get season", zap.Error(err))
		return fmt.Errorf("season not found: %w", err)
	}

	if season.Monitored == 0 {
		log.Debug("season is not monitored, cannot search")
		return fmt.Errorf("season is not monitored")
	}

	snapshot, err := m.prepareSearchSnapshot(ctx)
	if err != nil {
		return err
	}

	series, err := m.storage.GetSeries(ctx, table.Series.ID.EQ(sqlite.Int32(season.SeriesID)))
	if err != nil {
		log.Error("failed to get series for season", zap.Error(err))
		return fmt.Errorf("failed to get series: %w", err)
	}

	if series.QualityProfileID == 0 {
		log.Warn("series quality profile id is nil, skipping search")
		return fmt.Errorf("series has no quality profile")
	}

	qualityProfile, err := m.storage.GetQualityProfile(ctx, int64(series.QualityProfileID))
	if err != nil {
		log.Error("failed to get quality profile", zap.Error(err))
		return fmt.Errorf("failed to get quality profile: %w", err)
	}

	seriesMetadata, err := m.storage.GetSeriesMetadata(ctx, table.SeriesMetadata.ID.EQ(sqlite.Int32(*series.SeriesMetadataID)))
	if err != nil {
		log.Error("failed to get series metadata", zap.Error(err))
		return fmt.Errorf("failed to get series metadata: %w", err)
	}

	if seriesMetadata.Title == "" {
		log.Error("series metadata has empty title")
		return fmt.Errorf("series has no title")
	}

	searchType := indexer.TypeTV
	releases, err := m.executeSearch(ctx, snapshot, TV_CATEGORIES, indexer.SearchOptions{
		Query:  seriesMetadata.Title,
		Season: &season.SeasonNumber,
		Type:   &searchType,
	})
	if err != nil {
		return err
	}

	err = m.reconcileMissingSeason(ctx, seriesMetadata.Title, season, snapshot, qualityProfile, releases)
	if err != nil {
		log.Error("failed to reconcile season", zap.Error(err))
		return err
	}

	searchTime := now()
	err = m.storage.UpdateSeason(ctx, model.Season{
		ID:             season.ID,
		Monitored:      season.Monitored,
		LastSearchTime: &searchTime,
	}, table.Season.ID.EQ(sqlite.Int32(season.ID)))
	if err != nil {
		log.Warn("failed to update season last search time", zap.Error(err))
	}

	log.Debug("manual search completed for season")
	return nil
}
