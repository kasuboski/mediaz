package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
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

		_, err := store.CreateMovieMetadata(ctx, model.MovieMetadata{
			TmdbID: 123, Title: "Downloading Movie", Year: int32Ptr(2024), Images: `["poster.jpg"]`,
		})
		require.NoError(t, err)

		path1 := "path/missing"
		_, err = store.CreateMovie(ctx, storage.Movie{Movie: model.Movie{Path: &path1, Monitored: 1}}, storage.MovieStateMissing)
		require.NoError(t, err)

		path2 := "path/downloading"
		movieID, err := store.CreateMovie(ctx, storage.Movie{Movie: model.Movie{Path: &path2, Monitored: 1}}, storage.MovieStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.LinkMovieMetadata(ctx, movieID, 1))

		_, err = store.CreateDownloadClient(ctx, model.DownloadClient{
			Type: "nzb", Implementation: "sabnzbd", Scheme: "http", Host: "localhost", Port: 8080,
		})
		require.NoError(t, err)

		require.NoError(t, store.UpdateMovieState(ctx, movieID, storage.MovieStateDownloading, &storage.TransitionStateMetadata{
			DownloadClientID: int32Ptr(1), DownloadID: stringPtr("dl-001"),
		}))

		movies, err := store.ListDownloadingMovies(ctx)
		require.NoError(t, err)
		require.Len(t, movies, 1)

		m := movies[0]
		assert.Equal(t, movieID, m.ID)
		assert.Equal(t, int64(123), m.TMDBID)
		assert.Equal(t, "Downloading Movie", m.Title)
		assert.Equal(t, 2024, m.Year)
		assert.Equal(t, `["poster.jpg"]`, m.PosterPath)
		assert.Equal(t, "downloading", m.State)
		assert.False(t, m.StateSince.IsZero())
		assert.Equal(t, "dl-001", m.DownloadID)
		assert.Equal(t, &storage.DownloadClientInfo{ID: 1, Host: "localhost", Port: 8080}, m.DownloadClient)
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

		_, err := store.CreateSeries(ctx, storage.Series{Series: model.Series{Monitored: 1, QualityProfileID: 1}}, storage.SeriesStateMissing)
		require.NoError(t, err)
		_, err = store.CreateSeriesMetadata(ctx, model.SeriesMetadata{
			TmdbID: 456, Title: "Downloading Series", PosterPath: stringPtr("/poster.jpg"),
		})
		require.NoError(t, err)
		require.NoError(t, store.LinkSeriesMetadata(ctx, 1, 1))

		seasonID, err := store.CreateSeason(ctx, storage.Season{Season: model.Season{SeriesID: 1, SeasonNumber: 1}}, storage.SeasonStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.UpdateSeasonState(ctx, seasonID, storage.SeasonStateDownloading, &storage.TransitionStateMetadata{
			DownloadID: stringPtr("dl-002"),
		}))

		series, err := store.ListDownloadingSeries(ctx)
		require.NoError(t, err)
		require.Len(t, series, 1)

		s := series[0]
		assert.Equal(t, seasonID, s.ID)
		assert.Equal(t, int64(456), s.TMDBID)
		assert.Equal(t, "Downloading Series", s.Title)
		assert.Equal(t, "/poster.jpg", s.PosterPath)
		assert.Equal(t, "downloading", s.State)
		assert.False(t, s.StateSince.IsZero())
		assert.Equal(t, "dl-002", s.DownloadID)
		require.NotNil(t, s.SeasonNumber)
		assert.Equal(t, 1, *s.SeasonNumber)
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

		_, err := store.CreateJob(ctx, storage.Job{Job: model.Job{Type: "rss_sync"}}, storage.JobStatePending)
		require.NoError(t, err)

		jobID, err := store.CreateJob(ctx, storage.Job{Job: model.Job{Type: "series_search"}}, storage.JobStatePending)
		require.NoError(t, err)
		require.NoError(t, store.UpdateJobState(ctx, jobID, storage.JobStateRunning, nil))

		jobs, err := store.ListRunningJobs(ctx)
		require.NoError(t, err)
		require.Len(t, jobs, 1)

		j := jobs[0]
		assert.Equal(t, jobID, j.ID)
		assert.Equal(t, "series_search", j.Type)
		assert.Equal(t, "running", j.State)
		assert.False(t, j.CreatedAt.IsZero())
		assert.False(t, j.UpdatedAt.IsZero())
	})
}

// ============================================================================
// ListErrorJobs
// ============================================================================

