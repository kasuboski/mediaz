package manager

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/go-jet/jet/v2/sqlite"
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

type TMDBClientInterface tmdb.ITmdb

type MediaManager struct {
	tmdb    TMDBClientInterface
	indexer IndexerStore
	library library.Library
	storage storage.Storage
	factory download.Factory
}

func New(tmbdClient TMDBClientInterface, prowlarrClient prowlarr.IProwlarr, library library.Library, storage storage.Storage, factory download.Factory) MediaManager {
	return MediaManager{
		tmdb:    tmbdClient,
		indexer: NewIndexerStore(prowlarrClient, storage),
		library: library,
		storage: storage,
		factory: factory,
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

var (
	// TODO: these are specific per indexer it seems.. need to store categories with the indexer
	MOVIE_CATEGORIES = []int32{2000}
	TV_CATEGORIES    = []int32{5000}
)

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

func (m MediaManager) ListMoviesInLibrary(ctx context.Context) ([]library.MovieFile, error) {
	return m.library.FindMovies(ctx)
}

func (m MediaManager) Run(ctx context.Context) error {
	log := logger.FromCtx(ctx)

	movieIndexTicker := time.NewTicker(time.Minute * 10)
	defer movieIndexTicker.Stop()
	movieReconcileTicker := time.NewTicker(time.Minute * 20)
	defer movieReconcileTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-movieIndexTicker.C:
			err := m.IndexMovieLibrary(ctx)
			if err != nil {
				log.Errorf("movie indexing failed: %w", err)
				continue
			}
		case <-movieReconcileTicker.C:
			err := m.ReconcileMovies(ctx)
			if err != nil {
				log.Errorf("movie reconciling failed: %w", err)
				continue
			}
		}
	}
}

func (m MediaManager) IndexMovieLibrary(ctx context.Context) error {
	// TODO: this probably shouldn't be synchronous... meaning kick if it off and check back
	log := logger.FromCtx(ctx)
	files, err := m.library.FindMovies(ctx)
	if err != nil {
		return fmt.Errorf("failed indexing movie library: %w", err)
	}

	for _, f := range files {
		mov := storage.Movie{
			Movie: model.Movie{
				Path:      &f.Path,
				Monitored: 0,
			},
		}

		movID, err := m.storage.CreateMovie(ctx, mov, storage.MovieStateDiscovered)
		if err != nil {
			log.Errorf("couldn't add movie to db: %w", err)
		}
		movieID := int32(movID)
		mf := model.MovieFile{
			RelativePath: &f.Path, // TODO: make sure it's actually relative
			Size:         f.Size,
			MovieID:      movieID,
		}
		mfID, err := m.storage.CreateMovieFile(ctx, mf)
		if err != nil {
			log.Error("couldn't add movie file", zap.Any("file", mf), zap.Error(err))
			continue
		}
		fileID := int32(mfID)
		mov.MovieFileID = &fileID
		mov.ID = movieID
		_, err = m.storage.CreateMovie(ctx, mov, storage.MovieStateDiscovered)
		if err != nil {
			log.Error("couldn't update movie to db", zap.Error(err))
		}
	}

	return nil
}

// AddMovieRequest describes what is required to add a movie to a library
type AddMovieRequest struct {
	TMDBID           int   `json:"tmdbID"`
	QualityProfileID int32 `json:"qualityProfileID"`
}

