package download

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
)

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type DownloadClient interface {
	Add(ctx context.Context, request AddRequest) (Status, error)
	Get(ctx context.Context, request GetRequest) (Status, error)
	List(ctx context.Context) ([]Status, error)
}

type Factory interface {
	NewDownloadClient(config model.DownloadClient) (DownloadClient, error)
}

type DownloadClientFactory struct{}

func NewDownloadClientFactory() Factory {
	return DownloadClientFactory{}
}

// NewDownloadClient returns a downloada client for the given configuration
// TODO: handle supporting configurations such as timeouts, etc
func (DownloadClientFactory) NewDownloadClient(config model.DownloadClient) (DownloadClient, error) {
	switch config.Implementation {
	case "transmission":
		// TODO: Replace default http client with stored configurations
		return NewTransmissionClient(http.DefaultClient, config.Scheme, config.Host, int(config.Port)), nil
	default:
		return nil, fmt.Errorf("unknown download client implementation: %v", config.Implementation)
	}
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
	Progress float64       `json:"progress"` // percentage
	Speed    int64         `json:"speed"`    // assumed mb/s
	Size     int64         `json:"size"`     // assumed mb
	ETA      time.Duration `json:"eta"`
}
