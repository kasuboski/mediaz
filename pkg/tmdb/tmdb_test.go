package tmdb

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseMediaDetailsResponse(t *testing.T) {
	// Test setup
	t.Run("Test successful response", func(t *testing.T) {
		res := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBuffer([]byte(`{"id": 1, "adult": false, "backdrop_path": "/path/to/backdrop"}`))),
		}

		results, err := parseMediaDetailsResponse(res)
		assert.NoError(t, err)
		assert.NotNil(t, results)
	})

	t.Run("Test response with status code other than 200", func(t *testing.T) {
		res := &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(bytes.NewBuffer([]byte(`{"error": "not found"}`))),
		}

		_, err := parseMediaDetailsResponse(res)
		assert.Error(t, err)
	})

	// Test edge cases
	t.Run("Test empty response body", func(t *testing.T) {
		res := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBuffer([]byte(`{}`))),
		}

		_, err := parseMediaDetailsResponse(res)
		assert.Error(t, err)
	})

	// Test invalid JSON response
	t.Run("Test response with invalid JSON", func(t *testing.T) {
		res := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBuffer([]byte(`{"a": 1, "b": 2}`))),
		}

		_, err := parseMediaDetailsResponse(res)
		assert.Error(t, err)
	})

	// Test unmarshalling large JSON objects
	t.Run("Test response with very large JSON object", func(t *testing.T) {
		res := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBuffer([]byte(strings.Repeat("{\"key\":\"value\"}", 1000)))),
		}

		_, err := parseMediaDetailsResponse(res)
		assert.Error(t, err)
	})

	// Test nil response
	t.Run("Test nil response", func(t *testing.T) {
		res := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBuffer(nil)),
		}

		_, err := parseMediaDetailsResponse(res)
		assert.Error(t, err)
	})
}
