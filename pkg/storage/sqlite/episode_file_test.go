package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/pkg/ptr"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEpisodeFileStorage(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	assert.NotNil(t, store)

	// Test creating an episode file
	file := model.EpisodeFile{
		Quality:      "HD",
		Size:         1024,
		RelativePath: ptr.To("test/path.mp4"),
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
			RelativePath:     ptr.To("series/episode.mp4"),
			OriginalFilePath: ptr.To("/tv/series/episode.mp4"),
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
		assert.ErrorIs(t, err, storage.ErrNotFound)
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
			RelativePath:     ptr.To("series/episode1.mp4"),
			OriginalFilePath: ptr.To("/old/path/tv/series/episode1.mp4"),
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
			RelativePath:     ptr.To("series/episode2.mp4"),
			OriginalFilePath: ptr.To("/tv/series/episode2.mp4"),
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
			RelativePath:     ptr.To("series/episode.mp4"),
			OriginalFilePath: ptr.To("/tv/series/episode.mp4"),
		}

		err := store.UpdateEpisodeFile(ctx, 99999, nonExistentFile)
		assert.NoError(t, err)
	})
}

func TestCreateEpisodeFileWithExplicitID(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	require.NotNil(t, store)

	added := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	file := model.EpisodeFile{
		ID:               501,
		Quality:          "UHD",
		Size:             4096,
		Added:            added,
		RelativePath:     ptr.To("series/explicit.mp4"),
		OriginalFilePath: ptr.To("/tv/series/explicit.mp4"),
	}

	id, err := store.CreateEpisodeFile(ctx, file)
	require.NoError(t, err)
	assert.Equal(t, int64(501), id)

	retrieved, err := store.GetEpisodeFile(ctx, 501)
	require.NoError(t, err)
	assert.Equal(t, "UHD", retrieved.Quality)
	assert.Equal(t, int64(4096), retrieved.Size)
	assert.WithinDuration(t, added, retrieved.Added, time.Second)
	assert.Equal(t, file.RelativePath, retrieved.RelativePath)
	assert.Equal(t, file.OriginalFilePath, retrieved.OriginalFilePath)
}

func TestCreateEpisodeFileConflictKeepsExisting(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	require.NotNil(t, store)

	// The legacy upsert assigned every mutable column to its own current
	// value on conflict, so a conflicting insert never overwrites the stored
	// row and returns the existing id. This behavior is preserved.
	original := model.EpisodeFile{
		ID:               502,
		Quality:          "HD",
		Size:             1024,
		Added:            time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second),
		RelativePath:     ptr.To("series/original.mp4"),
		OriginalFilePath: ptr.To("/tv/original.mp4"),
	}

	id, err := store.CreateEpisodeFile(ctx, original)
	require.NoError(t, err)
	require.Equal(t, int64(502), id)

	conflicting := model.EpisodeFile{
		ID:               502,
		Quality:          "UHD",
		Size:             999999,
		Added:            time.Now().UTC(),
		RelativePath:     ptr.To("series/conflict.mp4"),
		OriginalFilePath: ptr.To("/tv/conflict.mp4"),
	}

	upsertID, err := store.CreateEpisodeFile(ctx, conflicting)
	require.NoError(t, err)
	assert.Equal(t, int64(502), upsertID)

	retrieved, err := store.GetEpisodeFile(ctx, 502)
	require.NoError(t, err)
	assert.Equal(t, original.Quality, retrieved.Quality, "quality must be preserved on conflict")
	assert.Equal(t, original.Size, retrieved.Size, "size must be preserved on conflict")
	assert.WithinDuration(t, original.Added, retrieved.Added, time.Second, "added must be preserved on conflict")
	require.NotNil(t, retrieved.RelativePath)
	assert.Equal(t, "series/original.mp4", *retrieved.RelativePath, "relative path must be preserved on conflict")
	require.NotNil(t, retrieved.OriginalFilePath)
	assert.Equal(t, "/tv/original.mp4", *retrieved.OriginalFilePath, "original file path must be preserved on conflict")
}

func TestCreateEpisodeFileDefaultAddedTime(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	require.NotNil(t, store)

	// A zero-value id omits the added column so the CURRENT_TIMESTAMP column
	// default applies, like the legacy insert branch.
	file := model.EpisodeFile{
		Quality: "HD",
		Size:    512,
	}

	id, err := store.CreateEpisodeFile(ctx, file)
	require.NoError(t, err)
	require.Greater(t, id, int64(0))

	retrieved, err := store.GetEpisodeFile(ctx, int32(id))
	require.NoError(t, err)
	assert.False(t, retrieved.Added.IsZero(), "added should fall back to the column default")
	assert.WithinDuration(t, time.Now().UTC(), retrieved.Added, time.Minute)
}
