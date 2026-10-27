package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/manager"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	mediaSqlite "github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/stretchr/testify/require"
)

// This deliberately does not use golden normalization: sequence 1 must never
// become Unix(1), and neither entity creation nor a series transition is the
// timestamp source for an active season-backed series.
func TestActivityTimestampFollowupSequenceOneUTC(t *testing.T) {
	// The wire suite's TestMain forces UTC. Override both TZ and the cached
	// process location here, restoring them before any other test can run.
	// Do not run this test (or its subtests) in parallel.
	t.Setenv("TZ", "Pacific/Auckland")
	location, err := time.LoadLocation("Pacific/Auckland")
	require.NoError(t, err)
	previousLocal := time.Local
	time.Local = location
	t.Cleanup(func() { time.Local = previousLocal })
	stamp := time.Date(2020, 2, 3, 4, 5, 6, 0, time.UTC)
	_, offset := stamp.In(time.Local).Zone()
	require.NotZero(t, offset, "fixture must execute with a non-UTC local timezone")
	entityCreated := time.Date(2010, 1, 2, 3, 4, 5, 0, time.UTC)

	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "activity.db")
	store, err := mediaSqlite.New(ctx, path)
	require.NoError(t, err)
	// A separate test-only connection sets controlled DATETIME literals. All
	// reads below use the real initialized storage and generated query scans.
	db, err := sql.Open("sqlite3", path+"?_foreign_keys=on&_busy_timeout=5000")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	meta := seedMinimalMovieMetadata(t, ctx, store, 31001, "Fixed UTC Movie")
	movieID, err := store.CreateMovie(ctx, sqlcdb.Movie{
		MovieMetadataID: nullInt64(meta.ID), QualityProfileID: 1, Monitored: 1, Added: nullTime(entityCreated),
	}, storage.MovieStateMissing)
	require.NoError(t, err)
	seriesMetaID, seasonMetaID := seedSeriesTree(t, ctx, store, 32001, "Fixed UTC Series", false)
	seriesID, err := store.CreateSeries(ctx, sqlcdb.Series{
		SeriesMetadataID: nullInt64(seriesMetaID), QualityProfileID: 1, Monitored: 1, Added: nullTime(entityCreated),
	}, storage.SeriesStateDiscovered)
	require.NoError(t, err)
	seasonID, err := store.CreateSeason(ctx, sqlcdb.Season{
		SeriesID: seriesID, SeasonNumber: 1, SeasonMetadataID: nullInt64(seasonMetaID), Monitored: 1,
	}, storage.SeasonStateMissing)
	require.NoError(t, err)
	for _, fixture := range []struct {
		query string
		id    int64
		stamp string
	}{
		{"UPDATE movie_transition SET to_state='downloading', created_at=? WHERE movie_id=? AND sort_key=1 AND most_recent=1", movieID, "2020-02-03 04:05:06"},
		{"UPDATE season_transition SET to_state='downloading', created_at=? WHERE season_id=? AND sort_key=1 AND most_recent=1", seasonID, "2020-02-03 04:05:06"},
		// A different series transition time also guards against using the
		// parent series' transition instead of the downloading season's.
		{"UPDATE series_transition SET created_at=? WHERE series_id=? AND sort_key=1 AND most_recent=1", seriesID, "2011-01-02 03:04:05"},
	} {
		result, err := db.ExecContext(ctx, fixture.query, fixture.stamp, fixture.id)
		require.NoError(t, err)
		changed, err := result.RowsAffected()
		require.NoError(t, err)
		require.EqualValues(t, 1, changed)
	}

	movieRows, err := store.ListDownloadingMovies(ctx)
	require.NoError(t, err)
	require.Equal(t, []sqlcdb.ListDownloadingMoviesRow{{
		ID: movieID, TmdbID: 31001, Title: "Fixed UTC Movie", ToState: "downloading", SortKey: 1,
		CreatedAt: sql.NullTime{Time: stamp, Valid: true},
	}}, movieRows)
	seriesRows, err := store.ListDownloadingSeries(ctx)
	require.NoError(t, err)
	require.Equal(t, []sqlcdb.ListDownloadingSeriesRow{{
		ID: seasonID, TmdbID: nullInt64(32001), Title: nullString("Fixed UTC Series"),
		ToState: "downloading", SortKey: 1, CreatedAt: sql.NullTime{Time: stamp, Valid: true}, SeasonNumber: 1,
	}}, seriesRows)

	service := manager.NewJobService(store, store, store, config.Manager{}, nil)
	before := time.Now()
	active, err := service.GetActiveActivity(ctx)
	require.NoError(t, err)
	after := time.Now()
	require.Len(t, active.Movies, 1)
	require.Len(t, active.Series, 1)
	for _, duration := range []string{active.Movies[0].Duration, active.Series[0].Duration} {
		require.Contains(t, []string{activityFollowupDuration(before.Sub(stamp)), activityFollowupDuration(after.Sub(stamp))}, duration)
	}
	require.Equal(t, &manager.ActiveActivityResponse{
		Movies: []*manager.ActiveMovie{{ID: movieID, TMDBID: 31001, Title: "Fixed UTC Movie", State: "downloading", StateSince: stamp, Duration: active.Movies[0].Duration}},
		Series: []*manager.ActiveSeries{{ID: seasonID, TMDBID: 32001, Title: "Fixed UTC Series", State: "downloading", StateSince: stamp, Duration: active.Series[0].Duration}},
		Jobs:   []*manager.ActiveJob{},
	}, active)

	app := newWireAppWithStore(t, store)
	rr := httptest.NewRecorder()
	app.router.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/activity/active", nil))
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	// Decode raw strings, not time.Time, so an equivalent offset timestamp
	// cannot pass in place of the required canonical UTC Z representation.
	var wire struct {
		Response struct {
			Movies []struct {
				StateSince string `json:"stateSince"`
			} `json:"movies"`
			Series []struct {
				StateSince string `json:"stateSince"`
			} `json:"series"`
		} `json:"response"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &wire))
	require.Len(t, wire.Response.Movies, 1)
	require.Len(t, wire.Response.Series, 1)
	for _, tc := range []struct {
		name string
		got  string
	}{
		{"movie", wire.Response.Movies[0].StateSince},
		{"season-backed series", wire.Response.Series[0].StateSince},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, "2020-02-03T04:05:06Z", tc.got)
			require.NotEqual(t, time.Unix(1, 0).UTC().Format(time.RFC3339), tc.got)
			require.NotEqual(t, entityCreated.Format(time.RFC3339), tc.got)
		})
	}
}

// The fixed timestamp is years old, so formatDuration's hour/minute branch
// applies. Accept the before/after bounds above to avoid minute-boundary races.
func activityFollowupDuration(elapsed time.Duration) string {
	hours, minutes := int64(elapsed.Hours()), int64(elapsed.Minutes())%60
	if minutes == 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dh %dm", hours, minutes)
}
