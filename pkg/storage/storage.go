package storage

import (
	"context"
	"errors"
	"os"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/pkg/machine"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
)

var ErrNotFound = errors.New("not found in storage")

type Storage interface {
	Init(ctx context.Context, schemas ...string) error
	IndexerStorage
	QualityStorage
	MovieStorage
	MovieMetadataStorage
	DownloadClientStorage
	ShowStorage
}

type IndexerStorage interface {
	CreateIndexer(ctx context.Context, indexer model.Indexer) (int64, error)
	DeleteIndexer(ctx context.Context, id int64) error
	ListIndexers(ctx context.Context) ([]*model.Indexer, error)
}

type QualityStorage interface {
	CreateQualityProfile(ctx context.Context, profile model.QualityProfile) (int64, error)
	GetQualityProfile(ctx context.Context, id int64) (QualityProfile, error)
	ListQualityProfiles(ctx context.Context) ([]*QualityProfile, error)
	DeleteQualityProfile(ctx context.Context, id int64) error //TODO: do we cascade associated items?

	CreateQualityProfileItem(ctx context.Context, item model.QualityProfileItem) (int64, error)
	DeleteQualityProfileItem(ctx context.Context, id int64) error
	GetQualityProfileItem(ctx context.Context, id int64) (model.QualityProfileItem, error)
	ListQualityProfileItems(ctx context.Context) ([]*model.QualityProfileItem, error)

	CreateQualityDefinition(ctx context.Context, definition model.QualityDefinition) (int64, error)
	GetQualityDefinition(ctx context.Context, id int64) (model.QualityDefinition, error)
	ListQualityDefinitions(ctx context.Context) ([]*model.QualityDefinition, error)
	DeleteQualityDefinition(ctx context.Context, id int64) error
}

type MovieState string

const (
	MovieStateNew         MovieState = ""
	MovieStateMissing     MovieState = "missing"
	MovieStateDiscovered  MovieState = "discovered"
	MovieStateUnreleased  MovieState = "unreleased"
	MovieStateDownloading MovieState = "downloading"
	MovieStateDownloaded  MovieState = "downloaded"
)

type MovieStateMetadata struct {
	DownloadID       *string
	DownloadClientID *int32
}

type Movie struct {
	model.Movie
	State            MovieState `alias:"movie_transition.to_state" json:"state"`
	DownloadID       string     `alias:"movie_transition.download_id" json:"-"`
	DownloadClientID int32      `alias:"movie_transition.download_client_id" json:"-"`
}

type MovieTransition model.MovieTransition

func (m Movie) Machine() *machine.StateMachine[MovieState] {
	return machine.New(m.State,
		machine.From(MovieStateNew).To(MovieStateUnreleased, MovieStateMissing, MovieStateDiscovered),
		machine.From(MovieStateMissing).To(MovieStateDiscovered, MovieStateDownloading),
		machine.From(MovieStateUnreleased).To(MovieStateDiscovered, MovieStateMissing),
		machine.From(MovieStateDownloading).To(MovieStateDownloaded),
	)
}

type MovieStorage interface {
	GetMovie(ctx context.Context, id int64) (*Movie, error)
	GetMovieByMovieFileID(ctx context.Context, fileID int64) (*Movie, error)
	GetMovieByMetadataID(ctx context.Context, metadataID int) (*Movie, error)
	CreateMovie(ctx context.Context, movie Movie, state MovieState) (int64, error)
	DeleteMovie(ctx context.Context, id int64) error
	ListMovies(ctx context.Context) ([]*Movie, error)
	ListMoviesByState(ctx context.Context, state MovieState) ([]*Movie, error)
	UpdateMovieState(ctx context.Context, id int64, state MovieState, metadata *MovieStateMetadata) error
	UpdateMovieMovieFileID(ctx context.Context, id int64, fileID int64) error

	GetMovieFiles(ctx context.Context, id int64) ([]*model.MovieFile, error)
	CreateMovieFile(ctx context.Context, movieFile model.MovieFile) (int64, error)
	DeleteMovieFile(ctx context.Context, id int64) error
	ListMovieFiles(ctx context.Context) ([]*model.MovieFile, error)
}

type MovieMetadataStorage interface {
	CreateMovieMetadata(ctx context.Context, movieMeta model.MovieMetadata) (int64, error)
	DeleteMovieMetadata(ctx context.Context, id int64) error
	ListMovieMetadata(ctx context.Context) ([]*model.MovieMetadata, error)
	GetMovieMetadata(ctx context.Context, where sqlite.BoolExpression) (*model.MovieMetadata, error)
}

type DownloadClientStorage interface {
	CreateDownloadClient(ctx context.Context, client model.DownloadClient) (int64, error)
	GetDownloadClient(ctx context.Context, id int64) (model.DownloadClient, error)
	ListDownloadClients(ctx context.Context) ([]*model.DownloadClient, error)
	DeleteDownloadClient(ctx context.Context, id int64) error
}

type QualityProfile struct {
	Name            string              `json:"name"`
	Qualities       []QualityDefinition `json:"qualities"`
	ID              int32               `sql:"primary_key" json:"id"`
	CutoffQualityID int32               `alias:"cutoff_quality_id" json:"cutoff_quality_id"`
	UpgradeAllowed  bool                `json:"upgradeAllowed"`
}

type QualityDefinition struct {
	Name          string  `alias:"quality_definition.name" json:"name"`
	MediaType     string  `alias:"quality_definition.media_type" json:"type"`
	PreferredSize float64 `alias:"quality_definition.preferred_size" json:"preferredSize"`
	MinSize       float64 `alias:"quality_definition.min_size" json:"minSize"`
	MaxSize       float64 `alias:"quality_definition.max_size" json:"maxSize"`
	QualityID     int32   `alias:"quality_definition.quality_id" json:"-"`
}

type Episode struct {
	model.Episode
	State            EpisodeState `json:"state"`
	DownloadID       string       `json:"-"`
	DownloadClientID int32        `json:"-"`
}

type EpisodeState string

const (
	EpisodeStateNew         EpisodeState = ""
	EpisodeStateMissing     EpisodeState = "missing"
	EpisodeStateUnreleased  EpisodeState = "unreleased"
	EpisodeStateDownloading EpisodeState = "downloading"
)

type ShowStorage interface {
	GetShow(ctx context.Context, id int64) (*model.Show, error)
	CreateShow(ctx context.Context, show model.Show) (int64, error)
	DeleteShow(ctx context.Context, id int64) error
	ListShows(ctx context.Context) ([]*model.Show, error)

	GetSeason(ctx context.Context, id int64) (*model.Season, error)
	CreateSeason(ctx context.Context, season model.Season) (int64, error)
	DeleteSeason(ctx context.Context, id int64) error
	ListSeasons(ctx context.Context, showID int64) ([]*model.Season, error)

	GetEpisode(ctx context.Context, id int64) (*Episode, error)
	CreateEpisode(ctx context.Context, episode Episode) (int64, error)
	DeleteEpisode(ctx context.Context, id int64) error
	ListEpisodes(ctx context.Context, seasonID int64) ([]*Episode, error)
	ListEpisodesByState(ctx context.Context, state EpisodeState) ([]*Episode, error)
}

func ReadSchemaFiles(files ...string) ([]string, error) {
	var schemas []string
	for _, f := range files {
		f, err := os.ReadFile(f)
		if err != nil {
			return schemas, err
		}

		schemas = append(schemas, string(f))
	}

	return schemas, nil
}
