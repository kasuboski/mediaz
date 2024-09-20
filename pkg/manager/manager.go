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

	"github.com/kasuboski/mediaz/pkg/library"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/model"
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
}

func New(tmbdClient TMDBClientInterface, prowlarrClient prowlarr.IProwlarr, library library.Library, storage storage.Storage) MediaManager {
	return MediaManager{
		tmdb:    tmbdClient,
		indexer: NewIndexerStore(prowlarrClient, storage),
		library: library,
		storage: storage,
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

// SearchMovie querie tmdb for tv shows
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

// AddMovieRequest describes what is required to add a movie
// TODO: add quality profiles
type AddMovieRequest struct {
	TMDBID int `json:"tmdbid"`
}

// AddMovieToLibrary adds a movie to be managed by mediaz
// TODO: fetch trackers from indexer
// TODO: decide tracker based on quality profile (part of request.. ui will have to do a lookup here before request)
// TODO: query each indexer asynchronously?
// TODO: pass to torrent client
// TODO: always write status to database for given movie (queue, downloaded, missing (error?), Unreleased)
func (m MediaManager) AddMovieToLibrary(ctx context.Context, request AddMovieRequest) (*prowlarr.ReleaseResource, error) {
	log := logger.FromCtx(ctx)

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
		log.Debug("failed to search indexer", zap.Any("indexers", indexerIds), zap.Error(err))
		return nil, err
	}

	log.Debugw("releases for consideration", "releases", len(releases))
	releases = slices.DeleteFunc(releases, rejectReleaseFunc(ctx, det))
	log.Debugw("releases after rejection", "releases", len(releases))
	if len(releases) == 0 {
		// TODO: This probably isn't really an error... just need to search again later
		return nil, errors.New("no matching releases found")
	}
	slices.SortFunc(releases, sortReleaseFunc())
	chosenRelease := releases[len(releases)-1]

	b, _ := json.Marshal(chosenRelease)

	log.Debug("found release", zap.String("release", string(b)))

	return chosenRelease, nil
}

// rejectReleaseFunc returns a function that returns true if the given release should be rejected
func rejectReleaseFunc(ctx context.Context, det *MediaDetails) func(*prowlarr.ReleaseResource) bool {
	log := logger.FromCtx(ctx)
	// TODO: Get this dynamically
	qs := QualitySize{
		Quality:   "HDTV-720p",
		Min:       17.1,
		Preferred: 1999,
		Max:       2000,
	}
	return func(r *prowlarr.ReleaseResource) bool {
		// bytes to megabytes
		sizeMB := *r.Size >> 20
		metQuality := MeetsQualitySize(qs, uint64(sizeMB), uint64(*det.Runtime))
		decision := !metQuality
		if decision {
			log.Debugw("rejecting release", "release", r.Title, "metQuality", metQuality, "size", r.Size, "runtime", det.Runtime)
		}
		return decision
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
	model.Indexers
}

// AddIndexer stores a new indexer in the database
func (m MediaManager) AddIndexer(ctx context.Context, request AddIndexerRequest) (AddIndexerRequest, error) {
	indexer := request

	if indexer.Name == "" {
		return indexer, fmt.Errorf("indexer name is required")
	}

	id, err := m.storage.CreateIndexer(ctx, indexer.Indexers)
	if err != nil {
		return indexer, err
	}

	indexer.ID = int32(id)

	return indexer, nil
}

// DeleteIndexerRequest describes what is required to delete an indexer
type DeleteIndexerRequest struct {
	ID *int `json:"id" yaml:"id" mapstructure:"id"`
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
