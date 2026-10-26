package sqlite

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// ListDownloadingMovies
// ============================================================================

func TestListDownloadingMovies(t *testing.T) {
	t.Run("returns empty when no downloading movies", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)

		movies, err := store.ListDownloadingMovies(ctx)
		require.NoError(t, err)
		assert.Empty(t, movies)
	})

	t.Run("returns only movies in downloading state", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)

		_, err := store.CreateMovieMetadata(ctx, sqlcdb.MovieMetadatum{
			TmdbID: 123, Title: "Downloading Movie", Year: sql.NullInt64{Int64: 2024, Valid: true}, Images: `["poster.jpg"]`,
		})
		require.NoError(t, err)

		path1 := "path/missing"
		_, err = store.CreateMovie(ctx, sqlcdb.Movie{Path: sql.NullString{String: path1, Valid: true}, Monitored: 1, QualityProfileID: 1}, storage.MovieStateMissing)
		require.NoError(t, err)

		path2 := "path/downloading"
		movieID, err := store.CreateMovie(ctx, sqlcdb.Movie{Path: sql.NullString{String: path2, Valid: true}, Monitored: 1, QualityProfileID: 1}, storage.MovieStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.LinkMovieMetadata(ctx, movieID, 1))

		_, err = store.CreateDownloadClient(ctx, sqlcdb.DownloadClient{
			Type: "nzb", Implementation: "sabnzbd", Scheme: "http", Host: "localhost", Port: 8080,
		})
		require.NoError(t, err)

		require.NoError(t, store.UpdateMovieState(ctx, movieID, storage.MovieStateDownloading, &storage.TransitionStateMetadata{
			DownloadClientID: int64Ptr(1), DownloadID: stringPtr("dl-001"),
		}))

		movies, err := store.ListDownloadingMovies(ctx)
		require.NoError(t, err)
		require.Len(t, movies, 1)
		require.True(t, movies[0].CreatedAt.Valid)
		assert.WithinDuration(t, time.Now(), movies[0].CreatedAt.Time, 5*time.Second)

		assert.Equal(t, []sqlcdb.ListDownloadingMoviesRow{{
			ID: movieID, TmdbID: 123, Title: "Downloading Movie", Year: sql.NullInt64{Int64: 2024, Valid: true},
			Images: `["poster.jpg"]`, ToState: "downloading", SortKey: 2, CreatedAt: movies[0].CreatedAt,
			DownloadID: sql.NullString{String: "dl-001", Valid: true},
			DcID:       sql.NullInt64{Int64: 1, Valid: true}, DcHost: sql.NullString{String: "localhost", Valid: true}, DcPort: sql.NullInt64{Int64: 8080, Valid: true},
		}}, movies)
	})
}

// ============================================================================
// ListDownloadingSeries
// ============================================================================

func TestListDownloadingSeries(t *testing.T) {
	t.Run("returns empty when no downloading series", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)

		series, err := store.ListDownloadingSeries(ctx)
		require.NoError(t, err)
		assert.Empty(t, series)
	})

	t.Run("returns only seasons in downloading state", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)

		_, err := store.CreateSeries(ctx, sqlcdb.Series{Monitored: 1, QualityProfileID: 1}, storage.SeriesStateMissing)
		require.NoError(t, err)
		_, err = store.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{
			TmdbID: 456, Title: "Downloading Series", PosterPath: sql.NullString{String: "/poster.jpg", Valid: true},
		})
		require.NoError(t, err)
		require.NoError(t, store.LinkSeriesMetadata(ctx, 1, 1))

		seasonID, err := store.CreateSeason(ctx, sqlcdb.Season{SeriesID: 1, SeasonNumber: 1}, storage.SeasonStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.UpdateSeasonState(ctx, seasonID, storage.SeasonStateDownloading, &storage.TransitionStateMetadata{
			DownloadID: stringPtr("dl-002"),
		}))

		series, err := store.ListDownloadingSeries(ctx)
		require.NoError(t, err)
		require.Len(t, series, 1)
		require.True(t, series[0].CreatedAt.Valid)
		assert.WithinDuration(t, time.Now(), series[0].CreatedAt.Time, 5*time.Second)

		assert.Equal(t, []sqlcdb.ListDownloadingSeriesRow{{
			ID: seasonID, TmdbID: sql.NullInt64{Int64: 456, Valid: true}, Title: sql.NullString{String: "Downloading Series", Valid: true},
			PosterPath: sql.NullString{String: "/poster.jpg", Valid: true}, ToState: "downloading", SortKey: 2, CreatedAt: series[0].CreatedAt,
			DownloadID: sql.NullString{String: "dl-002", Valid: true}, SeasonNumber: 1,
		}}, series)
	})
}

// ============================================================================
// ListRunningJobs
// ============================================================================

func TestListRunningJobs(t *testing.T) {
	t.Run("returns empty when no running jobs", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)

		jobs, err := store.ListRunningJobs(ctx)
		require.NoError(t, err)
		assert.Empty(t, jobs)
	})

	t.Run("returns only running jobs", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)

		_, err := store.CreateJob(ctx, sqlcdb.Job{Type: "rss_sync"}, storage.JobStatePending)
		require.NoError(t, err)

		jobID, err := store.CreateJob(ctx, sqlcdb.Job{Type: "series_search"}, storage.JobStatePending)
		require.NoError(t, err)
		require.NoError(t, store.UpdateJobState(ctx, jobID, storage.JobStateRunning, nil))

		jobs, err := store.ListRunningJobs(ctx)
		require.NoError(t, err)
		require.Len(t, jobs, 1)

		require.True(t, jobs[0].CreatedAt.Valid)
		require.True(t, jobs[0].UpdatedAt.Valid)
		assert.Equal(t, []sqlcdb.ListRunningJobsRow{{ID: jobID, Type: "series_search", ToState: "running", CreatedAt: jobs[0].CreatedAt, UpdatedAt: jobs[0].UpdatedAt}}, jobs)
	})
}

