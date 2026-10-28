package manager

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/kasuboski/mediaz/pkg/prowlarr"
)

var (
	movieReleaseYear       = regexp.MustCompile(`^(?:19|20)\d{2}(?: |$)`)
	movieTechnicalSuffix   = regexp.MustCompile(`^(?:\d{3,4}p|4k|bluray|blu ray|brrip|web(?: dl|dl| rip|rip)?|hdtv|sdtv|dvd(?:rip)?|remux|[xh] ?26[45]|hevc|av1|aac|dts|hdr(?:10)?|sdr|dv|10bit|proper|repack|amzn|nf|dsnp)(?: |$)`)
	movieEditionSuffix     = regexp.MustCompile(`^(?:directors cut|extended(?: edition| cut)?|theatrical(?: cut)?|ultimate(?: edition)?|unrated|special edition|edition)(?: |$)`)
	movieAdditionalDetails = regexp.MustCompile(`^(?:(?:ddp?|ddpa|ac3|eac3|truehd|aac|dts)[0-9]*|atmos|avc|ma|hd|[0-9]+(?:ch|bit)?|mkv|mp4|avi)(?: |$)`)
	movieReleaseGroup      = regexp.MustCompile(`-([[:alnum:]][[:alnum:]_.-]*)$`)
)

// normalizeReleaseIdentity preserves title words and digits, but ignores release
// separators and punctuation. Apostrophes are removed rather than splitting a
// word, so "Ocean's" and "Oceans" compare equally.
func normalizeReleaseIdentity(s string) string {
	var b strings.Builder
	space := true
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			space = false
		} else if r != '\'' && r != '’' && !space {
			b.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSpace(b.String())
}

// movieReleaseTitleMatches uses the known aliases to locate the title boundary.
// Only a year or recognized release details may follow a complete alias. This
// avoids treating a year in a numeric title (e.g. 1917) as the release year, and
// does not mistake "Brothers Resolution" for "Brothers".
func movieReleaseTitleMatches(title string, params ReleaseFilterParams) bool {
	normalized := normalizeReleaseIdentity(title)
	aliases := []string{params.Title}
	if params.OriginalTitle != nil {
		aliases = append(aliases, *params.OriginalTitle)
	}
	if params.CleanTitle != nil {
		aliases = append(aliases, *params.CleanTitle)
	}
	for _, alias := range aliases {
		alias = normalizeReleaseIdentity(alias)
		if alias == "" || (normalized != alias && !strings.HasPrefix(normalized, alias+" ")) {
			continue
		}
		suffix := strings.TrimSpace(strings.TrimPrefix(normalized, alias))
		// Edition labels may occur between the title and release year/source.
		for {
			loc := movieEditionSuffix.FindStringIndex(suffix)
			if loc == nil {
				break
			}
			suffix = strings.TrimSpace(suffix[loc[1]:])
		}
		if suffix == "" {
			return true
		}
		if loc := movieReleaseYear.FindStringIndex(suffix); loc != nil {
			year, _ := strconv.ParseInt(suffix[:4], 10, 32)
			if params.Year == nil || int32(year) == *params.Year {
				return true
			}
			continue
		}
		if movieTechnicalSuffix.MatchString(suffix) && movieTechnicalDetailsMatch(title, suffix, params.Year) {
			return true
		}
	}
	return false
}

// Without a release year as a title boundary, require recognizable details
// throughout the suffix. Only an explicitly hyphenated release group is opaque.
func movieTechnicalDetailsMatch(release, suffix string, expectedYear *int32) bool {
	if group := movieReleaseGroup.FindStringSubmatch(release); len(group) == 2 {
		groupSuffix := " " + normalizeReleaseIdentity(group[1])
		suffix = strings.TrimSuffix(suffix, groupSuffix)
	}
	for suffix != "" {
		if loc := movieReleaseYear.FindStringIndex(suffix); loc != nil {
			year, _ := strconv.ParseInt(suffix[:4], 10, 32)
			if expectedYear != nil && int32(year) != *expectedYear {
				return false
			}
			suffix = strings.TrimSpace(suffix[loc[1]:])
			continue
		}
		consumed := false
		for _, pattern := range []*regexp.Regexp{movieTechnicalSuffix, movieEditionSuffix, movieAdditionalDetails} {
			if loc := pattern.FindStringIndex(suffix); loc != nil {
				suffix = strings.TrimSpace(suffix[loc[1]:])
				consumed = true
				break
			}
		}
		if !consumed {
			return false
		}
	}
	return true
}

// compareReleaseIDs only compares positive IDs present on both sides. Missing,
// zero and malformed IDs are unavailable evidence, not mismatches. A conflict
// in one namespace must not be hidden by a match in another namespace.
func compareReleaseIDs(release *prowlarr.ReleaseResource, tmdbID int64, imdbID string, tvdbID int64) (matched bool, conflict bool) {
	if release == nil {
		return false, false
	}
	check := func(got *int32, want int64) {
		if got == nil || *got <= 0 || want <= 0 {
			return
		}
		if int64(*got) == want {
			matched = true
		} else {
			conflict = true
		}
	}
	check(release.TmdbID, tmdbID)
	check(release.ImdbID, parseIMDbID(imdbID))
	check(release.TvdbID, tvdbID)
	return matched, conflict
}

func parseIMDbID(value string) int64 {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "tt")
	if value == "" || strings.IndexFunc(value, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
		return 0
	}
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0
	}
	return id
}
