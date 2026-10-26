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

// tvTestEpisode creates a series, season and episode and returns the episode id.
func tvTestEpisode(t *testing.T, ctx context.Context, store storage.Storage, episodeNumber int32) int64 {
	t.Helper()

	seriesID, err := store.CreateSeries(ctx, storage.Series{
		Series: model.Series{Monitored: 1, QualityProfileID: 1},
	}, storage.SeriesStateMissing)
	require.NoError(t, err)

	seasonID, err := store.CreateSeason(ctx, storage.Season{
		Season: model.Season{SeriesID: int32(seriesID), SeasonNumber: 1, Monitored: 1},
	}, storage.SeasonStateMissing)
	require.NoError(t, err)

	episodeID, err := store.CreateEpisode(ctx, storage.Episode{
		Episode: model.Episode{
			SeasonID:      int32(seasonID),
			EpisodeNumber: episodeNumber,
			Monitored:     1,
		},
	}, storage.EpisodeStateMissing)
	require.NoError(t, err)

	return episodeID
}

func TestEpisodeStorage(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	require.NotNil(t, store)

	seriesID, err := store.CreateSeries(ctx, storage.Series{
		Series: model.Series{Monitored: 1, QualityProfileID: 1, Added: ptr.To(time.Now())},
	}, storage.SeriesStateMissing)
	require.NoError(t, err)

	seasonID, err := store.CreateSeason(ctx, storage.Season{
		Season: model.Season{SeriesID: int32(seriesID), SeasonNumber: 1, Monitored: 1},
	}, storage.SeasonStateMissing)
	require.NoError(t, err)

	episode := storage.Episode{
		Episode: model.Episode{
			SeasonID:      int32(seasonID),
			EpisodeNumber: 1,
			Monitored:     1,
		},
	}
	id, err := store.CreateEpisode(ctx, episode, storage.EpisodeStateMissing)
	require.NoError(t, err)
	assert.Greater(t, id, int64(0))

	retrieved, err := store.GetEpisodeByID(ctx, id)
	assert.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, episode.SeasonID, retrieved.SeasonID)
	assert.Equal(t, episode.EpisodeNumber, retrieved.EpisodeNumber)
	assert.Equal(t, storage.EpisodeStateMissing, retrieved.State)

	episodes, err := store.ListEpisodes(ctx, storage.EpisodeFilter{SeasonID: &seasonID})
	assert.NoError(t, err)
	assert.Len(t, episodes, 1)
	assert.Equal(t, episode.EpisodeNumber, episodes[0].EpisodeNumber)

	err = store.DeleteEpisode(ctx, id)
	assert.NoError(t, err)

	episodes, err = store.ListEpisodes(ctx, storage.EpisodeFilter{SeasonID: &seasonID})
	assert.NoError(t, err)
	assert.Empty(t, episodes)

	_, err = store.GetEpisodeByID(ctx, id)
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestEpisodeStorage_CreateWithID(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	require.NotNil(t, store)

	seriesID, err := store.CreateSeries(ctx, storage.Series{
		Series: model.Series{Monitored: 1, QualityProfileID: 1},
	}, storage.SeriesStateMissing)
	require.NoError(t, err)

	seasonID, err := store.CreateSeason(ctx, storage.Season{
		Season: model.Season{SeriesID: int32(seriesID), SeasonNumber: 1, Monitored: 1},
	}, storage.SeasonStateMissing)
	require.NoError(t, err)

	// Insert with an explicit id exercises the non-zero id insert branch.
	episode := storage.Episode{
		Episode: model.Episode{
			ID:            100,
			SeasonID:      int32(seasonID),
			EpisodeNumber: 7,
			Monitored:     1,
		},
	}
	id, err := store.CreateEpisode(ctx, episode, storage.EpisodeStateMissing)
	require.NoError(t, err)
	assert.Equal(t, int64(100), id)

	found, err := store.GetEpisodeByID(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, int32(7), found.EpisodeNumber)

	// Pin legacy behavior: re-creating with an existing id updates the
	// episode row (upsert) but then fails to insert a second initial
	// transition with sort_key 1, violating the (episode_id, sort_key) unique
	// index. Use UpdateEpisodeState to advance the state of an existing
	// episode instead.
	updated := episode
	updated.EpisodeNumber = 9
	_, err = store.CreateEpisode(ctx, updated, storage.EpisodeStateDiscovered)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "UNIQUE constraint failed")

	episodes, err := store.ListEpisodes(ctx, storage.EpisodeFilter{SeasonID: &seasonID})
	require.NoError(t, err)
	assert.Len(t, episodes, 1)
	assert.Equal(t, int32(7), episodes[0].EpisodeNumber)
}

