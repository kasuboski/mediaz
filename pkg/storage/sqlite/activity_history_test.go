package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// backdateMostRecentTransition stamps the most recent transition row of an
// entity with an explicit UTC timestamp. Only the latest (most_recent = 1)
// row is touched, so demoted history rows keep their original times.
func backdateMostRecentTransition(t *testing.T, store storage.Storage, ctx context.Context, table, entityColumn string, entityID int64, ts time.Time) {
	t.Helper()
	stamp := ts.UTC().Format(timestampFormat)
	res, err := store.(*SQLite).db.ExecContext(ctx,
		fmt.Sprintf("UPDATE %s SET created_at = ?, updated_at = ? WHERE %s = ? AND most_recent = 1", table, entityColumn),
		stamp, stamp, entityID)
	require.NoError(t, err)
	affected, err := res.RowsAffected()
	require.NoError(t, err)
	require.EqualValues(t, 1, affected, "expected to backdate exactly one most-recent transition row")
}

// backdateAllTransitions stamps every transition row of an entity — including
// demoted ones — with an explicit UTC timestamp, so demoted rows land inside
// the query window and exclusion depends on the most_recent = 1 filter rather
// than on their original creation time.
func backdateAllTransitions(t *testing.T, store storage.Storage, ctx context.Context, table, entityColumn string, entityID int64, ts time.Time) {
	t.Helper()
	stamp := ts.UTC().Format(timestampFormat)
	res, err := store.(*SQLite).db.ExecContext(ctx,
		fmt.Sprintf("UPDATE %s SET created_at = ?, updated_at = ? WHERE %s = ?", table, entityColumn),
		stamp, stamp, entityID)
	require.NoError(t, err)
	affected, err := res.RowsAffected()
	require.NoError(t, err)
	require.GreaterOrEqual(t, affected, int64(1), "expected to backdate at least one transition row")
}

// ============================================================================
// GetSeriesTransitionsByDate
// ============================================================================

