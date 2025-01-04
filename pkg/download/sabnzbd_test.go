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

	var historyResponse HistoryResponse
	err = json.Unmarshal([]byte(testHistoyResponse), &historyResponse)
	require.NoError(t, err)

	statuses, err := queueToStatus(response.Queue, historyResponse.History)
	assert.NoError(t, err)

	assert.Len(t, statuses, 2)
	firstStatus := statuses[0]
	assert.Equal(t, "SABnzbd_nzo_p86tgx", firstStatus.ID)
	assert.Equal(t, "TV.Show.S04E11.720p.HDTV.x264", firstStatus.Name)
	assert.Equal(t, 2.5, firstStatus.Progress)
	assert.Equal(t, int64(1), firstStatus.Speed)
	assert.Equal(t, int64(1277), firstStatus.Size)
	assert.Equal(t, []string{"/path/to/TV.Show.S04E02.720p.BluRay.x264-xHD"}, firstStatus.FilePath)

	secondStatus := statuses[1]
	assert.Equal(t, "SABnzbd_nzo_ksfai6", secondStatus.ID)
	assert.Equal(t, "TV.Show.S04E12.720p.HDTV.x264", secondStatus.Name)
	assert.Equal(t, 50.0, secondStatus.Progress)
	assert.Equal(t, int64(1), secondStatus.Speed)
	assert.Equal(t, int64(1277), secondStatus.Size)
	assert.Equal(t, []string{"/path2/to/TV.Show.S04E02.720p.BluRay.x264-xHD"}, secondStatus.FilePath)
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
			NzoIDs: []string{"SABnzbd_nzo_ksfai6"},
		}

		addResponseBody, err := json.Marshal(addResponse)
		require.NoError(t, err)

		getResponseBody, err := json.Marshal(getResponse)
		require.NoError(t, err)

		addMock := mockHttp.EXPECT().Do(gomock.Any()).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBuffer(addResponseBody)),
		}, nil)

		getMock := mockHttp.EXPECT().Do(gomock.Any()).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBuffer(getResponseBody)),
		}, nil)

		historyResponse := HistoryResponse{
			History: History{
				Slots: []HistorySlot{
					{
						NzoID:   "SABnzbd_nzo_ksfai6",
						Storage: "/downloads/TV.Show.S04E12.720p.HDTV.x264",
					},
				},
			},
		}

		historyResponseBody, err := json.Marshal(historyResponse)
		require.NoError(t, err)

		historyMock := mockHttp.EXPECT().Do(gomock.Any()).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBuffer(historyResponseBody)),
		}, nil)

		gomock.InOrder(addMock, getMock, historyMock)

		status, err := client.Add(ctx, addRequest)
		assert.NoError(t, err)

		expectedStatus := Status{
			ID:       "SABnzbd_nzo_ksfai6",
			Name:     "TV.Show.S04E12.720p.HDTV.x264",
			Progress: 40,
			Speed:    1,
			Size:     1277,
			FilePath: []string{"/downloads/TV.Show.S04E12.720p.HDTV.x264"},
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
			NzoIDs: []string{"SABnzbd_nzo_ksfai6"},
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

		queueResponse := QueueResponse{
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

		queueResponseBody, err := json.Marshal(queueResponse)
		require.NoError(t, err)

		queueMock := mockHttp.EXPECT().Do(gomock.Any()).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBuffer(queueResponseBody)),
		}, nil)

		historyResponse := HistoryResponse{
			History: History{
				Slots: []HistorySlot{
					{
						NzoID:   "SABnzbd_nzo_ksfai6",
						Storage: "/downloads/TV.Show.S04E12.720p.HDTV.x264",
					},
				},
			},
		}

		historyResponseBody, err := json.Marshal(historyResponse)
		require.NoError(t, err)

		historyMock := mockHttp.EXPECT().Do(gomock.Any()).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBuffer(historyResponseBody)),
		}, nil)

		gomock.InOrder(queueMock, historyMock)

		status, err := client.Get(ctx, getRequest)
		assert.NoError(t, err)

		expectedStatus := Status{
			ID:       "SABnzbd_nzo_ksfai6",
			Name:     "TV.Show.S04E12.720p.HDTV.x264",
			Progress: 40,
			Speed:    1,
			Size:     1277,
			FilePath: []string{"/downloads/TV.Show.S04E12.720p.HDTV.x264"},
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

		queueResponse := QueueResponse{
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

		queueResponseBody, err := json.Marshal(queueResponse)
		require.NoError(t, err)

		queueMock := mockHttp.EXPECT().Do(gomock.Any()).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBuffer(queueResponseBody)),
		}, nil)

		historyResponse := HistoryResponse{
			History: History{
				Slots: []HistorySlot{
					{
						NzoID:   "SABnzbd_nzo_ksfai6",
						Storage: "/downloads/TV.Show.S04E12.720p.HDTV.x264",
					},
				},
			},
		}

		historyResponseBody, err := json.Marshal(historyResponse)
		require.NoError(t, err)

		historyMock := mockHttp.EXPECT().Do(gomock.Any()).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBuffer(historyResponseBody)),
		}, nil)

		gomock.InOrder(queueMock, historyMock)

		getRequest := GetRequest{
			ID: "1",
		}

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

		queueResponse := QueueResponse{
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

		queueResponseBody, err := json.Marshal(queueResponse)
		require.NoError(t, err)

		queueMock := mockHttp.EXPECT().Do(gomock.Any()).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBuffer(queueResponseBody)),
		}, nil)

		historyResponse := HistoryResponse{
			History: History{
				Slots: []HistorySlot{
					{
						NzoID:   "SABnzbd_nzo_ksfai6",
						Storage: "/downloads/TV.Show.S04E12.720p.HDTV.x264",
					},
					{
						NzoID:   "SABnzbd_nzo_ksfai7",
						Storage: "/downloads/TV.Show.S04E13.720p.HDTV.x264",
					},
					{
						NzoID:   "SABnzbd_nzo_ksfai8",
						Storage: "/downloads/TV.Show.S04E10.720p.HDTV.x264",
					},
				},
			},
		}

		historyResponseBody, err := json.Marshal(historyResponse)
		require.NoError(t, err)

		historyMock := mockHttp.EXPECT().Do(gomock.Any()).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBuffer(historyResponseBody)),
		}, nil)

		gomock.InOrder(queueMock, historyMock)

		mockHttp.EXPECT()

		status, err := client.List(ctx)
		assert.NoError(t, err)

		want := []Status{
			{
				ID:       "SABnzbd_nzo_ksfai6",
				Name:     "TV.Show.S04E12.720p.HDTV.x264",
				Progress: 40,
				Speed:    1,
				Size:     1277,
				FilePath: []string{"/downloads/TV.Show.S04E12.720p.HDTV.x264"},
			},
			{
				ID:       "SABnzbd_nzo_ksfai7",
				Name:     "TV.Show.S04E13.720p.HDTV.x264",
				Progress: 2,
				Speed:    1,
				Size:     12237,
				FilePath: []string{"/downloads/TV.Show.S04E13.720p.HDTV.x264"},
			},
			{
				ID:       "SABnzbd_nzo_ksfai8",
				Name:     "TV.Show.S04E10.720p.HDTV.x264",
				Progress: 22.5,
				Speed:    1,
				Size:     127,
				FilePath: []string{"/downloads/TV.Show.S04E10.720p.HDTV.x264"},
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

		gomock.InOrder(
			mockHttp.EXPECT().Do(gomock.Any()).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBuffer(getResponseBody)),
			}, nil),
			mockHttp.EXPECT().Do(gomock.Any()).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBuffer(getResponseBody)),
			}, nil),
		)

		statuses, err := client.List(ctx)
		assert.NoError(t, err)
		assert.Empty(t, statuses)
	})
}

