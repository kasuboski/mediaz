package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/mattn/go-sqlite3"
)

// CreateJob stores a job and creates an initial state. When the initial state
// is pending, the check-then-insert pair runs inside the same immediate
// transaction that inserts the job, and a UNIQUE violation of
// idx_job_type_pending (another pending job of the same type) maps to
// storage.ErrJobAlreadyPending.
func (s *SQLite) CreateJob(ctx context.Context, job sqlcdb.Job, initialState storage.JobState) (int64, error) {
	if err := storage.JobStateNew.Machine().ToState(initialState); err != nil {
		return 0, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	q := s.txQueries(tx)

	if initialState == storage.JobStatePending {
		_, err := q.GetJobByTypeAndState(ctx, sqlcdb.GetJobByTypeAndStateParams{
			Type:    job.Type,
			ToState: string(storage.JobStatePending),
		})
		if err == nil {
			tx.Rollback()
			return 0, storage.ErrJobAlreadyPending
		}
		if !errors.Is(err, sql.ErrNoRows) {
			tx.Rollback()
			return 0, fmt.Errorf("failed to check for pending job: %w", err)
		}
	}

	inserted, err := q.CreateJob(ctx, job.Type)
	if err != nil {
		tx.Rollback()
		return 0, err
	}

	if err := q.CreateJobTransition(ctx, sqlcdb.CreateJobTransitionParams{
		JobID:      inserted,
		Type:       job.Type,
		ToState:    string(initialState),
		MostRecent: true,
		SortKey:    1,
	}); err != nil {
		tx.Rollback()
		if isUniqueViolation(err, "idx_job_type_pending") {
			return 0, storage.ErrJobAlreadyPending
		}
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		tx.Rollback()
		return 0, err
	}

	return inserted, nil
}

// isUniqueViolation reports whether err is a SQLite UNIQUE constraint
// violation, optionally on the given index.
func isUniqueViolation(err error, index string) bool {
	var sqliteErr sqlite3.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}
	if sqliteErr.ExtendedCode != sqlite3.ErrConstraintUnique && sqliteErr.ExtendedCode != sqlite3.ErrConstraintPrimaryKey {
		return false
	}
	if index == "" {
		return true
	}
	if strings.Contains(sqliteErr.Error(), index) {
		return true
	}
	// SQLite reports indexed columns, not the partial index name.
	return index == "idx_job_type_pending" && sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique && strings.Contains(sqliteErr.Error(), "job_transition.type, job_transition.to_state")
}

// GetJob retrieves a job by ID with its current state
func (s *SQLite) GetJob(ctx context.Context, id int64) (sqlcdb.GetJobByIDRow, error) {
	row, err := s.Queries.GetJobByID(ctx, id)
	if err != nil {
		return sqlcdb.GetJobByIDRow{}, mapQueryErr(err, "failed to get job")
	}
	return row, nil
}

// CountJobs returns the total count of jobs matching the filter. Queries are
// always constrained to the most recent transition of each job.
func (s *SQLite) CountJobs(ctx context.Context, filter storage.JobFilter) (int, error) {
	count, err := s.Queries.CountJobs(ctx, sqlcdb.CountJobsParams{
		JobType: filter.Type,
		State:   filter.State,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to count jobs: %w", err)
	}

	return int(count), nil
}

// ListJobs lists jobs with optional pagination and filtering. A zero limit
// returns all jobs. Ordering is created_at DESC, id DESC so pagination windows
// are stable across same-second rows.
func (s *SQLite) ListJobs(ctx context.Context, offset, limit int, filter storage.JobFilter) ([]sqlcdb.ListJobsRow, error) {
	if offset < 0 || limit < 0 {
		return nil, fmt.Errorf("offset and limit must be nonnegative")
	}
	rows, err := s.Queries.ListJobs(ctx, sqlcdb.ListJobsParams{
		JobType: filter.Type,
		State:   filter.State,
		Lim:     int64(limit),
		Offs:    int64(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}

	return rows, nil
}

// UpdateJobState updates the state of a job, optionally setting an error
// message. The demote+insert pair runs in one transaction; on any failure
// (including an invalid transition) nothing is written.
func (s *SQLite) UpdateJobState(ctx context.Context, id int64, state storage.JobState, errorMsg *string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	q := s.txQueries(tx)
	job, err := q.GetJobByID(ctx, id)
	if err != nil {
		return mapQueryErr(err, "failed to get job")
	}
	if err := storage.JobState(job.State).Machine().ToState(state); err != nil {
		return err
	}

	previous, err := q.DeactivateJobTransition(ctx, id)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to update previous job transition: %w", err)
	}

	fromState := previous.ToState
	transition := sqlcdb.CreateJobTransitionParams{
		JobID:      id,
		Type:       job.Job.Type,
		FromState:  sql.NullString{String: fromState, Valid: true},
		ToState:    string(state),
		MostRecent: true,
		SortKey:    previous.SortKey + 1,
	}
	if errorMsg != nil {
		transition.Error = sql.NullString{String: *errorMsg, Valid: true}
	}

	if err := q.CreateJobTransition(ctx, transition); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to insert new job transition: %w", err)
	}

	return tx.Commit()
}

// DeleteJob removes a job by ID
func (s *SQLite) DeleteJob(ctx context.Context, id int64) error {
	if err := s.Queries.DeleteJob(ctx, id); err != nil {
		return fmt.Errorf("failed to delete job: %w", err)
	}
	return nil
}

// DeleteJobsByIDs removes multiple jobs by their ids in a single statement
// and returns the number of jobs removed. An empty ids slice is a no-op.
func (s *SQLite) DeleteJobsByIDs(ctx context.Context, ids []int64) (int64, error) {
	count, err := s.Queries.DeleteJobsByIDs(ctx, ids)
	if err != nil {
		return 0, fmt.Errorf("failed to delete jobs: %w", err)
	}
	return count, nil
}
