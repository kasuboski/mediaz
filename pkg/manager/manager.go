package manager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-jet/jet/v2/qrm"
	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/download"
	"github.com/kasuboski/mediaz/pkg/indexer"
	"github.com/kasuboski/mediaz/pkg/library"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/pagination"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/ptr"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
	"github.com/kasuboski/mediaz/pkg/tmdb"
	"github.com/oapi-codegen/nullable"
	"go.uber.org/zap"
)

type MediaManager struct {
	tmdb                  tmdb.ITmdb
	indexerService        *IndexerService
	library               library.Library
	movieStorage          storage.MovieStorage
	movieMetaStorage      storage.MovieMetadataStorage
	seriesStorage         storage.SeriesStorage
	seriesMetaStorage     storage.SeriesMetadataStorage
	downloadClientService *DownloadClientService
	qualityService        *QualityService
	jobService            *JobService
	config                config.Config
	configs               config.Manager
}

func New(tmbdClient tmdb.ITmdb, indexerFactory indexer.Factory, library library.Library, store storage.Storage, factory download.Factory, managerConfigs config.Manager, fullConfig config.Config) MediaManager {
	m := MediaManager{
		tmdb:                  tmbdClient,
		indexerService:        NewIndexerService(store, store, indexerFactory),
		library:               library,
		movieStorage:          store,
		movieMetaStorage:      store,
		seriesStorage:         store,
		seriesMetaStorage:     store,
		downloadClientService: NewDownloadClientService(store, factory),
		qualityService:        NewQualityService(store),
		config:                fullConfig,
		configs:               managerConfigs,
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
			return m.indexerService.RefreshAllIndexerSources(ctx)
		},
	}

	m.jobService = NewJobService(store, store, store, managerConfigs, executors)

	return m
}

