package manager

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"

	"github.com/kasuboski/mediaz/pkg/library"
	libraryMocks "github.com/kasuboski/mediaz/pkg/library/mocks"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/ptr"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/oapi-codegen/nullable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestReleaseFiltersPreserveInt64Numbers(t *testing.T) {
	const season int64 = math.MaxInt32 + 1
	const episode int64 = math.MaxInt32 + 2
	const runtime int64 = math.MaxInt32 + 3
	profile := []sqlcdb.ListQualityProfileRowsRow{{QualityDefinition: sqlcdb.QualityDefinition{MinSize: 0.9, MaxSize: 1.1}}}
	params := SeriesReleaseFilterParams{Title: "Show", SeasonNumber: season, EpisodeNumber: episode, Runtime: runtime}
	seasonFilter := RejectSeasonReleaseFunc(context.Background(), params, profile, nil)
	episodeFilter := RejectEpisodeReleaseFunc(context.Background(), params, profile, nil)
	release := func(title string) *prowlarr.ReleaseResource {
		return &prowlarr.ReleaseResource{Title: nullable.NewNullableWithValue(title), Size: ptr.To(runtime << 20)}
	}
	assert.False(t, seasonFilter(release(fmt.Sprintf("Show.S%d.1080p", season))))
	assert.False(t, episodeFilter(release(fmt.Sprintf("Show.S%dE%d.1080p", season, episode))))
	assert.False(t, episodeFilter(release(fmt.Sprintf("Show.%dx%d.1080p", season, episode))))
	assert.True(t, seasonFilter(release(fmt.Sprintf("Show.S%dE%d.1080p", season, episode))), "a season search must reject individual episodes")
	assert.True(t, seasonFilter(release("Show.S01.1080p")))
	assert.True(t, episodeFilter(release("Show.S01E02.1080p")))
	assert.True(t, seasonFilter(release("Show.S9223372036854775808.1080p")), "overflow must not match")
	assert.True(t, episodeFilter(release("Show.S9223372036854775808E01.1080p")))
	assert.True(t, rejectSeasonReleaseFunc("Show", 1, release("Show.S10.1080p")), "season matching must not accept a numeric substring")
	assert.False(t, RejectMovieReleaseFunc(context.Background(), ReleaseFilterParams{Title: "Show", Runtime: runtime}, profile, nil)(release("Show")), "movie quality filtering must also preserve runtime")
}

func TestReleaseFiltersDisambiguateDimensionsAndEpisodes(t *testing.T) {
	profile := []sqlcdb.ListQualityProfileRowsRow{{QualityDefinition: sqlcdb.QualityDefinition{MinSize: 0.9, MaxSize: 1.1}}}
	for _, tt := range []struct {
		name, title                         string
		season, episode                     int64
		wantSeasonReject, wantEpisodeReject bool
	}{
		{"1080p season pack", "Show.S01.1920x1080.WEB-DL", 1, 2, false, true},
		{"720p season pack", "Show.S01.1280x720.WEB-DL", 1, 2, false, true},
		{"4k season pack", "Show.S01.3840x2160.WEB-DL", 1, 2, false, true},
		{"dimension is not an ordinal", "Show.S01.1920x1080.WEB-DL", 1920, 1080, true, true},
		{"conventional NxM", "Show.1x02.1920x1080.WEB-DL", 1, 2, true, false},
		{"NxM excludes season pack", "Show.S01.1x02.1920x1080.WEB-DL", 1, 2, true, false},
		{"three digit NxM", "Show.108x123.WEB-DL", 108, 123, true, false},
		{"dimensions before NxM", "Show.1920x1080.1x02.WEB-DL", 1, 2, true, false},
		{"explicit S/E after dimensions", "Show.1920x1080.S01E02.WEB-DL", 1, 2, true, false},
		{"explicit S/E takes priority", "Show.3x04.S01E02.WEB-DL", 1, 2, true, false},
		{"explicit S/E can use dimension numbers", "Show.S1920E1080.WEB-DL", 1920, 1080, true, false},
		{"underscore S/E", "Show_S01E02_1920x1080_WEB-DL", 1, 2, true, false},
		{"large NxM after dimensions", "Show.1920x1080.2147483648x2147483649.WEB-DL", 2147483648, 2147483649, true, false},
		{"large NxM excludes season pack", "Show.S2147483648.2147483648x2147483649.1920x1080.WEB-DL", 2147483648, 2147483649, true, false},
		{"overflowing NxM", "Show.9223372036854775808x01.WEB-DL", 1, 1, true, true},
		{"large NxM wrong episode", "Show.2147483648x2147483649.WEB-DL", 2147483648, 2147483650, true, true},
		{"large S/E after dimensions", "Show.1920x1080.S2147483648E2147483649.WEB-DL", 2147483648, 2147483649, true, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			params := SeriesReleaseFilterParams{Title: "Show", SeasonNumber: tt.season, EpisodeNumber: tt.episode, Runtime: 60}
			release := &prowlarr.ReleaseResource{Title: nullable.NewNullableWithValue(tt.title), Size: ptr.To(int64(60 << 20))}
			assert.Equal(t, tt.wantSeasonReject, RejectSeasonReleaseFunc(context.Background(), params, profile, nil)(release))
			assert.Equal(t, tt.wantEpisodeReject, RejectEpisodeReleaseFunc(context.Background(), params, profile, nil)(release))
		})
	}
}

func TestAddEpisodeFileToLibraryPreservesInt64Season(t *testing.T) {
	ctrl := gomock.NewController(t)
	lib := libraryMocks.NewMockLibrary(ctrl)
	ctx := context.Background()
	const season int64 = math.MaxInt32 + 1
	stop := errors.New("stop after verifying library arguments")
	lib.EXPECT().AddEpisode(ctx, "Show", season, "episode.mkv").Return(library.EpisodeFile{}, stop)
	manager := MediaManager{library: lib}
	require.ErrorIs(t, manager.addEpisodeFileToLibrary(ctx, "Show", season, "episode.mkv", sqlcdb.ListEpisodesRow{}), stop)
}

func TestBuildEpisodeResultPreservesInt64Ordinals(t *testing.T) {
	const season int64 = math.MaxInt32 + 1
	const number int64 = math.MaxInt32 + 2
	row := sqlcdb.ListEpisodesRow{Episode: sqlcdb.Episode{EpisodeNumber: number, Monitored: 1}}
	assert.Equal(t, EpisodeResult{
		SeriesID: 42, SeasonNumber: season, Number: number,
		Title: fmt.Sprintf("Episode %d", number), Monitored: true,
	}, buildEpisodeResult(row, nil, 42, season))
	meta := &sqlcdb.EpisodeMetadatum{TmdbID: 123, Number: number, Title: "Episode"}
	assert.Equal(t, EpisodeResult{
		SeriesID: 42, SeasonNumber: season, Number: number, TMDBID: 123,
		Title: "Episode", Monitored: true,
	}, buildEpisodeResult(row, meta, 42, season))
}
