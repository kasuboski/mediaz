package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEpisodeStorage(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	assert.NotNil(t, store)

	series := storage.Series{
		Series: model.Series{
			Monitored:        1,
			QualityProfileID: 1,
			Added:            ptr(time.Now()),
		},
	}

	seriesID, err := store.CreateSeries(ctx, series, storage.SeriesStateMissing)
	require.Nil(t, err)

	season := storage.Season{
		Season: model.Season{
			SeriesID: int32(seriesID),
		},
	}

	seasonID, err := store.CreateSeason(ctx, season, storage.SeasonStateMissing)
	require.Nil(t, err)

	// Test creating an episode
	episode := storage.Episode{
		Episode: model.Episode{
			SeasonID:      int32(seasonID),
			EpisodeNumber: 1,
			Monitored:     1,
		},
	}

	id, err := store.CreateEpisode(ctx, episode, storage.EpisodeStateMissing)
	assert.Nil(t, err)
	assert.Greater(t, id, int64(0))

	// Test getting the episode
	retrieved, err := store.GetEpisode(ctx, table.Episode.ID.EQ(sqlite.Int64(id)))
	assert.Nil(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, episode.SeasonID, retrieved.SeasonID)
	assert.Equal(t, episode.EpisodeNumber, retrieved.EpisodeNumber)
	assert.Equal(t, episode.Monitored, retrieved.Monitored)
	assert.Equal(t, storage.EpisodeStateMissing, retrieved.State)

	// Test listing episodes
	episodes, err := store.ListEpisodes(ctx, table.Episode.SeasonID.EQ(sqlite.Int64(seasonID)))
	assert.Nil(t, err)
	assert.Len(t, episodes, 1)
	assert.Equal(t, episode.EpisodeNumber, episodes[0].EpisodeNumber)

	// Test listing episodes by state
	where := table.EpisodeTransition.ToState.EQ(sqlite.String(string(storage.EpisodeStateMissing))).AND(table.Episode.SeasonID.EQ(sqlite.Int64(seasonID)))
	stateEpisodes, err := store.ListEpisodes(ctx, where)
	assert.Nil(t, err)
	assert.Len(t, stateEpisodes, 1)
	assert.Equal(t, storage.EpisodeStateMissing, stateEpisodes[0].State)

	// Test updating episode file ID
	err = store.UpdateEpisodeEpisodeFileID(ctx, id, 123)
	assert.Nil(t, err)

	// Test getting episode by file ID
	byFile, err := store.GetEpisodeByEpisodeFileID(ctx, 123)
	assert.Nil(t, err)
	assert.Equal(t, id, int64(byFile.ID))

	// Test deleting the episode
	err = store.DeleteEpisode(ctx, id)
	assert.Nil(t, err)

	// Verify deletion
	episodes, err = store.ListEpisodes(ctx, table.Episode.SeasonID.EQ(sqlite.Int64(seasonID)))
	assert.Nil(t, err)
	assert.Empty(t, episodes)

	// Test getting non-existent episode
	_, err = store.GetEpisode(ctx, table.Episode.ID.EQ(sqlite.Int64(id)))
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestEpisodeFileStorage(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	assert.NotNil(t, store)

	// Test creating an episode file
	file := model.EpisodeFile{
		Quality:      "HD",
		Size:         1024,
		RelativePath: ptr("test/path.mp4"),
	}

	id, err := store.CreateEpisodeFile(ctx, file)
	assert.Nil(t, err)
	assert.Greater(t, id, int64(0))

	// Test getting the episode file
	retrieved, err := store.GetEpisodeFileByID(ctx, id)
	assert.Nil(t, err)
	assert.NotEmpty(t, retrieved)
	assert.Equal(t, file.Quality, retrieved[0].Quality)
	assert.Equal(t, file.Size, retrieved[0].Size)
	assert.Equal(t, file.RelativePath, retrieved[0].RelativePath)

	// Test listing episode files
	files, err := store.ListEpisodeFiles(ctx)
	assert.Nil(t, err)
	assert.Len(t, files, 1)
	assert.Equal(t, file.Quality, files[0].Quality)

	// Test deleting the episode file
	err = store.DeleteEpisodeFile(ctx, id)
	assert.Nil(t, err)

	// Verify deletion
	files, err = store.ListEpisodeFiles(ctx)
	assert.Nil(t, err)
	assert.Empty(t, files)

	// Test getting non-existent episode file
	_, err = store.GetEpisodeFileByID(ctx, id)
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestGetEpisodeFile(t *testing.T) {
	t.Run("get episode file by id", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)
		require.NotNil(t, store)

		file := model.EpisodeFile{
			Quality:          "HD",
			Size:             2048,
			RelativePath:     ptr("series/episode.mp4"),
			OriginalFilePath: ptr("/tv/series/episode.mp4"),
		}

		id, err := store.CreateEpisodeFile(ctx, file)
		require.NoError(t, err)
		require.Greater(t, id, int64(0))

		retrieved, err := store.GetEpisodeFile(ctx, int32(id))
		require.NoError(t, err)
		require.NotNil(t, retrieved)

		assert.Equal(t, file.Quality, retrieved.Quality)
		assert.Equal(t, file.Size, retrieved.Size)
		assert.Equal(t, file.RelativePath, retrieved.RelativePath)
		assert.Equal(t, file.OriginalFilePath, retrieved.OriginalFilePath)
	})

	t.Run("get non-existent episode file", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)
		require.NotNil(t, store)

		_, err := store.GetEpisodeFile(ctx, 99999)
		assert.Error(t, err)
	})
}

