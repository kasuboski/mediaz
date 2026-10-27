package manager

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/kasuboski/mediaz/pkg/library"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/storage"
	sqlcdb "github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/kasuboski/mediaz/pkg/tmdb"
	"go.uber.org/zap"
)

// ErrMovieAlreadyExists is returned when adding a movie that already exists
// in a terminal or in-progress state (downloaded or downloading).
var ErrMovieAlreadyExists = errors.New("movie already exists in library")

// MovieMetadataProvider provides movie metadata operations needed by MovieService.
// This decouples MovieService from the full MediaManager, allowing the metadata
// subsystem to be extracted independently later.
type MovieMetadataProvider interface {
	GetMovieMetadata(ctx context.Context, tmdbID int64) (*sqlcdb.MovieMetadatum, error)
	GetMovieMetadataByID(ctx context.Context, metadataID int64) (*sqlcdb.MovieMetadatum, error)
}

// MovieService handles movie CRUD, search, and detail lookups.
// Indexing and reconciliation remain on MediaManager for now.
type MovieService struct {
	tmdb             tmdb.ITmdb
	library          library.Library
	movieStorage     storage.MovieStorage
	qualityService   *QualityService
	metadataProvider MovieMetadataProvider
}

// NewMovieService creates a MovieService with the given dependencies.
func NewMovieService(tmdbClient tmdb.ITmdb, lib library.Library, movieStorage storage.MovieStorage, qualityService *QualityService, metadataProvider MovieMetadataProvider) *MovieService {
	return &MovieService{
		tmdb:             tmdbClient,
		library:          lib,
		movieStorage:     movieStorage,
		qualityService:   qualityService,
		metadataProvider: metadataProvider,
	}
}

// ---------------------------------------------------------------------------
// CRUD Operations
// ---------------------------------------------------------------------------

// AddMovieToLibrary adds a movie to be managed by mediaz.
// If the movie already exists and is in a downloaded or downloading state it returns
// the movie alongside ErrMovieAlreadyExists. Movies in discovered, missing, unreleased,
// or new states are returned without error — they still need reconciliation.
func (s *MovieService) AddMovieToLibrary(ctx context.Context, request AddMovieRequest) (*sqlcdb.GetMovieByIDRow, error) {
	log := logger.FromCtx(ctx)

	profile, err := s.qualityService.GetQualityProfile(ctx, request.QualityProfileID)
	if err != nil {
		log.Debug("failed to get quality profile", zap.Int64("id", request.QualityProfileID), zap.Error(err))
		return nil, err
	}

	det, err := s.metadataProvider.GetMovieMetadata(ctx, request.TMDBID)
	if err != nil {
		log.Debug("failed to get movie metadata", zap.Error(err))
		return nil, err
	}

	movie, err := s.movieStorage.GetMovieByMetadataID(ctx, det.ID)
	if err == nil {
		result := sqlcdb.GetMovieByIDRow(movie)
		// Movie already exists — check its state before deciding what to return.
		switch storage.MovieState(movie.State) {
		case storage.MovieStateDownloaded, storage.MovieStateDownloading:
			log.Debug("movie already exists in library",
				zap.Int64("movie_id", movie.Movie.ID),
				zap.String("state", movie.State),
			)
			return &result, ErrMovieAlreadyExists
		default:
			// discovered, missing, unreleased, new — still needs processing
			log.Debug("movie exists but still needs reconciliation",
				zap.Int64("movie_id", movie.Movie.ID),
				zap.String("state", movie.State),
			)
			return &result, nil
		}
	}

	// anything other than a not found error is an internal error
	if !errors.Is(err, storage.ErrNotFound) {
		log.Warn("couldn't find movie by metadata", zap.Int64("meta_id", det.ID), zap.Error(err))
		return nil, err
	}

	// need to add the movie if it does not exist
	newMovie := sqlcdb.Movie{
		MovieMetadataID:  sql.NullInt64{Int64: det.ID, Valid: true},
		QualityProfileID: profile.ID,
		Monitored:        1,
		Path:             sql.NullString{String: det.Title, Valid: true},
	}

	state := initialMovieState(det.ReleaseDate)

	id, err := s.movieStorage.CreateMovie(ctx, newMovie, state)
	if err != nil {
		log.Warn("failed to create movie", zap.Error(err))
		return nil, err
	}

	log.Debug("created movie", zap.Any("movie", newMovie))

	created, err := s.movieStorage.GetMovie(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("GetMovie after create: %w", err)
	}

	return &created, nil
}

