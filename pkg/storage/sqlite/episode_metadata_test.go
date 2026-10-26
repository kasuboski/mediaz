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

func TestEpisodeMetadataStorage(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	assert.NotNil(t, store)

	// Test creating episode metadata
	metadata := model.EpisodeMetadata{
		SeasonMetadataID: 1,
		Number:           1,
		TmdbID:           12345,
		Title:            "Test Episode",
		Overview:         ptr.To("Test episode overview"),
		Runtime:          ptr.To(int32(45)),
	}

	id, err := store.CreateEpisodeMetadata(ctx, metadata)
	assert.Nil(t, err)
	assert.Greater(t, id, int64(0))

	// Test getting the episode metadata
	retrieved, err := store.GetEpisodeMetadataByID(ctx, id)
	assert.Nil(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, metadata.TmdbID, retrieved.TmdbID)
	assert.Equal(t, metadata.SeasonMetadataID, retrieved.SeasonMetadataID)
	assert.Equal(t, metadata.Number, retrieved.Number)
	assert.Equal(t, metadata.Title, retrieved.Title)
	assert.Equal(t, metadata.Overview, retrieved.Overview)
	assert.Equal(t, metadata.Runtime, retrieved.Runtime)

	// Test listing episode metadata for the season metadata
	metadataList, err := store.ListEpisodeMetadataBySeasonMetadataID(ctx, 1)
	assert.Nil(t, err)
	assert.Len(t, metadataList, 1)
	assert.Equal(t, metadata.Title, metadataList[0].Title)

	// Test listing episode metadata for another season metadata
	otherList, err := store.ListEpisodeMetadataBySeasonMetadataID(ctx, 2)
	assert.Nil(t, err)
	assert.Empty(t, otherList)

	// Test deleting the episode metadata
	err = store.DeleteEpisodeMetadata(ctx, id)
	assert.Nil(t, err)

	// Verify deletion
	metadataList, err = store.ListEpisodeMetadataBySeasonMetadataID(ctx, 1)
	assert.Nil(t, err)
	assert.Empty(t, metadataList)

	// Test getting non-existent episode metadata
	_, err = store.GetEpisodeMetadataByID(ctx, id)
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestGetEpisodeMetadataByTmdbID(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	require.NotNil(t, store)

	metadata := model.EpisodeMetadata{
		SeasonMetadataID: 2,
		Number:           5,
		TmdbID:           424242,
		Title:            "TMDB Episode",
	}

	id, err := store.CreateEpisodeMetadata(ctx, metadata)
	require.NoError(t, err)
	require.Greater(t, id, int64(0))

	retrieved, err := store.GetEpisodeMetadataByTmdbID(ctx, int64(metadata.TmdbID))
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, id, int64(retrieved.ID))
	assert.Equal(t, metadata.SeasonMetadataID, retrieved.SeasonMetadataID)
	assert.Equal(t, metadata.Number, retrieved.Number)
	assert.Equal(t, metadata.Title, retrieved.Title)

	// Test getting non-existent episode metadata
	_, err = store.GetEpisodeMetadataByTmdbID(ctx, 1)
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestGetEpisodeMetadataBySeasonAndNumber(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	require.NotNil(t, store)

	metadata := model.EpisodeMetadata{
		SeasonMetadataID: 3,
		Number:           7,
		TmdbID:           9,
		Title:            "S1E7",
	}

	_, err := store.CreateEpisodeMetadata(ctx, metadata)
	require.NoError(t, err)

	retrieved, err := store.GetEpisodeMetadataBySeasonAndNumber(ctx, 3, 7)
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, metadata.TmdbID, retrieved.TmdbID)
	assert.Equal(t, metadata.Title, retrieved.Title)

	// wrong number for the season
	_, err = store.GetEpisodeMetadataBySeasonAndNumber(ctx, 3, 8)
	assert.ErrorIs(t, err, storage.ErrNotFound)

	// right number, wrong season
	_, err = store.GetEpisodeMetadataBySeasonAndNumber(ctx, 4, 7)
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestListEpisodeMetadataByIDs(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	require.NotNil(t, store)

	first := model.EpisodeMetadata{SeasonMetadataID: 1, Number: 1, TmdbID: 101, Title: "One"}
	second := model.EpisodeMetadata{SeasonMetadataID: 1, Number: 2, TmdbID: 102, Title: "Two"}
	third := model.EpisodeMetadata{SeasonMetadataID: 1, Number: 3, TmdbID: 103, Title: "Three"}

	firstID, err := store.CreateEpisodeMetadata(ctx, first)
	require.NoError(t, err)
	_, err = store.CreateEpisodeMetadata(ctx, second)
	require.NoError(t, err)
	thirdID, err := store.CreateEpisodeMetadata(ctx, third)
	require.NoError(t, err)

	list, err := store.ListEpisodeMetadataByIDs(ctx, []int64{firstID, thirdID})
	require.NoError(t, err)
	require.Len(t, list, 2)
	titles := []string{list[0].Title, list[1].Title}
	assert.ElementsMatch(t, []string{"One", "Three"}, titles)

	list, err = store.ListEpisodeMetadataByIDs(ctx, []int64{int64(firstID)})
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "One", list[0].Title)

	list, err = store.ListEpisodeMetadataByIDs(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestCreateEpisodeMetadataUpsertConflict(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	require.NotNil(t, store)

	// Episode metadata conflicting creates only update the title, matching
	// the legacy DO UPDATE SET title upsert.
	original := model.EpisodeMetadata{
		SeasonMetadataID: 1,
		Number:           1,
		TmdbID:           11,
		Title:            "Original",
		Overview:         ptr.To("keep me"),
		Runtime:          ptr.To(int32(30)),
	}

	id, err := store.CreateEpisodeMetadata(ctx, original)
	require.NoError(t, err)
	require.Greater(t, id, int64(0))

	conflicting := model.EpisodeMetadata{
		SeasonMetadataID: 2,
		Number:           9,
		TmdbID:           11,
		Title:            "Updated",
		Overview:         ptr.To("CHANGED"),
		Runtime:          ptr.To(int32(99)),
	}

	upsertID, err := store.CreateEpisodeMetadata(ctx, conflicting)
	require.NoError(t, err)
	assert.Equal(t, id, upsertID, "conflicting create must return the existing id")

	retrieved, err := store.GetEpisodeMetadataByTmdbID(ctx, 11)
	require.NoError(t, err)
	assert.Equal(t, "Updated", retrieved.Title, "title must be updated on conflict")
	assert.Equal(t, int32(1), retrieved.SeasonMetadataID, "season metadata link must be preserved on conflict")
	assert.Equal(t, int32(1), retrieved.Number, "number must be preserved on conflict")
	require.NotNil(t, retrieved.Overview)
	assert.Equal(t, "keep me", *retrieved.Overview, "overview must be preserved on conflict")
	require.NotNil(t, retrieved.Runtime)
	assert.Equal(t, int32(30), *retrieved.Runtime, "runtime must be preserved on conflict")

	rows, err := store.ListEpisodeMetadataBySeasonMetadataID(ctx, 1)
	require.NoError(t, err)
	assert.Len(t, rows, 1, "conflicting create must not add a second row")
}
