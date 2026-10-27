// Package storage defines the persistence interfaces for mediaz plus the
// small set of genuine business wrappers that live above the generated sqlc
// layer (pkg/storage/sqlcdb): state-machine types, transition metadata,
// filters and domain errors. Presentation belongs to manager/server.
//
// All entity and row types come straight from sqlc (sqlcdb.Movie,
// sqlcdb.GetMovieByIDRow, ...) — IDs are int64 and nullables are sql.Null*
// end to end. int32/string conversions happen only at external SDK edges
// (TMDB/Prowlarr/download clients) in the caller packages.
package storage

import (
	"context"
	"errors"

	"github.com/kasuboski/mediaz/pkg/machine"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

// ErrNotFound is returned when a requested entity does not exist.
var ErrNotFound = errors.New("not found in storage")

// ErrJobAlreadyPending is returned when creating a pending job of a type that
// already has a pending job (DB-enforced by idx_job_type_pending).
var ErrJobAlreadyPending = errors.New("job of this type already pending")

// Storage is the root persistence interface implemented by pkg/storage/sqlite.
type Storage interface {
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
	CreateIndexer(ctx context.Context, indexer sqlcdb.Indexer) (int64, error)
	UpdateIndexer(ctx context.Context, id int64, indexer sqlcdb.Indexer) error
	DeleteIndexer(ctx context.Context, id int64) error
	ListIndexers(ctx context.Context) ([]sqlcdb.Indexer, error)
}

type IndexerSourceStorage interface {
	CreateIndexerSource(ctx context.Context, source sqlcdb.IndexerSource) (int64, error)
	GetIndexerSource(ctx context.Context, id int64) (sqlcdb.IndexerSource, error)
	ListIndexerSources(ctx context.Context) ([]sqlcdb.IndexerSource, error)
	ListEnabledIndexerSources(ctx context.Context) ([]sqlcdb.IndexerSource, error)
	UpdateIndexerSource(ctx context.Context, id int64, source sqlcdb.IndexerSource) error
	DeleteIndexerSource(ctx context.Context, id int64) error
	// DeleteIndexerSourceCascade deletes an indexer source and all its child
	// indexers in a single transaction. If any operation fails the transaction
	// is rolled back and no rows are removed.
	DeleteIndexerSourceCascade(ctx context.Context, id int64) error
}

// QualityStorage reads and writes generated profiles, definitions and joined rows.
type QualityStorage interface {
	CreateQualityProfile(ctx context.Context, profile sqlcdb.QualityProfile) (int64, error)
	GetQualityProfile(ctx context.Context, id int64) (sqlcdb.QualityProfile, error)
	ListQualityProfiles(ctx context.Context) ([]sqlcdb.QualityProfile, error)
	ListQualityProfilesByMediaType(ctx context.Context, mediaType string) ([]sqlcdb.QualityProfile, error)
	ListQualityProfileRows(ctx context.Context, profileID int64) ([]sqlcdb.ListQualityProfileRowsRow, error)
	UpdateQualityProfile(ctx context.Context, id int64, profile sqlcdb.QualityProfile) error
	DeleteQualityProfile(ctx context.Context, id int64) error

	CreateQualityProfileItem(ctx context.Context, item sqlcdb.QualityProfileItem) (int64, error)
	CreateQualityProfileItems(ctx context.Context, items []sqlcdb.QualityProfileItem) error
	DeleteQualityProfileItem(ctx context.Context, id int64) error
	DeleteQualityProfileItemsByProfileID(ctx context.Context, profileID int64) error
	GetQualityProfileItem(ctx context.Context, id int64) (sqlcdb.QualityProfileItem, error)
	ListQualityProfileItems(ctx context.Context) ([]sqlcdb.QualityProfileItem, error)

	CreateQualityDefinition(ctx context.Context, definition sqlcdb.QualityDefinition) (int64, error)
	GetQualityDefinition(ctx context.Context, id int64) (sqlcdb.QualityDefinition, error)
	ListQualityDefinitions(ctx context.Context) ([]sqlcdb.QualityDefinition, error)
	UpdateQualityDefinition(ctx context.Context, id int64, definition sqlcdb.QualityDefinition) error
	DeleteQualityDefinition(ctx context.Context, id int64) error
}

// MovieState enumerates the movie transition states. MovieStateNew ("") is
// the zero value of a movie before its first transition.
type MovieState string

const (
	MovieStateNew         MovieState = ""
	MovieStateMissing     MovieState = "missing"
	MovieStateDiscovered  MovieState = "discovered"
	MovieStateUnreleased  MovieState = "unreleased"
	MovieStateDownloading MovieState = "downloading"
	MovieStateDownloaded  MovieState = "downloaded"
)

// Machine returns the movie state machine rooted at s.
func (s MovieState) Machine() *machine.StateMachine[MovieState] {
	return machine.New(s,
		machine.From(MovieStateNew).To(MovieStateUnreleased, MovieStateMissing, MovieStateDiscovered),
		machine.From(MovieStateMissing).To(MovieStateDiscovered, MovieStateDownloading, MovieStateDownloaded),
		machine.From(MovieStateUnreleased).To(MovieStateDiscovered, MovieStateMissing),
		machine.From(MovieStateDownloading).To(MovieStateDownloaded),
	)
}

// TransitionStateMetadata carries optional download information attached to a
// transition write. Nil fields are stored as NULL.
type TransitionStateMetadata struct {
	DownloadID             *string
	DownloadClientID       *int64
	IsEntireSeasonDownload *bool // applicable only to episodes
}

// MovieStorage reads and writes movies, movie files and movie transitions.
// Entity reads return generated join rows (entity embedded + named transition
// columns) so callers get the movie and its current state in one round trip.
type MovieStorage interface {
	GetMovie(ctx context.Context, id int64) (sqlcdb.GetMovieByIDRow, error)
	GetMovieByMovieFileID(ctx context.Context, fileID int64) (sqlcdb.GetMovieByFileIDRow, error)
	GetMovieByPath(ctx context.Context, path string) (sqlcdb.GetMovieByPathRow, error)
	GetMovieByMetadataID(ctx context.Context, metadataID int64) (sqlcdb.GetMovieByMetadataIDRow, error)
	CreateMovie(ctx context.Context, movie sqlcdb.Movie, state MovieState) (int64, error)
	DeleteMovie(ctx context.Context, id int64) error
	ListMovies(ctx context.Context) ([]sqlcdb.ListMoviesRow, error)
	// ListMoviesByState filters movies by any combination of current states; a
	// zero-value MovieState (or no arguments) applies no state filter.
	ListMoviesByState(ctx context.Context, states ...MovieState) ([]sqlcdb.ListMoviesByStatesRow, error)
	UpdateMovieState(ctx context.Context, id int64, state MovieState, metadata *TransitionStateMetadata) error
	UpdateMovieMovieFileID(ctx context.Context, id int64, fileID int64) error
	UpdateMovieQualityProfile(ctx context.Context, id int64, qualityProfileID int64) error
	UpdateMovieByID(ctx context.Context, id int64, movie sqlcdb.Movie) error

	GetMovieFilesByMovieName(ctx context.Context, name string) ([]sqlcdb.MovieFile, error)
	CreateMovieFile(ctx context.Context, movieFile sqlcdb.MovieFile) (int64, error)
	DeleteMovieFile(ctx context.Context, id int64) error
	ListMovieFiles(ctx context.Context) ([]sqlcdb.MovieFile, error)
	LinkMovieMetadata(ctx context.Context, movieID int64, metadataID int64) error
}

type MovieMetadataStorage interface {
	CreateMovieMetadata(ctx context.Context, movieMeta sqlcdb.MovieMetadatum) (int64, error)
	UpdateMovieMetadata(ctx context.Context, metadata sqlcdb.MovieMetadatum) error
	DeleteMovieMetadata(ctx context.Context, id int64) error
	ListMovieMetadata(ctx context.Context) ([]sqlcdb.MovieMetadatum, error)
	GetMovieMetadataByID(ctx context.Context, id int64) (sqlcdb.MovieMetadatum, error)
	GetMovieMetadataByTmdbID(ctx context.Context, tmdbID int64) (sqlcdb.MovieMetadatum, error)
}

type DownloadClientStorage interface {
	CreateDownloadClient(ctx context.Context, client sqlcdb.DownloadClient) (int64, error)
	GetDownloadClient(ctx context.Context, id int64) (sqlcdb.DownloadClient, error)
	ListDownloadClients(ctx context.Context) ([]sqlcdb.DownloadClient, error)
	UpdateDownloadClient(ctx context.Context, id int64, client sqlcdb.DownloadClient) error
	DeleteDownloadClient(ctx context.Context, id int64) error
}

// JobState enumerates the job transition states.
type JobState string

const (
	JobStateNew       JobState = ""
	JobStatePending   JobState = "pending"
	JobStateRunning   JobState = "running"
	JobStateError     JobState = "error"
	JobStateDone      JobState = "done"
	JobStateCancelled JobState = "cancelled"
)

// Machine returns the job state machine rooted at s.
func (s JobState) Machine() *machine.StateMachine[JobState] {
	return machine.New(s,
		machine.From(JobStateNew).To(JobStatePending),
		machine.From(JobStatePending).To(JobStateRunning, JobStateError, JobStateCancelled),
		machine.From(JobStateRunning).To(JobStateError, JobStateDone, JobStateCancelled),
	)
}

// JobFilter constrains current job state. Zero value matches all jobs;
// implementations always constrain to the most recent transition.
type JobFilter struct {
	Type  *string
	State *string
}

type JobStorage interface {
	CreateJob(ctx context.Context, job sqlcdb.Job, initialState JobState) (int64, error)
	GetJob(ctx context.Context, id int64) (sqlcdb.GetJobByIDRow, error)
	CountJobs(ctx context.Context, filter JobFilter) (int, error)
	// ListJobs returns jobs ordered by created_at DESC with an id DESC
	// tie-break (stable pagination). limit 0 means unlimited.
	ListJobs(ctx context.Context, offset, limit int, filter JobFilter) ([]sqlcdb.ListJobsRow, error)
	UpdateJobState(ctx context.Context, id int64, state JobState, errorMsg *string) error
	DeleteJob(ctx context.Context, id int64) error
	DeleteJobsByIDs(ctx context.Context, ids []int64) (int64, error)
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
	SeriesStateCompleted   SeriesState = "completed"

	SeasonStateNew         SeasonState = ""
	SeasonStateMissing     SeasonState = "missing"
	SeasonStateDiscovered  SeasonState = "discovered"
	SeasonStateUnreleased  SeasonState = "unreleased"
	SeasonStateContinuing  SeasonState = "continuing"
	SeasonStateDownloading SeasonState = "downloading"
	SeasonStateCompleted   SeasonState = "completed"

	EpisodeStateNew         EpisodeState = ""
	EpisodeStateMissing     EpisodeState = "missing"
	EpisodeStateDiscovered  EpisodeState = "discovered"
	EpisodeStateUnreleased  EpisodeState = "unreleased"
	EpisodeStateDownloading EpisodeState = "downloading"
	EpisodeStateDownloaded  EpisodeState = "downloaded"
	EpisodeStateCompleted   EpisodeState = "completed"
)

// Machine returns the series state machine rooted at s.
func (s SeriesState) Machine() *machine.StateMachine[SeriesState] {
	return machine.New(s,
		machine.From(SeriesStateNew).To(SeriesStateUnreleased, SeriesStateMissing, SeriesStateDiscovered),
		machine.From(SeriesStateDiscovered).To(SeriesStateMissing, SeriesStateContinuing, SeriesStateCompleted),
		machine.From(SeriesStateMissing).To(SeriesStateDiscovered, SeriesStateDownloading),
		machine.From(SeriesStateUnreleased).To(SeriesStateDiscovered, SeriesStateMissing),
		machine.From(SeriesStateDownloading).To(SeriesStateContinuing, SeriesStateCompleted),
		machine.From(SeriesStateContinuing).To(SeriesStateCompleted, SeriesStateMissing),
		machine.From(SeriesStateCompleted).To(SeriesStateContinuing),
	)
}

// Machine returns the season state machine rooted at s.
func (s SeasonState) Machine() *machine.StateMachine[SeasonState] {
	return machine.New(s,
		machine.From(SeasonStateNew).To(SeasonStateUnreleased, SeasonStateMissing, SeasonStateDiscovered),
		machine.From(SeasonStateDiscovered).To(SeasonStateMissing, SeasonStateContinuing, SeasonStateCompleted),
		machine.From(SeasonStateMissing).To(SeasonStateDiscovered, SeasonStateDownloading),
		machine.From(SeasonStateUnreleased).To(SeasonStateDiscovered, SeasonStateMissing),
		machine.From(SeasonStateDownloading).To(SeasonStateContinuing, SeasonStateCompleted),
		machine.From(SeasonStateContinuing).To(SeasonStateCompleted, SeasonStateMissing),
		machine.From(SeasonStateCompleted).To(SeasonStateContinuing),
	)
}

// Machine returns the episode state machine rooted at s.
func (s EpisodeState) Machine() *machine.StateMachine[EpisodeState] {
	return machine.New(s,
		machine.From(EpisodeStateNew).To(EpisodeStateUnreleased, EpisodeStateMissing, EpisodeStateDiscovered),
		machine.From(EpisodeStateDiscovered).To(EpisodeStateCompleted),
		machine.From(EpisodeStateMissing).To(EpisodeStateDiscovered, EpisodeStateDownloading, EpisodeStateUnreleased),
		machine.From(EpisodeStateUnreleased).To(EpisodeStateDiscovered, EpisodeStateMissing),
		machine.From(EpisodeStateDownloading).To(EpisodeStateDownloaded),
		machine.From(EpisodeStateDownloaded).To(EpisodeStateCompleted),
	)
}

// SeriesFilter constrains series by current state. Zero value matches all.
// States accepts any number of states (no fixed-slot cap).
type SeriesFilter struct {
	States    []SeriesState
	Monitored *bool
}

// SeasonFilter constrains seasons. Zero value matches all.
type SeasonFilter struct {
	SeriesID *int64
	States   []SeasonState
}

// EpisodeFilter constrains episodes. Zero value matches all.
type EpisodeFilter struct {
	SeasonID *int64
	States   []EpisodeState
}

// SeriesStorage reads and writes series, seasons, episodes and their files.
// Entity reads return generated join rows (entity embedded + named transition
// columns). Seasons/episodes are ordered by (season/series id, number).
type SeriesStorage interface {
	GetSeriesByID(ctx context.Context, id int64) (sqlcdb.GetSeriesRow, error)
	GetSeriesByMetadataID(ctx context.Context, metadataID int64) (sqlcdb.GetSeriesRow, error)
	GetSeriesByPath(ctx context.Context, path string) (sqlcdb.GetSeriesRow, error)
	CreateSeries(ctx context.Context, series sqlcdb.Series, initialState SeriesState) (int64, error)
	DeleteSeries(ctx context.Context, id int64) error
	ListSeries(ctx context.Context, filter SeriesFilter) ([]sqlcdb.ListSeriesRow, error)
	UpdateSeriesState(ctx context.Context, id int64, state SeriesState, metadata *TransitionStateMetadata) error
	LinkSeriesMetadata(ctx context.Context, seriesID int64, metadataID int64) error
	UpdateSeriesByID(ctx context.Context, id int64, series sqlcdb.Series) error

	GetSeasonByID(ctx context.Context, id int64) (sqlcdb.GetSeasonByIDRow, error)
	GetSeasonBySeriesIDAndNumber(ctx context.Context, seriesID int64, seasonNumber int64) (sqlcdb.GetSeasonBySeriesIDAndNumberRow, error)
	CreateSeason(ctx context.Context, season sqlcdb.Season, initialState SeasonState) (int64, error)
	DeleteSeason(ctx context.Context, id int64) error
	ListSeasons(ctx context.Context, filter SeasonFilter) ([]sqlcdb.ListSeasonsRow, error)
	UpdateSeasonState(ctx context.Context, id int64, state SeasonState, metadata *TransitionStateMetadata) error
	LinkSeasonMetadata(ctx context.Context, seasonID int64, metadataID int64) error

	GetEpisodeByID(ctx context.Context, id int64) (sqlcdb.GetEpisodeByIDRow, error)
	GetEpisodeByEpisodeFileID(ctx context.Context, fileID int64) (sqlcdb.GetEpisodeByEpisodeFileIDRow, error)
	CreateEpisode(ctx context.Context, episode sqlcdb.Episode, initialState EpisodeState) (int64, error)
	DeleteEpisode(ctx context.Context, id int64) error
	ListEpisodes(ctx context.Context, filter EpisodeFilter) ([]sqlcdb.ListEpisodesRow, error)
	UpdateEpisodeEpisodeFileID(ctx context.Context, id int64, fileID int64) error
	UpdateEpisodeState(ctx context.Context, id int64, state EpisodeState, metadata *TransitionStateMetadata) error
	LinkEpisodeMetadata(ctx context.Context, episodeID int64, seasonID int64, episodeMetadataID int64) error

	GetEpisodeFile(ctx context.Context, id int64) (sqlcdb.EpisodeFile, error)
	GetEpisodeFileByID(ctx context.Context, id int64) ([]sqlcdb.EpisodeFile, error)
	CreateEpisodeFile(ctx context.Context, episodeFile sqlcdb.EpisodeFile) (int64, error)
	UpdateEpisodeFile(ctx context.Context, id int64, episodeFile sqlcdb.EpisodeFile) error
	DeleteEpisodeFile(ctx context.Context, id int64) error
	ListEpisodeFiles(ctx context.Context) ([]sqlcdb.EpisodeFile, error)
}

type SeriesMetadataStorage interface {
	CreateSeriesMetadata(ctx context.Context, SeriesMeta sqlcdb.SeriesMetadatum) (int64, error)
	UpdateSeriesMetadata(ctx context.Context, metadata sqlcdb.SeriesMetadatum) error
	DeleteSeriesMetadata(ctx context.Context, id int64) error
	ListSeriesMetadata(ctx context.Context) ([]sqlcdb.SeriesMetadatum, error)
	ListSeriesMetadataByStatus(ctx context.Context, status string) ([]sqlcdb.SeriesMetadatum, error)
	GetSeriesMetadataByID(ctx context.Context, id int64) (sqlcdb.SeriesMetadatum, error)
	GetSeriesMetadataByTmdbID(ctx context.Context, tmdbID int64) (sqlcdb.SeriesMetadatum, error)

	CreateSeasonMetadata(ctx context.Context, seasonMeta sqlcdb.SeasonMetadatum) (int64, error)
	DeleteSeasonMetadata(ctx context.Context, id int64) error
	ListSeasonMetadataBySeriesMetadataID(ctx context.Context, seriesMetadataID int64) ([]sqlcdb.SeasonMetadatum, error)
	GetSeasonMetadataByID(ctx context.Context, id int64) (sqlcdb.SeasonMetadatum, error)
	GetSeasonMetadataByTmdbID(ctx context.Context, tmdbID int64) (sqlcdb.SeasonMetadatum, error)

	CreateEpisodeMetadata(ctx context.Context, episodeMeta sqlcdb.EpisodeMetadatum) (int64, error)
	DeleteEpisodeMetadata(ctx context.Context, id int64) error
	ListEpisodeMetadataBySeasonMetadataID(ctx context.Context, seasonMetadataID int64) ([]sqlcdb.EpisodeMetadatum, error)
	ListEpisodeMetadataByIDs(ctx context.Context, ids []int64) ([]sqlcdb.EpisodeMetadatum, error)
	GetEpisodeMetadataByID(ctx context.Context, id int64) (sqlcdb.EpisodeMetadatum, error)
	GetEpisodeMetadataByTmdbID(ctx context.Context, tmdbID int64) (sqlcdb.EpisodeMetadatum, error)
	GetEpisodeMetadataBySeasonAndNumber(ctx context.Context, seasonMetadataID, number int64) (sqlcdb.EpisodeMetadatum, error)
}