// DeleteMovie removes a movie and optionally its files from disk.
func (s *MovieService) DeleteMovie(ctx context.Context, movieID int64, deleteFiles bool) error {
	log := logger.FromCtx(ctx)

	movie, err := s.movieStorage.GetMovie(ctx, movieID)
	if err != nil {
		return fmt.Errorf("failed to get movie: %w", err)
	}

	if deleteFiles {
		if !movie.Movie.Path.Valid {
			return fmt.Errorf("cannot delete files: movie path is nil")
		}

		if err := s.library.DeleteMovieDirectory(ctx, movie.Movie.Path.String); err != nil {
			return fmt.Errorf("failed to delete movie directory %s: %w", movie.Movie.Path.String, err)
		}
	}

	if err := s.movieStorage.DeleteMovie(ctx, movieID); err != nil {
		return fmt.Errorf("failed to delete movie: %w", err)
	}

	log.Info("deleted movie", zap.Int64("id", movieID), zap.Bool("files_deleted", deleteFiles))
	return nil
}

// UpdateMovieMonitored toggles the monitored state of a movie.
func (s *MovieService) UpdateMovieMonitored(ctx context.Context, movieID int64, monitored bool) (*sqlcdb.GetMovieByIDRow, error) {
	monitoredInt := int64(0)
	if monitored {
		monitoredInt = 1
	}

	movieUpdate := sqlcdb.Movie{Monitored: monitoredInt}
	err := s.movieStorage.UpdateMovieByID(ctx, movieID, movieUpdate)
	if err != nil {
		return nil, err
	}

	movie, err := s.movieStorage.GetMovie(ctx, movieID)
	if err != nil {
		return nil, err
	}

	logger.FromCtx(ctx).Info("updated monitoring", zap.Int64("movie_id", movieID), zap.Bool("monitored", monitored))
	return &movie, nil
}

// UpdateMovieQualityProfile updates the quality profile for a movie.
func (s *MovieService) UpdateMovieQualityProfile(ctx context.Context, movieID int64, qualityProfileID int64) (*sqlcdb.GetMovieByIDRow, error) {
	err := s.movieStorage.UpdateMovieQualityProfile(ctx, movieID, qualityProfileID)
	if err != nil {
		return nil, err
	}

	movie, err := s.movieStorage.GetMovie(ctx, movieID)
	if err != nil {
		return nil, err
	}

	logger.FromCtx(ctx).Info("updated quality profile", zap.Int64("movie_id", movieID), zap.Int64("quality_profile_id", qualityProfileID))
	return &movie, nil
}

// ---------------------------------------------------------------------------
// Search & Detail
// ---------------------------------------------------------------------------

