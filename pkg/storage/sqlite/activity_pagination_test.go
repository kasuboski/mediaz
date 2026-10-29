package sqlite

import (
	"testing"
	"time"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetActivityTimelinePagePagination(t *testing.T) {
	ctx := t.Context()
	store := initSqlite(t, ctx).(*SQLite)
	stamp := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

	movieMeta, err := store.CreateMovieMetadata(ctx, sqlcdb.MovieMetadatum{TmdbID: 1, Title: "Movie"})
	require.NoError(t, err)
	movie, err := store.CreateMovie(ctx, sqlcdb.Movie{QualityProfileID: 1}, storage.MovieStateMissing)
	require.NoError(t, err)
	require.NoError(t, store.LinkMovieMetadata(ctx, movie, movieMeta))
	require.NoError(t, store.UpdateMovieState(ctx, movie, storage.MovieStateDownloading, nil))

	series, err := store.CreateSeries(ctx, sqlcdb.Series{QualityProfileID: 1}, storage.SeriesStateMissing)
	require.NoError(t, err)
	seriesMeta, err := store.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{TmdbID: 2, Title: "Series"})
	require.NoError(t, err)
	require.NoError(t, store.LinkSeriesMetadata(ctx, series, seriesMeta))
	season, err := store.CreateSeason(ctx, sqlcdb.Season{SeriesID: series, SeasonNumber: 1}, storage.SeasonStateMissing)
	require.NoError(t, err)
	require.NoError(t, store.UpdateSeasonState(ctx, season, storage.SeasonStateDownloading, nil))
	episode, err := store.CreateEpisode(ctx, sqlcdb.Episode{SeasonID: season, EpisodeNumber: 1}, storage.EpisodeStateMissing)
	require.NoError(t, err)
	require.NoError(t, store.UpdateEpisodeState(ctx, episode, storage.EpisodeStateDownloading, nil))
	job, err := store.CreateJob(ctx, sqlcdb.Job{Type: "rss_sync"}, storage.JobStatePending)
	require.NoError(t, err)
	require.NoError(t, store.UpdateJobState(ctx, job, storage.JobStateRunning, nil))

	// Another movie has no metadata and must not appear in the page or total.
	missing, err := store.CreateMovie(ctx, sqlcdb.Movie{QualityProfileID: 1}, storage.MovieStateMissing)
	require.NoError(t, err)
	require.NoError(t, store.UpdateMovieState(ctx, missing, storage.MovieStateDownloading, nil))

	for _, table := range []struct {
		table, column string
		id            int64
	}{
		{"movie_transition", "movie_id", movie}, {"season_transition", "season_id", season},
		{"episode_transition", "episode_id", episode}, {"job_transition", "job_id", job}, {"movie_transition", "movie_id", missing},
	} {
		backdateMostRecentTransition(t, store, ctx, table.table, table.column, table.id, stamp)
	}
	params := func(offset, limit int64) sqlcdb.GetActivityTimelinePageParams {
		return sqlcdb.GetActivityTimelinePageParams{Start: stamp.Format(timestampFormat), End: stamp.Format(timestampFormat), Offs: offset, Lim: limit}
	}
	all, err := store.GetActivityTimelinePage(ctx, params(0, 0))
	require.NoError(t, err)
	require.Len(t, all, 4)
	assert.Equal(t, []string{"episode", "job", "movie", "season"}, []string{all[0].EntityType, all[1].EntityType, all[2].EntityType, all[3].EntityType})
	assert.Equal(t, []string{"Series", "rss_sync", "Movie", "Series"}, []string{all[0].EntityTitle, all[1].EntityTitle, all[2].EntityTitle, all[3].EntityTitle})
	for _, row := range all {
		require.True(t, row.CreatedAt.Valid)
		assert.Equal(t, stamp, row.CreatedAt.Time)
	}

	page, err := store.GetActivityTimelinePage(ctx, params(1, 2))
	require.NoError(t, err)
	assert.Equal(t, all[1:3], page)
	repeat, err := store.GetActivityTimelinePage(ctx, params(1, 2))
	require.NoError(t, err)
	assert.Equal(t, page, repeat)
	for _, tc := range []struct{ off, lim int64 }{{4, 2}, {0, 0}} {
		if tc.off == 0 {
			continue
		}
		empty, err := store.GetActivityTimelinePage(ctx, params(tc.off, tc.lim))
		require.NoError(t, err)
		assert.Empty(t, empty)
	}
	before, err := store.GetActivityTimelinePage(ctx, sqlcdb.GetActivityTimelinePageParams{Start: stamp.Add(time.Second).Format(timestampFormat), End: stamp.Add(time.Hour).Format(timestampFormat), Lim: 10})
	require.NoError(t, err)
	assert.Empty(t, before)
	// The inclusive endpoints retain all four eligible records; metadata exclusion
	// happened within the SQL UNION before LIMIT/OFFSET, not after paging.
	assert.Equal(t, int64(4), int64(len(all)))
}
