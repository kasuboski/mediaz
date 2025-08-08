package manager

import (
	"context"
	"errors"
	"testing"

	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/library"
	mockLibrary "github.com/kasuboski/mediaz/pkg/library/mocks"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestIndexSeriesLibrary(t *testing.T) {
	t.Run("error finding files in library", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ctx := context.Background()

		library := mockLibrary.NewMockLibrary(ctrl)
		library.EXPECT().FindEpisodes(ctx).Times(1).Return(nil, errors.New("expected tested error"))
		m := New(nil, nil, library, nil, nil, config.Manager{})
		require.NotNil(t, m)

		err := m.IndexSeriesLibrary(ctx)
		assert.Error(t, err)
		assert.EqualError(t, err, "expected tested error")
	})

	t.Run("no files discovered", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ctx := context.Background()

		mockLibrary := mockLibrary.NewMockLibrary(ctrl)
		mockLibrary.EXPECT().FindEpisodes(ctx).Times(1).Return([]library.EpisodeFile{}, nil)
		m := New(nil, nil, mockLibrary, nil, nil, config.Manager{})
		require.NotNil(t, m)

		err := m.IndexSeriesLibrary(ctx)
		assert.NoError(t, err)
	})

	t.Run("error listing episode files from storage", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ctx := context.Background()

		store := newStore(t, ctx)
		mockLibrary := mockLibrary.NewMockLibrary(ctrl)

		discoveredFiles := []library.EpisodeFile{
			{RelativePath: "Show 1/Season 01/episode1.mkv", AbsolutePath: "/tv/Show 1/Season 01/episode1.mkv", SeriesName: "Show 1"},
		}

		mockLibrary.EXPECT().FindEpisodes(ctx).Times(1).Return(discoveredFiles, nil)

		m := New(nil, nil, mockLibrary, store, nil, config.Manager{})
		require.NotNil(t, m)

		err := m.IndexSeriesLibrary(ctx)
		assert.NoError(t, err)
	})

	t.Run("successfully indexes new episode files", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ctx := context.Background()

		store := newStore(t, ctx)
		mockLibrary := mockLibrary.NewMockLibrary(ctrl)

		discoveredFiles := []library.EpisodeFile{
			{RelativePath: "Show 1/Season 01/episode1.mkv", AbsolutePath: "/tv/Show 1/Season 01/episode1.mkv", Size: 1024, SeriesName: "Show 1"},
			{RelativePath: "Show 2/Season 01/episode1.mp4", AbsolutePath: "/tv/Show 2/Season 01/episode1.mp4", Size: 2048, SeriesName: "Show 2"},
		}

		mockLibrary.EXPECT().FindEpisodes(ctx).Times(1).Return(discoveredFiles, nil)

		m := New(nil, nil, mockLibrary, store, nil, config.Manager{})
		require.NotNil(t, m)

		err := m.IndexSeriesLibrary(ctx)
		assert.NoError(t, err)

		series, err := store.ListSeries(ctx)
		require.NoError(t, err)
		assert.Len(t, series, 2)

		episodeFiles, err := store.ListEpisodeFiles(ctx)
		require.NoError(t, err)
		assert.Len(t, episodeFiles, 2)

		assert.Equal(t, "Show 1/Season 01/episode1.mkv", *episodeFiles[0].RelativePath)
		assert.Equal(t, int64(1024), episodeFiles[0].Size)
		assert.Equal(t, "Show 2/Season 01/episode1.mp4", *episodeFiles[1].RelativePath)
		assert.Equal(t, int64(2048), episodeFiles[1].Size)

		assert.Equal(t, "Show 1", *series[0].Path)
		assert.Equal(t, "Show 2", *series[1].Path)
	})

	t.Run("create series for already tracked file", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ctx := context.Background()

		store := newStore(t, ctx)
		mockLibrary := mockLibrary.NewMockLibrary(ctrl)

		_, err := store.CreateEpisodeFile(ctx, model.EpisodeFile{
			Size:             1024,
			RelativePath:     ptr("Show 1/Season 01/episode1.mkv"),
			OriginalFilePath: ptr("/tv/Show 1/Season 01/episode1.mkv"),
		})
		require.NoError(t, err)

		discoveredFiles := []library.EpisodeFile{
			{RelativePath: "Show 1/Season 01/episode1.mkv", AbsolutePath: "/tv/Show 1/Season 01/episode1.mkv", SeriesName: "Show 1"},
		}

		mockLibrary.EXPECT().FindEpisodes(ctx).Times(1).Return(discoveredFiles, nil)

		m := New(nil, nil, mockLibrary, store, nil, config.Manager{})
		require.NotNil(t, m)

		err = m.IndexSeriesLibrary(ctx)
		assert.NoError(t, err)

		episodeFiles, err := store.ListEpisodeFiles(ctx)
		require.NoError(t, err)
		assert.Len(t, episodeFiles, 1)

		series, err := store.ListSeries(ctx)
		require.NoError(t, err)
		require.Len(t, series, 1)

		assert.Equal(t, "Show 1", *series[0].Path)
	})

	t.Run("link new episode file to existing series", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ctx := context.Background()

		store := newStore(t, ctx)
		mockLibrary := mockLibrary.NewMockLibrary(ctrl)

		_, err := store.CreateEpisodeFile(ctx, model.EpisodeFile{
			Size:             1024,
			RelativePath:     ptr("Show 1/Season 01/episode1.mkv"),
			OriginalFilePath: ptr("/tv/Show 1/Season 01/episode1.mkv"),
		})
		require.NoError(t, err)

		discoveredFiles := []library.EpisodeFile{
			{RelativePath: "Show 1/Season 01/episode1.mkv", AbsolutePath: "/tv/Show 1/Season 01/episode1.mkv", SeriesName: "Show 1"},
		}

		mockLibrary.EXPECT().FindEpisodes(ctx).Times(1).Return(discoveredFiles, nil)

		m := New(nil, nil, mockLibrary, store, nil, config.Manager{})
		require.NotNil(t, m)

		err = m.IndexSeriesLibrary(ctx)
		assert.NoError(t, err)

		episodeFiles, err := store.ListEpisodeFiles(ctx)
		require.NoError(t, err)
		assert.Len(t, episodeFiles, 1)

		series, err := store.ListSeries(ctx)
		require.NoError(t, err)
		require.Len(t, series, 1)

		assert.Equal(t, "Show 1", *series[0].Path)

		discoveredFiles = []library.EpisodeFile{
			{RelativePath: "Show 1/Season 01/episode1.mkv", AbsolutePath: "/tv/Show 1/Season 01/episode1.mkv", SeriesName: "Show 1"},
			{RelativePath: "Show 1/Season 01/episode2.mkv", AbsolutePath: "/tv/Show 1/Season 01/episode2.mkv", SeriesName: "Show 1"},
		}

		mockLibrary.EXPECT().FindEpisodes(ctx).Times(1).Return(discoveredFiles, nil)

		err = m.IndexSeriesLibrary(ctx)
		assert.NoError(t, err)

		episodeFiles, err = store.ListEpisodeFiles(ctx)
		require.NoError(t, err)
		assert.Len(t, episodeFiles, 2)

		series, err = store.ListSeries(ctx)
		require.NoError(t, err)
		require.Len(t, series, 1)
	})
}
