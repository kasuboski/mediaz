package manager

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseDiscoveryEvidence(t *testing.T) {
	tests := []struct {
		folder string
		files  []string
		tv     bool
		want   DiscoveryEvidence
		bad    bool
	}{
		{"Apocalypse Now (1979)", nil, false, DiscoveryEvidence{Title: "Apocalypse Now", Query: "Apocalypse Now", Year: ptrInt(1979), names: []string{"Apocalypse Now (1979)"}}, false},
		{"Blade Runner 2049", nil, false, DiscoveryEvidence{Title: "Blade Runner 2049", Query: "Blade Runner 2049", names: []string{"Blade Runner 2049"}}, false},
		{"Cam (2018)", nil, false, DiscoveryEvidence{Title: "Cam", Query: "Cam", Year: ptrInt(2018), names: []string{"Cam (2018)"}}, false},
		{"Generic", []string{"Generic.2012.mkv"}, false, DiscoveryEvidence{Title: "Generic", Query: "Generic", Year: ptrInt(2012), names: []string{"Generic", "Generic.2012"}}, false},
		{"Movie", []string{"movie.mkv"}, false, DiscoveryEvidence{}, false},
		{"Show", []string{"Show.S02E03.1080p.mkv"}, true, DiscoveryEvidence{Title: "Show", Query: "Show", names: []string{"Show", "Show"}}, false},
		{"Film {tmdb-42}", []string{"film.mkv"}, false, DiscoveryEvidence{Title: "Film", Query: "Film", TMDBID: 42, names: []string{"Film", "film"}}, false},
		{"Film {tmdb-42}", []string{"film {tmdb-43}.mkv"}, false, DiscoveryEvidence{}, true},
		{"Show", []string{"Other.S01E01.mkv"}, true, DiscoveryEvidence{Title: "Show", Query: "Show", names: []string{"Show", "Other"}}, false},
	}
	for _, tc := range tests {
		got, err := ParseDiscoveryEvidence(tc.folder, tc.files, tc.tv)
		if tc.bad {
			require.Error(t, err)
			continue
		}
		require.NoError(t, err)
		require.Equal(t, tc.want, got)
	}
}

func TestMatchDiscoveryCandidate(t *testing.T) {
	id := func(i int) *int { return &i }
	str := func(s string) *string { return &s }
	e := DiscoveryEvidence{Title: "Brothers"}
	hits := []*SearchMediaResult{{ID: id(1), Title: str("Brothers"), ReleaseDate: str("2009-01-01")}, {ID: id(2), Title: str("Brothers"), ReleaseDate: str("2024-01-01")}}
	require.Equal(t, DiscoveryMatch{Status: DiscoveryAmbiguous}, MatchDiscoveryCandidate(e, hits, false))
	e.Year = new(int)
	*e.Year = 2009
	require.Equal(t, DiscoveryMatch{Status: DiscoveryMatched, Candidate: hits[0]}, MatchDiscoveryCandidate(e, hits, false))
	e.Title = "Wrong title"
	require.Equal(t, DiscoveryMatch{Status: DiscoveryUnmatched}, MatchDiscoveryCandidate(e, hits, false))
	showHit := &SearchMediaResult{ID: id(3), Name: str("Show"), FirstAirDate: str("2020-01-01")}
	require.Equal(t, DiscoveryMatch{Status: DiscoveryMatched, Candidate: showHit}, MatchDiscoveryCandidate(DiscoveryEvidence{Title: "Show", Year: ptrInt(2020)}, []*SearchMediaResult{showHit}, true))
	duplicateHits := []*SearchMediaResult{{ID: id(1), Title: str("Film")}, {ID: id(1), Title: str("Film")}}
	require.Equal(t, DiscoveryMatch{Status: DiscoveryMatched, Candidate: duplicateHits[1]}, MatchDiscoveryCandidate(DiscoveryEvidence{Title: "Film"}, duplicateHits, false))
}
func ptrInt(i int) *int { return &i }
