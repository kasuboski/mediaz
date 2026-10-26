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

func TestSeriesStorage(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	require.NotNil(t, store)

	seriesList, err := store.ListSeries(ctx, storage.SeriesFilter{})
	assert.NoError(t, err)
	assert.Empty(t, seriesList)

	added := time.Now()
	series := storage.Series{
		Series: model.Series{
			Monitored:        1,
			QualityProfileID: 1,
			Added:            &added,
		},
	}

	id, err := store.CreateSeries(ctx, series, storage.SeriesStateMissing)
	assert.NoError(t, err)

	// Test getting the Series
	retrieved, err := store.GetSeriesByID(ctx, id)
	assert.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, series.Monitored, retrieved.Monitored)
	assert.Equal(t, series.QualityProfileID, retrieved.QualityProfileID)

	// Test listing series
	seriesList, err = store.ListSeries(ctx, storage.SeriesFilter{})
	assert.NoError(t, err)
	assert.Len(t, seriesList, 1)
	assert.Equal(t, series.Monitored, seriesList[0].Monitored)

	// Test deleting the Series
	err = store.DeleteSeries(ctx, id)
	assert.NoError(t, err)

	// Verify deletion
	seriesList, err = store.ListSeries(ctx, storage.SeriesFilter{})
	assert.NoError(t, err)
	assert.Empty(t, seriesList)

	// Test getting non-existent Series
	_, err = store.GetSeriesByID(ctx, id)
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestSeriesStorage_CreateAddedDefault(t *testing.T) {
	ctx := context.Background()

	t.Run("nil added falls back to database default", func(t *testing.T) {
		store := initSqlite(t, ctx)
		require.NotNil(t, store)

		id, err := store.CreateSeries(ctx, storage.Series{
			Series: model.Series{
				Monitored:        1,
				QualityProfileID: 1,
			},
		}, storage.SeriesStateMissing)
		require.NoError(t, err)

		found, err := store.GetSeriesByID(ctx, id)
		require.NoError(t, err)
		require.NotNil(t, found.Added)
		assert.False(t, found.Added.IsZero())
		assert.WithinDuration(t, time.Now().UTC(), found.Added.UTC(), time.Minute)
	})

	t.Run("zero added falls back to database default", func(t *testing.T) {
		store := initSqlite(t, ctx)
		require.NotNil(t, store)

		id, err := store.CreateSeries(ctx, storage.Series{
			Series: model.Series{
				Monitored:        1,
				QualityProfileID: 1,
				Added:            ptr.To(time.Time{}),
			},
		}, storage.SeriesStateMissing)
		require.NoError(t, err)

		found, err := store.GetSeriesByID(ctx, id)
		require.NoError(t, err)
		require.NotNil(t, found.Added)
		assert.False(t, found.Added.IsZero())
		assert.WithinDuration(t, time.Now().UTC(), found.Added.UTC(), time.Minute)
	})

	t.Run("explicit added is preserved", func(t *testing.T) {
		store := initSqlite(t, ctx)
		require.NotNil(t, store)

		added := time.Now().Add(-48 * time.Hour).UTC().Truncate(time.Second)
		id, err := store.CreateSeries(ctx, storage.Series{
			Series: model.Series{
				Monitored:        1,
				QualityProfileID: 1,
				Added:            &added,
			},
		}, storage.SeriesStateMissing)
		require.NoError(t, err)

		found, err := store.GetSeriesByID(ctx, id)
		require.NoError(t, err)
		require.NotNil(t, found.Added)
		assert.True(t, added.Equal(*found.Added), "expected added %v, got %v", added, *found.Added)
	})
}

