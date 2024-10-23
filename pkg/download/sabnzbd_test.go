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
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestQueueToStatus(t *testing.T) {
	var response QueueResponse
	err := json.Unmarshal([]byte(testQueueResponse), &response)
	require.NoError(t, err)

	statuses, err := queueToStatus(response.Queue)
	assert.NoError(t, err)

	assert.Len(t, statuses, 2)
	for _, s := range statuses {
		assert.NotEmpty(t, s.ID)
		assert.NotEmpty(t, s.Name)
		assert.NotEmpty(t, s.Progress)
		assert.NotEmpty(t, s.Size)
		assert.NotEmpty(t, s.Speed)
	}
}

func TestNewSabnzbdClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockHttp := httpMock.NewMockHTTPClient(ctrl)

	client := NewSabnzbdClient(mockHttp, "http", "localhost", "secret")
	sabnzbdClient, ok := client.(*SabnzbdClient)
	assert.True(t, ok, "client should be of type *sabnzbdClient")
	assert.Equal(t, "localhost", sabnzbdClient.host, "Host should not include port")
	assert.Equal(t, mockHttp, sabnzbdClient.http, "HTTP client should match")
	assert.Equal(t, "http", sabnzbdClient.scheme, "Scheme should match")
	assert.Equal(t, "secret", sabnzbdClient.apiKey, "ApiKey should match")
}

