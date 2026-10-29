package manager

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kasuboski/mediaz/pkg/pagination"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseRenderedDuration parses formatDuration output ("45s", "1m 5s", "2h 2m")
// back into a time.Duration.
func parseRenderedDuration(t *testing.T, rendered string) time.Duration {
	t.Helper()
	d, err := time.ParseDuration(strings.ReplaceAll(rendered, " ", ""))
	require.NoError(t, err, "duration %q must match formatDuration output", rendered)
	return d
}

// assertDurationWithin asserts that a freshly rendered duration corresponds to
// a real interval that began after the fixtures were started: nonnegative and
// no larger than the test's elapsed time (plus a one-second margin covering
// formatDuration's truncation to whole seconds). Pinning exact strings like
// "0s"/"1s" would flake whenever a scheduling pause stretches the fixture
// window past a second boundary.
func assertDurationWithin(t *testing.T, started time.Time, rendered string) {
	t.Helper()
	parsed := parseRenderedDuration(t, rendered)
	assert.GreaterOrEqual(t, parsed, time.Duration(0), "duration %q must be nonnegative", rendered)
	assert.LessOrEqual(t, parsed, time.Since(started)+time.Second, "duration %q must not predate the fixtures", rendered)
}

// newStampedActivityTestStorage returns an in-memory store plus a raw DB
// handle sharing one named shared-cache database. Stamping explicit
// transition timestamps needs a second connection, which a private
// ":memory:" database cannot serve. A "file:<name>?mode=memory&cache=shared"
// URI makes every connection opened with the same <name> join one in-memory
// database that vanishes when its last connection closes, and the unique
// per-test name keeps tests isolated from each other.
//
// The valueless trailing "_sentinel" parameter is required plumbing:
// sqlite.New appends its own "?_foreign_keys=on&..." parameters to the DSN,
// and that second "?" would otherwise attach to the cache value, which the
// SQLite URI parser rejects ("no such cache mode: shared?_foreign_keys=on").
// Because the sentinel swallows sqlite.New's appended foreign-key parameter,
// _foreign_keys=on is stated explicitly so pooled connections keep matching
// production DSN behavior.
func newStampedActivityTestStorage(t *testing.T, ctx context.Context) (storage.Storage, *sql.DB) {
	t.Helper()
	path := fmt.Sprintf("file:mediaz-activity-%s?mode=memory&cache=shared&_foreign_keys=on&_sentinel", uuid.NewString())
	store, err := sqlite.New(ctx, path)
	require.NoError(t, err)
	db, err := sql.Open("sqlite3", path+"&_busy_timeout=5000&_txlock=immediate")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	return store, db
}

// stampMostRecentTransition overwrites the timestamps of an entity's most
// recent transition row so tests can control rendered durations and date
// buckets exactly. Only the latest (most_recent = 1) row is touched, so
// demoted history rows keep their original timestamps.
func stampMostRecentTransition(t *testing.T, db *sql.DB, table, entityColumn string, entityID int64, ts time.Time) {
	t.Helper()
	stamp := ts.UTC().Format("2006-01-02 15:04:05")
	res, err := db.Exec(
		fmt.Sprintf("UPDATE %s SET created_at = ?, updated_at = ? WHERE %s = ? AND most_recent = 1", table, entityColumn),
		stamp, stamp, entityID)
	require.NoError(t, err)
	affected, err := res.RowsAffected()
	require.NoError(t, err)
	require.EqualValues(t, 1, affected, "expected to stamp exactly one most-recent transition row")
}

// ============================================================================
// GetActiveActivity
// ============================================================================