// ErrValidation is returned when request validation fails.
var ErrValidation = errors.New("validation error")

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
	movie, err := m.movieStorage.GetMovieByMetadataID(ctx, int(metadata.ID))
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
	series, err := m.seriesStorage.GetSeries(ctx, table.Series.SeriesMetadataID.EQ(sqlite.Int32(metadata.ID)))
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		log.Debug("error checking series library status", zap.Error(err), zap.Int32("metadataID", metadata.ID))
	}

	// Get seasons and episodes information for the consolidated response
	var seasons []SeasonResult
	if series != nil {
		seasonsData, err := m.getSeasonsWithEpisodes(ctx, series.ID)
		if err != nil {
			log.Debug("failed to get seasons and episodes", zap.Error(err), zap.Int32("metadataID", metadata.ID))
			// Continue without seasons data - buildTVDetailResult will handle empty seasons
			seasons = []SeasonResult{}
		} else {
			seasons = seasonsData
		}
	} else {
		seasons = []SeasonResult{}
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
		monitorNewSeasons := series.MonitorNewSeasons == 1
		result.MonitorNewSeasons = &monitorNewSeasons
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
	seasons, err := m.seriesStorage.ListSeasons(ctx,
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

		seasonMeta, err := m.seriesMetaStorage.GetSeasonMetadata(ctx,
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

// preloadEpisodeMetadata fetches all episode metadata for the given IDs in a single query,
// returning a map keyed by metadata ID.
func (m MediaManager) preloadEpisodeMetadata(ctx context.Context, episodes []*storage.Episode) map[int32]*model.EpisodeMetadata {
	ids := make([]sqlite.Expression, 0, len(episodes))
	for _, ep := range episodes {
		if ep.EpisodeMetadataID != nil {
			ids = append(ids, sqlite.Int32(*ep.EpisodeMetadataID))
		}
	}
	if len(ids) == 0 {
		return nil
	}

	metas, err := m.seriesMetaStorage.ListEpisodeMetadata(ctx, table.EpisodeMetadata.ID.IN(ids...))
	if err != nil {
		log := logger.FromCtx(ctx)
		log.Error("failed to batch fetch episode metadata", zap.Error(err))
		return nil
	}

	result := make(map[int32]*model.EpisodeMetadata, len(metas))
	for _, meta := range metas {
		result[meta.ID] = meta
	}
	return result
}

// buildEpisodeResult constructs an EpisodeResult from a storage episode and optional metadata.
func buildEpisodeResult(episode *storage.Episode, episodeMeta *model.EpisodeMetadata, seriesID int32, seasonNumber int32) EpisodeResult {
	result := EpisodeResult{
		SeriesID:     seriesID,
		SeasonNumber: seasonNumber,
		Monitored:    episode.Monitored == 1,
		Downloaded:   episode.State == storage.EpisodeStateDownloaded || episode.State == storage.EpisodeStateCompleted,
	}

	if episodeMeta != nil {
		result.TMDBID = episodeMeta.TmdbID
		result.Number = episodeMeta.Number
		result.Title = episodeMeta.Title

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
		result.TMDBID = 0
		result.Number = episode.EpisodeNumber
		result.Title = fmt.Sprintf("Episode %d", episode.EpisodeNumber)
	}

	return result
}

// getEpisodesForSeason retrieves episodes for a specific season
func (m MediaManager) getEpisodesForSeason(ctx context.Context, seasonID int32, seriesID int32, seasonNumber int32) ([]EpisodeResult, error) {
	log := logger.FromCtx(ctx)

	// Query episodes for this season
	episodes, err := m.seriesStorage.ListEpisodes(ctx,
		table.Episode.SeasonID.EQ(sqlite.Int32(seasonID)))
	if err != nil {
		log.Error("failed to list episodes", zap.Error(err), zap.Int32("seasonID", seasonID))
		return nil, err
	}

	results := make([]EpisodeResult, 0, len(episodes))
	metaMap := m.preloadEpisodeMetadata(ctx, episodes)
	for _, episode := range episodes {
		var episodeMeta *model.EpisodeMetadata
		if episode.EpisodeMetadataID != nil {
			episodeMeta = metaMap[*episode.EpisodeMetadataID]
		}
		results = append(results, buildEpisodeResult(episode, episodeMeta, seriesID, seasonNumber))
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
func (m MediaManager) GetLibraryStats(ctx context.Context) (*storage.LibraryStats, error) {
	return m.jobService.GetLibraryStats(ctx)
}

func (m MediaManager) GetActiveActivity(ctx context.Context) (*ActiveActivityResponse, error) {
	return m.jobService.GetActiveActivity(ctx)
}

func (m MediaManager) GetRecentFailures(ctx context.Context, hours int) (*FailuresResponse, error) {
	return m.jobService.GetRecentFailures(ctx, hours)
}

func (m MediaManager) GetActivityTimeline(ctx context.Context, days int, params pagination.Params) (*TimelineResponse, error) {
	return m.jobService.GetActivityTimeline(ctx, days, params)
}

func (m MediaManager) GetEntityTransitionHistory(ctx context.Context, entityType string, entityID int64) (*HistoryResponse, error) {
	return m.jobService.GetEntityTransitionHistory(ctx, entityType, entityID)
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
	return m.indexerService.listIndexersInternal(ctx)
}

func (m MediaManager) ListIndexers(ctx context.Context) ([]IndexerResponse, error) {
	return m.indexerService.ListIndexers(ctx)
}

func (m MediaManager) ListShowsInLibrary(ctx context.Context) ([]LibraryShow, error) {
	series, err := m.seriesStorage.ListSeries(ctx)
	if err != nil {
		return nil, err
	}
	shows := filterAndMap(series, func(sp *storage.Series) (LibraryShow, bool) {
		// Skip series without metadata - they haven't been reconciled yet
		if sp.SeriesMetadataID == nil {
			return LibraryShow{}, false
		}
		ls := LibraryShow{State: string(sp.State)}
		if sp.Path != nil {
			ls.Path = *sp.Path
		}
		meta, err := m.seriesMetaStorage.GetSeriesMetadata(ctx, table.SeriesMetadata.ID.EQ(sqlite.Int(int64(*sp.SeriesMetadataID))))
		if err != nil || meta == nil {
			return LibraryShow{}, false
		}
		ls.TMDBID = meta.TmdbID
		ls.Title = meta.Title
		if meta.PosterPath != nil {
			ls.PosterPath = *meta.PosterPath
		}
		if meta.FirstAirDate != nil {
			ls.Year = int32(meta.FirstAirDate.Year())
		}
		return ls, true
	})
	return shows, nil
}

// ListMoviesInLibrary returns library movies enriched with metadata
func (m MediaManager) ListMoviesInLibrary(ctx context.Context) ([]LibraryMovie, error) {
	all, err := m.movieStorage.ListMovies(ctx)
	if err != nil {
		return nil, err
	}
	movies := filterAndMap(all, func(mp *storage.Movie) (LibraryMovie, bool) {
		// Skip movies without metadata - they haven't been reconciled yet
		if mp.MovieMetadataID == nil {
			return LibraryMovie{}, false
		}
		lm := LibraryMovie{State: string(mp.State)}
		if mp.Path != nil {
			lm.Path = *mp.Path
		}
		meta, err := m.GetMovieMetadataByID(ctx, *mp.MovieMetadataID)
		if err != nil || meta == nil {
			return LibraryMovie{}, false
		}
		lm.TMDBID = meta.TmdbID
		lm.Title = meta.Title
		lm.PosterPath = meta.Images
		if meta.Year != nil {
			lm.Year = *meta.Year
		}
		return lm, true
	})
	return movies, nil
}

func (m MediaManager) Run(ctx context.Context) error {
	log := logger.FromCtx(ctx)

	if err := m.RefreshAllIndexerSources(ctx); err != nil {
		log.Warn("failed to refresh indexer sources on startup", zap.Error(err))
	}

	go m.jobService.Run(ctx)

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

	movieFiles, err := m.movieStorage.ListMovieFiles(ctx)
	if err != nil {
		return fmt.Errorf("failed to list movie files: %w", err)
	}

	// Build a set of tracked paths for O(1) lookup instead of O(n*m) nested loop
	tracked := make(map[string]struct{}, len(movieFiles)*2)
	for _, mf := range movieFiles {
		if mf == nil {
			continue
		}
		if mf.RelativePath != nil {
			tracked[strings.ToLower(*mf.RelativePath)] = struct{}{}
		}
		if mf.OriginalFilePath != nil {
			tracked[strings.ToLower(*mf.OriginalFilePath)] = struct{}{}
		}
	}

	for _, discoveredFile := range discoveredFiles {
		_, trackedByRelPath := tracked[strings.ToLower(discoveredFile.RelativePath)]
		_, trackedByAbsPath := tracked[strings.ToLower(discoveredFile.AbsolutePath)]

		if trackedByRelPath {
			log.Debug("discovered file relative path matches monitored movie file relative path",
				zap.String("discovered file relative path", discoveredFile.RelativePath))
			continue
		}
		if trackedByAbsPath {
			log.Debug("discovered file absolute path matches monitored movie file original path",
				zap.String("discovered file absolute path", discoveredFile.AbsolutePath))
			continue
		}

		mf := model.MovieFile{
			OriginalFilePath: &discoveredFile.RelativePath, // this should always be relative if we discovered it in the library.. surely
			RelativePath:     &discoveredFile.RelativePath, // TODO: make sure it's actually relative
			Size:             discoveredFile.Size,
		}

		log.Debug("discovered new movie file", zap.String("path", discoveredFile.RelativePath))

		id, err := m.movieStorage.CreateMovieFile(ctx, mf)
		if err != nil {
			log.Error("couldn't store movie file", zap.Error(err))
			continue
		}

		log.Debug("created new movie file in storage", zap.Int64("movie file id", id))
	}

	// pull the updated movie file list in case we added anything above
	movieFiles, err = m.movieStorage.ListMovieFiles(ctx)
	if err != nil {
		return fmt.Errorf("failed to list movie files: %w", err)
	}

	for _, f := range movieFiles {
		movieName := library.MovieNameFromFilepath(*f.RelativePath)
		foundMovie, err := m.movieStorage.GetMovieByPath(ctx, movieName)
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

		_, err = m.movieStorage.CreateMovie(ctx, movie, storage.MovieStateDiscovered)
		if err != nil {
			log.Error("couldn't create new movie for discovered file", zap.Error(err))
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

	movie, err := m.movieStorage.GetMovieByMetadataID(ctx, int(det.ID))
	// if we find the movie we're done
	if err == nil {
		return movie, err
	}

	// anything other than a not found error is an internal error
	if !errors.Is(err, storage.ErrNotFound) {
		log.Warn("couldn't find movie by metadata", zap.Int32("meta_id", det.ID), zap.Error(err))
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

	state := initialMovieState(det.ReleaseDate)

	id, err := m.movieStorage.CreateMovie(ctx, *movie, state)
	if err != nil {
		log.Warn("failed to create movie", zap.Error(err))
		return nil, err
	}

	log.Debug("created movie", zap.Any("movie", movie))

	movie, err = m.movieStorage.GetMovie(ctx, id)
	if err != nil {
		log.Warn("failed to get created movie", zap.Error(err))
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

	series, err := m.seriesStorage.GetSeries(ctx, table.Series.ID.EQ(sqlite.Int32(seriesMetadata.ID)))
	// if we find the series we dont need to add it
	if err == nil {
		return series, err
	}
	if !errors.Is(err, storage.ErrNotFound) {
		log.Warn("couldn't find series by metadata", zap.Int32("meta_id", seriesMetadata.ID), zap.Error(err))
		return nil, err
	}

	monitorNewSeasons := int32(0)
	if request.MonitorNewSeasons {
		monitorNewSeasons = 1
	}
	series = &storage.Series{
		Series: model.Series{
			SeriesMetadataID:  &seriesMetadata.ID,
			QualityProfileID:  qualityProfile.ID,
			Monitored:         1,
			Path:              &seriesMetadata.Title,
			MonitorNewSeasons: monitorNewSeasons,
		},
	}

	state := initialSeriesState(seriesMetadata.FirstAirDate)

	seriesID, err := m.seriesStorage.CreateSeries(ctx, *series, state)
	if err != nil {
		log.Error("failed to create new missing series", zap.Error(err))
		return nil, err
	}

	log.Debug("created new missing series", zap.Any("series", series))

	// Get series to access its metadata ID
	seriesEntity, err := m.seriesStorage.GetSeries(ctx, table.Series.ID.EQ(sqlite.Int(seriesID)))
	if err != nil || seriesEntity.SeriesMetadataID == nil {
		log.Error("failed to get series or series has no metadata")
		return nil, fmt.Errorf("series has no metadata")
	}

	where := table.SeasonMetadata.SeriesMetadataID.EQ(sqlite.Int32(*seriesEntity.SeriesMetadataID))
	seasonMetadata, err := m.seriesMetaStorage.ListSeasonMetadata(ctx, where)
	if err != nil {
		return nil, err
	}

	for _, s := range seasonMetadata {
		season := storage.Season{
			Season: model.Season{
				SeriesID:         int32(seriesID),
				SeasonMetadataID: ptr.To(s.ID),
				Monitored:        1,
			},
		}

		seasonID, err := m.seriesStorage.CreateSeason(ctx, season, storage.SeasonStateMissing)
		if err != nil {
			log.Error("failed to create season", zap.Error(err))
			return nil, err
		}

		log.Debug("created new missing season", zap.Any("season", season))

		// Get the season to access its metadata ID for proper episode metadata querying
		seasonEntity, err := m.seriesStorage.GetSeason(ctx, table.Season.ID.EQ(sqlite.Int64(seasonID)))
		if err != nil || seasonEntity.SeasonMetadataID == nil {
			log.Error("failed to get season or season has no metadata linked")
			return nil, fmt.Errorf("season has no metadata")
		}

		where := table.EpisodeMetadata.SeasonMetadataID.EQ(sqlite.Int32(*seasonEntity.SeasonMetadataID))

		episodesMetadata, err := m.seriesMetaStorage.ListEpisodeMetadata(ctx, where)
		if err != nil {
			log.Error("failed to list episode metadata", zap.Error(err))
			return nil, err
		}

		for _, e := range episodesMetadata {
			episode := storage.Episode{
				Episode: model.Episode{
					EpisodeMetadataID: ptr.To(e.ID),
					SeasonID:          int32(seasonID),
					Monitored:         1,
					EpisodeNumber:     e.Number,
				},
			}

			_, err := m.seriesStorage.CreateEpisode(ctx, episode, storage.EpisodeStateMissing)
			if err != nil {
				log.Error("failed to create episode", zap.Error(err))
				return nil, err
			}

			log.Debug("created new missing episode", zap.Any("episode", episode))
		}
	}

	series, err = m.seriesStorage.GetSeries(ctx, table.Series.ID.EQ(sqlite.Int64(seriesID)))
	if err != nil {
		log.Warn("failed to get created series", zap.Error(err))
	}

	return series, err
}

func (m MediaManager) DeleteMovie(ctx context.Context, movieID int64, deleteFiles bool) error {
	log := logger.FromCtx(ctx)

	movie, err := m.movieStorage.GetMovie(ctx, movieID)
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

	if err := m.movieStorage.DeleteMovie(ctx, movieID); err != nil {
		return fmt.Errorf("failed to delete movie: %w", err)
	}

	log.Info("deleted movie", zap.Int64("id", movieID), zap.Bool("files_deleted", deleteFiles))
	return nil
}

func (m MediaManager) DeleteSeries(ctx context.Context, seriesID int64, deleteDirectory bool) error {
	log := logger.FromCtx(ctx)

	series, err := m.seriesStorage.GetSeries(ctx, table.Series.ID.EQ(sqlite.Int64(seriesID)))
	if err != nil {
		return fmt.Errorf("failed to get series: %w", err)
	}

	seasons, err := m.seriesStorage.ListSeasons(ctx, table.Season.SeriesID.EQ(sqlite.Int64(seriesID)))
	if err != nil {
		log.Warn("failed to get seasons for cleanup", zap.Error(err))
	}

	for _, season := range seasons {
		episodes, err := m.seriesStorage.ListEpisodes(ctx, table.Episode.SeasonID.EQ(sqlite.Int64(int64(season.ID))))
		if err != nil {
			log.Warn("failed to get episodes for cleanup", zap.Int32("season_id", season.ID), zap.Error(err))
			continue
		}
		for _, episode := range episodes {
			if episode.EpisodeFileID == nil {
				continue
			}
			if err := m.seriesStorage.DeleteEpisodeFile(ctx, int64(*episode.EpisodeFileID)); err != nil {
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

	if err := m.seriesStorage.DeleteSeries(ctx, seriesID); err != nil {
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
	err := m.movieStorage.UpdateMovie(ctx, movieUpdate, table.Movie.ID.EQ(sqlite.Int64(movieID)))
	if err != nil {
		return nil, err
	}

	movie, err := m.movieStorage.GetMovie(ctx, movieID)
	if err != nil {
		return nil, err
	}

	logger.FromCtx(ctx).Info("updated monitoring", zap.Int64("movie_id", movieID), zap.Bool("monitored", monitored))
	return movie, nil
}

func (m MediaManager) UpdateMovieQualityProfile(ctx context.Context, movieID int64, qualityProfileID int32) (*storage.Movie, error) {
	err := m.movieStorage.UpdateMovieQualityProfile(ctx, movieID, qualityProfileID)
	if err != nil {
		return nil, err
	}

	movie, err := m.movieStorage.GetMovie(ctx, movieID)
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
	err := m.seriesStorage.UpdateSeries(ctx, seriesUpdate, table.Series.ID.EQ(sqlite.Int64(seriesID)))
	if err != nil {
		return nil, err
	}

	series, err := m.seriesStorage.GetSeries(ctx, table.Series.ID.EQ(sqlite.Int64(seriesID)))
	if err != nil {
		return nil, err
	}

	logger.FromCtx(ctx).Info("updated monitoring", zap.Int64("series_id", seriesID), zap.Bool("monitored", monitored))
	return series, nil
}

func (m MediaManager) AddIndexer(ctx context.Context, request AddIndexerRequest) (IndexerResponse, error) {
	return m.indexerService.AddIndexer(ctx, request)
}

func (m MediaManager) UpdateIndexer(ctx context.Context, id int32, request UpdateIndexerRequest) (IndexerResponse, error) {
	return m.indexerService.UpdateIndexer(ctx, id, request)
}

func (m MediaManager) DeleteIndexer(ctx context.Context, request DeleteIndexerRequest) error {
	return m.indexerService.DeleteIndexer(ctx, request)
}

func (m MediaManager) CreateIndexerSource(ctx context.Context, req AddIndexerSourceRequest) (IndexerSourceResponse, error) {
	return m.indexerService.CreateIndexerSource(ctx, req)
}

func (m MediaManager) ListIndexerSources(ctx context.Context) ([]IndexerSourceResponse, error) {
	return m.indexerService.ListIndexerSources(ctx)
}

func (m MediaManager) GetIndexerSource(ctx context.Context, id int64) (IndexerSourceResponse, error) {
	return m.indexerService.GetIndexerSource(ctx, id)
}

func (m MediaManager) UpdateIndexerSource(ctx context.Context, id int64, req UpdateIndexerSourceRequest) (IndexerSourceResponse, error) {
	return m.indexerService.UpdateIndexerSource(ctx, id, req)
}

func (m MediaManager) DeleteIndexerSource(ctx context.Context, id int64) error {
	return m.indexerService.DeleteIndexerSource(ctx, id)
}

func (m MediaManager) TestIndexerSource(ctx context.Context, req AddIndexerSourceRequest) error {
	return m.indexerService.TestIndexerSource(ctx, req)
}

func (m MediaManager) RefreshIndexerSource(ctx context.Context, id int64) error {
	return m.indexerService.RefreshIndexerSource(ctx, id)
}

func (m MediaManager) RefreshAllIndexerSources(ctx context.Context) error {
	return m.indexerService.RefreshAllIndexerSources(ctx)
}

func (m MediaManager) SearchIndexers(ctx context.Context, indexers, categories []int32, opts indexer.SearchOptions) ([]*prowlarr.ReleaseResource, error) {
	return m.indexerService.SearchIndexers(ctx, indexers, categories, opts)
}

func nullableDefault[T any](n nullable.Nullable[T]) T {
	var def T
	if n.IsSpecified() {
		v, _ := n.Get()
		return v
	}

	return def
}

// initialMovieState returns Missing or Unreleased based on the release date.
func initialMovieState(releaseDate *time.Time) storage.MovieState {
	if !isReleased(now(), releaseDate) {
		return storage.MovieStateUnreleased
	}
	return storage.MovieStateMissing
}

// initialSeriesState returns Missing or Unreleased based on the first air date.
func initialSeriesState(firstAirDate *time.Time) storage.SeriesState {
	if !isReleased(now(), firstAirDate) {
		return storage.SeriesStateUnreleased
	}
	return storage.SeriesStateMissing
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

// filterAndMap applies fn to each non-nil element of items, collecting results where fn returns ok=true.
func filterAndMap[T any, R any](items []*T, fn func(*T) (R, bool)) []R {
	var result []R
	for _, item := range items {
		if item == nil {
			continue
		}
		if r, ok := fn(item); ok {
			result = append(result, r)
		}
	}
	return result
}

// GetMovieMetadataByID retrieves movie metadata by its primary key
func (m MediaManager) GetMovieMetadataByID(ctx context.Context, metadataID int32) (*model.MovieMetadata, error) {
	// fetch metadata record
	return m.movieMetaStorage.GetMovieMetadata(ctx, table.MovieMetadata.ID.EQ(sqlite.Int(int64(metadataID))))
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
	series, err := m.seriesStorage.GetSeries(ctx,
		table.Series.SeriesMetadataID.EQ(sqlite.Int32(metadata.ID)))
	if err != nil {
		log.Error("failed to get series", zap.Error(err), zap.Int32("metadataID", metadata.ID))
		return nil, err
	}

	// Query seasons with metadata join
	seasons, err := m.seriesStorage.ListSeasons(ctx,
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

		seasonMeta, err := m.seriesMetaStorage.GetSeasonMetadata(ctx,
			table.SeasonMetadata.ID.EQ(sqlite.Int32(*season.SeasonMetadataID)))
		if err != nil {
			log.Error("failed to get season metadata", zap.Error(err), zap.Int32("seasonMetadataID", *season.SeasonMetadataID))
			continue
		}

		// Count episodes for this season
		episodes, err := m.seriesStorage.ListEpisodes(ctx,
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
	series, err := m.seriesStorage.GetSeries(ctx,
		table.Series.SeriesMetadataID.EQ(sqlite.Int32(seriesMetadata.ID)))
	if err != nil {
		log.Error("failed to get series", zap.Error(err), zap.Int32("metadataID", seriesMetadata.ID))
		return nil, err
	}

	// Find all seasons for this series
	seasons, err := m.seriesStorage.ListSeasons(ctx,
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
			meta, err := m.seriesMetaStorage.GetSeasonMetadata(ctx,
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
			episodes, err := m.seriesStorage.ListEpisodes(ctx,
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
	episodes, err := m.seriesStorage.ListEpisodes(ctx,
		table.Episode.SeasonID.EQ(sqlite.Int32(targetSeason.ID)))
	if err != nil {
		log.Error("failed to list episodes", zap.Error(err), zap.Int32("seasonID", targetSeason.ID))
		return nil, err
	}

	// Determine season number for response
	seasonNum := int32(seasonNumber)
	if seasonMeta != nil {
		seasonNum = seasonMeta.Number
	}

	results := make([]EpisodeResult, 0, len(episodes))
	metaMap := m.preloadEpisodeMetadata(ctx, episodes)
	for _, episode := range episodes {
		var episodeMeta *model.EpisodeMetadata
		if episode.EpisodeMetadataID != nil {
			episodeMeta = metaMap[*episode.EpisodeMetadataID]
		}
		results = append(results, buildEpisodeResult(episode, episodeMeta, series.ID, seasonNum))
	}

	return results, nil
}

func (m MediaManager) CreateJob(ctx context.Context, request TriggerJobRequest) (JobResponse, error) {
	return m.jobService.CreateJob(ctx, request)
}

func (m MediaManager) GetJob(ctx context.Context, id int64) (JobResponse, error) {
	return m.jobService.GetJob(ctx, id)
}

func (m MediaManager) ListJobs(ctx context.Context, jobType *string, state *string, params pagination.Params) (JobListResponse, error) {
	return m.jobService.ListJobs(ctx, jobType, state, params)
}

func (m MediaManager) CancelJob(ctx context.Context, id int64) (JobResponse, error) {
	return m.jobService.CancelJob(ctx, id)
}

func (m MediaManager) AddQualityDefinition(ctx context.Context, request AddQualityDefinitionRequest) (model.QualityDefinition, error) {
	return m.qualityService.AddQualityDefinition(ctx, request)
}

func (m MediaManager) DeleteQualityDefinition(ctx context.Context, request DeleteQualityDefinitionRequest) error {
	return m.qualityService.DeleteQualityDefinition(ctx, request)
}

func (m MediaManager) ListQualityDefinitions(ctx context.Context) ([]*model.QualityDefinition, error) {
	return m.qualityService.ListQualityDefinitions(ctx)
}

func (m MediaManager) GetQualityDefinition(ctx context.Context, id int64) (model.QualityDefinition, error) {
	return m.qualityService.GetQualityDefinition(ctx, id)
}

func (m MediaManager) GetQualityProfile(ctx context.Context, id int64) (storage.QualityProfile, error) {
	return m.qualityService.GetQualityProfile(ctx, id)
}

func (m MediaManager) ListEpisodeQualityProfiles(ctx context.Context) ([]*storage.QualityProfile, error) {
	return m.qualityService.ListEpisodeQualityProfiles(ctx)
}

func (m MediaManager) ListMovieQualityProfiles(ctx context.Context) ([]*storage.QualityProfile, error) {
	return m.qualityService.ListMovieQualityProfiles(ctx)
}

func (m MediaManager) ListQualityProfiles(ctx context.Context) ([]*storage.QualityProfile, error) {
	return m.qualityService.ListQualityProfiles(ctx)
}

func (m MediaManager) UpdateQualityDefinition(ctx context.Context, id int64, request UpdateQualityDefinitionRequest) (model.QualityDefinition, error) {
	return m.qualityService.UpdateQualityDefinition(ctx, id, request)
}

func (m MediaManager) AddQualityProfile(ctx context.Context, request AddQualityProfileRequest) (storage.QualityProfile, error) {
	return m.qualityService.AddQualityProfile(ctx, request)
}

func (m MediaManager) UpdateQualityProfile(ctx context.Context, id int64, request UpdateQualityProfileRequest) (storage.QualityProfile, error) {
	return m.qualityService.UpdateQualityProfile(ctx, id, request)
}

func (m MediaManager) DeleteQualityProfile(ctx context.Context, request DeleteQualityProfileRequest) error {
	return m.qualityService.DeleteQualityProfile(ctx, request)
}

func (m MediaManager) CreateDownloadClient(ctx context.Context, request AddDownloadClientRequest) (model.DownloadClient, error) {
	return m.downloadClientService.CreateDownloadClient(ctx, request)
}

func (m MediaManager) UpdateDownloadClient(ctx context.Context, id int64, request UpdateDownloadClientRequest) (model.DownloadClient, error) {
	return m.downloadClientService.UpdateDownloadClient(ctx, id, request)
}

func (m MediaManager) TestDownloadClient(ctx context.Context, request AddDownloadClientRequest) error {
	return m.downloadClientService.TestDownloadClient(ctx, request)
}

func (m MediaManager) GetDownloadClient(ctx context.Context, id int64) (model.DownloadClient, error) {
	return m.downloadClientService.GetDownloadClient(ctx, id)
}

func (m MediaManager) ListDownloadClients(ctx context.Context) ([]*model.DownloadClient, error) {
	return m.downloadClientService.ListDownloadClients(ctx)
}

func (m MediaManager) DeleteDownloadClient(ctx context.Context, id int64) error {
	return m.downloadClientService.DeleteDownloadClient(ctx, id)
}
