package library

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type EpisodeFile struct {
	Name         string
	Size         int64
	RelativePath string
	AbsolutePath string

	SeriesName    string
	SeasonNumber  int
	EpisodeNumber int
}

var (
	seasonDirRe    = regexp.MustCompile(`(?i)^season\s*(\d+)`)
	seasonPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)s(\d+)e\d+`),
		regexp.MustCompile(`(?i)(\d+)x\d+`),
	}
	// Episode number extraction patterns, ordered by preference
	episodePatterns = []*regexp.Regexp{
		// S01E05 or s01e05 format
		regexp.MustCompile(`(?i)s\d+e(\d+)`),
		// 1x05 format
		regexp.MustCompile(`(?i)\d+x(\d+)`),
		// Episode 5 or Ep 5 format
		regexp.MustCompile(`(?i)(?:episode?|ep)\s*(\d+)`),
		// E05 format (standalone E followed by digits)
		regexp.MustCompile(`(?i)(?:^|[^a-z])e(\d+)(?:[^a-z]|$)`),
		// - 05 - format (episode number between dashes)
		regexp.MustCompile(`-\s*(\d+)\s*-`),
	}
)

func EpisodeFileFromPath(path string, libraryRoot ...string) EpisodeFile {
	name := sanitizeName(filepath.Base(path))

	// Extract series name from path structure
	// Support both directory structures:
	// 1. "Series/Season X/Episode.mkv" - traditional structure with season directories
	// 2. "Series/Episode.mkv" - flat structure with episodes directly in series directory
	pathParts := strings.Split(path, string(filepath.Separator))
	var series string
	if len(pathParts) >= 2 {
		// Extract series name from first path component
		series = sanitizeName(pathParts[0])
	} else {
		// Single path component - extract from directory structure
		series = sanitizeName(dirName(filepath.Dir(path)))
	}

	season := extractSeasonNumber(name, dirName(path))
	episode := extractEpisodeNumber(name)

	var absolutePath string
	if len(libraryRoot) > 0 && libraryRoot[0] != "" {
		absolutePath = filepath.Join(libraryRoot[0], path)
	}

	return EpisodeFile{
		Name:          name,
		RelativePath:  path,
		AbsolutePath:  absolutePath,
		SeriesName:    series,
		SeasonNumber:  season,
		EpisodeNumber: episode,
	}
}

func extractSeasonNumber(filename, dirName string) int {
	if m := seasonDirRe.FindStringSubmatch(dirName); len(m) == 2 {
		if n, err := strconv.Atoi(strings.TrimLeft(m[1], "0")); err == nil {
			return n
		}
	}

	name := strings.TrimSuffix(filename, filepath.Ext(filename))
	for _, pattern := range seasonPatterns {
		if matches := pattern.FindStringSubmatch(name); len(matches) >= 2 {
			if season, err := strconv.Atoi(strings.TrimLeft(matches[1], "0")); err == nil {
				if season > 0 {
					return season
				}
			}
		}
	}

	return 0
}

// extractEpisodeNumber extracts episode number from filename using various patterns
func extractEpisodeNumber(filename string) int {
	// Remove file extension for cleaner matching
	name := strings.TrimSuffix(filename, filepath.Ext(filename))

	for _, pattern := range episodePatterns {
		if matches := pattern.FindStringSubmatch(name); len(matches) >= 2 {
			if episode, err := strconv.Atoi(strings.TrimLeft(matches[1], "0")); err == nil {
				if episode > 0 { // Episode 0 is typically not valid
					return episode
				}
			}
		}
	}

	return 0 // No episode number found
}
