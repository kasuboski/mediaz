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
	log := logger.FromCtx(ctx)

	movieIndexTicker := time.NewTicker(s.config.Jobs.MovieIndex)
	defer movieIndexTicker.Stop()

	movieReconcileTicker := time.NewTicker(s.config.Jobs.MovieReconcile)
	defer movieReconcileTicker.Stop()

	seriesIndexTicker := time.NewTicker(s.config.Jobs.SeriesIndex)
	defer seriesIndexTicker.Stop()

	seriesReconcileTicker := time.NewTicker(s.config.Jobs.SeriesReconcile)
	defer seriesReconcileTicker.Stop()

	indexerSyncTicker := time.NewTicker(s.config.Jobs.IndexerSync)
	defer indexerSyncTicker.Stop()

	go s.processPendingJobs(ctx)

	for {
		select {
		case <-ctx.Done():
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
		case <-movieIndexTicker.C:
			_, _ = s.createPendingJob(ctx, MovieIndex)
		case <-movieReconcileTicker.C:
			_, _ = s.createPendingJob(ctx, MovieReconcile)
		case <-seriesIndexTicker.C:
			_, _ = s.createPendingJob(ctx, SeriesIndex)
		case <-seriesReconcileTicker.C:
			_, _ = s.createPendingJob(ctx, SeriesReconcile)
		case <-indexerSyncTicker.C:
			_, _ = s.createPendingJob(ctx, IndexerSync)
		}
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