// SearchMovie queries TMDB for a movie matching the given query.
func (s *MovieService) SearchMovie(ctx context.Context, query string) (*SearchMediaResponse, error) {
	log := logger.FromCtx(ctx)
	if query == "" {
		log.Debug("search movie query is empty", zap.String("query", query))
		return nil, errors.New("query is empty")
	}

	res, err := s.tmdb.SearchMovie(ctx, &tmdb.SearchMovieParams{Query: query})
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

// GetMovieDetailByTMDBID retrieves detailed information for a single movie by TMDB ID.
func (s *MovieService) GetMovieDetailByTMDBID(ctx context.Context, tmdbID int64) (*MovieDetailResult, error) {
	log := logger.FromCtx(ctx)

	// Get movie metadata from TMDB (creates if not exists)
	metadata, err := s.metadataProvider.GetMovieMetadata(ctx, tmdbID)
	if err != nil {
		log.Error("failed to get movie metadata", zap.Error(err), zap.Int64("tmdbID", tmdbID))
		return nil, err
	}

	// Create the detailed result from metadata
	runtime := int32(metadata.Runtime)
	year := nullI32Out(metadata.Year)
	var popularity *float64
	if metadata.Popularity.Valid {
		popularity = &metadata.Popularity.Float64
	}
	var collectionTmdbID *int64
	if metadata.CollectionTmdbID.Valid {
		collectionTmdbID = &metadata.CollectionTmdbID.Int64
	}
	result := &MovieDetailResult{
		TMDBID:           metadata.TmdbID,
		ImdbID:           nullStrOut(metadata.ImdbID),
		Title:            metadata.Title,
		OriginalTitle:    nullStrOut(metadata.OriginalTitle),
		Overview:         nullStrOut(metadata.Overview),
		PosterPath:       metadata.Images,
		Runtime:          &runtime,
		Genres:           nullStrOut(metadata.Genres),
		Studio:           nullStrOut(metadata.Studio),
		Website:          nullStrOut(metadata.Website),
		CollectionTmdbID: collectionTmdbID,
		CollectionTitle:  nullStrOut(metadata.CollectionTitle),
		Popularity:       popularity,
		Year:             year,
		LibraryStatus:    "Not In Library", // Default status
	}

	// Format release date as string if available
	if metadata.ReleaseDate.Valid {
		releaseDateStr := metadata.ReleaseDate.Time.Format("2006-01-02")
		result.ReleaseDate = &releaseDateStr
	}

	// Try to get library information (movie record)
	movie, err := s.movieStorage.GetMovieByMetadataID(ctx, metadata.ID)
	if err == nil {
		movieID := movie.Movie.ID
		qualityProfileID := movie.Movie.QualityProfileID
		path := nullStrOut(movie.Movie.Path)
		monitored := movie.Movie.Monitored == 1
		result.ID = &movieID
		result.LibraryStatus = movie.State
		result.Path = path
		result.QualityProfileID = &qualityProfileID
		result.Monitored = &monitored
	} else if !errors.Is(err, storage.ErrNotFound) {
		log.Debug("error checking movie library status", zap.Error(err), zap.Int64("metadataID", metadata.ID))
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// Listing
// ---------------------------------------------------------------------------

// ListMoviesInLibrary returns all tracked movies enriched with metadata.
func (s *MovieService) ListMoviesInLibrary(ctx context.Context) ([]LibraryMovie, error) {
	all, err := s.movieStorage.ListMovies(ctx)
	if err != nil {
		return nil, err
	}
	movies := filterAndMap(all, func(mp sqlcdb.ListMoviesRow) (LibraryMovie, bool) {
		// Skip movies without metadata - they haven't been reconciled yet
		if !mp.Movie.MovieMetadataID.Valid {
			return LibraryMovie{}, false
		}
		lm := LibraryMovie{State: mp.State}
		if mp.Movie.Path.Valid {
			lm.Path = mp.Movie.Path.String
		}
		meta, err := s.metadataProvider.GetMovieMetadataByID(ctx, mp.Movie.MovieMetadataID.Int64)
		if err != nil || meta == nil {
			return LibraryMovie{}, false
		}
		lm.TMDBID = meta.TmdbID
		lm.Title = meta.Title
		lm.PosterPath = meta.Images
		if meta.Year.Valid {
			lm.Year = int32(meta.Year.Int64)
		}
		return lm, true
	})
	return movies, nil
}

// nullI32Out converts a nullable int64 to an optional int32.
func nullI32Out(v sql.NullInt64) *int32 {
	if !v.Valid {
		return nil
	}
	n := int32(v.Int64)
	return &n
}
