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

type SearchMediaResponse struct {
	Page         *int                 `json:"page,omitempty"`
	Results      []*SearchMediaResult `json:"results,omitempty"`
	TotalPages   *int                 `json:"total_pages,omitempty"`
	TotalResults *int                 `json:"total_results,omitempty"`
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
	err = json.Unmarshal(b, &results)
	return results, err
}

// ListIndexers lists all managed indexers
func (m MediaManager) ListIndexers(ctx context.Context) ([]Indexer, error) {
	log := logger.FromCtx(ctx)

	if err := m.indexer.FetchIndexers(ctx); err != nil {
		log.Error(err)
	}
	return m.indexer.ListIndexers(ctx), nil
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
	Indexers []int32 `json:"indexers"`
	Query    string  `json:"query"`
}

// AddMovieToLibrary adds a movie to be managed by mediaz
// TODO: fetch trackers from indexer
// TODO: decide tracker based on quality profile (part of request.. ui will have to do a lookup here before request)
// TODO: query each indexer asynchronously?
// TODO: pass to torrent client
// TODO: always write status to database for given movie (queue, downloaded, missing (error?), Unreleased)
func (m MediaManager) AddMovieToLibrary(ctx context.Context, request AddMovieRequest) error {
	log := logger.FromCtx(ctx)

	res, err := m.SearchMovie(ctx, request.Query)
	if err != nil {
		return fmt.Errorf("couldn't search movie: %w", err)
	}

	r := res.Results
	if len(r) == 0 {
		return fmt.Errorf("no movie returned from search")
	}

	meta := FromSearchMediaResult(*r[0])
	det, err := m.GetMovieDetails(ctx, meta.TMDBID)
	if err != nil {
		return err
	}

	categories := []int32{MOVIE_CATEGORY}
	releases, err := m.SearchIndexers(ctx, request.Indexers, categories, meta.Title)
	if err != nil {
		log.Debug("failed to search indexer", zap.Error(err))
		return err
	}

	// TODO: Get this dynamically
	qs := QualitySize{
		Quality:   "HDTV-720p",
		Min:       17.1,
		Preferred: 1999,
		Max:       2000,
	}
	var chosenRelease *prowlarr.ReleaseResource
	var maxSeeders int32
	for _, r := range releases {
		if !MeetsQualitySize(qs, uint64(*r.Size), uint64(*det.Revenue)) {
			continue
		}

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
