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

func TestUpdateSeriesMetadata(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	require.NotNil(t, store)

	// Create initial series metadata
	metadata := model.SeriesMetadata{
		TmdbID:       12345,
		Title:        "Test Series",
		SeasonCount:  1,
		EpisodeCount: 1,
		Status:       "Continuing",
	}

	id, err := store.CreateSeriesMetadata(ctx, metadata)
	require.Nil(t, err)
	require.Greater(t, id, int64(0))

	// Get the initial metadata
	where := table.SeriesMetadata.ID.EQ(sqlite.Int64(id))
	initial, err := store.GetSeriesMetadata(ctx, where)
	require.Nil(t, err)
	require.NotNil(t, initial)

	// Sleep briefly to ensure timestamp will be different
	time.Sleep(10 * time.Millisecond)

	// Update the series metadata
	updated := *initial
	updated.Title = "Updated Test Series"
	updated.SeasonCount = 2
	updated.EpisodeCount = 10
	updated.Status = "Ended"

	err = store.UpdateSeriesMetadata(ctx, updated)
	require.Nil(t, err)

	// Retrieve the updated metadata
	retrieved, err := store.GetSeriesMetadata(ctx, where)
	require.Nil(t, err)
	require.NotNil(t, retrieved)

	// Verify the updates
	assert.Equal(t, "Updated Test Series", retrieved.Title)
	assert.Equal(t, int32(2), retrieved.SeasonCount)
	assert.Equal(t, int32(10), retrieved.EpisodeCount)
	assert.Equal(t, "Ended", retrieved.Status)

	// Verify LastInfoSync was automatically set
	require.NotNil(t, retrieved.LastInfoSync, "LastInfoSync should be set automatically")

	// Verify LastInfoSync is a recent timestamp (within last 5 seconds)
	now := time.Now()
	timeSinceSync := now.Sub(*retrieved.LastInfoSync)
	assert.Less(t, timeSinceSync, 5*time.Second, "LastInfoSync should be recent")
	assert.Greater(t, timeSinceSync, time.Duration(0), "LastInfoSync should be in the past")

	// If initial had a LastInfoSync, verify it was updated
	if initial.LastInfoSync != nil {
		assert.True(t, retrieved.LastInfoSync.After(*initial.LastInfoSync),
			"LastInfoSync should be updated to a newer timestamp")
	}
}
func TestSeriesMetadataStorage(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	assert.NotNil(t, store)

	// Test creating Series metadata
	metadata := model.SeriesMetadata{
		TmdbID:         12345,
		Title:          "Test Series",
		SeasonCount:    1,
		EpisodeCount:   1,
		Status:         "Continuing",
		ExternalIds:    nil, // Optional field
		WatchProviders: nil, // Optional field
	}

	id, err := store.CreateSeriesMetadata(ctx, metadata)
	assert.Nil(t, err)
	assert.Greater(t, id, int64(0))

	// Test getting the Series metadata
	where := table.SeriesMetadata.ID.EQ(sqlite.Int64(id))
	retrieved, err := store.GetSeriesMetadata(ctx, where)
	assert.Nil(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, metadata.TmdbID, retrieved.TmdbID)
	assert.Equal(t, metadata.Title, retrieved.Title)
	assert.Equal(t, metadata.SeasonCount, retrieved.SeasonCount)
	assert.Equal(t, metadata.EpisodeCount, retrieved.EpisodeCount)
	assert.Equal(t, metadata.Status, retrieved.Status)

	// Test listing Series metadata
	metadataList, err := store.ListSeriesMetadata(ctx)
	assert.Nil(t, err)
	assert.Len(t, metadataList, 1)
	assert.Equal(t, metadata.Title, metadataList[0].Title)

	// Test deleting the Series metadata
	err = store.DeleteSeriesMetadata(ctx, id)
	assert.Nil(t, err)

	// Verify deletion
	metadataList, err = store.ListSeriesMetadata(ctx)
	assert.Nil(t, err)
	assert.Empty(t, metadataList)

	// Test getting non-existent Series metadata
	_, err = store.GetSeriesMetadata(ctx, where)
	assert.ErrorIs(t, err, storage.ErrNotFound)
}
