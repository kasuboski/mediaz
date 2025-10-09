package sqlite

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-jet/jet/v2/qrm"
	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
)

// getJobByTypeAndState retrieves a job by type and state
func (s *SQLite) getJobByTypeAndState(ctx context.Context, jobType string, state storage.JobState) (*storage.Job, error) {
	stmt := table.Job.
		SELECT(table.Job.AllColumns).
		FROM(table.Job).
		WHERE(
			table.Job.Type.EQ(sqlite.String(jobType)).
				AND(table.Job.ToState.EQ(sqlite.String(string(state)))).
				AND(table.Job.MostRecent.EQ(sqlite.Bool(true))),
		)

	job := new(storage.Job)
	err := stmt.QueryContext(ctx, s.db, job)
	if err != nil {
		if errors.Is(err, qrm.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get job by type and state: %w", err)
	}

	return job, nil
}

// CreateJob stores a job and creates an initial state
func (s *SQLite) CreateJob(ctx context.Context, job storage.Job, initialState storage.JobState) (int64, error) {
	if job.State == "" {
		job.State = storage.JobStateNew
	}

	err := job.Machine().ToState(initialState)
	if err != nil {
		return 0, err
	}

	// Check for duplicate pending jobs
	if initialState == storage.JobStatePending {
		existing, err := s.getJobByTypeAndState(ctx, job.Type, storage.JobStatePending)
		if err == nil && existing != nil {
			return 0, storage.ErrJobAlreadyPending
		}
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			return 0, err
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}

	// Create the initial job record
	jobModel := model.Job{
		Type:       job.Type,
		ToState:    string(initialState),
		MostRecent: true,
		SortKey:    1,
	}

	stmt := table.Job.
		INSERT(table.Job.AllColumns.Except(table.Job.ID, table.Job.CreatedAt, table.Job.UpdatedAt, table.Job.FromState, table.Job.Error)).
		MODEL(jobModel).
		RETURNING(table.Job.ID)

	result, err := stmt.ExecContext(ctx, tx)
	if err != nil {
		tx.Rollback()
		return 0, err
	}

	inserted, err := result.LastInsertId()
	if err != nil {
		tx.Rollback()
		return 0, err
	}

	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		return 0, err
	}

	return inserted, nil
}

// GetJob retrieves a job by ID with its current state
func (s *SQLite) GetJob(ctx context.Context, id int64) (*storage.Job, error) {
	stmt := table.Job.
		SELECT(table.Job.AllColumns).
		FROM(table.Job).
		WHERE(
			table.Job.ID.EQ(sqlite.Int(id)).
				AND(table.Job.MostRecent.EQ(sqlite.Bool(true))),
		)

	job := new(storage.Job)
	err := stmt.QueryContext(ctx, s.db, job)
	if err != nil {
		if errors.Is(err, qrm.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	return job, nil
}

// ListJobs lists all jobs with their current state
func (s *SQLite) ListJobs(ctx context.Context) ([]*storage.Job, error) {
	stmt := table.Job.
		SELECT(table.Job.AllColumns).
		FROM(table.Job).
		WHERE(table.Job.MostRecent.EQ(sqlite.Bool(true))).
		ORDER_BY(table.Job.CreatedAt.ASC())

	jobs := make([]*storage.Job, 0)
	err := stmt.QueryContext(ctx, s.db, &jobs)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}

	return jobs, nil
}

// ListJobsByState lists all jobs in a specific state
func (s *SQLite) ListJobsByState(ctx context.Context, state storage.JobState) ([]*storage.Job, error) {
	stmt := table.Job.
		SELECT(table.Job.AllColumns).
		FROM(table.Job).
		WHERE(
			table.Job.MostRecent.EQ(sqlite.Bool(true)).
				AND(table.Job.ToState.EQ(sqlite.String(string(state))))).
		ORDER_BY(table.Job.CreatedAt.ASC())

	jobs := make([]*storage.Job, 0)
	err := stmt.QueryContext(ctx, s.db, &jobs)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs by state: %w", err)
	}

	return jobs, nil
}

// UpdateJobState updates the state of a job, optionally setting an error message
func (s *SQLite) UpdateJobState(ctx context.Context, id int64, state storage.JobState, errorMsg *string) error {
	job, err := s.GetJob(ctx, id)
	if err != nil {
		return err
	}

	err = job.Machine().ToState(state)
	if err != nil {
		return err
	}

	stmt := table.Job.
		UPDATE().
		SET(
			table.Job.FromState.SET(sqlite.String(job.ToState)),
			table.Job.ToState.SET(sqlite.String(string(state))),
			table.Job.SortKey.SET(sqlite.Int32(job.SortKey+1)),
			table.Job.UpdatedAt.SET(sqlite.TimestampExp(sqlite.String(time.Now().Format(timestampFormat)))),
		).
		WHERE(table.Job.ID.EQ(sqlite.Int64(id)))

	if errorMsg != nil {
		stmt = table.Job.
			UPDATE().
			SET(
				table.Job.FromState.SET(sqlite.String(job.ToState)),
				table.Job.ToState.SET(sqlite.String(string(state))),
				table.Job.SortKey.SET(sqlite.Int32(job.SortKey+1)),
				table.Job.UpdatedAt.SET(sqlite.TimestampExp(sqlite.String(time.Now().Format(timestampFormat)))),
				table.Job.Error.SET(sqlite.String(*errorMsg)),
			).
			WHERE(table.Job.ID.EQ(sqlite.Int64(id)))
	}

	_, err = s.handleStatement(ctx, stmt)
	if err != nil {
		return fmt.Errorf("failed to update job state: %w", err)
	}

	return nil
}

// DeleteJob removes a job by ID
func (s *SQLite) DeleteJob(ctx context.Context, id int64) error {
	stmt := table.Job.
		DELETE().
		WHERE(table.Job.ID.EQ(sqlite.Int64(id)))

	_, err := s.handleDelete(ctx, stmt)
	if err != nil {
		return fmt.Errorf("failed to delete job: %w", err)
	}

	return nil
}
