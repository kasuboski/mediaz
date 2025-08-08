package library

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEpisodeFileFromPath(t *testing.T) {
	f, err := os.Open("./testing/test_episodes.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		p := scanner.Text()
		ef := EpisodeFileFromPath(p)
		base := filepath.Base(p)
		if ef.Name != base {
			t.Fatalf("Name mismatch for %s: want %s got %s", p, base, ef.Name)
		}
		series := sanitizeName(dirName(filepath.Dir(p)))
		if ef.SeriesName != series {
			t.Fatalf("SeriesName mismatch for %s: want %s got %s", p, series, ef.SeriesName)
		}
		season := 0
		parent := dirName(p)
		if strings.HasPrefix(strings.ToLower(parent), "season ") {
			var n int
			fmt.Sscanf(parent, "Season %d", &n)
			season = n
		}
		if ef.SeasonNumber != season {
			t.Fatalf("SeasonNumber mismatch for %s: want %d got %d", p, season, ef.SeasonNumber)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestExtractEpisodeNumber(t *testing.T) {
	tests := []struct {
		filename string
		expected int
		desc     string
	}{
		// S##E## format
		{"Show Name S01E05 Episode Title.mkv", 5, "Standard S01E05 format"},
		{"show.name.s02e12.episode.title.avi", 12, "Lowercase s02e12 format"},
		{"Series S10E01.mp4", 1, "S10E01 format with leading zero"},
		{"Series S01E100.mkv", 100, "Three digit episode number"},

		// #x## format
		{"Show Name 1x05 Episode Title.avi", 5, "1x05 format"},
		{"series.2x12.title.mp4", 12, "2x12 format with dots"},
		{"Show 10x01 Title.mkv", 1, "Double digit season in #x## format"},

		// Episode/Ep format
		{"Show Name Episode 5 Title.mp4", 5, "Episode 5 format"},
		{"Series Ep 12 Title.avi", 12, "Ep 12 format"},
		{"Show Episode05 Title.mkv", 5, "Episode05 format (no space)"},
		{"Series Ep05.mp4", 5, "Ep05 format (no space)"},

		// E## format
		{"Show Name E05 Title.mkv", 5, "E05 format"},
		{"series.E12.title.avi", 12, "E12 format with dots"},
		{"Show-E01-Title.mp4", 1, "E01 format with dashes"},

		// - ## - format (episode between dashes)
		{"Show Name - 05 - Episode Title.mkv", 5, "Episode between dashes"},
		{"Series-12-Title.avi", 12, "Episode between dashes (no spaces)"},
		{"Show - 1 - Title.mp4", 1, "Single digit episode between dashes"},

		// Edge cases
		{"Show Name.mkv", 0, "No episode number"},
		{"Random File.avi", 0, "Random filename without episode info"},
		{"Show E00 Title.mp4", 0, "E00 should return 0 (invalid episode)"},
		{"Show Season 1 Title.mkv", 0, "Season in title but no episode"},

		// Real-world examples from test data
		{"Doctor Who (1963) - s01e01 - An Unearthly Child (1).mp4", 1, "Real example s01e01"},
		{"Grey's Anatomy (2005) - s01e02 - The First Cut is the Deepest.avi", 2, "Real example s01e02"},
		{"S01E01 - The Old Grist Mill Bluray-1080p.mkv", 1, "Real example S01E01 at start"},
		{"The Office (US) - s01e01 - Pilot.mkv", 1, "Real example with series name"},

		// Multi-episode files
		{"Grey's Anatomy (2005) - s02e01-e03.avi", 1, "Multi-episode file (should extract first)"},

		// Special characters and formats
		{"Show [1x05] Title.mkv", 5, "Episode in brackets"},
		{"Series (S01E05) Title.avi", 5, "Episode in parentheses"},
		{"Show.Name.1x05.720p.HDTV.x264.mkv", 5, "Episode with quality info"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			result := extractEpisodeNumber(tt.filename)
			if result != tt.expected {
				t.Errorf("extractEpisodeNumber(%q) = %d, want %d", tt.filename, result, tt.expected)
			}
		})
	}
}

func TestEpisodeFileFromPathWithEpisodeNumbers(t *testing.T) {
	tests := []struct {
		path            string
		expectedEpisode int
		expectedSeason  int
		expectedSeries  string
		desc            string
	}{
		{
			path:            "Doctor Who (1963)/Season 01/Doctor Who (1963) - s01e01 - An Unearthly Child (1).mp4",
			expectedEpisode: 1,
			expectedSeason:  1,
			expectedSeries:  "Doctor Who (1963)",
			desc:            "Standard season directory with S01E01 filename",
		},
		{
			path:            "Grey's Anatomy (2005)/Season 02/Grey's Anatomy (2005) - s02e04.m4v",
			expectedEpisode: 4,
			expectedSeason:  2,
			expectedSeries:  "Grey's Anatomy (2005)",
			desc:            "Season 2 with episode 4",
		},
		{
			path:            "Over the Garden Wall/S01E01 - The Old Grist Mill Bluray-1080p.mkv",
			expectedEpisode: 1,
			expectedSeason:  0,
			expectedSeries:  ".",
			desc:            "No season directory but S01E01 in filename",
		},
		{
			path:            "The Office (US) (2005)/Season 01/The Office (US) - 1x05 - Pilot.mkv",
			expectedEpisode: 5,
			expectedSeason:  1,
			expectedSeries:  "The Office (US) (2005)",
			desc:            "1x05 format in filename",
		},
		{
			path:            "Show Name/Season 03/Show Name Episode 12 Title.mp4",
			expectedEpisode: 12,
			expectedSeason:  3,
			expectedSeries:  "Show Name",
			desc:            "Episode 12 format",
		},
		{
			path:            "Random Show/Some File.avi",
			expectedEpisode: 0,
			expectedSeason:  0,
			expectedSeries:  ".",
			desc:            "No episode or season info",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			result := EpisodeFileFromPath(tt.path)

			if result.EpisodeNumber != tt.expectedEpisode {
				t.Errorf("EpisodeFileFromPath(%q).EpisodeNumber = %d, want %d", tt.path, result.EpisodeNumber, tt.expectedEpisode)
			}

			if result.SeasonNumber != tt.expectedSeason {
				t.Errorf("EpisodeFileFromPath(%q).SeasonNumber = %d, want %d", tt.path, result.SeasonNumber, tt.expectedSeason)
			}

			if result.SeriesName != tt.expectedSeries {
				t.Errorf("EpisodeFileFromPath(%q).SeriesName = %q, want %q", tt.path, result.SeriesName, tt.expectedSeries)
			}
		})
	}
}
