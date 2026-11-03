package manager

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	"github.com/kasuboski/mediaz/pkg/storage"
	sqlcdb "github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

// eligibleForAutomaticAcquisition encodes monitoring gates only. Callers must also
// check missing state, air date, and active-download coverage.
func eligibleForAutomaticAcquisition(series, season, episode int64) bool {
	return series != 0 && season != 0 && episode != 0
}

// isMissingMonitoredEpisode reports whether a scheduled acquisition may consider
// the episode at all: it must be monitored and still missing. Already
// downloading/downloaded/importing episodes are never re-enqueued.
func isMissingMonitoredEpisode(e sqlcdb.ListEpisodesRow) bool {
	return e.Episode.Monitored != 0 && storage.EpisodeState(e.State) == storage.EpisodeStateMissing
}

// hasActiveDownload reports whether the episode already has an in-flight or
// completed download attached, so a new request must not enqueue it again.
func hasActiveDownload(e sqlcdb.ListEpisodesRow) bool {
	return e.DownloadID.Valid && strings.TrimSpace(e.DownloadID.String) != ""
}

// tvProfileResolver resolves and validates effective quality profiles for TV
// acquisition. It uses the published episode > season > series precedence
// helper and never falls back to a different profile when the effective one is
// missing or incompatible.
type tvProfileResolver struct {
	ctx      context.Context
	m        *MediaManager
	seedID   int64
	seedRows []sqlcdb.ListQualityProfileRowsRow
	cache    map[int64][]sqlcdb.ListQualityProfileRowsRow
}

func newTVProfileResolver(ctx context.Context, m *MediaManager, seedID int64, seedRows []sqlcdb.ListQualityProfileRowsRow) *tvProfileResolver {
	return &tvProfileResolver{
		ctx:      ctx,
		m:        m,
		seedID:   seedID,
		seedRows: seedRows,
		cache:    make(map[int64][]sqlcdb.ListQualityProfileRowsRow),
	}
}

// resolve returns the episode-compatible quality profile rows for the effective
// persistent profile of the episode (episode > season > series). Errors are
// returned for missing/invalid/incompatible profiles; there is no fallback.
func (r *tvProfileResolver) resolve(seriesProfileID int64, seasonOverride, episodeOverride sql.NullInt64) ([]sqlcdb.ListQualityProfileRowsRow, int64, string, error) {
	profileID, source := ResolveTVQualityProfileID(seriesProfileID, seasonOverride, episodeOverride)
	rows, err := r.rowsByID(profileID)
	if err != nil {
		return nil, profileID, source, err
	}
	return rows, profileID, source, nil
}

func (r *tvProfileResolver) rowsByID(profileID int64) ([]sqlcdb.ListQualityProfileRowsRow, error) {
	if profileID <= 0 {
		return nil, fmt.Errorf("no effective quality profile")
	}
	if rows, ok := r.cache[profileID]; ok {
		return rows, nil
	}

	rows := r.seedRows
	if profileID != r.seedID || rows == nil {
		if r.m == nil {
			return nil, fmt.Errorf("quality profile %d unavailable: no quality service", profileID)
		}
		fetched, err := r.m.ListQualityProfileRows(r.ctx, profileID)
		if err != nil {
			return nil, fmt.Errorf("quality profile %d unavailable: %w", profileID, err)
		}
		rows = fetched
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("quality profile %d has no definitions", profileID)
	}
	for _, row := range rows {
		// Only episode-compatible profiles may acquire TV content; there is
		// no silent fallback to a movie or foreign profile.
		if row.QualityDefinition.MediaType != "episode" {
			return nil, fmt.Errorf("quality profile %d is not episode-compatible", profileID)
		}
	}

	r.cache[profileID] = rows
	return rows, nil
}

// seasonPackDecision records the conservative season-pack eligibility outcome.
type seasonPackDecision struct {
	eligible   bool
	profileID  int64
	episodeIDs []int64
	reason     string
}