func TestGetSeriesTransitionsByDate(t *testing.T) {
	t.Run("returns empty when no season transitions exist", func(t *testing.T) {
		ctx := t.Context()
		store := initSqlite(t, ctx)

		rows, err := store.GetSeriesTransitionsByDate(ctx, sqlcdb.GetSeriesTransitionsByDateParams{
			Start: "2000-01-01 00:00:00", End: "2000-01-02 00:00:00",
		})
		require.NoError(t, err)
		assert.Empty(t, rows)
	})

	t.Run("counts completed and downloading seasons per date using most recent transitions", func(t *testing.T) {
		ctx := t.Context()
		store := initSqlite(t, ctx)

		_, err := store.CreateSeries(ctx, sqlcdb.Series{QualityProfileID: 1, Monitored: 1}, storage.SeriesStateMissing)
		require.NoError(t, err)

		season1, err := store.CreateSeason(ctx, sqlcdb.Season{SeriesID: 1, SeasonNumber: 1}, storage.SeasonStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.UpdateSeasonState(ctx, season1, storage.SeasonStateDownloading, nil))
		require.NoError(t, store.UpdateSeasonState(ctx, season1, storage.SeasonStateCompleted, nil))

		season2, err := store.CreateSeason(ctx, sqlcdb.Season{SeriesID: 1, SeasonNumber: 2}, storage.SeasonStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.UpdateSeasonState(ctx, season2, storage.SeasonStateDownloading, nil))

		// Stamp every transition of both seasons into the window: the expected
		// bucket must not depend on when the fixtures were created, and
		// season1's demoted "downloading" row is excluded only by the
		// most_recent = 1 filter, never by falling outside the window.
		day := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
		backdateAllTransitions(t, store, ctx, "season_transition", "season_id", season1, day)
		backdateAllTransitions(t, store, ctx, "season_transition", "season_id", season2, day)

		rows, err := store.GetSeriesTransitionsByDate(ctx, sqlcdb.GetSeriesTransitionsByDateParams{
			Start: day.Add(-time.Hour).Format(timestampFormat), End: day.Add(time.Hour).Format(timestampFormat),
		})
		require.NoError(t, err)
		// season1's demoted "downloading" row is inside the window; only the
		// most_recent = 1 filter keeps it from counting.
		assert.Equal(t, []sqlcdb.GetSeriesTransitionsByDateRow{{
			Date: day.Format("2006-01-02"), Completed: 1, Downloading: 1,
		}}, rows)
	})

	t.Run("buckets out-of-window transitions on their own date", func(t *testing.T) {
		ctx := t.Context()
		store := initSqlite(t, ctx)

		_, err := store.CreateSeries(ctx, sqlcdb.Series{QualityProfileID: 1, Monitored: 1}, storage.SeriesStateMissing)
		require.NoError(t, err)

		season1, err := store.CreateSeason(ctx, sqlcdb.Season{SeriesID: 1, SeasonNumber: 1}, storage.SeasonStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.UpdateSeasonState(ctx, season1, storage.SeasonStateDownloading, nil))
		require.NoError(t, store.UpdateSeasonState(ctx, season1, storage.SeasonStateCompleted, nil))

		season2, err := store.CreateSeason(ctx, sqlcdb.Season{SeriesID: 1, SeasonNumber: 2}, storage.SeasonStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.UpdateSeasonState(ctx, season2, storage.SeasonStateDownloading, nil))

		// Stamp season1's demoted "downloading" row into the wide window along
		// with its counted "completed" row: only the most_recent = 1 filter —
		// not the window — excludes it.
		old := time.Date(2024, 6, 5, 12, 0, 0, 0, time.UTC)
		backdateAllTransitions(t, store, ctx, "season_transition", "season_id", season1, old)
		recent := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
		backdateMostRecentTransition(t, store, ctx, "season_transition", "season_id", season2, recent)

		// season1's backdated "completed" row falls outside the recent window;
		// only season2 remains.
		recentRows, err := store.GetSeriesTransitionsByDate(ctx, sqlcdb.GetSeriesTransitionsByDateParams{
			Start: recent.Add(-time.Hour).Format(timestampFormat), End: recent.Add(time.Hour).Format(timestampFormat),
		})
		require.NoError(t, err)
		assert.Equal(t, []sqlcdb.GetSeriesTransitionsByDateRow{
			{Date: recent.Format("2006-01-02"), Downloading: 1},
		}, recentRows)

		wide, err := store.GetSeriesTransitionsByDate(ctx, sqlcdb.GetSeriesTransitionsByDateParams{
			Start: old.AddDate(0, 0, -1).Format(timestampFormat), End: recent.Add(time.Hour).Format(timestampFormat),
		})
		require.NoError(t, err)
		assert.Equal(t, []sqlcdb.GetSeriesTransitionsByDateRow{
			{Date: old.Format("2006-01-02"), Completed: 1},
			{Date: recent.Format("2006-01-02"), Downloading: 1},
		}, wide)
	})
}

// ============================================================================
// GetJobTransitionsByDate
// ============================================================================

