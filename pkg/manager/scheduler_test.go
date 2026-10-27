package manager

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	mediaSqlite "github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newJobTestStore(t *testing.T, ctx context.Context) storage.Storage {
	t.Helper()
	store, err := mediaSqlite.New(ctx, ":memory:")
	require.NoError(t, err)
	return store
}

func asListJobRow(job sqlcdb.GetJobByIDRow) sqlcdb.ListJobsRow {
	return sqlcdb.ListJobsRow{
		Job:       job.Job,
		State:     job.State,
		UpdatedAt: job.UpdatedAt,
		Error:     job.Error,
	}
}

func TestScheduler_createPendingJob(t *testing.T) {
	t.Run("invalid job type", func(t *testing.T) {
		store, err := mediaSqlite.New(context.Background(), ":memory:")
		require.NoError(t, err)

		scheduler := NewScheduler(store, config.Manager{}, make(map[JobType]JobExecutor))
		id, err := scheduler.createPendingJob(context.TODO(), "my-fake-job")

		assert.Equal(t, id, int64(0))
		require.NotNil(t, err)
		assert.Equal(t, err.Error(), "invalid job type")
	})

	t.Run("create job and duplicate pending job", func(t *testing.T) {
		ctx := context.Background()
		store := newJobTestStore(t, ctx)

		scheduler := NewScheduler(store, config.Manager{}, make(map[JobType]JobExecutor))

		id, err := scheduler.createPendingJob(ctx, MovieIndex)
		require.Nil(t, err)
		assert.NotEqual(t, int64(0), id)

		jobs, err := scheduler.listPendingJobs(ctx)
		require.Nil(t, err)
		require.Len(t, jobs, 1)

		job := jobs[0]
		assert.Equal(t, id, job.Job.ID)
		assert.Equal(t, job.Job.Type, string(MovieIndex))
		assert.True(t, job.Job.CreatedAt.Valid)

		jobs, err = scheduler.listPendingJobsByType(ctx, MovieIndex)
		require.Nil(t, err)

		job = jobs[0]
		assert.Equal(t, id, job.Job.ID)
		assert.Equal(t, job.Job.Type, string(MovieIndex))
		assert.True(t, job.Job.CreatedAt.Valid)

		id, err = scheduler.createPendingJob(ctx, MovieIndex)
		require.NotNil(t, err)
		require.Equal(t, id, int64(0))
	})
}

func TestScheduler_listPendingJobs(t *testing.T) {
	t.Run("no pending jobs", func(t *testing.T) {
		ctx := context.Background()
		store := newJobTestStore(t, ctx)

		scheduler := NewScheduler(store, config.Manager{}, make(map[JobType]JobExecutor))

		jobs, err := scheduler.listPendingJobs(ctx)
		require.NoError(t, err)
		assert.Len(t, jobs, 0)
	})

	t.Run("list multiple pending jobs", func(t *testing.T) {
		ctx := context.Background()
		store := newJobTestStore(t, ctx)

		scheduler := NewScheduler(store, config.Manager{}, make(map[JobType]JobExecutor))

		id1, err := scheduler.createPendingJob(ctx, MovieIndex)
		require.NoError(t, err)
		assert.NotEqual(t, int64(0), id1)

		err = store.UpdateJobState(ctx, id1, storage.JobStateRunning, nil)
		require.NoError(t, err)

		id2, err := scheduler.createPendingJob(ctx, SeriesIndex)
		require.NoError(t, err)
		assert.NotEqual(t, int64(0), id2)

		jobs, err := scheduler.listPendingJobs(ctx)
		require.NoError(t, err)
		require.Len(t, jobs, 1)

		assert.Equal(t, id2, jobs[0].Job.ID)
		assert.Equal(t, jobs[0].Job.Type, string(SeriesIndex))
		assert.True(t, jobs[0].Job.CreatedAt.Valid)
	})
}

