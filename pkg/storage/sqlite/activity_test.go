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

		// Create metadata first
		_, err := store.CreateMovieMetadata(ctx, model.MovieMetadata{
			TmdbID: 123,
			Title:  "Downloading Movie",
			Year:   int32Ptr(2024),
			Images: `["poster.jpg"]`,
		})
		require.NoError(t, err)

		// Create a movie in missing state (should NOT appear)
		path1 := "path/missing"
		_, err = store.CreateMovie(ctx, storage.Movie{Movie: model.Movie{Path: &path1, Monitored: 1}}, storage.MovieStateMissing)
		require.NoError(t, err)

		// Create a movie and transition to downloading (SHOULD appear)
		path2 := "path/downloading"
		movieID, err := store.CreateMovie(ctx, storage.Movie{Movie: model.Movie{Path: &path2, Monitored: 1}}, storage.MovieStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.LinkMovieMetadata(ctx, movieID, 1))

		// Create a download client (must exist before transition references it)
		_, err = store.CreateDownloadClient(ctx, model.DownloadClient{
			Type:           "nzb",
			Implementation: "sabnzbd",
			Scheme:         "http",
			Host:           "localhost",
			Port:           8080,
		})
		require.NoError(t, err)

		require.NoError(t, store.UpdateMovieState(ctx, movieID, storage.MovieStateDownloading, &storage.TransitionStateMetadata{
			DownloadClientID: int32Ptr(1),
			DownloadID:       stringPtr("dl-001"),
		}))

		movies, err := store.ListDownloadingMovies(ctx)
		require.NoError(t, err)
		require.Len(t, movies, 1)

		m := movies[0]
		assert.Equal(t, movieID, m.ID)
		assert.Equal(t, int64(123), m.TMDBID)
		assert.Equal(t, "Downloading Movie", m.Title)
		assert.Equal(t, "downloading", m.State)
		assert.Equal(t, "dl-001", m.DownloadID)
		assert.Equal(t, `["poster.jpg"]`, m.PosterPath)
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

		// Create series with metadata
		_, err := store.CreateSeries(ctx, storage.Series{Series: model.Series{Monitored: 1, QualityProfileID: 1}}, storage.SeriesStateMissing)
		require.NoError(t, err)
		_, err = store.CreateSeriesMetadata(ctx, model.SeriesMetadata{
			TmdbID:     456,
			Title:      "Downloading Series",
			PosterPath: stringPtr("/poster.jpg"),
		})
		require.NoError(t, err)
		require.NoError(t, store.LinkSeriesMetadata(ctx, 1, 1))

		// Create a season and transition to downloading
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
		assert.Equal(t, "Downloading Series", s.Title)
		assert.Equal(t, "downloading", s.State)
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

		// Create a pending job (should NOT appear)
		_, err := store.CreateJob(ctx, storage.Job{Job: model.Job{Type: "rss_sync"}}, storage.JobStatePending)
		require.NoError(t, err)

		// Create a job and transition to running (SHOULD appear)
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
	})
}

// ============================================================================
// CountTransitionsByDate
// ============================================================================