func TestGetJobTransitionsByDate(t *testing.T) {
	t.Run("returns empty when no job transitions exist", func(t *testing.T) {
		ctx := t.Context()
		store := initSqlite(t, ctx)

		rows, err := store.GetJobTransitionsByDate(ctx, sqlcdb.GetJobTransitionsByDateParams{
			Start: "2000-01-01 00:00:00", End: "2000-01-02 00:00:00",
		})
		require.NoError(t, err)
		assert.Empty(t, rows)
	})

	t.Run("counts done and error jobs per date using most recent transitions", func(t *testing.T) {
		ctx := t.Context()
		store := initSqlite(t, ctx)

		job1, err := store.CreateJob(ctx, sqlcdb.Job{Type: "rss_sync"}, storage.JobStatePending)
		require.NoError(t, err)
		require.NoError(t, store.UpdateJobState(ctx, job1, storage.JobStateRunning, nil))
		require.NoError(t, store.UpdateJobState(ctx, job1, storage.JobStateDone, nil))

		job2, err := store.CreateJob(ctx, sqlcdb.Job{Type: "series_search"}, storage.JobStatePending)
		require.NoError(t, err)
		require.NoError(t, store.UpdateJobState(ctx, job2, storage.JobStateRunning, nil))
		errMsg := "boom"
		require.NoError(t, store.UpdateJobState(ctx, job2, storage.JobStateError, &errMsg))

		// Stamp every transition of both jobs into the window: the expected
		// bucket must not depend on when the fixtures were created, and the
		// demoted "running" rows are excluded only by the most_recent = 1
		// filter, never by falling outside the window.
		day := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
		backdateAllTransitions(t, store, ctx, "job_transition", "job_id", job1, day)
		backdateAllTransitions(t, store, ctx, "job_transition", "job_id", job2, day)

		rows, err := store.GetJobTransitionsByDate(ctx, sqlcdb.GetJobTransitionsByDateParams{
			Start: day.Add(-time.Hour).Format(timestampFormat), End: day.Add(time.Hour).Format(timestampFormat),
		})
		require.NoError(t, err)
		assert.Equal(t, []sqlcdb.GetJobTransitionsByDateRow{{
			Date: day.Format("2006-01-02"), Done: 1, Error: 1,
		}}, rows)
	})

	t.Run("buckets out-of-window transitions on their own date", func(t *testing.T) {
		ctx := t.Context()
		store := initSqlite(t, ctx)

		job1, err := store.CreateJob(ctx, sqlcdb.Job{Type: "rss_sync"}, storage.JobStatePending)
		require.NoError(t, err)
		require.NoError(t, store.UpdateJobState(ctx, job1, storage.JobStateRunning, nil))
		require.NoError(t, store.UpdateJobState(ctx, job1, storage.JobStateDone, nil))

		job2, err := store.CreateJob(ctx, sqlcdb.Job{Type: "series_search"}, storage.JobStatePending)
		require.NoError(t, err)
		errMsg := "boom"
		require.NoError(t, store.UpdateJobState(ctx, job2, storage.JobStateError, &errMsg))

		// Stamp job1's demoted "running" row into the wide window along with
		// its counted "done" row: only the most_recent = 1 filter — not the
		// window — excludes it. (job2 has a single "error" transition, so its
		// only row is the most recent one.)
		old := time.Date(2024, 6, 5, 12, 0, 0, 0, time.UTC)
		backdateMostRecentTransition(t, store, ctx, "job_transition", "job_id", job2, old)
		recent := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
		backdateAllTransitions(t, store, ctx, "job_transition", "job_id", job1, recent)

		recentRows, err := store.GetJobTransitionsByDate(ctx, sqlcdb.GetJobTransitionsByDateParams{
			Start: recent.Add(-time.Hour).Format(timestampFormat), End: recent.Add(time.Hour).Format(timestampFormat),
		})
		require.NoError(t, err)
		assert.Equal(t, []sqlcdb.GetJobTransitionsByDateRow{
			{Date: recent.Format("2006-01-02"), Done: 1},
		}, recentRows)

		wide, err := store.GetJobTransitionsByDate(ctx, sqlcdb.GetJobTransitionsByDateParams{
			Start: old.AddDate(0, 0, -1).Format(timestampFormat), End: recent.Add(time.Hour).Format(timestampFormat),
		})
		require.NoError(t, err)
		assert.Equal(t, []sqlcdb.GetJobTransitionsByDateRow{
			{Date: old.Format("2006-01-02"), Error: 1},
			{Date: recent.Format("2006-01-02"), Done: 1},
		}, wide)
	})
}

// ============================================================================
// GetSeasonTransitionItems
// ============================================================================

