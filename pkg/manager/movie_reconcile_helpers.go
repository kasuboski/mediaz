package manager

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/kasuboski/mediaz/pkg/storage"
)

// initialMovieState returns Missing or Unreleased based on the release date.
func initialMovieState(releaseDate sql.NullTime) storage.MovieState {
	if !releaseDate.Valid || releaseDate.Time.IsZero() || !time.Now().After(releaseDate.Time) {
		return storage.MovieStateUnreleased
	}
	return storage.MovieStateMissing
}

// findMatchingMovieResult picks the first movie search result, or one whose release year matches.
func findMatchingMovieResult(results []*SearchMediaResult, year *int32) *SearchMediaResult {
	if len(results) == 0 {
		return nil
	}

	if year == nil {
		return results[0]
	}

	for _, r := range results {
		if r.ReleaseDate != nil && len(*r.ReleaseDate) >= 4 {
			resultYear := (*r.ReleaseDate)[:4]
			if resultYear == fmt.Sprintf("%d", *year) {
				return r
			}
		}
	}

	return nil
}
