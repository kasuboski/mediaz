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
		SELECT(
			table.Job.AllColumns,
			table.JobTransition.ToState,
			table.JobTransition.UpdatedAt,
			table.JobTransition.Error,
		).
		FROM(
			table.Job.INNER_JOIN(
				table.JobTransition,
				table.Job.ID.EQ(table.JobTransition.JobID),
			),
		).
		WHERE(
			table.JobTransition.Type.EQ(sqlite.String(jobType)).
				AND(table.JobTransition.ToState.EQ(sqlite.String(string(state)))).
				AND(table.JobTransition.MostRecent.EQ(sqlite.Bool(true))),
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

	jobModel := model.Job{
		Type: job.Type,
	}

	stmt := table.Job.
		INSERT(table.Job.AllColumns.Except(table.Job.ID, table.Job.CreatedAt)).
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

	transition := storage.JobTransition{
		JobID:      int32(inserted),
		Type:       job.Type,
		ToState:    string(initialState),
		MostRecent: true,
		SortKey:    1,
	}

	transitionStmt := table.JobTransition.
		INSERT(table.JobTransition.AllColumns.
			Except(table.JobTransition.ID, table.JobTransition.CreatedAt, table.JobTransition.UpdatedAt)).
		MODEL(transition)

	_, err = transitionStmt.ExecContext(ctx, tx)
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
		SELECT(
			table.Job.AllColumns,
			table.JobTransition.ToState,
			table.JobTransition.UpdatedAt,
			table.JobTransition.Error,
		).
		FROM(
			table.Job.INNER_JOIN(
				table.JobTransition,
				table.Job.ID.EQ(table.JobTransition.JobID),
			),
		).
		WHERE(
			table.Job.ID.EQ(sqlite.Int(id)).
				AND(table.JobTransition.MostRecent.EQ(sqlite.Bool(true))),
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

// CountJobs returns the total count of jobs matching the where conditions
func (s *SQLite) CountJobs(ctx context.Context, where ...sqlite.BoolExpression) (int, error) {
	stmt := table.Job.
		SELECT(sqlite.COUNT(table.Job.ID).AS("count")).
		FROM(
			table.Job.INNER_JOIN(
				table.JobTransition,
				table.Job.ID.EQ(table.JobTransition.JobID),
			),
		)

	for _, w := range where {
		stmt = stmt.WHERE(w)
	}

	var result struct {
		Count int64
	}
	err := stmt.QueryContext(ctx, s.db, &result)
	if err != nil {
		return 0, fmt.Errorf("failed to count jobs: %w", err)
	}

	return int(result.Count), nil
}

// ListJobs lists all jobs with optional pagination and where expressions
// If limit is 0, returns all jobs without pagination
func (s *SQLite) ListJobs(ctx context.Context, offset, limit int, where ...sqlite.BoolExpression) ([]*storage.Job, error) {
	stmt := table.Job.
		SELECT(
			table.Job.AllColumns,
			table.JobTransition.ToState,
			table.JobTransition.UpdatedAt,
			table.JobTransition.Error,
		).
		FROM(
			table.Job.INNER_JOIN(
				table.JobTransition,
				table.Job.ID.EQ(table.JobTransition.JobID),
			),
		).
		ORDER_BY(table.Job.CreatedAt.ASC())

	for _, w := range where {
		stmt = stmt.WHERE(w)
	}

	if limit > 0 {
		stmt = stmt.LIMIT(int64(limit)).OFFSET(int64(offset))
	}

	jobs := make([]*storage.Job, 0)
	err := stmt.QueryContext(ctx, s.db, &jobs)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
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

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	previousStmt := table.JobTransition.
		UPDATE().
		SET(
			table.JobTransition.MostRecent.SET(sqlite.Bool(false)),
			table.JobTransition.UpdatedAt.SET(sqlite.TimestampExp(sqlite.String(time.Now().Format(timestampFormat)))),
		).
		WHERE(
			table.JobTransition.JobID.EQ(sqlite.Int32(int32(id))).
				AND(table.JobTransition.MostRecent.EQ(sqlite.Bool(true))),
		).
		RETURNING(table.JobTransition.AllColumns)

	var previousTransition storage.JobTransition
	err = previousStmt.QueryContext(ctx, tx, &previousTransition)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to update previous job transition: %w", err)
	}

	transition := storage.JobTransition{
		JobID:      int32(id),
		Type:       job.Type,
		FromState:  &previousTransition.ToState,
		ToState:    string(state),
		MostRecent: true,
		SortKey:    previousTransition.SortKey + 1,
		Error:      errorMsg,
	}

	insertStmt := table.JobTransition.
		INSERT(table.JobTransition.AllColumns.
			Except(table.JobTransition.ID, table.JobTransition.CreatedAt, table.JobTransition.UpdatedAt)).
		MODEL(transition)

	_, err = insertStmt.ExecContext(ctx, tx)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to insert new job transition: %w", err)
	}

	return tx.Commit()
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
