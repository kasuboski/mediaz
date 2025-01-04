package download

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	httpMock "github.com/kasuboski/mediaz/pkg/download/mocks/http"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/oapi-codegen/nullable"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestNewTransmissionClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockHttp := httpMock.NewMockHTTPClient(ctrl)

	client := NewTransmissionClient(mockHttp, "http", "localhost", 0)
	transmissionClient, ok := client.(*TransmissionClient)
	assert.True(t, ok, "client should be of type *TransmissionClient")
	assert.Equal(t, "localhost", transmissionClient.host, "Host should not include port")
	assert.Equal(t, mockHttp, transmissionClient.http, "HTTP client should match")
	assert.Equal(t, "http", transmissionClient.scheme, "Scheme should match")
	assert.NotNil(t, transmissionClient.mutex, "Mutex should not be nil")

	clientWithPort := NewTransmissionClient(mockHttp, "https", "example.com", 9090)
	transmissionClientWithPort, ok := clientWithPort.(*TransmissionClient)
	assert.True(t, ok, "client should be of type *TransmissionClient")
	assert.Equal(t, "example.com:9090", transmissionClientWithPort.host, "Host should include port")
	assert.Equal(t, mockHttp, transmissionClientWithPort.http, "HTTP client should match")
	assert.Equal(t, "https", transmissionClientWithPort.scheme, "Scheme should match")
	assert.NotNil(t, transmissionClientWithPort.mutex, "Mutex should not be nil")
}

func TestTransmissionClient_Add(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t.Run("success", func(t *testing.T) {
		mockHttp := httpMock.NewMockHTTPClient(ctrl)
		client := NewTransmissionClient(mockHttp, "http", "localhost", 0)
		ctx := context.Background()

		addRequest := AddRequest{
			Release: &prowlarr.ReleaseResource{GUID: nullable.NewNullableWithValue[string]("http://example.com/torrent")},
		}

		addResponse := AddTorrentResponse{
			Arguments: AddTorrentResponseArguments{
				TorrentAdded: AddedTorrent{
					HashString: "hash123",
					Name:       "torrent 1",
					ID:         1,
				},
			},
			Result: "success",
		}

		addResponseBody, err := json.Marshal(addResponse)
		assert.NoError(t, err)

		getResponse := TransmissionListTorrentsResponse{
			Arguments: TorrentList{
				Torrents: []TransmissionTorrent{
					{
						ID:          1,
						Name:        "torrent 1",
						DownloadDir: "/downloads",
						Files: []TransmissionFile{
							{
								Name: "file1",
							},
						},
					},
				},
			},
			Result: "success",
		}

		getResponseBody, err := json.Marshal(getResponse)
		assert.NoError(t, err)

		addMock := mockHttp.EXPECT().Do(gomock.Any()).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBuffer(addResponseBody)),
		}, nil)

		getMock := mockHttp.EXPECT().Do(gomock.Any()).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBuffer(getResponseBody)),
		}, nil)

		gomock.InOrder(addMock, getMock)

		status, err := client.Add(ctx, addRequest)
		assert.NoError(t, err)

		expectedStatus := Status{
			ID:       "1",
			Name:     "torrent 1",
			FilePath: []string{"/downloads/file1"},
		}
		assert.Equal(t, expectedStatus, status)
	})

	t.Run("missing guid", func(t *testing.T) {
		mockHttp := httpMock.NewMockHTTPClient(ctrl)
		client := NewTransmissionClient(mockHttp, "http", "localhost", 0)
		ctx := context.Background()

		addRequest := AddRequest{
			Release: &prowlarr.ReleaseResource{GUID: nullable.NewNullNullable[string]()},
		}

		status, err := client.Add(ctx, addRequest)

		assert.Error(t, err)
		assert.Equal(t, Status{}, status)
	})

	t.Run("error during request", func(t *testing.T) {
		mockHttp := httpMock.NewMockHTTPClient(ctrl)
		client := NewTransmissionClient(mockHttp, "http", "localhost", 0)
		ctx := context.Background()

		addRequest := AddRequest{
			Release: &prowlarr.ReleaseResource{GUID: nullable.NewNullableWithValue[string]("http://example.com/torrent")},
		}

		mockHttp.EXPECT().Do(gomock.Any()).Return(nil, fmt.Errorf("http error"))

		status, err := client.Add(ctx, addRequest)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "http error")
		assert.Equal(t, Status{}, status)
	})

	t.Run("error in response", func(t *testing.T) {
		mockHttp := httpMock.NewMockHTTPClient(ctrl)
		client := NewTransmissionClient(mockHttp, "http", "localhost", 0)
		ctx := context.Background()

		addRequest := AddRequest{
			Release: &prowlarr.ReleaseResource{GUID: nullable.NewNullableWithValue[string]("http://example.com/torrent")},
		}

		addResponse := AddTorrentResponse{
			Result: "error",
		}

		addResponseBody, err := json.Marshal(addResponse)
		assert.NoError(t, err)

		mockHttp.EXPECT().Do(gomock.Any()).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBuffer(addResponseBody)),
		}, nil)

		status, err := client.Add(ctx, addRequest)

		assert.NotNil(t, err)
		assert.Equal(t, Status{}, status)
	})

	t.Run("Error during Get RPC Call", func(t *testing.T) {
		mockHttp := httpMock.NewMockHTTPClient(ctrl)
		client := NewTransmissionClient(mockHttp, "http", "localhost", 0)
		ctx := context.Background()

		addRequest := AddRequest{
			Release: &prowlarr.ReleaseResource{GUID: nullable.NewNullableWithValue[string]("http://example.com/torrent")},
		}

		// Mock the successful Add response
		addResponse := AddTorrentResponse{
			Result: "success",
			Arguments: AddTorrentResponseArguments{
				TorrentAdded: AddedTorrent{
					HashString: "hash123",
					Name:       "torrent 1",
					ID:         1,
				},
			},
		}

		// Serialize the Add response
		addResponseBody, err := json.Marshal(addResponse)
		assert.NoError(t, err)

		// Mock the HTTP behavior for Add request
		mockHttp.EXPECT().Do(gomock.Any()).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBuffer(addResponseBody)),
		}, nil)

		// Mock error during Get request
		mockHttp.EXPECT().Do(gomock.Any()).Return(nil, fmt.Errorf("http error during Get"))

		// Call the Add method
		status, err := client.Add(ctx, addRequest)

		// Ensure an error is returned due to HTTP failure during the Get call
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "http error during Get")
		assert.Equal(t, Status{}, status)
	})
}

