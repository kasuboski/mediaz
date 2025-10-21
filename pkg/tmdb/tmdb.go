package tmdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/kasuboski/mediaz/pkg/logger"
	"go.uber.org/zap"
)

const (
	ReleaseDateFormat = "2006-01-02"
)

type ITmdb interface {
	ClientInterface
	GetMovieDetails(context.Context, int) (*MediaDetails, error)
	GetSeriesDetails(context.Context, int) (*SeriesDetails, error)
}

type TMDBClient struct {
	ClientInterface
}

func New(url, apiKey string, options ...ClientOption) (ITmdb, error) {
	options = append(options, WithRequestEditorFn(SetRequestAPIKey(apiKey)))

	client, err := NewClient(url, options...)
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

// GetSeriesDetails pulls all metadata for a series, its seasons, and the episodes contained within those seasons.
func (m TMDBClient) GetSeriesDetails(ctx context.Context, tmdbID int) (*SeriesDetails, error) {
	log := logger.FromCtx(ctx)

	res, err := m.ClientInterface.TvSeriesDetails(ctx, int32(tmdbID), nil)
	if err != nil {
		log.Debug("failed to get series details", zap.Error(err))
		return nil, err
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		log.Debug("failed to read series details", zap.Error(err))
		return nil, err
	}

	var seriesDetailsResponse SeriesDetailsResponse
	err = json.Unmarshal(b, &seriesDetailsResponse)
	if err != nil {
		log.Debug("failed to unmarshal series details", zap.Error(err))
		return nil, err
	}

	var seasons []Season
	for _, s := range seriesDetailsResponse.Seasons {
		resp, err := m.ClientInterface.TvSeasonDetails(ctx, int32(seriesDetailsResponse.ID), int32(s.SeasonNumber), nil)
		if err != nil {
			log.Debug("failed to get season details", zap.Error(err))
			continue
		}

		b, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Debug("failed to read season details", zap.Error(err))
			continue
		}

		var season TvSeasonDetails
		err = json.Unmarshal(b, &season)
		if err != nil {
			log.Debug("failed to unmarshal season details", zap.Error(err))
			continue
		}
		seasons = append(seasons, season.ToSeason())
		continue
	}

	seriesDetails := &SeriesDetails{
		Seasons:          seasons,
		ID:               seriesDetailsResponse.ID,
		FirstAirDate:     seriesDetailsResponse.FirstAirDate,
		NumberOfEpisodes: seriesDetailsResponse.NumberOfEpisodes,
		NumberOfSeasons:  seriesDetailsResponse.NumberOfSeasons,
		Name:             seriesDetailsResponse.Name,
		OriginalLanguage: seriesDetailsResponse.OriginalLanguage,
		Languages:        seriesDetailsResponse.Languages,
		PosterPath:       seriesDetailsResponse.PosterPath,
		Overview:         seriesDetailsResponse.Overview,
	}

	return seriesDetails, nil
}

func (t TvSeasonDetails) ToSeason() Season {
	episodes := make([]Episode, len(t.Episodes))

	for i, episode := range t.Episodes {
		episodes[i] = Episode{
			ID:             episode.ID,
			Name:           episode.Name,
			SeasonNumber:   episode.SeasonNumber,
			AirDate:        episode.AirDate,
			Runtime:        episode.Runtime,
			EpisodeNumber:  episode.EpisodeNumber,
			Overview:       episode.Overview,
			ProductionCode: episode.ProductionCode,
			StillPath:      episode.StillPath,
			Crew:           make([]Crew, len(episode.Crew)),
			GuestStars:     make([]GuestStar, len(episode.GuestStars)),
		}
	}

	season := Season{
		ID:           t.ID,
		AirDate:      t.AirDate,
		Name:         t.Name,
		PosterPath:   t.PosterPath,
		SeasonNumber: t.SeasonNumber,
		Overview:     t.Overview,
		Episodes:     episodes,
	}

	return season
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
	SpokenLanguages     *[]SpokenLanguage    `json:"spoken_languages,omitempty"`
	Genres              *[]Genre             `json:"genres,omitempty"`
	PosterPath          *string              `json:"poster_path,omitempty"`
	Budget              *int                 `json:"budget,omitempty"`
	ProductionCompanies *[]ProductionCompany `json:"production_companies,omitempty"`
	Homepage            *string              `json:"homepage,omitempty"`
	VoteCount           *int                 `json:"vote_count,omitempty"`
	ProductionCountries *[]ProductionCountry `json:"production_countries,omitempty"`
	OriginalLanguage    *string              `json:"original_language,omitempty"`
	OriginalTitle       *string              `json:"original_title,omitempty"`
	Overview            *string              `json:"overview,omitempty"`
	Popularity          *float32             `json:"popularity,omitempty"`
	BelongsToCollection *any                 `json:"belongs_to_collection,omitempty"`
	BackdropPath        *string              `json:"backdrop_path,omitempty"`
	ImdbID              *string              `json:"imdb_id,omitempty"`
	ReleaseDate         *string              `json:"release_date,omitempty"`
	Revenue             *int                 `json:"revenue,omitempty"`
	Runtime             *int                 `json:"runtime,omitempty"`
	Adult               *bool                `json:"adult,omitempty"`
	Status              *string              `json:"status,omitempty"`
	Tagline             *string              `json:"tagline,omitempty"`
	Title               *string              `json:"title,omitempty"`
	Video               *bool                `json:"video,omitempty"`
	VoteAverage         *float32             `json:"vote_average,omitempty"`
	ID                  int                  `json:"id,omitempty"`
}

