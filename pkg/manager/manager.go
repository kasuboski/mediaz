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
	"github.com/kasuboski/mediaz/pkg/download"
	"github.com/kasuboski/mediaz/pkg/library"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
	"github.com/kasuboski/mediaz/pkg/tmdb"
	"github.com/oapi-codegen/nullable"
	"go.uber.org/zap"
)

type MediaManager struct {
	tmdb    tmdb.ITmdb
	indexer IndexerStore
	library library.Library
	storage storage.Storage
	factory download.Factory
	configs config.Manager
}

func New(tmbdClient tmdb.ITmdb, prowlarrClient prowlarr.IProwlarr, library library.Library, storage storage.Storage, factory download.Factory, managerConfigs config.Manager) MediaManager {
	return MediaManager{
		tmdb:    tmbdClient,
		indexer: NewIndexerStore(prowlarrClient, storage),
		library: library,
		storage: storage,
		factory: factory,
		configs: managerConfigs,
	}
}

func now() time.Time {
	return time.Now()
}

type SearchMediaResponse struct {
	Page         *int                 `json:"page,omitempty"`
	TotalPages   *int                 `json:"total_pages,omitempty"`
	TotalResults *int                 `json:"total_results,omitempty"`
	Results      []*SearchMediaResult `json:"results,omitempty"`
}

type SearchMediaResult struct {
	Adult            *bool    `json:"adult,omitempty"`
	BackdropPath     *string  `json:"backdrop_path,omitempty"`
	GenreIds         *[]int   `json:"genre_ids,omitempty"`
	ID               *int     `json:"id,omitempty"`
	OriginalLanguage *string  `json:"original_language,omitempty"`
	OriginalTitle    *string  `json:"original_title,omitempty"`
	Overview         *string  `json:"overview,omitempty"`
	Popularity       *float32 `json:"popularity,omitempty"`
	PosterPath       *string  `json:"poster_path,omitempty"`
	ReleaseDate      *string  `json:"release_date,omitempty"`
	Title            *string  `json:"title,omitempty"`
	Video            *bool    `json:"video,omitempty"`
	VoteAverage      *float32 `json:"vote_average,omitempty"`
	VoteCount        *int     `json:"vote_count,omitempty"`
}

// MovieDetailResult provides detailed information for a single movie
type MovieDetailResult struct {
	// Core identifiers and basic info
	TMDBID           int32   `json:"tmdbID"`
	ImdbID           *string `json:"imdbID,omitempty"`
	Title            string  `json:"title"`
	OriginalTitle    *string `json:"originalTitle,omitempty"`
	Overview         *string `json:"overview,omitempty"`
	
	// Media assets
	PosterPath       string  `json:"posterPath,omitempty"`
	BackdropPath     *string `json:"backdropPath,omitempty"`
	
	// Release and timing info
	ReleaseDate      *string `json:"releaseDate,omitempty"`
	Year             *int32  `json:"year,omitempty"`
	Runtime          *int32  `json:"runtime,omitempty"`
	
	// Content classification and ratings
	Adult            *bool    `json:"adult,omitempty"`
	VoteAverage      *float32 `json:"voteAverage,omitempty"`
	VoteCount        *int     `json:"voteCount,omitempty"`
	Popularity       *float64 `json:"popularity,omitempty"`
	
	// Genre and production info
	Genres           *string `json:"genres,omitempty"`
	Studio           *string `json:"studio,omitempty"`
	Website          *string `json:"website,omitempty"`
	
	// Collection info
	CollectionTmdbID *int32  `json:"collectionTmdbID,omitempty"`
	CollectionTitle  *string `json:"collectionTitle,omitempty"`
	
	// Library status
	LibraryStatus    string  `json:"libraryStatus"` // Available, Missing, Requested, etc.
	Path             *string `json:"path,omitempty"`
	QualityProfileID *int32  `json:"qualityProfileID,omitempty"`
	Monitored        *bool   `json:"monitored,omitempty"`
}

