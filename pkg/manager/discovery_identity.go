package manager

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// DiscoveryEvidence keeps a convenient search query separately from the raw
// names used to validate results. Query cleanup is not proof of identity.
type DiscoveryEvidence struct {
	Title  string
	Query  string
	Year   *int
	TMDBID int
	names  []string
}

type DiscoveryMatchStatus string

const (
	DiscoveryUnmatched DiscoveryMatchStatus = "unmatched"
	DiscoveryMatched   DiscoveryMatchStatus = "matched"
	DiscoveryAmbiguous DiscoveryMatchStatus = "ambiguous"
)

type DiscoveryMatch struct {
	Status    DiscoveryMatchStatus
	Candidate *SearchMediaResult
}

var (
	discoveryID           = regexp.MustCompile(`(?i)\{tmdb-([0-9]+)\}`)
	discoveryBracketYear  = regexp.MustCompile(`[\(\[\{]((?:19|20)[0-9]{2})[\)\]\}]`)
	discoveryDiscFilename = regexp.MustCompile(`^(?:cd|disc|disk|part) ?[0-9]+$`)
)

// ParseDiscoveryEvidence reads the folder and stored filenames, never the
// filesystem itself. Meaningful names must all agree with the eventual TMDB
// candidate (including its original title); generic and auxiliary filenames add
// no evidence, and corroborated multipart suffixes are not part of the title.
func ParseDiscoveryEvidence(folder string, files []string, isTV bool) (DiscoveryEvidence, error) {
	var out DiscoveryEvidence
	add := func(raw string, file bool) error {
		name := raw
		if file {
			if isTV {
				name = filepath.Base(raw)
				name = strings.TrimSuffix(name, filepath.Ext(name))
			} else {
				var keep bool
				name, keep = movieDiscoveryFilename(folder, raw, files)
				if !keep {
					return nil
				}
			}
		}
		if isTV && file {
			// Only the portion before an episode/season marker identifies the
			// series. An ID after that marker may identify an episode, not a show.
			prefix, hasMarker := discoveryTVTitlePrefix(name)
			if !hasMarker {
				// A bare episode title such as Pilot.mkv is not a series title.
				return nil
			}
			name = prefix
		}
		for _, match := range discoveryID.FindAllStringSubmatch(name, -1) {
			id, err := strconv.Atoi(match[1])
			if err != nil || id <= 0 {
				return fmt.Errorf("invalid discovery TMDB ID %q", match[1])
			}
			if out.TMDBID > 0 && out.TMDBID != id {
				return fmt.Errorf("conflicting discovery TMDB IDs %d and %d", out.TMDBID, id)
			}
			out.TMDBID = id
		}
		name = strings.TrimSpace(discoveryID.ReplaceAllString(name, " "))
		var bracketYear string
		for _, match := range discoveryBracketYear.FindAllStringSubmatch(name, -1) {
			if bracketYear != "" && bracketYear != match[1] {
				return fmt.Errorf("conflicting bracketed discovery years in %q", name)
			}
			bracketYear = match[1]
		}
		query, year := discoverySearchTerm(name)
		normalized := normalizeReleaseIdentity(query)
		if normalized == "" || (genericDiscoveryFilename(normalized) && (file || year == nil)) {
			return nil
		}
		if file && movieTechnicalSuffix.MatchString(normalized) && movieTechnicalDetailsMatch(name, normalized, nil) {
			return nil // 1080p.mkv and x264.mkv are not movie titles.
		}
		out.names = append(out.names, name)
		if out.Query == "" {
			out.Title, out.Query = query, query
		}
		if out.Year == nil {
			out.Year = year
		}
		return nil
	}
	if err := add(folder, false); err != nil {
		return DiscoveryEvidence{}, err
	}
	for _, file := range files {
		if err := add(file, true); err != nil {
			return DiscoveryEvidence{}, err
		}
	}
	return out, nil
}

