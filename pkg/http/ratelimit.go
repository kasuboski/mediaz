package http

import (
	"fmt"
	"net/http"
	"strconv"
	"time"
)

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

const (
	DefaultMaxRetries  = 3
	DefaultBaseBackoff = time.Millisecond * 500
)

type RateLimitedClient struct {
	client      HTTPClient
	baseBackoff time.Duration
	maxRetries  int
}

// ClientOption is a function that can be used to configure a RateLimitedHTTPClient
type ClientOption func(*RateLimitedClient)

// NewRateLimitedHTTPClient creates a new RateLimitedHTTPClient that respects 429 status codes
func NewRateLimitedHTTPClient(opts ...ClientOption) *RateLimitedClient {
	c := &RateLimitedClient{
		client:      http.DefaultClient,
		maxRetries:  DefaultMaxRetries,
		baseBackoff: DefaultBaseBackoff,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// WithMaxRetries sets the maximum number of retries for the client
func WithMaxRetries(maxRetries int) ClientOption {
	return func(c *RateLimitedClient) {
		c.maxRetries = maxRetries
	}
}

// WithBaseBackoff sets the base backoff time for the client
func WithBaseBackoff(baseBackoff time.Duration) ClientOption {
	return func(c *RateLimitedClient) {
		c.baseBackoff = baseBackoff
	}
}

// WithHTTPClient sets the http client to use for the client
func WithHTTPClient(client HTTPClient) ClientOption {
	return func(c *RateLimitedClient) {
		c.client = client
	}
}

// Do executes the HTTP request while respecting 429 rate limits
// This is a blocking call until the request completes successfully or the backoff reaches the maximum retries
// If the maximum number of retries is reached, the response returned will be the last response received
func (c *RateLimitedClient) Do(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	for attempt := 0; attempt < c.maxRetries; attempt++ {
		resp, err = c.client.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusTooManyRequests {
			return resp, nil
		}

		retryAfter := c.getRetryAfter(resp, attempt)
		resp.Body.Close()

		ticker := time.NewTicker(retryAfter)
		<-ticker.C
		ticker.Stop()
	}

	return resp, fmt.Errorf("rate limit exceeded after %d retries", c.maxRetries)
}

// getRetryAfter calculates the appropriate retry delay
func (c *RateLimitedClient) getRetryAfter(resp *http.Response, attempt int) time.Duration {
	retryAfterHeader := resp.Header.Get("Retry-After")

	if retryAfterHeader != "" {
		seconds, err := strconv.Atoi(retryAfterHeader)
		if err == nil {
			return time.Duration(seconds) * time.Second
		}
	}

	// 2^n backoff
	return time.Duration(1<<attempt) * c.baseBackoff
}
