package manager

import (
	"testing"

	"github.com/kasuboski/mediaz/pkg/ptr"
	"github.com/stretchr/testify/require"
)

func TestDiscoveryLibraryNamingRegressions(t *testing.T) {
	movie := func(id int, title, year string) *SearchMediaResult {
		return &SearchMediaResult{ID: ptr.To(id), Title: ptr.To(title), ReleaseDate: ptr.To(year + "-01-01")}
	}
	show := func(id int, title, year string) *SearchMediaResult {
		return &SearchMediaResult{ID: ptr.To(id), Name: ptr.To(title), FirstAirDate: ptr.To(year + "-01-01")}
	}
	for _, tc := range []struct {
		name, folder   string
		files          []string
		tv             bool
		hits           []*SearchMediaResult
		want           DiscoveryMatch
		wantParseError bool
	}{
		{
			name: "Rosencrantz actor interview auxiliaries", folder: "Rosencrantz and Guildenstern are Dead (1990)",
			files: []string{"Rosencrantz and Guildenstern are Dead/Gary Oldman interview.avi", "Rosencrantz and Guildenstern are Dead/Tim Roth interview.avi", "Rosencrantz and Guildenstern Are Dead.1990.avi"},
			hits:  []*SearchMediaResult{movie(1, "Rosencrantz and Guildenstern Are Dead", "1990")},
			want:  DiscoveryMatch{Status: DiscoveryMatched, Candidate: movie(1, "Rosencrantz and Guildenstern Are Dead", "1990")},
		},
		{
			name: "WALL-E split filenames", folder: "WALL-E (2008)",
			files: []string{"WALL-E/Wall E-1.mp4", "WALL-E/Wall E-2.mp4", "WALL-E/Wall E.mkv"},
			hits:  []*SearchMediaResult{movie(2, "WALL·E", "2008")},
			want:  DiscoveryMatch{Status: DiscoveryMatched, Candidate: movie(2, "WALL·E", "2008")},
		},
		{
			name: "TV marker truncates episode-title tags", folder: "Would I Lie to You? (2007)", tv: true,
			files: []string{"Would I Lie to You Series 14 Episode 4 (S14E04)-UXTniZeKxHI.mp4", "Would I Lie to You S14E04.mp4"},
			hits:  []*SearchMediaResult{show(3, "Would I Lie to You?", "2007")},
			want:  DiscoveryMatch{Status: DiscoveryMatched, Candidate: show(3, "Would I Lie to You?", "2007")},
		},
		{
			name: "Toy Story sequel does not match original folder", folder: "Toy Story",
			files: []string{"Toy Story/Toy Story 4 (2019) Bluray-1080p.mkv"},
			hits:  []*SearchMediaResult{movie(4, "Toy Story", "1995"), movie(5, "Toy Story 4", "2019")},
			want:  DiscoveryMatch{Status: DiscoveryUnmatched},
		},
		{
			name: "Mockingjay part one does not match franchise or sequel", folder: "The Hunger Games",
			files: []string{"The Hunger Games/The Hunger Games Mockingjay - Part 1 (2014) Bluray-1080p.mp4"},
			hits:  []*SearchMediaResult{movie(6, "The Hunger Games", "2012"), movie(7, "The Hunger Games: Mockingjay - Part 1", "2014")},
			want:  DiscoveryMatch{Status: DiscoveryUnmatched},
		},
		{
			name: "genuine part-number title retained", folder: "The Hunger Games Mockingjay Part 1 (2014)",
			files: []string{"The Hunger Games Mockingjay Part 1.mkv"},
			hits:  []*SearchMediaResult{movie(7, "The Hunger Games: Mockingjay - Part 1", "2014")},
			want:  DiscoveryMatch{Status: DiscoveryMatched, Candidate: movie(7, "The Hunger Games: Mockingjay - Part 1", "2014")},
		},
		{
			name: "sequel digits remain with unsuffixed sibling", folder: "Toy Story",
			files: []string{"Toy Story.mkv", "Toy Story 1.mkv", "Toy Story 2.mkv"},
			hits:  []*SearchMediaResult{movie(4, "Toy Story", "1995"), movie(5, "Toy Story 2", "1999")},
			want:  DiscoveryMatch{Status: DiscoveryUnmatched},
		},
		{
			name: "interview word does not hide a conflicting feature", folder: "Alien (1979)",
			files: []string{"Interview with the Vampire (1994).mkv"},
			hits:  []*SearchMediaResult{movie(8, "Alien", "1979"), movie(9, "Interview with the Vampire", "1994")},
			want:  DiscoveryMatch{Status: DiscoveryUnmatched},
		},
		{
			name: "interview word does not hide an additional conflicting feature", folder: "Alien (1979)",
			files: []string{"Alien (1979).mkv", "Interview with the Vampire (1994).mkv"},
			hits:  []*SearchMediaResult{movie(8, "Alien", "1979"), movie(9, "Interview with the Vampire", "1994")},
			want:  DiscoveryMatch{Status: DiscoveryUnmatched},
		},
		{
			name: "label-like feature title with a year is not an auxiliary", folder: "Alien (1979)",
			files: []string{"Alien (1979).mkv", "The Interview (2014).mkv"},
			hits:  []*SearchMediaResult{movie(8, "Alien", "1979"), movie(12, "The Interview", "2014")},
			want:  DiscoveryMatch{Status: DiscoveryUnmatched},
		},
		{
			name: "year conflict remains unresolved", folder: "Locke (2014)",
			files: []string{"Locke (2013) Bluray-1080p.mp4"},
			hits:  []*SearchMediaResult{movie(10, "Locke", "2013"), movie(11, "Locke", "2014")},
			want:  DiscoveryMatch{Status: DiscoveryUnmatched},
		},
		{
			name: "conflicting feature ID", folder: "Film {tmdb-10}", files: []string{"Film {tmdb-11}.mkv"},
			wantParseError: true,
		},
		{
			name: "conflicting folder ID", folder: "Film {tmdb-10}", files: []string{"Film {tmdb-10}.mkv", "Film {tmdb-11}.mkv"},
			wantParseError: true,
		},
		{
			name: "feature ID not hidden by auxiliary words", folder: "Alien {tmdb-10}",
			files: []string{"The Interview {tmdb-11}.mkv"}, wantParseError: true,
		},
		{
			name: "ID before descriptive TV marker conflicts", folder: "Show {tmdb-20}", tv: true,
			files: []string{"Show {tmdb-21} Series 14 Episode 4 (S14E04).mkv"}, wantParseError: true,
		},
		{
			name: "ID after descriptive TV marker ignored", folder: "Show {tmdb-20}", tv: true,
			files: []string{"Show Series 14 Episode 4 {tmdb-99} (S14E04).mkv"},
			hits:  []*SearchMediaResult{show(20, "Show", "2020")},
			want:  DiscoveryMatch{Status: DiscoveryMatched, Candidate: show(20, "Show", "2020")},
		},
		{
			name: "ID after TV episode marker ignored", folder: "Show {tmdb-20}", tv: true,
			files: []string{"Show S01E01 {tmdb-99}.mkv"}, hits: []*SearchMediaResult{show(20, "Show", "2020")},
			want: DiscoveryMatch{Status: DiscoveryMatched, Candidate: show(20, "Show", "2020")},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			evidence, err := ParseDiscoveryEvidence(tc.folder, tc.files, tc.tv)
			if tc.wantParseError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, MatchDiscoveryCandidate(evidence, tc.hits, tc.tv))
		})
	}
}
