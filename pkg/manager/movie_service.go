package manager

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/pkg/library"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
	"github.com/kasuboski/mediaz/pkg/tmdb"
	"go.uber.org/zap"
)

// MovieMetadataProvider provides movie metadata operations needed by MovieService.
// This decouples MovieService from the full MediaManager, allowing the metadata
// subsystem to be extracted independently later.
type MovieMetadataProvider interface {
	GetMovieMetadata(ctx context.Context, tmdbID int) (*model.MovieMetadata, error)
	GetMovieMetadataByID(ctx context.Context, metadataID int32) (*model.MovieMetadata, error)
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
// TODO: check status of movie before doing anything else.. do we already have it tracked? is it downloaded or already discovered? error state?
func (s *MovieService) AddMovieToLibrary(ctx context.Context, request AddMovieRequest) (*storage.Movie, error) {
	log := logger.FromCtx(ctx)

	profile, err := s.qualityService.GetQualityProfile(ctx, int64(request.QualityProfileID))
	if err != nil {
		log.Debug("failed to get quality profile", zap.Int32("id", request.QualityProfileID), zap.Error(err))
		return nil, err
	}

	det, err := s.metadataProvider.GetMovieMetadata(ctx, request.TMDBID)
	if err != nil {
		log.Debug("failed to get movie metadata", zap.Error(err))
		return nil, err
	}

	movie, err := s.movieStorage.GetMovieByMetadataID(ctx, int(det.ID))
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

	id, err := s.movieStorage.CreateMovie(ctx, *movie, state)
	if err != nil {
		log.Warn("failed to create movie", zap.Error(err))
		return nil, err
	}

	log.Debug("created movie", zap.Any("movie", movie))

	movie, err = s.movieStorage.GetMovie(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("GetMovie after create: %w", err)
	}

	return movie, nil
}

// DeleteMovie removes a movie and optionally its files from disk.
func (s *MovieService) DeleteMovie(ctx context.Context, movieID int64, deleteFiles bool) error {
	log := logger.FromCtx(ctx)

	movie, err := s.movieStorage.GetMovie(ctx, movieID)
	if err != nil {
		return fmt.Errorf("failed to get movie: %w", err)
	}

	if deleteFiles {
		if movie.Path == nil {
			return fmt.Errorf("cannot delete files: movie path is nil")
		}

		if err := s.library.DeleteMovieDirectory(ctx, *movie.Path); err != nil {
			return fmt.Errorf("failed to delete movie directory %s: %w", *movie.Path, err)
		}
	}

	if err := s.movieStorage.DeleteMovie(ctx, movieID); err != nil {
		return fmt.Errorf("failed to delete movie: %w", err)
	}

	log.Info("deleted movie", zap.Int64("id", movieID), zap.Bool("files_deleted", deleteFiles))
	return nil
}

// UpdateMovieMonitored toggles the monitored state of a movie.
func (s *MovieService) UpdateMovieMonitored(ctx context.Context, movieID int64, monitored bool) (*storage.Movie, error) {
	monitoredInt := int32(0)
	if monitored {
		monitoredInt = 1
	}

	movieUpdate := model.Movie{Monitored: monitoredInt}
	err := s.movieStorage.UpdateMovie(ctx, movieUpdate, table.Movie.ID.EQ(sqlite.Int64(movieID)))
	if err != nil {
		return nil, err
	}

	movie, err := s.movieStorage.GetMovie(ctx, movieID)
	if err != nil {
		return nil, err
	}

	logger.FromCtx(ctx).Info("updated monitoring", zap.Int64("movie_id", movieID), zap.Bool("monitored", monitored))
	return movie, nil
}

// UpdateMovieQualityProfile updates the quality profile for a movie.
func (s *MovieService) UpdateMovieQualityProfile(ctx context.Context, movieID int64, qualityProfileID int32) (*storage.Movie, error) {
	err := s.movieStorage.UpdateMovieQualityProfile(ctx, movieID, qualityProfileID)
	if err != nil {
		return nil, err
	}

	movie, err := s.movieStorage.GetMovie(ctx, movieID)
	if err != nil {
		return nil, err
	}

	logger.FromCtx(ctx).Info("updated quality profile", zap.Int64("movie_id", movieID), zap.Int32("quality_profile_id", qualityProfileID))
	return movie, nil
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
func (s *MovieService) GetMovieDetailByTMDBID(ctx context.Context, tmdbID int) (*MovieDetailResult, error) {
	log := logger.FromCtx(ctx)

	// Get movie metadata from TMDB (creates if not exists)
	metadata, err := s.metadataProvider.GetMovieMetadata(ctx, tmdbID)
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
	movie, err := s.movieStorage.GetMovieByMetadataID(ctx, int(metadata.ID))
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

// ---------------------------------------------------------------------------
// Listing
// ---------------------------------------------------------------------------

// ListMoviesInLibrary returns all tracked movies enriched with metadata.
func (s *MovieService) ListMoviesInLibrary(ctx context.Context) ([]LibraryMovie, error) {
	all, err := s.movieStorage.ListMovies(ctx)
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
		meta, err := s.metadataProvider.GetMovieMetadataByID(ctx, *mp.MovieMetadataID)
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