func TestSeriesStorage_GetByMetadataIDAndPath(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	require.NotNil(t, store)

	id, err := store.CreateSeries(ctx, storage.Series{
		Series: model.Series{
			Path:             ptr.To("/library/tv/test"),
			Monitored:        1,
			QualityProfileID: 1,
			SeriesMetadataID: ptr.To(int32(42)),
		},
	}, storage.SeriesStateDiscovered)
	require.NoError(t, err)

	byMetadata, err := store.GetSeriesByMetadataID(ctx, 42)
	require.NoError(t, err)
	require.NotNil(t, byMetadata)
	assert.Equal(t, id, int64(byMetadata.ID))
	assert.Equal(t, ptr.To(int32(42)), byMetadata.SeriesMetadataID)

	byPath, err := store.GetSeriesByPath(ctx, "/library/tv/test")
	require.NoError(t, err)
	require.NotNil(t, byPath)
	assert.Equal(t, id, int64(byPath.ID))

	// No match for a different metadata id or path
	_, err = store.GetSeriesByMetadataID(ctx, 9999)
	assert.ErrorIs(t, err, storage.ErrNotFound)

	_, err = store.GetSeriesByPath(ctx, "/library/tv/missing")
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestSeriesStorage_ListFilters(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	require.NotNil(t, store)

	monitoredMissing, err := store.CreateSeries(ctx, storage.Series{
		Series: model.Series{Monitored: 1, QualityProfileID: 1},
	}, storage.SeriesStateMissing)
	require.NoError(t, err)

	unmonitoredDiscovered, err := store.CreateSeries(ctx, storage.Series{
		Series: model.Series{Monitored: 0, QualityProfileID: 1},
	}, storage.SeriesStateDiscovered)
	require.NoError(t, err)

	monitoredDownloading, err := store.CreateSeries(ctx, storage.Series{
		Series: model.Series{Monitored: 1, QualityProfileID: 1},
	}, storage.SeriesStateMissing)
	require.NoError(t, err)
	// The machine only allows new -> missing/discovered/unreleased on create,
	// so downloading is reached with a state update (missing -> downloading).
	require.NoError(t, store.UpdateSeriesState(ctx, monitoredDownloading, storage.SeriesStateDownloading, nil))

	t.Run("zero filter matches all", func(t *testing.T) {
		list, err := store.ListSeries(ctx, storage.SeriesFilter{})
		require.NoError(t, err)
		assert.Len(t, list, 3)
	})

	t.Run("monitored filter", func(t *testing.T) {
		list, err := store.ListSeries(ctx, storage.SeriesFilter{Monitored: ptr.To(true)})
		require.NoError(t, err)
		assert.Len(t, list, 2)
		ids := []int32{list[0].ID, list[1].ID}
		assert.ElementsMatch(t, []int32{int32(monitoredMissing), int32(monitoredDownloading)}, ids)

		list, err = store.ListSeries(ctx, storage.SeriesFilter{Monitored: ptr.To(false)})
		require.NoError(t, err)
		assert.Len(t, list, 1)
		assert.Equal(t, int32(unmonitoredDiscovered), list[0].ID)
	})

	t.Run("states filter", func(t *testing.T) {
		list, err := store.ListSeries(ctx, storage.SeriesFilter{States: []storage.SeriesState{storage.SeriesStateMissing}})
		require.NoError(t, err)
		assert.Len(t, list, 1)
		assert.Equal(t, int32(monitoredMissing), list[0].ID)
		assert.Equal(t, storage.SeriesStateMissing, list[0].State)

		list, err = store.ListSeries(ctx, storage.SeriesFilter{States: []storage.SeriesState{
			storage.SeriesStateMissing,
			storage.SeriesStateDownloading,
		}})
		require.NoError(t, err)
		assert.Len(t, list, 2)

		list, err = store.ListSeries(ctx, storage.SeriesFilter{States: []storage.SeriesState{storage.SeriesStateCompleted}})
		require.NoError(t, err)
		assert.Empty(t, list)
	})

	t.Run("too many states errors", func(t *testing.T) {
		list, err := store.ListSeries(ctx, storage.SeriesFilter{States: []storage.SeriesState{
			storage.SeriesStateMissing,
			storage.SeriesStateDiscovered,
			storage.SeriesStateUnreleased,
			storage.SeriesStateContinuing,
			storage.SeriesStateDownloading,
		}})
		assert.Error(t, err)
		assert.Nil(t, list)
	})
}

func TestSQLite_UpdateSeriesState(t *testing.T) {
	t.Run("update series state", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)
		require.NotNil(t, store)

		series := storage.Series{
			Series: model.Series{
				ID:               1,
				Monitored:        1,
				QualityProfileID: 1,
				Added:            ptr.To(time.Now()),
			},
		}

		seriesID, err := store.CreateSeries(ctx, series, storage.SeriesStateMissing)
		require.NoError(t, err)
		assert.Equal(t, int64(1), seriesID)

		series.ID = int32(seriesID)
		err = store.UpdateSeriesState(ctx, seriesID, storage.SeriesStateDownloading, &storage.TransitionStateMetadata{})
		require.NoError(t, err)

		foundSeries, err := store.GetSeriesByID(ctx, seriesID)
		require.NoError(t, err)
		require.NotNil(t, foundSeries)

		assert.Equal(t, storage.SeriesStateDownloading, foundSeries.State)
		assert.Equal(t, int32(1), foundSeries.Monitored)
		assert.Equal(t, int32(1), foundSeries.QualityProfileID)

		// Test updating to completed state
		err = store.UpdateSeriesState(ctx, seriesID, storage.SeriesStateCompleted, nil)
		require.NoError(t, err)

		foundSeries, err = store.GetSeriesByID(ctx, seriesID)
		require.NoError(t, err)
		require.NotNil(t, foundSeries)
		assert.Equal(t, storage.SeriesStateCompleted, foundSeries.State)

		// Test updating non-existent series
		err = store.UpdateSeriesState(ctx, 999999, storage.SeriesStateMissing, nil)
		assert.ErrorIs(t, err, storage.ErrNotFound)
	})

	t.Run("invalid state transition", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)
		require.NotNil(t, store)

		series := storage.Series{
			Series: model.Series{
				ID:               2,
				Monitored:        1,
				QualityProfileID: 1,
				Added:            ptr.To(time.Now()),
			},
		}

		// Create series in missing state
		seriesID, err := store.CreateSeries(ctx, series, storage.SeriesStateMissing)
		require.NoError(t, err)

		// First, move to downloading state
		err = store.UpdateSeriesState(ctx, seriesID, storage.SeriesStateDownloading, nil)
		require.NoError(t, err)

		// Then move to completed state
		err = store.UpdateSeriesState(ctx, seriesID, storage.SeriesStateCompleted, nil)
		require.NoError(t, err)

		// Now try to transition from completed back to missing (invalid transition)
		// According to the state machine, completed can only go to continuing
		err = store.UpdateSeriesState(ctx, seriesID, storage.SeriesStateMissing, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid state transition")
	})

	t.Run("test state machine transitions", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)
		require.NotNil(t, store)

		series := storage.Series{
			Series: model.Series{
				ID:               3,
				Monitored:        1,
				QualityProfileID: 1,
				Added:            ptr.To(time.Now()),
			},
		}

		// Start with missing state instead of new state to avoid the transition issue
		seriesID, err := store.CreateSeries(ctx, series, storage.SeriesStateMissing)
		require.NoError(t, err)

		// Test valid transition sequence: missing -> downloading -> completed
		err = store.UpdateSeriesState(ctx, seriesID, storage.SeriesStateDownloading, nil)
		require.NoError(t, err)

		foundSeries, err := store.GetSeriesByID(ctx, seriesID)
		require.NoError(t, err)
		assert.Equal(t, storage.SeriesStateDownloading, foundSeries.State)

		err = store.UpdateSeriesState(ctx, seriesID, storage.SeriesStateCompleted, nil)
		require.NoError(t, err)

		foundSeries, err = store.GetSeriesByID(ctx, seriesID)
		require.NoError(t, err)
		assert.Equal(t, storage.SeriesStateCompleted, foundSeries.State)

		// Test transition from completed to continuing (valid)
		err = store.UpdateSeriesState(ctx, seriesID, storage.SeriesStateContinuing, nil)
		require.NoError(t, err)

		foundSeries, err = store.GetSeriesByID(ctx, seriesID)
		require.NoError(t, err)
		assert.Equal(t, storage.SeriesStateContinuing, foundSeries.State)

		// Test transition from continuing to completed (valid)
		err = store.UpdateSeriesState(ctx, seriesID, storage.SeriesStateCompleted, nil)
		require.NoError(t, err)

		foundSeries, err = store.GetSeriesByID(ctx, seriesID)
		require.NoError(t, err)
		assert.Equal(t, storage.SeriesStateCompleted, foundSeries.State)
	})

	t.Run("test transitions from unreleased state", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)
		require.NotNil(t, store)

		series := storage.Series{
			Series: model.Series{
				ID:               4,
				Monitored:        1,
				QualityProfileID: 1,
				Added:            ptr.To(time.Now()),
			},
		}

		// Test valid transitions from unreleased: unreleased -> missing -> downloading
		seriesID, err := store.CreateSeries(ctx, series, storage.SeriesStateUnreleased)
		require.NoError(t, err)

		foundSeries, err := store.GetSeriesByID(ctx, seriesID)
		require.NoError(t, err)
		assert.Equal(t, storage.SeriesStateUnreleased, foundSeries.State)

		err = store.UpdateSeriesState(ctx, seriesID, storage.SeriesStateMissing, nil)
		require.NoError(t, err)

		foundSeries, err = store.GetSeriesByID(ctx, seriesID)
		require.NoError(t, err)
		assert.Equal(t, storage.SeriesStateMissing, foundSeries.State)

		err = store.UpdateSeriesState(ctx, seriesID, storage.SeriesStateDownloading, nil)
		require.NoError(t, err)

		foundSeries, err = store.GetSeriesByID(ctx, seriesID)
		require.NoError(t, err)
		assert.Equal(t, storage.SeriesStateDownloading, foundSeries.State)
	})
}

func TestSeriesStorage_DefaultState(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	require.NotNil(t, store)

	// Pin legacy behavior: the state machine has no new->new transition, so
	// creating a series with an explicit SeriesStateNew initial state errors
	// instead of creating an unvalidated row.
	id, err := store.CreateSeries(ctx, storage.Series{
		Series: model.Series{Monitored: 1, QualityProfileID: 1},
	}, storage.SeriesStateNew)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid state transition")
	assert.Equal(t, int64(0), id)

	list, err := store.ListSeries(ctx, storage.SeriesFilter{})
	require.NoError(t, err)
	assert.Empty(t, list)
}