func TestScheduler_listPendingJobsByType(t *testing.T) {
	t.Run("no pending jobs of type", func(t *testing.T) {
		ctx := context.Background()
		store := newJobTestStore(t, ctx)

		scheduler := NewScheduler(store, config.Manager{}, make(map[JobType]JobExecutor))

		jobs, err := scheduler.listPendingJobsByType(ctx, MovieIndex)
		require.NoError(t, err)
		assert.Len(t, jobs, 0)
	})

	t.Run("list only jobs of specific type", func(t *testing.T) {
		ctx := context.Background()
		store := newJobTestStore(t, ctx)

		scheduler := NewScheduler(store, config.Manager{}, make(map[JobType]JobExecutor))

		movieID, err := scheduler.createPendingJob(ctx, MovieIndex)
		require.NoError(t, err)
		assert.NotEqual(t, movieID, int64(0))

		seriesID, err := scheduler.createPendingJob(ctx, SeriesIndex)
		require.NoError(t, err)
		assert.NotEqual(t, seriesID, int64(0))

		err = store.UpdateJobState(ctx, movieID, storage.JobStateRunning, nil)
		require.NoError(t, err)

		jobs, err := scheduler.listPendingJobsByType(ctx, SeriesIndex)
		require.NoError(t, err)
		require.Len(t, jobs, 1)

		assert.Equal(t, seriesID, jobs[0].Job.ID)
		assert.Equal(t, jobs[0].Job.Type, string(SeriesIndex))
		assert.True(t, jobs[0].Job.CreatedAt.Valid)
	})
}

func TestScheduler_executeJob(t *testing.T) {
	t.Run("successful job execution", func(t *testing.T) {
		ctx := context.Background()
		store := newJobTestStore(t, ctx)

		executorCalled := false
		executors := map[JobType]JobExecutor{
			MovieIndex: func(ctx context.Context, jobID int64) error {
				executorCalled = true
				return nil
			},
		}

		scheduler := NewScheduler(store, config.Manager{}, executors)

		jobID, err := scheduler.createPendingJob(ctx, MovieIndex)
		require.NoError(t, err)

		job, err := store.GetJob(ctx, jobID)
		require.NoError(t, err)
		require.Equal(t, string(storage.JobStatePending), job.State)

		scheduler.executeJob(ctx, asListJobRow(job))

		assert.True(t, executorCalled)

		updatedJob, err := store.GetJob(ctx, jobID)
		require.NoError(t, err)
		assert.Equal(t, string(storage.JobStateDone), updatedJob.State)
		assert.Equal(t, sql.NullString{}, updatedJob.Error)

		_, inCache := scheduler.runningJobs.Get(jobID)
		assert.False(t, inCache, "job should be removed from cache after completion")
	})

	t.Run("no executor found for job type", func(t *testing.T) {
		ctx := context.Background()
		store := newJobTestStore(t, ctx)

		scheduler := NewScheduler(store, config.Manager{}, make(map[JobType]JobExecutor))

		jobID, err := scheduler.createPendingJob(ctx, MovieIndex)
		require.NoError(t, err)

		job, err := store.GetJob(ctx, jobID)
		require.NoError(t, err)

		scheduler.executeJob(ctx, asListJobRow(job))

		updatedJob, err := store.GetJob(ctx, jobID)
		require.NoError(t, err)
		assert.Equal(t, string(storage.JobStateError), updatedJob.State)
		assert.True(t, updatedJob.Error.Valid)
		assert.Equal(t, "no executor found for job type", updatedJob.Error.String)

		_, inCache := scheduler.runningJobs.Get(jobID)
		assert.False(t, inCache, "job should be removed from cache after error")
	})

	t.Run("executor returns error", func(t *testing.T) {
		ctx := context.Background()
		store := newJobTestStore(t, ctx)

		testError := "test execution error"
		executors := map[JobType]JobExecutor{
			MovieIndex: func(ctx context.Context, jobID int64) error {
				return errors.New(testError)
			},
		}

		scheduler := NewScheduler(store, config.Manager{}, executors)

		jobID, err := scheduler.createPendingJob(ctx, MovieIndex)
		require.NoError(t, err)

		job, err := store.GetJob(ctx, jobID)
		require.NoError(t, err)

		scheduler.executeJob(ctx, asListJobRow(job))

		updatedJob, err := store.GetJob(ctx, jobID)
		require.NoError(t, err)
		assert.Equal(t, string(storage.JobStateError), updatedJob.State)
		assert.True(t, updatedJob.Error.Valid)
		assert.Equal(t, testError, updatedJob.Error.String)

		_, inCache := scheduler.runningJobs.Get(jobID)
		assert.False(t, inCache, "job should be removed from cache after error")
	})

	t.Run("job added to and removed from running cache", func(t *testing.T) {
		ctx := context.Background()
		store := newJobTestStore(t, ctx)

		jobExecuting := make(chan bool)
		executors := map[JobType]JobExecutor{
			MovieIndex: func(ctx context.Context, jobID int64) error {
				jobExecuting <- true
				<-ctx.Done()
				return nil
			},
		}

		scheduler := NewScheduler(store, config.Manager{}, executors)

		jobID, err := scheduler.createPendingJob(ctx, MovieIndex)
		require.NoError(t, err)

		job, err := store.GetJob(ctx, jobID)
		require.NoError(t, err)

		jobCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		go scheduler.executeJob(jobCtx, asListJobRow(job))

		<-jobExecuting

		_, inCache := scheduler.runningJobs.Get(jobID)
		assert.True(t, inCache, "job should be in cache while running")

		cancel()

		require.Eventually(t, func() bool {
			_, inCache := scheduler.runningJobs.Get(jobID)
			return !inCache
		}, time.Second*2, time.Millisecond*10, "job should be removed from cache after completion")
	})
}

