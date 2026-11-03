package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/stretchr/testify/require"
)

func TestSeriesSelectionContractHTTP(t *testing.T) {
	app := buildSeriesApp(t)
	ctx := context.Background()
	seasons, err := app.store.ListSeasons(ctx, storage.SeasonFilter{SeriesID: &app.seriesID})
	require.NoError(t, err)
	require.Len(t, seasons, 1)
	for _, tc := range []struct {
		body   string
		status int
	}{
		{`{"monitored":null}`, 400}, {`{"seasons":null}`, 400}, {`{"qualityProfileID":null}`, 400},
		{`{"qualityProfileID":1}`, 400}, {`{"qualityProfileID":99999}`, 400},
		{`{"seasons":[{"id":1,"qualityProfileID":0}]}`, 400},
		{`{"seasons":[{"id":1},{"id":1}]}`, 400},
		{`{"episodes":[{"id":99999,"monitored":false}],"monitored":false}`, 404},
	} {
		r := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/library/tv/%d/selection", app.seriesID), strings.NewReader(tc.body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		app.router.ServeHTTP(w, r)
		require.Equal(t, tc.status, w.Code, w.Body.String())
		row, err := app.store.GetSeriesByID(ctx, app.seriesID)
		require.NoError(t, err)
		require.Equal(t, int64(1), row.Series.Monitored, "failed patch must roll back")
	}
	app.capture("series_selection_patch", "PATCH", fmt.Sprintf("/api/v1/library/tv/%d/selection", app.seriesID), fmt.Sprintf(`{"monitored":false,"monitorNewSeasons":false,"qualityProfileID":4,"seasons":[{"id":%d,"qualityProfileID":5}],"episodes":[{"id":%d,"qualityProfileID":null}]}`, seasons[0].Season.ID, app.episodeID), http.StatusOK, secretMustBeAbsent)
}
