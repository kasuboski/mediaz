package manager

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/pkg/download"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
	"go.uber.org/zap"
)

var (
	// TODO: these are specific per indexer it seems.. need to store categories with the indexer
	MOVIE_CATEGORIES = []int32{2000}
	TV_CATEGORIES    = []int32{5000}
)

// ReconcileSnapshot is a thread safe snapshot of the current reconcile loop state
type ReconcileSnapshot struct {
	time              time.Time
	downloadProtocols map[string]struct{}
	downloadClients   []*model.DownloadClient
	indexers          []Indexer
	indexerIDs        []int32
	mu                sync.Mutex
}

func (r *ReconcileSnapshot) GetDownloadClient(id int32) *model.DownloadClient {
	dcs := r.GetDownloadClients()

	for _, dc := range dcs {
		if dc.ID == id {
			return dc
		}
	}

	return nil
}

func newReconcileSnapshot(indexers []Indexer, downloadClients []*model.DownloadClient) *ReconcileSnapshot {
	ids := make([]int32, 0)
	for i := 0; i < len(indexers); i++ {
		ids = append(ids, indexers[i].ID)
	}

	protocols := availableProtocols(downloadClients)

	return &ReconcileSnapshot{
		downloadClients:   downloadClients,
		downloadProtocols: protocols,
		indexerIDs:        ids,
		indexers:          indexers,
		mu:                sync.Mutex{},
		time:              now(),
	}
}

func (r *ReconcileSnapshot) GetDownloadClients() []*model.DownloadClient {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.downloadClients
}

func (r *ReconcileSnapshot) GetProtocols() map[string]struct{} {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.downloadProtocols
}

func (r *ReconcileSnapshot) GetIndexerIDs() []int32 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.indexerIDs
}

func (r *ReconcileSnapshot) GetIndexers() []Indexer {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.indexers
}

func (m MediaManager) ReconcileMovies(ctx context.Context) error {
	var wg sync.WaitGroup

	log := logger.FromCtx(ctx)

	dcs, err := m.ListDownloadClients(ctx)
	if err != nil {
		return err
	}

	indexers, err := m.ListIndexers(ctx)
	if err != nil {
		return err
	}

	log.Debugw("listed indexers", "count", len(indexers))
	if len(indexers) == 0 {
		return errors.New("no indexers available")
	}

	snapshot := newReconcileSnapshot(indexers, dcs)

	wg.Add(1)
	go m.ReconcileMissingMovies(ctx, &wg, snapshot)

	wg.Add(1)
	go m.ReconcileUnreleasedMovies(ctx, &wg, snapshot)

	wg.Add(1)
	go m.ReconcileDownloadingMovies(ctx, &wg, snapshot)

	wg.Add(1)
	go m.ReconcileDiscoveredMovies(ctx, &wg, snapshot)

	wg.Wait()
	return nil
}

func (m MediaManager) ReconcileMissingMovies(ctx context.Context, wg *sync.WaitGroup, snapshot *ReconcileSnapshot) error {
	defer wg.Done()
	log := logger.FromCtx(ctx)

	if snapshot == nil {
		return fmt.Errorf("snapshot is nil")
	}

	movies, err := m.storage.ListMoviesByState(ctx, storage.MovieStateMissing)
	if err != nil {
		return fmt.Errorf("couldn't list missing movies: %w", err)
	}

	for _, movie := range movies {
		err = m.reconcileMissingMovie(ctx, movie, snapshot)
		if err != nil {
			log.Warn("failed to reconcile movie", zap.Error(err))
		}
	}

	return nil
}

func (m MediaManager) ReconcileDownloadingMovies(ctx context.Context, wg *sync.WaitGroup, snapshot *ReconcileSnapshot) error {
	defer wg.Done()
	log := logger.FromCtx(ctx)
	movies, err := m.storage.ListMoviesByState(ctx, storage.MovieStateDownloading)
	if err != nil {
		return fmt.Errorf("couldn't list downloading movies: %w", err)
	}

	for _, movie := range movies {
		err = m.reconcileDownloadingMovie(ctx, movie, snapshot)
		if err != nil {
			log.Warn("failed to reconcile downloading movie", zap.Error(err))
			continue
		}
	}

	return nil
}

