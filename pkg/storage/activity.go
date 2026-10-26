package storage

import (
	"context"
	"database/sql"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

// ActivityStorage exposes generated query results; aggregation belongs to manager.
type ActivityStorage interface {
	CountTransitionsByDate(ctx context.Context, arg sqlcdb.CountTransitionsByDateParams) (int64, error)
	GetEntityTransitionsEpisode(ctx context.Context, entityID int64) ([]sqlcdb.GetEntityTransitionsEpisodeRow, error)
	GetEntityTransitionsJob(ctx context.Context, entityID int64) ([]sqlcdb.GetEntityTransitionsJobRow, error)
	GetEntityTransitionsMovie(ctx context.Context, entityID int64) ([]sqlcdb.GetEntityTransitionsMovieRow, error)
	GetEntityTransitionsSeason(ctx context.Context, entityID int64) ([]sqlcdb.GetEntityTransitionsSeasonRow, error)
	GetEntityTransitionsSeries(ctx context.Context, entityID int64) ([]sqlcdb.GetEntityTransitionsSeriesRow, error)
	GetEpisodeTransitionItems(ctx context.Context, arg sqlcdb.GetEpisodeTransitionItemsParams) ([]sqlcdb.GetEpisodeTransitionItemsRow, error)
	GetJobTransitionItems(ctx context.Context, arg sqlcdb.GetJobTransitionItemsParams) ([]sqlcdb.GetJobTransitionItemsRow, error)
	GetJobTransitionsByDate(ctx context.Context, arg sqlcdb.GetJobTransitionsByDateParams) ([]sqlcdb.GetJobTransitionsByDateRow, error)
	GetMovieTransitionItems(ctx context.Context, arg sqlcdb.GetMovieTransitionItemsParams) ([]sqlcdb.GetMovieTransitionItemsRow, error)
	GetMovieTransitionsByDate(ctx context.Context, arg sqlcdb.GetMovieTransitionsByDateParams) ([]sqlcdb.GetMovieTransitionsByDateRow, error)
	GetSeasonTransitionItems(ctx context.Context, arg sqlcdb.GetSeasonTransitionItemsParams) ([]sqlcdb.GetSeasonTransitionItemsRow, error)
	GetSeriesTransitionsByDate(ctx context.Context, arg sqlcdb.GetSeriesTransitionsByDateParams) ([]sqlcdb.GetSeriesTransitionsByDateRow, error)
	ListDownloadingMovies(ctx context.Context) ([]sqlcdb.ListDownloadingMoviesRow, error)
	ListDownloadingSeries(ctx context.Context) ([]sqlcdb.ListDownloadingSeriesRow, error)
	ListErrorJobs(ctx context.Context, cutoffDatetime sql.NullTime) ([]sqlcdb.ListErrorJobsRow, error)
	ListRunningJobs(ctx context.Context) ([]sqlcdb.ListRunningJobsRow, error)
}