// TVDetailResult provides detailed information for a single TV show
type TVDetailResult struct {
	// Core identifiers and basic info
	TMDBID           int32   `json:"tmdbID"`
	Title            string  `json:"title"`
	OriginalTitle    *string `json:"originalTitle,omitempty"`
	Overview         *string `json:"overview,omitempty"`
	
	// Media assets
	PosterPath       string  `json:"posterPath,omitempty"`
	BackdropPath     *string `json:"backdropPath,omitempty"`
	
	// TV-specific timing info
	FirstAirDate     *string `json:"firstAirDate,omitempty"`
	LastAirDate      *string `json:"lastAirDate,omitempty"`
	
	// TV-specific metadata
	Networks         []string `json:"networks,omitempty"`
	SeasonCount      int32   `json:"seasonCount"`
	EpisodeCount     int32   `json:"episodeCount"`
	
	// Content classification and ratings
	Adult            *bool    `json:"adult,omitempty"`
	VoteAverage      *float32 `json:"voteAverage,omitempty"`
	VoteCount        *int     `json:"voteCount,omitempty"`
	Popularity       *float64 `json:"popularity,omitempty"`
	
	// Genre and production info
	Genres           []string `json:"genres,omitempty"`
	
	// Library status
	LibraryStatus    string  `json:"libraryStatus"` // Available, Missing, Requested, etc.
	Path             *string `json:"path,omitempty"`
	QualityProfileID *int32  `json:"qualityProfileID,omitempty"`
	Monitored        *bool   `json:"monitored,omitempty"`
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
		// Movie exists in library - add library status information
		result.LibraryStatus = string(movie.State)
		result.Path = movie.Path
		result.QualityProfileID = &movie.QualityProfileID
		monitored := movie.Monitored == 1
		result.Monitored = &monitored
	} else if !errors.Is(err, storage.ErrNotFound) {
		// Log non-NotFound errors but don't fail the request
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

	// Transform data into result
	result := m.buildTVDetailResult(metadata, seriesDetailsResponse, series)
	
	return result, nil
}

// getTVMetadataAndDetails retrieves both series metadata and full TMDB details
func (m MediaManager) getTVMetadataAndDetails(ctx context.Context, tmdbID int) (*model.SeriesMetadata, *tmdb.SeriesDetailsResponse, error) {
	// Get series metadata (creates if not exists)
	metadata, err := m.GetSeriesMetadata(ctx, tmdbID)
	if err != nil {
		return nil, nil, err
	}

	// Get the full series details response from TMDB to access networks and status
	res, err := m.tmdb.TvSeriesDetails(ctx, int32(tmdbID), nil)
	if err != nil {
		return nil, nil, err
	}
	defer res.Body.Close()

	b, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, nil, err
	}

	var seriesDetailsResponse tmdb.SeriesDetailsResponse
	err = json.Unmarshal(b, &seriesDetailsResponse)
	if err != nil {
		return nil, nil, err
	}

	return metadata, &seriesDetailsResponse, nil
}