func TestUpdateEpisodeFile(t *testing.T) {
	t.Run("update episode file absolute path", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)
		require.NotNil(t, store)

		file := model.EpisodeFile{
			Quality:          "HD",
			Size:             1024,
			RelativePath:     ptr("series/episode1.mp4"),
			OriginalFilePath: ptr("/old/path/tv/series/episode1.mp4"),
		}

		id, err := store.CreateEpisodeFile(ctx, file)
		require.NoError(t, err)
		require.Greater(t, id, int64(0))

		retrieved, err := store.GetEpisodeFile(ctx, int32(id))
		require.NoError(t, err)
		require.NotNil(t, retrieved)

		newAbsolutePath := "/new/path/tv/series/episode1.mp4"
		retrieved.OriginalFilePath = &newAbsolutePath

		err = store.UpdateEpisodeFile(ctx, int32(id), *retrieved)
		require.NoError(t, err)

		updated, err := store.GetEpisodeFile(ctx, int32(id))
		require.NoError(t, err)
		require.NotNil(t, updated)

		assert.Equal(t, newAbsolutePath, *updated.OriginalFilePath)
		assert.Equal(t, file.RelativePath, updated.RelativePath)
		assert.Equal(t, file.Quality, updated.Quality)
		assert.Equal(t, file.Size, updated.Size)
	})

	t.Run("update episode file quality and size", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)
		require.NotNil(t, store)

		file := model.EpisodeFile{
			Quality:          "SD",
			Size:             512,
			RelativePath:     ptr("series/episode2.mp4"),
			OriginalFilePath: ptr("/tv/series/episode2.mp4"),
		}

		id, err := store.CreateEpisodeFile(ctx, file)
		require.NoError(t, err)
		require.Greater(t, id, int64(0))

		retrieved, err := store.GetEpisodeFile(ctx, int32(id))
		require.NoError(t, err)
		require.NotNil(t, retrieved)

		retrieved.Quality = "UHD"
		retrieved.Size = 4096

		err = store.UpdateEpisodeFile(ctx, int32(id), *retrieved)
		require.NoError(t, err)

		updated, err := store.GetEpisodeFile(ctx, int32(id))
		require.NoError(t, err)
		require.NotNil(t, updated)

		assert.Equal(t, "UHD", updated.Quality)
		assert.Equal(t, int64(4096), updated.Size)
		assert.Equal(t, file.RelativePath, updated.RelativePath)
		assert.Equal(t, file.OriginalFilePath, updated.OriginalFilePath)
	})

	t.Run("update non-existent episode file", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)
		require.NotNil(t, store)

		nonExistentFile := model.EpisodeFile{
			ID:               99999,
			Quality:          "HD",
			Size:             1024,
			RelativePath:     ptr("series/episode.mp4"),
			OriginalFilePath: ptr("/tv/series/episode.mp4"),
		}

		err := store.UpdateEpisodeFile(ctx, 99999, nonExistentFile)
		assert.NoError(t, err)
	})
}
