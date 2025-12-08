package manager

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/storage"
	mediaSqlite "github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScheduler_createPendingJob(t *testing.T) {
	t.Run("invalid job type", func(t *testing.T) {
		store, err := mediaSqlite.New(":memory:")
		require.NoError(t, err)

		scheduler := NewScheduler(store, config.Manager{}, make(map[JobType]JobExecutor))
		id, err := scheduler.createPendingJob(context.TODO(), "my-fake-job")

		assert.Equal(t, id, int64(0))
		require.NotNil(t, err)
		assert.Equal(t, err.Error(), "invalid job type")
	})

	t.Run("create job and duplicate pending job", func(t *testing.T) {
		store, err := mediaSqlite.New(":memory:")
		require.NoError(t, err)

		schemas, err := storage.ReadSchemaFiles("../storage/sqlite/schema/schema.sql", "../storage/sqlite/schema/defaults.sql")
		require.NoError(t, err)

		ctx := context.Background()
		err = store.Init(ctx, schemas...)
		require.NoError(t, err)

		scheduler := NewScheduler(store, config.Manager{}, make(map[JobType]JobExecutor))

		id, err := scheduler.createPendingJob(ctx, MovieIndex)
		require.Nil(t, err)
		assert.NotEqual(t, int64(0), id)

		jobs, err := scheduler.listPendingJobs(ctx)
		require.Nil(t, err)
		require.Len(t, jobs, 1)

		job := jobs[0]
		assert.Equal(t, job.ID, int32(id))
		assert.Equal(t, job.Type, string(MovieIndex))
		assert.NotNil(t, job.CreatedAt)

		jobs, err = scheduler.listPendingJobsByType(ctx, MovieIndex)
		require.Nil(t, err)

		job = jobs[0]
		assert.Equal(t, job.ID, int32(id))
		assert.Equal(t, job.Type, string(MovieIndex))
		assert.NotNil(t, job.CreatedAt)

		id, err = scheduler.createPendingJob(ctx, MovieIndex)
		require.NotNil(t, err)
		require.Equal(t, id, int64(0))
	})
}

func TestScheduler_listPendingJobs(t *testing.T) {
	t.Run("no pending jobs", func(t *testing.T) {
		store, err := mediaSqlite.New(":memory:")
		require.NoError(t, err)

		schemas, err := storage.ReadSchemaFiles("../storage/sqlite/schema/schema.sql", "../storage/sqlite/schema/defaults.sql")
		require.NoError(t, err)

		ctx := context.Background()
		err = store.Init(ctx, schemas...)
		require.NoError(t, err)

		scheduler := NewScheduler(store, config.Manager{}, make(map[JobType]JobExecutor))

		jobs, err := scheduler.listPendingJobs(ctx)
		require.NoError(t, err)
		assert.Len(t, jobs, 0)
	})

	t.Run("list multiple pending jobs", func(t *testing.T) {
		store, err := mediaSqlite.New(":memory:")
		require.NoError(t, err)

		schemas, err := storage.ReadSchemaFiles("../storage/sqlite/schema/schema.sql", "../storage/sqlite/schema/defaults.sql")
		require.NoError(t, err)

		ctx := context.Background()
		err = store.Init(ctx, schemas...)
		require.NoError(t, err)

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

		assert.Equal(t, int64(jobs[0].ID), id2)
		assert.Equal(t, jobs[0].Type, string(SeriesIndex))
		assert.NotNil(t, jobs[0].CreatedAt)
	})
}

func TestScheduler_listPendingJobsByType(t *testing.T) {
	t.Run("no pending jobs of type", func(t *testing.T) {
		store, err := mediaSqlite.New(":memory:")
		require.NoError(t, err)

		schemas, err := storage.ReadSchemaFiles("../storage/sqlite/schema/schema.sql", "../storage/sqlite/schema/defaults.sql")
		require.NoError(t, err)

		ctx := context.Background()
		err = store.Init(ctx, schemas...)
		require.NoError(t, err)

		scheduler := NewScheduler(store, config.Manager{}, make(map[JobType]JobExecutor))

		jobs, err := scheduler.listPendingJobsByType(ctx, MovieIndex)
		require.NoError(t, err)
		assert.Len(t, jobs, 0)
	})

	t.Run("list only jobs of specific type", func(t *testing.T) {
		store, err := mediaSqlite.New(":memory:")
		require.NoError(t, err)

		schemas, err := storage.ReadSchemaFiles("../storage/sqlite/schema/schema.sql", "../storage/sqlite/schema/defaults.sql")
		require.NoError(t, err)

		ctx := context.Background()
		err = store.Init(ctx, schemas...)
		require.NoError(t, err)

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

		assert.Equal(t, int64(jobs[0].ID), seriesID)
		assert.Equal(t, jobs[0].Type, string(SeriesIndex))
		assert.NotNil(t, jobs[0].CreatedAt)
	})
}