func TestGetSeasonTransitionItems(t *testing.T) {
	t.Run("returns most recent season transitions joined with series titles", func(t *testing.T) {
		ctx := t.Context()
		store := initSqlite(t, ctx)

		// Series without metadata yields an empty (COALESCE'd) title.
		_, err := store.CreateSeries(ctx, sqlcdb.Series{QualityProfileID: 1, Monitored: 1}, storage.SeriesStateMissing)
		require.NoError(t, err)
		untitledSeason, err := store.CreateSeason(ctx, sqlcdb.Season{SeriesID: 1, SeasonNumber: 1}, storage.SeasonStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.UpdateSeasonState(ctx, untitledSeason, storage.SeasonStateDownloading, nil))

		// Series with metadata contributes its title; a second transition on the
		// same season must collapse to the single most recent row.
		_, err = store.CreateSeries(ctx, sqlcdb.Series{QualityProfileID: 1, Monitored: 1}, storage.SeriesStateMissing)
		require.NoError(t, err)
		_, err = store.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{TmdbID: 1, Title: "Meta Series"})
		require.NoError(t, err)
		require.NoError(t, store.LinkSeriesMetadata(ctx, 2, 1))
		metaSeason, err := store.CreateSeason(ctx, sqlcdb.Season{SeriesID: 2, SeasonNumber: 1}, storage.SeasonStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.UpdateSeasonState(ctx, metaSeason, storage.SeasonStateDownloading, nil))
		require.NoError(t, store.UpdateSeasonState(ctx, metaSeason, storage.SeasonStateCompleted, nil))

		now := time.Now().UTC()
		items, err := store.GetSeasonTransitionItems(ctx, sqlcdb.GetSeasonTransitionItemsParams{
			Start: now.Add(-time.Hour).Format(timestampFormat), End: now.Add(time.Hour).Format(timestampFormat),
		})
		require.NoError(t, err)
		require.Len(t, items, 2)
		require.True(t, items[0].CreatedAt.Valid)
		require.True(t, items[1].CreatedAt.Valid)
		// Same-second stamps are ordered by transition id descending, which
		// matches insertion order here; a second boundary keeps this order too
		// because created_at DESC is the primary sort.
		assert.Equal(t, []sqlcdb.GetSeasonTransitionItemsRow{
			{
				ID: items[0].ID, EntityType: "season", EntityID: metaSeason, ToState: "completed",
				FromState: sql.NullString{String: "downloading", Valid: true}, CreatedAt: items[0].CreatedAt,
				EntityTitle: "Meta Series",
			},
			{
				ID: items[1].ID, EntityType: "season", EntityID: untitledSeason, ToState: "downloading",
				FromState: sql.NullString{String: "missing", Valid: true}, CreatedAt: items[1].CreatedAt,
			},
		}, items)
	})

	t.Run("window filter and pagination apply", func(t *testing.T) {
		ctx := t.Context()
		store := initSqlite(t, ctx)

		_, err := store.CreateSeries(ctx, sqlcdb.Series{QualityProfileID: 1, Monitored: 1}, storage.SeriesStateMissing)
		require.NoError(t, err)
		season1, err := store.CreateSeason(ctx, sqlcdb.Season{SeriesID: 1, SeasonNumber: 1}, storage.SeasonStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.UpdateSeasonState(ctx, season1, storage.SeasonStateDownloading, nil))

		_, err = store.CreateSeries(ctx, sqlcdb.Series{QualityProfileID: 1, Monitored: 1}, storage.SeriesStateMissing)
		require.NoError(t, err)
		season2, err := store.CreateSeason(ctx, sqlcdb.Season{SeriesID: 2, SeasonNumber: 2}, storage.SeasonStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.UpdateSeasonState(ctx, season2, storage.SeasonStateDownloading, nil))

		old := time.Now().UTC().AddDate(0, 0, -3)
		backdateMostRecentTransition(t, store, ctx, "season_transition", "season_id", season1, old)

		now := time.Now().UTC()
		recent, err := store.GetSeasonTransitionItems(ctx, sqlcdb.GetSeasonTransitionItemsParams{
			Start: now.Add(-time.Hour).Format(timestampFormat), End: now.Add(time.Hour).Format(timestampFormat),
		})
		require.NoError(t, err)
		require.Len(t, recent, 1)
		assert.EqualValues(t, season2, recent[0].EntityID)

		// lim = 0 means unlimited; a nonzero lim caps the page.
		wide, err := store.GetSeasonTransitionItems(ctx, sqlcdb.GetSeasonTransitionItemsParams{
			Start: old.AddDate(0, 0, -1).Format(timestampFormat), End: now.Add(time.Hour).Format(timestampFormat),
			Lim: 1,
		})
		require.NoError(t, err)
		require.Len(t, wide, 1)
		assert.EqualValues(t, season2, wide[0].EntityID, "newest transition comes first")

		offs, err := store.GetSeasonTransitionItems(ctx, sqlcdb.GetSeasonTransitionItemsParams{
			Start: old.AddDate(0, 0, -1).Format(timestampFormat), End: now.Add(time.Hour).Format(timestampFormat),
			Offs: 1,
		})
		require.NoError(t, err)
		require.Len(t, offs, 1)
		assert.EqualValues(t, season1, offs[0].EntityID, "offset skips the newest transition")
	})
}

