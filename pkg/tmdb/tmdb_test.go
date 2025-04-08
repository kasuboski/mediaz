package tmdb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	mhttp "github.com/kasuboski/mediaz/pkg/http/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
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

func TestTMDBClient_GetSeriesDetails(t *testing.T) {
	t.Run("error getting series details", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		ctx := context.Background()

		mockHttp := mhttp.NewMockHTTPClient(ctrl)
		mockHttp.EXPECT().
			Do(gomock.Any()).
			Return(nil, fmt.Errorf("failed to get series details"))

		client, err := New("https://api.themoviedb.org", "1234", WithHTTPClient(mockHttp))
		assert.NoError(t, err)
		_, err = client.GetSeriesDetails(ctx, 123)
		assert.Error(t, err)
		assert.Equal(t, "failed to get series details", err.Error())
	})

	t.Run("error unmarshalling response", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		ctx := context.Background()

		mockHttp := mhttp.NewMockHTTPClient(ctrl)
		mockHttp.EXPECT().
			Do(gomock.Any()).
			Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBuffer([]byte(`{"id": 1, "adult": false, "backdrop_path": "/path"`))),
			}, nil)

		client, err := New("https://api.themoviedb.org", "1234", WithHTTPClient(mockHttp))
		assert.NoError(t, err)
		_, err = client.GetSeriesDetails(ctx, 123)
		assert.Error(t, err)
		assert.Equal(t, "unexpected end of JSON input", err.Error())
	})

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		ctx := context.Background()

		mockHttp := mhttp.NewMockHTTPClient(ctrl)

		SeriesDetails := SeriesDetailsResponse{
			ID: 1,
			Seasons: []SeriesDetailsResponseSeason{
				{
					ID: 1,
				},
			},
		}

		detailsBytes, err := json.Marshal(SeriesDetails)
		require.NoError(t, err)

		seasonDetails := TvSeasonDetails{
			ID: 1,
			Episodes: []Episode{
				{
					ID: 12,
				},
			},
		}

		seasonDetailsBytes, err := json.Marshal(seasonDetails)
		require.NoError(t, err)

		gomock.InOrder(
			mockHttp.EXPECT().
				Do(gomock.Any()).
				Return(&http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBuffer(detailsBytes)),
				}, nil),

			mockHttp.EXPECT().
				Do(gomock.Any()).
				Return(&http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBuffer(seasonDetailsBytes)),
				}, nil),
		)

		client, err := New("https://api.themoviedb.org", "1234", WithHTTPClient(mockHttp))
		require.NoError(t, err)

		details, err := client.GetSeriesDetails(ctx, 123)
		assert.NoError(t, err)

		assert.Equal(t, 1, details.ID)
		assert.Equal(t, 1, len(details.Seasons))
		assert.Equal(t, 1, details.Seasons[0].ID)
		assert.Equal(t, 1, len(details.Seasons[0].Episodes))
	})
}

func TestTvSeasonDetails_ToSeason(t *testing.T) {
	type fields struct {
		Identifier   string
		AirDate      string
		Episodes     []Episode
		ID           int
		Name         string
		Overview     string
		PosterPath   string
		SeasonNumber int
		VoteAverage  float32
	}
	tests := []struct {
		name   string
		fields fields
		want   Season
	}{
		{
			name: "Test ToSeason",
			fields: fields{
				Identifier: "test-identifier",
				AirDate:    "2023-10-01",
				Episodes: []Episode{
					{
						ID: 12,
					},
				},
				ID:           1,
				Name:         "Test Name",
				Overview:     "Test Overview",
				PosterPath:   "Test Poster Path",
				SeasonNumber: 1,
				VoteAverage:  1.0,
			},
			want: Season{
				ID:           1,
				AirDate:      "2023-10-01",
				Name:         "Test Name",
				PosterPath:   "Test Poster Path",
				SeasonNumber: 1,
				Overview:     "Test Overview",
				Episodes: []Episode{
					{
						ID:         12,
						GuestStars: []GuestStar{},
						Crew:       []Crew{},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := TvSeasonDetails{
				Identifier:   tt.fields.Identifier,
				AirDate:      tt.fields.AirDate,
				Episodes:     tt.fields.Episodes,
				ID:           tt.fields.ID,
				Name:         tt.fields.Name,
				Overview:     tt.fields.Overview,
				PosterPath:   tt.fields.PosterPath,
				SeasonNumber: tt.fields.SeasonNumber,
				VoteAverage:  tt.fields.VoteAverage,
			}

			assert.Equal(t, tr.ToSeason(), tt.want)
		})
	}
}
