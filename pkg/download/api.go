package download

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
)

type DownloadClient interface {
	Add(ctx context.Context, request AddRequest) (Status, error)
	Get(ctx context.Context, request GetRequest) (Status, error)
	List(ctx context.Context) ([]Status, error)
}

type Factory interface {
	NewDownloadClient(config model.DownloadClient) (DownloadClient, error)
}

type DownloadClientFactory struct {
	downloadMountPrefix string
}

// NewDownloadClientFactory creates a new download client factory.
// The mount prefix is optional. It can be used to prefix download mount directory in scenarios where the download client uses a different path.
func NewDownloadClientFactory(mountPrefix ...string) Factory {
	factory := DownloadClientFactory{}
	if len(mountPrefix) > 0 {
		factory.downloadMountPrefix = mountPrefix[0]
	}

	return factory
}

// NewDownloadClient returns a downloada client for the given configuration
// TODO: handle supporting configurations such as timeouts, etc
func (d DownloadClientFactory) NewDownloadClient(config model.DownloadClient) (DownloadClient, error) {
	switch config.Implementation {
	case "transmission":
		// TODO: Replace default http client with stored configurations
		return NewTransmissionClient(http.DefaultClient, config.Scheme, config.Host, d.downloadMountPrefix, int(config.Port)), nil
	case "sabnzbd":
		if config.APIKey == nil {
			return nil, errors.New("missing api key")
		}
		return NewSabnzbdClient(http.DefaultClient, config.Scheme, config.Host, d.downloadMountPrefix, *config.APIKey), nil
	default:
		return nil, fmt.Errorf("unsupported client implementation: %v", config.Implementation)
	}
}

type AddRequest struct {
	Release *prowlarr.ReleaseResource
}

type GetRequest struct {
	ID string
}

type Status struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	FilePaths []string `json:"filePaths"` // absolute path to the file
	Progress  float64  `json:"progress"`  // percentage
	Speed     int64    `json:"speed"`     // assumed mb/s
	Size      int64    `json:"size"`      // assumed mb
	Done      bool     `json:"done"`
}