func TestGetActiveActivityWithData(t *testing.T) {
	t.Run("returns downloading movies, series and jobs", func(t *testing.T) {
		ctx := context.Background()
		started := time.Now()
		store := newActivityTestStorage(t, ctx)
		svc := &JobService{activityStorage: store}

		// Movie with full metadata and an assigned download client.
		_, err := store.CreateMovieMetadata(ctx, sqlcdb.MovieMetadatum{
			TmdbID: 100, Title: "Active Movie", Year: sqlYear(2024), Images: `{"poster_path":"/active.jpg"}`,
		})
		require.NoError(t, err)
		_, err = store.CreateMovie(ctx, sqlcdb.Movie{QualityProfileID: 1, Path: sql.NullString{String: "Active Movie", Valid: true}, Monitored: 1}, storage.MovieStateMissing)
		require.NoError(t, err)
		clientID, err := store.CreateDownloadClient(ctx, sqlcdb.DownloadClient{
			Type: "nzb", Implementation: "sabnzbd", Scheme: "http", Host: "localhost", Port: 8080,
		})
		require.NoError(t, err)
		require.NoError(t, store.LinkMovieMetadata(ctx, 1, 1))
		require.NoError(t, store.UpdateMovieState(ctx, 1, storage.MovieStateDownloading, &storage.TransitionStateMetadata{
			DownloadClientID: &clientID, DownloadID: activityStringPtr("dl-100"),
		}))

		// Movie with unparsable images JSON, no year, and no download client.
		_, err = store.CreateMovieMetadata(ctx, sqlcdb.MovieMetadatum{TmdbID: 200, Title: "Bare Movie", Images: "not-json"})
		require.NoError(t, err)
		_, err = store.CreateMovie(ctx, sqlcdb.Movie{QualityProfileID: 1, Path: sql.NullString{String: "Bare Movie", Valid: true}, Monitored: 1}, storage.MovieStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.LinkMovieMetadata(ctx, 2, 2))
		require.NoError(t, store.UpdateMovieState(ctx, 2, storage.MovieStateDownloading, nil))

		// Downloading season under a series with metadata.
		_, err = store.CreateSeries(ctx, sqlcdb.Series{QualityProfileID: 4, Monitored: 1}, storage.SeriesStateMissing)
		require.NoError(t, err)
		_, err = store.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{
			TmdbID: 300, Title: "Active Series", PosterPath: sql.NullString{String: "/active-series.jpg", Valid: true},
		})
		require.NoError(t, err)
		require.NoError(t, store.LinkSeriesMetadata(ctx, 1, 1))
		seasonID, err := store.CreateSeason(ctx, sqlcdb.Season{SeriesID: 1, SeasonNumber: 2}, storage.SeasonStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.UpdateSeasonState(ctx, seasonID, storage.SeasonStateDownloading, &storage.TransitionStateMetadata{
			DownloadID: activityStringPtr("dl-200"),
		}))

		jobID, err := store.CreateJob(ctx, sqlcdb.Job{Type: "rss_sync"}, storage.JobStatePending)
		require.NoError(t, err)
		require.NoError(t, store.UpdateJobState(ctx, jobID, storage.JobStateRunning, nil))

		resp, err := svc.GetActiveActivity(ctx, ActivePaginationParams{Movies: pagination.Params{Page: 1, PageSize: 50}, Series: pagination.Params{Page: 1, PageSize: 50}, Jobs: pagination.Params{Page: 1, PageSize: 50}})
		require.NoError(t, err)
		require.NotNil(t, resp)

		// Movies share a sort_key, so ties fall back to most recent transition id.
		require.Len(t, resp.Movies, 2)
		bare := resp.Movies[0]
		require.False(t, bare.StateSince.IsZero())
		assertDurationWithin(t, started, bare.Duration)
		assert.Equal(t, &ActiveMovie{
			ID: 2, TMDBID: 200, Title: "Bare Movie", Year: 0, PosterPath: "", State: "downloading",
			StateSince: bare.StateSince, Duration: bare.Duration,
		}, bare)

		full := resp.Movies[1]
		require.False(t, full.StateSince.IsZero())
		assertDurationWithin(t, started, full.Duration)
		assert.Equal(t, &ActiveMovie{
			ID: 1, TMDBID: 100, Title: "Active Movie", Year: 2024, PosterPath: "/active.jpg", State: "downloading",
			StateSince: full.StateSince, Duration: full.Duration,
			DownloadClient: &DownloadClientInfo{ID: clientID, Host: "localhost", Port: 8080},
			DownloadID:     "dl-100",
		}, full)

		require.Len(t, resp.Series, 1)
		series := resp.Series[0]
		require.False(t, series.StateSince.IsZero())
		assertDurationWithin(t, started, series.Duration)
		assert.Equal(t, &ActiveSeries{
			ID: seasonID, TMDBID: 300, Title: "Active Series", PosterPath: "/active-series.jpg",
			State: "downloading", StateSince: series.StateSince, Duration: series.Duration,
			DownloadID: "dl-200",
		}, series)

		require.Len(t, resp.Jobs, 1)
		job := resp.Jobs[0]
		require.False(t, job.CreatedAt.IsZero())
		require.False(t, job.UpdatedAt.IsZero())
		assertDurationWithin(t, started, job.Duration)
		assert.Equal(t, &ActiveJob{
			ID: jobID, Type: "rss_sync", State: "running",
			CreatedAt: job.CreatedAt, UpdatedAt: job.UpdatedAt, Duration: job.Duration,
		}, job)
	})

	t.Run("completed and missing media are not active", func(t *testing.T) {
		ctx := context.Background()
		store := newActivityTestStorage(t, ctx)
		svc := &JobService{activityStorage: store}

		// The active-activity movie query inner-joins movie_metadata, so both
		// movies carry linked metadata: without it the join itself would hide
		// these rows and the state predicate would go untested.
		_, err := store.CreateMovieMetadata(ctx, sqlcdb.MovieMetadatum{TmdbID: 800, Title: "Completed Movie", Images: "[]"})
		require.NoError(t, err)
		_, err = store.CreateMovie(ctx, sqlcdb.Movie{QualityProfileID: 1, Path: sql.NullString{String: "completed", Valid: true}, Monitored: 1}, storage.MovieStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.LinkMovieMetadata(ctx, 1, 1))
		require.NoError(t, store.UpdateMovieState(ctx, 1, storage.MovieStateDownloaded, nil))

		_, err = store.CreateMovieMetadata(ctx, sqlcdb.MovieMetadatum{TmdbID: 900, Title: "Missing Movie", Images: "[]"})
		require.NoError(t, err)
		_, err = store.CreateMovie(ctx, sqlcdb.Movie{QualityProfileID: 1, Path: sql.NullString{String: "missing", Valid: true}, Monitored: 1}, storage.MovieStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.LinkMovieMetadata(ctx, 2, 2))

		// A season whose latest state is not downloading must be excluded too.
		_, err = store.CreateSeries(ctx, sqlcdb.Series{QualityProfileID: 4, Monitored: 1}, storage.SeriesStateMissing)
		require.NoError(t, err)
		_, err = store.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{TmdbID: 950, Title: "Completed Series"})
		require.NoError(t, err)
		require.NoError(t, store.LinkSeriesMetadata(ctx, 1, 1))
		seasonID, err := store.CreateSeason(ctx, sqlcdb.Season{SeriesID: 1, SeasonNumber: 1}, storage.SeasonStateMissing)
		require.NoError(t, err)
		// missing -> discovered is the only valid non-downloading move here.
		require.NoError(t, store.UpdateSeasonState(ctx, seasonID, storage.SeasonStateDiscovered, nil))

		jobID, err := store.CreateJob(ctx, sqlcdb.Job{Type: "rss_sync"}, storage.JobStatePending)
		require.NoError(t, err)
		require.NoError(t, store.UpdateJobState(ctx, jobID, storage.JobStateRunning, nil))
		require.NoError(t, store.UpdateJobState(ctx, jobID, storage.JobStateDone, nil))

		resp, err := svc.GetActiveActivity(ctx, ActivePaginationParams{Movies: pagination.Params{Page: 1, PageSize: 50}, Series: pagination.Params{Page: 1, PageSize: 50}, Jobs: pagination.Params{Page: 1, PageSize: 50}})
		require.NoError(t, err)
		assert.Equal(t, &ActiveActivityResponse{
			Movies: []*ActiveMovie{}, Series: []*ActiveSeries{}, Jobs: []*ActiveJob{},
			Counts: ActiveActivityCounts{},
			Pagination: ActiveActivityPagination{
				Movies: pagination.Meta{Page: 1, PageSize: 50},
				Series: pagination.Meta{Page: 1, PageSize: 50},
				Jobs:   pagination.Meta{Page: 1, PageSize: 50},
			},
		}, resp)
	})
}

