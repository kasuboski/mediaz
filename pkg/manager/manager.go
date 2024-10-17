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
	"sync"

	"github.com/kasuboski/mediaz/pkg/download"
	"github.com/kasuboski/mediaz/pkg/library"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/tmdb"
	"github.com/oapi-codegen/nullable"
	"go.uber.org/zap"
)

type TMDBClientInterface tmdb.ClientInterface

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

const (
	MOVIE_CATEGORY int32 = 2000
	TV_CATEGORY    int32 = 5000
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

func (m MediaManager) IndexMovieLibrary(ctx context.Context) error {
	// TODO: this probably shouldn't be synchronous... meaning kick if it off and check back
	log := logger.FromCtx(ctx)
	files, err := m.library.FindMovies(ctx)
	if err != nil {
		return fmt.Errorf("failed indexing movie library: %w", err)
	}

	for _, f := range files {
		mf := model.MovieFile{
			RelativePath: &f.Path, // TODO: make sure it's actually relative
			Size:         f.Size,
		}
		mfID, err := m.storage.CreateMovieFile(ctx, mf)
		if err != nil {
			log.Errorf("couldn't add movie to db: %w", err)
			continue
		}

		mov := model.Movie{
			Path:        f.Path,
			Monitored:   0,
			MovieFileID: int32(mfID),
		}
		_, err = m.storage.CreateMovie(ctx, mov)
		if err != nil {
			log.Errorf("couldn't add movie to db: %w", err)
		}
	}

	return nil
}

// AddMovieRequest describes what is required to add a movie to a library
type AddMovieRequest struct {
	TMDBID           int   `json:"tmdbID"`
	QualityProfileID int32 `json:"qualityProfileID"`
	DownloadClientID int32 `json:"downloadClientID"`
}

// AddMovieToLibrary adds a movie to be managed by mediaz
// TODO: check status of movie before doing anything else.. do we already have it tracked? is it downloaded or already discovered? error state?
// TODO: query each indexer asynchronously?
// TODO: always write status to database for given movie (queue, downloaded, missing (error?), Unreleased)
func (m MediaManager) AddMovieToLibrary(ctx context.Context, request AddMovieRequest) (*download.Status, error) {
	log := logger.FromCtx(ctx)

	profile, err := m.storage.GetQualityProfile(ctx, int64(request.QualityProfileID))
	if err != nil {
		return nil, err
	}

	det, err := m.GetMovieDetails(ctx, request.TMDBID)
	if err != nil {
		return nil, err
	}

	categories := []int32{MOVIE_CATEGORY}
	indexers, err := m.ListIndexers(ctx)
	if err != nil {
		return nil, err
	}
	log.Debugw("listed indexers", "count", len(indexers))
	if len(indexers) == 0 {
		return nil, errors.New("no indexers available")
	}
	indexerIds := make([]int32, len(indexers))
	for i, indexer := range indexers {
		indexerIds[i] = indexer.ID
	}
	releases, err := m.SearchIndexers(ctx, indexerIds, categories, *det.Title)
	if err != nil {
		log.Debugw("failed to search indexer", "indexers", indexerIds, zap.Error(err))
		return nil, err
	}

	log.Debugw("releases for consideration", "releases", len(releases))
	releases = slices.DeleteFunc(releases, rejectReleaseFunc(ctx, det, profile))
	log.Debugw("releases after rejection", "releases", len(releases))
	if len(releases) == 0 {
		// TODO: This probably isn't really an error... just need to search again later
		return nil, errors.New("no matching releases found")
	}

	slices.SortFunc(releases, sortReleaseFunc())
	chosenRelease := releases[len(releases)-1]

	log.Info("found release", "title", chosenRelease.Title)

	downloadClient, err := m.GetDownloadClient(ctx, int64(request.DownloadClientID)) // yolo cast?
	if err != nil {
		log.Debugw("failed to get download client", "id", request.DownloadClientID, zap.Error(err))
		return nil, err
	}

	downloadRequest := download.AddRequest{
		Release: chosenRelease,
	}

	status, err := downloadClient.Add(ctx, downloadRequest)
	if err != nil {
		log.Debug("failed to add movie download request", zap.Error(err))
		return nil, err
	}

	return &status, nil
}

// rejectReleaseFunc returns a function that returns true if the given release should be rejected
func rejectReleaseFunc(ctx context.Context, det *MediaDetails, profile storage.QualityProfile) func(*prowlarr.ReleaseResource) bool {
	log := logger.FromCtx(ctx)

	return func(r *prowlarr.ReleaseResource) bool {
		// bytes to megabytes
		sizeMB := *r.Size >> 20

		// items are assumed to be sorted quality so the highest media quality available is selected
		for _, quality := range profile.Qualities {
			metQuality := MeetsQualitySize(quality, uint64(sizeMB), uint64(*det.Runtime))

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

type AddQualityDefinitionRequest struct {
	model.QualityDefinition
}

// AddQualityDefinition stores a new quality definition in the database
func (m MediaManager) AddQualityDefinition(ctx context.Context, request AddQualityDefinitionRequest) (model.QualityDefinition, error) {
	definition := request.QualityDefinition

	id, err := m.storage.CreateQualityDefinition(ctx, request.QualityDefinition)
	if err != nil {
		return definition, err
	}

	definition.ID = int32(id)
	return definition, nil
}

// DeleteQualityDefinitionRequest request to delete a quality definition
type DeleteQualityDefinitionRequest struct {
	ID *int `json:"id" yaml:"id"`
}

// AddQualityDefinition stores a new quality definition in the database
func (m MediaManager) DeleteQualityDefinition(ctx context.Context, request DeleteQualityDefinitionRequest) error {
	if request.ID == nil {
		return fmt.Errorf("indexer id is required")
	}

	return m.storage.DeleteQualityDefinition(ctx, int64(*request.ID))
}

// ListQualityDefinitions list stored quality definitions
func (m MediaManager) ListQualityDefinitions(ctx context.Context) ([]*model.QualityDefinition, error) {
	return m.storage.ListQualityDefinitions(ctx)
}

func (m MediaManager) GetQualityProfile(ctx context.Context, id int64) (storage.QualityProfile, error) {
	return m.storage.GetQualityProfile(ctx, id)
}

func (m MediaManager) ListQualityProfiles(ctx context.Context) ([]*storage.QualityProfile, error) {
	return m.storage.ListQualityProfiles(ctx)
}

// AddDownloadClientRequest describes what is required to add a download client
type AddDownloadClientRequest struct {
	model.DownloadClient
}

func (m MediaManager) CreateDownloadClient(ctx context.Context, request AddDownloadClientRequest) (model.DownloadClient, error) {
	downloadClient := request.DownloadClient

	id, err := m.storage.CreateDownloadClient(ctx, request.DownloadClient)
	if err != nil {
		return downloadClient, err
	}

	downloadClient.ID = int32(id)
	return downloadClient, nil
}

func (m MediaManager) GetDownloadClient(ctx context.Context, id int64) (download.DownloadClient, error) {
	client, err := m.storage.GetDownloadClient(ctx, id)
	if err != nil {
		return nil, err
	}

	return m.factory.NewDownloadClient(client)
}

func (m MediaManager) ListDownloadClients(ctx context.Context) ([]*model.DownloadClient, error) {
	return m.storage.ListDownloadClients(ctx)
}

func (m MediaManager) DeleteDownloadClient(ctx context.Context, id int64) error {
	return m.storage.DeleteDownloadClient(ctx, id)
}

func nullableDefault[T any](n nullable.Nullable[T]) T {
	var def T
	if n.IsSpecified() {
		v, _ := n.Get()
		return v
	}

	return def
}
