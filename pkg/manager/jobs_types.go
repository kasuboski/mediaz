package manager

import (
	"time"

	"github.com/kasuboski/mediaz/pkg/storage"
)

// TriggerJobRequest represents the request to manually trigger a job
type TriggerJobRequest struct {
	Type string `json:"type"`
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
	Jobs  []JobResponse `json:"jobs"`
	Count int           `json:"count"`
}

// toJobResponse converts a storage.Job to a JobResponse
func toJobResponse(job *storage.Job) JobResponse {
	return JobResponse{
		ID:        int64(job.ID),
		Type:      job.Type,
		State:     string(job.State),
		CreatedAt: *job.CreatedAt,
		UpdatedAt: *job.UpdatedAt,
		Error:     job.Error,
	}
}

// isValidJobType validates that a job type string matches one of the defined JobType constants
func isValidJobType(jobType string) bool {
	switch JobType(jobType) {
	case MovieIndex, MovieReconcile, SeriesIndex, SeriesReconcile:
		return true
	default:
		return false
	}
}