func (m MediaManager) reconcileDownloadingMovie(ctx context.Context, movie *storage.Movie, snapshot *ReconcileSnapshot) error {
	log := logger.FromCtx(ctx)
	log = log.With("reconcile loop", "downloading", "movie id", movie.ID)

	if snapshot == nil {
		log.Warn("snapshot is nil, skipping reconcile")
		return nil
	}

	if movie.Monitored == 0 {
		log.Debug("movie is not monitored, skipping reconcile")
		return nil
	}

	if movie.Path == nil {
		log.Debug("movie path is nil, skipping reconcile")
		return nil
	}

	_, err := m.storage.GetMovieFilesByMovieName(ctx, *movie.Path)
	if err == nil {
		log.Info("movie files already tracked")
		return m.updateMovieState(ctx, movie, storage.MovieStateDownloaded, nil)
	}

	dc := snapshot.GetDownloadClient(movie.DownloadClientID)
	if dc == nil {
		log.Warn("movie download client not found in snapshot, skipping reconcile", zap.Int32("download client id", movie.DownloadClientID))
		return nil
	}

	downloadClient, err := m.factory.NewDownloadClient(*dc)
	if err != nil {
		log.Warn("failed to create download client", zap.Error(err))
		return err
	}

	status, err := downloadClient.Get(ctx, download.GetRequest{
		ID: movie.DownloadID,
	})
	if err != nil {
		log.Warn("failed to get download status", zap.Error(err))
		return err
	}

	log.Debug("status", zap.Any("status", status))
	if !status.Done {
		log.Debug("download not finished")
		return nil
	}

	movieMetadata, err := m.storage.GetMovieMetadata(ctx, table.MovieMetadata.ID.EQ(sqlite.Int32(*movie.MovieMetadataID)))
	if err != nil {
		log.Error("failed to get movie metadata", zap.Error(err))
		return err
	}

	log.Debug("attempting to move downloaded file")
	for _, f := range status.FilePaths {
		err = m.addMovieFileToLibrary(ctx, movieMetadata.Title, f, movie)
		if err != nil {
			log.Error("failed to add movie file to library", zap.Error(err))
			return err
		}

		log.Debug("successfully added movie file to library", zap.String("file", f))
	}

	return m.updateMovieState(ctx, movie, storage.MovieStateDownloaded, nil)
}

func (m MediaManager) addMovieFileToLibrary(ctx context.Context, title, filePath string, movie *storage.Movie) error {
	log := logger.FromCtx(ctx)
	log = log.With("movie id", movie.ID)

	mf, err := m.library.AddMovie(ctx, title, filePath)
	if err != nil {
		return fmt.Errorf("failed to add movie to library: %w", err)
	}

	_, err = m.storage.CreateMovieFile(ctx, model.MovieFile{
		RelativePath:     &mf.RelativePath,
		Size:             mf.Size,
		OriginalFilePath: &filePath,
	})
	if err != nil {
		return fmt.Errorf("failed to create movie file: %v", err)
	}

	log.Debug("created movie file", zap.String("path", mf.RelativePath))

	return nil
}

