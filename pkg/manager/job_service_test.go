package manager

import (
	"context"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/pagination"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobService_GetJob(t *testing.T) {
	t.Run("get job", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)
		js := NewJobService(store, store, store, config.Manager{}, make(map[JobType]JobExecutor))

		id, err := js.scheduler.createPendingJob(ctx, MovieIndex)
		require.Nil(t, err)
		require.NotEqual(t, id, int64(0))

		job, err := js.GetJob(ctx, id)
		require.Nil(t, err)

		job.CreatedAt = time.Time{}
		job.UpdatedAt = time.Time{}

		assert.Equal(t, JobResponse{
			ID:    1,
			Type:  string(MovieIndex),
			State: string(storage.JobStatePending),
			Error: nil,
		}, job)
	})

	t.Run("job not found", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)
		js := NewJobService(store, store, store, config.Manager{}, make(map[JobType]JobExecutor))

		_, err := js.GetJob(ctx, 999)
		require.Error(t, err)
		assert.Equal(t, storage.ErrNotFound, err)
	})
}

func TestJobService_CreateJob(t *testing.T) {
	t.Run("create job successfully", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)
		js := NewJobService(store, store, store, config.Manager{}, make(map[JobType]JobExecutor))

		req := TriggerJobRequest{Type: string(MovieIndex)}
		job, err := js.CreateJob(ctx, req)
		require.NoError(t, err)

		assert.Equal(t, int64(1), job.ID)
		assert.Equal(t, string(MovieIndex), job.Type)
		assert.Equal(t, string(storage.JobStatePending), job.State)
		assert.Nil(t, job.Error)

		retrievedJob, err := js.GetJob(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, job.ID, retrievedJob.ID)
		assert.Equal(t, job.Type, retrievedJob.Type)
		assert.Equal(t, job.State, retrievedJob.State)
	})

	t.Run("create job when pending already exists", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)
		js := NewJobService(store, store, store, config.Manager{}, make(map[JobType]JobExecutor))

		req := TriggerJobRequest{Type: string(MovieReconcile)}
		job1, err := js.CreateJob(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, int64(1), job1.ID)

		job2, err := js.CreateJob(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, job1.ID, job2.ID)
		assert.Equal(t, string(MovieReconcile), job2.Type)

		listResult, err := js.ListJobs(ctx, nil, nil, pagination.Params{Page: 1, PageSize: 0})
		require.NoError(t, err)
		assert.Equal(t, 1, listResult.Count)
		assert.Equal(t, job1.ID, listResult.Jobs[0].ID)
	})

	t.Run("create multiple different job types", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)
		js := NewJobService(store, store, store, config.Manager{}, make(map[JobType]JobExecutor))

		movieJob, err := js.CreateJob(ctx, TriggerJobRequest{Type: string(MovieIndex)})
		require.NoError(t, err)
		assert.Equal(t, string(MovieIndex), movieJob.Type)

		seriesJob, err := js.CreateJob(ctx, TriggerJobRequest{Type: string(SeriesIndex)})
		require.NoError(t, err)
		assert.Equal(t, string(SeriesIndex), seriesJob.Type)
		assert.NotEqual(t, movieJob.ID, seriesJob.ID)

		listResult, err := js.ListJobs(ctx, nil, nil, pagination.Params{Page: 1, PageSize: 0})
		require.NoError(t, err)
		assert.Equal(t, 2, listResult.Count)

		movieJobFound := false
		seriesJobFound := false
		for _, job := range listResult.Jobs {
			if job.ID == movieJob.ID {
				movieJobFound = true
				assert.Equal(t, string(MovieIndex), job.Type)
			}
			if job.ID == seriesJob.ID {
				seriesJobFound = true
				assert.Equal(t, string(SeriesIndex), job.Type)
			}
		}
		assert.True(t, movieJobFound, "movie job should be in list")
		assert.True(t, seriesJobFound, "series job should be in list")
	})
}

