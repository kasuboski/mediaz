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

func TestSeriesMetadataStorage(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	require.NotNil(t, store)

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
	retrieved, err := store.GetSeriesMetadataByID(ctx, id)
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
	_, err = store.GetSeriesMetadataByID(ctx, id)
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestGetSeriesMetadataByTmdbID(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	require.NotNil(t, store)

	metadata := model.SeriesMetadata{
		TmdbID:       98765,
		Title:        "TMDB Lookup Series",
		SeasonCount:  2,
		EpisodeCount: 20,
		Status:       "Ended",
	}

	id, err := store.CreateSeriesMetadata(ctx, metadata)
	require.NoError(t, err)
	require.Greater(t, id, int64(0))

	retrieved, err := store.GetSeriesMetadataByTmdbID(ctx, int64(metadata.TmdbID))
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, id, int64(retrieved.ID))
	assert.Equal(t, metadata.Title, retrieved.Title)
	assert.Equal(t, metadata.SeasonCount, retrieved.SeasonCount)
	assert.Equal(t, metadata.EpisodeCount, retrieved.EpisodeCount)
	assert.Equal(t, metadata.Status, retrieved.Status)

	// Test getting non-existent series metadata
	_, err = store.GetSeriesMetadataByTmdbID(ctx, 1)
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestListSeriesMetadataByStatus(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	require.NotNil(t, store)

	continuing := model.SeriesMetadata{
		TmdbID:       1,
		Title:        "Continuing Series",
		SeasonCount:  1,
		EpisodeCount: 1,
		Status:       "Continuing",
	}
	ended := model.SeriesMetadata{
		TmdbID:       2,
		Title:        "Ended Series",
		SeasonCount:  1,
		EpisodeCount: 1,
		Status:       "Ended",
	}

	_, err := store.CreateSeriesMetadata(ctx, continuing)
	require.NoError(t, err)
	_, err = store.CreateSeriesMetadata(ctx, ended)
	require.NoError(t, err)

	list, err := store.ListSeriesMetadataByStatus(ctx, "Continuing")
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, continuing.Title, list[0].Title)

	list, err = store.ListSeriesMetadataByStatus(ctx, "Ended")
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, ended.Title, list[0].Title)

	list, err = store.ListSeriesMetadataByStatus(ctx, "Upcoming")
	require.NoError(t, err)
	assert.Empty(t, list)
}

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
	require.NoError(t, err)
	require.Greater(t, id, int64(0))

	// Get the initial metadata
	initial, err := store.GetSeriesMetadataByID(ctx, id)
	require.NoError(t, err)
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
	require.NoError(t, err)

	// Retrieve the updated metadata
	retrieved, err := store.GetSeriesMetadataByID(ctx, id)
	require.NoError(t, err)
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

func TestCreateSeriesMetadataUpsert(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	require.NotNil(t, store)

	original := model.SeriesMetadata{
		TmdbID:       42,
		Title:        "Original Title",
		Overview:     ptr.To("original overview"),
		SeasonCount:  1,
		EpisodeCount: 5,
		Status:       "Continuing",
		PosterPath:   ptr.To("/original.jpg"),
	}

	id, err := store.CreateSeriesMetadata(ctx, original)
	require.NoError(t, err)
	require.Greater(t, id, int64(0))

	time.Sleep(10 * time.Millisecond)

	// Creating metadata with the same tmdb id updates only the legacy
	// conflict columns: title, last_info_sync, season_count, episode_count
	// and status. Everything else must be preserved.
	updated := model.SeriesMetadata{
		TmdbID:       42,
		Title:        "Updated Title",
		SeasonCount:  3,
		EpisodeCount: 30,
		Status:       "Ended",
	}

	upsertID, err := store.CreateSeriesMetadata(ctx, updated)
	require.NoError(t, err)
	assert.Equal(t, id, upsertID, "upsert must return the existing metadata id")

	retrieved, err := store.GetSeriesMetadataByTmdbID(ctx, 42)
	require.NoError(t, err)
	require.NotNil(t, retrieved)

	assert.Equal(t, "Updated Title", retrieved.Title)
	assert.Equal(t, int32(3), retrieved.SeasonCount)
	assert.Equal(t, int32(30), retrieved.EpisodeCount)
	assert.Equal(t, "Ended", retrieved.Status)

	assert.NotNil(t, retrieved.LastInfoSync, "LastInfoSync should be refreshed on conflict")
	assert.WithinDuration(t, time.Now().UTC(), *retrieved.LastInfoSync, 5*time.Second)

	assert.NotNil(t, retrieved.Overview)
	assert.Equal(t, "original overview", *retrieved.Overview, "overview must be preserved on conflict")
	assert.NotNil(t, retrieved.PosterPath)
	assert.Equal(t, "/original.jpg", *retrieved.PosterPath, "poster path must be preserved on conflict")

	count, err := store.ListSeriesMetadata(ctx)
	require.NoError(t, err)
	assert.Len(t, count, 1, "conflicting create must not add a second row")
}