func TestListErrorJobs(t *testing.T) {
	t.Run("returns empty when no error jobs", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)

		jobs, err := store.ListErrorJobs(ctx, 24)
		require.NoError(t, err)
		assert.Empty(t, jobs)
	})

	t.Run("returns error jobs within time window", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)

		jobID, err := store.CreateJob(ctx, storage.Job{Job: model.Job{Type: "rss_sync"}}, storage.JobStatePending)
		require.NoError(t, err)
		require.NoError(t, store.UpdateJobState(ctx, jobID, storage.JobStateRunning, nil))
		errMsg := "something went wrong"
		require.NoError(t, store.UpdateJobState(ctx, jobID, storage.JobStateError, &errMsg))

		jobs, err := store.ListErrorJobs(ctx, 24)
		require.NoError(t, err)
		require.Len(t, jobs, 1)

		j := jobs[0]
		assert.Equal(t, jobID, j.ID)
		assert.Equal(t, "rss_sync", j.Type)
		assert.Equal(t, "error", j.State)
		assert.False(t, j.CreatedAt.IsZero())
		assert.False(t, j.UpdatedAt.IsZero())
	})
}

// ============================================================================
// CountTransitionsByDate
// ============================================================================

func TestCountTransitionsByDate(t *testing.T) {
	t.Run("returns zero when no transitions in range", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)

		count, err := store.CountTransitionsByDate(ctx,
			time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
			time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC))
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("counts recent transitions", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)

		p := "path/1"
		_, err := store.CreateMovie(ctx, storage.Movie{Movie: model.Movie{Path: &p, Monitored: 1}}, storage.MovieStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.UpdateMovieState(ctx, 1, storage.MovieStateDownloading, nil))

		_, err = store.CreateJob(ctx, storage.Job{Job: model.Job{Type: "rss_sync"}}, storage.JobStatePending)
		require.NoError(t, err)
		require.NoError(t, store.UpdateJobState(ctx, 1, storage.JobStateRunning, nil))

		now := time.Now().UTC()
		count, err := store.CountTransitionsByDate(ctx, now.Add(-time.Hour), now.Add(time.Hour))
		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})
}

// ============================================================================
// GetTransitionsByDate
// ============================================================================

