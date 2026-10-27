package manager

import (
	"database/sql"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/pkg/ptr"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_initialMovieState(t *testing.T) {
	tests := []struct {
		name        string
		releaseDate sql.NullTime
		want        storage.MovieState
	}{
		{name: "null", want: storage.MovieStateUnreleased},
		{name: "invalid with past value", releaseDate: sql.NullTime{Time: time.Now().Add(-time.Hour)}, want: storage.MovieStateUnreleased},
		{name: "zero date", releaseDate: sql.NullTime{Valid: true}, want: storage.MovieStateUnreleased},
		{name: "past", releaseDate: sql.NullTime{Time: time.Now().Add(-time.Hour), Valid: true}, want: storage.MovieStateMissing},
		{name: "future", releaseDate: sql.NullTime{Time: time.Now().Add(time.Hour), Valid: true}, want: storage.MovieStateUnreleased},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, initialMovieState(tt.releaseDate))
		})
	}
}

func Test_findMatchingMovieResult(t *testing.T) {
	release2024 := "2024-01-15"
	release2009 := "2009-06-20"
	release2001 := "2001-11-03"

	tests := []struct {
		name     string
		year     *int32
		results  []*SearchMediaResult
		expected *SearchMediaResult
	}{
		{
			name: "no year - returns first result",
			year: nil,
			results: []*SearchMediaResult{
				{ID: ptr.To(1), Title: ptr.To("Brothers"), ReleaseDate: &release2024},
				{ID: ptr.To(2), Title: ptr.To("Brothers"), ReleaseDate: &release2009},
			},
			expected: &SearchMediaResult{ID: ptr.To(1), Title: ptr.To("Brothers"), ReleaseDate: &release2024},
		},
		{
			name: "year matches first result",
			year: ptr.To(int32(2024)),
			results: []*SearchMediaResult{
				{ID: ptr.To(1), Title: ptr.To("Brothers"), ReleaseDate: &release2024},
				{ID: ptr.To(2), Title: ptr.To("Brothers"), ReleaseDate: &release2009},
			},
			expected: &SearchMediaResult{ID: ptr.To(1), Title: ptr.To("Brothers"), ReleaseDate: &release2024},
		},
		{
			name: "year matches second result",
			year: ptr.To(int32(2009)),
			results: []*SearchMediaResult{
				{ID: ptr.To(1), Title: ptr.To("Brothers"), ReleaseDate: &release2024},
				{ID: ptr.To(2), Title: ptr.To("Brothers"), ReleaseDate: &release2009},
			},
			expected: &SearchMediaResult{ID: ptr.To(2), Title: ptr.To("Brothers"), ReleaseDate: &release2009},
		},
		{
			name: "year not found in results",
			year: ptr.To(int32(2025)),
			results: []*SearchMediaResult{
				{ID: ptr.To(1), Title: ptr.To("Brothers"), ReleaseDate: &release2024},
				{ID: ptr.To(2), Title: ptr.To("Brothers"), ReleaseDate: &release2009},
			},
			expected: nil,
		},
		{
			name: "result with nil release date",
			year: ptr.To(int32(2024)),
			results: []*SearchMediaResult{
				{ID: ptr.To(1), Title: ptr.To("Brothers"), ReleaseDate: nil},
				{ID: ptr.To(2), Title: ptr.To("Brothers"), ReleaseDate: &release2024},
			},
			expected: &SearchMediaResult{ID: ptr.To(2), Title: ptr.To("Brothers"), ReleaseDate: &release2024},
		},
		{
			name: "all results have nil release dates",
			year: ptr.To(int32(2024)),
			results: []*SearchMediaResult{
				{ID: ptr.To(1), Title: ptr.To("Brothers"), ReleaseDate: nil},
				{ID: ptr.To(2), Title: ptr.To("Brothers"), ReleaseDate: nil},
			},
			expected: nil,
		},
		{
			name:     "empty results",
			year:     ptr.To(int32(2024)),
			results:  []*SearchMediaResult{},
			expected: nil,
		},
		{
			name:     "nil results",
			year:     ptr.To(int32(2024)),
			results:  nil,
			expected: nil,
		},
		{
			name: "single result matching year",
			year: ptr.To(int32(2001)),
			results: []*SearchMediaResult{
				{ID: ptr.To(1), Title: ptr.To("Brothers"), ReleaseDate: &release2001},
			},
			expected: &SearchMediaResult{ID: ptr.To(1), Title: ptr.To("Brothers"), ReleaseDate: &release2001},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findMatchingMovieResult(tt.results, tt.year)
			if tt.expected == nil {
				assert.Nil(t, result)
				return
			}
			require.NotNil(t, result)
			assert.Equal(t, tt.expected, result)
		})
	}
}
