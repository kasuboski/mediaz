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
	SeriesStorage
	SeriesMetadataStorage
}

type IndexerStorage interface {
	CreateIndexer(ctx context.Context, indexer model.Indexer) (int64, error)
	DeleteIndexer(ctx context.Context, id int64) error
	ListIndexers(ctx context.Context) ([]*model.Indexer, error)
}

type QualityStorage interface {
	CreateQualityProfile(ctx context.Context, profile model.QualityProfile) (int64, error)
	GetQualityProfile(ctx context.Context, id int64) (QualityProfile, error)
	ListQualityProfiles(ctx context.Context, where ...sqlite.BoolExpression) ([]*QualityProfile, error)
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
	GetMovieByPath(ctx context.Context, path string) (*Movie, error)
	GetMovieByMetadataID(ctx context.Context, metadataID int) (*Movie, error)
	CreateMovie(ctx context.Context, movie Movie, state MovieState) (int64, error)
	DeleteMovie(ctx context.Context, id int64) error
	ListMovies(ctx context.Context) ([]*Movie, error)
	ListMoviesByState(ctx context.Context, state MovieState) ([]*Movie, error)
	UpdateMovieState(ctx context.Context, id int64, state MovieState, metadata *MovieStateMetadata) error
	UpdateMovieMovieFileID(ctx context.Context, id int64, fileID int64) error

	GetMovieFilesByMovieName(ctx context.Context, name string) ([]*model.MovieFile, error)
	CreateMovieFile(ctx context.Context, movieFile model.MovieFile) (int64, error)
	DeleteMovieFile(ctx context.Context, id int64) error
	ListMovieFiles(ctx context.Context) ([]*model.MovieFile, error)
	LinkMovieMetadata(ctx context.Context, movieID int64, metadataID int32) error
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

type (
	SeriesState  string
	SeasonState  string
	EpisodeState string
)

const (
	SeriesStateNew         SeriesState = ""
	SeriesStateMissing     SeriesState = "missing"
	SeriesStateDiscovered  SeriesState = "discovered"
	SeriesStateUnreleased  SeriesState = "unreleased"
	SeriesStateContinuing  SeriesState = "continuing"
	SeriesStateDownloading SeriesState = "downloading"
	SeriesStateEnded       SeriesState = "ended"

	SeasonStateNew         SeasonState = ""
	SeasonStateMissing     SeasonState = "missing"
	SeasonStateDiscovered  SeasonState = "discovered"
	SeasonStateUnreleased  SeasonState = "unreleased"
	SeasonStateContinuing  SeasonState = "continuing"
	SeasonStateDownloading SeasonState = "downloading"
	SeasonStateEnded       SeasonState = "ended"

	EpisodeStateNew         EpisodeState = ""
	EpisodeStateMissing     EpisodeState = "missing"
	EpisodeStateDiscovered  EpisodeState = "discovered"
	EpisodeStateUnreleased  EpisodeState = "unreleased"
	EpisodeStateDownloading EpisodeState = "downloading"
	EpisodeStateDownloaded  EpisodeState = "downloaded"
)

type Series struct {
	model.Series
	State            SeriesState `alias:"series_transition.to_state" json:"state"`
	DownloadID       string      `alias:"series_transition.download_id" json:"-"`
	DownloadClientID int32       `alias:"series_transition.download_client_id" json:"-"`
}

type SeriesTransition model.SeriesTransition

func (s Series) Machine() *machine.StateMachine[SeriesState] {
	return machine.New(s.State,
		machine.From(SeriesStateNew).To(SeriesStateUnreleased, SeriesStateMissing, SeriesStateDiscovered),
		machine.From(SeriesStateMissing).To(SeriesStateDiscovered, SeriesStateDownloading),
		machine.From(SeriesStateUnreleased).To(SeriesStateDiscovered, SeriesStateMissing),
		machine.From(SeriesStateDownloading).To(SeriesStateContinuing, SeriesStateEnded),
	)
}

type Season struct {
	model.Season
	State            SeasonState `alias:"season_transition.to_state" json:"state"`
	DownloadID       string      `json:"-"`
	DownloadClientID int32       `json:"-"`
}

type SeasonTransition model.SeasonTransition

func (s Season) Machine() *machine.StateMachine[SeasonState] {
	return machine.New(s.State,
		machine.From(SeasonStateNew).To(SeasonStateUnreleased, SeasonStateMissing, SeasonStateDiscovered),
		machine.From(SeasonStateMissing).To(SeasonStateDiscovered, SeasonStateDownloading),
		machine.From(SeasonStateUnreleased).To(SeasonStateDiscovered, SeasonStateMissing),
		machine.From(SeasonStateDownloading).To(SeasonStateContinuing, SeasonStateEnded),
	)
}

type Episode struct {
	model.Episode
	State            EpisodeState `alias:"episode_transition.to_state" json:"state"`
	DownloadID       string       `json:"-"`
	DownloadClientID int32        `json:"-"`
}

type EpisodeTransition model.EpisodeTransition

func (e Episode) Machine() *machine.StateMachine[EpisodeState] {
	return machine.New(e.State,
		machine.From(EpisodeStateNew).To(EpisodeStateUnreleased, EpisodeStateMissing, EpisodeStateDiscovered),
		machine.From(EpisodeStateNew).To(EpisodeStateUnreleased, EpisodeStateMissing, EpisodeStateDiscovered),
		machine.From(EpisodeStateMissing).To(EpisodeStateDiscovered, EpisodeStateDownloading),
	)
}

type SeriesStorage interface {
	GetSeries(ctx context.Context, where sqlite.BoolExpression) (*Series, error)
	CreateSeries(ctx context.Context, series Series, initialState SeriesState) (int64, error)
	DeleteSeries(ctx context.Context, id int64) error
	ListSeries(ctx context.Context, where ...sqlite.BoolExpression) ([]*Series, error)

	GetSeason(ctx context.Context, where sqlite.BoolExpression) (*Season, error)
	CreateSeason(ctx context.Context, season Season, initialState SeasonState) (int64, error)
	DeleteSeason(ctx context.Context, id int64) error
	ListSeasons(ctx context.Context, where ...sqlite.BoolExpression) ([]*Season, error)

	GetEpisode(ctx context.Context, where sqlite.BoolExpression) (*Episode, error)
	GetEpisodeByEpisodeFileID(ctx context.Context, fileID int64) (*Episode, error)
	CreateEpisode(ctx context.Context, episode Episode, initialState EpisodeState) (int64, error)
	DeleteEpisode(ctx context.Context, id int64) error
	ListEpisodes(ctx context.Context, where ...sqlite.BoolExpression) ([]*Episode, error)
	UpdateEpisodeEpisodeFileID(ctx context.Context, id int64, fileID int64) error

	GetEpisodeFiles(ctx context.Context, id int64) ([]*model.EpisodeFile, error)
	CreateEpisodeFile(ctx context.Context, episodeFile model.EpisodeFile) (int64, error)
	DeleteEpisodeFile(ctx context.Context, id int64) error
	ListEpisodeFiles(ctx context.Context) ([]*model.EpisodeFile, error)
}

type SeriesMetadataStorage interface {
	CreateSeriesMetadata(ctx context.Context, SeriesMeta model.SeriesMetadata) (int64, error)
	DeleteSeriesMetadata(ctx context.Context, id int64) error
	ListSeriesMetadata(ctx context.Context, where ...sqlite.BoolExpression) ([]*model.SeriesMetadata, error)
	GetSeriesMetadata(ctx context.Context, where sqlite.BoolExpression) (*model.SeriesMetadata, error)

	CreateSeasonMetadata(ctx context.Context, seasonMeta model.SeasonMetadata) (int64, error)
	DeleteSeasonMetadata(ctx context.Context, id int64) error
	ListSeasonMetadata(ctx context.Context, where ...sqlite.BoolExpression) ([]*model.SeasonMetadata, error)
	GetSeasonMetadata(ctx context.Context, where sqlite.BoolExpression) (*model.SeasonMetadata, error)

	CreateEpisodeMetadata(ctx context.Context, episodeMeta model.EpisodeMetadata) (int64, error)
	DeleteEpisodeMetadata(ctx context.Context, id int64) error
	ListEpisodeMetadata(ctx context.Context, where ...sqlite.BoolExpression) ([]*model.EpisodeMetadata, error)
	GetEpisodeMetadata(ctx context.Context, where sqlite.BoolExpression) (*model.EpisodeMetadata, error)
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
