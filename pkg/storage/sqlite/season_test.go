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

func TestSeasonStorage(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	assert.NotNil(t, store)

	series := storage.Series{
		Series: model.Series{
			Monitored:        1,
			QualityProfileID: 1,
			Added:            ptr.To(time.Now()),
		},
	}

	// Create a Series first
	seriesID, err := store.CreateSeries(ctx, series, storage.SeriesStateMissing)
	require.NoError(t, err)

	// Test creating a season
	season := storage.Season{
		Season: model.Season{
			SeriesID: int32(seriesID),
		},
	}

	id, err := store.CreateSeason(ctx, season, storage.SeasonStateMissing)
	assert.NoError(t, err)
	assert.Greater(t, id, int64(0))

	// Test getting the season
	retrieved, err := store.GetSeasonByID(ctx, id)
	assert.NoError(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, season.SeriesID, retrieved.SeriesID)
	assert.Equal(t, storage.SeasonStateMissing, retrieved.State)

	// Test listing seasons
	seasons, err := store.ListSeasons(ctx, storage.SeasonFilter{SeriesID: &seriesID})
	assert.NoError(t, err)
	assert.Len(t, seasons, 1)
	assert.Equal(t, season.SeriesID, seasons[0].SeriesID)
	assert.Equal(t, storage.SeasonStateMissing, seasons[0].State)

	// Test deleting the season
	err = store.DeleteSeason(ctx, id)
	assert.NoError(t, err)

	// Verify deletion
	seasons, err = store.ListSeasons(ctx, storage.SeasonFilter{SeriesID: &seriesID})
	assert.NoError(t, err)
	assert.Empty(t, seasons)

	// Test getting non-existent season
	_, err = store.GetSeasonByID(ctx, id)
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestSeasonStorage_CreateWithID(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	require.NotNil(t, store)

	seriesID, err := store.CreateSeries(ctx, storage.Series{
		Series: model.Series{Monitored: 1, QualityProfileID: 1},
	}, storage.SeriesStateMissing)
	require.NoError(t, err)

	// Insert with an explicit id exercises the non-zero id insert branch.
	season := storage.Season{
		Season: model.Season{
			ID:           1,
			SeriesID:     int32(seriesID),
			SeasonNumber: 1,
			Monitored:    1,
		},
	}
	id, err := store.CreateSeason(ctx, season, storage.SeasonStateMissing)
	require.NoError(t, err)
	assert.Equal(t, int64(1), id)

	found, err := store.GetSeasonByID(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, int32(1), found.SeasonNumber)

	// Pin legacy behavior: re-creating with an existing id updates the season
	// row (upsert) but then fails to insert a second initial transition with
	// sort_key 1, violating the (season_id, sort_key) unique index. Use
	// UpdateSeasonState to advance the state of an existing season instead.
	updated := season
	updated.SeasonNumber = 2
	_, err = store.CreateSeason(ctx, updated, storage.SeasonStateDiscovered)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "UNIQUE constraint failed")

	seasons, err := store.ListSeasons(ctx, storage.SeasonFilter{SeriesID: &seriesID})
	require.NoError(t, err)
	assert.Len(t, seasons, 1)
	assert.Equal(t, int32(1), seasons[0].SeasonNumber)
}

