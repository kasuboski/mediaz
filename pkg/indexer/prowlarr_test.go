package indexer

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/prowlarr/mocks"
	"github.com/kasuboski/mediaz/pkg/ptr"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/oapi-codegen/nullable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// mockIProwlarr wraps MockClientInterface to satisfy IProwlarr.
type mockIProwlarr struct {
	*mocks.MockClientInterface
	apiKey string
	apiURL string
}

func (m *mockIProwlarr) GetAPIKey() string { return m.apiKey }
func (m *mockIProwlarr) GetAPIURL() string { return m.apiURL }

func newMockIProwlarr(ctrl *gomock.Controller) *mockIProwlarr {
	return &mockIProwlarr{
		MockClientInterface: mocks.NewMockClientInterface(ctrl),
		apiKey:              "test-key",
		apiURL:              "http://localhost:9696",
	}
}

func TestNewIndexerSourceFactory(t *testing.T) {
	f := NewIndexerSourceFactory()
	require.NotNil(t, f)
}

func TestIndexerSourceFactory_NewIndexerSource(t *testing.T) {
	t.Run("returns error for disabled source", func(t *testing.T) {
		f := NewIndexerSourceFactory()
		_, err := f.NewIndexerSource(model.IndexerSource{
			Enabled:        false,
			Implementation: "prowlarr",
			Scheme:         "http",
			Host:           "localhost",
			APIKey:         ptr.To("key"),
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "disabled")
	})

	t.Run("returns error for unsupported implementation", func(t *testing.T) {
		f := NewIndexerSourceFactory()
		_, err := f.NewIndexerSource(model.IndexerSource{
			Enabled:        true,
			Implementation: "unsupported",
			Scheme:         "http",
			Host:           "localhost",
			APIKey:         ptr.To("key"),
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported indexer source implementation")
	})

	t.Run("creates prowlarr source", func(t *testing.T) {
		f := NewIndexerSourceFactory()
		src, err := f.NewIndexerSource(model.IndexerSource{
			Enabled:        true,
			Implementation: "prowlarr",
			Scheme:         "http",
			Host:           "localhost",
			APIKey:         ptr.To("my-key"),
		})
		require.NoError(t, err)
		assert.NotNil(t, src)
	})
}

func TestNewProwlarrSource(t *testing.T) {
	t.Run("returns error when api_key is nil", func(t *testing.T) {
		_, err := NewProwlarrSource(model.IndexerSource{
			Scheme: "http",
			Host:   "localhost",
			APIKey: nil,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "api_key")
	})

	t.Run("creates source without port", func(t *testing.T) {
		src, err := NewProwlarrSource(model.IndexerSource{
			Scheme: "http",
			Host:   "localhost",
			APIKey: ptr.To("my-key"),
		})
		require.NoError(t, err)
		assert.NotNil(t, src)
		assert.Equal(t, "my-key", src.client.GetAPIKey())
		assert.Equal(t, "http://localhost", src.client.GetAPIURL())
	})

	t.Run("creates source with custom port", func(t *testing.T) {
		src, err := NewProwlarrSource(model.IndexerSource{
			Scheme: "http",
			Host:   "localhost",
			Port:   ptr.To(int32(9696)),
			APIKey: ptr.To("my-key"),
		})
		require.NoError(t, err)
		assert.NotNil(t, src)
		assert.Contains(t, src.client.GetAPIURL(), "9696")
	})

	t.Run("omits standard port 80", func(t *testing.T) {
		src, err := NewProwlarrSource(model.IndexerSource{
			Scheme: "http",
			Host:   "localhost",
			Port:   ptr.To(int32(80)),
			APIKey: ptr.To("my-key"),
		})
		require.NoError(t, err)
		assert.NotNil(t, src)
		assert.Equal(t, "http://localhost", src.client.GetAPIURL())
	})

	t.Run("omits standard port 443", func(t *testing.T) {
		src, err := NewProwlarrSource(model.IndexerSource{
			Scheme: "https",
			Host:   "localhost",
			Port:   ptr.To(int32(443)),
			APIKey: ptr.To("my-key"),
		})
		require.NoError(t, err)
		assert.NotNil(t, src)
		assert.Equal(t, "https://localhost", src.client.GetAPIURL())
	})
}

func TestProwlarrIndexerSource_ListIndexers(t *testing.T) {
	ctx := context.Background()

	t.Run("returns indexers from prowlarr", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockClient := newMockIProwlarr(ctrl)

		indexerJSON := []prowlarr.IndexerResource{
			{
				ID:          ptr.To(int32(1)),
				Name:        nullable.NewNullableWithValue("test-indexer"),
				IndexerUrls: nullable.NewNullableWithValue([]string{"http://example.com"}),
				Priority:    ptr.To(int32(25)),
				Capabilities: &prowlarr.IndexerCapabilityResource{
					Categories: nullable.NewNullableWithValue([]prowlarr.IndexerCategory{
						{ID: ptr.To(int32(2000)), Name: nullable.NewNullableWithValue("Movies")},
					}),
				},
				Status: &prowlarr.IndexerStatusResource{},
			},
		}
		body, err := json.Marshal(indexerJSON)
		require.NoError(t, err)

		mockClient.EXPECT().GetAPIV1Indexer(gomock.Any(), gomock.Any()).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(body)),
		}, nil)

		src := &ProwlarrIndexerSource{client: mockClient, config: model.IndexerSource{}}
		got, err := src.ListIndexers(ctx)
		require.NoError(t, err)

		expected := []SourceIndexer{
			{
				ID:       1,
				Name:     "test-indexer",
				URI:      "http://example.com",
				Priority: 25,
				Categories: []prowlarr.IndexerCategory{
					{ID: ptr.To(int32(2000)), Name: nullable.NewNullableWithValue("Movies")},
				},
				Status: &prowlarr.IndexerStatusResource{},
			},
		}
		assert.Equal(t, expected, got)
	})

	t.Run("returns error when http request fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockClient := newMockIProwlarr(ctrl)

		mockClient.EXPECT().GetAPIV1Indexer(gomock.Any(), gomock.Any()).Return(nil, io.ErrUnexpectedEOF)

		src := &ProwlarrIndexerSource{client: mockClient, config: model.IndexerSource{}}
		_, err := src.ListIndexers(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to fetch indexers")
	})

	t.Run("returns error on non-200 status", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockClient := newMockIProwlarr(ctrl)

		mockClient.EXPECT().GetAPIV1Indexer(gomock.Any(), gomock.Any()).Return(&http.Response{
			StatusCode: http.StatusInternalServerError,
			Status:     "500 Internal Server Error",
			Body:       io.NopCloser(bytes.NewReader(nil)),
		}, nil)

		src := &ProwlarrIndexerSource{client: mockClient, config: model.IndexerSource{}}
		_, err := src.ListIndexers(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected status")
	})

	t.Run("handles indexers with missing optional fields", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockClient := newMockIProwlarr(ctrl)

		indexerJSON := []prowlarr.IndexerResource{
			{
				ID:           ptr.To(int32(2)),
				Name:         nullable.NewNullableWithValue("minimal"),
				Priority:     ptr.To(int32(50)),
				Capabilities: &prowlarr.IndexerCapabilityResource{},
			},
		}
		body, err := json.Marshal(indexerJSON)
		require.NoError(t, err)

		mockClient.EXPECT().GetAPIV1Indexer(gomock.Any(), gomock.Any()).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(body)),
		}, nil)

		src := &ProwlarrIndexerSource{client: mockClient, config: model.IndexerSource{}}
		got, err := src.ListIndexers(ctx)
		require.NoError(t, err)
		require.Len(t, got, 1)

		expected := SourceIndexer{
			ID:         2,
			Name:       "minimal",
			URI:        "",
			Priority:   50,
			Categories: nil,
			Status:     (*prowlarr.IndexerStatusResource)(nil),
		}
		assert.Equal(t, expected, got[0])
	})

	t.Run("returns error on invalid json", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockClient := newMockIProwlarr(ctrl)

		mockClient.EXPECT().GetAPIV1Indexer(gomock.Any(), gomock.Any()).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader([]byte("not json"))),
		}, nil)

		src := &ProwlarrIndexerSource{client: mockClient, config: model.IndexerSource{}}
		_, err := src.ListIndexers(ctx)
		require.Error(t, err)
	})
}

