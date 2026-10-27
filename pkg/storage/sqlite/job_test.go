package sqlite_test

import (
	"context"
	"testing"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initTestDB(t *testing.T) storage.Storage {
	store, err := sqlite.New(t.Context(), ":memory:")
	require.NoError(t, err)

	return store
}

func TestSQLite_CreateJob(t *testing.T) {
	t.Run("create with valid initial state", func(t *testing.T) {
		ctx := context.Background()
		store := initTestDB(t)

		job := sqlcdb.Job{
			Type: "test-job",
		}

		id, err := store.CreateJob(ctx, job, storage.JobStatePending)
		assert.NoError(t, err)
		assert.NotZero(t, id)

		got, err := store.GetJob(ctx, id)
		require.NoError(t, err)

		assert.Equal(t, int64(id), got.Job.ID)
		assert.Equal(t, "test-job", got.Job.Type)
		assert.Equal(t, string(storage.JobStatePending), got.State)
		assert.True(t, got.Job.CreatedAt.Valid)
		assert.True(t, got.UpdatedAt.Valid)
	})

	t.Run("invalid state transition", func(t *testing.T) {
		ctx := context.Background()
		store := initTestDB(t)

		job := sqlcdb.Job{
			Type: "invalid-job",
		}

		_, err := store.CreateJob(ctx, job, storage.JobStateDone)
		assert.Error(t, err)
	})

	t.Run("prevent duplicate pending jobs", func(t *testing.T) {
		ctx := context.Background()
		store := initTestDB(t)

		job := sqlcdb.Job{
			Type: "duplicate-test",
		}

		id1, err := store.CreateJob(ctx, job, storage.JobStatePending)
		assert.NoError(t, err)
		assert.NotZero(t, id1)

		_, err = store.CreateJob(ctx, job, storage.JobStatePending)
		assert.ErrorIs(t, err, storage.ErrJobAlreadyPending)

		got, err := store.GetJob(ctx, id1)
		assert.NoError(t, err)
		assert.Equal(t, "duplicate-test", got.Job.Type)
		assert.Equal(t, string(storage.JobStatePending), got.State)
	})
}

func TestSQLite_GetJob(t *testing.T) {
	t.Run("get existing job", func(t *testing.T) {
		ctx := context.Background()
		store := initTestDB(t)

		job := sqlcdb.Job{
			Type: "get-job",
		}

		id, err := store.CreateJob(ctx, job, storage.JobStatePending)
		require.NoError(t, err)

		got, err := store.GetJob(ctx, id)
		assert.NoError(t, err)

		assert.Equal(t, int64(id), got.Job.ID)
		assert.Equal(t, "get-job", got.Job.Type)
		assert.Equal(t, string(storage.JobStatePending), got.State)
		assert.True(t, got.Job.CreatedAt.Valid)
		assert.True(t, got.UpdatedAt.Valid)
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

		job := sqlcdb.Job{
			Type: "list-test",
		}
		id, err := store.CreateJob(ctx, job, storage.JobStatePending)
		require.NoError(t, err)

		got, err := store.ListJobs(ctx, 0, 0, storage.JobFilter{})
		assert.NoError(t, err)
		require.Len(t, got, 1)

		assert.Equal(t, int64(id), got[0].Job.ID)
		assert.Equal(t, "list-test", got[0].Job.Type)
		assert.Equal(t, string(storage.JobStatePending), got[0].State)
		assert.True(t, got[0].Job.CreatedAt.Valid)
		assert.True(t, got[0].UpdatedAt.Valid)
	})

	t.Run("list empty", func(t *testing.T) {
		ctx := context.Background()
		store := initTestDB(t)

		got, err := store.ListJobs(ctx, 0, 0, storage.JobFilter{})
		assert.NoError(t, err)

		want := []sqlcdb.ListJobsRow(nil)
		assert.Equal(t, want, got)
	})
}

func TestSQLite_UpdateJobState(t *testing.T) {
	t.Run("update pending to running", func(t *testing.T) {
		ctx := context.Background()
		store := initTestDB(t)

		job := sqlcdb.Job{
			Type: "update-job",
		}

		id, err := store.CreateJob(ctx, job, storage.JobStatePending)
		require.NoError(t, err)

		err = store.UpdateJobState(ctx, id, storage.JobStateRunning, nil)
		assert.NoError(t, err)

		got, err := store.GetJob(ctx, id)
		assert.NoError(t, err)

		assert.Equal(t, int64(id), got.Job.ID)
		assert.Equal(t, "update-job", got.Job.Type)
		assert.Equal(t, string(storage.JobStateRunning), got.State)
		assert.True(t, got.Job.CreatedAt.Valid)
		assert.True(t, got.UpdatedAt.Valid)
	})

	t.Run("update running to done", func(t *testing.T) {
		ctx := context.Background()
		store := initTestDB(t)

		job := sqlcdb.Job{
			Type: "done-job",
		}

		id, err := store.CreateJob(ctx, job, storage.JobStatePending)
		require.NoError(t, err)

		err = store.UpdateJobState(ctx, id, storage.JobStateRunning, nil)
		require.NoError(t, err)

		err = store.UpdateJobState(ctx, id, storage.JobStateDone, nil)
		assert.NoError(t, err)

		got, err := store.GetJob(ctx, id)
		assert.NoError(t, err)

		assert.Equal(t, int64(id), got.Job.ID)
		assert.Equal(t, "done-job", got.Job.Type)
		assert.Equal(t, string(storage.JobStateDone), got.State)
		assert.True(t, got.Job.CreatedAt.Valid)
		assert.True(t, got.UpdatedAt.Valid)
	})

	t.Run("update running to error with message", func(t *testing.T) {
		ctx := context.Background()
		store := initTestDB(t)

		job := sqlcdb.Job{
			Type: "error-job",
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

		assert.Equal(t, int64(id), got.Job.ID)
		assert.Equal(t, "error-job", got.Job.Type)
		assert.Equal(t, string(storage.JobStateError), got.State)
		assert.True(t, got.Error.Valid)
		assert.Equal(t, errorMsg, got.Error.String)
		assert.True(t, got.Job.CreatedAt.Valid)
		assert.True(t, got.UpdatedAt.Valid)
	})

	t.Run("invalid state transition", func(t *testing.T) {
		ctx := context.Background()
		store := initTestDB(t)

		job := sqlcdb.Job{
			Type: "invalid-transition",
		}

		id, err := store.CreateJob(ctx, job, storage.JobStatePending)
		require.NoError(t, err)

		err = store.UpdateJobState(ctx, id, storage.JobStateDone, nil)
		assert.Error(t, err)

		got, err := store.GetJob(ctx, id)
		assert.NoError(t, err)

		assert.Equal(t, int64(id), got.Job.ID)
		assert.Equal(t, "invalid-transition", got.Job.Type)
		assert.Equal(t, string(storage.JobStatePending), got.State)
		assert.True(t, got.Job.CreatedAt.Valid)
		assert.True(t, got.UpdatedAt.Valid)
	})
}

func TestSQLite_DeleteJob(t *testing.T) {
	t.Run("delete existing job", func(t *testing.T) {
		ctx := context.Background()
		store := initTestDB(t)

		job := sqlcdb.Job{
			Type: "delete-job",
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

func TestSQLite_ListJobs_Filters(t *testing.T) {
	ctx := context.Background()
	store := initTestDB(t)

	pendingType := "movie_index"
	doneType := "movie_reconcile"

	pendingID, err := store.CreateJob(ctx, sqlcdb.Job{Type: pendingType}, storage.JobStatePending)
	require.NoError(t, err)

	doneID, err := store.CreateJob(ctx, sqlcdb.Job{Type: doneType}, storage.JobStatePending)
	require.NoError(t, err)
	require.NoError(t, store.UpdateJobState(ctx, doneID, storage.JobStateRunning, nil))
	require.NoError(t, store.UpdateJobState(ctx, doneID, storage.JobStateDone, nil))

	t.Run("filter by type", func(t *testing.T) {
		jobType := pendingType
		jobs, err := store.ListJobs(ctx, 0, 0, storage.JobFilter{Type: &jobType})
		assert.NoError(t, err)
		require.Len(t, jobs, 1)
		assert.Equal(t, int64(pendingID), jobs[0].Job.ID)
		assert.Equal(t, string(storage.JobStatePending), jobs[0].State)
	})

	t.Run("filter by state matches only most recent transition", func(t *testing.T) {
		state := string(storage.JobStateDone)
		jobs, err := store.ListJobs(ctx, 0, 0, storage.JobFilter{State: &state})
		assert.NoError(t, err)
		require.Len(t, jobs, 1)
		assert.Equal(t, int64(doneID), jobs[0].Job.ID)

		state = string(storage.JobStatePending)
		jobs, err = store.ListJobs(ctx, 0, 0, storage.JobFilter{State: &state})
		assert.NoError(t, err)
		require.Len(t, jobs, 1)
		assert.Equal(t, int64(pendingID), jobs[0].Job.ID)
	})

	t.Run("zero filter lists all jobs", func(t *testing.T) {
		jobs, err := store.ListJobs(ctx, 0, 0, storage.JobFilter{})
		assert.NoError(t, err)
		assert.Len(t, jobs, 2)
	})

	t.Run("pagination", func(t *testing.T) {
		first, err := store.ListJobs(ctx, 0, 1, storage.JobFilter{})
		assert.NoError(t, err)
		require.Len(t, first, 1)

		second, err := store.ListJobs(ctx, 1, 1, storage.JobFilter{})
		assert.NoError(t, err)
		require.Len(t, second, 1)

		// pages must not overlap and must cover both jobs (created_at has
		// second precision, so the tie order between the two rows is not
		// guaranteed)
		ids := map[int64]bool{int64(pendingID): false, int64(doneID): false}
		ids[first[0].Job.ID] = true
		require.False(t, ids[second[0].Job.ID], "pages must not repeat a job")
		ids[second[0].Job.ID] = true
		for id, seen := range ids {
			assert.True(t, seen, "job %d missing from pages", id)
		}

		beyond, err := store.ListJobs(ctx, 5, 1, storage.JobFilter{})
		assert.NoError(t, err)
		assert.Empty(t, beyond)
	})
}

func TestSQLite_CountJobs_Filters(t *testing.T) {
	ctx := context.Background()
	store := initTestDB(t)

	total, err := store.CountJobs(ctx, storage.JobFilter{})
	assert.NoError(t, err)
	assert.Equal(t, 0, total)

	_, err = store.CreateJob(ctx, sqlcdb.Job{Type: "type_a"}, storage.JobStatePending)
	require.NoError(t, err)
	_, err = store.CreateJob(ctx, sqlcdb.Job{Type: "type_b"}, storage.JobStatePending)
	require.NoError(t, err)

	total, err = store.CountJobs(ctx, storage.JobFilter{})
	assert.NoError(t, err)
	assert.Equal(t, 2, total)

	jobType := "type_a"
	count, err := store.CountJobs(ctx, storage.JobFilter{Type: &jobType})
	assert.NoError(t, err)
	assert.Equal(t, 1, count)

	state := string(storage.JobStateRunning)
	count, err = store.CountJobs(ctx, storage.JobFilter{State: &state})
	assert.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestSQLite_DeleteJobsByIDs(t *testing.T) {
	ctx := context.Background()
	store := initTestDB(t)

	id1, err := store.CreateJob(ctx, sqlcdb.Job{Type: "delete-a"}, storage.JobStatePending)
	require.NoError(t, err)
	id2, err := store.CreateJob(ctx, sqlcdb.Job{Type: "delete-b"}, storage.JobStatePending)
	require.NoError(t, err)
	id3, err := store.CreateJob(ctx, sqlcdb.Job{Type: "delete-c"}, storage.JobStatePending)
	require.NoError(t, err)

	deleted, err := store.DeleteJobsByIDs(ctx, []int64{id1, id2})
	assert.NoError(t, err)
	assert.Equal(t, int64(2), deleted)

	_, err = store.GetJob(ctx, id1)
	assert.ErrorIs(t, err, storage.ErrNotFound)
	_, err = store.GetJob(ctx, id2)
	assert.ErrorIs(t, err, storage.ErrNotFound)
	got, err := store.GetJob(ctx, id3)
	assert.NoError(t, err)
	assert.NotNil(t, got)

	deleted, err = store.DeleteJobsByIDs(ctx, []int64{})
	assert.NoError(t, err)
	assert.Equal(t, int64(0), deleted)
}

func TestSQLite_UpdateJobState_History(t *testing.T) {
	ctx := context.Background()
	store := initTestDB(t)

	id, err := store.CreateJob(ctx, sqlcdb.Job{Type: "history"}, storage.JobStatePending)
	require.NoError(t, err)

	require.NoError(t, store.UpdateJobState(ctx, id, storage.JobStateRunning, nil))
	require.NoError(t, store.UpdateJobState(ctx, id, storage.JobStateDone, nil))

	// the same state transition again must fail (state machine does not allow
	// done -> running) and must leave the most recent transition untouched
	err = store.UpdateJobState(ctx, id, storage.JobStateRunning, nil)
	assert.Error(t, err)

	got, err := store.GetJob(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, string(storage.JobStateDone), got.State)
}
