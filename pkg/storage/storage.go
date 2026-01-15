package storage

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/pkg/machine"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
)

var ErrNotFound = errors.New("not found in storage")
var ErrJobAlreadyPending = errors.New("job of this type already pending")

//go:embed sqlite/schema/*.sql
var schemaFiles embed.FS

type Storage interface {
	Init(ctx context.Context, schemas ...string) error
	RunMigrations(ctx context.Context) error
	IndexerStorage
	IndexerSourceStorage
	QualityStorage
	MovieStorage
	MovieMetadataStorage
	DownloadClientStorage
	JobStorage
	SeriesStorage
	SeriesMetadataStorage
	StatisticsStorage
	ActivityStorage
}

type IndexerStorage interface {
	CreateIndexer(ctx context.Context, indexer model.Indexer) (int64, error)
	UpdateIndexer(ctx context.Context, id int64, indexer model.Indexer) error
	DeleteIndexer(ctx context.Context, id int64) error
	ListIndexers(ctx context.Context, where ...sqlite.BoolExpression) ([]*model.Indexer, error)
}

type IndexerSourceStorage interface {
	CreateIndexerSource(ctx context.Context, source model.IndexerSource) (int64, error)
	GetIndexerSource(ctx context.Context, id int64) (model.IndexerSource, error)
	ListIndexerSources(ctx context.Context, where ...sqlite.BoolExpression) ([]*model.IndexerSource, error)
	UpdateIndexerSource(ctx context.Context, id int64, source model.IndexerSource) error
	DeleteIndexerSource(ctx context.Context, id int64) error
}