// AddMovieToLibrary adds a movie to be managed by mediaz
// TODO: check status of movie before doing anything else.. do we already have it tracked? is it downloaded or already discovered? error state?
// TODO: always write status to database for given movie (queue, downloaded, missing (error?), Unreleased)
func (m MediaManager) AddMovieToLibrary(ctx context.Context, request AddMovieRequest) (*storage.Movie, error) {
	log := logger.FromCtx(ctx)

	profile, err := m.storage.GetQualityProfile(ctx, int64(request.QualityProfileID))
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
		},
	}

	state := storage.MovieStateMissing
	if !isMovieReleased(now(), det) {
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

// ReconcileSnapshot is a thread safe snapshot of the current reconcile loop state
type ReconcileSnapshot struct {
	time              time.Time
	downloadProtocols map[string]struct{}
	downloadClients   []*model.DownloadClient
	indexers          []Indexer
	indexerIDs        []int32
	mu                sync.Mutex
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

	wg.Wait()
	return nil
}

func (m MediaManager) ReconcileMissingMovies(ctx context.Context, wg *sync.WaitGroup, snapshot *ReconcileSnapshot) error {
	defer wg.Done()

	if snapshot == nil {
		return fmt.Errorf("snapshot is nil")
	}

	movies, err := m.storage.ListMoviesByState(ctx, storage.MovieStateMissing)
	if err != nil {
		return fmt.Errorf("couldn't list movies during reconcile: %w", err)
	}

	log := logger.FromCtx(ctx)

	for _, movie := range movies {
		err = m.reconcileMissingMovie(ctx, movie, snapshot)
		if err != nil {
			log.Warn("failed to reconcile movie", zap.Error(err))
		}
	}

	return nil
}

func (m MediaManager) reconcileMissingMovie(ctx context.Context, movie *storage.Movie, snapshot *ReconcileSnapshot) error {
	log := logger.FromCtx(ctx)
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
	releases = slices.DeleteFunc(releases, rejectReleaseFunc(ctx, det, profile, availableProtocols))
	log.Debugw("releases after rejection", "releases", len(releases))
	if len(releases) == 0 {
		return nil
	}

	slices.SortFunc(releases, sortReleaseFunc())
	chosenRelease := releases[len(releases)-1]

	log.Infow("found release", "title", chosenRelease.Title, "proto", *chosenRelease.Protocol)

	downloadRequest := download.AddRequest{
		Release: chosenRelease,
	}

	dcs := snapshot.GetDownloadClients()
	c := clientForProtocol(dcs, *chosenRelease.Protocol)
	if c == nil {
		return nil
	}
	downloadClient, err := m.factory.NewDownloadClient(*c)
	if err != nil {
		return err
	}
	status, err := downloadClient.Add(ctx, downloadRequest)
	if err != nil {
		log.Debug("failed to add movie download request", zap.Error(err))
		return err
	}

	// update the state so we no longer reconcile this movie including download metadata
	err = m.storage.UpdateMovieState(ctx, int64(movie.ID), storage.MovieStateDownloading, &storage.MovieStateMetadata{
		DownloadID:       &status.ID,
		DownloadClientID: &c.ID,
	})
	if err != nil {
		log.Debugw("failed to update movie state", zap.Error(err))
		return err
	}

	return nil
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

	if !isMovieReleased(snapshot.time, det) {
		log.Debug("movie is still unreleased")
		return nil
	}

	err = m.storage.UpdateMovieState(ctx, int64(movie.ID), storage.MovieStateMissing, nil)
	if err != nil {
		log.Warn("failed to update released movie state", zap.Error(err))
		return err
	}

	return nil
}

// rejectReleaseFunc returns a function that returns true if the given release should be rejected
func rejectReleaseFunc(ctx context.Context, det *model.MovieMetadata, profile storage.QualityProfile, protocolsAvailable map[string]struct{}) func(*prowlarr.ReleaseResource) bool {
	log := logger.FromCtx(ctx)

	return func(r *prowlarr.ReleaseResource) bool {
		if r.Title != nil {
			releaseTitle := strings.TrimSpace(r.Title.MustGet())
			if !strings.HasPrefix(releaseTitle, det.Title) {
				return true
			}
		}

		if r.Protocol != nil {
			// reject if we don't have a download client for it
			if _, has := protocolsAvailable[string(*r.Protocol)]; !has {
				return true
			}
		}
		// bytes to megabytes
		sizeMB := *r.Size >> 20

		// items are assumed to be sorted quality so the highest media quality available is selected
		for _, quality := range profile.Qualities {
			metQuality := MeetsQualitySize(quality, uint64(sizeMB), uint64(det.Runtime))

			if metQuality {
				log.Debugw("accepting release", "release", r.Title, "metQuality", metQuality, "size", r.Size, "runtime", det.Runtime)
				return false
			}

			// try again with the next item
			log.Debugw("rejecting release", "release", r.Title, "metQuality", metQuality, "size", r.Size, "runtime", det.Runtime)
		}

		return true
	}
}

// sortReleaseFunc returns a function that sorts releases by their number of seeders currently
func sortReleaseFunc() func(*prowlarr.ReleaseResource, *prowlarr.ReleaseResource) int {
	return func(r1 *prowlarr.ReleaseResource, r2 *prowlarr.ReleaseResource) int {
		return cmp.Compare(nullableDefault(r1.Seeders), nullableDefault(r2.Seeders))
	}
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

func isMovieReleased(now time.Time, det *model.MovieMetadata) bool {
	return det.ReleaseDate != nil && now.After(*det.ReleaseDate)
}
