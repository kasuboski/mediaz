package server

import (
	"fmt"
	"net/http"

	"github.com/kasuboski/mediaz/pkg/storage"
)

func movieStates(r *http.Request) ([]storage.MovieState, error) {
	seen := map[storage.MovieState]bool{}
	var result []storage.MovieState
	for _, raw := range r.URL.Query()["state"] {
		state := storage.MovieState(raw)
		switch state {
		case storage.MovieStateMissing, storage.MovieStateDiscovered, storage.MovieStateUnreleased, storage.MovieStateDownloading, storage.MovieStateDownloaded:
		default:
			return nil, fmt.Errorf("invalid movie state %q", raw)
		}
		if !seen[state] {
			result = append(result, state)
			seen[state] = true
		}
	}
	return result, nil
}

func seriesStates(r *http.Request) ([]storage.SeriesState, error) {
	seen := map[storage.SeriesState]bool{}
	var result []storage.SeriesState
	for _, raw := range r.URL.Query()["state"] {
		state := storage.SeriesState(raw)
		switch state {
		case storage.SeriesStateMissing, storage.SeriesStateDiscovered, storage.SeriesStateUnreleased, storage.SeriesStateContinuing, storage.SeriesStateDownloading, storage.SeriesStateCompleted:
		default:
			return nil, fmt.Errorf("invalid TV state %q", raw)
		}
		if !seen[state] {
			result = append(result, state)
			seen[state] = true
		}
	}
	return result, nil
}