func TestSeasonStorage_GetBySeriesIDAndNumber(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	require.NotNil(t, store)

	seriesID, err := store.CreateSeries(ctx, storage.Series{
		Series: model.Series{Monitored: 1, QualityProfileID: 1},
	}, storage.SeriesStateMissing)
	require.NoError(t, err)

	otherSeriesID, err := store.CreateSeries(ctx, storage.Series{
		Series: model.Series{Monitored: 1, QualityProfileID: 1},
	}, storage.SeriesStateMissing)
	require.NoError(t, err)

	for _, number := range []int32{1, 2, 3} {
		_, err := store.CreateSeason(ctx, storage.Season{
			Season: model.Season{SeriesID: int32(seriesID), SeasonNumber: number, Monitored: 1},
		}, storage.SeasonStateMissing)
		require.NoError(t, err)
	}
	_, err = store.CreateSeason(ctx, storage.Season{
		Season: model.Season{SeriesID: int32(otherSeriesID), SeasonNumber: 2, Monitored: 1},
	}, storage.SeasonStateMissing)
	require.NoError(t, err)

	found, err := store.GetSeasonBySeriesIDAndNumber(ctx, seriesID, 2)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, int32(2), found.SeasonNumber)
	assert.Equal(t, int32(seriesID), found.SeriesID)
	assert.Equal(t, storage.SeasonStateMissing, found.State)

	_, err = store.GetSeasonBySeriesIDAndNumber(ctx, seriesID, 9)
	assert.ErrorIs(t, err, storage.ErrNotFound)

	// Season number 2 of the other series must not leak in.
	_, err = store.GetSeasonBySeriesIDAndNumber(ctx, otherSeriesID+1000, 2)
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestSeasonStorage_ListFilters(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	require.NotNil(t, store)

	seriesID, err := store.CreateSeries(ctx, storage.Series{
		Series: model.Series{Monitored: 1, QualityProfileID: 1},
	}, storage.SeriesStateMissing)
	require.NoError(t, err)

	otherSeriesID, err := store.CreateSeries(ctx, storage.Series{
		Series: model.Series{Monitored: 1, QualityProfileID: 1},
	}, storage.SeriesStateMissing)
	require.NoError(t, err)

	seasonMissing, err := store.CreateSeason(ctx, storage.Season{
		Season: model.Season{SeriesID: int32(seriesID), SeasonNumber: 1, Monitored: 1},
	}, storage.SeasonStateMissing)
	require.NoError(t, err)

	seasonDownloading, err := store.CreateSeason(ctx, storage.Season{
		Season: model.Season{SeriesID: int32(seriesID), SeasonNumber: 2, Monitored: 1},
	}, storage.SeasonStateMissing)
	require.NoError(t, err)
	// The machine only allows new -> missing/discovered/unreleased on create,
	// so downloading is reached with a state update (missing -> downloading).
	require.NoError(t, store.UpdateSeasonState(ctx, seasonDownloading, storage.SeasonStateDownloading, nil))

	_, err = store.CreateSeason(ctx, storage.Season{
		Season: model.Season{SeriesID: int32(otherSeriesID), SeasonNumber: 1, Monitored: 1},
	}, storage.SeasonStateMissing)
	require.NoError(t, err)

	t.Run("zero filter matches all", func(t *testing.T) {
		seasons, err := store.ListSeasons(ctx, storage.SeasonFilter{})
		require.NoError(t, err)
		assert.Len(t, seasons, 3)
	})

	t.Run("series filter", func(t *testing.T) {
		seasons, err := store.ListSeasons(ctx, storage.SeasonFilter{SeriesID: &seriesID})
		require.NoError(t, err)
		assert.Len(t, seasons, 2)
	})

	t.Run("states filter", func(t *testing.T) {
		seasons, err := store.ListSeasons(ctx, storage.SeasonFilter{
			SeriesID: &seriesID,
			States:   []storage.SeasonState{storage.SeasonStateMissing},
		})
		require.NoError(t, err)
		require.Len(t, seasons, 1)
		assert.Equal(t, int32(seasonMissing), seasons[0].ID)

		seasons, err = store.ListSeasons(ctx, storage.SeasonFilter{
			States: []storage.SeasonState{
				storage.SeasonStateMissing,
				storage.SeasonStateDownloading,
			},
		})
		require.NoError(t, err)
		assert.Len(t, seasons, 3)

		seasons, err = store.ListSeasons(ctx, storage.SeasonFilter{
			SeriesID: &seriesID,
			States:   []storage.SeasonState{storage.SeasonStateCompleted},
		})
		require.NoError(t, err)
		assert.Empty(t, seasons)

		// Sanity check the downloading season id used above.
		downloading, err := store.GetSeasonByID(ctx, seasonDownloading)
		require.NoError(t, err)
		assert.Equal(t, storage.SeasonStateDownloading, downloading.State)
	})
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
				SeasonMetadataID: ptr.To(int32(1)),
				Monitored:        1,
			},
		}

		seasonID, err := store.CreateSeason(ctx, season, storage.SeasonStateMissing)
		require.NoError(t, err)
		assert.Equal(t, int64(1), seasonID)

		season.ID = int32(seasonID)
		err = store.UpdateSeasonState(ctx, seasonID, storage.SeasonStateDownloading, &storage.TransitionStateMetadata{
			DownloadID:       ptr.To("123"),
			DownloadClientID: ptr.To(int32(2)),
		})
		require.NoError(t, err)

		foundSeason, err := store.GetSeasonByID(ctx, seasonID)
		require.NoError(t, err)
		require.NotNil(t, foundSeason)

		assert.Equal(t, storage.SeasonStateDownloading, foundSeason.State)
		assert.Equal(t, "123", foundSeason.DownloadID)
		assert.Equal(t, int32(1), foundSeason.SeriesID)
		assert.Equal(t, ptr.To(int32(1)), foundSeason.SeasonMetadataID)

		err = store.UpdateSeasonState(ctx, 999999, storage.SeasonStateMissing, nil)
		assert.ErrorIs(t, err, storage.ErrNotFound)
	})

	t.Run("repeated transitions track the most recent one", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)
		require.NotNil(t, store)

		seasonID, err := store.CreateSeason(ctx, storage.Season{
			Season: model.Season{SeriesID: 1, SeasonNumber: 1, Monitored: 1},
		}, storage.SeasonStateMissing)
		require.NoError(t, err)

		// missing -> downloading (with download metadata)
		require.NoError(t, store.UpdateSeasonState(ctx, seasonID, storage.SeasonStateDownloading, &storage.TransitionStateMetadata{
			DownloadID:       ptr.To("first"),
			DownloadClientID: ptr.To(int32(7)),
		}))

		// downloading -> completed (new metadata)
		require.NoError(t, store.UpdateSeasonState(ctx, seasonID, storage.SeasonStateCompleted, &storage.TransitionStateMetadata{
			DownloadID:       ptr.To("second"),
			DownloadClientID: ptr.To(int32(8)),
		}))

		// completed -> continuing (valid per the machine; no metadata clears
		// the download fields since a fresh transition row is written)
		require.NoError(t, store.UpdateSeasonState(ctx, seasonID, storage.SeasonStateContinuing, nil))

		found, err := store.GetSeasonByID(ctx, seasonID)
		require.NoError(t, err)
		assert.Equal(t, storage.SeasonStateContinuing, found.State)
		assert.Empty(t, found.DownloadID)
		assert.Equal(t, int32(0), found.DownloadClientID)
	})

	t.Run("invalid state transition", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)
		require.NotNil(t, store)

		seasonID, err := store.CreateSeason(ctx, storage.Season{
			Season: model.Season{SeriesID: 1, SeasonNumber: 1, Monitored: 1},
		}, storage.SeasonStateMissing)
		require.NoError(t, err)

		require.NoError(t, store.UpdateSeasonState(ctx, seasonID, storage.SeasonStateDownloading, nil))
		require.NoError(t, store.UpdateSeasonState(ctx, seasonID, storage.SeasonStateCompleted, nil))

		// completed can only go to continuing
		err = store.UpdateSeasonState(ctx, seasonID, storage.SeasonStateMissing, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid state transition")

		// the failed transition must not have changed the current state
		found, err := store.GetSeasonByID(ctx, seasonID)
		require.NoError(t, err)
		assert.Equal(t, storage.SeasonStateCompleted, found.State)
	})
}
