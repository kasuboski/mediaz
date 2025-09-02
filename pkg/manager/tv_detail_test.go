package manager

import (
	"context"
	"testing"

	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/mocks"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestListSeasonsForSeries(t *testing.T) {
	ctx := context.Background()

	t.Run("successful seasons listing", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := mocks.NewMockStorage(ctrl)
		m := New(nil, nil, nil, store, nil, config.Manager{})

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

		store.EXPECT().GetSeriesMetadata(gomock.Any(), gomock.Any()).Return(seriesMetadata, nil)
		store.EXPECT().ListSeasons(ctx, gomock.Any()).Return([]*storage.Season{season}, nil)
		store.EXPECT().GetSeasonMetadata(ctx, gomock.Any()).Return(seasonMetadata, nil)

		seasons, err := m.ListSeasonsForSeries(ctx, tmdbID)
		require.NoError(t, err)
		require.Len(t, seasons, 1)

		expectedSeason := SeasonResult{
			TMDBID:       67890,
			SeriesID:     1,
			Number:       1,
			Title:        "Season 1",
			EpisodeCount: 0,
			Monitored:    true,
		}

		assert.Equal(t, expectedSeason, seasons[0])
	})

	t.Run("series metadata storage error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := mocks.NewMockStorage(ctrl)
		m := New(nil, nil, nil, store, nil, config.Manager{})

		tmdbID := 99999
		// Mock storage to return an error other than ErrNotFound to avoid calling tmdb
		storageErr := assert.AnError
		store.EXPECT().GetSeriesMetadata(gomock.Any(), gomock.Any()).Return(nil, storageErr)

		seasons, err := m.ListSeasonsForSeries(ctx, tmdbID)
		require.Error(t, err)
		assert.Equal(t, storageErr, err)
		assert.Nil(t, seasons)
	})

	t.Run("empty seasons list", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := mocks.NewMockStorage(ctrl)
		m := New(nil, nil, nil, store, nil, config.Manager{})

		tmdbID := 12345
		seriesMetadata := &model.SeriesMetadata{
			ID:     1,
			TmdbID: int32(tmdbID),
			Title:  "Test Series",
		}

		store.EXPECT().GetSeriesMetadata(gomock.Any(), gomock.Any()).Return(seriesMetadata, nil)
		store.EXPECT().ListSeasons(ctx, gomock.Any()).Return([]*storage.Season{}, nil)

		seasons, err := m.ListSeasonsForSeries(ctx, tmdbID)
		require.NoError(t, err)
		assert.Empty(t, seasons)
	})
}

func TestListEpisodesForSeason(t *testing.T) {
	ctx := context.Background()

	t.Run("successful episodes listing", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := mocks.NewMockStorage(ctrl)
		m := New(nil, nil, nil, store, nil, config.Manager{})

		tmdbID := 12345
		seasonNumber := 1

		seriesMetadata := &model.SeriesMetadata{
			ID:     1,
			TmdbID: int32(tmdbID),
			Title:  "Test Series",
		}

		seasonMetadata := &model.SeasonMetadata{
			ID:       10,
			SeriesID: 1,
			Number:   int32(seasonNumber),
			TmdbID:   67890,
			Title:    "Season 1",
		}

		season := &storage.Season{
			Season: model.Season{
				ID:               100,
				SeriesID:         1,
				SeasonMetadataID: &seasonMetadata.ID,
				Monitored:        1,
			},
		}

		episodeMetadataID := int32(200)
		episode := &storage.Episode{
			Episode: model.Episode{
				ID:                200,
				SeasonID:          100,
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

		store.EXPECT().GetSeriesMetadata(gomock.Any(), gomock.Any()).Return(seriesMetadata, nil)
		store.EXPECT().GetSeasonMetadata(ctx, gomock.Any()).Return(seasonMetadata, nil)
		store.EXPECT().GetSeason(ctx, gomock.Any()).Return(season, nil)
		store.EXPECT().ListEpisodes(ctx, gomock.Any()).Return([]*storage.Episode{episode}, nil)
		store.EXPECT().GetEpisodeMetadata(ctx, gomock.Any()).Return(episodeMetadata, nil)

		episodes, err := m.ListEpisodesForSeason(ctx, tmdbID, seasonNumber)
		require.NoError(t, err)
		require.Len(t, episodes, 1)

		expectedEpisode := EpisodeResult{
			TMDBID:       54321,
			SeriesID:     1,
			SeasonNumber: 1,
			Number:       1,
			Title:        "Episode 1",
			Monitored:    true,
			Downloaded:   true,
		}

		assert.Equal(t, expectedEpisode, episodes[0])
	})

	t.Run("series metadata storage error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := mocks.NewMockStorage(ctrl)
		m := New(nil, nil, nil, store, nil, config.Manager{})

		tmdbID := 99999
		seasonNumber := 1

		// Mock storage to return an error other than ErrNotFound to avoid calling tmdb
		storageErr := assert.AnError
		store.EXPECT().GetSeriesMetadata(gomock.Any(), gomock.Any()).Return(nil, storageErr)

		episodes, err := m.ListEpisodesForSeason(ctx, tmdbID, seasonNumber)
		require.Error(t, err)
		assert.Equal(t, storageErr, err)
		assert.Nil(t, episodes)
	})

	t.Run("season not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := mocks.NewMockStorage(ctrl)
		m := New(nil, nil, nil, store, nil, config.Manager{})

		tmdbID := 12345
		seasonNumber := 99

		seriesMetadata := &model.SeriesMetadata{
			ID:     1,
			TmdbID: int32(tmdbID),
			Title:  "Test Series",
		}

		store.EXPECT().GetSeriesMetadata(gomock.Any(), gomock.Any()).Return(seriesMetadata, nil)
		store.EXPECT().GetSeasonMetadata(ctx, gomock.Any()).Return(nil, storage.ErrNotFound)

		episodes, err := m.ListEpisodesForSeason(ctx, tmdbID, seasonNumber)
		require.Error(t, err)
		assert.Nil(t, episodes)
	})

	t.Run("empty episodes list", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		store := mocks.NewMockStorage(ctrl)
		m := New(nil, nil, nil, store, nil, config.Manager{})

		tmdbID := 12345
		seasonNumber := 1

		seriesMetadata := &model.SeriesMetadata{
			ID:     1,
			TmdbID: int32(tmdbID),
			Title:  "Test Series",
		}

		seasonMetadata := &model.SeasonMetadata{
			ID:       10,
			SeriesID: 1,
			Number:   int32(seasonNumber),
			TmdbID:   67890,
			Title:    "Season 1",
		}

		season := &storage.Season{
			Season: model.Season{
				ID:               100,
				SeriesID:         1,
				SeasonMetadataID: &seasonMetadata.ID,
				Monitored:        1,
			},
		}

		store.EXPECT().GetSeriesMetadata(gomock.Any(), gomock.Any()).Return(seriesMetadata, nil)
		store.EXPECT().GetSeasonMetadata(ctx, gomock.Any()).Return(seasonMetadata, nil)
		store.EXPECT().GetSeason(ctx, gomock.Any()).Return(season, nil)
		store.EXPECT().ListEpisodes(ctx, gomock.Any()).Return([]*storage.Episode{}, nil)

		episodes, err := m.ListEpisodesForSeason(ctx, tmdbID, seasonNumber)
		require.NoError(t, err)
		assert.Empty(t, episodes)
	})
}