// ============================================================================
// GetEpisodeTransitionItems
// ============================================================================

func TestGetEpisodeTransitionItems(t *testing.T) {
	t.Run("returns most recent episode transitions joined with series titles", func(t *testing.T) {
		ctx := t.Context()
		store := initSqlite(t, ctx)

		_, err := store.CreateSeries(ctx, sqlcdb.Series{QualityProfileID: 1, Monitored: 1}, storage.SeriesStateMissing)
		require.NoError(t, err)
		_, err = store.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{TmdbID: 1, Title: "Episode Series"})
		require.NoError(t, err)
		require.NoError(t, store.LinkSeriesMetadata(ctx, 1, 1))
		_, err = store.CreateSeason(ctx, sqlcdb.Season{SeriesID: 1, SeasonNumber: 1}, storage.SeasonStateMissing)
		require.NoError(t, err)

		episode1, err := store.CreateEpisode(ctx, sqlcdb.Episode{SeasonID: 1, EpisodeNumber: 1, Monitored: 1}, storage.EpisodeStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.UpdateEpisodeState(ctx, episode1, storage.EpisodeStateDownloading, nil))
		require.NoError(t, store.UpdateEpisodeState(ctx, episode1, storage.EpisodeStateDownloaded, nil))

		episode2, err := store.CreateEpisode(ctx, sqlcdb.Episode{SeasonID: 1, EpisodeNumber: 2, Monitored: 1}, storage.EpisodeStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.UpdateEpisodeState(ctx, episode2, storage.EpisodeStateDownloading, nil))

		now := time.Now().UTC()
		items, err := store.GetEpisodeTransitionItems(ctx, sqlcdb.GetEpisodeTransitionItemsParams{
			Start: now.Add(-time.Hour).Format(timestampFormat), End: now.Add(time.Hour).Format(timestampFormat),
		})
		require.NoError(t, err)
		require.Len(t, items, 2)
		require.True(t, items[0].CreatedAt.Valid)
		require.True(t, items[1].CreatedAt.Valid)
		assert.Equal(t, []sqlcdb.GetEpisodeTransitionItemsRow{
			{
				ID: items[0].ID, EntityType: "episode", EntityID: episode2, ToState: "downloading",
				FromState: sql.NullString{String: "missing", Valid: true}, CreatedAt: items[0].CreatedAt,
				EntityTitle: "Episode Series",
			},
			{
				ID: items[1].ID, EntityType: "episode", EntityID: episode1, ToState: "downloaded",
				FromState: sql.NullString{String: "downloading", Valid: true}, CreatedAt: items[1].CreatedAt,
				EntityTitle: "Episode Series",
			},
		}, items)
	})
}

// ============================================================================
// GetJobTransitionItems
// ============================================================================

