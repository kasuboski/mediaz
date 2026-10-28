package manager

import (
	"testing"

	"github.com/kasuboski/mediaz/pkg/ptr"
	"github.com/stretchr/testify/require"
)

func TestDiscoveryMatchingUsesAllNamesAndUniqueIdentity(t *testing.T) {
	movie := func(id int, title, date string) *SearchMediaResult {
		return &SearchMediaResult{ID: ptr.To(id), Title: ptr.To(title), ReleaseDate: ptr.To(date)}
	}
	show := func(id int, title, date string) *SearchMediaResult {
		return &SearchMediaResult{ID: ptr.To(id), Name: ptr.To(title), FirstAirDate: ptr.To(date)}
	}
	for _, tc := range []struct {
		name, folder string
		files        []string
		tv           bool
		hits         []*SearchMediaResult
		status       DiscoveryMatchStatus
		index        int
	}{
		{"Apocalypse Now exact not first same year", "Apocalypse Now (1979)", nil, false, []*SearchMediaResult{movie(1, "Wrong Movie", "1979-01-01"), movie(2, "Apocalypse Now", "1979-08-15"), movie(3, "Apocalypse Now Redux", "1979-01-01")}, DiscoveryMatched, 1},
		{"yearless remakes ambiguous", "Brothers", nil, false, []*SearchMediaResult{movie(1, "Brothers", "2009-01-01"), movie(2, "Brothers", "2024-01-01")}, DiscoveryAmbiguous, 0},
		{"filename year refines folder", "Brothers", []string{"Brothers/Brothers.2009.1080p.BluRay.mkv"}, false, []*SearchMediaResult{movie(2, "Brothers", "2024-01-01"), movie(1, "Brothers", "2009-01-01")}, DiscoveryMatched, 1},
		{"conflicting filenames", "Brothers", []string{"Brothers.2009.mkv", "Brothers.2024.mkv"}, false, []*SearchMediaResult{movie(1, "Brothers", "2009-01-01"), movie(2, "Brothers", "2024-01-01")}, DiscoveryUnmatched, 0},
		{"conflicting folder year", "Brothers (2009)", []string{"Brothers.2024.mkv"}, false, []*SearchMediaResult{movie(1, "Brothers", "2009-01-01"), movie(2, "Brothers", "2024-01-01")}, DiscoveryUnmatched, 0},
		{"generic filename ignored", "Apocalypse Now (1979)", []string{"movie.mkv", "1080p.mkv", "x264.mkv", "cd1.mkv", "bonus.mkv"}, false, []*SearchMediaResult{movie(1, "Apocalypse Now", "1979-01-01")}, DiscoveryMatched, 0},
		{"generic folder refined", "Movie", []string{"Apocalypse.Now.1979.1080p.mkv"}, false, []*SearchMediaResult{movie(1, "Apocalypse Now", "1979-01-01")}, DiscoveryMatched, 0},
		{"different filename title conflicts", "Brothers", []string{"Step.Brothers.2008.1080p.mkv"}, false, []*SearchMediaResult{movie(1, "Brothers", "2008-01-01"), movie(2, "Step Brothers", "2008-01-01")}, DiscoveryUnmatched, 0},
		{"punctuation", "Ocean's Eleven (2001)", []string{"Oceans.Eleven.2001.mkv"}, false, []*SearchMediaResult{movie(1, "Ocean's Eleven", "2001-01-01")}, DiscoveryMatched, 0},
		{"Cam is a title not quality", "Cam (2018)", []string{"Cam.2018.1080p.mkv"}, false, []*SearchMediaResult{movie(1, "Cam", "2018-01-01")}, DiscoveryMatched, 0},
		{"HD is a title", "HD", nil, false, []*SearchMediaResult{movie(1, "HD", "2001-01-01")}, DiscoveryMatched, 0},
		{"1917", "1917 (2019)", []string{"1917.2019.1080p.mkv"}, false, []*SearchMediaResult{movie(1, "1917", "2019-01-01")}, DiscoveryMatched, 0},
		{"2012", "2012 (2009)", []string{"2012.2009.1080p.mkv"}, false, []*SearchMediaResult{movie(1, "2012", "2009-01-01")}, DiscoveryMatched, 0},
		{"2001 title", "2001: A Space Odyssey (1968)", nil, false, []*SearchMediaResult{movie(1, "2001: A Space Odyssey", "1968-01-01")}, DiscoveryMatched, 0},
		{"title year preserved", "Blade Runner 2049 (2017)", nil, false, []*SearchMediaResult{movie(1, "Blade Runner", "1982-01-01"), movie(2, "Blade Runner 2049", "2017-01-01")}, DiscoveryMatched, 1},
		{"bare title number preserved for matching", "Class of 1984", nil, false, []*SearchMediaResult{movie(1, "Class of 1984", "1982-01-01")}, DiscoveryMatched, 0},
		{"Unicode", "Amélie (2001)", nil, false, []*SearchMediaResult{movie(1, "Amélie", "2001-01-01")}, DiscoveryMatched, 0},
		{"year cannot be verified", "Brothers (2009)", nil, false, []*SearchMediaResult{{ID: ptr.To(1), Title: ptr.To("Brothers")}}, DiscoveryUnmatched, 0},
		{"missing ID cannot link", "Brothers", nil, false, []*SearchMediaResult{nil, {Title: ptr.To("Brothers")}, {ID: ptr.To(0), Title: ptr.To("Brothers")}}, DiscoveryUnmatched, 0},
		{"same ID is not ambiguity", "Brothers", nil, false, []*SearchMediaResult{movie(1, "Brothers", "2009-01-01"), movie(1, "Brothers", "2009-01-01")}, DiscoveryMatched, 1},
		{"series filename year not episode title", "The Flash", []string{"The Flash/Season 01/The.Flash.2014.S01E01.Pilot.2020.1080p.mkv"}, true, []*SearchMediaResult{show(1, "The Flash", "1990-01-01"), show(2, "The Flash", "2014-01-01")}, DiscoveryMatched, 1},
		{"series NxM filename", "Breaking Bad", []string{"Breaking.Bad.1x02.1080p.mkv"}, true, []*SearchMediaResult{show(1, "Breaking Bad", "2008-01-01")}, DiscoveryMatched, 0},
		{"episode title alone ignored", "Breaking Bad", []string{"Pilot.mkv"}, true, []*SearchMediaResult{show(1, "Breaking Bad", "2008-01-01")}, DiscoveryMatched, 0},
		{"series substring rejected", "Brothers", []string{"Step.Brothers.S01E01.mkv"}, true, []*SearchMediaResult{show(1, "Brothers", "2020-01-01"), show(2, "Step Brothers", "2020-01-01")}, DiscoveryUnmatched, 0},
		{"series yearless ambiguity", "The Flash", nil, true, []*SearchMediaResult{show(1, "The Flash", "1990-01-01"), show(2, "The Flash", "2014-01-01")}, DiscoveryAmbiguous, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			evidence, err := ParseDiscoveryEvidence(tc.folder, tc.files, tc.tv)
			require.NoError(t, err)
			want := DiscoveryMatch{Status: tc.status}
			if tc.status == DiscoveryMatched {
				want.Candidate = tc.hits[tc.index]
			}
			require.Equal(t, want, MatchDiscoveryCandidate(evidence, tc.hits, tc.tv))
		})
	}
}

