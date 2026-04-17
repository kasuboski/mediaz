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
	"github.com/kasuboski/mediaz/pkg/cache"
	"github.com/kasuboski/mediaz/pkg/download"
	"github.com/kasuboski/mediaz/pkg/indexer"
	"github.com/kasuboski/mediaz/pkg/library"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/pagination"
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
	indexerFactory        indexer.Factory
	indexerCache          *cache.Cache[int64, indexerCacheEntry]
	library               library.Library
	movieStorage          storage.MovieStorage
	movieMetaStorage      storage.MovieMetadataStorage
	seriesStorage         storage.SeriesStorage
	seriesMetaStorage     storage.SeriesMetadataStorage
	indexerStorage        storage.IndexerStorage
	indexerSrcStorage     storage.IndexerSourceStorage
	downloadClientService *DownloadClientService
	qualityService        *QualityService
	jobService            *JobService
	seriesService         *SeriesService
	config                config.Config
	configs               config.Manager
}

func New(tmbdClient tmdb.ITmdb, indexerFactory indexer.Factory, library library.Library, store storage.Storage, factory download.Factory, managerConfigs config.Manager, fullConfig config.Config) MediaManager {

	m := MediaManager{
		tmdb:                  tmbdClient,
		indexerFactory:        indexerFactory,
		indexerCache:          cache.New[int64, indexerCacheEntry](),
		library:               library,
		movieStorage:          store,
		movieMetaStorage:      store,
		seriesStorage:         store,
		seriesMetaStorage:     store,
		indexerStorage:        store,
		indexerSrcStorage:     store,
		downloadClientService: NewDownloadClientService(store, factory),
		qualityService:        NewQualityService(store),
		config:                fullConfig,
		configs:               managerConfigs,
	}

	m.seriesService = NewSeriesService(tmbdClient, library, store, store, m.qualityService, &m)

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
	return m.seriesService.GetTVDetailByTMDBID(ctx, tmdbID)
}

// buildTVDetailResult is delegated to SeriesService
func (m MediaManager) buildTVDetailResult(metadata *model.SeriesMetadata, details *tmdb.SeriesDetailsResponse, series *storage.Series, seasons []SeasonResult) *TVDetailResult {
	return m.seriesService.buildTVDetailResult(metadata, details, series, seasons)
}

// getSeasonsWithEpisodes retrieves seasons and their episodes for a series
func (m MediaManager) getSeasonsWithEpisodes(ctx context.Context, seriesID int32) ([]SeasonResult, error) {
	return m.seriesService.getSeasonsWithEpisodes(ctx, seriesID)
}

// preloadEpisodeMetadata fetches all episode metadata for the given IDs in a single query.
func (m MediaManager) preloadEpisodeMetadata(ctx context.Context, episodes []*storage.Episode) map[int32]*model.EpisodeMetadata {
	return m.seriesService.preloadEpisodeMetadata(ctx, episodes)
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
	return m.seriesService.getEpisodesForSeason(ctx, seasonID, seriesID, seasonNumber)
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

// SearchTV queries TMDB for TV shows
func (m MediaManager) SearchTV(ctx context.Context, query string) (*SearchMediaResponse, error) {
	return m.seriesService.SearchTV(ctx, query)
}

func (m MediaManager) GetSeriesDetails(ctx context.Context, tmdbID int) (model.SeriesMetadata, error) {
	return m.seriesService.GetSeriesDetails(ctx, tmdbID)
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

		for _, idx := range cached.Indexers {
			all = append(all, model.Indexer{
				ID:              idx.ID,
				IndexerSourceID: ptr.To(int32(sourceID)),
				Name:            idx.Name,
				Priority:        idx.Priority,
				URI:             idx.URI,
				APIKey:          nil,
			})
		}
	}

	dbIndexers, err := m.indexerStorage.ListIndexers(ctx)
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

	dbIndexers, err := m.indexerStorage.ListIndexers(ctx)
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
	return m.seriesService.ListShowsInLibrary(ctx)
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
	return m.seriesService.AddSeriesToLibrary(ctx, request)
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
	return m.seriesService.DeleteSeries(ctx, seriesID, deleteDirectory)
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
	return m.seriesService.UpdateSeriesMonitored(ctx, seriesID, monitored)
}

func (m MediaManager) AddIndexer(ctx context.Context, request AddIndexerRequest) (IndexerResponse, error) {
	indexer := request.Indexer

	if indexer.Name == "" {
		return IndexerResponse{}, fmt.Errorf("%w: indexer name is required", ErrValidation)
	}

	id, err := m.indexerStorage.CreateIndexer(ctx, indexer)
	if err != nil {
		return IndexerResponse{}, err
	}

	indexer.ID = int32(id)

	return toIndexerResponse(indexer), nil
}

func (m MediaManager) UpdateIndexer(ctx context.Context, id int32, request UpdateIndexerRequest) (IndexerResponse, error) {
	if request.Name == "" {
		return IndexerResponse{}, fmt.Errorf("%w: indexer name is required", ErrValidation)
	}

	indexer := model.Indexer{
		ID:       id,
		Name:     request.Name,
		Priority: request.Priority,
		URI:      request.URI,
		APIKey:   request.APIKey,
	}

	err := m.indexerStorage.UpdateIndexer(ctx, int64(id), indexer)
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

	return m.indexerStorage.DeleteIndexer(ctx, int64(*request.ID))
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

func toIndexerResponse(indexer model.Indexer) IndexerResponse {
	return IndexerResponse{
		ID:       indexer.ID,
		Name:     indexer.Name,
		Priority: indexer.Priority,
		URI:      indexer.URI,
	}
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
	return m.seriesService.ListSeasonsForSeries(ctx, tmdbID)
}

// ListEpisodesForSeason retrieves all episodes for a season by TMDB ID and season number
func (m MediaManager) ListEpisodesForSeason(ctx context.Context, tmdbID int, seasonNumber int) ([]EpisodeResult, error) {
	return m.seriesService.ListEpisodesForSeason(ctx, tmdbID, seasonNumber)
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
