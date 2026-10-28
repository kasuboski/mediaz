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
		{name: "overflow", params: Params{Page: math.MaxInt, PageSize: 2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			offset, limit := tt.params.CalculateOffsetLimit()
			require.Equal(t, tt.offset, offset)
			require.Equal(t, tt.limit, limit)
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { require.Equal(t, tt.want, tt.params.BuildMeta(tt.total)) })
	}
}
