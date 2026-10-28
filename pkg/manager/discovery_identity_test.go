package manager

import (
	"testing"
)

func TestParseDiscoveryEvidence(t *testing.T) {
	tests := []struct {
		folder string
		files  []string
		tv     bool
		title  string
		year   int
		id     int
		bad    bool
	}{
		{"Apocalypse Now (1979)", nil, false, "Apocalypse Now", 1979, 0, false},
		{"Blade Runner 2049", nil, false, "Blade Runner 2049", 0, 0, false},
		{"Cam (2018)", nil, false, "Cam", 2018, 0, false},
		{"Generic", []string{"Generic.2012.mkv"}, false, "Generic", 2012, 0, false},
		{"Movie", []string{"movie.mkv"}, false, "", 0, 0, false},
		{"Show", []string{"Show.S02E03.1080p.mkv"}, true, "Show", 0, 0, false},
		{"Film {tmdb-42}", []string{"film.mkv"}, false, "Film", 0, 42, false},
		{"Film {tmdb-42}", []string{"film {tmdb-43}.mkv"}, false, "", 0, 0, true},
		{"Show", []string{"Other.S01E01.mkv"}, true, "Show", 0, 0, false},
	}
	for _, tc := range tests {
		got, err := ParseDiscoveryEvidence(tc.folder, tc.files, tc.tv)
		if (err != nil) != tc.bad {
			t.Fatalf("%q err=%v", tc.folder, err)
		}
		if err == nil {
			if got.Title != tc.title || got.TMDBID != tc.id {
				t.Errorf("%q got %+v", tc.folder, got)
			}
			if tc.year > 0 && (got.Year == nil || *got.Year != tc.year) {
				t.Errorf("year: %+v", got)
			}
		}
	}
}

func TestMatchDiscoveryCandidate(t *testing.T) {
	id := func(i int) *int { return &i }
	str := func(s string) *string { return &s }
	e := DiscoveryEvidence{Title: "Brothers"}
	hits := []*SearchMediaResult{{ID: id(1), Title: str("Brothers"), ReleaseDate: str("2009-01-01")}, {ID: id(2), Title: str("Brothers"), ReleaseDate: str("2024-01-01")}}
	if got := MatchDiscoveryCandidate(e, hits, false); got.Status != DiscoveryAmbiguous {
		t.Fatal(got)
	}
	e.Year = new(int)
	*e.Year = 2009
	if got := MatchDiscoveryCandidate(e, hits, false); got.Status != DiscoveryMatched || *got.Candidate.ID != 1 {
		t.Fatal(got)
	}
	e.Title = "Wrong title"
	if got := MatchDiscoveryCandidate(e, hits, false); got.Status != DiscoveryUnmatched {
		t.Fatal(got)
	}
	if got := MatchDiscoveryCandidate(DiscoveryEvidence{Title: "Show", Year: ptrInt(2020)}, []*SearchMediaResult{{ID: id(3), Name: str("Show"), FirstAirDate: str("2020-01-01")}}, true); got.Status != DiscoveryMatched {
		t.Fatal(got)
	}
	if got := MatchDiscoveryCandidate(DiscoveryEvidence{Title: "Film"}, []*SearchMediaResult{{ID: id(1), Title: str("Film")}, {ID: id(1), Title: str("Film")}}, false); got.Status != DiscoveryMatched {
		t.Fatal(got)
	}
}
func ptrInt(i int) *int { return &i }