func TestScheduler_CancelJob(t *testing.T) {
	t.Run("cancel pending job", func(t *testing.T) {
		ctx := context.Background()
		store := newJobTestStore(t, ctx)

		scheduler := NewScheduler(store, config.Manager{}, make(map[JobType]JobExecutor))

		jobID, err := scheduler.createPendingJob(ctx, MovieIndex)
		require.NoError(t, err)

		job, err := store.GetJob(ctx, jobID)
		require.NoError(t, err)
		assert.Equal(t, string(storage.JobStatePending), job.State)

		err = scheduler.CancelJob(ctx, jobID)
		require.NoError(t, err)

		job, err = store.GetJob(ctx, jobID)
		require.NoError(t, err)
		assert.Equal(t, string(storage.JobStateCancelled), job.State)
	})

	t.Run("cancel running job", func(t *testing.T) {
		ctx := context.Background()
		store := newJobTestStore(t, ctx)

		jobExecuting := make(chan bool)
		executors := map[JobType]JobExecutor{
			MovieIndex: func(ctx context.Context, jobID int64) error {
				jobExecuting <- true
				<-ctx.Done()
				return ctx.Err()
			},
		}

		scheduler := NewScheduler(store, config.Manager{}, executors)

		jobID, err := scheduler.createPendingJob(ctx, MovieIndex)
		require.NoError(t, err)

		job, err := store.GetJob(ctx, jobID)
		require.NoError(t, err)

		go scheduler.executeJob(ctx, asListJobRow(job))

		<-jobExecuting

		_, inCache := scheduler.runningJobs.Get(jobID)
		assert.True(t, inCache, "job should be in cache while running")

		err = scheduler.CancelJob(ctx, jobID)
		require.NoError(t, err)

		_, inCache = scheduler.runningJobs.Get(jobID)
		assert.False(t, inCache, "job should be removed from cache after cancellation")

		updatedJob, err := store.GetJob(ctx, jobID)
		require.NoError(t, err)
		assert.Equal(t, string(storage.JobStateCancelled), updatedJob.State)
	})

	t.Run("cancel completed job does nothing", func(t *testing.T) {
		ctx := context.Background()
		store := newJobTestStore(t, ctx)

		scheduler := NewScheduler(store, config.Manager{}, make(map[JobType]JobExecutor))

		jobID, err := scheduler.createPendingJob(ctx, MovieIndex)
		require.NoError(t, err)

		err = store.UpdateJobState(ctx, jobID, storage.JobStateRunning, nil)
		require.NoError(t, err)

		err = store.UpdateJobState(ctx, jobID, storage.JobStateDone, nil)
		require.NoError(t, err)

		err = scheduler.CancelJob(ctx, jobID)
		require.NoError(t, err)

		job, err := store.GetJob(ctx, jobID)
		require.NoError(t, err)
		assert.Equal(t, string(storage.JobStateDone), job.State)
	})

	t.Run("cancel errored job does nothing", func(t *testing.T) {
		ctx := context.Background()
		store := newJobTestStore(t, ctx)

		scheduler := NewScheduler(store, config.Manager{}, make(map[JobType]JobExecutor))

		jobID, err := scheduler.createPendingJob(ctx, MovieIndex)
		require.NoError(t, err)

		errorMsg := "test error"
		err = store.UpdateJobState(ctx, jobID, storage.JobStateError, &errorMsg)
		require.NoError(t, err)

		err = scheduler.CancelJob(ctx, jobID)
		require.NoError(t, err)

		job, err := store.GetJob(ctx, jobID)
		require.NoError(t, err)
		assert.Equal(t, string(storage.JobStateError), job.State)
		assert.True(t, job.Error.Valid)
		assert.Equal(t, errorMsg, job.Error.String)
	})

	t.Run("cancel already cancelled job does nothing", func(t *testing.T) {
		ctx := context.Background()
		store := newJobTestStore(t, ctx)

		scheduler := NewScheduler(store, config.Manager{}, make(map[JobType]JobExecutor))

		jobID, err := scheduler.createPendingJob(ctx, MovieIndex)
		require.NoError(t, err)

		err = store.UpdateJobState(ctx, jobID, storage.JobStateCancelled, nil)
		require.NoError(t, err)

		err = scheduler.CancelJob(ctx, jobID)
		require.NoError(t, err)

		job, err := store.GetJob(ctx, jobID)
		require.NoError(t, err)
		assert.Equal(t, string(storage.JobStateCancelled), job.State)
	})

	t.Run("cancel non-existent job", func(t *testing.T) {
		ctx := context.Background()
		store := newJobTestStore(t, ctx)

		scheduler := NewScheduler(store, config.Manager{}, make(map[JobType]JobExecutor))

		err := scheduler.CancelJob(ctx, 9999)
		require.Error(t, err)
	})
}

