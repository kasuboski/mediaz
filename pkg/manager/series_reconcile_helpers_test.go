package manager

import (
	"testing"

	"github.com/kasuboski/mediaz/pkg/ptr"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_getSeasonRuntime(t *testing.T) {
	tests := []struct {
		name                string
		episodeMetadata     []*model.EpisodeMetadata
		totalSeasonEpisodes int
		want                int32
	}{
		{
			name: "all episodes have runtime",
			episodeMetadata: []*model.EpisodeMetadata{
				{Runtime: ptr.To(int32(30))},
				{Runtime: ptr.To(int32(30))},
				{Runtime: ptr.To(int32(30))},
			},
			totalSeasonEpisodes: 3,
			want:                90,
		},
		{
			name: "some episodes missing runtime",
			episodeMetadata: []*model.EpisodeMetadata{
				{Runtime: ptr.To(int32(30))},
				{Runtime: nil},
				{Runtime: ptr.To(int32(30))},
			},
			totalSeasonEpisodes: 3,
			want:                90, // Average of 30 mins applied to missing episode
		},
		{
			name: "all episodes missing runtime",
			episodeMetadata: []*model.EpisodeMetadata{
				{Runtime: nil},
				{Runtime: nil},
				{Runtime: nil},
			},
			totalSeasonEpisodes: 3,
			want:                0,
		},
		{
			name:                "empty episode list",
			episodeMetadata:     []*model.EpisodeMetadata{},
			totalSeasonEpisodes: 0,
			want:                0,
		},
		{
			name: "more total episodes than provided",
			episodeMetadata: []*model.EpisodeMetadata{
				{Runtime: ptr.To(int32(30))},
				{Runtime: ptr.To(int32(30))},
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
		episodes []*storage.Episode
		expected storage.SeasonState
	}{
		{
			name:     "empty episodes",
			episodes: []*storage.Episode{},
			expected: storage.SeasonStateMissing,
		},
		{
			name: "all episodes downloaded",
			episodes: []*storage.Episode{
				{Episode: model.Episode{ID: 1}, State: storage.EpisodeStateDownloaded},
				{Episode: model.Episode{ID: 2}, State: storage.EpisodeStateDownloaded},
				{Episode: model.Episode{ID: 3}, State: storage.EpisodeStateDownloaded},
			},
			expected: storage.SeasonStateCompleted,
		},
		{
			name: "some episodes downloading",
			episodes: []*storage.Episode{
				{Episode: model.Episode{ID: 1}, State: storage.EpisodeStateDownloaded},
				{Episode: model.Episode{ID: 2}, State: storage.EpisodeStateDownloading},
				{Episode: model.Episode{ID: 3}, State: storage.EpisodeStateMissing},
			},
			expected: storage.SeasonStateDownloading,
		},
		{
			name: "one episode downloading others downloaded",
			episodes: []*storage.Episode{
				{Episode: model.Episode{ID: 1}, State: storage.EpisodeStateDownloaded},
				{Episode: model.Episode{ID: 2}, State: storage.EpisodeStateDownloaded},
				{Episode: model.Episode{ID: 3}, State: storage.EpisodeStateDownloading},
			},
			expected: storage.SeasonStateDownloading,
		},
		{
			name: "continuing - downloaded and unreleased",
			episodes: []*storage.Episode{
				{Episode: model.Episode{ID: 1}, State: storage.EpisodeStateDownloaded},
				{Episode: model.Episode{ID: 2}, State: storage.EpisodeStateDownloaded},
				{Episode: model.Episode{ID: 3}, State: storage.EpisodeStateUnreleased},
				{Episode: model.Episode{ID: 4}, State: storage.EpisodeStateUnreleased},
			},
			expected: storage.SeasonStateContinuing,
		},
		{
			name: "continuing - missing and unreleased",
			episodes: []*storage.Episode{
				{Episode: model.Episode{ID: 1}, State: storage.EpisodeStateMissing},
				{Episode: model.Episode{ID: 2}, State: storage.EpisodeStateMissing},
				{Episode: model.Episode{ID: 3}, State: storage.EpisodeStateUnreleased},
				{Episode: model.Episode{ID: 4}, State: storage.EpisodeStateUnreleased},
			},
			expected: storage.SeasonStateContinuing,
		},
		{
			name: "continuing - mix of downloaded, missing and unreleased",
			episodes: []*storage.Episode{
				{Episode: model.Episode{ID: 1}, State: storage.EpisodeStateDownloaded},
				{Episode: model.Episode{ID: 2}, State: storage.EpisodeStateMissing},
				{Episode: model.Episode{ID: 3}, State: storage.EpisodeStateUnreleased},
			},
			expected: storage.SeasonStateContinuing,
		},
		{
			name: "missing - all aired episodes missing, no unreleased",
			episodes: []*storage.Episode{
				{Episode: model.Episode{ID: 1}, State: storage.EpisodeStateMissing},
				{Episode: model.Episode{ID: 2}, State: storage.EpisodeStateMissing},
				{Episode: model.Episode{ID: 3}, State: storage.EpisodeStateMissing},
			},
			expected: storage.SeasonStateMissing,
		},
		{
			name: "missing - mix of downloaded and missing, no unreleased",
			episodes: []*storage.Episode{
				{Episode: model.Episode{ID: 1}, State: storage.EpisodeStateDownloaded},
				{Episode: model.Episode{ID: 2}, State: storage.EpisodeStateMissing},
				{Episode: model.Episode{ID: 3}, State: storage.EpisodeStateMissing},
			},
			expected: storage.SeasonStateMissing,
		},
		{
			name: "unreleased - all episodes unreleased",
			episodes: []*storage.Episode{
				{Episode: model.Episode{ID: 1}, State: storage.EpisodeStateUnreleased},
				{Episode: model.Episode{ID: 2}, State: storage.EpisodeStateUnreleased},
				{Episode: model.Episode{ID: 3}, State: storage.EpisodeStateUnreleased},
			},
			expected: storage.SeasonStateUnreleased,
		},
		{
			name: "downloading takes priority over continuing",
			episodes: []*storage.Episode{
				{Episode: model.Episode{ID: 1}, State: storage.EpisodeStateDownloaded},
				{Episode: model.Episode{ID: 2}, State: storage.EpisodeStateDownloading},
				{Episode: model.Episode{ID: 3}, State: storage.EpisodeStateUnreleased},
			},
			expected: storage.SeasonStateDownloading,
		},
		{
			name: "downloading takes priority over all other states",
			episodes: []*storage.Episode{
				{Episode: model.Episode{ID: 1}, State: storage.EpisodeStateDownloaded},
				{Episode: model.Episode{ID: 2}, State: storage.EpisodeStateDownloading},
				{Episode: model.Episode{ID: 3}, State: storage.EpisodeStateMissing},
				{Episode: model.Episode{ID: 4}, State: storage.EpisodeStateUnreleased},
			},
			expected: storage.SeasonStateDownloading,
		},
		{
			name: "single downloaded episode",
			episodes: []*storage.Episode{
				{Episode: model.Episode{ID: 1}, State: storage.EpisodeStateDownloaded},
			},
			expected: storage.SeasonStateCompleted,
		},
		{
			name: "single missing episode",
			episodes: []*storage.Episode{
				{Episode: model.Episode{ID: 1}, State: storage.EpisodeStateMissing},
			},
			expected: storage.SeasonStateMissing,
		},
		{
			name: "single unreleased episode",
			episodes: []*storage.Episode{
				{Episode: model.Episode{ID: 1}, State: storage.EpisodeStateUnreleased},
			},
			expected: storage.SeasonStateUnreleased,
		},
		{
			name: "single downloading episode",
			episodes: []*storage.Episode{
				{Episode: model.Episode{ID: 1}, State: storage.EpisodeStateDownloading},
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
		episodes       []*storage.Episode
		expectedState  storage.SeasonState
		expectedCounts map[string]int
	}{
		{
			name:          "empty episodes",
			episodes:      []*storage.Episode{},
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
			episodes: []*storage.Episode{
				{Episode: model.Episode{ID: 1}, State: storage.EpisodeStateDownloaded},
				{Episode: model.Episode{ID: 2}, State: storage.EpisodeStateDownloaded},
				{Episode: model.Episode{ID: 3}, State: storage.EpisodeStateDownloading},
				{Episode: model.Episode{ID: 4}, State: storage.EpisodeStateMissing},
				{Episode: model.Episode{ID: 5}, State: storage.EpisodeStateMissing},
				{Episode: model.Episode{ID: 6}, State: storage.EpisodeStateUnreleased},
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
			episodes: []*storage.Episode{
				{Episode: model.Episode{ID: 1}, State: storage.EpisodeStateDiscovered},
				{Episode: model.Episode{ID: 2}, State: storage.EpisodeStateCompleted},
				{Episode: model.Episode{ID: 3}, State: storage.EpisodeStateDiscovered},
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
			episodes: []*storage.Episode{
				{Episode: model.Episode{ID: 1}, State: storage.EpisodeStateDiscovered},
				{Episode: model.Episode{ID: 2}, State: storage.EpisodeStateDiscovered},
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
			episodes: []*storage.Episode{
				{Episode: model.Episode{ID: 1}, State: storage.EpisodeStateDownloaded},
				{Episode: model.Episode{ID: 2}, State: storage.EpisodeStateMissing},
				{Episode: model.Episode{ID: 3}, State: storage.EpisodeStateUnreleased},
				{Episode: model.Episode{ID: 4}, State: storage.EpisodeStateUnreleased},
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
