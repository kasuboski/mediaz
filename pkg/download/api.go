package download

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kasuboski/mediaz/pkg/indexer"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

type DownloadClient interface {
	Add(ctx context.Context, request AddRequest) (Status, error)
	Get(ctx context.Context, request GetRequest) (Status, error)
	List(ctx context.Context) ([]Status, error)
}

type Factory interface {
	NewDownloadClient(config sqlcdb.DownloadClient) (DownloadClient, error)
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

// ValidateSABConfig validates endpoint fields without performing network I/O.
func ValidateSABConfig(scheme, host string, port int64) error {
	if scheme != "http" && scheme != "https" {
		return errors.New("SABnzbd scheme must be http or https")
	}
	_, inline, err := parseSABHost(host)
	if err != nil {
		return err
	}
	if port < 0 || port > 65535 {
		return errors.New("invalid SABnzbd port")
	}
	if inline != 0 && port != 0 && int64(inline) != port {
		return errors.New("conflicting SABnzbd ports")
	}
	return nil
}

func parseSABHost(host string) (string, int, error) {
	bad := func() (string, int, error) { return "", 0, errors.New("invalid SABnzbd host") }
	if host == "" || strings.ContainsAny(host, "/\\?#@\r\n\t ") {
		return bad()
	}
	for _, r := range host {
		if r < 32 || r == 127 {
			return bad()
		}
	}
	name, inline := host, 0
	if strings.HasPrefix(host, "[") {
		h, p, err := net.SplitHostPort(host)
		if err == nil {
			name = h
			inline, err = strconv.Atoi(p)
			if err != nil || inline < 1 || inline > 65535 {
				return bad()
			}
		} else {
			if !strings.HasSuffix(host, "]") || net.ParseIP(strings.Trim(host, "[]")) == nil {
				return bad()
			}
			name = strings.Trim(host, "[]")
		}
	} else if strings.Count(host, ":") > 1 {
		if net.ParseIP(host) == nil {
			return bad()
		}
	} else if strings.Contains(host, ":") {
		var p string
		var err error
		name, p, err = net.SplitHostPort(host)
		if err != nil || name == "" {
			return bad()
		}
		inline, err = strconv.Atoi(p)
		if err != nil || inline < 1 || inline > 65535 {
			return bad()
		}
	}
	if name == "" {
		return bad()
	}
	return name, inline, nil
}

func sabHTTPClient() *http.Client {
	return &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

// NewDownloadClient returns a downloada client for the given configuration
// TODO: handle supporting configurations such as timeouts, etc
func (d DownloadClientFactory) NewDownloadClient(config sqlcdb.DownloadClient) (DownloadClient, error) {
	switch config.Implementation {
	case "transmission":
		// TODO: Replace default http client with stored configurations
		return NewTransmissionClient(http.DefaultClient, config.Scheme, config.Host, d.downloadMountPrefix, int(config.Port)), nil
	case "sabnzbd":
		if !config.ApiKey.Valid || config.ApiKey.String == "" {
			return nil, errors.New("missing api key")
		}
		if err := ValidateSABConfig(config.Scheme, config.Host, config.Port); err != nil {
			return nil, err
		}
		hostname, inline, _ := parseSABHost(config.Host)
		host := config.Host
		if inline == 0 {
			if config.Port != 0 {
				host = net.JoinHostPort(hostname, strconv.FormatInt(config.Port, 10))
			} else if net.ParseIP(hostname) != nil && strings.Contains(hostname, ":") {
				host = "[" + hostname + "]"
			}
		}
		return NewSabnzbdClient(sabHTTPClient(), config.Scheme, host, d.downloadMountPrefix, config.ApiKey.String), nil
	default:
		return nil, fmt.Errorf("unsupported client implementation: %v", config.Implementation)
	}
}

type AddRequest struct {
	Release *indexer.Release
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
