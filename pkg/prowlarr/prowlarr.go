package prowlarr

import (
	"context"
	"net/http"
)

var _ IProwlarr = &ProwlarrClient{}

type IProwlarr interface {
	ClientInterface
	GetAPIKey() string
	GetAPIURL() string
}

type ProwlarrClient struct {
	ClientInterface
	apiKey string
	server string
}

func New(url, apiKey string) (*ProwlarrClient, error) {
	client, err := NewClient(url, WithRequestEditorFn(SetRequestAPIKey(apiKey)))
	if err != nil {
		return nil, err
	}
	return &ProwlarrClient{
		ClientInterface: client,
		apiKey:          apiKey,
		server:          url,
	}, nil
}

func (ci ProwlarrClient) GetAPIKey() string {
	return ci.apiKey
}

func (ci ProwlarrClient) GetAPIURL() string {
	return ci.server
}

func SetRequestAPIKey(apiKey string) func(ctx context.Context, req *http.Request) error {
	return func(ctx context.Context, req *http.Request) error {
		q := req.URL.Query()
		q.Set("apikey", apiKey)
		req.URL.RawQuery = q.Encode()
		req.Header.Add("Accept", "application/json")
		return nil
	}
}
