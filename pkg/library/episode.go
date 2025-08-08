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
	seasonDirRe = regexp.MustCompile(`(?i)^season\s*(\d+)`)
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

func EpisodeFileFromPath(path string) EpisodeFile {
	name := sanitizeName(filepath.Base(path))
	series := sanitizeName(dirName(filepath.Dir(path)))
	season := 0
	parent := dirName(path)
	if m := seasonDirRe.FindStringSubmatch(parent); len(m) == 2 {
		if n, err := strconv.Atoi(strings.TrimLeft(m[1], "0")); err == nil {
			season = n
		}
	}

	// Extract episode number from filename
	episode := extractEpisodeNumber(name)

	return EpisodeFile{
		Name:          name,
		RelativePath:  path,
		SeriesName:    series,
		SeasonNumber:  season,
		EpisodeNumber: episode,
	}
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
