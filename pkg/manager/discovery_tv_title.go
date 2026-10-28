package manager

import (
	"regexp"
	"strings"
)

var descriptiveEpisodeMarker = regexp.MustCompile(`(?i)\b(?:series|season)[ ._-]+([0-9]+)[ ._-]+episode[ ._-]+[0-9]+\b`)

// discoveryTVTitlePrefix returns the identifying series prefix and whether a
// recognized episode marker was present. name is a basename without extension.
func discoveryTVTitlePrefix(name string) (prefix string, hasEpisodeMarker bool) {
	boundary := len(name)
	for _, pattern := range []*regexp.Regexp{episodeNumberPattern, episodeCrossPattern, seriesSeasonMarker} {
		if loc := pattern.FindStringIndex(name); loc != nil && loc[0] < boundary {
			boundary = loc[0]
		}
	}
	if loc := descriptiveEpisodeMarker.FindStringIndex(name); loc != nil && loc[0] < boundary {
		boundary = loc[0]
	}
	if boundary == len(name) {
		return name, false
	}
	return strings.TrimSpace(name[:boundary]), true
}
