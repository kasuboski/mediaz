package sqlite

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActiveActivityPagesAndCounts(t *testing.T) {
	ctx := t.Context()
	s := initSqlite(t, ctx).(*SQLite)

	// Five movies with metadata; ties deliberately share a sort key. A sixth
	// downloading movie lacks metadata and must be excluded by both SQL queries.
	movieIDs := make([]int64, 0, 5)
	for i := 0; i < 5; i++ {
		meta, err := s.CreateMovieMetadata(ctx, sqlcdb.MovieMetadatum{TmdbID: int64(i + 1), Title: fmt.Sprintf("Movie %d", i+1)})
		require.NoError(t, err)
		id, err := s.CreateMovie(ctx, sqlcdb.Movie{QualityProfileID: 1}, storage.MovieStateMissing)
		require.NoError(t, err)
		require.NoError(t, s.LinkMovieMetadata(ctx, id, meta))
		require.NoError(t, s.UpdateMovieState(ctx, id, storage.MovieStateDownloading, nil))
		movieIDs = append(movieIDs, id)
	}
	missing, err := s.CreateMovie(ctx, sqlcdb.Movie{QualityProfileID: 1}, storage.MovieStateMissing)
	require.NoError(t, err)
	require.NoError(t, s.UpdateMovieState(ctx, missing, storage.MovieStateDownloading, nil))
	for i, id := range movieIDs {
		key := int64(100 - i/2)
		_, err := s.db.ExecContext(ctx, "UPDATE movie_transition SET sort_key=? WHERE movie_id=? AND most_recent=1", key, id)
		require.NoError(t, err)
	}

	// Three downloading seasons, including a series with NULL metadata.
	seasonIDs := make([]int64, 0, 3)
	for i := 0; i < 3; i++ {
		series, err := s.CreateSeries(ctx, sqlcdb.Series{QualityProfileID: 1}, storage.SeriesStateMissing)
		require.NoError(t, err)
		if i != 1 {
			meta, err := s.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{TmdbID: int64(20 + i), Title: fmt.Sprintf("Series %d", i)})
			require.NoError(t, err)
			require.NoError(t, s.LinkSeriesMetadata(ctx, series, meta))
		}
		season, err := s.CreateSeason(ctx, sqlcdb.Season{SeriesID: series, SeasonNumber: int64(i + 1)}, storage.SeasonStateMissing)
		require.NoError(t, err)
		require.NoError(t, s.UpdateSeasonState(ctx, season, storage.SeasonStateDownloading, nil))
		seasonIDs = append(seasonIDs, season)
		_, err = s.db.ExecContext(ctx, "UPDATE season_transition SET sort_key=7 WHERE season_id=? AND most_recent=1", season)
		require.NoError(t, err)
	}

	// Four running jobs and a non-running job, to span multiple pages and
	// verify the stable created_at/id tie break.
	jobIDs := make([]int64, 0, 4)
	for i := 0; i < 5; i++ {
		id, err := s.CreateJob(ctx, sqlcdb.Job{Type: fmt.Sprintf("job-%d", i)}, storage.JobStatePending)
		require.NoError(t, err)
		if i < 4 {
			require.NoError(t, s.UpdateJobState(ctx, id, storage.JobStateRunning, nil))
			jobIDs = append(jobIDs, id)
		} else {
			require.NoError(t, s.UpdateJobState(ctx, id, storage.JobStateRunning, nil))
			require.NoError(t, s.UpdateJobState(ctx, id, storage.JobStateDone, nil))
		}
	}
	// Equal creation timestamps force the ID tie-break to determine order.
	stamp := "2026-06-01 12:00:00"
	for _, id := range jobIDs {
		_, err := s.db.ExecContext(ctx, "UPDATE job SET created_at=? WHERE id=?", stamp, id)
		require.NoError(t, err)
	}

	t.Run("movies", func(t *testing.T) {
		total, err := s.CountDownloadingMovies(ctx)
		require.NoError(t, err)
		assert.EqualValues(t, 5, total)
		var all []sqlcdb.ListDownloadingMoviesPageRow
		for offset := int64(0); offset < total; offset += 2 {
			page, err := s.ListDownloadingMoviesPage(ctx, sqlcdb.ListDownloadingMoviesPageParams{Offs: offset, Lim: 2})
			require.NoError(t, err)
			all = append(all, page...)
		}
		assert.Len(t, all, int(total))
		assert.Equal(t, []int64{movieIDs[1], movieIDs[0], movieIDs[3], movieIDs[2], movieIDs[4]}, []int64{all[0].ID, all[1].ID, all[2].ID, all[3].ID, all[4].ID})
		for i, row := range all {
			assert.Equal(t, []string{"Movie 2", "Movie 1", "Movie 4", "Movie 3", "Movie 5"}[i], row.Title)
			assert.Equal(t, "downloading", row.ToState)
		}
		page, err := s.ListDownloadingMoviesPage(ctx, sqlcdb.ListDownloadingMoviesPageParams{Offs: total, Lim: 2})
		require.NoError(t, err)
		assert.Empty(t, page)
	})
	t.Run("series", func(t *testing.T) {
		total, err := s.CountDownloadingSeries(ctx)
		require.NoError(t, err)
		assert.EqualValues(t, 3, total)
		page, err := s.ListDownloadingSeriesPage(ctx, sqlcdb.ListDownloadingSeriesPageParams{Offs: 0, Lim: 2})
		require.NoError(t, err)
		assert.Len(t, page, 2)
		all, err := s.ListDownloadingSeriesPage(ctx, sqlcdb.ListDownloadingSeriesPageParams{Offs: 0, Lim: 10})
		require.NoError(t, err)
		assert.Len(t, all, int(total))
		assert.Equal(t, []int64{seasonIDs[2], seasonIDs[1], seasonIDs[0]}, []int64{all[0].ID, all[1].ID, all[2].ID})
		assert.True(t, all[1].Title.Valid == false || all[1].Title.String == "")
		out, err := s.ListDownloadingSeriesPage(ctx, sqlcdb.ListDownloadingSeriesPageParams{Offs: total, Lim: 2})
		require.NoError(t, err)
		assert.Empty(t, out)
	})
	t.Run("running jobs", func(t *testing.T) {
		total, err := s.CountRunningJobs(ctx)
		require.NoError(t, err)
		assert.EqualValues(t, 4, total)
		first, err := s.ListRunningJobsPage(ctx, sqlcdb.ListRunningJobsPageParams{Offs: 0, Lim: 2})
		require.NoError(t, err)
		second, err := s.ListRunningJobsPage(ctx, sqlcdb.ListRunningJobsPageParams{Offs: 2, Lim: 2})
		require.NoError(t, err)
		all := append(first, second...)
		assert.Len(t, all, int(total))
		assert.Equal(t, []int64{jobIDs[3], jobIDs[2], jobIDs[1], jobIDs[0]}, []int64{all[0].ID, all[1].ID, all[2].ID, all[3].ID})
		for i, row := range all {
			assert.Equal(t, fmt.Sprintf("job-%d", 3-i), row.Type)
			assert.Equal(t, "running", row.ToState)
		}
		out, err := s.ListRunningJobsPage(ctx, sqlcdb.ListRunningJobsPageParams{Offs: total, Lim: 2})
		require.NoError(t, err)
		assert.Empty(t, out)
	})
}

