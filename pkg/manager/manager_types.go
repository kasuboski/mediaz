package manager

import (
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
)

// LibraryShow summarizes a tracked TV series in the local library for list views.
// Path may be empty when not yet imported; TMDBID maps to series metadata; PosterPath can be unset; Year is optional; State reflects storage state, not TMDB status.
type LibraryShow struct {
	Path       string `json:"path"`
	TMDBID     int32  `json:"tmdbID"`
	Title      string `json:"title"`
	PosterPath string `json:"poster_path"`
	Year       int32  `json:"year,omitempty"`
	State      string `json:"state"`
}

// SearchMediaResponse is the paginated envelope returned by movie and TV search.
// All fields are optional; callers must nil-check Page/TotalPages/TotalResults and Results.
type SearchMediaResponse struct {
	Page         *int                 `json:"page,omitempty"`
	TotalPages   *int                 `json:"total_pages,omitempty"`
	TotalResults *int                 `json:"total_results,omitempty"`
	Results      []*SearchMediaResult `json:"results,omitempty"`
}

// SearchMediaResult represents a single movie/TV search hit.
// Many fields are optional and may be empty; ReleaseDate is a string date; ID is the TMDB ID when present.
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

// MovieDetailResult is a consolidated movie view combining metadata and library status.
// Fields like ImdbID, OriginalTitle, Overview, BackdropPath, ReleaseDate, Year, Runtime, ratings, and collection info are optional.
// LibraryStatus is derived from storage; Path may be empty; QualityProfileID/Monitored are set only when tracked.
type MovieDetailResult struct {
	TMDBID           int32    `json:"tmdbID"`
	ImdbID           *string  `json:"imdbID,omitempty"`
	Title            string   `json:"title"`
	OriginalTitle    *string  `json:"originalTitle,omitempty"`
	Overview         *string  `json:"overview,omitempty"`
	PosterPath       string   `json:"posterPath,omitempty"`
	BackdropPath     *string  `json:"backdropPath,omitempty"`
	ReleaseDate      *string  `json:"releaseDate,omitempty"`
	Year             *int32   `json:"year,omitempty"`
	Runtime          *int32   `json:"runtime,omitempty"`
	Adult            *bool    `json:"adult,omitempty"`
	VoteAverage      *float32 `json:"voteAverage,omitempty"`
	VoteCount        *int     `json:"voteCount,omitempty"`
	Popularity       *float64 `json:"popularity,omitempty"`
	Genres           *string  `json:"genres,omitempty"`
	Studio           *string  `json:"studio,omitempty"`
	Website          *string  `json:"website,omitempty"`
	CollectionTmdbID *int32   `json:"collectionTmdbID,omitempty"`
	CollectionTitle  *string  `json:"collectionTitle,omitempty"`
	LibraryStatus    string   `json:"libraryStatus"`
	Path             *string  `json:"path,omitempty"`
	QualityProfileID *int32   `json:"qualityProfileID,omitempty"`
	Monitored        *bool    `json:"monitored,omitempty"`
}

// TVDetailResult is a consolidated TV series view combining metadata and library status.
// BackdropPath, FirstAirDate, LastAirDate, Networks, Genres, and ratings may be absent; counts are from metadata and can be zero.
// LibraryStatus is storage-derived; Path may be empty; QualityProfileID/Monitored are only present when tracked.
type TVDetailResult struct {
	TMDBID           int32    `json:"tmdbID"`
	Title            string   `json:"title"`
	OriginalTitle    *string  `json:"originalTitle,omitempty"`
	Overview         string   `json:"overview,omitempty"`
	PosterPath       string   `json:"posterPath,omitempty"`
	BackdropPath     *string  `json:"backdropPath,omitempty"`
	FirstAirDate     *string  `json:"firstAirDate,omitempty"`
	LastAirDate      *string  `json:"lastAirDate,omitempty"`
	Networks         []string `json:"networks,omitempty"`
	SeasonCount      int32    `json:"seasonCount"`
	EpisodeCount     int32    `json:"episodeCount"`
	Adult            *bool    `json:"adult,omitempty"`
	VoteAverage      *float32 `json:"voteAverage,omitempty"`
	VoteCount        *int     `json:"voteCount,omitempty"`
	Popularity       *float64 `json:"popularity,omitempty"`
	Genres           []string `json:"genres,omitempty"`
	LibraryStatus    string   `json:"libraryStatus"`
	Path             *string  `json:"path,omitempty"`
	QualityProfileID *int32   `json:"qualityProfileID,omitempty"`
	Monitored        *bool    `json:"monitored,omitempty"`
}

// LibraryMovie summarizes a tracked movie in the local library.
// Path may be empty prior to import; Year is optional; PosterPath can be empty; State reflects storage state.
type LibraryMovie struct {
	Path       string `json:"path"`
	TMDBID     int32  `json:"tmdbID"`
	Title      string `json:"title"`
	PosterPath string `json:"poster_path"`
	Year       int32  `json:"year,omitempty"`
	State      string `json:"state"`
}

// AddMovieRequest describes inputs to start managing a movie.
// TMDBID must refer to a valid TMDB movie; QualityProfileID must match an existing quality profile.
type AddMovieRequest struct {
	TMDBID           int   `json:"tmdbID"`
	QualityProfileID int32 `json:"qualityProfileID"`
}

// AddSeriesRequest describes inputs to start managing a TV series.
// TMDBID must refer to a valid TMDB series; QualityProfileID must match an existing quality profile.
type AddSeriesRequest struct {
	TMDBID           int   `json:"tmdbID"`
	QualityProfileID int32 `json:"qualityProfileID"`
}

// AddIndexerRequest wraps a storage Indexer model to create a new indexer.
// Ensure Name and Priority are set appropriately; credentials/URI should be validated upstream.
type AddIndexerRequest struct {
	model.Indexer
}

// DeleteIndexerRequest identifies an indexer to delete by ID.
// ID must be provided; callers should confirm existence before deletion to avoid silent no-ops.
type DeleteIndexerRequest struct {
	ID *int `json:"id" yaml:"id"`
}

// SeasonResult represents a season with metadata for API responses.
// Overview, AirDate, and PosterPath are optional fields from TMDB; EpisodeCount reflects known episodes; Monitored indicates tracking status.
type SeasonResult struct {
	TMDBID       int32   `json:"tmdbID"`
	SeriesID     int32   `json:"seriesID"`
	Number       int32   `json:"seasonNumber"`
	Title        string  `json:"title"`
	Overview     *string `json:"overview,omitempty"`
	AirDate      *string `json:"airDate,omitempty"`
	PosterPath   *string `json:"posterPath,omitempty"`
	EpisodeCount int32   `json:"episodeCount"`
	Monitored    bool    `json:"monitored"`
}

// EpisodeResult represents an episode with metadata for API responses.
// Overview, AirDate, StillPath, Runtime, and VoteAverage are optional TMDB fields; Downloaded reflects local status.
type EpisodeResult struct {
	TMDBID       int32    `json:"tmdbID"`
	SeriesID     int32    `json:"seriesID"`
	SeasonNumber int32    `json:"seasonNumber"`
	Number       int32    `json:"episodeNumber"`
	Title        string   `json:"title"`
	Overview     *string  `json:"overview,omitempty"`
	AirDate      *string  `json:"airDate,omitempty"`
	StillPath    *string  `json:"stillPath,omitempty"`
	Runtime      *int32   `json:"runtime,omitempty"`
	VoteAverage  *float64 `json:"voteAverage,omitempty"`
	Monitored    bool     `json:"monitored"`
	Downloaded   bool     `json:"downloaded"`
}
