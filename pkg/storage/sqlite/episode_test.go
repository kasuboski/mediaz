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

func TestUpdateEpisodeState(t *testing.T) {
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
	seriesID, err := store.CreateSeries(ctx, series, storage.SeriesStateMissing)
	require.Nil(t, err)

	season := storage.Season{
		Season: model.Season{
			SeriesID: int32(seriesID),
		},
	}
	seasonID, err := store.CreateSeason(ctx, season, storage.SeasonStateMissing)
	require.Nil(t, err)

	episode := storage.Episode{
		Episode: model.Episode{
			SeasonID:      int32(seasonID),
			EpisodeNumber: 1,
			Monitored:     1,
		},
	}
	id, err := store.CreateEpisode(ctx, episode, storage.EpisodeStateMissing)
	require.Nil(t, err)

	downloadID := "12"
	isSeasonDownload := true
	metadata := &storage.TransitionStateMetadata{
		DownloadID:             &downloadID,
		DownloadClientID:       ptr(int32(1)),
		IsEntireSeasonDownload: &isSeasonDownload,
	}

	err = store.UpdateEpisodeState(ctx, id, storage.EpisodeStateDownloading, metadata)
	assert.Nil(t, err)

	updated, err := store.GetEpisode(ctx, table.Episode.ID.EQ(sqlite.Int64(id)))
	assert.Nil(t, err)

	assert.Nil(t, err)
	assert.Equal(t, storage.EpisodeStateDownloading, updated.State)
	assert.Equal(t, downloadID, updated.DownloadID)
	assert.Equal(t, int32(1), updated.DownloadClientID)
	assert.Equal(t, isSeasonDownload, updated.IsEntireSeasonDownload)
	assert.Equal(t, storage.EpisodeStateDownloading, updated.State)

	err = store.UpdateEpisodeState(ctx, 999999, storage.EpisodeStateMissing, nil)
	assert.ErrorIs(t, err, storage.ErrNotFound)
}
