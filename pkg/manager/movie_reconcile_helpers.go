package manager

import (
	"database/sql"
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