func TestCountTransitionsByDate(t *testing.T) {
	t.Run("returns zero when no transitions in range", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)

		start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)

		count, err := store.CountTransitionsByDate(ctx, start, end)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("counts recent transitions", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)

		// Create a movie transition
		p := "path/1"
		_, err := store.CreateMovie(ctx, storage.Movie{Movie: model.Movie{Path: &p, Monitored: 1}}, storage.MovieStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.UpdateMovieState(ctx, 1, storage.MovieStateDownloading, nil))

		// Create a job transition
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
		require.NotNil(t, resp)
		assert.Empty(t, resp.Timeline)
		assert.Empty(t, resp.Transitions)
		assert.Equal(t, 0, resp.Count)
	})

	t.Run("returns timeline and transitions", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)

		// Create a movie with metadata
		_, err := store.CreateMovieMetadata(ctx, model.MovieMetadata{
			TmdbID: 100,
			Title:  "Timeline Movie",
			Images: "[]",
		})
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
		assert.Equal(t, "movie", resp.Transitions[0].EntityType)
		assert.Equal(t, "downloading", resp.Transitions[0].ToState)
		assert.Equal(t, "Timeline Movie", resp.Transitions[0].EntityTitle)
	})

	t.Run("returns transitions without limit", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)

		// Create a movie with metadata
		_, err := store.CreateMovieMetadata(ctx, model.MovieMetadata{
			TmdbID: 100,
			Title:  "NoLimit Movie",
			Images: "[]",
		})
		require.NoError(t, err)

		p := "path/1"
		_, err = store.CreateMovie(ctx, storage.Movie{Movie: model.Movie{Path: &p, Monitored: 1}}, storage.MovieStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.LinkMovieMetadata(ctx, 1, 1))
		require.NoError(t, store.UpdateMovieState(ctx, 1, storage.MovieStateDownloading, nil))

		// Exercises the unlimited path (limit = 0)
		now := time.Now().UTC()
		resp, err := store.GetTransitionsByDate(ctx, now.Add(-time.Hour), now.Add(time.Hour), 0, 0)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, 1, resp.Count)
		require.Len(t, resp.Transitions, 1)
		assert.Equal(t, "movie", resp.Transitions[0].EntityType)
		assert.Equal(t, "downloading", resp.Transitions[0].ToState)
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

		// Create movie metadata
		_, err := store.CreateMovieMetadata(ctx, model.MovieMetadata{
			TmdbID: 200,
			Title:  "History Movie",
			Images: `["poster.jpg"]`,
		})
		require.NoError(t, err)

		// Create movie
		p := "path/1"
		movieID, err := store.CreateMovie(ctx, storage.Movie{Movie: model.Movie{Path: &p, Monitored: 1}}, storage.MovieStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.LinkMovieMetadata(ctx, movieID, 1))

		// Create download client for transition metadata
		_, err = store.CreateDownloadClient(ctx, model.DownloadClient{
			Type:           "nzb",
			Implementation: "sabnzbd",
			Scheme:         "http",
			Host:           "localhost",
			Port:           8080,
		})
		require.NoError(t, err)

		// Transition to downloading with download metadata
		require.NoError(t, store.UpdateMovieState(ctx, movieID, storage.MovieStateDownloading, &storage.TransitionStateMetadata{
			DownloadClientID: int32Ptr(1),
			DownloadID:       stringPtr("dl-100"),
		}))

		resp, err := store.GetEntityTransitions(ctx, "movie", movieID)
		require.NoError(t, err)
		require.NotNil(t, resp.Entity)
		assert.Equal(t, "movie", resp.Entity.Type)
		assert.Equal(t, "History Movie", resp.Entity.Title)
		assert.Equal(t, `["poster.jpg"]`, resp.Entity.PosterPath)

		require.Len(t, resp.History, 2)          // missing -> downloading
		assert.Nil(t, resp.History[0].FromState) // initial transition has no from_state
		assert.Equal(t, "missing", resp.History[0].ToState)
		assert.Equal(t, "downloading", resp.History[1].ToState)

		// Last entry should have download metadata
		require.NotNil(t, resp.History[1].Metadata)
		assert.Equal(t, "dl-100", resp.History[1].Metadata.DownloadID)
	})

	t.Run("series transitions", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)

		seriesID, err := store.CreateSeries(ctx, storage.Series{Series: model.Series{Monitored: 1, QualityProfileID: 1}}, storage.SeriesStateMissing)
		require.NoError(t, err)
		_, err = store.CreateSeriesMetadata(ctx, model.SeriesMetadata{
			TmdbID:     300,
			Title:      "History Series",
			PosterPath: stringPtr("/series-poster.jpg"),
		})
		require.NoError(t, err)
		require.NoError(t, store.LinkSeriesMetadata(ctx, seriesID, 1))
		require.NoError(t, store.UpdateSeriesState(ctx, seriesID, storage.SeriesStateDiscovered, nil))

		resp, err := store.GetEntityTransitions(ctx, "series", seriesID)
		require.NoError(t, err)
		require.NotNil(t, resp.Entity)
		assert.Equal(t, "series", resp.Entity.Type)
		assert.Equal(t, "History Series", resp.Entity.Title)
		require.Len(t, resp.History, 2) // missing -> discovered
		assert.Equal(t, "discovered", resp.History[1].ToState)
	})

	t.Run("season transitions", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)

		// Create series with metadata so season can reference it
		_, err := store.CreateSeries(ctx, storage.Series{Series: model.Series{Monitored: 1, QualityProfileID: 1}}, storage.SeriesStateMissing)
		require.NoError(t, err)
		_, err = store.CreateSeriesMetadata(ctx, model.SeriesMetadata{
			TmdbID:     400,
			Title:      "Season Series",
			PosterPath: stringPtr("/season-poster.jpg"),
		})
		require.NoError(t, err)
		require.NoError(t, store.LinkSeriesMetadata(ctx, 1, 1))

		// Create download client for transition metadata
		_, err = store.CreateDownloadClient(ctx, model.DownloadClient{
			Type:           "nzb",
			Implementation: "sabnzbd",
			Scheme:         "http",
			Host:           "localhost",
			Port:           8080,
		})
		require.NoError(t, err)

		seasonID, err := store.CreateSeason(ctx, storage.Season{Season: model.Season{SeriesID: 1, SeasonNumber: 2}}, storage.SeasonStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.UpdateSeasonState(ctx, seasonID, storage.SeasonStateDownloading, &storage.TransitionStateMetadata{
			DownloadClientID: int32Ptr(1),
			DownloadID:       stringPtr("dl-season"),
		}))

		resp, err := store.GetEntityTransitions(ctx, "season", seasonID)
		require.NoError(t, err)
		require.NotNil(t, resp.Entity)
		assert.Equal(t, "season", resp.Entity.Type)
		assert.Equal(t, "Season Series", resp.Entity.Title)
		require.Len(t, resp.History, 2) // missing -> downloading
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
		require.NotNil(t, resp.Entity)
		assert.Equal(t, "job", resp.Entity.Type)
		assert.Equal(t, "rss_sync", resp.Entity.Title)
		require.Len(t, resp.History, 2) // pending -> running
		assert.Equal(t, "running", resp.History[1].ToState)
	})

	t.Run("episode transitions", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)

		// Create series -> season -> episode chain
		_, err := store.CreateSeries(ctx, storage.Series{Series: model.Series{Monitored: 1, QualityProfileID: 1}}, storage.SeriesStateMissing)
		require.NoError(t, err)
		_, err = store.CreateSeriesMetadata(ctx, model.SeriesMetadata{
			TmdbID:     500,
			Title:      "Episode Series",
			PosterPath: stringPtr("/ep-poster.jpg"),
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
		require.NotNil(t, resp.Entity)
		assert.Equal(t, "episode", resp.Entity.Type)
		assert.Equal(t, "Episode Series", resp.Entity.Title)
		require.Len(t, resp.History, 2) // missing -> downloading
		assert.Equal(t, "downloading", resp.History[1].ToState)
	})

	t.Run("movie transitions without download client", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)

		// Create movie metadata
		_, err := store.CreateMovieMetadata(ctx, model.MovieMetadata{
			TmdbID: 600,
			Title:  "No Client Movie",
			Images: "[]",
		})
		require.NoError(t, err)

		p := "path/no-client"
		movieID, err := store.CreateMovie(ctx, storage.Movie{Movie: model.Movie{Path: &p, Monitored: 1}}, storage.MovieStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.LinkMovieMetadata(ctx, movieID, 1))

		// Transition without download metadata
		require.NoError(t, store.UpdateMovieState(ctx, movieID, storage.MovieStateDownloading, nil))

		resp, err := store.GetEntityTransitions(ctx, "movie", movieID)
		require.NoError(t, err)
		require.NotNil(t, resp.Entity)
		assert.Equal(t, "No Client Movie", resp.Entity.Title)
		require.Len(t, resp.History, 2) // missing -> downloading

		// Metadata is present but download client has zero values (no dc linked)
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
