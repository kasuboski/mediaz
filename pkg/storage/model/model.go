// Package model holds the storage domain models.
//
// These structs are hand-maintained copies of the legacy ORM-generated shapes
// (int32 IDs, pointers for nullable columns) preserved verbatim during the
// sqlc migration so application callers keep a stable, ORM-free contract.
// See MIGRATION_CONTRACT.md: field names/types are frozen to the schema in
// pkg/storage/sqlite/schema/schema.sql; change both together.
package model

import "time"

type DownloadClient struct {
	ID             int32
	Type           string
	Implementation string
	Scheme         string
	Host           string
	Port           int32
	APIKey         *string
}

type Episode struct {
	ID                int32
	SeasonID          int32
	EpisodeNumber     int32
	Monitored         int32
	EpisodeMetadataID *int32
	EpisodeFileID     *int32
}

type EpisodeFile struct {
	ID               int32
	Quality          string
	Size             int64
	Added            time.Time
	RelativePath     *string
	OriginalFilePath *string
}

type EpisodeMetadata struct {
	ID               int32
	SeasonMetadataID int32
	Number           int32
	TmdbID           int32
	Title            string
	Overview         *string
	AirDate          *time.Time
	Runtime          *int32
	StillPath        *string
}

type EpisodeTransition struct {
	ID                     int32
	EpisodeID              int32
	ToState                string
	FromState              *string
	MostRecent             bool
	SortKey                int32
	DownloadClientID       *int32
	DownloadID             *string
	IsEntireSeasonDownload *bool
	CreatedAt              *time.Time
	UpdatedAt              *time.Time
}

type Indexer struct {
	ID              int32
	Name            string
	Priority        int32
	URI             string
	APIKey          *string
	IndexerSourceID *int32
}

type IndexerSource struct {
	ID             int32
	Name           string
	Implementation string
	Scheme         string
	Host           string
	Port           *int32
	APIKey         *string
	Enabled        bool
	CreatedAt      *time.Time
	UpdatedAt      *time.Time
}

type Job struct {
	ID        int32
	Type      string
	CreatedAt *time.Time
}

type JobTransition struct {
	ID         int32
	JobID      int32
	Type       string
	ToState    string
	FromState  *string
	MostRecent bool
	SortKey    int32
	Error      *string
	CreatedAt  *time.Time
	UpdatedAt  *time.Time
}

type Movie struct {
	ID                  int32
	Path                *string
	Monitored           int32
	QualityProfileID    int32
	Added               *time.Time
	Tags                *string
	AddOptions          *string
	MovieFileID         *int32
	MinimumAvailability int32
	MovieMetadataID     *int32
	LastSearchTime      *time.Time
}

type MovieFile struct {
	ID               int32
	Quality          string
	Size             int64
	DateAdded        time.Time
	SceneName        *string
	MediaInfo        *string
	ReleaseGroup     *string
	RelativePath     *string
	Edition          *string
	Languages        string
	IndexerFlags     int32
	OriginalFilePath *string
}

type MovieMetadata struct {
	ID                 int32
	TmdbID             int32
	ImdbID             *string
	Images             string
	Genres             *string
	Title              string
	SortTitle          *string
	CleanTitle         *string
	OriginalTitle      *string
	CleanOriginalTitle *string
	OriginalLanguage   int32
	Status             int32
	LastInfoSync       *time.Time
	Runtime            int32
	InCinemas          *time.Time
	ReleaseDate        *time.Time
	Year               *int32
	SecondaryYear      *int32
	Ratings            *string
	Recommendations    string
	Certification      *string
	YoutubeTrailerID   *string
	Studio             *string
	Overview           *string
	Website            *string
	Popularity         *float64
	CollectionTmdbID   *int32
	CollectionTitle    *string
}

type MovieTransition struct {
	ID               int32
	MovieID          int32
	ToState          string
	FromState        *string
	MostRecent       bool
	SortKey          int32
	DownloadClientID *int32
	DownloadID       *string
	CreatedAt        *time.Time
	UpdatedAt        *time.Time
}

type QualityDefinition struct {
	ID            int32
	QualityID     int32
	Name          string
	PreferredSize float64
	MinSize       float64
	MaxSize       float64
	MediaType     string
}

type QualityProfile struct {
	ID              int32
	Name            string
	CutoffQualityID *int32
	UpgradeAllowed  bool
}

type QualityProfileItem struct {
	ID        *int32
	ProfileID int32
	QualityID int32
}

type Season struct {
	ID               int32
	SeriesID         int32
	SeasonNumber     int32
	SeasonMetadataID *int32
	Monitored        int32
}

type SeasonMetadata struct {
	ID               int32
	SeriesMetadataID int32
	Number           int32
	TmdbID           int32
	Title            string
	Overview         *string
	AirDate          *time.Time
}

type SeasonTransition struct {
	ID                     int32
	SeasonID               int32
	ToState                string
	FromState              *string
	MostRecent             bool
	SortKey                int32
	DownloadClientID       *int32
	DownloadID             *string
	IsEntireSeasonDownload *bool
	CreatedAt              *time.Time
	UpdatedAt              *time.Time
}

type Series struct {
	ID                int32
	Path              *string
	Monitored         int32
	Added             *time.Time
	QualityProfileID  int32
	SeriesMetadataID  *int32
	LastSearchTime    *time.Time
	MonitorNewSeasons int32
}

type SeriesMetadata struct {
	ID             int32
	TmdbID         int32
	Title          string
	Overview       *string
	LastInfoSync   *time.Time
	FirstAirDate   *time.Time
	LastAirDate    *time.Time
	SeasonCount    int32
	EpisodeCount   int32
	Status         string
	PosterPath     *string
	ExternalIds    *string
	WatchProviders *string
}

type SeriesTransition struct {
	ID         int32
	SeriesID   int32
	ToState    string
	FromState  *string
	MostRecent bool
	SortKey    int32
	CreatedAt  *time.Time
	UpdatedAt  *time.Time
}
