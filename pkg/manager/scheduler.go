package manager

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/cache"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
	"go.uber.org/zap"
)

type JobType string

const (
	MovieIndex      JobType = "MovieIndex"
	MovieReconcile  JobType = "MovieReconcile"
	SeriesIndex     JobType = "SeriesIndex"
	SeriesReconcile JobType = "SeriesReconcile"
	IndexerSync     JobType = "IndexerSync"
)

type JobExecutor func(ctx context.Context, jobID int64) error

type Scheduler struct {
	storage     storage.Storage
	config      config.Manager
	executors   map[JobType]JobExecutor
	runningJobs *cache.Cache[int64, context.CancelFunc]
}

// New creates a new scheduler for jobs
func NewScheduler(storage storage.Storage, config config.Manager, executors map[JobType]JobExecutor) *Scheduler {
	return &Scheduler{
		storage:     storage,
		config:      config,
		executors:   executors,
		runningJobs: cache.New[int64, context.CancelFunc](),
	}
}

func (s *Scheduler) Run(ctx context.Context) error {
	go s.processPendingJobs(ctx)
	go s.runPruning(ctx)
	return s.runJobScheduling(ctx)
}

func (s *Scheduler) runPruning(ctx context.Context) {
	if s.config.Jobs.CleanupPeriod == -1 {
		return
	}

	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.pruneOldJobs(ctx)
		}
	}
}

func (s *Scheduler) pruneOldJobs(ctx context.Context) {
	log := logger.FromCtx(ctx)

	cutoff := time.Now().Add(-s.config.Jobs.CleanupPeriod)

	jobTypes := []JobType{MovieIndex, MovieReconcile, SeriesIndex, SeriesReconcile, IndexerSync}
	jobIDsToPreserve := make([]int32, 0)

	for _, jobType := range jobTypes {
		where := sqlite.AND(
			table.Job.Type.EQ(sqlite.String(string(jobType))),
			table.JobTransition.MostRecent.EQ(sqlite.Bool(true)),
		)
		jobs, err := s.storage.ListJobs(ctx, 0, s.config.Jobs.MinJobsToKeep, where)
		if err != nil {
			log.Error("failed to list jobs for preservation",
				zap.String("type", string(jobType)),
				zap.Error(err))
			continue
		}

		for _, job := range jobs {
			jobIDsToPreserve = append(jobIDsToPreserve, job.ID)
		}
	}

	log.Info("jobs to preserve",
		zap.Int("count", len(jobIDsToPreserve)),
		zap.Any("ids", jobIDsToPreserve))

	whereConditions := []sqlite.BoolExpression{
		table.Job.CreatedAt.LT(sqlite.TimestampExp(sqlite.String(cutoff.Format(time.DateTime)))),
	}
	if len(jobIDsToPreserve) > 0 {
		ids := make([]sqlite.Expression, len(jobIDsToPreserve))
		for i, id := range jobIDsToPreserve {
			ids[i] = sqlite.Int32(id)
		}
		whereConditions = append(whereConditions,
			table.Job.ID.NOT_IN(ids...),
		)
	}

	deleted, err := s.storage.DeleteJobs(ctx, whereConditions...)
	if err != nil {
		log.Error("failed to prune old jobs", zap.Error(err))
		return
	}

	if deleted > 0 {
		log.Info("pruned old jobs", zap.Int64("count", deleted))
	}
}

func (s *Scheduler) runJobScheduling(ctx context.Context) error {
	ticker := time.NewTicker(s.config.Jobs.JobScheduleInterval)
	defer ticker.Stop()

	jobTypes := []JobType{MovieIndex, MovieReconcile, SeriesIndex, SeriesReconcile, IndexerSync}

	for {
		select {
		case <-ctx.Done():
			return s.shutdownJobs(ctx)
		case <-ticker.C:
			for _, jobType := range jobTypes {
				s.checkAndScheduleJob(ctx, jobType)
			}
		}
	}
}

