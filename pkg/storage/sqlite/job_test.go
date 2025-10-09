package sqlite_test

import (
	"context"
	"testing"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initTestDB(t *testing.T) storage.Storage {
	store, err := sqlite.New(":memory:")
	require.NoError(t, err)

	schemas, err := storage.ReadSchemaFiles("./schema/schema.sql")
	require.NoError(t, err)

	err = store.Init(context.Background(), schemas...)
	require.NoError(t, err)

	return store
}

func TestSQLite_CreateJob(t *testing.T) {
	t.Run("create with valid initial state", func(t *testing.T) {
		ctx := context.Background()
		store := initTestDB(t)

		job := storage.Job{
			Job: model.Job{
				Type: "test-job",
			},
		}

		id, err := store.CreateJob(ctx, job, storage.JobStatePending)
		assert.NoError(t, err)
		assert.NotZero(t, id)

		got, err := store.GetJob(ctx, id)
		require.NoError(t, err)

		want := &storage.Job{
			Job: model.Job{
				ID:         int32(id),
				Type:       "test-job",
				ToState:    string(storage.JobStatePending),
				MostRecent: true,
				SortKey:    1,
			},
			State: storage.JobStatePending,
		}

		got.CreatedAt = nil
		got.UpdatedAt = nil
		assert.Equal(t, want, got)
	})

	t.Run("invalid state transition", func(t *testing.T) {
		ctx := context.Background()
		store := initTestDB(t)

		job := storage.Job{
			Job: model.Job{
				Type: "invalid-job",
			},
		}

		_, err := store.CreateJob(ctx, job, storage.JobStateDone)
		assert.Error(t, err)
	})
}

func TestSQLite_GetJob(t *testing.T) {
	t.Run("get existing job", func(t *testing.T) {
		ctx := context.Background()
		store := initTestDB(t)

		job := storage.Job{
			Job: model.Job{
				Type: "get-job",
			},
		}

		id, err := store.CreateJob(ctx, job, storage.JobStatePending)
		require.NoError(t, err)

		got, err := store.GetJob(ctx, id)
		assert.NoError(t, err)

		want := &storage.Job{
			Job: model.Job{
				ID:         int32(id),
				Type:       "get-job",
				ToState:    string(storage.JobStatePending),
				MostRecent: true,
				SortKey:    1,
			},
			State: storage.JobStatePending,
		}

		got.CreatedAt = nil
		got.UpdatedAt = nil
		assert.Equal(t, want, got)
	})

	t.Run("get non-existent job", func(t *testing.T) {
		ctx := context.Background()
		store := initTestDB(t)

		_, err := store.GetJob(ctx, 999999)
		assert.ErrorIs(t, err, storage.ErrNotFound)
	})
}

func TestSQLite_ListJobs(t *testing.T) {
	t.Run("list all jobs", func(t *testing.T) {
		ctx := context.Background()
		store := initTestDB(t)

		job1 := storage.Job{
			Job: model.Job{
				Type: "job-1",
			},
		}
		id1, err := store.CreateJob(ctx, job1, storage.JobStatePending)
		require.NoError(t, err)

		job2 := storage.Job{
			Job: model.Job{
				Type: "job-2",
			},
		}
		id2, err := store.CreateJob(ctx, job2, storage.JobStateRunning)
		require.NoError(t, err)

		got, err := store.ListJobs(ctx)
		assert.NoError(t, err)

		want := []*storage.Job{
			{
				Job: model.Job{
					ID:         int32(id1),
					Type:       "job-1",
					ToState:    string(storage.JobStatePending),
					MostRecent: true,
					SortKey:    1,
				},
				State: storage.JobStatePending,
			},
			{
				Job: model.Job{
					ID:         int32(id2),
					Type:       "job-2",
					ToState:    string(storage.JobStateRunning),
					MostRecent: true,
					SortKey:    1,
				},
				State: storage.JobStateRunning,
			},
		}

		for i := range got {
			got[i].CreatedAt = nil
			got[i].UpdatedAt = nil
		}
		assert.Equal(t, want, got)
	})

	t.Run("list empty", func(t *testing.T) {
		ctx := context.Background()
		store := initTestDB(t)

		got, err := store.ListJobs(ctx)
		assert.NoError(t, err)

		want := []*storage.Job{}
		assert.Equal(t, want, got)
	})
}

func TestSQLite_ListJobsByState(t *testing.T) {
	t.Run("list pending jobs", func(t *testing.T) {
		ctx := context.Background()
		store := initTestDB(t)

		pendingJob := storage.Job{
			Job: model.Job{
				Type: "pending-job",
			},
		}
		id, err := store.CreateJob(ctx, pendingJob, storage.JobStatePending)
		require.NoError(t, err)

		runningJob := storage.Job{
			Job: model.Job{
				Type: "running-job",
			},
		}
		_, err = store.CreateJob(ctx, runningJob, storage.JobStateRunning)
		require.NoError(t, err)

		got, err := store.ListJobsByState(ctx, storage.JobStatePending)
		assert.NoError(t, err)

		want := []*storage.Job{
			{
				Job: model.Job{
					ID:         int32(id),
					Type:       "pending-job",
					ToState:    string(storage.JobStatePending),
					MostRecent: true,
					SortKey:    1,
				},
				State: storage.JobStatePending,
			},
		}

		for i := range got {
			got[i].CreatedAt = nil
			got[i].UpdatedAt = nil
		}
		assert.Equal(t, want, got)
	})

	t.Run("list running jobs", func(t *testing.T) {
		ctx := context.Background()
		store := initTestDB(t)

		runningJob := storage.Job{
			Job: model.Job{
				Type: "running-job",
			},
		}
		id, err := store.CreateJob(ctx, runningJob, storage.JobStateRunning)
		require.NoError(t, err)

		got, err := store.ListJobsByState(ctx, storage.JobStateRunning)
		assert.NoError(t, err)

		want := []*storage.Job{
			{
				Job: model.Job{
					ID:         int32(id),
					Type:       "running-job",
					ToState:    string(storage.JobStateRunning),
					MostRecent: true,
					SortKey:    1,
				},
				State: storage.JobStateRunning,
			},
		}

		for i := range got {
			got[i].CreatedAt = nil
			got[i].UpdatedAt = nil
		}
		assert.Equal(t, want, got)
	})
}

func TestSQLite_UpdateJobState(t *testing.T) {
	t.Run("update pending to running", func(t *testing.T) {
		ctx := context.Background()
		store := initTestDB(t)

		job := storage.Job{
			Job: model.Job{
				Type: "update-job",
			},
		}

		id, err := store.CreateJob(ctx, job, storage.JobStatePending)
		require.NoError(t, err)

		err = store.UpdateJobState(ctx, id, storage.JobStateRunning, nil)
		assert.NoError(t, err)

		got, err := store.GetJob(ctx, id)
		assert.NoError(t, err)

		fromState := string(storage.JobStatePending)
		want := &storage.Job{
			Job: model.Job{
				ID:         int32(id),
				Type:       "update-job",
				ToState:    string(storage.JobStateRunning),
				FromState:  &fromState,
				MostRecent: true,
				SortKey:    2,
			},
			State: storage.JobStateRunning,
		}

		got.CreatedAt = nil
		got.UpdatedAt = nil
		assert.Equal(t, want, got)
	})

	t.Run("update running to done", func(t *testing.T) {
		ctx := context.Background()
		store := initTestDB(t)

		job := storage.Job{
			Job: model.Job{
				Type: "done-job",
			},
		}

		id, err := store.CreateJob(ctx, job, storage.JobStatePending)
		require.NoError(t, err)

		err = store.UpdateJobState(ctx, id, storage.JobStateRunning, nil)
		require.NoError(t, err)

		err = store.UpdateJobState(ctx, id, storage.JobStateDone, nil)
		assert.NoError(t, err)

		got, err := store.GetJob(ctx, id)
		assert.NoError(t, err)

		fromState := string(storage.JobStateRunning)
		want := &storage.Job{
			Job: model.Job{
				ID:         int32(id),
				Type:       "done-job",
				ToState:    string(storage.JobStateDone),
				FromState:  &fromState,
				MostRecent: true,
				SortKey:    3,
			},
			State: storage.JobStateDone,
		}

		got.CreatedAt = nil
		got.UpdatedAt = nil
		assert.Equal(t, want, got)
	})

	t.Run("update running to error with message", func(t *testing.T) {
		ctx := context.Background()
		store := initTestDB(t)

		job := storage.Job{
			Job: model.Job{
				Type: "error-job",
			},
		}

		id, err := store.CreateJob(ctx, job, storage.JobStatePending)
		require.NoError(t, err)

		err = store.UpdateJobState(ctx, id, storage.JobStateRunning, nil)
		require.NoError(t, err)

		errorMsg := "Something went wrong"
		err = store.UpdateJobState(ctx, id, storage.JobStateError, &errorMsg)
		assert.NoError(t, err)

		got, err := store.GetJob(ctx, id)
		assert.NoError(t, err)

		fromState := string(storage.JobStateRunning)
		want := &storage.Job{
			Job: model.Job{
				ID:         int32(id),
				Type:       "error-job",
				ToState:    string(storage.JobStateError),
				FromState:  &fromState,
				MostRecent: true,
				SortKey:    3,
				Error:      &errorMsg,
			},
			State: storage.JobStateError,
		}

		got.CreatedAt = nil
		got.UpdatedAt = nil
		assert.Equal(t, want, got)
	})

	t.Run("invalid state transition", func(t *testing.T) {
		ctx := context.Background()
		store := initTestDB(t)

		job := storage.Job{
			Job: model.Job{
				Type: "invalid-transition",
			},
		}

		id, err := store.CreateJob(ctx, job, storage.JobStatePending)
		require.NoError(t, err)

		err = store.UpdateJobState(ctx, id, storage.JobStateDone, nil)
		assert.Error(t, err)

		got, err := store.GetJob(ctx, id)
		assert.NoError(t, err)

		want := &storage.Job{
			Job: model.Job{
				ID:         int32(id),
				Type:       "invalid-transition",
				ToState:    string(storage.JobStatePending),
				MostRecent: true,
				SortKey:    1,
			},
			State: storage.JobStatePending,
		}

		got.CreatedAt = nil
		got.UpdatedAt = nil
		assert.Equal(t, want, got)
	})
}

func TestSQLite_DeleteJob(t *testing.T) {
	t.Run("delete existing job", func(t *testing.T) {
		ctx := context.Background()
		store := initTestDB(t)

		job := storage.Job{
			Job: model.Job{
				Type: "delete-job",
			},
		}

		id, err := store.CreateJob(ctx, job, storage.JobStatePending)
		require.NoError(t, err)

		_, err = store.GetJob(ctx, id)
		assert.NoError(t, err)

		err = store.DeleteJob(ctx, id)
		assert.NoError(t, err)

		_, err = store.GetJob(ctx, id)
		assert.ErrorIs(t, err, storage.ErrNotFound)
	})
}
