package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/stretchr/testify/require"
)

func TestConfigPaginationHTTP(t *testing.T) {
	ctx := context.Background()
	app := newWireAppWithStore(t, newWireStore(t))

	// Exercise all five routes against the real SQLite-backed application. The
	// source and definition collections exceed the default page size.
	for i := 0; i < 61; i++ {
		_, err := app.store.CreateIndexerSource(ctx, sqlcdb.IndexerSource{Name: fmt.Sprintf("source-%02d", i), Implementation: "test", Host: fmt.Sprintf("host-%02d", i), Enabled: true})
		require.NoError(t, err)
		_, err = app.store.CreateQualityDefinition(ctx, sqlcdb.QualityDefinition{Name: fmt.Sprintf("definition-%02d", i), QualityID: int64(i + 1), MediaType: "movie", PreferredSize: 1, MinSize: 1, MaxSize: 2})
		require.NoError(t, err)
		mediaType := "movie"
		if i%2 == 0 {
			mediaType = "episode"
		}
		_, err = app.store.CreateQualityDefinition(ctx, sqlcdb.QualityDefinition{Name: fmt.Sprintf("mixed-definition-%02d", i), QualityID: int64(i + 100), MediaType: mediaType, PreferredSize: 1, MinSize: 1, MaxSize: 2})
		require.NoError(t, err)
	}
	profileID, err := app.store.CreateQualityProfile(ctx, sqlcdb.QualityProfile{Name: "profile-with-child", CutoffQualityID: sql.NullInt64{Int64: 1, Valid: true}, UpgradeAllowed: true})
	require.NoError(t, err)
	_, err = app.store.CreateQualityProfileItem(ctx, sqlcdb.QualityProfileItem{ProfileID: profileID, QualityID: 1})
	require.NoError(t, err)
	_, err = app.store.CreateDownloadClient(ctx, sqlcdb.DownloadClient{Type: "test", Implementation: "test", Scheme: "http", Host: "download-host", Port: 1234, ApiKey: sql.NullString{String: "must-not-leak", Valid: true}})
	require.NoError(t, err)
	_, err = app.store.CreateIndexer(ctx, sqlcdb.Indexer{Name: "sort-name", Uri: "http://x"})
	require.NoError(t, err)

	routes := []struct {
		path  string
		check func(*testing.T, string)
	}{
		{"/api/v1/indexers", func(t *testing.T, raw string) {
			assertPage(t, raw, 1, 1, 50)
			require.Contains(t, raw, `"name":"sort-name"`)
		}},
		{"/api/v1/indexer-sources", func(t *testing.T, raw string) { assertPage(t, raw, 1, 61, 50) }},
		{"/api/v1/download/clients", func(t *testing.T, raw string) {
			assertPage(t, raw, 1, 1, 50)
			require.Contains(t, raw, `"Host":"download-host"`)
			require.NotContains(t, raw, "must-not-leak")
		}},
		{"/api/v1/quality/definitions", func(t *testing.T, raw string) { assertPage(t, raw, 1, 150, 50) }},
		{"/api/v1/quality/profiles", func(t *testing.T, raw string) {
			assertPage(t, raw, 1, 7, 50)
			require.Contains(t, raw, `"qualities":[{"id":1`)
		}},
	}
	for _, route := range routes {
		t.Run(route.path, func(t *testing.T) {
			request := func(path string) (int, string) {
				r := httptest.NewRequest(http.MethodGet, path, nil)
				w := httptest.NewRecorder()
				app.router.ServeHTTP(w, r)
				return w.Code, w.Body.String()
			}
			status, body := request(route.path)
			require.Equal(t, http.StatusOK, status, body)
			route.check(t, body)
			if route.path == "/api/v1/indexer-sources" || route.path == "/api/v1/quality/definitions" {
				total := 61
				if route.path == "/api/v1/quality/definitions" {
					total = 150
				}
				status, body = request(route.path + "?page=2&pageSize=50")
				require.Equal(t, http.StatusOK, status, body)
				require.Contains(t, body, `"page":2`)
				require.Contains(t, body, fmt.Sprintf(`"totalItems":%d`, total))
				status, body = request(route.path + "?page=99&pageSize=10")
				require.Equal(t, http.StatusOK, status, body)
				require.Contains(t, body, `"items":[]`)
				require.Contains(t, body, fmt.Sprintf(`"totalItems":%d`, total))
			}
		})
	}

	// Definition type filters are applied before pagination; series is the
	// public alias for the stored episode media type.
	for _, tc := range []struct {
		query string
		total int
	}{
		{"?type=series&page=2&pageSize=10", 45},
		{"?type=movie&page=2&pageSize=10", 105},
	} {
		status, body := func() (int, string) {
			r := httptest.NewRequest(http.MethodGet, "/api/v1/quality/definitions"+tc.query, nil)
			w := httptest.NewRecorder()
			app.router.ServeHTTP(w, r)
			return w.Code, w.Body.String()
		}()
		require.Equal(t, http.StatusOK, status, body)
		assertPage(t, body, 2, tc.total, 10)
	}
	for _, query := range []string{"?type=bogus", "?type=movie&page=0"} {
		w := httptest.NewRecorder()
		app.router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/quality/definitions"+query, nil))
		require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	}

	// A quality-type filter is applied before the reported count and page.
	for i := 0; i < 3; i++ {
		_, err := app.store.CreateQualityProfile(ctx, sqlcdb.QualityProfile{Name: fmt.Sprintf("movie-profile-%d", i), CutoffQualityID: sql.NullInt64{Int64: 1, Valid: true}, UpgradeAllowed: true})
		require.NoError(t, err)
	}
	w := httptest.NewRecorder()
	app.router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/quality/profiles?type=movie&pageSize=2", nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assertPage(t, w.Body.String(), 1, 4, 2)

	// Parsing must reject bad sizes before any manager method is invoked.
	for _, path := range []string{"/api/v1/indexers", "/api/v1/indexer-sources", "/api/v1/download/clients", "/api/v1/quality/definitions", "/api/v1/quality/profiles"} {
		s := newTestServer()
		r := httptest.NewRequest(http.MethodGet, path+"?pageSize=0", nil)
		recorder := httptest.NewRecorder()
		router := mux.NewRouter()
		s.registerRoutes(router)
		router.ServeHTTP(recorder, r)
		require.Equal(t, http.StatusBadRequest, recorder.Code, path)
	}
}

func assertPage(t *testing.T, raw string, page, total, size int) {
	t.Helper()
	var envelope struct {
		Response struct {
			Items      []json.RawMessage `json:"items"`
			Pagination struct {
				Page       int `json:"page"`
				PageSize   int `json:"pageSize"`
				TotalItems int `json:"totalItems"`
				TotalPages int `json:"totalPages"`
			} `json:"pagination"`
		} `json:"response"`
	}
	require.NoError(t, json.Unmarshal([]byte(raw), &envelope), raw)
	require.Equal(t, page, envelope.Response.Pagination.Page)
	require.Equal(t, size, envelope.Response.Pagination.PageSize)
	require.Equal(t, total, envelope.Response.Pagination.TotalItems)
	require.Equal(t, (total+size-1)/size, envelope.Response.Pagination.TotalPages)
	wantItems := total - (page-1)*size
	if wantItems > size {
		wantItems = size
	}
	if wantItems < 0 {
		wantItems = 0
	}
	require.Len(t, envelope.Response.Items, wantItems)
}
