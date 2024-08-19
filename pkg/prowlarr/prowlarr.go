package prowlarr

import (
	"context"
	"net/http"
)

func SetRequestAPIKey(apiKey string) func(ctx context.Context, req *http.Request) error {
	return func(ctx context.Context, req *http.Request) error {
		q := req.URL.Query()
		q.Set("apikey", apiKey)
		req.URL.RawQuery = q.Encode()
		req.Header.Add("Accept", "application/json")
		return nil
	}
}
