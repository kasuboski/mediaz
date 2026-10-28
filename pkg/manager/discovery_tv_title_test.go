package manager

import "testing"

func TestDiscoveryTVTitlePrefix(t *testing.T) {
	tests := []struct {
		name   string
		want   string
		marker bool
	}{
		{"Show.S01E02.mkv", "Show", true},
		{"Show 1x02 - Pilot", "Show", true},
		{"Show Season 2", "Show", true},
		{"Show S02", "Show", true},
		{"Would I Lie To You Series 14 Episode 4 (S14E04)-UXTniZeKxHI", "Would I Lie To You", true},
		{"Show Season 2 Episode 3.mkv", "Show", true},
		{"Show.series_14_episode_4 [tmdb-12]", "Show.", true},
		{"A Series of Unfortunate Events", "A Series of Unfortunate Events", false},
		{"Episodes", "Episodes", false},
		{"Season of the Witch", "Season of the Witch", false},
		{"Series 14", "Series 14", false},
		{"1917", "1917", false},
		{"Show {tmdb-12} S01E02", "Show {tmdb-12}", true},
		{"Show S01E02 {tmdb-12}", "Show", true},
		{"Show {tmdb-12} Series 14 Episode 4 {tmdb-99} (S14E04)", "Show {tmdb-12}", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, marker := discoveryTVTitlePrefix(tt.name)
			if got != tt.want || marker != tt.marker {
				t.Fatalf("discoveryTVTitlePrefix(%q) = (%q, %v), want (%q, %v)", tt.name, got, marker, tt.want, tt.marker)
			}
		})
	}
}
