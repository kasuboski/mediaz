package sqlite

import (
	"context"
	"database/sql"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

func (s *SQLite) ListDownloadingMoviesPage(c context.Context, a sqlcdb.ListDownloadingMoviesPageParams) ([]sqlcdb.ListDownloadingMoviesPageRow, error) {
	return s.Queries.ListDownloadingMoviesPage(c, a)
}
func (s *SQLite) CountDownloadingMovies(c context.Context) (int64, error) {
	return s.Queries.CountDownloadingMovies(c)
}
func (s *SQLite) ListDownloadingSeriesPage(c context.Context, a sqlcdb.ListDownloadingSeriesPageParams) ([]sqlcdb.ListDownloadingSeriesPageRow, error) {
	return s.Queries.ListDownloadingSeriesPage(c, a)
}
func (s *SQLite) CountDownloadingSeries(c context.Context) (int64, error) {
	return s.Queries.CountDownloadingSeries(c)
}
func (s *SQLite) ListRunningJobsPage(c context.Context, a sqlcdb.ListRunningJobsPageParams) ([]sqlcdb.ListRunningJobsPageRow, error) {
	return s.Queries.ListRunningJobsPage(c, a)
}
func (s *SQLite) CountRunningJobs(c context.Context) (int64, error) {
	return s.Queries.CountRunningJobs(c)
}
func (s *SQLite) ListErrorJobsPage(c context.Context, a sqlcdb.ListErrorJobsPageParams) ([]sqlcdb.ListErrorJobsPageRow, error) {
	return s.Queries.ListErrorJobsPage(c, a)
}
func (s *SQLite) CountErrorJobs(c context.Context, a sql.NullTime) (int64, error) {
	if !a.Valid {
		return s.Queries.CountErrorJobs(c, nil)
	}
	return s.Queries.CountErrorJobs(c, a.Time.UTC().Format("2006-01-02 15:04:05.000000000"))
}
func (s *SQLite) GetMovieHistoryPage(c context.Context, a sqlcdb.GetMovieHistoryPageParams) ([]sqlcdb.GetMovieHistoryPageRow, error) {
	return s.Queries.GetMovieHistoryPage(c, a)
}
func (s *SQLite) CountMovieHistory(c context.Context, id int64) (int64, error) {
	return s.Queries.CountMovieHistory(c, id)
}
func (s *SQLite) GetSeriesHistoryPage(c context.Context, a sqlcdb.GetSeriesHistoryPageParams) ([]sqlcdb.GetSeriesHistoryPageRow, error) {
	return s.Queries.GetSeriesHistoryPage(c, a)
}
func (s *SQLite) CountSeriesHistory(c context.Context, id int64) (int64, error) {
	return s.Queries.CountSeriesHistory(c, id)
}
func (s *SQLite) GetSeasonHistoryPage(c context.Context, a sqlcdb.GetSeasonHistoryPageParams) ([]sqlcdb.GetSeasonHistoryPageRow, error) {
	return s.Queries.GetSeasonHistoryPage(c, a)
}
func (s *SQLite) CountSeasonHistory(c context.Context, id int64) (int64, error) {
	return s.Queries.CountSeasonHistory(c, id)
}
func (s *SQLite) GetEpisodeHistoryPage(c context.Context, a sqlcdb.GetEpisodeHistoryPageParams) ([]sqlcdb.GetEpisodeHistoryPageRow, error) {
	return s.Queries.GetEpisodeHistoryPage(c, a)
}
func (s *SQLite) CountEpisodeHistory(c context.Context, id int64) (int64, error) {
	return s.Queries.CountEpisodeHistory(c, id)
}
func (s *SQLite) GetJobHistoryPage(c context.Context, a sqlcdb.GetJobHistoryPageParams) ([]sqlcdb.GetJobHistoryPageRow, error) {
	return s.Queries.GetJobHistoryPage(c, a)
}
func (s *SQLite) CountJobHistory(c context.Context, id int64) (int64, error) {
	return s.Queries.CountJobHistory(c, id)
}