func TestEpisodeStorage_ListFilters(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	require.NotNil(t, store)

	episodeA := tvTestEpisode(t, ctx, store, 1)
	episodeB := tvTestEpisode(t, ctx, store, 2)

	episodeARecord, err := store.GetEpisodeByID(ctx, episodeA)
	require.NoError(t, err)
	episodeBRecord, err := store.GetEpisodeByID(ctx, episodeB)
	require.NoError(t, err)

	// Each helper call creates its own series/season, so move episode B into
	// the same season to exercise the season filter.
	require.NoError(t, store.LinkEpisodeMetadata(ctx, episodeB, episodeARecord.SeasonID, int32(episodeB)))
	episodeBRecord, err = store.GetEpisodeByID(ctx, episodeB)
	require.NoError(t, err)
	assert.Equal(t, episodeARecord.SeasonID, episodeBRecord.SeasonID)

	// episode B goes to downloaded, A stays missing
	require.NoError(t, store.UpdateEpisodeState(ctx, episodeB, storage.EpisodeStateDownloading, nil))
	require.NoError(t, store.UpdateEpisodeState(ctx, episodeB, storage.EpisodeStateDownloaded, nil))

	t.Run("zero filter matches all", func(t *testing.T) {
		episodes, err := store.ListEpisodes(ctx, storage.EpisodeFilter{})
		require.NoError(t, err)
		assert.Len(t, episodes, 2)
	})

	t.Run("season filter", func(t *testing.T) {
		seasonIDFilter := int64(episodeARecord.SeasonID)
		episodes, err := store.ListEpisodes(ctx, storage.EpisodeFilter{SeasonID: &seasonIDFilter})
		require.NoError(t, err)
		assert.Len(t, episodes, 2)
	})

	t.Run("states filter", func(t *testing.T) {
		seasonIDFilter := int64(episodeARecord.SeasonID)
		episodes, err := store.ListEpisodes(ctx, storage.EpisodeFilter{
			SeasonID: &seasonIDFilter,
			States:   []storage.EpisodeState{storage.EpisodeStateMissing},
		})
		require.NoError(t, err)
		require.Len(t, episodes, 1)
		assert.Equal(t, int32(episodeA), episodes[0].ID)

		episodes, err = store.ListEpisodes(ctx, storage.EpisodeFilter{
			SeasonID: &seasonIDFilter,
			States:   []storage.EpisodeState{storage.EpisodeStateDownloaded},
		})
		require.NoError(t, err)
		require.Len(t, episodes, 1)
		assert.Equal(t, int32(episodeB), episodes[0].ID)
		assert.Equal(t, storage.EpisodeStateDownloaded, episodes[0].State)

		episodes, err = store.ListEpisodes(ctx, storage.EpisodeFilter{
			States: []storage.EpisodeState{storage.EpisodeStateCompleted},
		})
		require.NoError(t, err)
		assert.Empty(t, episodes)
	})
}

