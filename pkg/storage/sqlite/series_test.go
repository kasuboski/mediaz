package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeriesStorage(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	require.NotNil(t, store)

	seriesList, err := store.ListSeries(ctx)
	assert.Nil(t, err)
	assert.Empty(t, seriesList)

	series := storage.Series{
		Series: model.Series{
			Monitored:        1,
			QualityProfileID: 1,
			Added:            ptr(time.Now()),
		},
	}

	id, err := store.CreateSeries(ctx, series, storage.SeriesStateMissing)
	assert.Nil(t, err)

	// Test getting the Series
	retrieved, err := store.GetSeries(ctx, table.Series.ID.EQ(sqlite.Int64(id)))
	assert.Nil(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, series.Monitored, retrieved.Monitored)
	assert.Equal(t, series.QualityProfileID, retrieved.QualityProfileID)

	// Test listing series
	seriesList, err = store.ListSeries(ctx)
	assert.Nil(t, err)
	assert.Len(t, seriesList, 1)
	assert.Equal(t, series.Monitored, seriesList[0].Monitored)

	// Test deleting the Series
	err = store.DeleteSeries(ctx, id)
	assert.Nil(t, err)

	// Verify deletion
	seriesList, err = store.ListSeries(ctx)
	assert.Nil(t, err)
	assert.Empty(t, seriesList)

	// Test getting non-existent Series
	_, err = store.GetSeries(ctx, table.Series.ID.EQ(sqlite.Int64(id)))
	assert.ErrorIs(t, err, storage.ErrNotFound)
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
				Added:            ptr(time.Now()),
			},
		}

		seriesID, err := store.CreateSeries(ctx, series, storage.SeriesStateMissing)
		require.NoError(t, err)
		assert.Equal(t, int64(1), seriesID)

		series.ID = int32(seriesID)
		err = store.UpdateSeriesState(ctx, seriesID, storage.SeriesStateDownloading, &storage.TransitionStateMetadata{})
		require.NoError(t, err)

		foundSeries, err := store.GetSeries(ctx, table.Series.ID.EQ(sqlite.Int64(seriesID)))
		require.NoError(t, err)
		require.NotNil(t, foundSeries)

		assert.Equal(t, storage.SeriesStateDownloading, foundSeries.State)
		assert.Equal(t, int32(1), foundSeries.Monitored)
		assert.Equal(t, int32(1), foundSeries.QualityProfileID)

		// Test updating to completed state
		err = store.UpdateSeriesState(ctx, seriesID, storage.SeriesStateCompleted, nil)
		require.NoError(t, err)

		foundSeries, err = store.GetSeries(ctx, table.Series.ID.EQ(sqlite.Int64(seriesID)))
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
				Added:            ptr(time.Now()),
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
		// According to the state machine, completed can only go to continuing or ended
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
				Added:            ptr(time.Now()),
			},
		}

		// Start with missing state instead of new state to avoid the transition issue
		seriesID, err := store.CreateSeries(ctx, series, storage.SeriesStateMissing)
		require.NoError(t, err)

		// Test valid transition sequence: missing -> downloading -> completed
		err = store.UpdateSeriesState(ctx, seriesID, storage.SeriesStateDownloading, nil)
		require.NoError(t, err)

		foundSeries, err := store.GetSeries(ctx, table.Series.ID.EQ(sqlite.Int64(seriesID)))
		require.NoError(t, err)
		assert.Equal(t, storage.SeriesStateDownloading, foundSeries.State)

		err = store.UpdateSeriesState(ctx, seriesID, storage.SeriesStateCompleted, nil)
		require.NoError(t, err)

		foundSeries, err = store.GetSeries(ctx, table.Series.ID.EQ(sqlite.Int64(seriesID)))
		require.NoError(t, err)
		assert.Equal(t, storage.SeriesStateCompleted, foundSeries.State)

		// Test transition from completed to continuing (valid)
		err = store.UpdateSeriesState(ctx, seriesID, storage.SeriesStateContinuing, nil)
		require.NoError(t, err)

		foundSeries, err = store.GetSeries(ctx, table.Series.ID.EQ(sqlite.Int64(seriesID)))
		require.NoError(t, err)
		assert.Equal(t, storage.SeriesStateContinuing, foundSeries.State)

		// Test transition from continuing to completed (valid)
		err = store.UpdateSeriesState(ctx, seriesID, storage.SeriesStateCompleted, nil)
		require.NoError(t, err)

		foundSeries, err = store.GetSeries(ctx, table.Series.ID.EQ(sqlite.Int64(seriesID)))
		require.NoError(t, err)
		assert.Equal(t, storage.SeriesStateCompleted, foundSeries.State)
	})

	t.Run("test transitions from new state", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)
		require.NotNil(t, store)

		series := storage.Series{
			Series: model.Series{
				ID:               4,
				Monitored:        1,
				QualityProfileID: 1,
				Added:            ptr(time.Now()),
			},
		}

		// Test valid transitions from unreleased: unreleased -> missing -> downloading
		seriesID, err := store.CreateSeries(ctx, series, storage.SeriesStateUnreleased)
		require.NoError(t, err)

		foundSeries, err := store.GetSeries(ctx, table.Series.ID.EQ(sqlite.Int64(seriesID)))
		require.NoError(t, err)
		assert.Equal(t, storage.SeriesStateUnreleased, foundSeries.State)

		err = store.UpdateSeriesState(ctx, seriesID, storage.SeriesStateMissing, nil)
		require.NoError(t, err)

		foundSeries, err = store.GetSeries(ctx, table.Series.ID.EQ(sqlite.Int64(seriesID)))
		require.NoError(t, err)
		assert.Equal(t, storage.SeriesStateMissing, foundSeries.State)

		err = store.UpdateSeriesState(ctx, seriesID, storage.SeriesStateDownloading, nil)
		require.NoError(t, err)

		foundSeries, err = store.GetSeries(ctx, table.Series.ID.EQ(sqlite.Int64(seriesID)))
		require.NoError(t, err)
		assert.Equal(t, storage.SeriesStateDownloading, foundSeries.State)
	})
}
