package pagination

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCalculateOffsetLimit(t *testing.T) {
	tests := []struct {
		name          string
		params        Params
		offset, limit int
	}{
		{name: "first page", params: Params{Page: 1, PageSize: 50}, offset: 0, limit: 50},
		{name: "later page", params: Params{Page: 4, PageSize: 10}, offset: 30, limit: 10},
		{name: "invalid page", params: Params{Page: 0, PageSize: 10}},
		{name: "invalid size", params: Params{Page: 1, PageSize: 0}},
		{name: "largest safe offset", params: Params{Page: math.MaxInt/2 + 1, PageSize: 2}, offset: math.MaxInt - 1, limit: 2},
		{name: "overflow", params: Params{Page: math.MaxInt/2 + 2, PageSize: 2}},
		{name: "overflow at max page", params: Params{Page: math.MaxInt, PageSize: 2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			offset, limit := tt.params.CalculateOffsetLimit()
			require.Equal(t, tt.offset, offset)
			require.Equal(t, tt.limit, limit)
		})
	}
}

func TestPage(t *testing.T) {
	items := []string{"a", "b", "c", "d", "e"}
	tests := []struct {
		name   string
		params Params
		want   PageResult[string]
	}{
		{name: "middle page", params: Params{Page: 2, PageSize: 2}, want: PageResult[string]{Items: []string{"c", "d"}, Pagination: Meta{Page: 2, PageSize: 2, TotalItems: 5, TotalPages: 3}}},
		{name: "partial last page", params: Params{Page: 3, PageSize: 2}, want: PageResult[string]{Items: []string{"e"}, Pagination: Meta{Page: 3, PageSize: 2, TotalItems: 5, TotalPages: 3}}},
		{name: "beyond last page", params: Params{Page: 4, PageSize: 2}, want: PageResult[string]{Items: []string{}, Pagination: Meta{Page: 4, PageSize: 2, TotalItems: 5, TotalPages: 3}}},
		{name: "overflow offset", params: Params{Page: math.MaxInt, PageSize: 2}, want: PageResult[string]{Items: []string{}, Pagination: Meta{Page: math.MaxInt, PageSize: 2, TotalItems: 5, TotalPages: 3}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, Page(items, tt.params))
		})
	}
}

func TestBuildMeta(t *testing.T) {
	tests := []struct {
		name   string
		params Params
		total  int
		want   Meta
	}{
		{name: "multiple pages", params: Params{Page: 2, PageSize: 10}, total: 21, want: Meta{Page: 2, PageSize: 10, TotalItems: 21, TotalPages: 3}},
		{name: "exact page", params: Params{Page: 1, PageSize: 10}, total: 20, want: Meta{Page: 1, PageSize: 10, TotalItems: 20, TotalPages: 2}},
		{name: "empty", params: Params{Page: 1, PageSize: 10}, want: Meta{Page: 1, PageSize: 10}},
		{name: "partial page", params: Params{Page: 1, PageSize: 10}, total: 1, want: Meta{Page: 1, PageSize: 10, TotalItems: 1, TotalPages: 1}},
		{name: "items with zero page size", params: Params{Page: 3}, total: 7, want: Meta{Page: 3, TotalItems: 7}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { require.Equal(t, tt.want, tt.params.BuildMeta(tt.total)) })
	}
}