func TestSabnzbdClient_Add(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t.Run("success", func(t *testing.T) {
		mockHttp := httpMock.NewMockHTTPClient(ctrl)
		client := NewSabnzbdClient(mockHttp, "http", "localhost", "secret")
		ctx := context.Background()

		addRequest := AddRequest{
			Release: &prowlarr.ReleaseResource{DownloadURL: nullable.NewNullableWithValue[string]("http://example.com/group")},
		}

		getResponse := QueueResponse{
			Queue: Queue{
				Speed: "1.3 M",
				Slots: []Slot{{
					NzoID:      "SABnzbd_nzo_ksfai6",
					Filename:   "TV.Show.S04E12.720p.HDTV.x264",
					MB:         "1277.76",
					Percentage: "40",
				}},
			},
		}

		addResponse := AddNewsResponse{
			Status: true,
			NZOIDS: []string{"SABnzbd_nzo_ksfai6"},
		}

		addResponseBody, err := json.Marshal(addResponse)
		require.NoError(t, err)

		getResponseBody, err := json.Marshal(getResponse)
		require.NoError(t, err)

		first := mockHttp.EXPECT().Do(gomock.Any()).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBuffer(addResponseBody)),
		}, nil)

		second := mockHttp.EXPECT().Do(gomock.Any()).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBuffer(getResponseBody)),
		}, nil)

		gomock.InOrder(first, second)

		status, err := client.Add(ctx, addRequest)
		assert.NoError(t, err)

		expectedStatus := Status{
			ID:       "SABnzbd_nzo_ksfai6",
			Name:     "TV.Show.S04E12.720p.HDTV.x264",
			Progress: 40,
			Speed:    1,
			Size:     1277,
		}
		assert.Equal(t, expectedStatus, status)
	})

	t.Run("missing guid", func(t *testing.T) {
		mockHttp := httpMock.NewMockHTTPClient(ctrl)
		client := NewSabnzbdClient(mockHttp, "http", "localhost", "secret")
		ctx := context.Background()

		addRequest := AddRequest{
			Release: &prowlarr.ReleaseResource{DownloadURL: nullable.NewNullNullable[string]()},
		}

		status, err := client.Add(ctx, addRequest)

		assert.Error(t, err)
		assert.Equal(t, Status{}, status)
	})

	t.Run("error during request", func(t *testing.T) {
		mockHttp := httpMock.NewMockHTTPClient(ctrl)
		client := NewSabnzbdClient(mockHttp, "http", "localhost", "secret")
		ctx := context.Background()

		addRequest := AddRequest{
			Release: &prowlarr.ReleaseResource{DownloadURL: nullable.NewNullableWithValue[string]("http://example.com/torrent")},
		}

		mockHttp.EXPECT().Do(gomock.Any()).Return(nil, fmt.Errorf("http error"))

		status, err := client.Add(ctx, addRequest)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "http error")
		assert.Equal(t, Status{}, status)
	})

	t.Run("error in response", func(t *testing.T) {
		mockHttp := httpMock.NewMockHTTPClient(ctrl)
		client := NewSabnzbdClient(mockHttp, "http", "localhost", "secret")
		ctx := context.Background()

		addRequest := AddRequest{
			Release: &prowlarr.ReleaseResource{DownloadURL: nullable.NewNullableWithValue[string]("http://example.com/torrent")},
		}

		addResponse := AddNewsResponse{}

		addResponseBody, err := json.Marshal(addResponse)
		require.NoError(t, err)

		mockHttp.EXPECT().Do(gomock.Any()).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBuffer(addResponseBody)),
		}, nil)

		status, err := client.Add(ctx, addRequest)

		assert.Error(t, err)
		assert.Equal(t, Status{}, status)
	})

	t.Run("Error during Get", func(t *testing.T) {
		mockHttp := httpMock.NewMockHTTPClient(ctrl)
		client := NewSabnzbdClient(mockHttp, "http", "localhost", "secret")
		ctx := context.Background()

		addRequest := AddRequest{
			Release: &prowlarr.ReleaseResource{DownloadURL: nullable.NewNullableWithValue[string]("http://example.com/group")},
		}

		// Mock the successful Add response
		addResponse := AddNewsResponse{
			Status: true,
			NZOIDS: []string{"SABnzbd_nzo_ksfai6"},
		}

		// Serialize the Add response
		addResponseBody, err := json.Marshal(addResponse)
		require.NoError(t, err)

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

func TestSabnzbdClient_Get(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t.Run("success", func(t *testing.T) {
		mockHttp := httpMock.NewMockHTTPClient(ctrl)
		client := NewSabnzbdClient(mockHttp, "http", "localhost", "secret")
		ctx := context.Background()

		getRequest := GetRequest{
			ID: "SABnzbd_nzo_ksfai6",
		}

		getResponse := QueueResponse{
			Queue: Queue{
				Speed: "1.3 M",
				Slots: []Slot{{
					NzoID:      "SABnzbd_nzo_ksfai6",
					Filename:   "TV.Show.S04E12.720p.HDTV.x264",
					MB:         "1277.76",
					Percentage: "40",
				}},
			},
		}

		getResponseBody, err := json.Marshal(getResponse)
		require.NoError(t, err)

		mockHttp.EXPECT().Do(gomock.Any()).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBuffer(getResponseBody)),
		}, nil)

		status, err := client.Get(ctx, getRequest)
		assert.NoError(t, err)

		expectedStatus := Status{
			ID:       "SABnzbd_nzo_ksfai6",
			Name:     "TV.Show.S04E12.720p.HDTV.x264",
			Progress: 40,
			Speed:    1,
			Size:     1277,
		}
		assert.Equal(t, expectedStatus, status)
	})

	t.Run("error", func(t *testing.T) {
		mockHttp := httpMock.NewMockHTTPClient(ctrl)
		client := NewSabnzbdClient(mockHttp, "http", "localhost", "secret")
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

	t.Run("id not found", func(t *testing.T) {
		mockHttp := httpMock.NewMockHTTPClient(ctrl)
		client := NewSabnzbdClient(mockHttp, "http", "localhost", "secret")
		ctx := context.Background()

		getRequest := GetRequest{
			ID: "1",
		}

		getResponse := QueueResponse{
			Queue: Queue{
				Speed: "1.3 M",
				Slots: []Slot{{
					NzoID:      "SABnzbd_nzo_ksfai6",
					Filename:   "TV.Show.S04E12.720p.HDTV.x264",
					MB:         "1277.76",
					Percentage: "40",
				}},
			},
		}

		getResponseBody, err := json.Marshal(getResponse)
		require.NoError(t, err)

		mockHttp.EXPECT().Do(gomock.Any()).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBuffer(getResponseBody)),
		}, nil)

		status, err := client.Get(ctx, getRequest)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no download found")
		assert.Equal(t, Status{}, status)
	})
}