func TestSabnzbdClient_History(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t.Run("success with ids", func(t *testing.T) {
		mockHttp := httpMock.NewMockHTTPClient(ctrl)
		client := SabnzbdClient{
			http:   mockHttp,
			host:   "localhost",
			scheme: "http",
			apiKey: "secret",
		}
		ctx := context.Background()

		historyResponse := HistoryResponse{
			History: History{
				NoOfSlots: 1,
				Slots: []HistorySlot{
					{
						NzoID:   "SABnzbd_nzo_ksfai6",
						Name:    "TV.Show.S04E12.720p.HDTV.x264",
						Status:  "Completed",
						Storage: "/downloads/TV.Show.S04E12.720p.HDTV.x264",
					},
				},
			},
		}

		historyResponseBody, err := json.Marshal(historyResponse)
		require.NoError(t, err)

		mockHttp.EXPECT().Do(gomock.Any()).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBuffer(historyResponseBody)),
		}, nil)

		history, err := client.history(ctx, "SABnzbd_nzo_ksfai6")
		assert.NoError(t, err)
		assert.Equal(t, historyResponse, history)
	})

	t.Run("success without ids", func(t *testing.T) {
		mockHttp := httpMock.NewMockHTTPClient(ctrl)
		client := SabnzbdClient{
			http:   mockHttp,
			host:   "localhost",
			scheme: "http",
			apiKey: "secret",
		}
		ctx := context.Background()

		historyResponse := HistoryResponse{
			History: History{
				NoOfSlots: 1,
				Slots: []HistorySlot{
					{
						NzoID:   "SABnzbd_nzo_ksfai6",
						Name:    "TV.Show.S04E12.720p.HDTV.x264",
						Status:  "Completed",
						Storage: "/downloads/TV.Show.S04E12.720p.HDTV.x264",
					},
				},
			},
		}

		historyResponseBody, err := json.Marshal(historyResponse)
		require.NoError(t, err)

		mockHttp.EXPECT().Do(gomock.Any()).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBuffer(historyResponseBody)),
		}, nil)

		history, err := client.history(ctx)
		assert.NoError(t, err)
		assert.Equal(t, historyResponse, history)
	})

	t.Run("error during request", func(t *testing.T) {
		mockHttp := httpMock.NewMockHTTPClient(ctrl)
		client := SabnzbdClient{
			http:   mockHttp,
			host:   "localhost",
			scheme: "http",
			apiKey: "secret",
		}
		ctx := context.Background()

		mockHttp.EXPECT().Do(gomock.Any()).Return(nil, fmt.Errorf("http error"))

		history, err := client.history(ctx, "SABnzbd_nzo_ksfai6")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "http error")
		assert.Equal(t, HistoryResponse{}, history)
	})

	t.Run("error in response", func(t *testing.T) {
		mockHttp := httpMock.NewMockHTTPClient(ctrl)
		client := SabnzbdClient{
			http:   mockHttp,
			host:   "localhost",
			scheme: "http",
			apiKey: "secret",
		}

		ctx := context.Background()

		mockHttp.EXPECT().Do(gomock.Any()).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBuffer([]byte("invalid json"))),
		}, nil)

		history, err := client.history(ctx, "SABnzbd_nzo_ksfai6")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid character")
		assert.Equal(t, HistoryResponse{}, history)
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

