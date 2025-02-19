package http

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/exp/rand"
)

const (
	DefaultMaxRetries  = 3
	DefaultBaseBackoff = time.Millisecond * 500
)

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type RateLimitedClient struct {
	mu          sync.Mutex
	client      HTTPClient
	baseBackoff time.Duration
	maxRetries  int
}

// ClientOption is a function that can be used to configure a RateLimitedHTTPClient
type ClientOption func(*RateLimitedClient)

// NewRateLimitedHTTPClient creates a new RateLimitedHTTPClient that respects 429 status codes
// The client can be used concurrently
func NewRateLimitedHTTPClient(opts ...ClientOption) *RateLimitedClient {
	c := &RateLimitedClient{
		client:      http.DefaultClient,
		maxRetries:  DefaultMaxRetries,
		baseBackoff: DefaultBaseBackoff,
		mu:          sync.Mutex{},
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

func (c *RateLimitedClient) getBackoff() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.baseBackoff
}

func (c *RateLimitedClient) getMaxRetries() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.maxRetries
}

// Do executes the HTTP request while respecting 429 rate limits
// This is a blocking call until the request completes successfully or the backoff reaches the maximum retries
// If the maximum number of retries is reached, the response returned will be the last response received
func (c *RateLimitedClient) Do(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	for attempt := 0; attempt < c.getMaxRetries(); attempt++ {
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

	baseBackoff := c.getBackoff()

	// 2^n backoff
	expBackoff := time.Duration(1<<attempt) * baseBackoff

	// staggers the backoff to avoid a thundering herd
	jitter := time.Duration(rand.Int63n(int64(baseBackoff)))

	return expBackoff + jitter
}
