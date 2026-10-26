package manager

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/pagination"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"go.uber.org/zap"
)

// JobService owns job lifecycle, scheduling intervals, and activity/statistics recording.
type JobService struct {
	scheduler       *Scheduler
	jobStorage      storage.JobStorage
	activityStorage storage.ActivityStorage
	statsStorage    storage.StatisticsStorage
}

// TriggerJobRequest represents the request to manually trigger a job
type TriggerJobRequest struct {
	Type string `json:"type" validate:"required,oneof=MovieIndex MovieReconcile SeriesIndex SeriesReconcile IndexerSync"`
}

// JobResponse represents a single job in API responses
type JobResponse struct {
	ID        int64     `json:"id"`
	Type      string    `json:"type"`
	State     string    `json:"state"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Error     *string   `json:"error,omitempty"`
}

// JobListResponse represents a list of jobs in API responses
type JobListResponse struct {
	Jobs       []JobResponse    `json:"jobs"`
	Count      int              `json:"count"`
	Pagination *pagination.Meta `json:"pagination,omitempty"`
}

// toJobResponse converts a single-job read row to a JobResponse
func toJobResponse(job sqlcdb.GetJobByIDRow) JobResponse {
	return JobResponse{
		ID:        job.Job.ID,
		Type:      job.Job.Type,
		State:     job.State,
		CreatedAt: job.Job.CreatedAt.Time,
		UpdatedAt: job.UpdatedAt.Time,
		Error:     nullStrOut(job.Error),
	}
}

// toJobListResponse converts a listed job row to a JobResponse
func toJobListResponse(job sqlcdb.ListJobsRow) JobResponse {
	return JobResponse{
		ID:        job.Job.ID,
		Type:      job.Job.Type,
		State:     job.State,
		CreatedAt: job.Job.CreatedAt.Time,
		UpdatedAt: job.UpdatedAt.Time,
		Error:     nullStrOut(job.Error),
	}
}

// nullStrOut converts a nullable string to an optional string.
func nullStrOut(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	s := v.String
	return &s
}

// isValidJobType validates that a job type string matches one of the defined JobType constants
func isValidJobType(jobType string) bool {
	switch JobType(jobType) {
	case MovieIndex, MovieReconcile, SeriesIndex, SeriesReconcile, IndexerSync:
		return true
	default:
		return false
	}
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
		return toJobListResponse(jobs[0]), nil
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
	filter := storage.JobFilter{}

	if jobType != nil {
		if !isValidJobType(*jobType) {
			return JobListResponse{}, fmt.Errorf("invalid job type: %s", *jobType)
		}
		filter.Type = jobType
	}

	if state != nil {
		switch storage.JobState(*state) {
		case storage.JobStatePending, storage.JobStateRunning, storage.JobStateDone,
			storage.JobStateError, storage.JobStateCancelled:
			filter.State = state
		default:
			return JobListResponse{}, fmt.Errorf("invalid job state: %s", *state)
		}
	}

	totalCount, err := js.jobStorage.CountJobs(ctx, filter)
	if err != nil {
		return JobListResponse{}, err
	}

	offset, limit := params.CalculateOffsetLimit()

	jobs, err := js.jobStorage.ListJobs(ctx, offset, limit, filter)
	if err != nil {
		return JobListResponse{}, err
	}

	responses := make([]JobResponse, len(jobs))
	for i, job := range jobs {
		responses[i] = toJobListResponse(job)
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