// ============================================================================
// GetRecentFailures
// ============================================================================

func TestGetRecentFailuresWindow(t *testing.T) {
	t.Run("returns error jobs inside the window with retryable items", func(t *testing.T) {
		ctx := context.Background()
		store := newActivityTestStorage(t, ctx)
		svc := &JobService{activityStorage: store}

		jobID, err := store.CreateJob(ctx, sqlcdb.Job{Type: "rss_sync"}, storage.JobStatePending)
		require.NoError(t, err)
		errMsg := "boom"
		require.NoError(t, store.UpdateJobState(ctx, jobID, storage.JobStateError, &errMsg))

		// A running job is never a failure; a done job is never a failure.
		runningID, err := store.CreateJob(ctx, sqlcdb.Job{Type: "series_search"}, storage.JobStatePending)
		require.NoError(t, err)
		require.NoError(t, store.UpdateJobState(ctx, runningID, storage.JobStateRunning, nil))
		doneID, err := store.CreateJob(ctx, sqlcdb.Job{Type: "movie_search"}, storage.JobStatePending)
		require.NoError(t, err)
		require.NoError(t, store.UpdateJobState(ctx, doneID, storage.JobStateRunning, nil))
		require.NoError(t, store.UpdateJobState(ctx, doneID, storage.JobStateDone, nil))

		resp, err := svc.GetRecentFailures(ctx, 24, pagination.Params{Page: 1, PageSize: 50})
		require.NoError(t, err)
		require.Len(t, resp.Failures, 1)

		rows, err := store.ListErrorJobs(ctx, sql.NullTime{Time: time.Now().UTC().Add(-24 * time.Hour), Valid: true})
		require.NoError(t, err)
		require.Len(t, rows, 1)
		assert.Equal(t, &FailureItem{
			Type: "job", ID: jobID, Title: "rss_sync", Subtitle: "", State: "error",
			FailedAt: rows[0].UpdatedAt.Time, Error: "", Retryable: true,
		}, resp.Failures[0])
	})

	t.Run("zero-hour window excludes fresh failures", func(t *testing.T) {
		ctx := context.Background()
		store := newActivityTestStorage(t, ctx)
		svc := &JobService{activityStorage: store}

		jobID, err := store.CreateJob(ctx, sqlcdb.Job{Type: "rss_sync"}, storage.JobStatePending)
		require.NoError(t, err)
		require.NoError(t, store.UpdateJobState(ctx, jobID, storage.JobStateError, activityStringPtr("boom")))

		// The public API rejects non-positive windows rather than treating an
		// invalid duration as an empty successful query.
		_, err = svc.GetRecentFailures(ctx, 0, pagination.Params{Page: 1, PageSize: 50})
		require.Error(t, err)
	})

	t.Run("old failures fall outside the window while recent ones are returned", func(t *testing.T) {
		ctx := context.Background()
		store, db := newStampedActivityTestStorage(t, ctx)
		svc := &JobService{activityStorage: store}

		// An error stamped well beyond a 24h window must not be reported, while
		// an equally fresh error inside the window must be.
		oldAt := time.Now().UTC().Add(-48 * time.Hour).Truncate(time.Second)

		oldID, err := store.CreateJob(ctx, sqlcdb.Job{Type: "movie_search"}, storage.JobStatePending)
		require.NoError(t, err)
		require.NoError(t, store.UpdateJobState(ctx, oldID, storage.JobStateError, activityStringPtr("stale boom")))
		stampMostRecentTransition(t, db, "job_transition", "job_id", oldID, oldAt)

		recentID, err := store.CreateJob(ctx, sqlcdb.Job{Type: "rss_sync"}, storage.JobStatePending)
		require.NoError(t, err)
		before := time.Now().UTC()
		require.NoError(t, store.UpdateJobState(ctx, recentID, storage.JobStateError, activityStringPtr("boom")))
		after := time.Now().UTC()

		resp, err := svc.GetRecentFailures(ctx, 24, pagination.Params{Page: 1, PageSize: 50})
		require.NoError(t, err)
		require.Len(t, resp.Failures, 1)
		assert.Equal(t, recentID, resp.Failures[0].ID)
		// FailedAt is the second-precision stamp written during the guarded
		// UpdateJobState call: flooring can only pull it earlier, so the
		// bounds allow exactly that and nothing else.
		assert.WithinRange(t, resp.Failures[0].FailedAt, before.Truncate(time.Second), after)
	})

	t.Run("returns empty response when nothing failed", func(t *testing.T) {
		ctx := context.Background()
		store := newActivityTestStorage(t, ctx)
		svc := &JobService{activityStorage: store}

		resp, err := svc.GetRecentFailures(ctx, 24, pagination.Params{Page: 1, PageSize: 50})
		require.NoError(t, err)
		assert.Equal(t, &FailuresResponse{
			Failures:   []*FailureItem{},
			Pagination: pagination.Meta{Page: 1, PageSize: 50},
		}, resp)
	})
}

