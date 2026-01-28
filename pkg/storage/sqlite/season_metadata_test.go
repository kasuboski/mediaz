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

func TestSeasonMetadataStorage(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	assert.NotNil(t, store)

	// Test creating season metadata
	metadata := model.SeasonMetadata{
		TmdbID:   12345,
		Title:    "Season 1",
		Overview: ptr("Test season overview"),
		Number:   1,
	}

	id, err := store.CreateSeasonMetadata(ctx, metadata)
	assert.Nil(t, err)
	assert.Greater(t, id, int64(0))

	// Test getting the season metadata
	where := table.SeasonMetadata.ID.EQ(sqlite.Int64(id))
	retrieved, err := store.GetSeasonMetadata(ctx, where)
	assert.Nil(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, metadata.TmdbID, retrieved.TmdbID)
	assert.Equal(t, metadata.Title, retrieved.Title)
	assert.Equal(t, metadata.Overview, retrieved.Overview)
	assert.Equal(t, metadata.Number, retrieved.Number)

	// Test listing season metadata
	metadataList, err := store.ListSeasonMetadata(ctx)
	assert.Nil(t, err)
	assert.Len(t, metadataList, 1)
	assert.Equal(t, metadata.Title, metadataList[0].Title)

	// Test deleting the season metadata
	err = store.DeleteSeasonMetadata(ctx, id)
	assert.Nil(t, err)

	// Verify deletion
	metadataList, err = store.ListSeasonMetadata(ctx)
	assert.Nil(t, err)
	assert.Empty(t, metadataList)

	// Test getting non-existent season metadata
	_, err = store.GetSeasonMetadata(ctx, where)
	assert.ErrorIs(t, err, storage.ErrNotFound)
}