// evaluateSeasonPackEligibility applies the conservative issue-101 pack rule: a
// season pack is only eligible when every known episode in the season is
// selected (monitored), still missing, aired, not actively downloading, and
// shares a single effective quality profile. Partial selection or heterogeneous
// profiles reject the pack so acquisition falls back to individual episodes.
func (r *tvProfileResolver) evaluateSeasonPackEligibility(seriesProfileID int64, seasonOverride sql.NullInt64, season sqlcdb.Season, episodes []sqlcdb.ListEpisodesRow, aired func(sqlcdb.ListEpisodesRow) bool) seasonPackDecision {
	if season.ID == 0 {
		return seasonPackDecision{reason: "no season"}
	}
	if len(episodes) == 0 {
		return seasonPackDecision{reason: "season has no known episodes"}
	}

	var packProfile int64
	ids := make([]int64, 0, len(episodes))
	for _, e := range episodes {
		if e.Episode.Monitored == 0 {
			return seasonPackDecision{reason: fmt.Sprintf("episode %d is not monitored", e.Episode.EpisodeNumber)}
		}
		if storage.EpisodeState(e.State) != storage.EpisodeStateMissing {
			return seasonPackDecision{reason: fmt.Sprintf("episode %d is not missing", e.Episode.EpisodeNumber)}
		}
		if hasActiveDownload(e) {
			return seasonPackDecision{reason: fmt.Sprintf("episode %d already has an active download", e.Episode.EpisodeNumber)}
		}
		if aired != nil && !aired(e) {
			return seasonPackDecision{reason: fmt.Sprintf("episode %d is not aired or lacks air-date metadata", e.Episode.EpisodeNumber)}
		}

		_, profileID, _, err := r.resolve(seriesProfileID, seasonOverride, e.Episode.QualityProfileID)
		if err != nil {
			return seasonPackDecision{reason: fmt.Sprintf("episode %d: %v", e.Episode.EpisodeNumber, err)}
		}
		if packProfile != 0 && packProfile != profileID {
			return seasonPackDecision{reason: fmt.Sprintf("episode %d resolves to a different quality profile (%d != %d)", e.Episode.EpisodeNumber, profileID, packProfile)}
		}
		packProfile = profileID
		ids = append(ids, int64(e.Episode.ID))
	}

	return seasonPackDecision{eligible: true, profileID: packProfile, episodeIDs: ids}
}

// Multi-episode range continuations directly after an explicit S/E marker,
// e.g. S01E01-E02, S01E01+E03, S01E01-02, S01E02-S01E03.
var multiEpisodeRangePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^\s*[-+~]\s*(?:S\d+E|E)\d+`),
	regexp.MustCompile(`(?i)^\s*[-+~]\s*\d{1,3}(?:\D|$)`),
	regexp.MustCompile(`(?i)^\s*E\d+`),
}

// looksLikeMultiEpisodeRelease conservatively reports whether a release title
// appears to cover more than one episode. Single-episode acquisition must never
// collect unrequested siblings via multi-episode releases.
func looksLikeMultiEpisodeRelease(title string) bool {
	if strings.TrimSpace(title) == "" {
		return true
	}

	if loc := episodeNumberPattern.FindStringIndex(title); loc != nil {
		tail := title[loc[1]:]
		for _, pattern := range multiEpisodeRangePatterns {
			if pattern.MatchString(tail) {
				return true
			}
		}
		// Any further episode marker in the tail means extra episodes.
		if len(episodeNumberPattern.FindAllStringSubmatch(tail, -1)) > 0 {
			return true
		}
		if len(episodeCrossPattern.FindAllStringSubmatch(tail, -1)) > 0 {
			return true
		}
		return false
	}

	// Cross-marker path (NxM): more than one marker covers multiple episodes.
	return len(episodeCrossPattern.FindAllStringSubmatch(title, -1)) > 1
}