func TestIsValidJobType(t *testing.T) {
	t.Run("valid job types", func(t *testing.T) {
		validTypes := []string{
			string(MovieIndex),
			string(MovieReconcile),
			string(SeriesIndex),
			string(SeriesReconcile),
		}

		for _, jobType := range validTypes {
			assert.True(t, isValidJobType(jobType), "expected %s to be valid", jobType)
		}
	})

	t.Run("invalid job types", func(t *testing.T) {
		invalidTypes := []string{
			"InvalidType",
			"movie-index",
			"",
			"SomeRandomString",
		}

		for _, jobType := range invalidTypes {
			assert.False(t, isValidJobType(jobType), "expected %s to be invalid", jobType)
		}
	})
}

func TestToJobResponse(t *testing.T) {
	now := time.Now()
	errorMsg := "test error"

	t.Run("job with no error", func(t *testing.T) {
		job := sqlcdb.GetJobByIDRow{
			Job:       sqlcdb.Job{ID: 1, Type: string(MovieIndex), CreatedAt: sql.NullTime{Time: now, Valid: true}},
			State:     string(storage.JobStateDone),
			UpdatedAt: sql.NullTime{Time: now, Valid: true},
		}

		response := toJobResponse(job)

		assert.Equal(t, int64(1), response.ID)
		assert.Equal(t, string(MovieIndex), response.Type)
		assert.Equal(t, string(storage.JobStateDone), response.State)
		assert.Equal(t, now, response.CreatedAt)
		assert.Equal(t, now, response.UpdatedAt)
		assert.Nil(t, response.Error)
	})

	t.Run("job with error", func(t *testing.T) {
		job := sqlcdb.GetJobByIDRow{
			Job:       sqlcdb.Job{ID: 2, Type: string(SeriesIndex), CreatedAt: sql.NullTime{Time: now, Valid: true}},
			State:     string(storage.JobStateError),
			UpdatedAt: sql.NullTime{Time: now, Valid: true},
			Error:     sql.NullString{String: errorMsg, Valid: true},
		}

		response := toJobResponse(job)

		assert.Equal(t, int64(2), response.ID)
		assert.Equal(t, string(SeriesIndex), response.Type)
		assert.Equal(t, string(storage.JobStateError), response.State)
		assert.Equal(t, now, response.CreatedAt)
		assert.Equal(t, now, response.UpdatedAt)
		assert.NotNil(t, response.Error)
		assert.Equal(t, errorMsg, *response.Error)
	})
}

