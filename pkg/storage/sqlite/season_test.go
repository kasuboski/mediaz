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

func TestSeasonStorage(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	assert.NotNil(t, store)

	series := storage.Series{
		Series: model.Series{
			Monitored:        1,
			QualityProfileID: 1,
			Added:            ptr(time.Now()),
		},
	}

	// Create a Series first
	SeriesID, err := store.CreateSeries(ctx, series, storage.SeriesStateMissing)
	require.Nil(t, err)

	// Test creating a season
	season := storage.Season{
		Season: model.Season{
			SeriesID: int32(SeriesID),
		},
	}

	id, err := store.CreateSeason(ctx, season, storage.SeasonStateMissing)
	assert.Nil(t, err)
	assert.Greater(t, id, int64(0))

	// Test getting the season
	retrieved, err := store.GetSeason(ctx, table.Season.ID.EQ(sqlite.Int64(id)))
	assert.Nil(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, season.SeriesID, retrieved.SeriesID)
	assert.Equal(t, storage.SeasonStateMissing, retrieved.State)

	// Test listing seasons
	seasons, err := store.ListSeasons(ctx, table.Season.SeriesID.EQ(sqlite.Int64(SeriesID)))
	assert.Nil(t, err)
	assert.Len(t, seasons, 1)
	assert.Equal(t, season.SeriesID, seasons[0].SeriesID)
	assert.Equal(t, storage.SeasonStateMissing, seasons[0].State)

	// Test deleting the season
	err = store.DeleteSeason(ctx, id)
	assert.Nil(t, err)

	// Verify deletion
	seasons, err = store.ListSeasons(ctx, table.Season.SeriesID.EQ(sqlite.Int64(SeriesID)))
	assert.Nil(t, err)
	assert.Empty(t, seasons)

	// Test getting non-existent season
	_, err = store.GetSeason(ctx, table.Season.ID.EQ(sqlite.Int64(id)))
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestSQLite_UpdateSeasonState(t *testing.T) {
	t.Run("update season state", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)
		require.NotNil(t, store)

		season := storage.Season{
			Season: model.Season{
				ID:               1,
				SeriesID:         1,
				SeasonMetadataID: ptr(int32(1)),
				Monitored:        1,
			},
		}

		seasonID, err := store.CreateSeason(ctx, season, storage.SeasonStateMissing)
		require.NoError(t, err)
		assert.Equal(t, int64(1), seasonID)

		season.ID = int32(seasonID)
		err = store.UpdateSeasonState(ctx, seasonID, storage.SeasonStateDownloading, &storage.TransitionStateMetadata{
			DownloadID:             ptr("123"),
			DownloadClientID:       ptr(int32(2)),
			IsEntireSeasonDownload: ptr(true),
		})
		require.NoError(t, err)

		foundSeason, err := store.GetSeason(ctx, table.Season.ID.EQ(sqlite.Int64(seasonID)))
		require.NoError(t, err)
		require.NotNil(t, foundSeason)

		assert.Equal(t, storage.SeasonStateDownloading, foundSeason.State)
		assert.Equal(t, "123", foundSeason.DownloadID)
		assert.Equal(t, int32(1), foundSeason.SeriesID)
		assert.Equal(t, ptr(int32(1)), foundSeason.SeasonMetadataID)

		err = store.UpdateSeasonState(ctx, 999999, storage.SeasonStateMissing, nil)
		assert.ErrorIs(t, err, storage.ErrNotFound)
	})
}