// ============================================================================
// ListErrorJobs
// ============================================================================

func TestListErrorJobs(t *testing.T) {
	t.Run("returns empty when no error jobs", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)

		jobs, err := store.ListErrorJobs(ctx, sql.NullTime{Time: time.Now().UTC().Add(-24 * time.Hour), Valid: true})
		require.NoError(t, err)
		assert.Empty(t, jobs)
	})

	t.Run("returns error jobs within time window", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)

		jobID, err := store.CreateJob(ctx, sqlcdb.Job{Type: "rss_sync"}, storage.JobStatePending)
		require.NoError(t, err)
		require.NoError(t, store.UpdateJobState(ctx, jobID, storage.JobStateRunning, nil))
		errMsg := "something went wrong"
		require.NoError(t, store.UpdateJobState(ctx, jobID, storage.JobStateError, &errMsg))

		jobs, err := store.ListErrorJobs(ctx, sql.NullTime{Time: time.Now().UTC().Add(-24 * time.Hour), Valid: true})
		require.NoError(t, err)
		require.Len(t, jobs, 1)

		require.True(t, jobs[0].CreatedAt.Valid)
		require.True(t, jobs[0].UpdatedAt.Valid)
		assert.Equal(t, []sqlcdb.ListErrorJobsRow{{ID: jobID, Type: "rss_sync", ToState: "error", CreatedAt: jobs[0].CreatedAt, UpdatedAt: jobs[0].UpdatedAt}}, jobs)
	})
}

// ============================================================================
// CountTransitionsByDate
// ============================================================================

func TestCountTransitionsByDate(t *testing.T) {
	t.Run("returns zero when no transitions in range", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)

		count, err := store.CountTransitionsByDate(ctx, sqlcdb.CountTransitionsByDateParams{Start: "2020-01-01 00:00:00", End: "2020-01-02 00:00:00"})
		require.NoError(t, err)
		assert.Equal(t, int64(0), count)
	})

	t.Run("counts recent transitions", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)

		p := "path/1"
		_, err := store.CreateMovie(ctx, sqlcdb.Movie{Path: sql.NullString{String: p, Valid: true}, Monitored: 1, QualityProfileID: 1}, storage.MovieStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.UpdateMovieState(ctx, 1, storage.MovieStateDownloading, nil))

		_, err = store.CreateJob(ctx, sqlcdb.Job{Type: "rss_sync"}, storage.JobStatePending)
		require.NoError(t, err)
		require.NoError(t, store.UpdateJobState(ctx, 1, storage.JobStateRunning, nil))

		now := time.Now().UTC()
		count, err := store.CountTransitionsByDate(ctx, sqlcdb.CountTransitionsByDateParams{Start: now.Add(-time.Hour).Format(timestampFormat), End: now.Add(time.Hour).Format(timestampFormat)})
		require.NoError(t, err)
		assert.Equal(t, int64(2), count)
	})
}

func TestActivityGeneratedTransitionRows(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	id, err := store.CreateMovie(ctx, sqlcdb.Movie{Path: sql.NullString{String: "movie", Valid: true}, QualityProfileID: 1}, storage.MovieStateMissing)
	require.NoError(t, err)
	require.NoError(t, store.UpdateMovieState(ctx, id, storage.MovieStateDownloading, nil))
	history, err := store.GetEntityTransitionsMovie(ctx, id)
	require.NoError(t, err)
	require.Len(t, history, 2)
	for _, row := range history {
		require.True(t, row.CreatedAt.Valid)
	}
	assert.Equal(t, []sqlcdb.GetEntityTransitionsMovieRow{
		{ToState: "missing", SortKey: 1, CreatedAt: history[0].CreatedAt, Metadata: `{"downloadClient":{"id":null,"host":null,"port":null},"downloadID":null}`},
		{ToState: "downloading", FromState: sql.NullString{String: "missing", Valid: true}, SortKey: 2, CreatedAt: history[1].CreatedAt, Metadata: `{"downloadClient":{"id":null,"host":null,"port":null},"downloadID":null}`},
	}, history)
	now := time.Now().UTC()
	start, end := now.Add(-time.Hour).Format(timestampFormat), now.Add(time.Hour).Format(timestampFormat)
	items, err := store.GetMovieTransitionItems(ctx, sqlcdb.GetMovieTransitionItemsParams{Start: start, End: end})
	require.NoError(t, err)
	assert.Equal(t, []sqlcdb.GetMovieTransitionItemsRow{{ID: 2, EntityType: "movie", EntityID: id, ToState: "downloading", FromState: sql.NullString{String: "missing", Valid: true}, CreatedAt: history[1].CreatedAt}}, items)
	counts, err := store.GetMovieTransitionsByDate(ctx, sqlcdb.GetMovieTransitionsByDateParams{Start: start, End: end})
	require.NoError(t, err)
	assert.Equal(t, []sqlcdb.GetMovieTransitionsByDateRow{{Date: history[1].CreatedAt.Time.Format("2006-01-02"), Downloading: 1}}, counts)
	// Unlimited query pagination must still apply its offset.
	items, err = store.GetMovieTransitionItems(ctx, sqlcdb.GetMovieTransitionItemsParams{Start: start, End: end, Offs: 1})
	require.NoError(t, err)
	assert.Empty(t, items)
}

func int64Ptr(i int64) *int64 {
	return &i
}

func stringPtr(s string) *string {
	return &s
}
