package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

// jobJoinRow mirrors the shared job+job_transition select shape used by the
// job lookup queries so one conversion covers them all.
type jobJoinRow struct {
	ID        int64
	Type      string
	CreatedAt sql.NullTime
	ToState   string
	UpdatedAt sql.NullTime
	Error     sql.NullString
}

func jobStrPtr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	s := v.String
	return &s
}

func jobTimePtr(v sql.NullTime) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time
	return &t
}

func jobFromJoinRow(r jobJoinRow) storage.Job {
	return storage.Job{
		Job: model.Job{
			ID:        int32(r.ID),
			Type:      r.Type,
			CreatedAt: jobTimePtr(r.CreatedAt),
		},
		State:     storage.JobState(r.ToState),
		UpdatedAt: jobTimePtr(r.UpdatedAt),
		Error:     jobStrPtr(r.Error),
	}
}

// jobFilterArgs maps the typed filter onto the nullable query filters. A nil
// filter field means "no constraint".
func jobFilterArgs(filter storage.JobFilter) (*string, *string) {
	return filter.Type, filter.State
}

func jobTransitionParams(t storage.JobTransition) sqlcdb.CreateJobTransitionParams {
	return sqlcdb.CreateJobTransitionParams{
		JobID:      int64(t.JobID),
		Type:       t.Type,
		ToState:    t.ToState,
		FromState:  movieNullStr(t.FromState),
		MostRecent: t.MostRecent,
		SortKey:    int64(t.SortKey),
		Error:      movieNullStr(t.Error),
	}
}

// getJobByTypeAndState retrieves a job by type and state
func (s *SQLite) getJobByTypeAndState(ctx context.Context, jobType string, state storage.JobState) (*storage.Job, error) {
	row, err := s.Queries.GetJobByTypeAndState(ctx, sqlcdb.GetJobByTypeAndStateParams{
		Type:    jobType,
		ToState: string(state),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get job by type and state: %w", err)
	}

	job := jobFromJoinRow(jobJoinRow(row))
	return &job, nil
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

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	q := s.txQueries(tx)

	inserted, err := q.CreateJob(ctx, job.Type)
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

	if err := q.CreateJobTransition(ctx, jobTransitionParams(transition)); err != nil {
		tx.Rollback()
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		tx.Rollback()
		return 0, err
	}

	return inserted, nil
}

// GetJob retrieves a job by ID with its current state
func (s *SQLite) GetJob(ctx context.Context, id int64) (*storage.Job, error) {
	row, err := s.Queries.GetJobByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	job := jobFromJoinRow(jobJoinRow(row))
	return &job, nil
}

// CountJobs returns the total count of jobs matching the filter. Queries are
// always constrained to the most recent transition of each job.
func (s *SQLite) CountJobs(ctx context.Context, filter storage.JobFilter) (int, error) {
	jobType, state := jobFilterArgs(filter)
	count, err := s.Queries.CountJobs(ctx, sqlcdb.CountJobsParams{
		JobType: jobType,
		State:   state,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to count jobs: %w", err)
	}

	return int(count), nil
}

// ListJobs lists all jobs with optional pagination and filtering. If limit is
// 0, returns all jobs without pagination. Queries are always constrained to
// the most recent transition of each job.
func (s *SQLite) ListJobs(ctx context.Context, offset, limit int, filter storage.JobFilter) ([]*storage.Job, error) {
	jobType, state := jobFilterArgs(filter)
	rows, err := s.Queries.ListJobs(ctx, sqlcdb.ListJobsParams{
		JobType: jobType,
		State:   state,
		Lim:     limit,
		Offs:    int64(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}

	jobs := make([]*storage.Job, 0, len(rows))
	for _, r := range rows {
		job := jobFromJoinRow(jobJoinRow(r))
		jobs = append(jobs, &job)
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

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	q := s.txQueries(tx)

	previous, err := q.DeactivateJobTransition(ctx, sqlcdb.DeactivateJobTransitionParams{
		UpdatedAt: time.Now().Format(timestampFormat),
		JobID:     id,
	})
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to update previous job transition: %w", err)
	}

	fromState := previous.ToState
	transition := storage.JobTransition{
		JobID:      int32(id),
		Type:       job.Type,
		FromState:  &fromState,
		ToState:    string(state),
		MostRecent: true,
		SortKey:    int32(previous.SortKey) + 1,
		Error:      errorMsg,
	}

	if err := q.CreateJobTransition(ctx, jobTransitionParams(transition)); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to insert new job transition: %w", err)
	}

	return tx.Commit()
}

// DeleteJob removes a job by ID
func (s *SQLite) DeleteJob(ctx context.Context, id int64) error {
	err := s.Queries.DeleteJob(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to delete job: %w", err)
	}

	return nil
}

// DeleteJobsByIDs removes multiple jobs by their ids in a single statement and
// returns the number of jobs removed.
func (s *SQLite) DeleteJobsByIDs(ctx context.Context, ids []int64) (int64, error) {
	count, err := s.Queries.DeleteJobsByIDs(ctx, ids)
	if err != nil {
		return 0, fmt.Errorf("failed to delete jobs: %w", err)
	}

	return count, nil
}
