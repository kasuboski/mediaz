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
	store, err := sqlite.New(t.Context(), ":memory:")
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

		assert.Equal(t, int32(id), got.ID)
		assert.Equal(t, "test-job", got.Type)
		assert.Equal(t, storage.JobStatePending, got.State)
		assert.NotNil(t, got.CreatedAt)
		assert.NotNil(t, got.UpdatedAt)
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

	t.Run("prevent duplicate pending jobs", func(t *testing.T) {
		ctx := context.Background()
		store := initTestDB(t)

		job := storage.Job{
			Job: model.Job{
				Type: "duplicate-test",
			},
		}

		id1, err := store.CreateJob(ctx, job, storage.JobStatePending)
		assert.NoError(t, err)
		assert.NotZero(t, id1)

		_, err = store.CreateJob(ctx, job, storage.JobStatePending)
		assert.ErrorIs(t, err, storage.ErrJobAlreadyPending)

		got, err := store.GetJob(ctx, id1)
		assert.NoError(t, err)
		assert.Equal(t, "duplicate-test", got.Type)
		assert.Equal(t, storage.JobStatePending, got.State)
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

		assert.Equal(t, int32(id), got.ID)
		assert.Equal(t, "get-job", got.Type)
		assert.Equal(t, storage.JobStatePending, got.State)
		assert.NotNil(t, got.CreatedAt)
		assert.NotNil(t, got.UpdatedAt)
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

		job := storage.Job{
			Job: model.Job{
				Type: "list-test",
			},
		}
		id, err := store.CreateJob(ctx, job, storage.JobStatePending)
		require.NoError(t, err)

		got, err := store.ListJobs(ctx, 0, 0)
		assert.NoError(t, err)
		require.Len(t, got, 1)

		assert.Equal(t, int32(id), got[0].ID)
		assert.Equal(t, "list-test", got[0].Type)
		assert.Equal(t, storage.JobStatePending, got[0].State)
		assert.NotNil(t, got[0].CreatedAt)
		assert.NotNil(t, got[0].UpdatedAt)
	})

	t.Run("list empty", func(t *testing.T) {
		ctx := context.Background()
		store := initTestDB(t)

		got, err := store.ListJobs(ctx, 0, 0)
		assert.NoError(t, err)

		want := []*storage.Job{}
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

		assert.Equal(t, int32(id), got.ID)
		assert.Equal(t, "update-job", got.Type)
		assert.Equal(t, storage.JobStateRunning, got.State)
		assert.NotNil(t, got.CreatedAt)
		assert.NotNil(t, got.UpdatedAt)
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

		assert.Equal(t, int32(id), got.ID)
		assert.Equal(t, "done-job", got.Type)
		assert.Equal(t, storage.JobStateDone, got.State)
		assert.NotNil(t, got.CreatedAt)
		assert.NotNil(t, got.UpdatedAt)
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

		assert.Equal(t, int32(id), got.ID)
		assert.Equal(t, "error-job", got.Type)
		assert.Equal(t, storage.JobStateError, got.State)
		assert.NotNil(t, got.Error)
		assert.Equal(t, errorMsg, *got.Error)
		assert.NotNil(t, got.CreatedAt)
		assert.NotNil(t, got.UpdatedAt)
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

		assert.Equal(t, int32(id), got.ID)
		assert.Equal(t, "invalid-transition", got.Type)
		assert.Equal(t, storage.JobStatePending, got.State)
		assert.NotNil(t, got.CreatedAt)
		assert.NotNil(t, got.UpdatedAt)
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