func TestUpdateEpisodeState(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	assert.NotNil(t, store)

	episodeID := tvTestEpisode(t, ctx, store, 1)

	downloadID := "12"
	isSeasonDownload := true
	metadata := &storage.TransitionStateMetadata{
		DownloadID:             &downloadID,
		DownloadClientID:       ptr.To(int32(1)),
		IsEntireSeasonDownload: &isSeasonDownload,
	}

	err := store.UpdateEpisodeState(ctx, episodeID, storage.EpisodeStateDownloading, metadata)
	assert.NoError(t, err)

	updated, err := store.GetEpisodeByID(ctx, episodeID)
	assert.NoError(t, err)

	assert.Equal(t, storage.EpisodeStateDownloading, updated.State)
	assert.Equal(t, downloadID, updated.DownloadID)
	assert.Equal(t, int32(1), updated.DownloadClientID)
	assert.True(t, updated.IsEntireSeasonDownload)

	err = store.UpdateEpisodeState(ctx, 999999, storage.EpisodeStateMissing, nil)
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestUpdateEpisodeState_InvalidTransition(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	require.NotNil(t, store)

	episodeID := tvTestEpisode(t, ctx, store, 1)

	// missing -> discovered is valid, discovered -> downloading is not
	require.NoError(t, store.UpdateEpisodeState(ctx, episodeID, storage.EpisodeStateDiscovered, nil))

	err := store.UpdateEpisodeState(ctx, episodeID, storage.EpisodeStateDownloading, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid state transition")

	found, err := store.GetEpisodeByID(ctx, episodeID)
	require.NoError(t, err)
	assert.Equal(t, storage.EpisodeStateDiscovered, found.State)
}

func TestEpisodeStorage_UpdateEpisodeFileID(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	require.NotNil(t, store)

	episodeID := tvTestEpisode(t, ctx, store, 1)

	require.NoError(t, store.UpdateEpisodeEpisodeFileID(ctx, episodeID, 55))

	found, err := store.GetEpisodeByID(ctx, episodeID)
	require.NoError(t, err)
	require.NotNil(t, found.EpisodeFileID)
	assert.Equal(t, int32(55), *found.EpisodeFileID)
}

func TestEpisodeStorage_GetByEpisodeFileID(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	require.NotNil(t, store)

	episodeID := tvTestEpisode(t, ctx, store, 1)

	require.NoError(t, store.UpdateEpisodeEpisodeFileID(ctx, episodeID, 55))

	byFile, err := store.GetEpisodeByEpisodeFileID(ctx, 55)
	require.NoError(t, err)
	require.NotNil(t, byFile)
	assert.Equal(t, int32(episodeID), byFile.ID)
	assert.Equal(t, storage.EpisodeStateMissing, byFile.State)

	_, err = store.GetEpisodeByEpisodeFileID(ctx, 999)
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestEpisodeStorage_GetByEpisodeFileID_LimitedTransitionColumns(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	require.NotNil(t, store)

	episodeID := tvTestEpisode(t, ctx, store, 1)

	// Move to downloading with full metadata, including the season download flag.
	require.NoError(t, store.UpdateEpisodeEpisodeFileID(ctx, episodeID, 55))
	require.NoError(t, store.UpdateEpisodeState(ctx, episodeID, storage.EpisodeStateDownloading, &storage.TransitionStateMetadata{
		DownloadID:             ptr.To("bundle-1"),
		DownloadClientID:       ptr.To(int32(3)),
		IsEntireSeasonDownload: ptr.To(true),
	}))

	byFile, err := store.GetEpisodeByEpisodeFileID(ctx, 55)
	require.NoError(t, err)
	assert.Equal(t, storage.EpisodeStateDownloading, byFile.State)
	assert.Equal(t, "bundle-1", byFile.DownloadID)
	assert.Equal(t, int32(3), byFile.DownloadClientID)
	// The legacy statement did not select is_entire_season_download on this
	// path, so the field stays false here even though it is set in storage.
	assert.False(t, byFile.IsEntireSeasonDownload)

	byID, err := store.GetEpisodeByID(ctx, episodeID)
	require.NoError(t, err)
	assert.True(t, byID.IsEntireSeasonDownload)
}

func TestEpisodeStorage_LinkEpisodeMetadata(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	require.NotNil(t, store)

	episodeID := tvTestEpisode(t, ctx, store, 1)
	found, err := store.GetEpisodeByID(ctx, episodeID)
	require.NoError(t, err)

	require.NoError(t, store.LinkEpisodeMetadata(ctx, episodeID, found.SeasonID, 77))

	updated, err := store.GetEpisodeByID(ctx, episodeID)
	require.NoError(t, err)
	assert.Equal(t, found.SeasonID, updated.SeasonID)
	require.NotNil(t, updated.EpisodeMetadataID)
	assert.Equal(t, int32(77), *updated.EpisodeMetadataID)
}
