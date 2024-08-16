package tmdb

import (
	"context"
	"net/http"
)

func SetRequestAPIKey(apiKey string) func(ctx context.Context, req *http.Request) error {
	return func(ctx context.Context, req *http.Request) error {
		req.Header.Add("Authorization", "Bearer "+apiKey)
		req.Header.Add("accept", "application/json")
		return nil
	}
}
