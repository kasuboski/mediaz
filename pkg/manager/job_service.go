package manager

import (
	"context"
	"fmt"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/pagination"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
	"go.uber.org/zap"
)

// JobService owns job lifecycle, scheduling intervals, and activity/statistics recording.
type JobService struct {
	scheduler       *Scheduler
	jobStorage      storage.JobStorage
	activityStorage storage.ActivityStorage
	statsStorage    storage.StatisticsStorage
}

func NewJobService(jobStorage storage.JobStorage, activityStorage storage.ActivityStorage, statsStorage storage.StatisticsStorage, cfg config.Manager, executors map[JobType]JobExecutor) *JobService {
	return &JobService{
		scheduler:       NewScheduler(jobStorage, cfg, executors),
		jobStorage:      jobStorage,
		activityStorage: activityStorage,
		statsStorage:    statsStorage,
	}
}

func (js *JobService) Run(ctx context.Context) error {
	return js.scheduler.Run(ctx)
}

func (js *JobService) GetLibraryStats(ctx context.Context) (*storage.LibraryStats, error) {
	return js.statsStorage.GetLibraryStats(ctx)
}

func (js *JobService) CreateJob(ctx context.Context, request TriggerJobRequest) (JobResponse, error) {
	log := logger.FromCtx(ctx)

	jobID, err := js.scheduler.createPendingJob(ctx, JobType(request.Type))
	if err == storage.ErrJobAlreadyPending {
		log.Debug("job already pending, returning existing job")
		jobs, err := js.scheduler.listPendingJobsByType(ctx, JobType(request.Type))
		if err != nil {
			return JobResponse{}, err
		}
		if len(jobs) == 0 {
			return JobResponse{}, fmt.Errorf("pending job not found after conflict")
		}
		return toJobResponse(jobs[0]), nil
	}
	if err != nil {
		return JobResponse{}, err
	}

	job, err := js.jobStorage.GetJob(ctx, jobID)
	if err != nil {
		return JobResponse{}, err
	}

	return toJobResponse(job), nil
}

func (js *JobService) GetJob(ctx context.Context, id int64) (JobResponse, error) {
	job, err := js.jobStorage.GetJob(ctx, id)
	if err != nil {
		return JobResponse{}, err
	}
	return toJobResponse(job), nil
}

func (js *JobService) ListJobs(ctx context.Context, jobType *string, state *string, params pagination.Params) (JobListResponse, error) {
	var conditions []sqlite.BoolExpression

	if jobType != nil {
		if !isValidJobType(*jobType) {
			return JobListResponse{}, fmt.Errorf("invalid job type: %s", *jobType)
		}
		conditions = append(conditions, table.Job.Type.EQ(sqlite.String(*jobType)))
	}

	if state != nil {
		switch storage.JobState(*state) {
		case storage.JobStatePending, storage.JobStateRunning, storage.JobStateDone,
			storage.JobStateError, storage.JobStateCancelled:
			conditions = append(conditions, table.JobTransition.ToState.EQ(sqlite.String(*state)))
		default:
			return JobListResponse{}, fmt.Errorf("invalid job state: %s", *state)
		}
	}

	conditions = append(conditions, table.JobTransition.MostRecent.EQ(sqlite.Bool(true)))

	where := sqlite.AND(conditions...)

	totalCount, err := js.jobStorage.CountJobs(ctx, where)
	if err != nil {
		return JobListResponse{}, err
	}

	offset, limit := params.CalculateOffsetLimit()

	jobs, err := js.jobStorage.ListJobs(ctx, offset, limit, where)
	if err != nil {
		return JobListResponse{}, err
	}

	responses := make([]JobResponse, len(jobs))
	for i, job := range jobs {
		responses[i] = toJobResponse(job)
	}

	if params.PageSize == 0 {
		return JobListResponse{
			Jobs:  responses,
			Count: totalCount,
		}, nil
	}

	meta := params.BuildMeta(totalCount)
	return JobListResponse{
		Jobs:       responses,
		Count:      totalCount,
		Pagination: &meta,
	}, nil
}

func (js *JobService) CancelJob(ctx context.Context, id int64) (JobResponse, error) {
	log := logger.FromCtx(ctx)

	err := js.scheduler.CancelJob(ctx, id)
	if err != nil {
		log.Error("failed to cancel job", zap.Error(err), zap.Int64("job_id", id))
		return JobResponse{}, err
	}

	job, err := js.jobStorage.GetJob(ctx, id)
	if err != nil {
		return JobResponse{}, err
	}

	return toJobResponse(job), nil
}