func TestScheduler_checkAndScheduleJob(t *testing.T) {
	t.Run("no previous jobs schedules immediately", func(t *testing.T) {
		store, err := mediaSqlite.New(context.Background(), ":memory:")
		require.NoError(t, err)

		err = store.RunMigrations(context.Background())
		require.NoError(t, err)

		ctx := context.Background()
		cfg := config.Manager{
			Jobs: config.Jobs{
				MovieIndex: 10 * time.Minute,
			},
		}

		scheduler := NewScheduler(store, cfg, make(map[JobType]JobExecutor))

		scheduler.checkAndScheduleJob(ctx, MovieIndex)

		jobs, err := scheduler.listPendingJobsByType(ctx, MovieIndex)
		require.NoError(t, err)
		assert.Len(t, jobs, 1, "should create pending job when no previous jobs exist")
	})

	t.Run("interval elapsed schedules new job", func(t *testing.T) {
		store, err := mediaSqlite.New(context.Background(), ":memory:")
		require.NoError(t, err)

		err = store.RunMigrations(context.Background())
		require.NoError(t, err)

		ctx := context.Background()
		cfg := config.Manager{
			Jobs: config.Jobs{
				MovieIndex: 1 * time.Millisecond,
			},
		}

		scheduler := NewScheduler(store, cfg, make(map[JobType]JobExecutor))

		jobID, err := scheduler.createPendingJob(ctx, MovieIndex)
		require.NoError(t, err)

		err = store.UpdateJobState(ctx, jobID, storage.JobStateRunning, nil)
		require.NoError(t, err)

		err = store.UpdateJobState(ctx, jobID, storage.JobStateDone, nil)
		require.NoError(t, err)

		time.Sleep(2 * time.Millisecond)

		scheduler.checkAndScheduleJob(ctx, MovieIndex)

		jobs, err := scheduler.listPendingJobsByType(ctx, MovieIndex)
		require.NoError(t, err)
		assert.Len(t, jobs, 1, "should create new job when interval has elapsed")
	})

	t.Run("interval not elapsed does not schedule", func(t *testing.T) {
		store, err := mediaSqlite.New(context.Background(), ":memory:")
		require.NoError(t, err)

		err = store.RunMigrations(context.Background())
		require.NoError(t, err)

		ctx := context.Background()
		cfg := config.Manager{
			Jobs: config.Jobs{
				MovieIndex: 1 * time.Hour,
			},
		}

		scheduler := NewScheduler(store, cfg, make(map[JobType]JobExecutor))

		jobID, err := scheduler.createPendingJob(ctx, MovieIndex)
		require.NoError(t, err)

		err = store.UpdateJobState(ctx, jobID, storage.JobStateRunning, nil)
		require.NoError(t, err)

		err = store.UpdateJobState(ctx, jobID, storage.JobStateDone, nil)
		require.NoError(t, err)

		scheduler.checkAndScheduleJob(ctx, MovieIndex)

		jobs, err := scheduler.listPendingJobsByType(ctx, MovieIndex)
		require.NoError(t, err)
		assert.Len(t, jobs, 0, "should not create new job when interval has not elapsed")
	})

	t.Run("considers error state as completed", func(t *testing.T) {
		store, err := mediaSqlite.New(context.Background(), ":memory:")
		require.NoError(t, err)

		err = store.RunMigrations(context.Background())
		require.NoError(t, err)

		ctx := context.Background()
		cfg := config.Manager{
			Jobs: config.Jobs{
				MovieIndex: 1 * time.Millisecond,
			},
		}

		scheduler := NewScheduler(store, cfg, make(map[JobType]JobExecutor))

		jobID, err := scheduler.createPendingJob(ctx, MovieIndex)
		require.NoError(t, err)

		err = store.UpdateJobState(ctx, jobID, storage.JobStateRunning, nil)
		require.NoError(t, err)

		errMsg := "test error"
		err = store.UpdateJobState(ctx, jobID, storage.JobStateError, &errMsg)
		require.NoError(t, err)

		time.Sleep(2 * time.Millisecond)

		scheduler.checkAndScheduleJob(ctx, MovieIndex)

		jobs, err := scheduler.listPendingJobsByType(ctx, MovieIndex)
		require.NoError(t, err)
		assert.Len(t, jobs, 1, "should schedule new job after error state")
	})

	t.Run("considers cancelled state as completed", func(t *testing.T) {
		store, err := mediaSqlite.New(context.Background(), ":memory:")
		require.NoError(t, err)

		err = store.RunMigrations(context.Background())
		require.NoError(t, err)

		ctx := context.Background()
		cfg := config.Manager{
			Jobs: config.Jobs{
				MovieIndex: 1 * time.Millisecond,
			},
		}

		scheduler := NewScheduler(store, cfg, make(map[JobType]JobExecutor))

		jobID, err := scheduler.createPendingJob(ctx, MovieIndex)
		require.NoError(t, err)

		err = store.UpdateJobState(ctx, jobID, storage.JobStateCancelled, nil)
		require.NoError(t, err)

		time.Sleep(2 * time.Millisecond)

		scheduler.checkAndScheduleJob(ctx, MovieIndex)

		jobs, err := scheduler.listPendingJobsByType(ctx, MovieIndex)
		require.NoError(t, err)
		assert.Len(t, jobs, 1, "should schedule new job after cancelled state")
	})

	t.Run("ignores running jobs", func(t *testing.T) {
		store, err := mediaSqlite.New(context.Background(), ":memory:")
		require.NoError(t, err)

		err = store.RunMigrations(context.Background())
		require.NoError(t, err)

		ctx := context.Background()
		cfg := config.Manager{
			Jobs: config.Jobs{
				MovieIndex: 1 * time.Millisecond,
			},
		}

		scheduler := NewScheduler(store, cfg, make(map[JobType]JobExecutor))

		oldJobID, err := scheduler.createPendingJob(ctx, MovieIndex)
		require.NoError(t, err)

		err = store.UpdateJobState(ctx, oldJobID, storage.JobStateRunning, nil)
		require.NoError(t, err)

		err = store.UpdateJobState(ctx, oldJobID, storage.JobStateDone, nil)
		require.NoError(t, err)

		currentJobID, err := scheduler.createPendingJob(ctx, MovieIndex)
		require.NoError(t, err)

		err = store.UpdateJobState(ctx, currentJobID, storage.JobStateRunning, nil)
		require.NoError(t, err)

		time.Sleep(2 * time.Millisecond)

		scheduler.checkAndScheduleJob(ctx, MovieIndex)

		jobs, err := scheduler.listPendingJobsByType(ctx, MovieIndex)
		require.NoError(t, err)
		assert.Empty(t, jobs, "should not create a pending job while one is running")
	})
}