func TestJobService_ListJobs(t *testing.T) {
	t.Run("list all jobs", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)
		js := NewJobService(store, store, store, config.Manager{}, make(map[JobType]JobExecutor))

		_, err := js.CreateJob(ctx, TriggerJobRequest{Type: string(MovieIndex)})
		require.NoError(t, err)
		_, err = js.CreateJob(ctx, TriggerJobRequest{Type: string(SeriesIndex)})
		require.NoError(t, err)

		result, err := js.ListJobs(ctx, nil, nil, pagination.Params{Page: 1, PageSize: 0})
		require.NoError(t, err)
		assert.Equal(t, 2, result.Count)
		assert.Len(t, result.Jobs, 2)
	})

	t.Run("list jobs filtered by type", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)
		js := NewJobService(store, store, store, config.Manager{}, make(map[JobType]JobExecutor))

		_, err := js.CreateJob(ctx, TriggerJobRequest{Type: string(MovieIndex)})
		require.NoError(t, err)
		_, err = js.CreateJob(ctx, TriggerJobRequest{Type: string(SeriesIndex)})
		require.NoError(t, err)
		_, err = js.CreateJob(ctx, TriggerJobRequest{Type: string(MovieReconcile)})
		require.NoError(t, err)

		jobType := string(MovieIndex)
		result, err := js.ListJobs(ctx, &jobType, nil, pagination.Params{Page: 1, PageSize: 0})
		require.NoError(t, err)
		assert.Equal(t, 1, result.Count)
		assert.Len(t, result.Jobs, 1)
		assert.Equal(t, string(MovieIndex), result.Jobs[0].Type)
	})

	t.Run("list jobs filtered by state", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)
		js := NewJobService(store, store, store, config.Manager{}, make(map[JobType]JobExecutor))

		id1, err := js.CreateJob(ctx, TriggerJobRequest{Type: string(MovieIndex)})
		require.NoError(t, err)
		_, err = js.CreateJob(ctx, TriggerJobRequest{Type: string(SeriesIndex)})
		require.NoError(t, err)

		err = store.UpdateJobState(ctx, id1.ID, storage.JobStateRunning, nil)
		require.NoError(t, err)
		err = store.UpdateJobState(ctx, id1.ID, storage.JobStateDone, nil)
		require.NoError(t, err)

		state := string(storage.JobStatePending)
		result, err := js.ListJobs(ctx, nil, &state, pagination.Params{Page: 1, PageSize: 0})
		require.NoError(t, err)
		assert.Equal(t, 1, result.Count)
		assert.Len(t, result.Jobs, 1)
		assert.Equal(t, string(storage.JobStatePending), result.Jobs[0].State)
	})

	t.Run("list jobs filtered by type and state", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)
		js := NewJobService(store, store, store, config.Manager{}, make(map[JobType]JobExecutor))

		id1, err := js.CreateJob(ctx, TriggerJobRequest{Type: string(MovieIndex)})
		require.NoError(t, err)
		_, err = js.CreateJob(ctx, TriggerJobRequest{Type: string(SeriesIndex)})
		require.NoError(t, err)
		_, err = js.CreateJob(ctx, TriggerJobRequest{Type: string(MovieReconcile)})
		require.NoError(t, err)

		err = store.UpdateJobState(ctx, id1.ID, storage.JobStateRunning, nil)
		require.NoError(t, err)
		err = store.UpdateJobState(ctx, id1.ID, storage.JobStateDone, nil)
		require.NoError(t, err)

		jobType := string(MovieIndex)
		state := string(storage.JobStateDone)
		result, err := js.ListJobs(ctx, &jobType, &state, pagination.Params{Page: 1, PageSize: 0})
		require.NoError(t, err)
		assert.Equal(t, 1, result.Count)
		assert.Len(t, result.Jobs, 1)
		assert.Equal(t, string(MovieIndex), result.Jobs[0].Type)
		assert.Equal(t, string(storage.JobStateDone), result.Jobs[0].State)
	})

	t.Run("list jobs with no results", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)
		js := NewJobService(store, store, store, config.Manager{}, make(map[JobType]JobExecutor))

		result, err := js.ListJobs(ctx, nil, nil, pagination.Params{Page: 1, PageSize: 0})
		require.NoError(t, err)
		assert.Equal(t, 0, result.Count)
		assert.Empty(t, result.Jobs)
	})

	t.Run("invalid job type", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)
		js := NewJobService(store, store, store, config.Manager{}, make(map[JobType]JobExecutor))

		invalidType := "InvalidJobType"
		_, err := js.ListJobs(ctx, &invalidType, nil, pagination.Params{Page: 1, PageSize: 0})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid job type")
	})

	t.Run("invalid job state", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)
		js := NewJobService(store, store, store, config.Manager{}, make(map[JobType]JobExecutor))

		invalidState := "InvalidState"
		_, err := js.ListJobs(ctx, nil, &invalidState, pagination.Params{Page: 1, PageSize: 0})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid job state")
	})
}