type QualityStorage interface {
	CreateQualityProfile(ctx context.Context, profile model.QualityProfile) (int64, error)
	GetQualityProfile(ctx context.Context, id int64) (QualityProfile, error)
	ListQualityProfiles(ctx context.Context, where ...sqlite.BoolExpression) ([]*QualityProfile, error)
	UpdateQualityProfile(ctx context.Context, id int64, profile model.QualityProfile) error
	DeleteQualityProfile(ctx context.Context, id int64) error //TODO: do we cascade associated items?

	CreateQualityProfileItem(ctx context.Context, item model.QualityProfileItem) (int64, error)
	CreateQualityProfileItems(ctx context.Context, items []model.QualityProfileItem) error
	DeleteQualityProfileItem(ctx context.Context, id int64) error
	DeleteQualityProfileItemsByProfileID(ctx context.Context, profileID int64) error
	GetQualityProfileItem(ctx context.Context, id int64) (model.QualityProfileItem, error)
	ListQualityProfileItems(ctx context.Context) ([]*model.QualityProfileItem, error)

	CreateQualityDefinition(ctx context.Context, definition model.QualityDefinition) (int64, error)
	GetQualityDefinition(ctx context.Context, id int64) (model.QualityDefinition, error)
	ListQualityDefinitions(ctx context.Context) ([]*model.QualityDefinition, error)
	UpdateQualityDefinition(ctx context.Context, id int64, definition model.QualityDefinition) error
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

type TransitionStateMetadata struct {
	DownloadID             *string
	DownloadClientID       *int32
	IsEntireSeasonDownload *bool // applicable only to episodes
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
	UpdateMovieState(ctx context.Context, id int64, state MovieState, metadata *TransitionStateMetadata) error
	UpdateMovieMovieFileID(ctx context.Context, id int64, fileID int64) error
	UpdateMovieQualityProfile(ctx context.Context, id int64, qualityProfileID int32) error
	UpdateMovie(ctx context.Context, movie model.Movie, where ...sqlite.BoolExpression) error

	GetMovieFilesByMovieName(ctx context.Context, name string) ([]*model.MovieFile, error)
	CreateMovieFile(ctx context.Context, movieFile model.MovieFile) (int64, error)
	DeleteMovieFile(ctx context.Context, id int64) error
	ListMovieFiles(ctx context.Context) ([]*model.MovieFile, error)
	LinkMovieMetadata(ctx context.Context, movieID int64, metadataID int32) error
}

type MovieMetadataStorage interface {
	CreateMovieMetadata(ctx context.Context, movieMeta model.MovieMetadata) (int64, error)
	UpdateMovieMetadata(ctx context.Context, metadata model.MovieMetadata) error
	DeleteMovieMetadata(ctx context.Context, id int64) error
	ListMovieMetadata(ctx context.Context) ([]*model.MovieMetadata, error)
	GetMovieMetadata(ctx context.Context, where sqlite.BoolExpression) (*model.MovieMetadata, error)
}

type DownloadClientStorage interface {
	CreateDownloadClient(ctx context.Context, client model.DownloadClient) (int64, error)
	GetDownloadClient(ctx context.Context, id int64) (model.DownloadClient, error)
	ListDownloadClients(ctx context.Context) ([]*model.DownloadClient, error)
	UpdateDownloadClient(ctx context.Context, id int64, client model.DownloadClient) error
	DeleteDownloadClient(ctx context.Context, id int64) error
}

type JobState string

const (
	JobStateNew       JobState = ""
	JobStatePending   JobState = "pending"
	JobStateRunning   JobState = "running"
	JobStateError     JobState = "error"
	JobStateDone      JobState = "done"
	JobStateCancelled JobState = "cancelled"
)

type Job struct {
	model.Job
	State     JobState   `alias:"job_transition.to_state" json:"state"`
	UpdatedAt *time.Time `alias:"job_transition.updated_at" json:"updatedAt"`
	Error     *string    `alias:"job_transition.error" json:"error,omitempty"`
}

type JobTransition model.JobTransition

func (j Job) Machine() *machine.StateMachine[JobState] {
	return machine.New(j.State,
		machine.From(JobStateNew).To(JobStatePending),
		machine.From(JobStatePending).To(JobStateRunning, JobStateError, JobStateCancelled),
		machine.From(JobStateRunning).To(JobStateError, JobStateDone, JobStateCancelled),
	)
}

type JobStorage interface {
	CreateJob(ctx context.Context, job Job, initialState JobState) (int64, error)
	GetJob(ctx context.Context, id int64) (*Job, error)
	CountJobs(ctx context.Context, where ...sqlite.BoolExpression) (int, error)
	ListJobs(ctx context.Context, offset, limit int, where ...sqlite.BoolExpression) ([]*Job, error)
	UpdateJobState(ctx context.Context, id int64, state JobState, errorMsg *string) error
	DeleteJob(ctx context.Context, id int64) error
	DeleteJobs(ctx context.Context, where ...sqlite.BoolExpression) (int64, error)
}

type QualityProfile struct {
	Name            string              `json:"name"`
	Qualities       []QualityDefinition `json:"qualities"`
	ID              int32               `sql:"primary_key" json:"id"`
	CutoffQualityID *int32              `alias:"cutoff_quality_id" json:"cutoff_quality_id,omitempty"`
	UpgradeAllowed  bool                `json:"upgradeAllowed"`
}

type QualityDefinition struct {
	ID            int32   `alias:"quality_definition.id" json:"id"`
	Name          string  `alias:"quality_definition.name" json:"name"`
	MediaType     string  `alias:"quality_definition.media_type" json:"type"`
	PreferredSize float64 `alias:"quality_definition.preferred_size" json:"preferredSize"`
	MinSize       float64 `alias:"quality_definition.min_size" json:"minSize"`
	MaxSize       float64 `alias:"quality_definition.max_size" json:"maxSize"`
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
	SeriesStatePartial     SeriesState = "partial"
	SeriesStateCompleted   SeriesState = "completed"

	SeasonStateNew         SeasonState = ""
	SeasonStateMissing     SeasonState = "missing"
	SeasonStateDiscovered  SeasonState = "discovered"
	SeasonStateUnreleased  SeasonState = "unreleased"
	SeasonStateContinuing  SeasonState = "continuing"
	SeasonStateDownloading SeasonState = "downloading"
	SeasonStatePartial     SeasonState = "partial"
	SeasonStateCompleted   SeasonState = "completed"

	EpisodeStateNew         EpisodeState = ""
	EpisodeStateMissing     EpisodeState = "missing"
	EpisodeStateDiscovered  EpisodeState = "discovered"
	EpisodeStateUnreleased  EpisodeState = "unreleased"
	EpisodeStateDownloading EpisodeState = "downloading"
	EpisodeStateDownloaded  EpisodeState = "downloaded"
	EpisodeStateCompleted   EpisodeState = "completed"
)

type Series struct {
	model.Series
	State SeriesState `alias:"series_transition.to_state" json:"state"`
}

type SeriesTransition model.SeriesTransition

func (s Series) Machine() *machine.StateMachine[SeriesState] {
	return machine.New(s.State,
		machine.From(SeriesStateNew).To(SeriesStateUnreleased, SeriesStateMissing, SeriesStateDiscovered),
		machine.From(SeriesStateDiscovered).To(SeriesStateMissing, SeriesStateContinuing, SeriesStatePartial, SeriesStateCompleted),
		machine.From(SeriesStateMissing).To(SeriesStateDiscovered, SeriesStateDownloading),
		machine.From(SeriesStateUnreleased).To(SeriesStateDiscovered, SeriesStateMissing),
		machine.From(SeriesStateDownloading).To(SeriesStateContinuing, SeriesStatePartial, SeriesStateCompleted),
		machine.From(SeriesStateContinuing).To(SeriesStatePartial, SeriesStateCompleted, SeriesStateMissing),
		machine.From(SeriesStatePartial).To(SeriesStateContinuing, SeriesStateCompleted, SeriesStateMissing),
		machine.From(SeriesStateCompleted).To(SeriesStateContinuing, SeriesStatePartial),
	)
}

type Season struct {
	model.Season
	DownloadID       string      `alias:"season_transition.download_id" json:"-"`
	DownloadClientID int32       `alias:"season_transition.download_client_id" json:"-"`
	State            SeasonState `alias:"season_transition.to_state" json:"state"`
}

type SeasonTransition model.SeasonTransition

func (s Season) Machine() *machine.StateMachine[SeasonState] {
	return machine.New(s.State,
		machine.From(SeasonStateNew).To(SeasonStateUnreleased, SeasonStateMissing, SeasonStateDiscovered),
		machine.From(SeasonStateDiscovered).To(SeasonStateMissing, SeasonStateContinuing, SeasonStatePartial, SeasonStateCompleted),
		machine.From(SeasonStateMissing).To(SeasonStateDiscovered, SeasonStateDownloading),
		machine.From(SeasonStateUnreleased).To(SeasonStateDiscovered, SeasonStateMissing),
		machine.From(SeasonStateDownloading).To(SeasonStateContinuing, SeasonStatePartial, SeasonStateCompleted),
		machine.From(SeasonStateContinuing).To(SeasonStatePartial, SeasonStateCompleted, SeasonStateMissing),
		machine.From(SeasonStatePartial).To(SeasonStateContinuing, SeasonStateCompleted, SeasonStateMissing),
		machine.From(SeasonStateCompleted).To(SeasonStateContinuing, SeasonStatePartial),
	)
}

type Episode struct {
	model.Episode
	State                  EpisodeState `alias:"episode_transition.to_state" json:"state"`
	DownloadID             string       `alias:"episode_transition.download_id" json:"-"`
	DownloadClientID       int32        `alias:"episode_transition.download_client_id" json:"-"`
	IsEntireSeasonDownload bool         `alias:"episode_transition.is_entire_season_download" json:"-"`
}

type EpisodeTransition model.EpisodeTransition

func (e Episode) Machine() *machine.StateMachine[EpisodeState] {
	return machine.New(e.State,
		machine.From(EpisodeStateNew).To(EpisodeStateUnreleased, EpisodeStateMissing, EpisodeStateDiscovered),
		machine.From(EpisodeStateDiscovered).To(EpisodeStateCompleted),
		machine.From(EpisodeStateMissing).To(EpisodeStateDiscovered, EpisodeStateDownloading, EpisodeStateUnreleased),
		machine.From(EpisodeStateUnreleased).To(EpisodeStateDiscovered, EpisodeStateMissing),
		machine.From(EpisodeStateDownloading).To(EpisodeStateDownloaded),
		machine.From(EpisodeStateDownloaded).To(EpisodeStateCompleted),
	)
}

type SeriesStorage interface {
	GetSeries(ctx context.Context, where sqlite.BoolExpression) (*Series, error)
	CreateSeries(ctx context.Context, series Series, initialState SeriesState) (int64, error)
	DeleteSeries(ctx context.Context, id int64) error
	ListSeries(ctx context.Context, where ...sqlite.BoolExpression) ([]*Series, error)
	UpdateSeriesState(ctx context.Context, id int64, state SeriesState, metadata *TransitionStateMetadata) error
	LinkSeriesMetadata(ctx context.Context, seriesID int64, metadataID int32) error
	UpdateSeries(ctx context.Context, series model.Series, where ...sqlite.BoolExpression) error

	GetSeason(ctx context.Context, where sqlite.BoolExpression) (*Season, error)
	CreateSeason(ctx context.Context, season Season, initialState SeasonState) (int64, error)
	DeleteSeason(ctx context.Context, id int64) error
	ListSeasons(ctx context.Context, where ...sqlite.BoolExpression) ([]*Season, error)
	UpdateSeasonState(ctx context.Context, id int64, season SeasonState, metadata *TransitionStateMetadata) error
	UpdateSeason(ctx context.Context, season model.Season, where ...sqlite.BoolExpression) error
	LinkSeasonMetadata(ctx context.Context, seasonID int64, metadataID int32) error

	GetEpisode(ctx context.Context, where sqlite.BoolExpression) (*Episode, error)
	GetEpisodeByEpisodeFileID(ctx context.Context, fileID int64) (*Episode, error)
	CreateEpisode(ctx context.Context, episode Episode, initialState EpisodeState) (int64, error)
	DeleteEpisode(ctx context.Context, id int64) error
	ListEpisodes(ctx context.Context, where ...sqlite.BoolExpression) ([]*Episode, error)
	UpdateEpisodeEpisodeFileID(ctx context.Context, id int64, fileID int64) error
	UpdateEpisodeState(ctx context.Context, id int64, state EpisodeState, metadata *TransitionStateMetadata) error
	UpdateEpisode(ctx context.Context, episode model.Episode, where ...sqlite.BoolExpression) error
	LinkEpisodeMetadata(ctx context.Context, episodeID int64, seasonID int32, episodeMetadataID int32) error

	GetEpisodeFile(ctx context.Context, id int32) (*model.EpisodeFile, error)
	GetEpisodeFiles(ctx context.Context, id int64) ([]*model.EpisodeFile, error)
	CreateEpisodeFile(ctx context.Context, episodeFile model.EpisodeFile) (int64, error)
	UpdateEpisodeFile(ctx context.Context, id int32, episodeFile model.EpisodeFile) error
	DeleteEpisodeFile(ctx context.Context, id int64) error
	ListEpisodeFiles(ctx context.Context) ([]*model.EpisodeFile, error)
}

type SeriesMetadataStorage interface {
	CreateSeriesMetadata(ctx context.Context, SeriesMeta model.SeriesMetadata) (int64, error)
	UpdateSeriesMetadata(ctx context.Context, metadata model.SeriesMetadata) error
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

// GetSchemas returns the SQL schema files as string slices
func GetSchemas() ([]string, error) {
	var schemas []string

	schemaSQL, err := schemaFiles.ReadFile("sqlite/schema/schema.sql")
	if err != nil {
		return nil, fmt.Errorf("failed to read schema.sql: %w", err)
	}
	schemas = append(schemas, string(schemaSQL))

	defaultsSQL, err := schemaFiles.ReadFile("sqlite/schema/defaults.sql")
	if err != nil {
		return nil, fmt.Errorf("failed to read defaults.sql: %w", err)
	}
	schemas = append(schemas, string(defaultsSQL))

	return schemas, nil
}