func (m MediaManager) reconcileMissingMovie(ctx context.Context, movie *storage.Movie, snapshot *ReconcileSnapshot) error {
	log := logger.FromCtx(ctx)
	log = log.With("reconcile loop", "missing movie")
	log = log.With("movie id", movie.ID)

	if movie.MovieMetadataID == nil {
		log.Warn("movie metadata id is nil, skipping reconcile")
		return nil
	}

	if movie.QualityProfileID == 0 {
		log.Warn("movie quality profile id is nil, skipping reconcile")
		return nil
	}

	if movie.Monitored == 0 {
		log.Debug("movie is not monitored, skipping reconcile")
	}

	if movie.MovieFileID != nil {
		// TODO: should this update state? If we have a movie ID we don't need to download the actual file most likely
		log.Debug("movie file id already exists for movie, skipping reconcile")
		return nil
	}

	det, err := m.storage.GetMovieMetadata(ctx, table.MovieMetadata.ID.EQ(sqlite.Int32(*movie.MovieMetadataID)))
	if err != nil {
		log.Debugw("failed to find movie metadata", "meta_id", *movie.MovieMetadataID)
		return err
	}

	profile, err := m.storage.GetQualityProfile(ctx, int64(movie.QualityProfileID))
	if err != nil {
		log.Warnw("failed to find movie qualityprofile", "quality_id", movie.QualityProfileID)
		return err
	}

	indexerIDs := snapshot.GetIndexerIDs()
	releases, err := m.SearchIndexers(ctx, indexerIDs, MOVIE_CATEGORIES, det.Title)
	if err != nil {
		log.Debugw("failed to search indexer", "indexers", indexerIDs, zap.Error(err))
		return err
	}

	availableProtocols := snapshot.GetProtocols()
	log.Debugw("releases for consideration", "releases", len(releases))
	releases = slices.DeleteFunc(releases, RejectMovieReleaseFunc(ctx, det.Title, det.Runtime, profile, availableProtocols))
	log.Debugw("releases after rejection", "releases", len(releases))
	if len(releases) == 0 {
		return nil
	}

	slices.SortFunc(releases, sortReleaseFunc())
	chosenRelease := releases[len(releases)-1]

	log.Infow("found release", "title", chosenRelease.Title, "proto", *chosenRelease.Protocol)

	clientID, status, err := m.requestReleaseDownload(ctx, snapshot, chosenRelease)
	if err != nil {
		log.Debug("failed to add movie download request", zap.Error(err))
		return fmt.Errorf("failed to add movie download request: %w", err)
	}

	return m.updateMovieState(ctx, movie, storage.MovieStateDownloading, &storage.TransitionStateMetadata{
		DownloadID:       &status.ID,
		DownloadClientID: &clientID,
	})
}

func (m MediaManager) ReconcileUnreleasedMovies(ctx context.Context, wg *sync.WaitGroup, snapshot *ReconcileSnapshot) error {
	defer wg.Done()

	if snapshot == nil {
		return fmt.Errorf("snapshot is nil")
	}

	log := logger.FromCtx(ctx)

	movies, err := m.storage.ListMoviesByState(ctx, storage.MovieStateUnreleased)
	if err != nil {
		log.Warn("failed to list unreleased movies", zap.Error(err))
		return fmt.Errorf("couldn't list movies during unrelease reconcile: %w", err)
	}

	for _, movie := range movies {
		err = m.reconcileUnreleasedMovie(ctx, movie, snapshot)
		if err != nil {
			log.Warn("error reconciling unreleased movie", zap.Error(err))
		}
	}

	return nil
}

func (m *MediaManager) reconcileUnreleasedMovie(ctx context.Context, movie *storage.Movie, snapshot *ReconcileSnapshot) error {
	log := logger.FromCtx(ctx)
	log = log.With("movie id", movie.ID)

	if movie.Monitored == 0 {
		log.Info("movie is not monitored, skipping reconcile")
		return nil
	}

	if movie.MovieMetadataID == nil {
		log.Info("movie metadata id is nil, skipping reconcile")
		return nil
	}

	det, err := m.storage.GetMovieMetadata(ctx, table.MovieMetadata.ID.EQ(sqlite.Int32(*movie.MovieMetadataID)))
	if err != nil {
		log.Debug("failed to find movie metadata", zap.Error(err))
		return err
	}

	if !isReleased(snapshot.time, det.ReleaseDate) {
		log.Debug("movie is still unreleased")
		return nil
	}

	return m.updateMovieState(ctx, movie, storage.MovieStateMissing, nil)
}

func (m MediaManager) updateMovieState(ctx context.Context, movie *storage.Movie, state storage.MovieState, metadata *storage.TransitionStateMetadata) error {
	log := logger.FromCtx(ctx).With("movie id", movie.ID, "from state", movie.State, "to state", state)
	err := m.storage.UpdateMovieState(ctx, int64(movie.ID), state, metadata)
	if err != nil {
		log.Warn("failed to update movie state", zap.Error(err))
		return err
	}

	log.Info("successfully updated movie state")
	return nil
}

