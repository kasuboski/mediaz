package manager

import (
	"context"
	"testing"

	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/ptr"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/oapi-codegen/nullable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeReleaseIdentity(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"L'été: Spider-Man", "lété spider man"},
		{"Amélie (東京)", "amélie 東京"},
		{"AME\u0301LIE", "amélie"},
		{"Q\u0301.2", "q\u0301 2"},
		{"कहानी", "कहानी"},
		{"1\u20dd", "1\u20dd"},
		{"2001: A Space Odyssey", "2001 a space odyssey"},
		{"Ocean’s.Eleven", "oceans eleven"},
		{"  Breaking_Bad-- ", "breaking bad"},
	} {
		assert.Equal(t, tc.want, normalizeReleaseIdentity(tc.in))
	}
}

func TestMovieReleaseTitleMatches(t *testing.T) {
	for _, tc := range []struct {
		name, title, release string
		year                 *int32
		want                 bool
	}{
		{"exact", "Brothers", "Brothers", nil, true},
		{"dots", "Brothers", "Brothers.2024.1080p.WEB-DL.x264-GROUP", ptr.To(int32(2024)), true},
		{"longer title", "Brothers", "Brothers Resolution 2024 1080p", nil, false},
		{"word prefix", "Brothers", "Brothersome 2024 1080p", nil, false},
		{"leading words", "Brothers", "Step Brothers 2024 1080p", nil, false},
		{"leading article matters", "Brothers", "The Brothers 2024 1080p", nil, false},
		{"unknown words before year", "Brothers", "Brothers UnknownTag 2024 1080p", nil, false},
		{"unknown words before quality", "Brothers", "Brothers UnknownTag WEB-DL", nil, false},
		{"unknown words after quality", "Brothers", "Brothers 1080p Resolution 2024", nil, false},
		{"known year after quality conflicts", "Brothers", "Brothers 1080p 2009 BluRay", ptr.To(int32(2024)), false},
		{"yearless source and audio", "Brothers", "Brothers AMZN WEB-DL DDP5.1 x264-GROUP", nil, true},
		{"wrong year", "Brothers", "Brothers (2009) 1080p", ptr.To(int32(2024)), false},
		{"no year", "Brothers", "Brothers 1080p BluRay x264-GROUP", ptr.To(int32(2024)), true},
		{"edition", "Brothers", "Brothers {edition-Extended Edition} (2024) [Bluray-1080p]", ptr.To(int32(2024)), true},
		{"edition unknown words", "Brothers", "Brothers Extended Sequel 2024", nil, false},
		{"apostrophe", "Ocean's Eleven", "Oceans.Eleven.2001.1080p", ptr.To(int32(2001)), true},
		{"punctuation", "Mission: Impossible", "Mission.Impossible.1996.1080p", ptr.To(int32(1996)), true},
		{"Unicode", "Amélie", "AMÉLIE.2001.1080p", ptr.To(int32(2001)), true},
		{"decomposed release", "Amélie", "AME\u0301LIE.2001.1080p", ptr.To(int32(2001)), true},
		{"decomposed title", "Ame\u0301lie", "AMÉLIE.2001.1080p", ptr.To(int32(2001)), true},
		{"combining mark distinguishes titles", "Q\u0301", "Q.2024.1080p", nil, false},
		{"Unicode words differ", "東京", "京都.2024.1080p", nil, false},
		{"numeric title 1917", "1917", "1917.2019.1080p", ptr.To(int32(2019)), true},
		{"numeric title 2012", "2012", "2012.2009.BluRay", ptr.To(int32(2009)), true},
		{"numeric title no year", "1917", "1917.1080p.BluRay", ptr.To(int32(2019)), true},
		{"year in title", "2001: A Space Odyssey", "2001.A.Space.Odyssey.1968.1080p", ptr.To(int32(1968)), true},
		{"title digits preserved", "John Wick", "John Wick 2 2017 1080p", nil, false},
		{"empty expected", "", "Brothers 2024", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, movieReleaseTitleMatches(tc.release, ReleaseFilterParams{Title: tc.title, Year: tc.year}))
		})
	}
}

