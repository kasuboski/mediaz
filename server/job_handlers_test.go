package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/manager"
	"github.com/kasuboski/mediaz/pkg/storage"
	storeMocks "github.com/kasuboski/mediaz/pkg/storage/mocks"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	tmdbMocks "github.com/kasuboski/mediaz/pkg/tmdb/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func TestServer_ListJobs(t *testing.T) {
	t.Run("success - list all jobs", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		createdAt := time.Now().Add(-1 * time.Hour)
		updatedAt := time.Now()
		errorMsg := "test error"
		jobs := []*storage.Job{
			{
				Job: model.Job{
					ID:        1,
					Type:      "MovieIndex",
					CreatedAt: &createdAt,
				},
				State:     storage.JobStateDone,
				UpdatedAt: &updatedAt,
				Error:     nil,
			},
			{
				Job: model.Job{
					ID:        2,
					Type:      "SeriesReconcile",
					CreatedAt: &createdAt,
				},
				State:     storage.JobStateError,
				UpdatedAt: &updatedAt,
				Error:     &errorMsg,
			},
		}

		store.EXPECT().CountJobs(gomock.Any(), gomock.Any()).Return(len(jobs), nil)
		store.EXPECT().ListJobs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(jobs, nil)

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := Server{
			baseLogger: zap.NewNop().Sugar(),
			manager:    mgr,
		}

		req, err := http.NewRequest("GET", "/jobs", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.ListJobs()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("content-type"))

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		jobList, ok := response.Response.(map[string]any)
		require.True(t, ok, "Response should be a map")

		jobsArray, ok := jobList["jobs"].([]any)
		require.True(t, ok, "jobs should be an array")
		assert.Len(t, jobsArray, 2)

		assert.Equal(t, float64(2), jobList["count"])

		job1 := jobsArray[0].(map[string]any)
		assert.Equal(t, float64(1), job1["id"])
		assert.Equal(t, "MovieIndex", job1["type"])
		assert.Equal(t, "done", job1["state"])

		job2 := jobsArray[1].(map[string]any)
		assert.Equal(t, float64(2), job2["id"])
		assert.Equal(t, "SeriesReconcile", job2["type"])
		assert.Equal(t, "error", job2["state"])
		assert.Equal(t, "test error", job2["error"])
	})

	t.Run("success - empty job list", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		store.EXPECT().CountJobs(gomock.Any(), gomock.Any()).Return(0, nil)
		store.EXPECT().ListJobs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*storage.Job{}, nil)

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := Server{
			baseLogger: zap.NewNop().Sugar(),
			manager:    mgr,
		}

		req, err := http.NewRequest("GET", "/jobs", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.ListJobs()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		jobList, ok := response.Response.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, float64(0), jobList["count"])
	})

	t.Run("error - storage error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		store.EXPECT().CountJobs(gomock.Any(), gomock.Any()).Return(0, errors.New("storage error"))

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := Server{
			baseLogger: zap.NewNop().Sugar(),
			manager:    mgr,
		}

		req, err := http.NewRequest("GET", "/jobs", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.ListJobs()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("content-type"))
	})
}

