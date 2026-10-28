package server

import (
	"fmt"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/kasuboski/mediaz/pkg/pagination"
	"github.com/stretchr/testify/require"
)

func TestParseActiveActivityPaginationParams(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	overflowPage := strconv.FormatInt(int64(maxInt/50+2), 10)
	tests := []struct {
		name  string
		query string
		want  [3]pagination.Params
		bad   bool
	}{
		{name: "defaults", want: [3]pagination.Params{{Page: 1, PageSize: 50}, {Page: 1, PageSize: 50}, {Page: 1, PageSize: 50}}},
		{name: "shared fallback", query: "page=3&pageSize=7", want: [3]pagination.Params{{Page: 3, PageSize: 7}, {Page: 3, PageSize: 7}, {Page: 3, PageSize: 7}}},
		{name: "independent named pages", query: "page=4&pageSize=8&moviesPage=2&seriesPage=3&jobsPage=1", want: [3]pagination.Params{{Page: 2, PageSize: 8}, {Page: 3, PageSize: 8}, {Page: 1, PageSize: 8}}},
	}
	for _, key := range []string{"page", "moviesPage", "seriesPage", "jobsPage"} {
		for _, value := range []string{"0", "-1", "1&" + key + "=2", "999999999999999999999999", overflowPage} {
			query := fmt.Sprintf("%s=%s", key, value)
			// Duplicate values are expressed as distinct query parameters.
			if value == "1&"+key+"=2" {
				query = key + "=1&" + key + "=2"
			}
			tests = append(tests, struct {
				name  string
				query string
				want  [3]pagination.Params
				bad   bool
			}{name: key + " invalid " + value, query: query, bad: true})
		}
	}
	for _, key := range []string{"pageSize"} {
		for _, value := range []string{"0", "-1", "101", "999999999999999999999999", "1&" + key + "=2"} {
			query := key + "=" + value
			if value == "1&"+key+"=2" {
				query = key + "=1&" + key + "=2"
			}
			tests = append(tests, struct {
				name  string
				query string
				want  [3]pagination.Params
				bad   bool
			}{name: key + " invalid " + value, query: query, bad: true})
		}
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/api/v1/activity/active?"+tt.query, nil)
			got, err := parseActivePaginationParams(r)
			if tt.bad {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want[0], got.Movies)
			require.Equal(t, tt.want[1], got.Series)
			require.Equal(t, tt.want[2], got.Jobs)
		})
	}
}
