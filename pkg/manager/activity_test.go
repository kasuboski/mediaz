package manager

import (
	"context"
	"database/sql"
	"github.com/kasuboski/mediaz/pkg/pagination"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func newActivityTestStorage(t *testing.T, ctx context.Context) storage.Storage {
	t.Helper()
	store, err := sqlite.New(ctx, ":memory:")
	require.NoError(t, err)
	return store
}

func TestGetTransitionsByDate(t *testing.T) {
	t.Run("returns empty when no transitions", func(t *testing.T) {
		ctx := context.Background()
		store := newActivityTestStorage(t, ctx)
		svc := &JobService{activityStorage: store}

		now := time.Now().UTC()
		resp, err := svc.getTransitionsByDate(ctx, now.Add(-time.Hour), now.Add(time.Hour), 0, 10)
		require.NoError(t, err)
		assert.Equal(t, &TimelineResponse{Transitions: []*TransitionItem{}, Count: 0}, resp)
	})

	t.Run("returns timeline and transitions with limit", func(t *testing.T) {
		ctx := context.Background()
		store := newActivityTestStorage(t, ctx)
		svc := &JobService{activityStorage: store}

		_, err := store.CreateMovieMetadata(ctx, sqlcdb.MovieMetadatum{TmdbID: 100, Title: "Timeline Movie", Images: "[]"})
		require.NoError(t, err)

		p := "path/1"
		_, err = store.CreateMovie(ctx, sqlcdb.Movie{QualityProfileID: 1, Path: sql.NullString{String: p, Valid: true}, Monitored: 1}, storage.MovieStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.LinkMovieMetadata(ctx, 1, 1))
		require.NoError(t, store.UpdateMovieState(ctx, 1, storage.MovieStateDownloading, nil))

		now := time.Now().UTC()
		resp, err := svc.getTransitionsByDate(ctx, now.Add(-time.Hour), now.Add(time.Hour), 0, 10)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, 1, resp.Count)
		require.Len(t, resp.Transitions, 1)

		item := resp.Transitions[0]
		assert.Equal(t, "movie", item.EntityType)
		assert.Equal(t, int64(1), item.EntityID)
		assert.Equal(t, "Timeline Movie", item.EntityTitle)
		assert.Equal(t, "downloading", item.ToState)
	})

	t.Run("applies global pagination across entity types", func(t *testing.T) {
		ctx := context.Background()
		store := newActivityTestStorage(t, ctx)
		svc := &JobService{activityStorage: store}

		movieMetadata, err := store.CreateMovieMetadata(ctx, sqlcdb.MovieMetadatum{TmdbID: 100, Title: "Timeline Movie", Images: "[]"})
		require.NoError(t, err)
		p := "path/1"
		_, err = store.CreateMovie(ctx, sqlcdb.Movie{QualityProfileID: 1, Path: sql.NullString{String: p, Valid: true}, Monitored: 1}, storage.MovieStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.LinkMovieMetadata(ctx, 1, movieMetadata))
		require.NoError(t, store.UpdateMovieState(ctx, 1, storage.MovieStateDownloading, nil))

		_, err = store.CreateJob(ctx, sqlcdb.Job{Type: "rss_sync"}, storage.JobStatePending)
		require.NoError(t, err)
		require.NoError(t, store.UpdateJobState(ctx, 1, storage.JobStateRunning, nil))

		now := time.Now().UTC()

		// limit=1 returns exactly 1 item globally (not 2 from per-entity limits)
		resp, err := svc.getTransitionsByDate(ctx, now.Add(-time.Hour), now.Add(time.Hour), 0, 1)
		require.NoError(t, err)
		assert.Equal(t, 2, resp.Count)
		require.Len(t, resp.Transitions, 1)

		// offset=1, limit=1 returns the second item
		resp, err = svc.getTransitionsByDate(ctx, now.Add(-time.Hour), now.Add(time.Hour), 1, 1)
		require.NoError(t, err)
		assert.Equal(t, 2, resp.Count)
		require.Len(t, resp.Transitions, 1)

		// limit=0 returns all items
		resp, err = svc.getTransitionsByDate(ctx, now.Add(-time.Hour), now.Add(time.Hour), 0, 0)
		require.NoError(t, err)
		assert.Equal(t, 2, resp.Count)
		require.Len(t, resp.Transitions, 2)
		all := resp.Transitions

		// Unlimited still honors a nonzero global offset.
		resp, err = svc.getTransitionsByDate(ctx, now.Add(-time.Hour), now.Add(time.Hour), 1, 0)
		require.NoError(t, err)
		assert.Equal(t, &TimelineResponse{Timeline: resp.Timeline, Transitions: all[1:], Count: 2}, resp)

		resp, err = svc.getTransitionsByDate(ctx, now.Add(-time.Hour), now.Add(time.Hour), 99, 0)
		require.NoError(t, err)
		assert.Empty(t, resp.Transitions)
		assert.Equal(t, 2, resp.Count)
	})

	t.Run("timeline is sorted by date ascending", func(t *testing.T) {
		ctx := context.Background()
		store := newActivityTestStorage(t, ctx)
		svc := &JobService{activityStorage: store}

		p := "path/1"
		_, err := store.CreateMovie(ctx, sqlcdb.Movie{QualityProfileID: 1, Path: sql.NullString{String: p, Valid: true}, Monitored: 1}, storage.MovieStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.UpdateMovieState(ctx, 1, storage.MovieStateDownloading, nil))

		now := time.Now().UTC()
		resp, err := svc.getTransitionsByDate(ctx, now.Add(-24*time.Hour), now.Add(time.Hour), 0, 0)
		require.NoError(t, err)
		for i := 1; i < len(resp.Timeline); i++ {
			assert.LessOrEqual(t, resp.Timeline[i-1].Date, resp.Timeline[i].Date)
		}
	})
}

