package manager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/kasuboski/mediaz/pkg/library"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/tmdb"
	"go.uber.org/zap"
)

type ProwlarrClientInterface prowlarr.ClientInterface
type TMDBClientInterface tmdb.ClientInterface

type MediaManager struct {
	tmdb    TMDBClientInterface
	indexer IndexerStore
	library library.Library
}

func New(tmbdClient TMDBClientInterface, prowlarrClient ProwlarrClientInterface, library library.Library) MediaManager {
	return MediaManager{
		tmdb:    tmbdClient,
		indexer: NewIndexerStore(prowlarrClient),
		library: library,
	}
}

type SearchMovieResponse struct {
	Page         *int            `json:"page,omitempty"`
	Results      []*SearchResult `json:"results,omitempty"`
	TotalPages   *int            `json:"total_pages,omitempty"`
	TotalResults *int            `json:"total_results,omitempty"`
}

type SearchResult struct {
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
)

// SearchMovie querie tmdb for a movie
func (m MediaManager) SearchMovie(ctx context.Context, query string) (*SearchMovieResponse, error) {
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

	if res.StatusCode != http.StatusOK {
		log.Debug("unexpected response", zap.String("status", res.Status))
		return nil, fmt.Errorf("unexpected status: %s", res.Status)
	}

	b, err := io.ReadAll(res.Body)
	if err != nil {
		log.Error("failed to read movie search response", zap.Error(err))
		return nil, err
	}

	results := new(SearchMovieResponse)
	err = json.Unmarshal(b, &results)
	if err != nil {
		log.Error("failed to unmarshal search movie response", zap.Error(err))
		return nil, err
	}

	return results, nil
}

// ListIndexers lists all managed indexers
func (m MediaManager) ListIndexers(ctx context.Context) ([]Indexer, error) {
	log := logger.FromCtx(ctx)

	if err := m.indexer.FetchIndexers(ctx); err != nil {
		log.Error(err)
	}
	return m.indexer.ListIndexers(ctx), nil
}

func (m MediaManager) ListEpisodesOnDisk(ctx context.Context) ([]string, error) {
	return m.library.FindEpisodes(ctx)
}

func (m MediaManager) ListMoviesOnDisk(ctx context.Context) ([]library.Movie, error) {
	return m.library.FindMovies(ctx)
}

// AddMovieRequest describes what is required to add a movie
// TODO: add quality profiles
type AddMovieRequest struct {
	Indexers []int32 `json:"indexers"`
	Query    string  `json:"query"`
}

// AddMovie adds a movie to be managed by mediaz
// TODO: fetch trackers from indexer
// TODO: decide tracker based on quality profile (part of request.. ui will have to do a lookup here before request)
// TODO: query each indexer asynchronously?
// TODO: pass to torrent client
// TODO: always write status to database for given movie (queue, downloaded, missing (error?), Unreleased)
func (m MediaManager) AddMovie(ctx context.Context, request AddMovieRequest) error {
	log := logger.FromCtx(ctx)

	categories := []int32{MOVIE_CATEGORY}
	releases, err := m.SearchIndexers(ctx, request.Indexers, categories, request.Query)
	if err != nil {
		log.Debug("failed to search indexer", zap.Error(err))
		return err
	}

	var chosenRelease *prowlarr.ReleaseResource
	var maxSeeders int32
	for _, r := range releases {
		seeders, err := r.Seeders.Get()
		if err != nil {
			log.Debug("failed to get seeders from release", zap.Any("release", r))
		}

		if seeders > maxSeeders {
			chosenRelease = r
		}
	}

	b, _ := json.Marshal(chosenRelease)

	log.Debug("found release", zap.String("release", string(b)))

	return nil
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