func TestProwlarrIndexerSource_Search(t *testing.T) {
	ctx := context.Background()

	t.Run("searches with query only", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockClient := newMockIProwlarr(ctrl)

		releases := []*prowlarr.ReleaseResource{
			{Title: nullable.NewNullableWithValue("Movie.2024.1080p.mkv")},
		}
		body, err := json.Marshal(releases)
		require.NoError(t, err)

		mockClient.EXPECT().GetAPIV1Search(gomock.Any(), gomock.Any(), gomock.Any()).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(body)),
		}, nil)

		src := &ProwlarrIndexerSource{client: mockClient, config: model.IndexerSource{}}
		got, err := src.Search(ctx, 1, nil, SearchOptions{Query: "Movie"})
		require.NoError(t, err)
		assert.Equal(t, releases, got)
	})

	t.Run("formats season and episode in query", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockClient := newMockIProwlarr(ctrl)

		body, err := json.Marshal([]*prowlarr.ReleaseResource{})
		require.NoError(t, err)

		season := int32(3)
		episode := int32(7)
		mockClient.EXPECT().GetAPIV1Search(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, params *prowlarr.GetAPIV1SearchParams, _ ...interface{}) (*http.Response, error) {
				assert.Equal(t, "Show S03E07", *params.Query)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader(body)),
				}, nil
			},
		)

		src := &ProwlarrIndexerSource{client: mockClient, config: model.IndexerSource{}}
		_, err = src.Search(ctx, 1, nil, SearchOptions{
			Query:   "Show",
			Season:  &season,
			Episode: &episode,
		})
		require.NoError(t, err)
	})

	t.Run("formats season without episode", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockClient := newMockIProwlarr(ctrl)

		body, err := json.Marshal([]*prowlarr.ReleaseResource{})
		require.NoError(t, err)

		season := int32(2)
		mockClient.EXPECT().GetAPIV1Search(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, params *prowlarr.GetAPIV1SearchParams, _ ...interface{}) (*http.Response, error) {
				assert.Equal(t, "Show S02", *params.Query)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader(body)),
				}, nil
			},
		)

		src := &ProwlarrIndexerSource{client: mockClient, config: model.IndexerSource{}}
		_, err = src.Search(ctx, 1, nil, SearchOptions{
			Query:  "Show",
			Season: &season,
		})
		require.NoError(t, err)
	})

	t.Run("returns error on non-200 status", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockClient := newMockIProwlarr(ctrl)

		mockClient.EXPECT().GetAPIV1Search(gomock.Any(), gomock.Any(), gomock.Any()).Return(&http.Response{
			StatusCode: http.StatusBadGateway,
			Status:     "502 Bad Gateway",
			Body:       io.NopCloser(bytes.NewReader(nil)),
		}, nil)

		src := &ProwlarrIndexerSource{client: mockClient, config: model.IndexerSource{}}
		_, err := src.Search(ctx, 1, nil, SearchOptions{Query: "Movie"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected status")
	})

	t.Run("returns error when client fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockClient := newMockIProwlarr(ctrl)

		mockClient.EXPECT().GetAPIV1Search(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, io.ErrUnexpectedEOF)

		src := &ProwlarrIndexerSource{client: mockClient, config: model.IndexerSource{}}
		_, err := src.Search(ctx, 1, nil, SearchOptions{Query: "Movie"})
		require.Error(t, err)
	})

	t.Run("returns error on invalid json response", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockClient := newMockIProwlarr(ctrl)

		mockClient.EXPECT().GetAPIV1Search(gomock.Any(), gomock.Any(), gomock.Any()).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader([]byte("not json"))),
		}, nil)

		src := &ProwlarrIndexerSource{client: mockClient, config: model.IndexerSource{}}
		_, err := src.Search(ctx, 1, nil, SearchOptions{Query: "Movie"})
		require.Error(t, err)
	})
}