type TvSeasonDetails struct {
	Identifier   string    `json:"_id,omitempty"`
	AirDate      string    `json:"air_date,omitempty"`
	Episodes     []Episode `json:"episodes,omitempty"`
	ID           int       `json:"id,omitempty"`
	Name         string    `json:"name,omitempty"`
	Overview     string    `json:"overview,omitempty"`
	PosterPath   string    `json:"poster_path,omitempty"`
	SeasonNumber int       `json:"season_number,omitempty"`
	VoteAverage  float32   `json:"vote_average,omitempty"`
}

type TvSeasonDetailsEpisode struct {
	AirDate        string                      `json:"air_date,omitempty"`
	Crew           []TvSeasonDetailsCrewMember `json:"crew,omitempty"`
	EpisodeNumber  int                         `json:"episode_number,omitempty"`
	GuestStars     []TvSeasonDetailsGuestStar  `json:"guest_stars,omitempty"`
	ID             int                         `json:"id,omitempty"`
	Name           string                      `json:"name,omitempty"`
	Overview       string                      `json:"overview,omitempty"`
	ProductionCode string                      `json:"production_code,omitempty"`
	Runtime        int                         `json:"runtime,omitempty"`
	SeasonNumber   int                         `json:"season_number,omitempty"`
	ShowID         int                         `json:"show_id,omitempty"`
	StillPath      string                      `json:"still_path,omitempty"`
	VoteAverage    float32                     `json:"vote_average,omitempty"`
	VoteCount      int                         `json:"vote_count,omitempty"`
}

type TvSeasonDetailsCrewMember struct {
	Adult              bool    `json:"adult,omitempty"`
	CreditID           string  `json:"credit_id,omitempty"`
	Department         string  `json:"department,omitempty"`
	Gender             int     `json:"gender,omitempty"`
	ID                 int     `json:"id,omitempty"`
	Job                string  `json:"job,omitempty"`
	KnownForDepartment string  `json:"known_for_department,omitempty"`
	Name               string  `json:"name,omitempty"`
	OriginalName       string  `json:"original_name,omitempty"`
	Popularity         float32 `json:"popularity,omitempty"`
	ProfilePath        string  `json:"profile_path,omitempty"`
}

type TvSeasonDetailsGuestStar struct {
	Adult              bool    `json:"adult,omitempty"`
	Character          string  `json:"character,omitempty"`
	CreditID           string  `json:"credit_id,omitempty"`
	Gender             int     `json:"gender,omitempty"`
	ID                 int     `json:"id,omitempty"`
	KnownForDepartment string  `json:"known_for_department,omitempty"`
	Name               string  `json:"name,omitempty"`
	Order              int     `json:"order,omitempty"`
	OriginalName       string  `json:"original_name,omitempty"`
	Popularity         float32 `json:"popularity,omitempty"`
	ProfilePath        string  `json:"profile_path,omitempty"`
}

// SeriesDetailsResponse is the series response from the TMDB api.
type SeriesDetailsResponse struct {
	Adult            bool                        `json:"adult,omitempty"`
	BackdropPath     string                      `json:"backdrop_path,omitempty"`
	CreatedBy        []Creator                   `json:"created_by,omitempty"`
	EpisodeRunTime   []int                       `json:"episode_run_time,omitempty"`
	FirstAirDate     string                      `json:"first_air_date,omitempty"`
	Genres           []Genre                     `json:"genres,omitempty"`
	Homepage         string                      `json:"homepage,omitempty"`
	ID               int                         `json:"id,omitempty"`
	InProduction     bool                        `json:"in_production,omitempty"`
	Languages        []string                    `json:"languages,omitempty"`
	LastAirDate      string                      `json:"last_air_date,omitempty"`
	LastEpisodeToAir SeriesDetailsResponseSeason `json:"last_episode_to_air,omitempty"`
	Name             string                      `json:"name,omitempty"`
	NextEpisodeToAir SeriesDetailsResponseSeason `json:"next_episode_to_air,omitempty"`
	Networks         []Network                   `json:"networks,omitempty"`
	NumberOfEpisodes int                         `json:"number_of_episodes,omitempty"`
	NumberOfSeasons  int                         `json:"number_of_seasons,omitempty"`
	OriginCountry    []string                    `json:"origin_country,omitempty"`
	OriginalLanguage string                      `json:"original_language,omitempty"`
	OriginalName     string                      `json:"original_name,omitempty"`
	Overview         string                      `json:"overview,omitempty"`
	Popularity       float64                     `json:"popularity,omitempty"`
	PosterPath       string                      `json:"poster_path,omitempty"`
	// Add series-level rating fields
	VoteAverage         float64                       `json:"vote_average,omitempty"`
	VoteCount           int                           `json:"vote_count,omitempty"`
	ProductionCompanies []ProductionCompany           `json:"production_companies,omitempty"`
	ProductionCountries []ProductionCountry           `json:"production_countries,omitempty"`
	Seasons             []SeriesDetailsResponseSeason `json:"seasons,omitempty"`
	Status              string                        `json:"status,omitempty"`
}

