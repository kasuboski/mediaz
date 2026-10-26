package manager

import (
	"database/sql"
	"testing"

	"github.com/kasuboski/mediaz/pkg/ptr"
	"github.com/kasuboski/mediaz/pkg/storage"
	sqlcdb "github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_getSeasonRuntime(t *testing.T) {
	tests := []struct {
		name                string
		episodeMetadata     []*sqlcdb.EpisodeMetadatum
		totalSeasonEpisodes int
		want                int64
	}{
		{
			name: "all episodes have runtime",
			episodeMetadata: []*sqlcdb.EpisodeMetadatum{
				{Runtime: sql.NullInt64{Int64: 30, Valid: true}},
				{Runtime: sql.NullInt64{Int64: 30, Valid: true}},
				{Runtime: sql.NullInt64{Int64: 30, Valid: true}},
			},
			totalSeasonEpisodes: 3,
			want:                90,
		},
		{
			name: "some episodes missing runtime",
			episodeMetadata: []*sqlcdb.EpisodeMetadatum{
				{Runtime: sql.NullInt64{Int64: 30, Valid: true}},
				{Runtime: sql.NullInt64{}},
				{Runtime: sql.NullInt64{Int64: 30, Valid: true}},
			},
			totalSeasonEpisodes: 3,
			want:                90, // Average of 30 mins applied to missing episode
		},
		{
			name: "all episodes missing runtime",
			episodeMetadata: []*sqlcdb.EpisodeMetadatum{
				{Runtime: sql.NullInt64{}},
				{Runtime: sql.NullInt64{}},
				{Runtime: sql.NullInt64{}},
			},
			totalSeasonEpisodes: 3,
			want:                0,
		},
		{
			name:                "empty episode list",
			episodeMetadata:     []*sqlcdb.EpisodeMetadatum{},
			totalSeasonEpisodes: 0,
			want:                0,
		},
		{
			name: "more total episodes than provided",
			episodeMetadata: []*sqlcdb.EpisodeMetadatum{
				{Runtime: sql.NullInt64{Int64: 30, Valid: true}},
				{Runtime: sql.NullInt64{Int64: 30, Valid: true}},
			},
			totalSeasonEpisodes: 4,
			want:                120, // (30+30) + (30*2) for missing episodes
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getSeasonRuntime(tt.episodeMetadata, tt.totalSeasonEpisodes)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDetermineSeasonState(t *testing.T) {
	tests := []struct {
		name     string
		episodes []sqlcdb.ListEpisodesRow
		expected storage.SeasonState
	}{
		{
			name:     "empty episodes",
			episodes: []sqlcdb.ListEpisodesRow{},
			expected: storage.SeasonStateMissing,
		},
		{
			name: "all episodes downloaded",
			episodes: []sqlcdb.ListEpisodesRow{
				{Episode: sqlcdb.Episode{ID: 1}, State: string(storage.EpisodeStateDownloaded)},
				{Episode: sqlcdb.Episode{ID: 2}, State: string(storage.EpisodeStateDownloaded)},
				{Episode: sqlcdb.Episode{ID: 3}, State: string(storage.EpisodeStateDownloaded)},
			},
			expected: storage.SeasonStateCompleted,
		},
		{
			name: "some episodes downloading",
			episodes: []sqlcdb.ListEpisodesRow{
				{Episode: sqlcdb.Episode{ID: 1}, State: string(storage.EpisodeStateDownloaded)},
				{Episode: sqlcdb.Episode{ID: 2}, State: string(storage.EpisodeStateDownloading)},
				{Episode: sqlcdb.Episode{ID: 3}, State: string(storage.EpisodeStateMissing)},
			},
			expected: storage.SeasonStateDownloading,
		},
		{
			name: "one episode downloading others downloaded",
			episodes: []sqlcdb.ListEpisodesRow{
				{Episode: sqlcdb.Episode{ID: 1}, State: string(storage.EpisodeStateDownloaded)},
				{Episode: sqlcdb.Episode{ID: 2}, State: string(storage.EpisodeStateDownloaded)},
				{Episode: sqlcdb.Episode{ID: 3}, State: string(storage.EpisodeStateDownloading)},
			},
			expected: storage.SeasonStateDownloading,
		},
		{
			name: "continuing - downloaded and unreleased",
			episodes: []sqlcdb.ListEpisodesRow{
				{Episode: sqlcdb.Episode{ID: 1}, State: string(storage.EpisodeStateDownloaded)},
				{Episode: sqlcdb.Episode{ID: 2}, State: string(storage.EpisodeStateDownloaded)},
				{Episode: sqlcdb.Episode{ID: 3}, State: string(storage.EpisodeStateUnreleased)},
				{Episode: sqlcdb.Episode{ID: 4}, State: string(storage.EpisodeStateUnreleased)},
			},
			expected: storage.SeasonStateContinuing,
		},
		{
			name: "continuing - missing and unreleased",
			episodes: []sqlcdb.ListEpisodesRow{
				{Episode: sqlcdb.Episode{ID: 1}, State: string(storage.EpisodeStateMissing)},
				{Episode: sqlcdb.Episode{ID: 2}, State: string(storage.EpisodeStateMissing)},
				{Episode: sqlcdb.Episode{ID: 3}, State: string(storage.EpisodeStateUnreleased)},
				{Episode: sqlcdb.Episode{ID: 4}, State: string(storage.EpisodeStateUnreleased)},
			},
			expected: storage.SeasonStateContinuing,
		},
		{
			name: "continuing - mix of downloaded, missing and unreleased",
			episodes: []sqlcdb.ListEpisodesRow{
				{Episode: sqlcdb.Episode{ID: 1}, State: string(storage.EpisodeStateDownloaded)},
				{Episode: sqlcdb.Episode{ID: 2}, State: string(storage.EpisodeStateMissing)},
				{Episode: sqlcdb.Episode{ID: 3}, State: string(storage.EpisodeStateUnreleased)},
			},
			expected: storage.SeasonStateContinuing,
		},
		{
			name: "missing - all aired episodes missing, no unreleased",
			episodes: []sqlcdb.ListEpisodesRow{
				{Episode: sqlcdb.Episode{ID: 1}, State: string(storage.EpisodeStateMissing)},
				{Episode: sqlcdb.Episode{ID: 2}, State: string(storage.EpisodeStateMissing)},
				{Episode: sqlcdb.Episode{ID: 3}, State: string(storage.EpisodeStateMissing)},
			},
			expected: storage.SeasonStateMissing,
		},
		{
			name: "missing - mix of downloaded and missing, no unreleased",
			episodes: []sqlcdb.ListEpisodesRow{
				{Episode: sqlcdb.Episode{ID: 1}, State: string(storage.EpisodeStateDownloaded)},
				{Episode: sqlcdb.Episode{ID: 2}, State: string(storage.EpisodeStateMissing)},
				{Episode: sqlcdb.Episode{ID: 3}, State: string(storage.EpisodeStateMissing)},
			},
			expected: storage.SeasonStateMissing,
		},
		{
			name: "unreleased - all episodes unreleased",
			episodes: []sqlcdb.ListEpisodesRow{
				{Episode: sqlcdb.Episode{ID: 1}, State: string(storage.EpisodeStateUnreleased)},
				{Episode: sqlcdb.Episode{ID: 2}, State: string(storage.EpisodeStateUnreleased)},
				{Episode: sqlcdb.Episode{ID: 3}, State: string(storage.EpisodeStateUnreleased)},
			},
			expected: storage.SeasonStateUnreleased,
		},
		{
			name: "downloading takes priority over continuing",
			episodes: []sqlcdb.ListEpisodesRow{
				{Episode: sqlcdb.Episode{ID: 1}, State: string(storage.EpisodeStateDownloaded)},
				{Episode: sqlcdb.Episode{ID: 2}, State: string(storage.EpisodeStateDownloading)},
				{Episode: sqlcdb.Episode{ID: 3}, State: string(storage.EpisodeStateUnreleased)},
			},
			expected: storage.SeasonStateDownloading,
		},
		{
			name: "downloading takes priority over all other states",
			episodes: []sqlcdb.ListEpisodesRow{
				{Episode: sqlcdb.Episode{ID: 1}, State: string(storage.EpisodeStateDownloaded)},
				{Episode: sqlcdb.Episode{ID: 2}, State: string(storage.EpisodeStateDownloading)},
				{Episode: sqlcdb.Episode{ID: 3}, State: string(storage.EpisodeStateMissing)},
				{Episode: sqlcdb.Episode{ID: 4}, State: string(storage.EpisodeStateUnreleased)},
			},
			expected: storage.SeasonStateDownloading,
		},
		{
			name: "single downloaded episode",
			episodes: []sqlcdb.ListEpisodesRow{
				{Episode: sqlcdb.Episode{ID: 1}, State: string(storage.EpisodeStateDownloaded)},
			},
			expected: storage.SeasonStateCompleted,
		},
		{
			name: "single missing episode",
			episodes: []sqlcdb.ListEpisodesRow{
				{Episode: sqlcdb.Episode{ID: 1}, State: string(storage.EpisodeStateMissing)},
			},
			expected: storage.SeasonStateMissing,
		},
		{
			name: "single unreleased episode",
			episodes: []sqlcdb.ListEpisodesRow{
				{Episode: sqlcdb.Episode{ID: 1}, State: string(storage.EpisodeStateUnreleased)},
			},
			expected: storage.SeasonStateUnreleased,
		},
		{
			name: "single downloading episode",
			episodes: []sqlcdb.ListEpisodesRow{
				{Episode: sqlcdb.Episode{ID: 1}, State: string(storage.EpisodeStateDownloading)},
			},
			expected: storage.SeasonStateDownloading,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, result := determineSeasonState(tt.episodes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDetermineSeasonStateWithCounts(t *testing.T) {
	tests := []struct {
		name           string
		episodes       []sqlcdb.ListEpisodesRow
		expectedState  storage.SeasonState
		expectedCounts map[string]int
	}{
		{
			name:          "empty episodes",
			episodes:      []sqlcdb.ListEpisodesRow{},
			expectedState: storage.SeasonStateMissing,
			expectedCounts: map[string]int{
				"done":        0,
				"downloading": 0,
				"missing":     0,
				"unreleased":  0,
				"discovered":  0,
			},
		},
		{
			name: "mix of all states",
			episodes: []sqlcdb.ListEpisodesRow{
				{Episode: sqlcdb.Episode{ID: 1}, State: string(storage.EpisodeStateDownloaded)},
				{Episode: sqlcdb.Episode{ID: 2}, State: string(storage.EpisodeStateDownloaded)},
				{Episode: sqlcdb.Episode{ID: 3}, State: string(storage.EpisodeStateDownloading)},
				{Episode: sqlcdb.Episode{ID: 4}, State: string(storage.EpisodeStateMissing)},
				{Episode: sqlcdb.Episode{ID: 5}, State: string(storage.EpisodeStateMissing)},
				{Episode: sqlcdb.Episode{ID: 6}, State: string(storage.EpisodeStateUnreleased)},
			},
			expectedState: storage.SeasonStateDownloading,
			expectedCounts: map[string]int{
				"done":        2,
				"downloading": 1,
				"missing":     2,
				"unreleased":  1,
				"discovered":  0,
			},
		},
		{
			name: "discovered with completed episodes",
			episodes: []sqlcdb.ListEpisodesRow{
				{Episode: sqlcdb.Episode{ID: 1}, State: string(storage.EpisodeStateDiscovered)},
				{Episode: sqlcdb.Episode{ID: 2}, State: string(storage.EpisodeStateCompleted)},
				{Episode: sqlcdb.Episode{ID: 3}, State: string(storage.EpisodeStateDiscovered)},
			},
			expectedState: storage.SeasonStateContinuing,
			expectedCounts: map[string]int{
				"done":        1,
				"downloading": 0,
				"missing":     0,
				"unreleased":  0,
				"discovered":  2,
			},
		},
		{
			name: "all discovered episodes",
			episodes: []sqlcdb.ListEpisodesRow{
				{Episode: sqlcdb.Episode{ID: 1}, State: string(storage.EpisodeStateDiscovered)},
				{Episode: sqlcdb.Episode{ID: 2}, State: string(storage.EpisodeStateDiscovered)},
			},
			expectedState: storage.SeasonStateDiscovered,
			expectedCounts: map[string]int{
				"done":        0,
				"downloading": 0,
				"missing":     0,
				"unreleased":  0,
				"discovered":  2,
			},
		},
		{
			name: "continuing season counts",
			episodes: []sqlcdb.ListEpisodesRow{
				{Episode: sqlcdb.Episode{ID: 1}, State: string(storage.EpisodeStateDownloaded)},
				{Episode: sqlcdb.Episode{ID: 2}, State: string(storage.EpisodeStateMissing)},
				{Episode: sqlcdb.Episode{ID: 3}, State: string(storage.EpisodeStateUnreleased)},
				{Episode: sqlcdb.Episode{ID: 4}, State: string(storage.EpisodeStateUnreleased)},
			},
			expectedState: storage.SeasonStateContinuing,
			expectedCounts: map[string]int{
				"done":        1,
				"downloading": 0,
				"missing":     1,
				"unreleased":  2,
				"discovered":  0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			counts, state := determineSeasonState(tt.episodes)
			assert.Equal(t, tt.expectedState, state)
			assert.Equal(t, tt.expectedCounts, counts)
		})
	}
}

func Test_findMatchingSeriesResult(t *testing.T) {
	air2024 := "2024-01-15"
	air2004 := "2004-09-22"

	tests := []struct {
		name     string
		year     *int32
		results  []*SearchMediaResult
		expected *SearchMediaResult
	}{
		{
			name: "no year - returns first result",
			year: nil,
			results: []*SearchMediaResult{
				{ID: ptr.To(1), Name: ptr.To("Battlestar Galactica"), FirstAirDate: &air2004},
				{ID: ptr.To(2), Name: ptr.To("Battlestar Galactica"), FirstAirDate: &air2024},
			},
			expected: &SearchMediaResult{ID: ptr.To(1), Name: ptr.To("Battlestar Galactica"), FirstAirDate: &air2004},
		},
		{
			name: "year matches first result",
			year: ptr.To(int32(2004)),
			results: []*SearchMediaResult{
				{ID: ptr.To(1), Name: ptr.To("Battlestar Galactica"), FirstAirDate: &air2004},
				{ID: ptr.To(2), Name: ptr.To("Battlestar Galactica"), FirstAirDate: &air2024},
			},
			expected: &SearchMediaResult{ID: ptr.To(1), Name: ptr.To("Battlestar Galactica"), FirstAirDate: &air2004},
		},
		{
			name: "year matches second result",
			year: ptr.To(int32(2024)),
			results: []*SearchMediaResult{
				{ID: ptr.To(1), Name: ptr.To("Battlestar Galactica"), FirstAirDate: &air2004},
				{ID: ptr.To(2), Name: ptr.To("Battlestar Galactica"), FirstAirDate: &air2024},
			},
			expected: &SearchMediaResult{ID: ptr.To(2), Name: ptr.To("Battlestar Galactica"), FirstAirDate: &air2024},
		},
		{
			name: "year not found in results",
			year: ptr.To(int32(2025)),
			results: []*SearchMediaResult{
				{ID: ptr.To(1), Name: ptr.To("Battlestar Galactica"), FirstAirDate: &air2004},
				{ID: ptr.To(2), Name: ptr.To("Battlestar Galactica"), FirstAirDate: &air2024},
			},
			expected: nil,
		},
		{
			name: "result with nil air date",
			year: ptr.To(int32(2024)),
			results: []*SearchMediaResult{
				{ID: ptr.To(1), Name: ptr.To("Show"), FirstAirDate: nil},
				{ID: ptr.To(2), Name: ptr.To("Show"), FirstAirDate: &air2024},
			},
			expected: &SearchMediaResult{ID: ptr.To(2), Name: ptr.To("Show"), FirstAirDate: &air2024},
		},
		{
			name: "all results have nil air dates",
			year: ptr.To(int32(2024)),
			results: []*SearchMediaResult{
				{ID: ptr.To(1), Name: ptr.To("Show"), FirstAirDate: nil},
				{ID: ptr.To(2), Name: ptr.To("Show"), FirstAirDate: nil},
			},
			expected: nil,
		},
		{
			name:     "empty results",
			year:     ptr.To(int32(2024)),
			results:  []*SearchMediaResult{},
			expected: nil,
		},
		{
			name:     "nil results",
			year:     ptr.To(int32(2024)),
			results:  nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findMatchingSeriesResult(tt.results, tt.year)
			if tt.expected == nil {
				assert.Nil(t, result)
				return
			}
			require.NotNil(t, result)
			assert.Equal(t, tt.expected, result)
		})
	}
}