func TestGetJobTransitionItems(t *testing.T) {
	t.Run("returns only the most recent transition per job ordered newest first", func(t *testing.T) {
		ctx := t.Context()
		store := initSqlite(t, ctx)

		job1, err := store.CreateJob(ctx, sqlcdb.Job{Type: "rss_sync"}, storage.JobStatePending)
		require.NoError(t, err)
		require.NoError(t, store.UpdateJobState(ctx, job1, storage.JobStateRunning, nil))
		require.NoError(t, store.UpdateJobState(ctx, job1, storage.JobStateDone, nil))

		job2, err := store.CreateJob(ctx, sqlcdb.Job{Type: "series_search"}, storage.JobStatePending)
		require.NoError(t, err)
		require.NoError(t, store.UpdateJobState(ctx, job2, storage.JobStateRunning, nil))
		errMsg := "boom"
		require.NoError(t, store.UpdateJobState(ctx, job2, storage.JobStateError, &errMsg))

		now := time.Now().UTC()
		items, err := store.GetJobTransitionItems(ctx, sqlcdb.GetJobTransitionItemsParams{
			Start: now.Add(-time.Hour).Format(timestampFormat), End: now.Add(time.Hour).Format(timestampFormat),
		})
		require.NoError(t, err)
		require.Len(t, items, 2)
		require.True(t, items[0].CreatedAt.Valid)
		require.True(t, items[1].CreatedAt.Valid)
		assert.Equal(t, []sqlcdb.GetJobTransitionItemsRow{
			{
				ID: items[0].ID, EntityType: "job", EntityID: job2, EntityTitle: "series_search", ToState: "error",
				FromState: sql.NullString{String: "running", Valid: true}, CreatedAt: items[0].CreatedAt,
			},
			{
				ID: items[1].ID, EntityType: "job", EntityID: job1, EntityTitle: "rss_sync", ToState: "done",
				FromState: sql.NullString{String: "running", Valid: true}, CreatedAt: items[1].CreatedAt,
			},
		}, items)
	})

	t.Run("window filter excludes backdated transitions", func(t *testing.T) {
		ctx := t.Context()
		store := initSqlite(t, ctx)

		job1, err := store.CreateJob(ctx, sqlcdb.Job{Type: "rss_sync"}, storage.JobStatePending)
		require.NoError(t, err)
		require.NoError(t, store.UpdateJobState(ctx, job1, storage.JobStateRunning, nil))
		require.NoError(t, store.UpdateJobState(ctx, job1, storage.JobStateDone, nil))

		job2, err := store.CreateJob(ctx, sqlcdb.Job{Type: "series_search"}, storage.JobStatePending)
		require.NoError(t, err)
		errMsg := "boom"
		require.NoError(t, store.UpdateJobState(ctx, job2, storage.JobStateError, &errMsg))

		old := time.Now().UTC().AddDate(0, 0, -2)
		backdateMostRecentTransition(t, store, ctx, "job_transition", "job_id", job2, old)

		now := time.Now().UTC()
		recent, err := store.GetJobTransitionItems(ctx, sqlcdb.GetJobTransitionItemsParams{
			Start: now.Add(-time.Hour).Format(timestampFormat), End: now.Add(time.Hour).Format(timestampFormat),
		})
		require.NoError(t, err)
		require.Len(t, recent, 1)
		assert.EqualValues(t, job1, recent[0].EntityID)
	})
}

// ============================================================================
// GetEntityTransitions*
// ============================================================================

func TestGetEntityTransitionsSeries(t *testing.T) {
	t.Run("returns full transition history joined with series metadata", func(t *testing.T) {
		ctx := t.Context()
		store := initSqlite(t, ctx)

		seriesID, err := store.CreateSeries(ctx, sqlcdb.Series{QualityProfileID: 1, Monitored: 1}, storage.SeriesStateMissing)
		require.NoError(t, err)
		_, err = store.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{
			TmdbID: 1, Title: "History Series", PosterPath: sql.NullString{String: "/history.jpg", Valid: true},
		})
		require.NoError(t, err)
		require.NoError(t, store.LinkSeriesMetadata(ctx, seriesID, 1))
		require.NoError(t, store.UpdateSeriesState(ctx, seriesID, storage.SeriesStateDownloading, nil))
		require.NoError(t, store.UpdateSeriesState(ctx, seriesID, storage.SeriesStateCompleted, nil))

		rows, err := store.GetEntityTransitionsSeries(ctx, seriesID)
		require.NoError(t, err)
		require.Len(t, rows, 3)
		for i := range rows {
			require.True(t, rows[i].CreatedAt.Valid)
		}
		assert.Equal(t, []sqlcdb.GetEntityTransitionsSeriesRow{
			{
				EntityTitle: sql.NullString{String: "History Series", Valid: true},
				PosterPath:  sql.NullString{String: "/history.jpg", Valid: true},
				ToState:     "missing", SortKey: 1, CreatedAt: rows[0].CreatedAt,
			},
			{
				EntityTitle: sql.NullString{String: "History Series", Valid: true},
				PosterPath:  sql.NullString{String: "/history.jpg", Valid: true},
				ToState:     "downloading", FromState: sql.NullString{String: "missing", Valid: true},
				SortKey: 2, CreatedAt: rows[1].CreatedAt,
			},
			{
				EntityTitle: sql.NullString{String: "History Series", Valid: true},
				PosterPath:  sql.NullString{String: "/history.jpg", Valid: true},
				ToState:     "completed", FromState: sql.NullString{String: "downloading", Valid: true},
				SortKey: 3, CreatedAt: rows[2].CreatedAt,
			},
		}, rows)
	})

	t.Run("returns null metadata columns when the series has no metadata", func(t *testing.T) {
		ctx := t.Context()
		store := initSqlite(t, ctx)

		seriesID, err := store.CreateSeries(ctx, sqlcdb.Series{QualityProfileID: 1, Monitored: 1}, storage.SeriesStateMissing)
		require.NoError(t, err)

		rows, err := store.GetEntityTransitionsSeries(ctx, seriesID)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		assert.False(t, rows[0].EntityTitle.Valid)
		assert.False(t, rows[0].PosterPath.Valid)
		assert.Equal(t, "missing", rows[0].ToState)
	})

	t.Run("returns no rows for unknown series id", func(t *testing.T) {
		ctx := t.Context()
		store := initSqlite(t, ctx)

		rows, err := store.GetEntityTransitionsSeries(ctx, 999)
		require.NoError(t, err)
		assert.Empty(t, rows)
	})
}