// ============================================================================
// GetActivityTimeline
// ============================================================================

func TestGetActivityTimelineWithData(t *testing.T) {
	t.Run("aggregates counts across entity types and paginates transitions", func(t *testing.T) {
		ctx := context.Background()
		store, db := newStampedActivityTestStorage(t, ctx)
		svc := &JobService{activityStorage: store}

		// movie 1: missing -> downloading -> downloaded (most recent: downloaded)
		_, err := store.CreateMovieMetadata(ctx, sqlcdb.MovieMetadatum{TmdbID: 100, Title: "Timeline Movie One", Images: "[]"})
		require.NoError(t, err)
		movieOne, err := store.CreateMovie(ctx, sqlcdb.Movie{QualityProfileID: 1, Path: sql.NullString{String: "one", Valid: true}, Monitored: 1}, storage.MovieStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.LinkMovieMetadata(ctx, movieOne, 1))
		require.NoError(t, store.UpdateMovieState(ctx, movieOne, storage.MovieStateDownloading, nil))
		require.NoError(t, store.UpdateMovieState(ctx, movieOne, storage.MovieStateDownloaded, nil))

		// movie 2: missing -> downloading (most recent: downloading)
		_, err = store.CreateMovieMetadata(ctx, sqlcdb.MovieMetadatum{TmdbID: 200, Title: "Timeline Movie Two", Images: "[]"})
		require.NoError(t, err)
		movieTwo, err := store.CreateMovie(ctx, sqlcdb.Movie{QualityProfileID: 1, Path: sql.NullString{String: "two", Valid: true}, Monitored: 1}, storage.MovieStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.LinkMovieMetadata(ctx, movieTwo, 2))
		require.NoError(t, store.UpdateMovieState(ctx, movieTwo, storage.MovieStateDownloading, nil))

		// series + season: season missing -> downloading
		_, err = store.CreateSeries(ctx, sqlcdb.Series{QualityProfileID: 4, Monitored: 1}, storage.SeriesStateMissing)
		require.NoError(t, err)
		_, err = store.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{TmdbID: 300, Title: "Timeline Series"})
		require.NoError(t, err)
		require.NoError(t, store.LinkSeriesMetadata(ctx, 1, 1))
		seasonID, err := store.CreateSeason(ctx, sqlcdb.Season{SeriesID: 1, SeasonNumber: 1}, storage.SeasonStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.UpdateSeasonState(ctx, seasonID, storage.SeasonStateDownloading, nil))

		// jobs: one done, one errored
		job1, err := store.CreateJob(ctx, sqlcdb.Job{Type: "rss_sync"}, storage.JobStatePending)
		require.NoError(t, err)
		require.NoError(t, store.UpdateJobState(ctx, job1, storage.JobStateRunning, nil))
		require.NoError(t, store.UpdateJobState(ctx, job1, storage.JobStateDone, nil))
		job2, err := store.CreateJob(ctx, sqlcdb.Job{Type: "series_search"}, storage.JobStatePending)
		require.NoError(t, err)
		require.NoError(t, store.UpdateJobState(ctx, job2, storage.JobStateRunning, nil))
		require.NoError(t, store.UpdateJobState(ctx, job2, storage.JobStateError, activityStringPtr("boom")))

		// Stamp every counted transition to one explicit second so the expected
		// date bucket and the 7-day window do not depend on when the fixtures
		// were created; a pause across UTC midnight must not change the outcome.
		stampedAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
		stampMostRecentTransition(t, db, "movie_transition", "movie_id", movieOne, stampedAt)
		stampMostRecentTransition(t, db, "movie_transition", "movie_id", movieTwo, stampedAt)
		stampMostRecentTransition(t, db, "season_transition", "season_id", seasonID, stampedAt)
		stampMostRecentTransition(t, db, "job_transition", "job_id", job1, stampedAt)
		stampMostRecentTransition(t, db, "job_transition", "job_id", job2, stampedAt)

		today := stampedAt.Format("2006-01-02")
		resp, err := svc.GetActivityTimeline(ctx, 7, pagination.Params{})
		require.NoError(t, err)

		// Five most-recent transitions fall inside the window: the demoted
		// intermediate transitions and the series' own transition do not count.
		assert.Equal(t, 5, resp.Count)
		assert.Equal(t, []*TimelineEntry{{
			Date:   today,
			Movies: &MovieCounts{Downloaded: 1, Downloading: 1},
			Series: &SeriesCounts{Downloading: 1},
			Jobs:   &JobCounts{Done: 1, Error: 1},
		}}, resp.Timeline)

		require.Len(t, resp.Transitions, 5)
		byKey := make(map[string]*TransitionItem, len(resp.Transitions))
		for _, item := range resp.Transitions {
			require.False(t, item.CreatedAt.IsZero())
			byKey[fmt.Sprintf("%s/%d/%s", item.EntityType, item.EntityID, item.ToState)] = item
		}
		// Transition ids are per-table autoincrement values, deterministic for
		// the seeding order above.
		assert.Equal(t, map[string]*TransitionItem{
			"movie/1/downloaded":   {ID: 3, EntityType: "movie", EntityID: 1, EntityTitle: "Timeline Movie One", ToState: "downloaded", FromState: activityStringPtr("downloading"), CreatedAt: byKey["movie/1/downloaded"].CreatedAt},
			"movie/2/downloading":  {ID: 5, EntityType: "movie", EntityID: 2, EntityTitle: "Timeline Movie Two", ToState: "downloading", FromState: activityStringPtr("missing"), CreatedAt: byKey["movie/2/downloading"].CreatedAt},
			"season/1/downloading": {ID: 2, EntityType: "season", EntityID: seasonID, EntityTitle: "Timeline Series", ToState: "downloading", FromState: activityStringPtr("missing"), CreatedAt: byKey["season/1/downloading"].CreatedAt},
			"job/2/error":          {ID: 6, EntityType: "job", EntityID: job2, EntityTitle: "series_search", ToState: "error", FromState: activityStringPtr("running"), CreatedAt: byKey["job/2/error"].CreatedAt},
			"job/1/done":           {ID: 3, EntityType: "job", EntityID: job1, EntityTitle: "rss_sync", ToState: "done", FromState: activityStringPtr("running"), CreatedAt: byKey["job/1/done"].CreatedAt},
		}, byKey)

		// Pagination slices the globally sorted transitions without affecting Count.
		page1, err := svc.GetActivityTimeline(ctx, 7, pagination.Params{Page: 1, PageSize: 2})
		require.NoError(t, err)
		assert.Equal(t, 5, page1.Count)
		assert.Len(t, page1.Transitions, 2)
		assert.Equal(t, page1.Transitions, resp.Transitions[:2], "first page holds the newest items")

		beyond, err := svc.GetActivityTimeline(ctx, 7, pagination.Params{Page: 99, PageSize: 2})
		require.NoError(t, err)
		assert.Equal(t, 5, beyond.Count)
		assert.Equal(t, []*TransitionItem{}, beyond.Transitions)
	})

	t.Run("zero-day window may or may not include same-second transitions but never errors", func(t *testing.T) {
		ctx := context.Background()
		store := newActivityTestStorage(t, ctx)
		svc := &JobService{activityStorage: store}

		_, err := store.CreateMovie(ctx, sqlcdb.Movie{QualityProfileID: 1, Path: sql.NullString{String: "one", Valid: true}, Monitored: 1}, storage.MovieStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.UpdateMovieState(ctx, 1, storage.MovieStateDownloading, nil))

		resp, err := svc.GetActivityTimeline(ctx, 0, pagination.Params{})
		require.NoError(t, err)
		require.NotNil(t, resp.Timeline)
		require.NotNil(t, resp.Transitions)
		for _, entry := range resp.Timeline {
			assert.NotEmpty(t, entry.Date)
		}
	})
}