func (m MediaManager) ReconcileDiscoveredMovies(ctx context.Context, wg *sync.WaitGroup, snapshot *ReconcileSnapshot) error {
	defer wg.Done()

	if snapshot == nil {
		return fmt.Errorf("snapshot is nil")
	}

	movies, err := m.storage.ListMoviesByState(ctx, storage.MovieStateDiscovered)
	if err != nil {
		return fmt.Errorf("couldn't list discovered movies: %w", err)
	}

	log := logger.FromCtx(ctx)

	for _, movie := range movies {
		err = m.reconcileDiscoveredMovie(ctx, movie)
		if err != nil {
			log.Warn("failed to reconcile movie", zap.Error(err))
		}
	}

	return nil
}

func (m MediaManager) reconcileDiscoveredMovie(ctx context.Context, movie *storage.Movie) error {
	log := logger.FromCtx(ctx)
	log = log.With("reconcile loop", "discovered", "movie id", movie.ID)

	if movie.MovieMetadataID != nil {
		log.Debug("movie already has metadata, skipping reconcile")
		return nil
	}

	searchTerm := pathToSearchTerm(*movie.Path)
	searchResp, err := m.SearchMovie(ctx, searchTerm)
	if err != nil {
		return fmt.Errorf("failed to search for movie: %w", err)
	}

	if len(searchResp.Results) == 0 {
		log.Warn("no results found for movie", zap.String("path", *movie.Path), zap.String("search_term", searchTerm))
		return nil
	}

	if len(searchResp.Results) > 1 {
		log.Debug("multiple results found for movie", zap.String("path", *movie.Path), zap.String("search_term", searchTerm), zap.Int("count", len(searchResp.Results)))
	}

	// Use first result
	result := searchResp.Results[0]
	if result.ID == nil {
		return fmt.Errorf("movie result has no ID")
	}

	metadata, err := m.GetMovieMetadata(ctx, *result.ID)
	if err != nil {
		return fmt.Errorf("failed to get movie metadata: %w", err)
	}

	err = m.storage.LinkMovieMetadata(ctx, int64(movie.ID), metadata.ID)
	if err != nil {
		return fmt.Errorf("failed to update movie: %w", err)
	}

	// Update the movie struct with the metadata ID
	movie.MovieMetadataID = &metadata.ID

	log.Info("updated movie with metadata", zap.Int32("metadata_id", metadata.ID))
	return nil
}

func (m MediaManager) requestReleaseDownload(ctx context.Context, snapshot *ReconcileSnapshot, release *prowlarr.ReleaseResource) (int32, download.Status, error) {
	dcs := snapshot.GetDownloadClients()
	c := clientForProtocol(dcs, *release.Protocol)
	if c == nil {
		return 0, download.Status{}, fmt.Errorf("no download client found for protocol: %s", *release.Protocol)
	}

	id := c.ID

	downloadClient, err := m.factory.NewDownloadClient(*c)
	if err != nil {
		return id, download.Status{}, fmt.Errorf("failed to create download client: %w", err)
	}

	status, err := downloadClient.Add(ctx, download.AddRequest{Release: release})
	return id, status, err
}

func getSeasonRuntime(episodes []*storage.Episode, totalSeasonEpisodes int) int32 {
	var runtime int32
	var consideredRuntimeCount int
	for _, e := range episodes {
		if e.Runtime != nil {
			runtime = runtime + *e.Runtime
			consideredRuntimeCount++
		}
	}

	// if we're missing some of the runtimes, we can try to estimate the remaining runtime based on the average of the other episodes
	// this could be pretty inaccurate in cases where we are missing more than a few runtimes, but it's better than nothing
	if consideredRuntimeCount > 0 && consideredRuntimeCount < totalSeasonEpisodes {
		averageRuntime := runtime / int32(consideredRuntimeCount)
		runtime += averageRuntime * int32(totalSeasonEpisodes-consideredRuntimeCount)
	}

	return runtime
}
