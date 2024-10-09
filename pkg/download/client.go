package download

import (
	"context"
	"time"

	"github.com/kasuboski/mediaz/pkg/prowlarr"
)

type Client interface {
	Add(ctx context.Context, request AddRequest) (*Status, error)
	Get(ctx context.Context, request GetRequest) (*Status, error)
	// Delete(ctx context.Context, request DeleteRequest) error
	List(ctx context.Context) ([]Status, error)
	// Pause(ctx context.Context, request PauseRequest) error
	// Start(ctx context.Context, request StartRequest) error
}

type AddRequest struct {
	Release *prowlarr.ReleaseResource
}

type GetRequest struct {
	ID int
}

type DeleteRequest struct {
	ID int
}

type PauseRequest struct {
	ID int
}

type StartRequest struct {
	ID int
}

type Status struct {
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	State    string        `json:"state"`
	Progress float64       `json:"progress"` // percentage
	Speed    int64         `json:"speed"`    // assumed mb/s
	Size     int64         `json:"size"`     // assumed mb
	ETA      time.Duration `json:"eta"`
}