func (s *Scheduler) shutdownJobs(ctx context.Context) error {
	log := logger.FromCtx(ctx)
	log.Debug("scheduler context cancelled")

	jobIDs := s.runningJobs.Keys()

	var wg sync.WaitGroup
	for _, id := range jobIDs {
		wg.Add(1)
		go func(ctx context.Context, jobID int64) {
			defer wg.Done()
			if err := s.CancelJob(ctx, jobID); err != nil {
				log.Warn("failed to cancel job on context cancellation",
					zap.Int64("job_id", jobID),
					zap.Error(err))
			}
		}(ctx, id)
	}

	wg.Wait()
	log.Debug("all jobs cancelled on context cancellation", zap.Int("count", len(jobIDs)))
	return nil
}

func (s *Scheduler) checkAndScheduleJob(ctx context.Context, jobType JobType) {
	log := logger.FromCtx(ctx).With(zap.String("job_type", string(jobType)))

	interval := s.getIntervalForJobType(jobType)

	where := sqlite.AND(
		table.Job.Type.EQ(sqlite.String(string(jobType))),
		table.JobTransition.MostRecent.EQ(sqlite.Bool(true)),
	)
	jobs, err := s.storage.ListJobs(ctx, 0, 1, where)

	if err != nil {
		log.Error("failed to get last job", zap.Error(err))
		return
	}

	if len(jobs) == 0 {
		log.Debug("no previous jobs found, scheduling immediately")
		_, err := s.createPendingJob(ctx, jobType)
		if err != nil {
			log.Error("failed to create pending job", zap.Error(err))
		}
		return
	}

	lastJob := jobs[0]

	switch lastJob.State {
	case storage.JobStatePending, storage.JobStateRunning:
		log.Debug("job already pending or running, not scheduling",
			zap.String("state", string(lastJob.State)))
		return
	case storage.JobStateDone, storage.JobStateError, storage.JobStateCancelled:
		timeSinceLastJob := time.Since(*lastJob.CreatedAt)

		if timeSinceLastJob >= interval {
			log.Debug("interval elapsed, scheduling job",
				zap.Duration("time_since_last", timeSinceLastJob),
				zap.Duration("interval", interval))
			_, err := s.createPendingJob(ctx, jobType)
			if err != nil {
				log.Error("failed to create pending job", zap.Error(err))
			}
			return
		}

		log.Debug("interval not elapsed yet",
			zap.Duration("time_since_last", timeSinceLastJob),
			zap.Duration("interval", interval),
			zap.Duration("time_remaining", interval-timeSinceLastJob))
	}
}

func (s *Scheduler) getIntervalForJobType(jobType JobType) time.Duration {
	switch jobType {
	case MovieIndex:
		return s.config.Jobs.MovieIndex
	case MovieReconcile:
		return s.config.Jobs.MovieReconcile
	case SeriesIndex:
		return s.config.Jobs.SeriesIndex
	case SeriesReconcile:
		return s.config.Jobs.SeriesReconcile
	case IndexerSync:
		return s.config.Jobs.IndexerSync
	default:
		return 10 * time.Minute
	}
}

func (s *Scheduler) createPendingJob(ctx context.Context, jobType JobType) (int64, error) {
	newJobType := string(jobType)
	log := logger.FromCtx(ctx).With(zap.String("job_type", newJobType))

	if !isValidJobType(newJobType) {
		return 0, errors.New("invalid job type")
	}

	job := storage.Job{
		Job: model.Job{
			Type: newJobType,
		},
	}

	id, err := s.storage.CreateJob(ctx, job, storage.JobStatePending)
	if err == storage.ErrJobAlreadyPending {
		log.Debug("pending job already exists for type")
		return 0, err
	}
	if err != nil {
		log.Error("failed to create pending job", zap.Error(err))
		return 0, err
	}
	log.Debug("created pending job", zap.Int64("id", id))
	return id, nil
}