func TestFailureActivityPagesAndCountsFractionalCutoff(t *testing.T) {
	ctx := t.Context()
	s := initSqlite(t, ctx).(*SQLite)
	cutoff := time.Date(2026, 6, 1, 12, 0, 0, 250_000_000, time.UTC)
	jobs := []struct {
		name string
		ts   string
	}{
		{"same-second-after", "2026-06-01 12:00:00.750"},
		{"same-second-equal", "2026-06-01 12:00:00.250"},
		{"just-before", "2026-06-01 12:00:00.249"},
		{"next-second", "2026-06-01 12:00:01.000"},
		{"invalid", "not-a-date"},
		{"old-current-error", "2025-06-01 12:00:00.000"},
	}
	ids := make([]int64, len(jobs))
	for i, fixture := range jobs {
		id, err := s.CreateJob(ctx, sqlcdb.Job{Type: fixture.name}, storage.JobStatePending)
		require.NoError(t, err)
		require.NoError(t, s.UpdateJobState(ctx, id, storage.JobStateRunning, nil))
		msg := "failure"
		require.NoError(t, s.UpdateJobState(ctx, id, storage.JobStateError, &msg))
		ids[i] = id
		_, err = s.db.ExecContext(ctx, "UPDATE job_transition SET updated_at=? WHERE job_id=? AND most_recent=1", fixture.ts, id)
		require.NoError(t, err)
	}

	arg := func(offset, limit int64) sqlcdb.ListErrorJobsPageParams {
		return sqlcdb.ListErrorJobsPageParams{CutoffDatetime: cutoff.Format("2006-01-02 15:04:05.000000000"), Offs: offset, Lim: limit}
	}
	count, err := s.CountErrorJobs(ctx, sql.NullTime{Time: cutoff, Valid: true})
	require.NoError(t, err)
	page1, err := s.ListErrorJobsPage(ctx, arg(0, 2))
	require.NoError(t, err)
	page2, err := s.ListErrorJobsPage(ctx, arg(2, 2))
	require.NoError(t, err)
	all := append(page1, page2...)
	// Strictly after .250 includes .750 and the next second; the equal
	// timestamp and .249 fixture are intentionally excluded.
	assert.EqualValues(t, 2, count)
	assert.Len(t, all, int(count))
	require.Len(t, all, 2)
	assert.Equal(t, []string{"next-second", "same-second-after"}, []string{all[0].Type, all[1].Type})
	for _, row := range all {
		assert.Equal(t, "error", row.ToState)
	}
	out, err := s.ListErrorJobsPage(ctx, arg(count, 2))
	require.NoError(t, err)
	assert.Empty(t, out)
}
