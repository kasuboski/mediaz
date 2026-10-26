package sqlite

import (
	"context"
	"testing"

	"github.com/kasuboski/mediaz/pkg/ptr"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeasonMetadataStorage(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	assert.NotNil(t, store)

	// Test creating season metadata
	metadata := model.SeasonMetadata{
		SeriesMetadataID: 1,
		TmdbID:           12345,
		Title:            "Season 1",
		Overview:         ptr.To("Test season overview"),
		Number:           1,
	}

	id, err := store.CreateSeasonMetadata(ctx, metadata)
	assert.Nil(t, err)
	assert.Greater(t, id, int64(0))

	// Test getting the season metadata
	retrieved, err := store.GetSeasonMetadataByID(ctx, id)
	assert.Nil(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, metadata.TmdbID, retrieved.TmdbID)
	assert.Equal(t, metadata.SeriesMetadataID, retrieved.SeriesMetadataID)
	assert.Equal(t, metadata.Title, retrieved.Title)
	assert.Equal(t, metadata.Overview, retrieved.Overview)
	assert.Equal(t, metadata.Number, retrieved.Number)

	// Test listing season metadata for the series metadata
	metadataList, err := store.ListSeasonMetadataBySeriesMetadataID(ctx, 1)
	assert.Nil(t, err)
	assert.Len(t, metadataList, 1)
	assert.Equal(t, metadata.Title, metadataList[0].Title)

	// Test listing season metadata for another series metadata
	otherList, err := store.ListSeasonMetadataBySeriesMetadataID(ctx, 2)
	assert.Nil(t, err)
	assert.Empty(t, otherList)

	// Test deleting the season metadata
	err = store.DeleteSeasonMetadata(ctx, id)
	assert.Nil(t, err)

	// Verify deletion
	metadataList, err = store.ListSeasonMetadataBySeriesMetadataID(ctx, 1)
	assert.Nil(t, err)
	assert.Empty(t, metadataList)

	// Test getting non-existent season metadata
	_, err = store.GetSeasonMetadataByID(ctx, id)
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestGetSeasonMetadataByTmdbID(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	require.NotNil(t, store)

	metadata := model.SeasonMetadata{
		SeriesMetadataID: 3,
		Number:           2,
		TmdbID:           55555,
		Title:            "Season 2",
	}

	id, err := store.CreateSeasonMetadata(ctx, metadata)
	require.NoError(t, err)
	require.Greater(t, id, int64(0))

	retrieved, err := store.GetSeasonMetadataByTmdbID(ctx, int64(metadata.TmdbID))
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, id, int64(retrieved.ID))
	assert.Equal(t, metadata.SeriesMetadataID, retrieved.SeriesMetadataID)
	assert.Equal(t, metadata.Number, retrieved.Number)
	assert.Equal(t, metadata.Title, retrieved.Title)

	// Test getting non-existent season metadata
	_, err = store.GetSeasonMetadataByTmdbID(ctx, 1)
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestCreateSeasonMetadataUpsertConflict(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	require.NotNil(t, store)

	// Season metadata creation is add-only: conflicting tmdb ids keep the
	// existing row untouched, matching the legacy DO NOTHING upsert.
	original := model.SeasonMetadata{
		SeriesMetadataID: 1,
		Number:           1,
		TmdbID:           7,
		Title:            "Season 1",
		Overview:         ptr.To("original overview"),
	}

	id, err := store.CreateSeasonMetadata(ctx, original)
	require.NoError(t, err)
	require.Greater(t, id, int64(0))

	conflicting := model.SeasonMetadata{
		SeriesMetadataID: 2,
		Number:           9,
		TmdbID:           7,
		Title:            "CHANGED",
	}

	upsertID, err := store.CreateSeasonMetadata(ctx, conflicting)
	require.NoError(t, err)
	assert.Equal(t, id, upsertID, "conflicting create must return the existing id")

	retrieved, err := store.GetSeasonMetadataByTmdbID(ctx, 7)
	require.NoError(t, err)
	assert.Equal(t, "Season 1", retrieved.Title, "title must be preserved on conflict")
	assert.Equal(t, int32(1), retrieved.Number, "number must be preserved on conflict")
	assert.Equal(t, int32(1), retrieved.SeriesMetadataID, "series metadata link must be preserved on conflict")
	require.NotNil(t, retrieved.Overview)
	assert.Equal(t, "original overview", *retrieved.Overview, "overview must be preserved on conflict")

	rows, err := store.ListSeasonMetadataBySeriesMetadataID(ctx, 1)
	require.NoError(t, err)
	assert.Len(t, rows, 1, "conflicting create must not add a second row")
}
