package manager

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/mocks"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	tmdbMocks "github.com/kasuboski/mediaz/pkg/tmdb/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func int32Ptr(v int32) *int32 {
	return &v
}

func TestGetTVDetailByTMDBID_WithSeasonsAndEpisodes(t *testing.T) {
	ctx := context.Background()

	t.Run("successful TV detail with seasons and episodes", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := mocks.NewMockStorage(ctrl)
		tmdbClient := tmdbMocks.NewMockITmdb(ctrl)
		m := New(tmdbClient, nil, nil, store, nil, config.Manager{})

		tmdbID := 12345
		seriesMetadata := &model.SeriesMetadata{
			ID:     1,
			TmdbID: int32(tmdbID),
			Title:  "Test Series",
		}

		seasonMetadataID := int32(10)
		season := &storage.Season{
			Season: model.Season{
				ID:               1,
				SeriesID:         1,
				SeasonMetadataID: &seasonMetadataID,
				Monitored:        1,
			},
		}

		seasonMetadata := &model.SeasonMetadata{
			ID:     10,
			Number: 1,
			TmdbID: 67890,
			Title:  "Season 1",
		}

		episodeMetadataID := int32(200)
		episode := &storage.Episode{
			Episode: model.Episode{
				ID:                200,
				SeasonID:          1,
				EpisodeMetadataID: &episodeMetadataID,
				Monitored:         1,
				EpisodeNumber:     1,
			},
			State: storage.EpisodeStateDownloaded,
		}

		episodeMetadata := &model.EpisodeMetadata{
			ID:     200,
			Number: 1,
			TmdbID: 54321,
			Title:  "Episode 1",
		}

		series := &storage.Series{
			Series: model.Series{
				ID:               1,
				SeriesMetadataID: int32Ptr(1),
			},
		}

		// Mock calls for getting series metadata and details from TMDB
		store.EXPECT().GetSeriesMetadata(gomock.Any(), gomock.Any()).Return(seriesMetadata, nil)
		
		// Mock TMDB API call
		mockResponse := &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(`{"poster_path": "poster.jpg", "backdrop_path": "backdrop.jpg"}`)),
		}
		tmdbClient.EXPECT().TvSeriesDetails(gomock.Any(), int32(tmdbID), nil).Return(mockResponse, nil)
		
		// Mock storage calls for main TV detail
		store.EXPECT().GetSeries(ctx, gomock.Any()).Return(series, nil)
		
		// Mock storage calls for seasons and episodes
		store.EXPECT().ListSeasons(ctx, gomock.Any()).Return([]*storage.Season{season}, nil)
		store.EXPECT().GetSeasonMetadata(ctx, gomock.Any()).Return(seasonMetadata, nil)
		store.EXPECT().ListEpisodes(ctx, gomock.Any()).Return([]*storage.Episode{episode}, nil)
		store.EXPECT().GetEpisodeMetadata(ctx, gomock.Any()).Return(episodeMetadata, nil)

		result, err := m.GetTVDetailByTMDBID(ctx, tmdbID)
		require.NoError(t, err)
		require.NotNil(t, result)
		
		// Verify basic TV details
		assert.Equal(t, int32(tmdbID), result.TMDBID)
		assert.Equal(t, "Test Series", result.Title)
		
		// Verify seasons are included
		require.Len(t, result.Seasons, 1)
		season0 := result.Seasons[0]
		assert.Equal(t, int32(67890), season0.TMDBID)
		assert.Equal(t, int32(1), season0.SeriesID)
		assert.Equal(t, int32(1), season0.Number)
		assert.Equal(t, "Season 1", season0.Title)
		assert.True(t, season0.Monitored)
		
		// Verify episodes are included within seasons
		require.Len(t, season0.Episodes, 1)
		episode0 := season0.Episodes[0]
		assert.Equal(t, int32(54321), episode0.TMDBID)
		assert.Equal(t, int32(1), episode0.SeriesID)
		assert.Equal(t, int32(1), episode0.SeasonNumber)
		assert.Equal(t, int32(1), episode0.Number)
		assert.Equal(t, "Episode 1", episode0.Title)
		assert.True(t, episode0.Monitored)
		assert.True(t, episode0.Downloaded)
	})

	t.Run("TV detail with series not in library", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := mocks.NewMockStorage(ctrl)
		tmdbClient := tmdbMocks.NewMockITmdb(ctrl)
		m := New(tmdbClient, nil, nil, store, nil, config.Manager{})

		tmdbID := 99999
		seriesMetadata := &model.SeriesMetadata{
			ID:     1,
			TmdbID: int32(tmdbID),
			Title:  "Test Series",
		}

		// Mock calls for getting series metadata and details from TMDB
		store.EXPECT().GetSeriesMetadata(gomock.Any(), gomock.Any()).Return(seriesMetadata, nil)
		
		// Mock TMDB API call
		mockResponse := &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(`{"poster_path": "poster.jpg"}`)),
		}
		tmdbClient.EXPECT().TvSeriesDetails(gomock.Any(), int32(tmdbID), nil).Return(mockResponse, nil)
		
		// Mock storage calls - series not found
		store.EXPECT().GetSeries(ctx, gomock.Any()).Return(nil, storage.ErrNotFound)

		result, err := m.GetTVDetailByTMDBID(ctx, tmdbID)
		require.NoError(t, err)
		require.NotNil(t, result)
		
		// Verify basic TV details
		assert.Equal(t, int32(tmdbID), result.TMDBID)
		assert.Equal(t, "Test Series", result.Title)
		assert.Equal(t, "Not In Library", result.LibraryStatus)
		
		// Verify no seasons since series is not in library
		assert.Empty(t, result.Seasons)
	})

	t.Run("TV detail with empty seasons", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := mocks.NewMockStorage(ctrl)
		tmdbClient := tmdbMocks.NewMockITmdb(ctrl)
		m := New(tmdbClient, nil, nil, store, nil, config.Manager{})

		tmdbID := 12345
		seriesMetadata := &model.SeriesMetadata{
			ID:     1,
			TmdbID: int32(tmdbID),
			Title:  "Test Series",
		}

		series := &storage.Series{
			Series: model.Series{
				ID:               1,
				SeriesMetadataID: int32Ptr(1),
			},
		}

		// Mock calls for getting series metadata and details from TMDB
		store.EXPECT().GetSeriesMetadata(gomock.Any(), gomock.Any()).Return(seriesMetadata, nil)
		
		// Mock TMDB API call
		mockResponse := &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(`{"poster_path": "poster.jpg"}`)),
		}
		tmdbClient.EXPECT().TvSeriesDetails(gomock.Any(), int32(tmdbID), nil).Return(mockResponse, nil)
		
		// Mock storage calls
		store.EXPECT().GetSeries(ctx, gomock.Any()).Return(series, nil)
		store.EXPECT().ListSeasons(ctx, gomock.Any()).Return([]*storage.Season{}, nil)

		result, err := m.GetTVDetailByTMDBID(ctx, tmdbID)
		require.NoError(t, err)
		require.NotNil(t, result)
		
		// Verify basic TV details
		assert.Equal(t, int32(tmdbID), result.TMDBID)
		assert.Equal(t, "Test Series", result.Title)
		
		// Verify empty seasons
		assert.Empty(t, result.Seasons)
	})
}

