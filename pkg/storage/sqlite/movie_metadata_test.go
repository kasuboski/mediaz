package sqlite

import (
	"testing"
	"time"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMovieMetadataStorage(t *testing.T) {
	ctx := t.Context()
	store := initSqlite(t, ctx)
	require.NotNil(t, store)

	movieMeta := model.MovieMetadata{
		ID:      1,
		TmdbID:  1234,
		Title:   "My Cool Movie",
		Runtime: 1000,
	}
	id, err := store.CreateMovieMetadata(ctx, movieMeta)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), id)

	metadata, err := store.ListMovieMetadata(ctx)
	assert.NoError(t, err)
	assert.Len(t, metadata, 1)
	actual := metadata[0]
	assert.Equal(t, &movieMeta, actual)

	one, err := store.GetMovieMetadataByTmdbID(ctx, 1234)
	assert.NoError(t, err)
	assert.NotNil(t, one)
	assert.Equal(t, movieMeta.ID, one.ID)

	byID, err := store.GetMovieMetadataByID(ctx, 1)
	assert.NoError(t, err)
	require.NotNil(t, byID)
	assert.Equal(t, movieMeta.Title, byID.Title)

	notFound, err := store.GetMovieMetadataByTmdbID(ctx, 124)
	assert.ErrorIs(t, err, storage.ErrNotFound)
	assert.Nil(t, notFound)

	notFound, err = store.GetMovieMetadataByID(ctx, 999)
	assert.ErrorIs(t, err, storage.ErrNotFound)
	assert.Nil(t, notFound)

	err = store.DeleteMovieMetadata(ctx, id)
	assert.NoError(t, err)

	metadata, err = store.ListMovieMetadata(ctx)
	assert.NoError(t, err)
	assert.Empty(t, metadata)
}

func TestSQLite_UpdateMovieMetadata(t *testing.T) {
	t.Run("updates editable fields and preserves id, tmdb id, and last info sync", func(t *testing.T) {
		ctx := t.Context()
		store := initSqlite(t, ctx)
		require.NotNil(t, store)

		original := model.MovieMetadata{
			ID:      1,
			TmdbID:  42,
			Title:   "Original",
			Images:  "poster.png",
			Runtime: 100,
		}
		id, err := store.CreateMovieMetadata(ctx, original)
		require.NoError(t, err)
		require.Equal(t, int64(1), id)

		created, err := store.GetMovieMetadataByID(ctx, 1)
		require.NoError(t, err)
		require.Nil(t, created.LastInfoSync, "create leaves last info sync as database default (NULL)")

		updated := *created
		updated.Title = "Updated"
		updated.TmdbID = 9999 // must not be persisted
		newSync := time.Now().Add(time.Hour)
		updated.LastInfoSync = &newSync // must not be persisted

		err = store.UpdateMovieMetadata(ctx, updated)
		require.NoError(t, err)

		got, err := store.GetMovieMetadataByID(ctx, 1)
		require.NoError(t, err)
		assert.Equal(t, "Updated", got.Title)
		assert.Equal(t, int32(42), got.TmdbID, "tmdb id must be preserved")
		assert.Nil(t, got.LastInfoSync, "last info sync must be preserved (not updated)")
	})

	t.Run("no error for non-existent metadata", func(t *testing.T) {
		ctx := t.Context()
		store := initSqlite(t, ctx)
		require.NotNil(t, store)

		err := store.UpdateMovieMetadata(ctx, model.MovieMetadata{ID: 123, TmdbID: 1, Title: "missing"})
		assert.NoError(t, err)
	})
}