func TestMovieReleaseIdentityFilter(t *testing.T) {
	params := ReleaseFilterParams{Title: "Brothers", OriginalTitle: ptr.To("Frères"), CleanTitle: ptr.To("Alternate Title"), Year: ptr.To(int32(2024)), Runtime: 60, TmdbID: 42, IMDbID: "tt09860566"}
	profile := []sqlcdb.ListQualityProfileRowsRow{{QualityDefinition: sqlcdb.QualityDefinition{MinSize: 0, MaxSize: 10000}}}
	for _, tc := range []struct {
		name, title string
		tmdb, imdb  *int32
		wantReject  bool
	}{
		{"matching TMDB overrides title and year", "Un autre titre 2009 1080p", ptr.To(int32(42)), nil, false},
		{"matching IMDb overrides title and year", "Un autre titre 2009 1080p", nil, ptr.To(int32(9860566)), false},
		{"TMDB conflict overrides title", "Brothers 2024 1080p", ptr.To(int32(43)), nil, true},
		{"IMDb conflict overrides title", "Brothers 2024 1080p", nil, ptr.To(int32(765010)), true},
		{"one conflict overrides other match", "Brothers 2024 1080p", ptr.To(int32(42)), ptr.To(int32(765010)), true},
		{"other conflict overrides match", "Brothers 2024 1080p", ptr.To(int32(43)), ptr.To(int32(9860566)), true},
		{"zero IDs fall back", "Brothers 2024 1080p", ptr.To(int32(0)), ptr.To(int32(0)), false},
		{"missing IDs fall back", "Brothers 2024 1080p", nil, nil, false},
		{"original title", "Frères.2024.1080p", nil, nil, false},
		{"clean alias", "Alternate.Title.2024.1080p", nil, nil, false},
		{"year veto in fallback", "Brothers 2009 1080p", nil, nil, true},
		{"title suffix collision", "Brothers Resolution 2024 1080p", nil, nil, true},
		{"empty title even with ID", " ", ptr.To(int32(42)), nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &prowlarr.ReleaseResource{Title: nullable.NewNullableWithValue(tc.title), TmdbID: tc.tmdb, ImdbID: tc.imdb, Size: ptr.To(int64(500 << 20))}
			assert.Equal(t, tc.wantReject, RejectMovieReleaseFunc(context.Background(), params, profile, nil)(r))
		})
	}
	for _, tc := range []struct {
		name       string
		size       *int64
		protocol   *prowlarr.DownloadProtocol
		filename   nullable.Nullable[string]
		wantReject bool
	}{
		{"missing size", nil, nil, nil, true},
		{"negative size", ptr.To(int64(-1)), nil, nil, true},
		{"oversize", ptr.To(int64(1 << 50)), nil, nil, true},
		{"unavailable protocol", ptr.To(int64(500 << 20)), ptr.To(prowlarr.DownloadProtocolTorrent), nil, true},
		{"null filename", ptr.To(int64(500 << 20)), nil, nullable.NewNullNullable[string](), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &prowlarr.ReleaseResource{Title: nullable.NewNullableWithValue("Different title"), TmdbID: ptr.To(int32(42)), Size: tc.size, Protocol: tc.protocol, FileName: tc.filename}
			assert.Equal(t, tc.wantReject, RejectMovieReleaseFunc(context.Background(), params, profile, nil)(r))
		})
	}
}

func TestCompareReleaseIDs(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		release                 *prowlarr.ReleaseResource
		tmdb                    int64
		imdb                    string
		tvdb                    int64
		wantMatch, wantConflict bool
	}{
		{"nil", nil, 42, "tt123", 5, false, false},
		{"TMDB", &prowlarr.ReleaseResource{TmdbID: ptr.To(int32(42))}, 42, "", 0, true, false},
		{"IMDb leading zeros", &prowlarr.ReleaseResource{ImdbID: ptr.To(int32(123))}, 0, "tt0000123", 0, true, false},
		{"numeric IMDb", &prowlarr.ReleaseResource{ImdbID: ptr.To(int32(123))}, 0, "123", 0, true, false},
		{"TVDB", &prowlarr.ReleaseResource{TvdbID: ptr.To(int32(5))}, 0, "", 5, true, false},
		{"conflicting namespaces", &prowlarr.ReleaseResource{TmdbID: ptr.To(int32(42)), ImdbID: ptr.To(int32(124))}, 42, "tt123", 0, true, true},
		{"no expected ID", &prowlarr.ReleaseResource{TmdbID: ptr.To(int32(42)), ImdbID: ptr.To(int32(123))}, 0, "", 0, false, false},
		{"invalid IMDb", &prowlarr.ReleaseResource{ImdbID: ptr.To(int32(123))}, 0, "tt+123", 0, false, false},
		{"overflow IMDb", &prowlarr.ReleaseResource{ImdbID: ptr.To(int32(123))}, 0, "tt99999999999999999999999", 0, false, false},
		{"zero unavailable", &prowlarr.ReleaseResource{TmdbID: ptr.To(int32(0)), ImdbID: ptr.To(int32(0))}, 42, "tt123", 0, false, false},
		{"negative unavailable", &prowlarr.ReleaseResource{TmdbID: ptr.To(int32(-1))}, 42, "", 0, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, c := compareReleaseIDs(tc.release, tc.tmdb, tc.imdb, tc.tvdb)
			require.Equal(t, tc.wantMatch, m)
			require.Equal(t, tc.wantConflict, c)
		})
	}
}