func TestJobService_CancelJob(t *testing.T) {
	t.Run("cancel running job", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)

		jobExecuted := false
		jobCancelled := false
		executors := map[JobType]JobExecutor{
			MovieIndex: func(ctx context.Context, jobID int64) error {
				jobExecuted = true
				<-ctx.Done()
				jobCancelled = true
				return ctx.Err()
			},
		}
		js := NewJobService(store, store, store, config.Manager{}, executors)

		job, err := js.CreateJob(ctx, TriggerJobRequest{Type: string(MovieIndex)})
		require.NoError(t, err)

		go js.scheduler.executeJob(ctx, &storage.Job{
			Job: model.Job{
				ID:   int32(job.ID),
				Type: job.Type,
			},
			State: storage.JobStatePending,
		})

		time.Sleep(100 * time.Millisecond)
		require.True(t, jobExecuted, "job should have started executing")

		result, err := js.CancelJob(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, job.ID, result.ID)

		time.Sleep(200 * time.Millisecond)
		require.True(t, jobCancelled, "job should have been cancelled")

		cancelledJob, err := store.GetJob(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, storage.JobStateCancelled, cancelledJob.State)

		retrievedJob, err := js.GetJob(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, string(storage.JobStateCancelled), retrievedJob.State)
	})

	t.Run("cancel pending job", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)
		js := NewJobService(store, store, store, config.Manager{}, make(map[JobType]JobExecutor))

		job, err := js.CreateJob(ctx, TriggerJobRequest{Type: string(MovieIndex)})
		require.NoError(t, err)

		result, err := js.CancelJob(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, job.ID, result.ID)

		cancelledJob, err := store.GetJob(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, storage.JobStateCancelled, cancelledJob.State)

		retrievedJob, err := js.GetJob(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, string(storage.JobStateCancelled), retrievedJob.State)
	})

	t.Run("cancel completed job", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)
		js := NewJobService(store, store, store, config.Manager{}, make(map[JobType]JobExecutor))

		job, err := js.CreateJob(ctx, TriggerJobRequest{Type: string(MovieIndex)})
		require.NoError(t, err)

		err = store.UpdateJobState(ctx, job.ID, storage.JobStateRunning, nil)
		require.NoError(t, err)
		err = store.UpdateJobState(ctx, job.ID, storage.JobStateDone, nil)
		require.NoError(t, err)

		result, err := js.CancelJob(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, job.ID, result.ID)

		doneJob, err := store.GetJob(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, storage.JobStateDone, doneJob.State)

		retrievedJob, err := js.GetJob(ctx, job.ID)
		require.NoError(t, err)
		assert.Equal(t, string(storage.JobStateDone), retrievedJob.State)
	})

	t.Run("cancel non-existent job", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)
		js := NewJobService(store, store, store, config.Manager{}, make(map[JobType]JobExecutor))

		_, err := js.CancelJob(ctx, 999)
		require.Error(t, err)
		assert.Equal(t, storage.ErrNotFound, err)
	})
}