func TestScheduler_executeJob(t *testing.T) {
	t.Run("successful job execution", func(t *testing.T) {
		store, err := mediaSqlite.New(":memory:")
		require.NoError(t, err)

		schemas, err := storage.ReadSchemaFiles("../storage/sqlite/schema/schema.sql", "../storage/sqlite/schema/defaults.sql")
		require.NoError(t, err)

		ctx := context.Background()
		err = store.Init(ctx, schemas...)
		require.NoError(t, err)

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
		require.Equal(t, storage.JobStatePending, job.State)

		scheduler.executeJob(ctx, job)

		assert.True(t, executorCalled)

		updatedJob, err := store.GetJob(ctx, jobID)
		require.NoError(t, err)
		assert.Equal(t, storage.JobStateDone, updatedJob.State)
		assert.Nil(t, updatedJob.Error)

		_, inCache := scheduler.runningJobs.Get(jobID)
		assert.False(t, inCache, "job should be removed from cache after completion")
	})

	t.Run("no executor found for job type", func(t *testing.T) {
		store, err := mediaSqlite.New(":memory:")
		require.NoError(t, err)

		schemas, err := storage.ReadSchemaFiles("../storage/sqlite/schema/schema.sql", "../storage/sqlite/schema/defaults.sql")
		require.NoError(t, err)

		ctx := context.Background()
		err = store.Init(ctx, schemas...)
		require.NoError(t, err)

		scheduler := NewScheduler(store, config.Manager{}, make(map[JobType]JobExecutor))

		jobID, err := scheduler.createPendingJob(ctx, MovieIndex)
		require.NoError(t, err)

		job, err := store.GetJob(ctx, jobID)
		require.NoError(t, err)

		scheduler.executeJob(ctx, job)

		updatedJob, err := store.GetJob(ctx, jobID)
		require.NoError(t, err)
		assert.Equal(t, storage.JobStateError, updatedJob.State)
		assert.NotNil(t, updatedJob.Error)
		assert.Equal(t, "no executor found for job type", *updatedJob.Error)

		_, inCache := scheduler.runningJobs.Get(jobID)
		assert.False(t, inCache, "job should be removed from cache after error")
	})

	t.Run("executor returns error", func(t *testing.T) {
		store, err := mediaSqlite.New(":memory:")
		require.NoError(t, err)

		schemas, err := storage.ReadSchemaFiles("../storage/sqlite/schema/schema.sql", "../storage/sqlite/schema/defaults.sql")
		require.NoError(t, err)

		ctx := context.Background()
		err = store.Init(ctx, schemas...)
		require.NoError(t, err)

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

		scheduler.executeJob(ctx, job)

		updatedJob, err := store.GetJob(ctx, jobID)
		require.NoError(t, err)
		assert.Equal(t, storage.JobStateError, updatedJob.State)
		assert.NotNil(t, updatedJob.Error)
		assert.Equal(t, testError, *updatedJob.Error)

		_, inCache := scheduler.runningJobs.Get(jobID)
		assert.False(t, inCache, "job should be removed from cache after error")
	})

	t.Run("job added to and removed from running cache", func(t *testing.T) {
		store, err := mediaSqlite.New(":memory:")
		require.NoError(t, err)

		schemas, err := storage.ReadSchemaFiles("../storage/sqlite/schema/schema.sql", "../storage/sqlite/schema/defaults.sql")
		require.NoError(t, err)

		ctx := context.Background()
		err = store.Init(ctx, schemas...)
		require.NoError(t, err)

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

		go scheduler.executeJob(jobCtx, job)

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
	t.Run("cancel running job", func(t *testing.T) {
		store, err := mediaSqlite.New(":memory:")
		require.NoError(t, err)

		schemas, err := storage.ReadSchemaFiles("../storage/sqlite/schema/schema.sql", "../storage/sqlite/schema/defaults.sql")
		require.NoError(t, err)

		ctx := context.Background()
		err = store.Init(ctx, schemas...)
		require.NoError(t, err)

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

		go scheduler.executeJob(ctx, job)

		<-jobExecuting

		_, inCache := scheduler.runningJobs.Get(jobID)
		assert.True(t, inCache, "job should be in cache while running")

		err = scheduler.CancelJob(ctx, jobID)
		require.NoError(t, err)

		_, inCache = scheduler.runningJobs.Get(jobID)
		assert.False(t, inCache, "job should be removed from cache after cancellation")

		updatedJob, err := store.GetJob(ctx, jobID)
		require.NoError(t, err)
		assert.Equal(t, storage.JobStateCancelled, updatedJob.State)
	})

	t.Run("cancel non-running job", func(t *testing.T) {
		store, err := mediaSqlite.New(":memory:")
		require.NoError(t, err)

		schemas, err := storage.ReadSchemaFiles("../storage/sqlite/schema/schema.sql", "../storage/sqlite/schema/defaults.sql")
		require.NoError(t, err)

		ctx := context.Background()
		err = store.Init(ctx, schemas...)
		require.NoError(t, err)

		scheduler := NewScheduler(store, config.Manager{}, make(map[JobType]JobExecutor))

		jobID, err := scheduler.createPendingJob(ctx, MovieIndex)
		require.NoError(t, err)

		err = scheduler.CancelJob(ctx, jobID)
		require.NoError(t, err)

		job, err := store.GetJob(ctx, jobID)
		require.NoError(t, err)
		assert.Equal(t, storage.JobStatePending, job.State)
	})

	t.Run("cancel non-existent job", func(t *testing.T) {
		store, err := mediaSqlite.New(":memory:")
		require.NoError(t, err)

		schemas, err := storage.ReadSchemaFiles("../storage/sqlite/schema/schema.sql", "../storage/sqlite/schema/defaults.sql")
		require.NoError(t, err)

		ctx := context.Background()
		err = store.Init(ctx, schemas...)
		require.NoError(t, err)

		scheduler := NewScheduler(store, config.Manager{}, make(map[JobType]JobExecutor))

		err = scheduler.CancelJob(ctx, 9999)
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
		job := &storage.Job{
			Job: model.Job{
				ID:        1,
				Type:      string(MovieIndex),
				CreatedAt: &now,
			},
			State:     storage.JobStateDone,
			UpdatedAt: &now,
			Error:     nil,
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
		job := &storage.Job{
			Job: model.Job{
				ID:        2,
				Type:      string(SeriesIndex),
				CreatedAt: &now,
			},
			State:     storage.JobStateError,
			UpdatedAt: &now,
			Error:     &errorMsg,
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