func TestDiscoveryOriginalTitlesAcrossSources(t *testing.T) {
	for _, tv := range []bool{false, true} {
		file := "Le.Film.2020.mkv"
		candidate := &SearchMediaResult{ID: ptr.To(42), Title: ptr.To("The Film"), OriginalTitle: ptr.To("Le Film"), ReleaseDate: ptr.To("2020-01-01")}
		if tv {
			file = "Le.Film.2020.S01E01.mkv"
			candidate = &SearchMediaResult{ID: ptr.To(42), Name: ptr.To("The Film"), OriginalName: ptr.To("Le Film"), FirstAirDate: ptr.To("2020-01-01")}
		}
		evidence, err := ParseDiscoveryEvidence("The Film (2020)", []string{file}, tv)
		require.NoError(t, err)
		require.Equal(t, DiscoveryMatch{Status: DiscoveryMatched, Candidate: candidate}, MatchDiscoveryCandidate(evidence, []*SearchMediaResult{candidate}, tv))
	}
}

func TestDiscoveryExplicitIDScopeAndConflicts(t *testing.T) {
	for _, tc := range []struct {
		folder  string
		files   []string
		tv      bool
		want    DiscoveryEvidence
		wantErr bool
	}{
		{"Film {tmdb-42}", []string{"Film {tmdb-42}.mkv"}, false, DiscoveryEvidence{Title: "Film", Query: "Film", TMDBID: 42, names: []string{"Film", "Film"}}, false},
		{"Film {tmdb-42}", []string{"Film {tmdb-43}.mkv"}, false, DiscoveryEvidence{}, true},
		{"Film {tmdb-0}", nil, false, DiscoveryEvidence{}, true},
		{"Film (2019) [1984]", nil, false, DiscoveryEvidence{}, true},
		{"Film {tmdb-9999999999999999999999999}", nil, false, DiscoveryEvidence{}, true},
		{"Show {tmdb-42}", []string{"Show.S01E01.{tmdb-999}.mkv"}, true, DiscoveryEvidence{Title: "Show", Query: "Show", TMDBID: 42, names: []string{"Show", "Show"}}, false},
		{"Show {tmdb-42}", []string{"Show.{tmdb-43}.S01E01.mkv"}, true, DiscoveryEvidence{}, true},
	} {
		evidence, err := ParseDiscoveryEvidence(tc.folder, tc.files, tc.tv)
		if tc.wantErr {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			require.Equal(t, tc.want, evidence)
		}
	}
}