// ============================================================================
// GetEntityTransitionHistory
// ============================================================================

func TestGetEntityTransitionHistoryWithDuration(t *testing.T) {
	t.Run("renders exact completed and bounded ongoing durations", func(t *testing.T) {
		ctx := context.Background()
		store, db := newStampedActivityTestStorage(t, ctx)
		svc := &JobService{activityStorage: store}

		_, err := store.CreateMovieMetadata(ctx, sqlcdb.MovieMetadatum{
			TmdbID: 700, Title: "History Movie", Images: `{"poster_path":"/history.jpg"}`,
		})
		require.NoError(t, err)
		movieID, err := store.CreateMovie(ctx, sqlcdb.Movie{QualityProfileID: 1, Path: sql.NullString{String: "history", Valid: true}, Monitored: 1}, storage.MovieStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.LinkMovieMetadata(ctx, movieID, 1))

		// The two transitions are stamped 150s apart: a completed interval of
		// exactly 2m30s that no scheduling pause can alter. Same-second stamps
		// would render both durations as "0s" and verify nothing.
		missingAt := time.Now().UTC().Add(-4 * time.Minute).Truncate(time.Second)
		downloadingAt := missingAt.Add(150 * time.Second)
		stampMostRecentTransition(t, db, "movie_transition", "movie_id", movieID, missingAt)
		require.NoError(t, store.UpdateMovieState(ctx, movieID, storage.MovieStateDownloading, nil))
		stampMostRecentTransition(t, db, "movie_transition", "movie_id", movieID, downloadingAt)

		before := time.Now().UTC()
		resp, err := svc.GetEntityTransitionHistory(ctx, "movie", movieID, pagination.Params{Page: 1, PageSize: 50})
		after := time.Now().UTC()
		require.NoError(t, err)
		require.Len(t, resp.History, 2)

		assert.Equal(t, &EntityInfo{
			Type: "movie", ID: movieID, Title: "History Movie", PosterPath: `{"poster_path":"/history.jpg"}`,
		}, resp.Entity)

		first, last := resp.History[0], resp.History[1]
		// Movie transitions always embed a metadata JSON blob; null download
		// fields unmarshal into zero values.
		emptyMetadata := &TransitionMetadata{DownloadClient: &DownloadClientInfo{}}
		assert.Equal(t, &HistoryEntry{
			SortKey: 1, ToState: "missing", CreatedAt: first.CreatedAt, Duration: first.Duration,
			Metadata: emptyMetadata,
		}, first)
		assert.Equal(t, &HistoryEntry{
			SortKey: 2, ToState: "downloading", FromState: activityStringPtr("missing"), CreatedAt: last.CreatedAt, Duration: last.Duration,
			Metadata: emptyMetadata,
		}, last)

		// The persisted stamps must match the seeded ones exactly.
		require.True(t, first.CreatedAt.Equal(missingAt), "first stamp: got %v, want %v", first.CreatedAt, missingAt)
		require.True(t, last.CreatedAt.Equal(downloadingAt), "last stamp: got %v, want %v", last.CreatedAt, downloadingAt)

		// Completed interval: derived from the persisted stamps, so it must be
		// exact - an implementation that used time.Since instead could not
		// produce this value.
		assert.Equal(t, "2m 30s", first.Duration)
		assert.Equal(t, formatDuration(last.CreatedAt.Sub(first.CreatedAt)), first.Duration)

		// Ongoing interval: time.Since(downloadingAt) evaluated inside the
		// guarded call and truncated to whole seconds. The elapsed time at call
		// entry (minus one second of flooring) and at call exit bound it
		// exactly, so no fixed scheduling allowance is needed.
		parsed := parseRenderedDuration(t, last.Duration)
		assert.GreaterOrEqual(t, parsed, before.Sub(downloadingAt)-time.Second, "ongoing duration must not lag the seeded stamp")
		assert.LessOrEqual(t, parsed, after.Sub(downloadingAt), "ongoing duration must not run ahead of the seeded stamp")
	})
}

// ============================================================================
// formatDuration
// ============================================================================

func TestFormatDurationUnits(t *testing.T) {
	testCases := []struct {
		name string
		in   time.Duration
		want string
	}{
		{"zero", 0, "0s"},
		{"seconds", 45 * time.Second, "45s"},
		{"exact minute", 60 * time.Second, "1m"},
		{"minute and seconds", 65 * time.Second, "1m 5s"},
		{"minutes", 125 * time.Second, "2m 5s"},
		{"exact hour", time.Hour, "1h"},
		{"hour and minutes", time.Hour + 5*time.Minute, "1h 5m"},
		{"hours", 2*time.Hour + 2*time.Minute + 5*time.Second, "2h 2m"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, formatDuration(tc.in))
		})
	}
}

func sqlYear(year int) sql.NullInt64 {
	return sql.NullInt64{Int64: int64(year), Valid: true}
}
