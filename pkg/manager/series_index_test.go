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
		m := New(nil, nil, library, nil, nil, config.Manager{}, config.Config{})
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
		m := New(nil, nil, mockLibrary, nil, nil, config.Manager{}, config.Config{})
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

		m := New(nil, nil, mockLibrary, store, nil, config.Manager{}, config.Config{})
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

		m := New(nil, nil, mockLibrary, store, nil, config.Manager{}, config.Config{})
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

		m := New(nil, nil, mockLibrary, store, nil, config.Manager{}, config.Config{})
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

		m := New(nil, nil, mockLibrary, store, nil, config.Manager{}, config.Config{})
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

	t.Run("updates absolute path when library moves", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ctx := context.Background()

		store := newStore(t, ctx)
		mockLibrary := mockLibrary.NewMockLibrary(ctrl)

		fileID, err := store.CreateEpisodeFile(ctx, model.EpisodeFile{
			Size:             1024,
			RelativePath:     ptr("Fargo/Fargo - S01E01.mkv"),
			OriginalFilePath: ptr("/old/library/tv/Fargo/Fargo - S01E01.mkv"),
		})
		require.NoError(t, err)

		discoveredFiles := []library.EpisodeFile{
			{
				RelativePath:  "Fargo/Fargo - S01E01.mkv",
				AbsolutePath:  "/new/library/tv/Fargo/Fargo - S01E01.mkv",
				SeriesName:    "Fargo",
				SeasonNumber:  1,
				EpisodeNumber: 1,
				Size:          1024,
			},
		}

		mockLibrary.EXPECT().FindEpisodes(ctx).Times(1).Return(discoveredFiles, nil)

		m := New(nil, nil, mockLibrary, store, nil, config.Manager{}, config.Config{})
		require.NotNil(t, m)

		err = m.IndexSeriesLibrary(ctx)
		assert.NoError(t, err)

		episodeFiles, err := store.ListEpisodeFiles(ctx)
		require.NoError(t, err)
		assert.Len(t, episodeFiles, 1, "Should still have 1 file, not duplicate")

		updatedFile, err := store.GetEpisodeFile(ctx, int32(fileID))
		require.NoError(t, err)
		assert.Equal(t, "/new/library/tv/Fargo/Fargo - S01E01.mkv", *updatedFile.OriginalFilePath, "Absolute path should be updated")
		assert.Equal(t, "Fargo/Fargo - S01E01.mkv", *updatedFile.RelativePath, "Relative path should remain the same")
	})

	t.Run("does not update when paths match", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ctx := context.Background()

		store := newStore(t, ctx)
		mockLibrary := mockLibrary.NewMockLibrary(ctrl)

		fileID, err := store.CreateEpisodeFile(ctx, model.EpisodeFile{
			Size:             1024,
			RelativePath:     ptr("Fargo/Fargo - S01E01.mkv"),
			OriginalFilePath: ptr("/library/tv/Fargo/Fargo - S01E01.mkv"),
		})
		require.NoError(t, err)

		discoveredFiles := []library.EpisodeFile{
			{
				RelativePath:  "Fargo/Fargo - S01E01.mkv",
				AbsolutePath:  "/library/tv/Fargo/Fargo - S01E01.mkv",
				SeriesName:    "Fargo",
				SeasonNumber:  1,
				EpisodeNumber: 1,
				Size:          1024,
			},
		}

		mockLibrary.EXPECT().FindEpisodes(ctx).Times(1).Return(discoveredFiles, nil)

		m := New(nil, nil, mockLibrary, store, nil, config.Manager{}, config.Config{})
		require.NotNil(t, m)

		err = m.IndexSeriesLibrary(ctx)
		assert.NoError(t, err)

		updatedFile, err := store.GetEpisodeFile(ctx, int32(fileID))
		require.NoError(t, err)
		assert.Equal(t, "/library/tv/Fargo/Fargo - S01E01.mkv", *updatedFile.OriginalFilePath, "Path should remain unchanged")
	})

	t.Run("handles multiple files with mixed scenarios", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ctx := context.Background()

		store := newStore(t, ctx)
		mockLibrary := mockLibrary.NewMockLibrary(ctrl)

		existingFileID, err := store.CreateEpisodeFile(ctx, model.EpisodeFile{
			Size:             1024,
			RelativePath:     ptr("Fargo/Fargo - S01E01.mkv"),
			OriginalFilePath: ptr("/old/path/tv/Fargo/Fargo - S01E01.mkv"),
		})
		require.NoError(t, err)

		movedFileID, err := store.CreateEpisodeFile(ctx, model.EpisodeFile{
			Size:             2048,
			RelativePath:     ptr("Fargo/Fargo - S01E02.mkv"),
			OriginalFilePath: ptr("/old/path/tv/Fargo/Fargo - S01E02.mkv"),
		})
		require.NoError(t, err)

		discoveredFiles := []library.EpisodeFile{
			{
				RelativePath:  "Fargo/Fargo - S01E01.mkv",
				AbsolutePath:  "/old/path/tv/Fargo/Fargo - S01E01.mkv",
				SeriesName:    "Fargo",
				SeasonNumber:  1,
				EpisodeNumber: 1,
				Size:          1024,
			},
			{
				RelativePath:  "Fargo/Fargo - S01E02.mkv",
				AbsolutePath:  "/new/path/tv/Fargo/Fargo - S01E02.mkv",
				SeriesName:    "Fargo",
				SeasonNumber:  1,
				EpisodeNumber: 2,
				Size:          2048,
			},
			{
				RelativePath:  "Fargo/Fargo - S01E03.mkv",
				AbsolutePath:  "/new/path/tv/Fargo/Fargo - S01E03.mkv",
				SeriesName:    "Fargo",
				SeasonNumber:  1,
				EpisodeNumber: 3,
				Size:          3072,
			},
		}

		mockLibrary.EXPECT().FindEpisodes(ctx).Times(1).Return(discoveredFiles, nil)

		m := New(nil, nil, mockLibrary, store, nil, config.Manager{}, config.Config{})
		require.NotNil(t, m)

		err = m.IndexSeriesLibrary(ctx)
		assert.NoError(t, err)

		episodeFiles, err := store.ListEpisodeFiles(ctx)
		require.NoError(t, err)
		assert.Len(t, episodeFiles, 3, "Should have 3 files: 2 existing + 1 new")

		unchangedFile, err := store.GetEpisodeFile(ctx, int32(existingFileID))
		require.NoError(t, err)
		assert.Equal(t, "/old/path/tv/Fargo/Fargo - S01E01.mkv", *unchangedFile.OriginalFilePath, "File with same path should not be updated")

		movedFile, err := store.GetEpisodeFile(ctx, int32(movedFileID))
		require.NoError(t, err)
		assert.Equal(t, "/new/path/tv/Fargo/Fargo - S01E02.mkv", *movedFile.OriginalFilePath, "File with new path should be updated")

		var newFile *model.EpisodeFile
		for _, f := range episodeFiles {
			if *f.RelativePath == "Fargo/Fargo - S01E03.mkv" {
				newFile = f
				break
			}
		}
		require.NotNil(t, newFile, "Should find new file")
		assert.Equal(t, "/new/path/tv/Fargo/Fargo - S01E03.mkv", *newFile.OriginalFilePath, "New file should have correct path")
		assert.Equal(t, int64(3072), newFile.Size, "New file should have correct size")
	})

	t.Run("does not match on empty paths", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ctx := context.Background()

		store := newStore(t, ctx)
		mockLibrary := mockLibrary.NewMockLibrary(ctrl)

		_, err := store.CreateEpisodeFile(ctx, model.EpisodeFile{
			Size:             1024,
			RelativePath:     ptr(""),
			OriginalFilePath: ptr(""),
		})
		require.NoError(t, err)

		discoveredFiles := []library.EpisodeFile{
			{
				RelativePath:  "Fargo/Fargo - S01E01.mkv",
				AbsolutePath:  "/path/tv/Fargo/Fargo - S01E01.mkv",
				SeriesName:    "Fargo",
				SeasonNumber:  1,
				EpisodeNumber: 1,
				Size:          2048,
			},
		}

		mockLibrary.EXPECT().FindEpisodes(ctx).Times(1).Return(discoveredFiles, nil)

		m := New(nil, nil, mockLibrary, store, nil, config.Manager{}, config.Config{})
		require.NotNil(t, m)

		err = m.IndexSeriesLibrary(ctx)
		assert.NoError(t, err)

		episodeFiles, err := store.ListEpisodeFiles(ctx)
		require.NoError(t, err)
		assert.Len(t, episodeFiles, 2, "Should create new file instead of matching empty path")
	})
}
