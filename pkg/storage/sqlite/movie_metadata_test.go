package sqlite

import (
	"context"
	"testing"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMovieMetadataStorage(t *testing.T) {
	ctx := context.Background()
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

	one, err := store.GetMovieMetadata(ctx, table.MovieMetadata.TmdbID.EQ(sqlite.Int(1234)))
	assert.NoError(t, err)
	assert.NotNil(t, one)

	notFound, err := store.GetMovieMetadata(ctx, table.MovieMetadata.TmdbID.EQ(sqlite.Int(124)))
	assert.ErrorIs(t, err, storage.ErrNotFound)
	assert.Nil(t, notFound)

	err = store.DeleteMovieMetadata(ctx, id)
	assert.NoError(t, err)

	metadata, err = store.ListMovieMetadata(ctx)
	assert.NoError(t, err)
	assert.Empty(t, metadata)
}
