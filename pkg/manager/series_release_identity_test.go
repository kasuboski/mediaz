package manager

import (
	"context"
	"testing"

	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/ptr"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/oapi-codegen/nullable"
	"github.com/stretchr/testify/assert"
)

func TestSeriesReleaseIdentity(t *testing.T) {
	profile := []sqlcdb.ListQualityProfileRowsRow{{QualityDefinition: sqlcdb.QualityDefinition{MinSize: 0, MaxSize: 10000}}}
	for _, tc := range []struct {
		name, title, release string
		tmdb, imdb, tvdb     *int32
		pack, wantReject     bool
	}{
		{"episode dotted words", "Breaking Bad", "Breaking.Bad.S01E02.1080p", nil, nil, nil, false, false},
		{"pack dotted words", "Breaking Bad", "Breaking.Bad.S01.1080p", nil, nil, nil, true, false},
		{"pack underscores", "Breaking Bad", "Breaking_Bad_Season_01_Complete_720p", nil, nil, nil, true, false},
		{"episode title prefix collision", "Brothers", "Step.Brothers.S01E02.1080p", nil, nil, nil, false, true},
		{"episode title suffix collision", "Brothers", "Brothers.Resolution.S01E02.1080p", nil, nil, nil, false, true},
		{"pack title prefix collision", "Brothers", "Step.Brothers.S01.1080p", nil, nil, nil, true, true},
		{"pack title suffix collision", "Brothers", "Brothers.Resolution.S01.1080p", nil, nil, nil, true, true},
		{"episode apostrophe", "Schitt's Creek", "Schitts.Creek.S01E02.1080p", nil, nil, nil, false, false},
		{"episode year", "The Flash", "The.Flash.2014.S01E02.1080p", nil, nil, nil, false, false},
		{"episode wrong year", "The Flash", "The.Flash.1990.S01E02.1080p", nil, nil, nil, false, true},
		{"pack wrong year", "The Flash", "The.Flash.1990.S01.1080p", nil, nil, nil, true, true},
		{"ID overrides year", "The Flash", "The.Flash.1990.S01E02.1080p", ptr.To(int32(42)), nil, nil, false, false},
		{"numeric title", "1899", "1899.S01E02.1080p", nil, nil, nil, false, false},
		{"TMDB match episode", "Breaking Bad", "Translated.Title.S01E02.1080p", ptr.To(int32(42)), nil, nil, false, false},
		{"TMDB match pack", "Breaking Bad", "Translated.Title.S01.1080p", ptr.To(int32(42)), nil, nil, true, false},
		{"IMDb match", "Breaking Bad", "Translated.Title.S01E02.1080p", nil, ptr.To(int32(123)), nil, false, false},
		{"TVDB match", "Breaking Bad", "Translated.Title.S01E02.1080p", nil, nil, ptr.To(int32(5)), false, false},
		{"TMDB conflict", "Breaking Bad", "Breaking.Bad.S01E02.1080p", ptr.To(int32(43)), nil, nil, false, true},
		{"TVDB conflict overrides match", "Breaking Bad", "Breaking.Bad.S01E02.1080p", ptr.To(int32(42)), nil, ptr.To(int32(6)), false, true},
		{"matching ID wrong season", "Breaking Bad", "Other.S02E02.1080p", ptr.To(int32(42)), nil, nil, false, true},
		{"matching ID wrong episode", "Breaking Bad", "Other.S01E03.1080p", ptr.To(int32(42)), nil, nil, false, true},
		{"matching ID pack wrong season", "Breaking Bad", "Other.S02.1080p", ptr.To(int32(42)), nil, nil, true, true},
		{"matching ID episode not pack", "Breaking Bad", "Other.S01E02.1080p", ptr.To(int32(42)), nil, nil, true, true},
		{"matching ID pack not episode", "Breaking Bad", "Other.S01.1080p", ptr.To(int32(42)), nil, nil, false, true},
		{"matching ID still needs marker", "Breaking Bad", "Other.1080p", ptr.To(int32(42)), nil, nil, false, true},
		{"matching ID empty title", "Breaking Bad", "", ptr.To(int32(42)), nil, nil, false, true},
		{"zero IDs fallback", "Breaking Bad", "Breaking.Bad.S01E02.1080p", ptr.To(int32(0)), ptr.To(int32(0)), ptr.To(int32(0)), false, false},
		{"malformed season marker", "Breaking Bad", "Breaking.Bad.S01junk.1080p", nil, nil, nil, true, true},
		{"no title", "", "Other.S01E02.1080p", nil, nil, nil, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			params := SeriesReleaseFilterParams{Title: tc.title, SeasonNumber: 1, EpisodeNumber: 2, Runtime: 60, TmdbID: 42, IMDbID: "tt0000123", TVDBID: 5, Year: ptr.To(int32(2014))}
			r := &prowlarr.ReleaseResource{Title: nullable.NewNullableWithValue(tc.release), TmdbID: tc.tmdb, ImdbID: tc.imdb, TvdbID: tc.tvdb, Size: ptr.To(int64(500 << 20))}
			filter := RejectEpisodeReleaseFunc(context.Background(), params, profile, nil)
			if tc.pack {
				filter = RejectSeasonReleaseFunc(context.Background(), params, profile, nil)
			}
			assert.Equal(t, tc.wantReject, filter(r))
		})
	}
}
