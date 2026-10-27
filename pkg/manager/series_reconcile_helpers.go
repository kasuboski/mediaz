package manager

import (
	"fmt"
	"strings"
	"time"

	"github.com/kasuboski/mediaz/pkg/storage"
	sqlcdb "github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

// initialSeriesState returns Missing or Unreleased based on the first air date.
func initialSeriesState(firstAirDate *time.Time) storage.SeriesState {
	if !isReleased(now(), firstAirDate) {
		return storage.SeriesStateUnreleased
	}
	return storage.SeriesStateMissing
}

// findMatchingSeriesResult picks the first series search result, or one whose first air date year matches.
func findMatchingSeriesResult(results []*SearchMediaResult, year *int32) *SearchMediaResult {
	if len(results) == 0 {
		return nil
	}

	if year == nil {
		return results[0]
	}

	for _, r := range results {
		if r.FirstAirDate != nil && len(*r.FirstAirDate) >= 4 {
			resultYear := (*r.FirstAirDate)[:4]
			if resultYear == fmt.Sprintf("%d", *year) {
				return r
			}
		}
	}

	return nil
}

// getSeasonRuntime sums episode runtimes, estimating missing values from the average.
func getSeasonRuntime(episodeMetadata []*sqlcdb.EpisodeMetadatum, totalSeasonEpisodes int) int64 {
	var runtime int64
	var consideredRuntimeCount int
	for _, meta := range episodeMetadata {
		if meta.Runtime.Valid {
			runtime += meta.Runtime.Int64
			consideredRuntimeCount++
		}
	}

	if consideredRuntimeCount == 0 {
		return 0
	}

	// If we have runtimes for some but not all episodes, calculate an average and apply it to the missing ones.
	if consideredRuntimeCount < totalSeasonEpisodes {
		averageRuntime := runtime / int64(consideredRuntimeCount)
		missingRuntimes := totalSeasonEpisodes - consideredRuntimeCount
		runtime += averageRuntime * int64(missingRuntimes)
	}

	return runtime
}

// determineSeasonState counts episode states and derives the overall season state.
func determineSeasonState(episodes []sqlcdb.ListEpisodesRow) (map[string]int, storage.SeasonState) {
	var done, downloading, missing, unreleased, discovered int
	for _, episode := range episodes {
		switch storage.EpisodeState(episode.State) {
		case storage.EpisodeStateDownloaded, storage.EpisodeStateCompleted:
			done++
		case storage.EpisodeStateDownloading:
			downloading++
		case storage.EpisodeStateMissing:
			missing++
		case storage.EpisodeStateUnreleased:
			unreleased++
		case storage.EpisodeStateDiscovered:
			discovered++
		}
	}

	counts := map[string]int{
		"done":        done,
		"downloading": downloading,
		"missing":     missing,
		"unreleased":  unreleased,
		"discovered":  discovered,
	}

	switch {
	case len(episodes) == 0:
		return counts, storage.SeasonStateMissing
	case done == len(episodes):
		return counts, storage.SeasonStateCompleted
	case downloading > 0:
		return counts, storage.SeasonStateDownloading
	case discovered > 0 && (done > 0 || missing > 0 || downloading > 0):
		return counts, storage.SeasonStateContinuing
	case (done > 0 || missing > 0) && unreleased > 0:
		return counts, storage.SeasonStateContinuing
	case missing > 0 && unreleased == 0:
		return counts, storage.SeasonStateMissing
	case unreleased > 0 && done == 0 && missing == 0:
		return counts, storage.SeasonStateUnreleased
	case discovered > 0 && done == 0 && missing == 0 && downloading == 0 && unreleased == 0:
		return counts, storage.SeasonStateDiscovered
	default:
		return counts, storage.SeasonStateMissing
	}
}

// seriesStateCounts holds per-state counts of seasons used to derive the overall series state.
type seriesStateCounts struct {
	completed   int
	downloading int
	missing     int
	unreleased  int
	discovered  int
	continuing  int
}

// determineSeriesState counts season states (excluding specials) and derives the series state from them
// and the TMDB status string.
func determineSeriesState(seasons []sqlcdb.ListSeasonsRow, tmdbStatus string) (seriesStateCounts, storage.SeriesState) {
	total := 0
	var counts seriesStateCounts
	for _, season := range seasons {
		// dont count specials
		if season.Season.SeasonNumber == 0 {
			continue
		}
		total++
		switch storage.SeasonState(season.State) {
		case storage.SeasonStateCompleted:
			counts.completed++
		case storage.SeasonStateDownloading:
			counts.downloading++
		case storage.SeasonStateMissing:
			counts.missing++
		case storage.SeasonStateUnreleased:
			counts.unreleased++
		case storage.SeasonStateDiscovered:
			counts.discovered++
		case storage.SeasonStateContinuing:
			counts.continuing++
		}
	}

	isSeriesEnded := strings.EqualFold(tmdbStatus, "ended") || strings.EqualFold(tmdbStatus, "canceled")
	isSeriesContinuing := strings.EqualFold(tmdbStatus, "returning series") || strings.EqualFold(tmdbStatus, "in production")

	var state storage.SeriesState
	switch {
	case counts.completed == total && isSeriesEnded:
		state = storage.SeriesStateCompleted
	case counts.downloading > 0:
		state = storage.SeriesStateDownloading
	case counts.discovered == total && isSeriesEnded:
		state = storage.SeriesStateDiscovered
	case isSeriesEnded && counts.missing == 0 && counts.discovered == 0 && (counts.completed > 0 || counts.continuing > 0):
		state = storage.SeriesStateCompleted
	case counts.continuing > 0 || counts.discovered > 0 || (isSeriesContinuing && (counts.completed > 0 || counts.discovered > 0)):
		state = storage.SeriesStateContinuing
	case counts.unreleased == total:
		state = storage.SeriesStateUnreleased
	default:
		state = storage.SeriesStateMissing
	}

	return counts, state
}

// seasonListRow widens a single-season read row into the list-row shape used
// by reconcile helpers (identical columns, non-null state).
func seasonListRow(r sqlcdb.GetSeasonByIDRow) sqlcdb.ListSeasonsRow {
	return sqlcdb.ListSeasonsRow{
		Season:                 r.Season,
		State:                  r.State.String,
		DownloadID:             r.DownloadID,
		DownloadClientID:       r.DownloadClientID,
		IsEntireSeasonDownload: r.IsEntireSeasonDownload,
	}
}

// episodeListRow widens a single-episode read row into the list-row shape
// used by reconcile helpers (identical columns, non-null state).
func episodeListRow(r sqlcdb.GetEpisodeByIDRow) sqlcdb.ListEpisodesRow {
	return sqlcdb.ListEpisodesRow{
		Episode:                r.Episode,
		State:                  r.State.String,
		DownloadID:             r.DownloadID,
		DownloadClientID:       r.DownloadClientID,
		IsEntireSeasonDownload: r.IsEntireSeasonDownload,
	}
}