func TestGetEntityTransitionsSeason(t *testing.T) {
	t.Run("includes download client metadata in transition rows", func(t *testing.T) {
		ctx := t.Context()
		store := initSqlite(t, ctx)

		_, err := store.CreateSeries(ctx, sqlcdb.Series{QualityProfileID: 1, Monitored: 1}, storage.SeriesStateMissing)
		require.NoError(t, err)
		_, err = store.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{TmdbID: 1, Title: "Season Series"})
		require.NoError(t, err)
		require.NoError(t, store.LinkSeriesMetadata(ctx, 1, 1))
		_, err = store.CreateDownloadClient(ctx, sqlcdb.DownloadClient{
			Type: "nzb", Implementation: "sabnzbd", Scheme: "http", Host: "localhost", Port: 8080,
		})
		require.NoError(t, err)

		seasonID, err := store.CreateSeason(ctx, sqlcdb.Season{SeriesID: 1, SeasonNumber: 1}, storage.SeasonStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.UpdateSeasonState(ctx, seasonID, storage.SeasonStateDownloading, &storage.TransitionStateMetadata{
			DownloadClientID: int64Ptr(1), DownloadID: stringPtr("dl-77"),
		}))

		rows, err := store.GetEntityTransitionsSeason(ctx, seasonID)
		require.NoError(t, err)
		require.Len(t, rows, 2)
		for i := range rows {
			require.True(t, rows[i].CreatedAt.Valid)
		}
		assert.Equal(t, []sqlcdb.GetEntityTransitionsSeasonRow{
			{
				EntityTitle: sql.NullString{String: "Season Series", Valid: true},
				ToState:     "missing", SortKey: 1, CreatedAt: rows[0].CreatedAt,
				Metadata: `{"downloadClient":{"id":null,"host":null,"port":null},"downloadID":null}`,
			},
			{
				EntityTitle: sql.NullString{String: "Season Series", Valid: true},
				ToState:     "downloading", FromState: sql.NullString{String: "missing", Valid: true},
				SortKey: 2, CreatedAt: rows[1].CreatedAt,
				Metadata: `{"downloadClient":{"id":1,"host":"localhost","port":8080},"downloadID":"dl-77"}`,
			},
		}, rows)
	})

	t.Run("returns null title when the parent series has no metadata", func(t *testing.T) {
		ctx := t.Context()
		store := initSqlite(t, ctx)

		_, err := store.CreateSeries(ctx, sqlcdb.Series{QualityProfileID: 1, Monitored: 1}, storage.SeriesStateMissing)
		require.NoError(t, err)
		seasonID, err := store.CreateSeason(ctx, sqlcdb.Season{SeriesID: 1, SeasonNumber: 1}, storage.SeasonStateMissing)
		require.NoError(t, err)

		rows, err := store.GetEntityTransitionsSeason(ctx, seasonID)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		assert.False(t, rows[0].EntityTitle.Valid)
	})
}