type SeriesDetails struct {
	ID               int      `json:"id"`
	FirstAirDate     string   `json:"first_air_date"`
	NumberOfEpisodes int      `json:"number_of_episodes"`
	NumberOfSeasons  int      `json:"number_of_seasons"`
	Name             string   `json:"name"`
	OriginalLanguage string   `json:"original_language"`
	Languages        []string `json:"languages"`
	PosterPath       string   `json:"poster_path"`
	Seasons          []Season `json:"seasons"`
	Overview         string   `json:"overview"`
}

type Season struct {
	ID           int       `json:"id"`
	AirDate      string    `json:"air_date"`
	Name         string    `json:"name"`
	PosterPath   string    `json:"poster_path"`
	SeasonNumber int       `json:"season_number"`
	Runtime      int       `json:"runtime"`
	Overview     string    `json:"overview"`
	Episodes     []Episode `json:"episodes"`
}

type Episode struct {
	ID             int         `json:"id"`
	Name           string      `json:"name"`
	SeasonNumber   int         `json:"season_number"`
	AirDate        string      `json:"air_date"`
	EpisodeNumber  int         `json:"episode_number"`
	Overview       string      `json:"overview"`
	ProductionCode string      `json:"production_code"`
	StillPath      string      `json:"still_path"`
	Crew           []Crew      `json:"crew"`
	Runtime        int         `json:"runtime"`
	GuestStars     []GuestStar `json:"guest_stars"`
}

type Crew struct {
	Job                string  `json:"job"`
	Department         string  `json:"department"`
	CreditID           string  `json:"credit_id"`
	Adult              bool    `json:"adult"`
	Gender             int     `json:"gender"`
	ID                 int     `json:"id"`
	KnownForDepartment string  `json:"known_for_department"`
	Name               string  `json:"name"`
	OriginalName       string  `json:"original_name"`
	Popularity         float64 `json:"popularity"`
	ProfilePath        string  `json:"profile_path"`
}

type GuestStar struct {
	Character    string  `json:"character"`
	CreditID     string  `json:"credit_id"`
	Order        int     `json:"order"`
	Adult        bool    `json:"adult"`
	Gender       int     `json:"gender"`
	ID           int     `json:"id"`
	KnownFor     string  `json:"known_for"`
	Name         string  `json:"name"`
	OriginalName string  `json:"original_name"`
	Popularity   float64 `json:"popularity"`
	ProfilePath  string  `json:"profile_path"`
}

type Creator struct {
	ID           int    `json:"id,omitempty"`
	CreditID     string `json:"credit_id,omitempty"`
	Name         string `json:"name,omitempty"`
	OriginalName string `json:"original_name,omitempty"`
	Gender       int    `json:"gender,omitempty"`
	ProfilePath  string `json:"profile_path,omitempty"`
}

type SeriesDetailsResponseEpisode struct {
	ID             int     `json:"id,omitempty"`
	Name           string  `json:"name,omitempty"`
	Overview       string  `json:"overview,omitempty"`
	VoteAverage    float64 `json:"vote_average,omitempty"`
	VoteCount      int     `json:"vote_count,omitempty"`
	AirDate        string  `json:"air_date,omitempty"`
	EpisodeNumber  int     `json:"episode_number,omitempty"`
	EpisodeType    string  `json:"episode_type,omitempty"`
	ProductionCode string  `json:"production_code,omitempty"`
	Runtime        int     `json:"runtime,omitempty"`
	SeasonNumber   int     `json:"season_number,omitempty"`
	ShowID         int     `json:"show_id,omitempty"`
	StillPath      string  `json:"still_path,omitempty"`
}

type Network struct {
	ID            int    `json:"id,omitempty"`
	LogoPath      string `json:"logo_path,omitempty"`
	Name          string `json:"name,omitempty"`
	OriginCountry string `json:"origin_country,omitempty"`
}

type SeriesDetailsResponseSeason struct {
	AirDate      string  `json:"air_date,omitempty"`
	EpisodeCount int     `json:"episode_count,omitempty"`
	ID           int     `json:"id,omitempty"`
	Name         string  `json:"name,omitempty"`
	Overview     string  `json:"overview,omitempty"`
	PosterPath   string  `json:"poster_path,omitempty"`
	SeasonNumber int     `json:"season_number,omitempty"`
	VoteAverage  float64 `json:"vote_average,omitempty"`
}

type Genre struct {
	ID   int    `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
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