// ============================================================================
// GetEntityTransitions
// ============================================================================

func TestGetEntityTransitions(t *testing.T) {
	t.Run("returns error for unsupported entity type", func(t *testing.T) {
		ctx := context.Background()
		store := newActivityTestStorage(t, ctx)
		svc := &JobService{activityStorage: store}

		_, err := svc.getEntityTransitions(ctx, "unknown", 1, 0, 50)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported entity type")
	})

	t.Run("movie transitions with metadata", func(t *testing.T) {
		ctx := context.Background()
		store := newActivityTestStorage(t, ctx)
		svc := &JobService{activityStorage: store}

		_, err := store.CreateMovieMetadata(ctx, sqlcdb.MovieMetadatum{
			TmdbID: 200, Title: "History Movie", Images: `["poster.jpg"]`,
		})
		require.NoError(t, err)

		p := "path/1"
		movieID, err := store.CreateMovie(ctx, sqlcdb.Movie{QualityProfileID: 1, Path: sql.NullString{String: p, Valid: true}, Monitored: 1}, storage.MovieStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.LinkMovieMetadata(ctx, movieID, 1))

		_, err = store.CreateDownloadClient(ctx, sqlcdb.DownloadClient{
			Type: "nzb", Implementation: "sabnzbd", Scheme: "http", Host: "localhost", Port: 8080,
		})
		require.NoError(t, err)

		require.NoError(t, store.UpdateMovieState(ctx, movieID, storage.MovieStateDownloading, &storage.TransitionStateMetadata{
			DownloadClientID: activityInt64Ptr(1), DownloadID: activityStringPtr("dl-100"),
		}))

		resp, err := svc.getEntityTransitions(ctx, "movie", movieID, 0, 50)
		require.NoError(t, err)

		assert.Equal(t, &EntityInfo{
			Type: "movie", ID: movieID, Title: "History Movie", PosterPath: `["poster.jpg"]`,
		}, resp.Entity)

		require.Len(t, resp.History, 2)
		assert.Equal(t, pagination.Meta{Page: 1, PageSize: 50, TotalItems: 2, TotalPages: 1}, resp.Pagination)
		assert.Nil(t, resp.History[0].FromState)
		assert.Equal(t, "missing", resp.History[0].ToState)
		assert.Equal(t, "downloading", resp.History[1].ToState)
		assert.False(t, resp.History[1].CreatedAt.IsZero())

		require.NotNil(t, resp.History[1].Metadata)
		assert.Equal(t, "dl-100", resp.History[1].Metadata.DownloadID)
	})

	t.Run("series transitions", func(t *testing.T) {
		ctx := context.Background()
		store := newActivityTestStorage(t, ctx)
		svc := &JobService{activityStorage: store}

		seriesID, err := store.CreateSeries(ctx, sqlcdb.Series{QualityProfileID: 4, Monitored: 1}, storage.SeriesStateMissing)
		require.NoError(t, err)
		_, err = store.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{
			TmdbID: 300, Title: "History Series", PosterPath: sql.NullString{String: "/series-poster.jpg", Valid: true},
		})
		require.NoError(t, err)
		require.NoError(t, store.LinkSeriesMetadata(ctx, seriesID, 1))
		require.NoError(t, store.UpdateSeriesState(ctx, seriesID, storage.SeriesStateDiscovered, nil))

		resp, err := svc.getEntityTransitions(ctx, "series", seriesID, 0, 50)
		require.NoError(t, err)

		assert.Equal(t, &EntityInfo{
			Type: "series", ID: seriesID, Title: "History Series", PosterPath: "/series-poster.jpg",
		}, resp.Entity)
		require.Len(t, resp.History, 2)
		assert.Equal(t, pagination.Meta{Page: 1, PageSize: 50, TotalItems: 2, TotalPages: 1}, resp.Pagination)
		assert.Equal(t, "discovered", resp.History[1].ToState)
	})

	t.Run("season transitions", func(t *testing.T) {
		ctx := context.Background()
		store := newActivityTestStorage(t, ctx)
		svc := &JobService{activityStorage: store}

		_, err := store.CreateSeries(ctx, sqlcdb.Series{QualityProfileID: 4, Monitored: 1}, storage.SeriesStateMissing)
		require.NoError(t, err)
		_, err = store.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{
			TmdbID: 400, Title: "Season Series", PosterPath: sql.NullString{String: "/season-poster.jpg", Valid: true},
		})
		require.NoError(t, err)
		require.NoError(t, store.LinkSeriesMetadata(ctx, 1, 1))

		_, err = store.CreateDownloadClient(ctx, sqlcdb.DownloadClient{
			Type: "nzb", Implementation: "sabnzbd", Scheme: "http", Host: "localhost", Port: 8080,
		})
		require.NoError(t, err)

		seasonID, err := store.CreateSeason(ctx, sqlcdb.Season{SeriesID: 1, SeasonNumber: 2}, storage.SeasonStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.UpdateSeasonState(ctx, seasonID, storage.SeasonStateDownloading, &storage.TransitionStateMetadata{
			DownloadClientID: activityInt64Ptr(1), DownloadID: activityStringPtr("dl-season"),
		}))

		resp, err := svc.getEntityTransitions(ctx, "season", seasonID, 0, 50)
		require.NoError(t, err)

		assert.Equal(t, &EntityInfo{
			Type: "season", ID: seasonID, Title: "Season Series", PosterPath: "/season-poster.jpg",
		}, resp.Entity)
		require.Len(t, resp.History, 2)
		assert.Equal(t, pagination.Meta{Page: 1, PageSize: 50, TotalItems: 2, TotalPages: 1}, resp.Pagination)
		assert.Equal(t, "downloading", resp.History[1].ToState)
		require.NotNil(t, resp.History[1].Metadata)
		assert.Equal(t, "dl-season", resp.History[1].Metadata.DownloadID)
	})

	t.Run("job transitions", func(t *testing.T) {
		ctx := context.Background()
		store := newActivityTestStorage(t, ctx)
		svc := &JobService{activityStorage: store}

		jobID, err := store.CreateJob(ctx, sqlcdb.Job{Type: "rss_sync"}, storage.JobStatePending)
		require.NoError(t, err)
		require.NoError(t, store.UpdateJobState(ctx, jobID, storage.JobStateRunning, nil))

		resp, err := svc.getEntityTransitions(ctx, "job", jobID, 0, 50)
		require.NoError(t, err)

		assert.Equal(t, &EntityInfo{Type: "job", ID: jobID, Title: "rss_sync"}, resp.Entity)
		require.Len(t, resp.History, 2)
		assert.Equal(t, pagination.Meta{Page: 1, PageSize: 50, TotalItems: 2, TotalPages: 1}, resp.Pagination)
		assert.Equal(t, "running", resp.History[1].ToState)
	})

	t.Run("episode transitions", func(t *testing.T) {
		ctx := context.Background()
		store := newActivityTestStorage(t, ctx)
		svc := &JobService{activityStorage: store}

		_, err := store.CreateSeries(ctx, sqlcdb.Series{QualityProfileID: 4, Monitored: 1}, storage.SeriesStateMissing)
		require.NoError(t, err)
		_, err = store.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{
			TmdbID: 500, Title: "Episode Series", PosterPath: sql.NullString{String: "/ep-poster.jpg", Valid: true},
		})
		require.NoError(t, err)
		require.NoError(t, store.LinkSeriesMetadata(ctx, 1, 1))
		_, err = store.CreateSeason(ctx, sqlcdb.Season{SeriesID: 1, SeasonNumber: 1}, storage.SeasonStateMissing)
		require.NoError(t, err)

		episodeID, err := store.CreateEpisode(ctx, sqlcdb.Episode{SeasonID: 1, EpisodeNumber: 1, Monitored: 1}, storage.EpisodeStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.UpdateEpisodeState(ctx, episodeID, storage.EpisodeStateDownloading, nil))

		resp, err := svc.getEntityTransitions(ctx, "episode", episodeID, 0, 50)
		require.NoError(t, err)

		assert.Equal(t, &EntityInfo{
			Type: "episode", ID: episodeID, Title: "Episode Series", PosterPath: "/ep-poster.jpg",
		}, resp.Entity)
		require.Len(t, resp.History, 2)
		assert.Equal(t, pagination.Meta{Page: 1, PageSize: 50, TotalItems: 2, TotalPages: 1}, resp.Pagination)
		assert.Equal(t, "downloading", resp.History[1].ToState)
	})

	t.Run("movie transitions without download client", func(t *testing.T) {
		ctx := context.Background()
		store := newActivityTestStorage(t, ctx)
		svc := &JobService{activityStorage: store}

		_, err := store.CreateMovieMetadata(ctx, sqlcdb.MovieMetadatum{TmdbID: 600, Title: "No Client Movie", Images: "[]"})
		require.NoError(t, err)

		p := "path/no-client"
		movieID, err := store.CreateMovie(ctx, sqlcdb.Movie{QualityProfileID: 1, Path: sql.NullString{String: p, Valid: true}, Monitored: 1}, storage.MovieStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.LinkMovieMetadata(ctx, movieID, 1))
		require.NoError(t, store.UpdateMovieState(ctx, movieID, storage.MovieStateDownloading, nil))

		resp, err := svc.getEntityTransitions(ctx, "movie", movieID, 0, 50)
		require.NoError(t, err)
		require.NotNil(t, resp.Entity)
		require.Len(t, resp.History, 2)
		assert.Equal(t, pagination.Meta{Page: 1, PageSize: 50, TotalItems: 2, TotalPages: 1}, resp.Pagination)
		require.NotNil(t, resp.History[1].Metadata)
		assert.Empty(t, resp.History[1].Metadata.DownloadID)
	})
}

// ============================================================================
// Helpers
// ============================================================================

func activityInt64Ptr(i int64) *int64 {
	return &i
}

func activityStringPtr(s string) *string {
	return &s
}
