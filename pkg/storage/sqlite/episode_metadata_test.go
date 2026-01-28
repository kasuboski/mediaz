package sqlite

import (
	"context"
	"testing"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
	"github.com/stretchr/testify/assert"
)

func TestEpisodeMetadataStorage(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	assert.NotNil(t, store)

	// Test creating episode metadata
	metadata := model.EpisodeMetadata{
		TmdbID:   12345,
		Title:    "Test Episode",
		Overview: ptr("Test episode overview"),
		Runtime:  ptr(int32(45)),
	}

	id, err := store.CreateEpisodeMetadata(ctx, metadata)
	assert.Nil(t, err)
	assert.Greater(t, id, int64(0))

	// Test getting the episode metadata
	where := table.EpisodeMetadata.ID.EQ(sqlite.Int64(id))
	retrieved, err := store.GetEpisodeMetadata(ctx, where)
	assert.Nil(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, metadata.TmdbID, retrieved.TmdbID)
	assert.Equal(t, metadata.Title, retrieved.Title)
	assert.Equal(t, metadata.Overview, retrieved.Overview)
	assert.Equal(t, metadata.Runtime, retrieved.Runtime)

	// Test listing episode metadata
	metadataList, err := store.ListEpisodeMetadata(ctx)
	assert.Nil(t, err)
	assert.Len(t, metadataList, 1)
	assert.Equal(t, metadata.Title, metadataList[0].Title)

	// Test deleting the episode metadata
	err = store.DeleteEpisodeMetadata(ctx, id)
	assert.Nil(t, err)

	// Verify deletion
	metadataList, err = store.ListEpisodeMetadata(ctx)
	assert.Nil(t, err)
	assert.Empty(t, metadataList)

	// Test getting non-existent episode metadata
	_, err = store.GetEpisodeMetadata(ctx, where)
	assert.ErrorIs(t, err, storage.ErrNotFound)
}