func TestCreateSeriesMetadataWithExplicitID(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	require.NotNil(t, store)

	metadata := model.SeriesMetadata{
		ID:           777,
		TmdbID:       99,
		Title:        "Explicit ID Series",
		SeasonCount:  1,
		EpisodeCount: 1,
		Status:       "Continuing",
	}

	id, err := store.CreateSeriesMetadata(ctx, metadata)
	require.NoError(t, err)
	assert.Equal(t, int64(777), id)

	retrieved, err := store.GetSeriesMetadataByID(ctx, 777)
	require.NoError(t, err)
	assert.Equal(t, int32(99), retrieved.TmdbID)
	assert.Equal(t, "Explicit ID Series", retrieved.Title)
}

func TestUpdateSeriesState(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	require.NotNil(t, store)

	series := storage.Series{
		Series: model.Series{
			Monitored:        1,
			QualityProfileID: 1,
		},
	}

	seriesID, err := store.CreateSeries(ctx, series, storage.SeriesStateMissing)
	require.NoError(t, err)
	require.Greater(t, seriesID, int64(0))

	// valid transition: missing -> discovered
	err = store.UpdateSeriesState(ctx, seriesID, storage.SeriesStateDiscovered, nil)
	require.NoError(t, err)

	retrieved, err := store.GetSeriesByID(ctx, seriesID)
	require.NoError(t, err)
	assert.Equal(t, storage.SeriesStateDiscovered, retrieved.State)

	// another valid transition: discovered -> missing
	err = store.UpdateSeriesState(ctx, seriesID, storage.SeriesStateMissing, nil)
	require.NoError(t, err)

	retrieved, err = store.GetSeriesByID(ctx, seriesID)
	require.NoError(t, err)
	assert.Equal(t, storage.SeriesStateMissing, retrieved.State)

	// invalid transition: missing -> completed is not allowed
	err = store.UpdateSeriesState(ctx, seriesID, storage.SeriesStateCompleted, nil)
	assert.Error(t, err)

	// unknown series
	err = store.UpdateSeriesState(ctx, 999999, storage.SeriesStateMissing, nil)
	assert.Error(t, err)
}

func TestUpdateSeriesByID(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	require.NotNil(t, store)

	series := storage.Series{
		Series: model.Series{
			Monitored:        1,
			QualityProfileID: 1,
		},
	}

	seriesID, err := store.CreateSeries(ctx, series, storage.SeriesStateMissing)
	require.NoError(t, err)

	err = store.UpdateSeriesByID(ctx, seriesID, model.Series{Monitored: 0})
	require.NoError(t, err)

	retrieved, err := store.GetSeriesByID(ctx, seriesID)
	require.NoError(t, err)
	assert.Equal(t, int32(0), retrieved.Monitored)

	// unknown id updates nothing and does not error, like the legacy update
	err = store.UpdateSeriesByID(ctx, 999999, model.Series{Monitored: 1})
	assert.NoError(t, err)
}

func TestLinkSeriesMetadata(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	require.NotNil(t, store)

	series := storage.Series{
		Series: model.Series{
			Monitored:        1,
			QualityProfileID: 1,
		},
	}

	seriesID, err := store.CreateSeries(ctx, series, storage.SeriesStateMissing)
	require.NoError(t, err)

	metadataID, err := store.CreateSeriesMetadata(ctx, model.SeriesMetadata{
		TmdbID:       5150,
		Title:        "Linked Series",
		SeasonCount:  1,
		EpisodeCount: 1,
		Status:       "Continuing",
	})
	require.NoError(t, err)

	err = store.LinkSeriesMetadata(ctx, seriesID, int32(metadataID))
	require.NoError(t, err)

	retrieved, err := store.GetSeriesByID(ctx, seriesID)
	require.NoError(t, err)
	require.NotNil(t, retrieved.SeriesMetadataID)
	assert.Equal(t, metadataID, int64(*retrieved.SeriesMetadataID))
}