func TestTransmissionClient_Get(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t.Run("success", func(t *testing.T) {
		mockHttp := httpMock.NewMockHTTPClient(ctrl)
		client := NewTransmissionClient(mockHttp, "http", "localhost", 0)
		ctx := context.Background()

		getRequest := GetRequest{
			ID: "1",
		}

		getResponse := TransmissionListTorrentsResponse{
			Arguments: TorrentList{
				Torrents: []TransmissionTorrent{
					{
						ID:   1,
						Name: "torrent 1",
					},
				},
			},
			Result: "success",
		}

		getResponseBody, err := json.Marshal(getResponse)
		assert.NoError(t, err)

		mockHttp.EXPECT().Do(gomock.Any()).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBuffer(getResponseBody)),
		}, nil)

		status, err := client.Get(ctx, getRequest)
		assert.NoError(t, err)

		expectedStatus := Status{
			ID:   "1",
			Name: "torrent 1",
		}
		assert.Equal(t, expectedStatus, status)
	})

	t.Run("error", func(t *testing.T) {
		mockHttp := httpMock.NewMockHTTPClient(ctrl)
		client := NewTransmissionClient(mockHttp, "http", "localhost", 0)
		ctx := context.Background()

		getRequest := GetRequest{
			ID: "1",
		}

		mockHttp.EXPECT().Do(gomock.Any()).Return(nil, fmt.Errorf("http error"))

		status, err := client.Get(ctx, getRequest)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "http error")
		assert.Equal(t, Status{}, status)
	})

	t.Run("error in response", func(t *testing.T) {
		mockHttp := httpMock.NewMockHTTPClient(ctrl)
		client := NewTransmissionClient(mockHttp, "http", "localhost", 0)
		ctx := context.Background()

		getRequest := GetRequest{
			ID: "1",
		}

		getResponse := TransmissionListTorrentsResponse{
			Arguments: TorrentList{
				Torrents: []TransmissionTorrent{},
			},
			Result: "error",
		}

		getResponseBody, err := json.Marshal(getResponse)
		assert.NoError(t, err)

		mockHttp.EXPECT().Do(gomock.Any()).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBuffer(getResponseBody)),
		}, nil)

		status, err := client.Get(ctx, getRequest)
		assert.Error(t, err)
		assert.Equal(t, Status{}, status)
	})
}

func TestTransmissionClient_List(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t.Run("success", func(t *testing.T) {
		mockHttp := httpMock.NewMockHTTPClient(ctrl)
		client := NewTransmissionClient(mockHttp, "http", "localhost", 0)
		ctx := context.Background()

		getResponse := TransmissionListTorrentsResponse{
			Arguments: TorrentList{
				Torrents: []TransmissionTorrent{
					{
						ID:   1,
						Name: "torrent 1",
					},
					{
						ID:   2,
						Name: "torrent 2",
					},
				},
			},
			Result: "success",
		}

		getResponseBody, err := json.Marshal(getResponse)
		assert.NoError(t, err)

		mockHttp.EXPECT().Do(gomock.Any()).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBuffer(getResponseBody)),
		}, nil)

		status, err := client.List(ctx)
		assert.NoError(t, err)

		want := []Status{
			{
				ID:   "1",
				Name: "torrent 1",
			},
			{
				ID:   "2",
				Name: "torrent 2",
			},
		}
		assert.Equal(t, want, status)
	})

	t.Run("error during request", func(t *testing.T) {
		mockHttp := httpMock.NewMockHTTPClient(ctrl)
		client := NewTransmissionClient(mockHttp, "http", "localhost", 0)
		ctx := context.Background()

		mockHttp.EXPECT().Do(gomock.Any()).Return(nil, fmt.Errorf("http error"))

		status, err := client.List(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "http error")
		assert.Nil(t, status)
	})

	t.Run("error in response", func(t *testing.T) {
		mockHttp := httpMock.NewMockHTTPClient(ctrl)
		client := NewTransmissionClient(mockHttp, "http", "localhost", 0)
		ctx := context.Background()

		getResponse := TransmissionListTorrentsResponse{
			Arguments: TorrentList{
				Torrents: []TransmissionTorrent{},
			},
			Result: "error",
		}

		getResponseBody, err := json.Marshal(getResponse)
		assert.NoError(t, err)

		mockHttp.EXPECT().Do(gomock.Any()).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBuffer(getResponseBody)),
		}, nil)

		statuses, err := client.List(ctx)
		assert.Error(t, err)
		assert.Nil(t, statuses)
	})
}