func (s *Scheduler) listPendingJobs(ctx context.Context) ([]*storage.Job, error) {
	where := sqlite.AND(
		table.JobTransition.ToState.EQ(sqlite.String(string(storage.JobStatePending))),
		table.JobTransition.MostRecent.EQ(sqlite.Bool(true)),
	)
	return s.storage.ListJobs(ctx, 0, 0, where)
}

func (s *Scheduler) listPendingJobsByType(ctx context.Context, jobType JobType) ([]*storage.Job, error) {
	where := sqlite.AND(
		table.JobTransition.Type.EQ(sqlite.String(string(jobType))),
		table.JobTransition.ToState.EQ(sqlite.String(string(storage.JobStatePending))),
		table.JobTransition.MostRecent.EQ(sqlite.Bool(true)),
	)
	return s.storage.ListJobs(ctx, 0, 0, where)
}

func (s *Scheduler) processPendingJobs(ctx context.Context) {
	ticker := time.NewTicker(time.Second * 5)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			log := logger.FromCtx(ctx)

			jobs, err := s.listPendingJobs(ctx)
			if err != nil {
				log.Debug("failed to list pending jobs", zap.Error(err))
				continue
			}
			if len(jobs) == 0 {
				log.Debug("no pending jobs found")
				continue
			}

			log.Debug("found pending jobs", zap.Int("count", len(jobs)))

			for _, job := range jobs {
				if err := ctx.Err(); err != nil {
					return
				}

				s.executeJob(ctx, job)
			}
		}
	}
}

func (s *Scheduler) executeJob(ctx context.Context, job *storage.Job) {
	log := logger.FromCtx(ctx).With(
		zap.Int64("job_id", int64(job.ID)),
		zap.String("job_type", job.Type),
	)

	executor, ok := s.executors[JobType(job.Type)]
	if !ok {
		log.Error("no executor found for job type")
		errMsg := "no executor found for job type"
		s.storage.UpdateJobState(ctx, int64(job.ID), storage.JobStateError, &errMsg)
		return
	}

	err := s.storage.UpdateJobState(ctx, int64(job.ID), storage.JobStateRunning, nil)
	if err != nil {
		log.Error("failed to update job state to running", zap.Error(err))
		return
	}

	jobCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	s.runningJobs.Set(int64(job.ID), cancel)

	defer func() {
		s.runningJobs.Delete(int64(job.ID))
	}()

	log.Debug("executing job")

	err = executor(jobCtx, int64(job.ID))
	if err != nil {
		if jobCtx.Err() == context.Canceled {
			log.Info("job cancelled")
			s.storage.UpdateJobState(ctx, int64(job.ID), storage.JobStateCancelled, nil)
			return
		}

		log.Error("job execution failed", zap.Error(err))
		errMsg := err.Error()
		s.storage.UpdateJobState(ctx, int64(job.ID), storage.JobStateError, &errMsg)
		return
	}

	err = s.storage.UpdateJobState(ctx, int64(job.ID), storage.JobStateDone, nil)
	if err != nil {
		log.Error("failed to update job state to done", zap.Error(err))
		return
	}

	log.Debug("job completed successfully")
}

func (s *Scheduler) CancelJob(ctx context.Context, jobID int64) error {
	log := logger.FromCtx(ctx).With(zap.Int64("job_id", jobID))

	job, err := s.storage.GetJob(ctx, jobID)
	if err != nil {
		return err
	}

	switch job.State {
	case storage.JobStatePending:
		log.Debug("cancelling pending job")
		return s.storage.UpdateJobState(ctx, jobID, storage.JobStateCancelled, nil)

	case storage.JobStateRunning:
		cancel, ok := s.runningJobs.Get(jobID)
		if !ok {
			log.Debug("job not found in running jobs map")
			return nil
		}

		log.Debug("cancelling running job")
		cancel()

		timeout := time.After(30 * time.Second)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-timeout:
				log.Error("timeout waiting for job to complete cancellation")
				return nil
			case <-ticker.C:
				if _, exists := s.runningJobs.Get(jobID); !exists {
					log.Debug("job was cancelled")
					return nil
				}
			}
		}

	default:
		return nil
	}
}