func TestSabnzbdClient_List(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t.Run("success", func(t *testing.T) {
		mockHttp := httpMock.NewMockHTTPClient(ctrl)
		client := NewSabnzbdClient(mockHttp, "http", "localhost", "secret")
		ctx := context.Background()

		getResponse := QueueResponse{
			Queue: Queue{
				Speed: "1.3 M",
				Slots: []Slot{
					{
						NzoID:      "SABnzbd_nzo_ksfai6",
						Filename:   "TV.Show.S04E12.720p.HDTV.x264",
						MB:         "1277.76",
						Percentage: "40",
					},
					{
						NzoID:      "SABnzbd_nzo_ksfai7",
						Filename:   "TV.Show.S04E13.720p.HDTV.x264",
						MB:         "12237.76",
						Percentage: "2.0",
					},
					{
						NzoID:      "SABnzbd_nzo_ksfai8",
						Filename:   "TV.Show.S04E10.720p.HDTV.x264",
						MB:         "127.76",
						Percentage: "22.5",
					},
				},
			},
		}

		getResponseBody, err := json.Marshal(getResponse)
		require.NoError(t, err)

		mockHttp.EXPECT().Do(gomock.Any()).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBuffer(getResponseBody)),
		}, nil)

		status, err := client.List(ctx)
		assert.NoError(t, err)

		want := []Status{
			{
				ID:       "SABnzbd_nzo_ksfai6",
				Name:     "TV.Show.S04E12.720p.HDTV.x264",
				Progress: 40,
				Speed:    1,
				Size:     1277,
			},
			{
				ID:       "SABnzbd_nzo_ksfai7",
				Name:     "TV.Show.S04E13.720p.HDTV.x264",
				Progress: 2,
				Speed:    1,
				Size:     12237,
			},
			{
				ID:       "SABnzbd_nzo_ksfai8",
				Name:     "TV.Show.S04E10.720p.HDTV.x264",
				Progress: 22.5,
				Speed:    1,
				Size:     127,
			},
		}
		assert.Equal(t, want, status)
	})

	t.Run("error during request", func(t *testing.T) {
		mockHttp := httpMock.NewMockHTTPClient(ctrl)
		client := NewSabnzbdClient(mockHttp, "http", "localhost", "secret")
		ctx := context.Background()

		mockHttp.EXPECT().Do(gomock.Any()).Return(nil, fmt.Errorf("http error"))

		status, err := client.List(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "http error")
		assert.Nil(t, status)
	})

	t.Run("empty queue", func(t *testing.T) {
		mockHttp := httpMock.NewMockHTTPClient(ctrl)
		client := NewSabnzbdClient(mockHttp, "http", "localhost", "secret")
		ctx := context.Background()

		getResponse := QueueResponse{
			Queue: Queue{
				Speed: "0",
				Slots: []Slot{},
			},
		}

		getResponseBody, err := json.Marshal(getResponse)
		require.NoError(t, err)

		mockHttp.EXPECT().Do(gomock.Any()).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBuffer(getResponseBody)),
		}, nil)

		statuses, err := client.List(ctx)
		assert.NoError(t, err)
		assert.Empty(t, statuses)
	})
}

const testQueueResponse = `{
    "queue": {
        "status": "Downloading",
        "speedlimit": "9",
        "speedlimit_abs": "4718592.0",
        "paused": false,
        "noofslots_total": 2,
        "noofslots": 2,
        "limit": 10,
        "start": 0,
        "timeleft": "0:16:44",
        "speed": "1.3 M",
        "kbpersec": "1296.02",
        "size": "1.2 GB",
        "sizeleft": "1.2 GB",
        "mb": "1277.65",
        "mbleft": "1271.58",
        "slots": [
            {
                "status": "Downloading",
                "index": 0,
                "password": "",
                "avg_age": "2895d",
                "script": "None",
                "direct_unpack": "10/30",
                "mb": "1277.65",
                "mbleft": "1271.59",
                "mbmissing": "0.0",
                "size": "1.2 GB",
                "sizeleft": "1.2 GB",
                "filename": "TV.Show.S04E11.720p.HDTV.x264",
                "labels": [],
                "priority": 1,
                "cat": "tv",
                "timeleft": "0:16:44",
                "percentage": "2.5",
                "nzo_id": "SABnzbd_nzo_p86tgx",
                "unpackopts": "3"
            },
            {
                "status": "Paused",
                "index": 1,
                "password": "",
                "avg_age": "2895d",
                "script": "None",
                "direct_unpack": null,
                "mb": "1277.76",
                "mbleft": "1277.76",
                "mbmissing": "0.0",
                "size": "1.2 GB",
                "sizeleft": "1.2 GB",
                "filename": "TV.Show.S04E12.720p.HDTV.x264",
                "labels": [
                    "TOO LARGE",
                    "DUPLICATE"
                ],
                "priority": 1,
                "cat": "tv",
                "timeleft": "0:00:00",
                "percentage": "50",
                "nzo_id": "SABnzbd_nzo_ksfai6",
                "unpackopts": "3"
            }
        ],
        "diskspace1": "161.16",
        "diskspace2": "161.16",
        "diskspacetotal1": "465.21",
        "diskspacetotal2": "465.21",
        "diskspace1_norm": "161.2 G",
        "diskspace2_norm": "161.2 G",
        "have_warnings": "0",
        "pause_int": "0",
        "left_quota": "0 ",
        "version": "3.x.x",
        "finish": 2,
        "cache_art": "16",
        "cache_size": "6 MB",
        "finishaction": null,
        "paused_all": false,
        "quota": "0 ",
        "have_quota": false
    }
}`
