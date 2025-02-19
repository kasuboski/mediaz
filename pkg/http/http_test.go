package http

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/pkg/http/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestNewRateLimitedHTTPClient(t *testing.T) {
	type args struct {
		opts []ClientOption
	}
	tests := []struct {
		name string
		args args
		want *RateLimitedClient
	}{
		{
			name: "default",
			args: args{
				opts: []ClientOption{},
			},
			want: &RateLimitedClient{
				client:      http.DefaultClient,
				maxRetries:  DefaultMaxRetries,
				baseBackoff: DefaultBaseBackoff,
			},
		},
		{
			name: "custom",
			args: args{
				opts: []ClientOption{
					WithMaxRetries(5),
					WithBaseBackoff(time.Millisecond * 100),
					WithHTTPClient(&http.Client{
						Transport: &http.Transport{
							MaxIdleConns: 10,
						},
					}),
				},
			},
			want: &RateLimitedClient{
				client: &http.Client{
					Transport: &http.Transport{
						MaxIdleConns: 10,
					},
				},
				maxRetries:  5,
				baseBackoff: time.Millisecond * 100,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewRateLimitedHTTPClient(tt.args.opts...); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewRateLimitedHTTPClient() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRateLimitedHTTPClient_Do(t *testing.T) {
	t.Run("error during request", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mhttp := mocks.NewMockHTTPClient(ctrl)

		req, err := http.NewRequest("GET", "https://example.com", nil)
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
			return
		}

		mhttp.EXPECT().Do(req).Return(nil, errors.New("http error"))
		client := NewRateLimitedHTTPClient(WithHTTPClient(mhttp))
		resp, err := client.Do(req)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("non 429 response", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mhttp := mocks.NewMockHTTPClient(ctrl)

		req, err := http.NewRequest("GET", "https://example.com", nil)
		if err != nil {
			t.Errorf("failed to create request: %v", err)
			return
		}

		mhttp.EXPECT().Do(req).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBuffer([]byte("non 429 response"))),
		}, nil)

		client := NewRateLimitedHTTPClient(WithHTTPClient(mhttp))
		resp, err := client.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		b, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Errorf("failed to read response body: %v", err)
			return
		}
		assert.Equal(t, "non 429 response", string(b))
	})

	t.Run("429 response - max retries", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mhttp := mocks.NewMockHTTPClient(ctrl)

		req, err := http.NewRequest("GET", "https://example.com", nil)
		if err != nil {
			t.Errorf("failed to create request: %v", err)
			return
		}

		mhttp.EXPECT().Do(req).Return(&http.Response{
			StatusCode: http.StatusTooManyRequests,
			Body:       io.NopCloser(bytes.NewBuffer([]byte("429 response"))),
		}, nil)
		client := NewRateLimitedHTTPClient(WithHTTPClient(mhttp), WithMaxRetries(1))
		resp, err := client.Do(req)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("429 response - with retry header", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mhttp := mocks.NewMockHTTPClient(ctrl)

		req, err := http.NewRequest("GET", "https://example.com", nil)
		if err != nil {
			t.Errorf("failed to create request: %v", err)
			return
		}

		mhttp.EXPECT().Do(req).Return(&http.Response{
			StatusCode: http.StatusTooManyRequests,
			Header: http.Header{
				"Retry-After": []string{"1"},
			},
			Body: io.NopCloser(bytes.NewBuffer([]byte("429 response"))),
		}, nil)
		client := NewRateLimitedHTTPClient(WithHTTPClient(mhttp), WithMaxRetries(1))
		resp, err := client.Do(req)
		assert.ErrorContains(t, err, "rate limit exceeded after 1 retries")
		assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	})
}
