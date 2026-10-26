package sqlite

import (
	"context"
	"database/sql"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

func (s *SQLite) CountTransitionsByDate(ctx context.Context, arg sqlcdb.CountTransitionsByDateParams) (int64, error) {
	return s.Queries.CountTransitionsByDate(ctx, arg)
}

func (s *SQLite) GetEntityTransitionsEpisode(ctx context.Context, entityID int64) ([]sqlcdb.GetEntityTransitionsEpisodeRow, error) {
	return s.Queries.GetEntityTransitionsEpisode(ctx, entityID)
}

func (s *SQLite) GetEntityTransitionsJob(ctx context.Context, entityID int64) ([]sqlcdb.GetEntityTransitionsJobRow, error) {
	return s.Queries.GetEntityTransitionsJob(ctx, entityID)
}

func (s *SQLite) GetEntityTransitionsMovie(ctx context.Context, entityID int64) ([]sqlcdb.GetEntityTransitionsMovieRow, error) {
	return s.Queries.GetEntityTransitionsMovie(ctx, entityID)
}

func (s *SQLite) GetEntityTransitionsSeason(ctx context.Context, entityID int64) ([]sqlcdb.GetEntityTransitionsSeasonRow, error) {
	return s.Queries.GetEntityTransitionsSeason(ctx, entityID)
}

func (s *SQLite) GetEntityTransitionsSeries(ctx context.Context, entityID int64) ([]sqlcdb.GetEntityTransitionsSeriesRow, error) {
	return s.Queries.GetEntityTransitionsSeries(ctx, entityID)
}

func (s *SQLite) GetEpisodeTransitionItems(ctx context.Context, arg sqlcdb.GetEpisodeTransitionItemsParams) ([]sqlcdb.GetEpisodeTransitionItemsRow, error) {
	return s.Queries.GetEpisodeTransitionItems(ctx, arg)
}

func (s *SQLite) GetJobTransitionItems(ctx context.Context, arg sqlcdb.GetJobTransitionItemsParams) ([]sqlcdb.GetJobTransitionItemsRow, error) {
	return s.Queries.GetJobTransitionItems(ctx, arg)
}

func (s *SQLite) GetJobTransitionsByDate(ctx context.Context, arg sqlcdb.GetJobTransitionsByDateParams) ([]sqlcdb.GetJobTransitionsByDateRow, error) {
	return s.Queries.GetJobTransitionsByDate(ctx, arg)
}

func (s *SQLite) GetMovieTransitionItems(ctx context.Context, arg sqlcdb.GetMovieTransitionItemsParams) ([]sqlcdb.GetMovieTransitionItemsRow, error) {
	return s.Queries.GetMovieTransitionItems(ctx, arg)
}

func (s *SQLite) GetMovieTransitionsByDate(ctx context.Context, arg sqlcdb.GetMovieTransitionsByDateParams) ([]sqlcdb.GetMovieTransitionsByDateRow, error) {
	return s.Queries.GetMovieTransitionsByDate(ctx, arg)
}

func (s *SQLite) GetSeasonTransitionItems(ctx context.Context, arg sqlcdb.GetSeasonTransitionItemsParams) ([]sqlcdb.GetSeasonTransitionItemsRow, error) {
	return s.Queries.GetSeasonTransitionItems(ctx, arg)
}

func (s *SQLite) GetSeriesTransitionsByDate(ctx context.Context, arg sqlcdb.GetSeriesTransitionsByDateParams) ([]sqlcdb.GetSeriesTransitionsByDateRow, error) {
	return s.Queries.GetSeriesTransitionsByDate(ctx, arg)
}

func (s *SQLite) ListDownloadingMovies(ctx context.Context) ([]sqlcdb.ListDownloadingMoviesRow, error) {
	return s.Queries.ListDownloadingMovies(ctx)
}

func (s *SQLite) ListDownloadingSeries(ctx context.Context) ([]sqlcdb.ListDownloadingSeriesRow, error) {
	return s.Queries.ListDownloadingSeries(ctx)
}

func (s *SQLite) ListErrorJobs(ctx context.Context, cutoffDatetime sql.NullTime) ([]sqlcdb.ListErrorJobsRow, error) {
	return s.Queries.ListErrorJobs(ctx, cutoffDatetime)
}

func (s *SQLite) ListRunningJobs(ctx context.Context) ([]sqlcdb.ListRunningJobsRow, error) {
	return s.Queries.ListRunningJobs(ctx)
}