func TestScheduler_pruneOldJobs(t *testing.T) {
	t.Run("retains only minimum jobs per type", func(t *testing.T) {
		store, err := mediaSqlite.New(context.Background(), ":memory:")
		require.NoError(t, err)

		err = store.RunMigrations(context.Background())
		require.NoError(t, err)

		ctx := context.Background()
		cfg := config.Manager{
			Jobs: config.Jobs{
				MinJobsToKeep: 2,
			},
		}

		scheduler := NewScheduler(store, cfg, make(map[JobType]JobExecutor))

		for i := 0; i < 5; i++ {
			jobID, err := scheduler.createPendingJob(ctx, MovieIndex)
			require.NoError(t, err)
			err = store.UpdateJobState(ctx, jobID, storage.JobStateRunning, nil)
			require.NoError(t, err)
			err = store.UpdateJobState(ctx, jobID, storage.JobStateDone, nil)
			require.NoError(t, err)
		}

		allJobs, err := store.ListJobs(ctx, 0, 0, storage.JobFilter{})
		require.NoError(t, err)
		assert.Len(t, allJobs, 5)

		scheduler.pruneOldJobs(ctx)

		remainingJobs, err := store.ListJobs(ctx, 0, 0, storage.JobFilter{})
		require.NoError(t, err)
		assert.Equal(t, 2, len(remainingJobs), "should retain exactly minimum jobs")
	})

	t.Run("preserves minimum jobs per type", func(t *testing.T) {
		store, err := mediaSqlite.New(context.Background(), ":memory:")
		require.NoError(t, err)

		err = store.RunMigrations(context.Background())
		require.NoError(t, err)

		ctx := context.Background()
		cfg := config.Manager{
			Jobs: config.Jobs{
				MinJobsToKeep: 1,
			},
		}

		scheduler := NewScheduler(store, cfg, make(map[JobType]JobExecutor))

		jobID, err := scheduler.createPendingJob(ctx, MovieIndex)
		require.NoError(t, err)
		err = store.UpdateJobState(ctx, jobID, storage.JobStateRunning, nil)
		require.NoError(t, err)
		err = store.UpdateJobState(ctx, jobID, storage.JobStateDone, nil)
		require.NoError(t, err)

		scheduler.pruneOldJobs(ctx)

		remainingJobs, err := store.ListJobs(ctx, 0, 0, storage.JobFilter{})
		require.NoError(t, err)
		require.Len(t, remainingJobs, 1)
		assert.Equal(t, jobID, remainingJobs[0].Job.ID)
	})

	t.Run("cleanup disabled with 0 does nothing", func(t *testing.T) {
		store, err := mediaSqlite.New(context.Background(), ":memory:")
		require.NoError(t, err)

		err = store.RunMigrations(context.Background())
		require.NoError(t, err)

		ctx := context.Background()
		cfg := config.Manager{
			Jobs: config.Jobs{
				MinJobsToKeep: 0,
			},
		}

		scheduler := NewScheduler(store, cfg, make(map[JobType]JobExecutor))

		jobID, err := scheduler.createPendingJob(ctx, MovieIndex)
		require.NoError(t, err)
		err = store.UpdateJobState(ctx, jobID, storage.JobStateRunning, nil)
		require.NoError(t, err)
		err = store.UpdateJobState(ctx, jobID, storage.JobStateDone, nil)
		require.NoError(t, err)

		called := false
		origRunPruning := func(_ context.Context) {
			if cfg.Jobs.MinJobsToKeep <= 0 {
				return
			}
			called = true
		}
		origRunPruning(ctx)

		assert.False(t, called, "pruning should not run when min jobs to keep is 0")

		remainingJobs, err := store.ListJobs(ctx, 0, 0, storage.JobFilter{})
		require.NoError(t, err)
		require.Len(t, remainingJobs, 1)
		assert.Equal(t, jobID, remainingJobs[0].Job.ID)
	})
}

func TestScheduler_getIntervalForJobType(t *testing.T) {
	cfg := config.Manager{
		Jobs: config.Jobs{
			MovieIndex:      1 * time.Minute,
			MovieReconcile:  2 * time.Minute,
			SeriesIndex:     3 * time.Minute,
			SeriesReconcile: 4 * time.Minute,
			IndexerSync:     5 * time.Minute,
		},
	}

	scheduler := NewScheduler(nil, cfg, nil)

	tests := []struct {
		jobType  JobType
		expected time.Duration
	}{
		{MovieIndex, 1 * time.Minute},
		{MovieReconcile, 2 * time.Minute},
		{SeriesIndex, 3 * time.Minute},
		{SeriesReconcile, 4 * time.Minute},
		{IndexerSync, 5 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(string(tt.jobType), func(t *testing.T) {
			interval := scheduler.getIntervalForJobType(tt.jobType)
			assert.Equal(t, tt.expected, interval)
		})
	}

	t.Run("unknown job type returns default", func(t *testing.T) {
		interval := scheduler.getIntervalForJobType("UnknownType")
		assert.Equal(t, 10*time.Minute, interval)
	})
}