func TestGetEntityTransitionsEpisode(t *testing.T) {
	t.Run("returns transition history joined through season and series metadata", func(t *testing.T) {
		ctx := t.Context()
		store := initSqlite(t, ctx)

		_, err := store.CreateSeries(ctx, sqlcdb.Series{QualityProfileID: 1, Monitored: 1}, storage.SeriesStateMissing)
		require.NoError(t, err)
		_, err = store.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{
			TmdbID: 1, Title: "Episode History", PosterPath: sql.NullString{String: "/ep.jpg", Valid: true},
		})
		require.NoError(t, err)
		require.NoError(t, store.LinkSeriesMetadata(ctx, 1, 1))
		_, err = store.CreateSeason(ctx, sqlcdb.Season{SeriesID: 1, SeasonNumber: 1}, storage.SeasonStateMissing)
		require.NoError(t, err)

		episodeID, err := store.CreateEpisode(ctx, sqlcdb.Episode{SeasonID: 1, EpisodeNumber: 3, Monitored: 1}, storage.EpisodeStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.UpdateEpisodeState(ctx, episodeID, storage.EpisodeStateDownloading, nil))
		require.NoError(t, store.UpdateEpisodeState(ctx, episodeID, storage.EpisodeStateDownloaded, nil))

		rows, err := store.GetEntityTransitionsEpisode(ctx, episodeID)
		require.NoError(t, err)
		require.Len(t, rows, 3)
		for i := range rows {
			require.True(t, rows[i].CreatedAt.Valid)
		}
		assert.Equal(t, []sqlcdb.GetEntityTransitionsEpisodeRow{
			{
				EntityTitle: sql.NullString{String: "Episode History", Valid: true},
				PosterPath:  sql.NullString{String: "/ep.jpg", Valid: true},
				ToState:     "missing", SortKey: 1, CreatedAt: rows[0].CreatedAt,
			},
			{
				EntityTitle: sql.NullString{String: "Episode History", Valid: true},
				PosterPath:  sql.NullString{String: "/ep.jpg", Valid: true},
				ToState:     "downloading", FromState: sql.NullString{String: "missing", Valid: true},
				SortKey: 2, CreatedAt: rows[1].CreatedAt,
			},
			{
				EntityTitle: sql.NullString{String: "Episode History", Valid: true},
				PosterPath:  sql.NullString{String: "/ep.jpg", Valid: true},
				ToState:     "downloaded", FromState: sql.NullString{String: "downloading", Valid: true},
				SortKey: 3, CreatedAt: rows[2].CreatedAt,
			},
		}, rows)
	})
}

func TestGetEntityTransitionsJob(t *testing.T) {
	t.Run("returns full job transition history", func(t *testing.T) {
		ctx := t.Context()
		store := initSqlite(t, ctx)

		jobID, err := store.CreateJob(ctx, sqlcdb.Job{Type: "movie_search"}, storage.JobStatePending)
		require.NoError(t, err)
		require.NoError(t, store.UpdateJobState(ctx, jobID, storage.JobStateRunning, nil))
		errMsg := "indexer unreachable"
		require.NoError(t, store.UpdateJobState(ctx, jobID, storage.JobStateError, &errMsg))

		rows, err := store.GetEntityTransitionsJob(ctx, jobID)
		require.NoError(t, err)
		require.Len(t, rows, 3)
		for i := range rows {
			require.True(t, rows[i].CreatedAt.Valid)
		}
		assert.Equal(t, []sqlcdb.GetEntityTransitionsJobRow{
			{EntityTitle: "movie_search", ToState: "pending", SortKey: 1, CreatedAt: rows[0].CreatedAt},
			{EntityTitle: "movie_search", ToState: "running", FromState: sql.NullString{String: "pending", Valid: true}, SortKey: 2, CreatedAt: rows[1].CreatedAt},
			{EntityTitle: "movie_search", ToState: "error", FromState: sql.NullString{String: "running", Valid: true}, SortKey: 3, CreatedAt: rows[2].CreatedAt},
		}, rows)
	})

	t.Run("returns no rows for unknown job id", func(t *testing.T) {
		ctx := t.Context()
		store := initSqlite(t, ctx)

		rows, err := store.GetEntityTransitionsJob(ctx, 999)
		require.NoError(t, err)
		assert.Empty(t, rows)
	})
}