func TestGetTransitionsByDate(t *testing.T) {
	t.Run("returns empty when no transitions", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)

		now := time.Now().UTC()
		resp, err := store.GetTransitionsByDate(ctx, now.Add(-time.Hour), now.Add(time.Hour), 0, 10)
		require.NoError(t, err)
		assert.Equal(t, &storage.TimelineResponse{Count: 0}, resp)
	})

	t.Run("returns timeline and transitions with limit", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)

		_, err := store.CreateMovieMetadata(ctx, model.MovieMetadata{TmdbID: 100, Title: "Timeline Movie", Images: "[]"})
		require.NoError(t, err)

		p := "path/1"
		_, err = store.CreateMovie(ctx, storage.Movie{Movie: model.Movie{Path: &p, Monitored: 1}}, storage.MovieStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.LinkMovieMetadata(ctx, 1, 1))
		require.NoError(t, store.UpdateMovieState(ctx, 1, storage.MovieStateDownloading, nil))

		now := time.Now().UTC()
		resp, err := store.GetTransitionsByDate(ctx, now.Add(-time.Hour), now.Add(time.Hour), 0, 10)
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
		store := initSqlite(t, ctx)

		p := "path/1"
		_, err := store.CreateMovie(ctx, storage.Movie{Movie: model.Movie{Path: &p, Monitored: 1}}, storage.MovieStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.UpdateMovieState(ctx, 1, storage.MovieStateDownloading, nil))

		_, err = store.CreateJob(ctx, storage.Job{Job: model.Job{Type: "rss_sync"}}, storage.JobStatePending)
		require.NoError(t, err)
		require.NoError(t, store.UpdateJobState(ctx, 1, storage.JobStateRunning, nil))

		now := time.Now().UTC()

		// limit=1 returns exactly 1 item globally (not 2 from per-entity limits)
		resp, err := store.GetTransitionsByDate(ctx, now.Add(-time.Hour), now.Add(time.Hour), 0, 1)
		require.NoError(t, err)
		assert.Equal(t, 2, resp.Count)
		require.Len(t, resp.Transitions, 1)

		// offset=1, limit=1 returns the second item
		resp, err = store.GetTransitionsByDate(ctx, now.Add(-time.Hour), now.Add(time.Hour), 1, 1)
		require.NoError(t, err)
		assert.Equal(t, 2, resp.Count)
		require.Len(t, resp.Transitions, 1)

		// limit=0 returns all items
		resp, err = store.GetTransitionsByDate(ctx, now.Add(-time.Hour), now.Add(time.Hour), 0, 0)
		require.NoError(t, err)
		assert.Equal(t, 2, resp.Count)
		require.Len(t, resp.Transitions, 2)
	})

	t.Run("timeline is sorted by date ascending", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)

		p := "path/1"
		_, err := store.CreateMovie(ctx, storage.Movie{Movie: model.Movie{Path: &p, Monitored: 1}}, storage.MovieStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.UpdateMovieState(ctx, 1, storage.MovieStateDownloading, nil))

		now := time.Now().UTC()
		resp, err := store.GetTransitionsByDate(ctx, now.Add(-24*time.Hour), now.Add(time.Hour), 0, 0)
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
		store := initSqlite(t, ctx)

		_, err := store.GetEntityTransitions(ctx, "unknown", 1)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported entity type")
	})

	t.Run("movie transitions with metadata", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)

		_, err := store.CreateMovieMetadata(ctx, model.MovieMetadata{
			TmdbID: 200, Title: "History Movie", Images: `["poster.jpg"]`,
		})
		require.NoError(t, err)

		p := "path/1"
		movieID, err := store.CreateMovie(ctx, storage.Movie{Movie: model.Movie{Path: &p, Monitored: 1}}, storage.MovieStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.LinkMovieMetadata(ctx, movieID, 1))

		_, err = store.CreateDownloadClient(ctx, model.DownloadClient{
			Type: "nzb", Implementation: "sabnzbd", Scheme: "http", Host: "localhost", Port: 8080,
		})
		require.NoError(t, err)

		require.NoError(t, store.UpdateMovieState(ctx, movieID, storage.MovieStateDownloading, &storage.TransitionStateMetadata{
			DownloadClientID: int32Ptr(1), DownloadID: stringPtr("dl-100"),
		}))

		resp, err := store.GetEntityTransitions(ctx, "movie", movieID)
		require.NoError(t, err)

		assert.Equal(t, &storage.EntityInfo{
			Type: "movie", ID: movieID, Title: "History Movie", PosterPath: `["poster.jpg"]`,
		}, resp.Entity)

		require.Len(t, resp.History, 2)
		assert.Nil(t, resp.History[0].FromState)
		assert.Equal(t, "missing", resp.History[0].ToState)
		assert.Equal(t, "downloading", resp.History[1].ToState)
		assert.False(t, resp.History[1].CreatedAt.IsZero())

		require.NotNil(t, resp.History[1].Metadata)
		assert.Equal(t, "dl-100", resp.History[1].Metadata.DownloadID)
	})

	t.Run("series transitions", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)

		seriesID, err := store.CreateSeries(ctx, storage.Series{Series: model.Series{Monitored: 1, QualityProfileID: 1}}, storage.SeriesStateMissing)
		require.NoError(t, err)
		_, err = store.CreateSeriesMetadata(ctx, model.SeriesMetadata{
			TmdbID: 300, Title: "History Series", PosterPath: stringPtr("/series-poster.jpg"),
		})
		require.NoError(t, err)
		require.NoError(t, store.LinkSeriesMetadata(ctx, seriesID, 1))
		require.NoError(t, store.UpdateSeriesState(ctx, seriesID, storage.SeriesStateDiscovered, nil))

		resp, err := store.GetEntityTransitions(ctx, "series", seriesID)
		require.NoError(t, err)

		assert.Equal(t, &storage.EntityInfo{
			Type: "series", ID: seriesID, Title: "History Series", PosterPath: "/series-poster.jpg",
		}, resp.Entity)
		require.Len(t, resp.History, 2)
		assert.Equal(t, "discovered", resp.History[1].ToState)
	})

	t.Run("season transitions", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)

		_, err := store.CreateSeries(ctx, storage.Series{Series: model.Series{Monitored: 1, QualityProfileID: 1}}, storage.SeriesStateMissing)
		require.NoError(t, err)
		_, err = store.CreateSeriesMetadata(ctx, model.SeriesMetadata{
			TmdbID: 400, Title: "Season Series", PosterPath: stringPtr("/season-poster.jpg"),
		})
		require.NoError(t, err)
		require.NoError(t, store.LinkSeriesMetadata(ctx, 1, 1))

		_, err = store.CreateDownloadClient(ctx, model.DownloadClient{
			Type: "nzb", Implementation: "sabnzbd", Scheme: "http", Host: "localhost", Port: 8080,
		})
		require.NoError(t, err)

		seasonID, err := store.CreateSeason(ctx, storage.Season{Season: model.Season{SeriesID: 1, SeasonNumber: 2}}, storage.SeasonStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.UpdateSeasonState(ctx, seasonID, storage.SeasonStateDownloading, &storage.TransitionStateMetadata{
			DownloadClientID: int32Ptr(1), DownloadID: stringPtr("dl-season"),
		}))

		resp, err := store.GetEntityTransitions(ctx, "season", seasonID)
		require.NoError(t, err)

		assert.Equal(t, &storage.EntityInfo{
			Type: "season", ID: seasonID, Title: "Season Series", PosterPath: "/season-poster.jpg",
		}, resp.Entity)
		require.Len(t, resp.History, 2)
		assert.Equal(t, "downloading", resp.History[1].ToState)
		require.NotNil(t, resp.History[1].Metadata)
		assert.Equal(t, "dl-season", resp.History[1].Metadata.DownloadID)
	})

	t.Run("job transitions", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)

		jobID, err := store.CreateJob(ctx, storage.Job{Job: model.Job{Type: "rss_sync"}}, storage.JobStatePending)
		require.NoError(t, err)
		require.NoError(t, store.UpdateJobState(ctx, jobID, storage.JobStateRunning, nil))

		resp, err := store.GetEntityTransitions(ctx, "job", jobID)
		require.NoError(t, err)

		assert.Equal(t, &storage.EntityInfo{Type: "job", ID: jobID, Title: "rss_sync"}, resp.Entity)
		require.Len(t, resp.History, 2)
		assert.Equal(t, "running", resp.History[1].ToState)
	})

	t.Run("episode transitions", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)

		_, err := store.CreateSeries(ctx, storage.Series{Series: model.Series{Monitored: 1, QualityProfileID: 1}}, storage.SeriesStateMissing)
		require.NoError(t, err)
		_, err = store.CreateSeriesMetadata(ctx, model.SeriesMetadata{
			TmdbID: 500, Title: "Episode Series", PosterPath: stringPtr("/ep-poster.jpg"),
		})
		require.NoError(t, err)
		require.NoError(t, store.LinkSeriesMetadata(ctx, 1, 1))
		_, err = store.CreateSeason(ctx, storage.Season{Season: model.Season{SeriesID: 1, SeasonNumber: 1}}, storage.SeasonStateMissing)
		require.NoError(t, err)

		episodeID, err := store.CreateEpisode(ctx, storage.Episode{Episode: model.Episode{SeasonID: 1, EpisodeNumber: 1, Monitored: 1}}, storage.EpisodeStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.UpdateEpisodeState(ctx, episodeID, storage.EpisodeStateDownloading, nil))

		resp, err := store.GetEntityTransitions(ctx, "episode", episodeID)
		require.NoError(t, err)

		assert.Equal(t, &storage.EntityInfo{
			Type: "episode", ID: episodeID, Title: "Episode Series", PosterPath: "/ep-poster.jpg",
		}, resp.Entity)
		require.Len(t, resp.History, 2)
		assert.Equal(t, "downloading", resp.History[1].ToState)
	})

	t.Run("movie transitions without download client", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)

		_, err := store.CreateMovieMetadata(ctx, model.MovieMetadata{TmdbID: 600, Title: "No Client Movie", Images: "[]"})
		require.NoError(t, err)

		p := "path/no-client"
		movieID, err := store.CreateMovie(ctx, storage.Movie{Movie: model.Movie{Path: &p, Monitored: 1}}, storage.MovieStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.LinkMovieMetadata(ctx, movieID, 1))
		require.NoError(t, store.UpdateMovieState(ctx, movieID, storage.MovieStateDownloading, nil))

		resp, err := store.GetEntityTransitions(ctx, "movie", movieID)
		require.NoError(t, err)
		require.NotNil(t, resp.Entity)
		require.Len(t, resp.History, 2)
		require.NotNil(t, resp.History[1].Metadata)
		assert.Empty(t, resp.History[1].Metadata.DownloadID)
	})
}

// ============================================================================
// Helpers
// ============================================================================

func int32Ptr(i int32) *int32 {
	return &i
}

func stringPtr(s string) *string {
	return &s
}
