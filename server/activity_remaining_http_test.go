package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kasuboski/mediaz/pkg/manager"
	"github.com/stretchr/testify/require"
)

func activityHTTP[T any](t *testing.T, app *wireApp, path string, wantStatus int) T {
	t.Helper()
	rr := httptest.NewRecorder()
	app.router.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
	require.Equal(t, wantStatus, rr.Code, "%s: %s", path, rr.Body.String())
	var envelope struct {
		Response T `json:"response"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &envelope), rr.Body.String())
	return envelope.Response
}

func TestActivityActiveHTTPPagination(t *testing.T) {
	app := buildActivityApp(t)
	baseline := activityHTTP[manager.ActiveActivityResponse](t, app, "/api/v1/activity/active", http.StatusOK)
	require.Equal(t, manager.ActiveActivityCounts{Movies: 1, Series: 1, Jobs: 1, Total: 3}, baseline.Counts)
	require.Len(t, baseline.Movies, 1)
	require.Len(t, baseline.Series, 1)
	require.Len(t, baseline.Jobs, 1)

	paged := activityHTTP[manager.ActiveActivityResponse](t, app, "/api/v1/activity/active?pageSize=1&moviesPage=2&seriesPage=1&jobsPage=1", http.StatusOK)
	require.Equal(t, baseline.Counts, paged.Counts)
	require.Empty(t, paged.Movies)
	require.Equal(t, baseline.Series, paged.Series)
	require.Equal(t, baseline.Jobs, paged.Jobs)
	require.Len(t, paged.Movies, 0)
	require.LessOrEqual(t, len(paged.Movies), 1)
	require.LessOrEqual(t, len(paged.Series), 1)
	require.LessOrEqual(t, len(paged.Jobs), 1)
}

func TestActivityFailuresHTTPPagination(t *testing.T) {
	app := buildActivityApp(t)
	baseline := activityHTTP[manager.FailuresResponse](t, app, "/api/v1/activity/failures", http.StatusOK)
	paged := activityHTTP[manager.FailuresResponse](t, app, "/api/v1/activity/failures?pageSize=1&page=2", http.StatusOK)
	require.Equal(t, baseline.Pagination.TotalItems, paged.Pagination.TotalItems)
	require.Greater(t, baseline.Pagination.TotalItems, 0)
	require.Empty(t, paged.Failures)
	require.LessOrEqual(t, len(paged.Failures), 1)
}

func TestActivityHistoryHTTPPagination(t *testing.T) {
	app := buildActivityApp(t)
	entities := []struct {
		typeName string
		id       int64
	}{
		{"movie", app.movieID}, {"series", app.seriesID}, {"episode", app.episodeID}, {"job", app.jobID},
	}
	for _, entity := range entities {
		t.Run(entity.typeName, func(t *testing.T) {
			path := fmt.Sprintf("/api/v1/activity/history/%s/%d", entity.typeName, entity.id)
			baseline := activityHTTP[manager.HistoryResponse](t, app, path, http.StatusOK)
			require.NotNil(t, baseline.Entity)
			require.Equal(t, entity.typeName, baseline.Entity.Type)
			require.Equal(t, entity.id, baseline.Entity.ID)
			require.NotEmpty(t, baseline.History)
			paged := activityHTTP[manager.HistoryResponse](t, app, path+"?page=999&pageSize=1", http.StatusOK)
			require.Equal(t, baseline.Entity, paged.Entity)
			require.Empty(t, paged.History)
			require.Equal(t, baseline.Pagination.TotalItems, paged.Pagination.TotalItems)
			require.Greater(t, paged.Pagination.TotalItems, 0)
		})
	}
}

func TestActivityHistoryHTTPRejectsUnsupportedType(t *testing.T) {
	app := buildActivityApp(t)
	activityHTTP[json.RawMessage](t, app, "/api/v1/activity/history/unsupported/1", http.StatusBadRequest)
}
