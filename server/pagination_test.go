package server

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/kasuboski/mediaz/pkg/pagination"
	"github.com/stretchr/testify/require"
)

func TestLibraryPaginationHTTP(t *testing.T) {
	app := buildMoviesApp(t)
	tests := []struct {
		name, path string
		status     int
		contains   []string
		omits      []string
	}{
		{name: "filtered count", path: "/api/v1/library/movies?state=downloaded", status: http.StatusOK, contains: []string{`"totalItems":1`, `"totalPages":1`, `"byState":{"discovered":1,"downloaded":1}`, `"title":"Golden Movie A"`}, omits: []string{`"title":"Minimal Movie B"`}},
		{name: "empty out of range", path: "/api/v1/library/movies?page=3&pageSize=1", status: http.StatusOK, contains: []string{`"items":[]`, `"page":3`, `"totalItems":2`}},
		{name: "bad pagination", path: "/api/v1/library/movies?pageSize=0", status: http.StatusBadRequest},
		{name: "invalid filter", path: "/api/v1/library/movies?state=bogus", status: http.StatusBadRequest},
		{name: "deduplicated filter", path: "/api/v1/library/movies?state=downloaded&state=downloaded", status: http.StatusOK, contains: []string{`"totalItems":1`, `"byState":{"discovered":1,"downloaded":1}`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()
			app.router.ServeHTTP(w, r)
			require.Equal(t, tt.status, w.Code, w.Body.String())
			for _, value := range tt.contains {
				require.Contains(t, w.Body.String(), value)
			}
			for _, value := range tt.omits {
				require.NotContains(t, w.Body.String(), value)
			}
		})
	}
}

func TestParsePaginationParams(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	overflowPage := strconv.FormatInt(int64(maxInt/50+2), 10)
	tests := []struct {
		name    string
		query   string
		want    pagination.Params
		wantErr bool
	}{
		{name: "defaults", want: pagination.Params{Page: 1, PageSize: 50}},
		{name: "page only", query: "page=3", want: pagination.Params{Page: 3, PageSize: 50}},
		{name: "valid", query: "page=2&pageSize=25", want: pagination.Params{Page: 2, PageSize: 25}},
		{name: "empty page", query: "page=", wantErr: true},
		{name: "malformed page", query: "page=x", wantErr: true},
		{name: "negative page", query: "page=-1", wantErr: true},
		{name: "zero page", query: "page=0", wantErr: true},
		{name: "empty size", query: "pageSize=", wantErr: true},
		{name: "malformed size", query: "pageSize=x", wantErr: true},
		{name: "negative size", query: "pageSize=-1", wantErr: true},
		{name: "zero size", query: "pageSize=0", wantErr: true},
		{name: "oversize", query: "pageSize=101", wantErr: true},
		{name: "duplicate page", query: "page=1&page=2", wantErr: true},
		{name: "duplicate size", query: "pageSize=10&pageSize=20", wantErr: true},
		{name: "integer overflow", query: "page=999999999999999999999999", wantErr: true},
		{name: "offset overflow", query: "page=" + overflowPage + "&pageSize=50", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/?"+tt.query, nil)
			got, err := ParsePaginationParams(r)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
