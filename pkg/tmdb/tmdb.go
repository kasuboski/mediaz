package tmdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

type ITmdb interface {
	ClientInterface
	GetMovieDetails(context.Context, int) (*MediaDetails, error)
}

type TMDBClient struct {
	ClientInterface
}

func New(url, apiKey string) (*TMDBClient, error) {
	client, err := NewClient(url, WithRequestEditorFn(SetRequestAPIKey(apiKey)))
	if err != nil {
		return nil, err
	}
	return &TMDBClient{
		ClientInterface: client,
	}, nil
}

func SetRequestAPIKey(apiKey string) func(ctx context.Context, req *http.Request) error {
	return func(ctx context.Context, req *http.Request) error {
		req.Header.Add("Authorization", "Bearer "+apiKey)
		req.Header.Add("accept", "application/json")
		return nil
	}
}

func (m TMDBClient) GetMovieDetails(ctx context.Context, tmdbID int) (*MediaDetails, error) {
	res, err := m.ClientInterface.MovieDetails(ctx, int32(tmdbID), nil)
	if err != nil {
		return nil, fmt.Errorf("couldn't get movie details: %w", err)
	}
	defer res.Body.Close()

	det, err := parseMediaDetailsResponse(res)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse details response: %w", err)
	}

	return det, nil
}

func parseMediaDetailsResponse(res *http.Response) (*MediaDetails, error) {
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected media query status status: %s", res.Status)
	}

	b, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if len(b) == 0 || (len(b) == 2 && string(b) == "{}") {
		return nil, errors.New("empty media details body")
	}
	results := new(MediaDetails)
	err = json.Unmarshal(b, results)
	if results.ID == 0 {
		return nil, errors.New("unable to parse media details response")
	}
	return results, err
}

type MediaDetails struct {
	Adult               *bool                `json:"adult,omitempty"`
	BackdropPath        *string              `json:"backdrop_path,omitempty"`
	BelongsToCollection *interface{}         `json:"belongs_to_collection,omitempty"`
	Budget              *int                 `json:"budget,omitempty"`
	Genres              *[]Genre             `json:"genres,omitempty"`
	Homepage            *string              `json:"homepage,omitempty"`
	ID                  int                  `json:"id,omitempty"`
	ImdbID              *string              `json:"imdb_id,omitempty"`
	OriginalLanguage    *string              `json:"original_language,omitempty"`
	OriginalTitle       *string              `json:"original_title,omitempty"`
	Overview            *string              `json:"overview,omitempty"`
	Popularity          *float32             `json:"popularity,omitempty"`
	PosterPath          *string              `json:"poster_path,omitempty"`
	ProductionCompanies *[]ProductionCompany `json:"production_companies,omitempty"`
	ProductionCountries *[]ProductionCountry `json:"production_countries,omitempty"`
	ReleaseDate         *string              `json:"release_date,omitempty"`
	Revenue             *int                 `json:"revenue,omitempty"`
	Runtime             *int                 `json:"runtime,omitempty"`
	SpokenLanguages     *[]SpokenLanguage    `json:"spoken_languages,omitempty"`
	Status              *string              `json:"status,omitempty"`
	Tagline             *string              `json:"tagline,omitempty"`
	Title               *string              `json:"title,omitempty"`
	Video               *bool                `json:"video,omitempty"`
	VoteAverage         *float32             `json:"vote_average,omitempty"`
	VoteCount           *int                 `json:"vote_count,omitempty"`
}

type Genre struct {
	ID   *int    `json:"id,omitempty"`
	Name *string `json:"name,omitempty"`
}

type ProductionCompany struct {
	ID            *int    `json:"id,omitempty"`
	LogoPath      *string `json:"logo_path,omitempty"`
	Name          *string `json:"name,omitempty"`
	OriginCountry *string `json:"origin_country,omitempty"`
}

type ProductionCountry struct {
	Iso31661 *string `json:"iso_3166_1,omitempty"`
	Name     *string `json:"name,omitempty"`
}

type SpokenLanguage struct {
	EnglishName *string `json:"english_name,omitempty"`
	Iso6391     *string `json:"iso_639_1,omitempty"`
	Name        *string `json:"name,omitempty"`
}