var testHistoyResponse = `{
    "history": {
        "noofslots": 220,
        "ppslots": 1,
        "day_size": "1.9 G",
        "week_size": "30.4 G",
        "month_size": "167.3 G",
        "total_size": "678.1 G",
        "last_history_update": 1469210913,
        "slots": [
            {
                "action_line": "",
                "duplicate_key": "TV.Show/4/2",
                "meta": null,
                "fail_message": "",
                "loaded": false,
                "size": "2.3 GB",
                "category": "tv",
                "pp": "D",
                "retry": 0,
                "script": "None",
                "nzb_name": "TV.Show.S04E02.720p.BluRay.x264-xHD.nzb",
                "download_time": 64,
                "storage": "/path/to/TV.Show.S04E02.720p.BluRay.x264-xHD",
                "has_rating": false,
                "status": "Completed",
                "script_line": "",
                "completed": 1469172988,
                "nzo_id": "SABnzbd_nzo_p86tgx",
                "downloaded": 2436906376,
                "report": "",
                "password": "",
                "path": "C:\\Users\\xxx\\Videos\\Complete\\TV.Show.S04E02.720p.BluRay.x264-xHD",
                "postproc_time": 40,
                "name": "TV.Show.S04E02.720p.BluRay.x264-xHD",
                "url": "TV.Show.S04E02.720p.BluRay.x264-xHD.nzb",
                "md5sum": "d2c16aeecbc1b1921d04422850e93013",
                "archive": false,
                "bytes": 2436906376,
                "url_info": "",
                "stage_log": [
                    {
                        "name": "Source",
                        "actions": [
                            "TV.Show.S04E02.720p.BluRay.x264-xHD.nzb"
                        ]
                    },
                    {
                        "name": "Download",
                        "actions": [
                            "Downloaded in 1 min 4 seconds at an average of 36.2 MB/s<br/>Age: 550d<br/>10 articles were malformed"
                        ]
                    },
                    {
                        "name": "Servers",
                        "actions": [
                            "Frugal=2.3 GB"
                        ]
                    },
                    {
                        "name": "Repair",
                        "actions": [
                            "[pA72r5Ac6lW3bmpd20T7Hj1Zg2bymUsINBB50skrI] Repaired in 19 seconds"
                        ]
                    },
                    {
                        "name": "Unpack",
                        "actions": [
                            "[pA72r5Ac6lW3bmpd20T7Hj1Zg2bymUsINBB50skrI] Unpacked 1 files/folders in 6 seconds"
                        ]
                    }
                ]
            },
			            {
                "action_line": "",
                "duplicate_key": "TV.Show/4/2",
                "meta": null,
                "fail_message": "",
                "loaded": false,
                "size": "2.3 GB",
                "category": "tv",
                "pp": "D",
                "retry": 0,
                "script": "None",
                "nzb_name": "TV.Show.S04E02.720p.BluRay.x264-xHD.nzb",
                "download_time": 64,
                "storage": "/path2/to/TV.Show.S04E02.720p.BluRay.x264-xHD",
                "has_rating": false,
                "status": "Completed",
                "script_line": "",
                "completed": 1469172988,
                "nzo_id": "SABnzbd_nzo_ksfai6",
                "downloaded": 2436906376,
                "report": "",
                "password": "",
                "path": "/path2/to/TV.Show.S04E02.720p.BluRay.x264-xHD",
                "postproc_time": 40,
                "name": "TV.Show.S04E02.720p.BluRay.x264-xHD",
                "url": "TV.Show.S04E02.720p.BluRay.x264-xHD.nzb",
                "md5sum": "d2c16aeecbc1b1921d04422850e93013",
                "archive": false,
                "bytes": 2436906376,
                "url_info": "",
                "stage_log": [
                    {
                        "name": "Source",
                        "actions": [
                            "TV.Show.S04E02.720p.BluRay.x264-xHD.nzb"
                        ]
                    },
                    {
                        "name": "Download",
                        "actions": [
                            "Downloaded in 1 min 4 seconds at an average of 36.2 MB/s<br/>Age: 550d<br/>10 articles were malformed"
                        ]
                    },
                    {
                        "name": "Servers",
                        "actions": [
                            "Frugal=2.3 GB"
                        ]
                    },
                    {
                        "name": "Repair",
                        "actions": [
                            "[pA72r5Ac6lW3bmpd20T7Hj1Zg2bymUsINBB50skrI] Repaired in 19 seconds"
                        ]
                    },
                    {
                        "name": "Unpack",
                        "actions": [
                            "[pA72r5Ac6lW3bmpd20T7Hj1Zg2bymUsINBB50skrI] Unpacked 1 files/folders in 6 seconds"
                        ]
                    }
                ]
            }
		]
	}
}`