func TestServer_GetJob(t *testing.T) {
	t.Run("success - get job by id", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		createdAt := time.Now().Add(-1 * time.Hour)
		updatedAt := time.Now()
		job := &storage.Job{
			Job: model.Job{
				ID:        42,
				Type:      "MovieIndex",
				CreatedAt: &createdAt,
			},
			State:     storage.JobStateRunning,
			UpdatedAt: &updatedAt,
			Error:     nil,
		}

		store.EXPECT().GetJob(gomock.Any(), int64(42)).Return(job, nil)

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := Server{
			baseLogger: zap.NewNop().Sugar(),
			manager:    mgr,
		}

		req, err := http.NewRequest("GET", "/jobs/42", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/jobs/{id}", s.GetJob()).Methods("GET")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("content-type"))

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		jobResp, ok := response.Response.(map[string]any)
		require.True(t, ok, "Response should be a map")

		assert.Equal(t, float64(42), jobResp["id"])
		assert.Equal(t, "MovieIndex", jobResp["type"])
		assert.Equal(t, "running", jobResp["state"])
		assert.Nil(t, jobResp["error"])
	})

	t.Run("invalid job id format", func(t *testing.T) {
		s := Server{baseLogger: zap.NewNop().Sugar()}

		req, err := http.NewRequest("GET", "/jobs/invalid", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/jobs/{id}", s.GetJob()).Methods("GET")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid id")
	})

	t.Run("error - job not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		store.EXPECT().GetJob(gomock.Any(), int64(999)).Return(nil, storage.ErrNotFound)

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := Server{
			baseLogger: zap.NewNop().Sugar(),
			manager:    mgr,
		}

		req, err := http.NewRequest("GET", "/jobs/999", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/jobs/{id}", s.GetJob()).Methods("GET")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestServer_CancelJob(t *testing.T) {
	t.Run("success - cancel running job", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		createdAt := time.Now().Add(-1 * time.Hour)
		updatedAt := time.Now()

		store.EXPECT().GetJob(gomock.Any(), int64(42)).Return(&storage.Job{
			Job: model.Job{
				ID:        42,
				Type:      "MovieIndex",
				CreatedAt: &createdAt,
			},
			State:     storage.JobStateRunning,
			UpdatedAt: &updatedAt,
		}, nil)

		store.EXPECT().GetJob(gomock.Any(), int64(42)).Return(&storage.Job{
			Job: model.Job{
				ID:        42,
				Type:      "MovieIndex",
				CreatedAt: &createdAt,
			},
			State:     storage.JobStateCancelled,
			UpdatedAt: &updatedAt,
		}, nil)

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := Server{
			baseLogger: zap.NewNop().Sugar(),
			manager:    mgr,
		}

		req, err := http.NewRequest("POST", "/jobs/42/cancel", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/jobs/{id}/cancel", s.CancelJob()).Methods("POST")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("content-type"))

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		jobResp, ok := response.Response.(map[string]any)
		require.True(t, ok, "Response should be a map")

		assert.Equal(t, float64(42), jobResp["id"])
		assert.Equal(t, "cancelled", jobResp["state"])
	})

	t.Run("invalid job id format", func(t *testing.T) {
		s := Server{baseLogger: zap.NewNop().Sugar()}

		req, err := http.NewRequest("POST", "/jobs/invalid/cancel", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/jobs/{id}/cancel", s.CancelJob()).Methods("POST")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid id")
	})

	t.Run("error - job not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		store.EXPECT().GetJob(gomock.Any(), int64(999)).Return(nil, storage.ErrNotFound)

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := Server{
			baseLogger: zap.NewNop().Sugar(),
			manager:    mgr,
		}

		req, err := http.NewRequest("POST", "/jobs/999/cancel", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		router := mux.NewRouter()
		router.HandleFunc("/jobs/{id}/cancel", s.CancelJob()).Methods("POST")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestServer_CreateJob(t *testing.T) {
	t.Run("success - create new job", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		createdAt := time.Now()
		updatedAt := time.Now()

		store.EXPECT().CreateJob(gomock.Any(), gomock.Any(), storage.JobStatePending).Return(int64(1), nil)
		store.EXPECT().GetJob(gomock.Any(), int64(1)).Return(&storage.Job{
			Job: model.Job{
				ID:        1,
				Type:      "MovieIndex",
				CreatedAt: &createdAt,
			},
			State:     storage.JobStatePending,
			UpdatedAt: &updatedAt,
		}, nil)

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := Server{
			baseLogger: zap.NewNop().Sugar(),
			manager:    mgr,
		}

		requestBody := `{"type":"MovieIndex"}`
		req, err := http.NewRequest("POST", "/jobs", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.CreateJob()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
		assert.Equal(t, "application/json", rr.Header().Get("content-type"))

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		jobResp, ok := response.Response.(map[string]any)
		require.True(t, ok, "Response should be a map")

		assert.Equal(t, float64(1), jobResp["id"])
		assert.Equal(t, "MovieIndex", jobResp["type"])
		assert.Equal(t, "pending", jobResp["state"])
	})

	t.Run("success - job already pending", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		createdAt := time.Now()
		updatedAt := time.Now()

		existingJob := &storage.Job{
			Job: model.Job{
				ID:        5,
				Type:      "MovieIndex",
				CreatedAt: &createdAt,
			},
			State:     storage.JobStatePending,
			UpdatedAt: &updatedAt,
		}

		store.EXPECT().CreateJob(gomock.Any(), gomock.Any(), storage.JobStatePending).Return(int64(0), storage.ErrJobAlreadyPending)
		store.EXPECT().ListJobs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*storage.Job{existingJob}, nil)

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := Server{
			baseLogger: zap.NewNop().Sugar(),
			manager:    mgr,
		}

		requestBody := `{"type":"MovieIndex"}`
		req, err := http.NewRequest("POST", "/jobs", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.CreateJob()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)

		var response GenericResponse
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)

		jobResp, ok := response.Response.(map[string]any)
		require.True(t, ok, "Response should be a map")

		assert.Equal(t, float64(5), jobResp["id"])
		assert.Equal(t, "pending", jobResp["state"])
	})

	t.Run("invalid request body", func(t *testing.T) {
		s := Server{baseLogger: zap.NewNop().Sugar()}

		requestBody := `{"invalid json`
		req, err := http.NewRequest("POST", "/jobs", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.CreateJob()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid request body")
	})

	t.Run("error - storage error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)

		store.EXPECT().CreateJob(gomock.Any(), gomock.Any(), storage.JobStatePending).Return(int64(0), errors.New("storage error"))

		mgr := manager.New(tmdbMock, nil, nil, store, nil, config.Manager{}, config.Config{})

		s := Server{
			baseLogger: zap.NewNop().Sugar(),
			manager:    mgr,
		}

		requestBody := `{"type":"MovieIndex"}`
		req, err := http.NewRequest("POST", "/jobs", strings.NewReader(requestBody))
		require.NoError(t, err)

		rr := httptest.NewRecorder()

		handler := s.CreateJob()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}