func genericDiscoveryFilename(name string) bool {
	switch name {
	case "movie", "video", "sample", "proof", "feature", "main", "episode", "bonus", "extras", "trailer", "featurette":
		return true
	}
	return discoveryDiscFilename.MatchString(name)
}

// discoverySearchTerm is deliberately only a query heuristic. The untouched
// name is later compared against each candidate's known title, so numbers in
// titles such as 1917, 2012 and Blade Runner 2049 are not mistaken for evidence
// of a conflicting release year.
func discoverySearchTerm(name string) (string, *int) {
	name = strings.TrimSpace(normalizeSeparators(name))
	var year *int
	if matches := discoveryBracketYear.FindAllStringSubmatchIndex(name, -1); len(matches) > 0 {
		match := matches[len(matches)-1]
		value, _ := strconv.Atoi(name[match[2]:match[3]])
		year = &value
		name = discoveryBracketYear.ReplaceAllString(name, " ")
	}
	words := strings.Fields(name)
	for i := 1; i < len(words); i++ {
		if suffix := normalizeReleaseIdentity(strings.Join(words[i:], " ")); movieTechnicalSuffix.MatchString(suffix) || movieEditionSuffix.MatchString(suffix) {
			words = words[:i]
			break
		}
	}
	if year == nil && len(words) > 1 {
		last := strings.Trim(words[len(words)-1], "()[]{}")
		if len(last) == 4 && movieReleaseYear.MatchString(last) {
			value, _ := strconv.Atoi(last)
			// A far-future number is more likely part of the title (2049)
			// than a release year. This affects querying only, not matching.
			if value <= time.Now().Year()+1 {
				year = &value
				words = words[:len(words)-1]
			}
		}
	}
	return strings.Join(words, " "), year
}

// MatchDiscoveryCandidate requires every meaningful source name to match a
// candidate's title or original title and any release year following that title.
// It never uses TMDB ranking/popularity to break an identity tie.
func MatchDiscoveryCandidate(e DiscoveryEvidence, candidates []*SearchMediaResult, isTV bool) DiscoveryMatch {
	seen := map[int]*SearchMediaResult{}
	names := e.names
	if len(names) == 0 && e.Title != "" {
		name := e.Title
		if e.Year != nil {
			name += fmt.Sprintf(" (%d)", *e.Year)
		}
		names = []string{name}
	}
	for _, candidate := range candidates {
		if candidate == nil || candidate.ID == nil || *candidate.ID <= 0 {
			continue
		}
		if e.TMDBID > 0 {
			if *candidate.ID == e.TMDBID {
				seen[*candidate.ID] = candidate
			}
			continue
		}
		title, original, date := candidate.Title, candidate.OriginalTitle, candidate.ReleaseDate
		if isTV {
			title, original, date = candidate.Name, candidate.OriginalName, candidate.FirstAirDate
		}
		// A missing date cannot substantiate a year in a source name. The
		// sentinel still allows an exact yearless title, including numeric titles.
		year := int32(-1)
		if date != nil && len(*date) >= 4 {
			if value, err := strconv.ParseInt((*date)[:4], 10, 32); err == nil {
				year = int32(value)
			}
		}
		params := ReleaseFilterParams{OriginalTitle: original, Year: &year}
		if title != nil {
			params.Title = *title
		}
		matches := len(names) > 0
		for _, name := range names {
			if !movieReleaseTitleMatches(name, params) {
				matches = false
				break
			}
		}
		if matches {
			seen[*candidate.ID] = candidate
		}
	}
	if len(seen) == 0 {
		return DiscoveryMatch{Status: DiscoveryUnmatched}
	}
	if len(seen) > 1 {
		return DiscoveryMatch{Status: DiscoveryAmbiguous}
	}
	for _, candidate := range seen {
		return DiscoveryMatch{Status: DiscoveryMatched, Candidate: candidate}
	}
	return DiscoveryMatch{Status: DiscoveryUnmatched}
}