// buildTVDetailResult transforms metadata and TMDB details into TVDetailResult
func (m MediaManager) buildTVDetailResult(metadata *model.SeriesMetadata, details *tmdb.SeriesDetailsResponse, series *storage.Series) *TVDetailResult {
	result := &TVDetailResult{
		TMDBID:        metadata.TmdbID,
		Title:         metadata.Title,
		Overview:      metadata.Overview,
		PosterPath:    details.PosterPath,
		SeasonCount:   metadata.SeasonCount,
		EpisodeCount:  metadata.EpisodeCount,
		LibraryStatus: "Not In Library", // Default status
	}

	// Set backdrop path only if not empty
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

	// Extract network names
	if len(details.Networks) > 0 {
		var networks []string
		for _, network := range details.Networks {
			networks = append(networks, network.Name)
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

	// Set library status information if series exists
	if series != nil {
		result.LibraryStatus = string(series.State)
		result.Path = series.Path
		result.QualityProfileID = &series.QualityProfileID
		monitored := series.Monitored == 1
		result.Monitored = &monitored
	}

	return result
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

// ListIndexers lists all managed indexers
func (m MediaManager) ListIndexers(ctx context.Context) ([]Indexer, error) {
	log := logger.FromCtx(ctx)

	if err := m.indexer.FetchIndexers(ctx); err != nil {
		log.Error("couldn't fetch indexer", err)
	}
	return m.indexer.ListIndexers(ctx)
}

func (m MediaManager) ListShowsInLibrary(ctx context.Context) ([]string, error) {
	return m.library.FindEpisodes(ctx)
}

// LibraryMovie is used for API responses, combining file and metadata info
type LibraryMovie struct {
	Path       string `json:"path"`
	TMDBID     int32  `json:"tmdbID"`
	Title      string `json:"title"`
	PosterPath string `json:"poster_path"`
	Year       int32  `json:"year,omitempty"`
	State      string `json:"state"`
}

// ListMoviesInLibrary returns library movies enriched with metadata
func (m MediaManager) ListMoviesInLibrary(ctx context.Context) ([]LibraryMovie, error) {
	// fetch by state
	discovered, err := m.storage.ListMoviesByState(ctx, storage.MovieStateDiscovered)
	if err != nil {
		return nil, err
	}
	downloaded, err := m.storage.ListMoviesByState(ctx, storage.MovieStateDownloaded)
	if err != nil {
		return nil, err
	}
	all := append(discovered, downloaded...)
	var movies []LibraryMovie
	for _, mp := range all {
		mrec := *mp
		lm := LibraryMovie{State: string(mrec.State)}
		if mrec.Path != nil {
			lm.Path = *mrec.Path
		}
		if mrec.MovieMetadataID != nil {
			meta, err := m.GetMovieMetadataByID(ctx, *mrec.MovieMetadataID)
			if err == nil && meta != nil {
				lm.TMDBID = meta.TmdbID
				lm.Title = meta.Title
				lm.PosterPath = meta.Images
				if meta.Year != nil {
					lm.Year = *meta.Year
				}
			}
		}
		movies = append(movies, lm)
	}
	return movies, nil
}

func (m MediaManager) Run(ctx context.Context) error {
	log := logger.FromCtx(ctx)

	movieIndexTicker := time.NewTicker(m.configs.Jobs.MovieIndex)
	defer movieIndexTicker.Stop()
	movieIndexerLock := new(sync.Mutex)

	movieReconcileTicker := time.NewTicker(m.configs.Jobs.MovieReconcile)
	defer movieReconcileTicker.Stop()
	movieReconcileLock := new(sync.Mutex)

	seriesReconcileTicker := time.NewTicker(m.configs.Jobs.SeriesReconcile)
	defer seriesReconcileTicker.Stop()
	seriesReconcileLock := new(sync.Mutex)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-movieIndexTicker.C:
			if !movieIndexerLock.TryLock() {
				continue
			}

			go lock(movieIndexerLock, func() {
				err := m.IndexMovieLibrary(ctx)
				if err != nil {
					log.Errorf("movie library indexing failed", zap.Error(err))
				}
			})

		case <-movieReconcileTicker.C:
			if !movieReconcileLock.TryLock() {
				continue
			}

			go lock(movieReconcileLock, func() {
				err := m.ReconcileMovies(ctx)
				if err != nil {
					log.Errorf("movie reconcile failed", zap.Error(err))
				}
			})

		case <-seriesReconcileTicker.C:
			if !seriesReconcileLock.TryLock() {
				continue
			}

			go lock(seriesReconcileLock, func() {
				err := m.ReconcileSeries(ctx)
				if err != nil {
					log.Errorf("series reconcile failed", zap.Error(err))
				}
			})
		}
	}
}

func lock(mu *sync.Mutex, fn func()) {
	if mu == nil {
		return
	}
	defer mu.Unlock()
	fn()
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
					zap.String("discovered file absolute path", discoveredFile.RelativePath),
					zap.String("monitored file original path", *mf.RelativePath))
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
				Monitored:   1,
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

// AddMovieRequest describes what is required to add a movie to a library
type AddMovieRequest struct {
	TMDBID           int   `json:"tmdbID"`
	QualityProfileID int32 `json:"qualityProfileID"`
}

// AddSeriesRequest describes what is required to add a series to a library
type AddSeriesRequest struct {
	TMDBID           int   `json:"tmdbID"`
	QualityProfileID int32 `json:"qualityProfileID"`
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

	series, err := m.storage.GetSeries(ctx, table.Series.ID.EQ(sqlite.Int32(seriesMetadata.ID)))
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
			Monitored:        1,
			TmdbID:           int32(seriesMetadata.TmdbID),
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

	where := table.SeasonMetadata.SeriesID.EQ(sqlite.Int(seriesID))
	seasonMetadata, err := m.storage.ListSeasonMetadata(ctx, where)
	if err != nil {
		return nil, err
	}

	for _, s := range seasonMetadata {
		season := storage.Season{
			Season: model.Season{
				SeriesID:         int32(seriesID),
				SeasonMetadataID: ptr(s.ID),
				Monitored:        1,
			},
		}

		seasonID, err := m.storage.CreateSeason(ctx, season, storage.SeasonStateMissing)
		if err != nil {
			log.Error("failed to create season", zap.Error(err))
			return nil, err
		}

		log.Debug("created new missing season", zap.Any("season", season))

		where := table.EpisodeMetadata.SeasonID.EQ(sqlite.Int64(seasonID))

		episodesMetadata, err := m.storage.ListEpisodeMetadata(ctx, where)
		if err != nil {
			log.Error("failed to list episode metadata", zap.Error(err))
			return nil, err
		}

		for _, e := range episodesMetadata {
			episode := storage.Episode{
				Episode: model.Episode{
					EpisodeMetadataID: ptr(e.ID),
					SeasonID:          int32(seasonID),
					Monitored:         1,
					Runtime:           e.Runtime,
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
	}

	series, err = m.storage.GetSeries(ctx, table.Series.ID.EQ(sqlite.Int64(seriesID)))
	if err != nil {
		log.Warnw("failed to get created series", "err", err)
	}

	return series, err
}

func (m MediaManager) SearchIndexers(ctx context.Context, indexers, categories []int32, query string) ([]*prowlarr.ReleaseResource, error) {
	var wg sync.WaitGroup

	var indexerError error
	releases := make([]*prowlarr.ReleaseResource, 0, 50)
	for _, indexer := range indexers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := m.indexer.searchIndexer(ctx, indexer, categories, query)
			if err != nil {
				indexerError = errors.Join(indexerError, err)
				return
			}

			releases = append(releases, res...)
		}()
	}
	wg.Wait()

	if len(releases) == 0 && indexerError != nil {
		// only return an error if no releases found and there was an error
		return nil, indexerError
	}

	return releases, nil
}

// AddIndexerRequest describes what is required to add an indexer
type AddIndexerRequest struct {
	model.Indexer
}

// AddIndexer stores a new indexer in the database
func (m MediaManager) AddIndexer(ctx context.Context, request AddIndexerRequest) (model.Indexer, error) {
	indexer := request.Indexer

	if indexer.Name == "" {
		return indexer, fmt.Errorf("indexer name is required")
	}

	id, err := m.storage.CreateIndexer(ctx, indexer)
	if err != nil {
		return indexer, err
	}

	indexer.ID = int32(id)

	return indexer, nil
}

// DeleteIndexerRequest request to delete an indexer
type DeleteIndexerRequest struct {
	ID *int `json:"id" yaml:"id"`
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

func ptr[A any](thing A) *A {
	return &thing
}

// GetMovieMetadataByID retrieves movie metadata by its primary key
func (m MediaManager) GetMovieMetadataByID(ctx context.Context, metadataID int32) (*model.MovieMetadata, error) {
	// fetch metadata record
	return m.storage.GetMovieMetadata(ctx, table.MovieMetadata.ID.EQ(sqlite.Int(int64(metadataID))))
}
