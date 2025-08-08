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

func TestEpisodeStorage(t *testing.T) {
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

	// Test creating an episode
	episode := storage.Episode{
		Episode: model.Episode{
			SeasonID:      int32(seasonID),
			EpisodeNumber: 1,
			Monitored:     1,
		},
	}

	id, err := store.CreateEpisode(ctx, episode, storage.EpisodeStateMissing)
	assert.Nil(t, err)
	assert.Greater(t, id, int64(0))

	// Test getting the episode
	retrieved, err := store.GetEpisode(ctx, table.Episode.ID.EQ(sqlite.Int64(id)))
	assert.Nil(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, episode.SeasonID, retrieved.SeasonID)
	assert.Equal(t, episode.EpisodeNumber, retrieved.EpisodeNumber)
	assert.Equal(t, episode.Monitored, retrieved.Monitored)
	assert.Equal(t, storage.EpisodeStateMissing, retrieved.State)

	// Test listing episodes
	episodes, err := store.ListEpisodes(ctx, table.Episode.SeasonID.EQ(sqlite.Int64(seasonID)))
	assert.Nil(t, err)
	assert.Len(t, episodes, 1)
	assert.Equal(t, episode.EpisodeNumber, episodes[0].EpisodeNumber)

	// Test listing episodes by state
	where := table.EpisodeTransition.ToState.EQ(sqlite.String(string(storage.EpisodeStateMissing))).AND(table.Episode.SeasonID.EQ(sqlite.Int64(seasonID)))
	stateEpisodes, err := store.ListEpisodes(ctx, where)
	assert.Nil(t, err)
	assert.Len(t, stateEpisodes, 1)
	assert.Equal(t, storage.EpisodeStateMissing, stateEpisodes[0].State)

	// Test updating episode file ID
	err = store.UpdateEpisodeEpisodeFileID(ctx, id, 123)
	assert.Nil(t, err)

	// Test getting episode by file ID
	byFile, err := store.GetEpisodeByEpisodeFileID(ctx, 123)
	assert.Nil(t, err)
	assert.Equal(t, id, int64(byFile.ID))

	// Test deleting the episode
	err = store.DeleteEpisode(ctx, id)
	assert.Nil(t, err)

	// Verify deletion
	episodes, err = store.ListEpisodes(ctx, table.Episode.SeasonID.EQ(sqlite.Int64(seasonID)))
	assert.Nil(t, err)
	assert.Empty(t, episodes)

	// Test getting non-existent episode
	_, err = store.GetEpisode(ctx, table.Episode.ID.EQ(sqlite.Int64(id)))
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestEpisodeFileStorage(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	assert.NotNil(t, store)

	// Test creating an episode file
	file := model.EpisodeFile{
		Quality:      "HD",
		Size:         1024,
		RelativePath: ptr("test/path.mp4"),
	}

	id, err := store.CreateEpisodeFile(ctx, file)
	assert.Nil(t, err)
	assert.Greater(t, id, int64(0))

	// Test getting the episode file
	retrieved, err := store.GetEpisodeFiles(ctx, id)
	assert.Nil(t, err)
	assert.NotEmpty(t, retrieved)
	assert.Equal(t, file.Quality, retrieved[0].Quality)
	assert.Equal(t, file.Size, retrieved[0].Size)
	assert.Equal(t, file.RelativePath, retrieved[0].RelativePath)

	// Test listing episode files
	files, err := store.ListEpisodeFiles(ctx)
	assert.Nil(t, err)
	assert.Len(t, files, 1)
	assert.Equal(t, file.Quality, files[0].Quality)

	// Test deleting the episode file
	err = store.DeleteEpisodeFile(ctx, id)
	assert.Nil(t, err)

	// Verify deletion
	files, err = store.ListEpisodeFiles(ctx)
	assert.Nil(t, err)
	assert.Empty(t, files)

	// Test getting non-existent episode file
	_, err = store.GetEpisodeFiles(ctx, id)
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestSeriesMetadataStorage(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	assert.NotNil(t, store)

	// Test creating Series metadata
	metadata := model.SeriesMetadata{
		TmdbID:       12345,
		Title:        "Test Series",
		SeasonCount:  1,
		EpisodeCount: 1,
		Status:       "Continuing",
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

func TestSeasonMetadataStorage(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	assert.NotNil(t, store)

	// Test creating season metadata
	metadata := model.SeasonMetadata{
		TmdbID:   12345,
		Title:    "Season 1",
		Overview: ptr("Test season overview"),
		Number:   1,
	}

	id, err := store.CreateSeasonMetadata(ctx, metadata)
	assert.Nil(t, err)
	assert.Greater(t, id, int64(0))

	// Test getting the season metadata
	where := table.SeasonMetadata.ID.EQ(sqlite.Int64(id))
	retrieved, err := store.GetSeasonMetadata(ctx, where)
	assert.Nil(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, metadata.TmdbID, retrieved.TmdbID)
	assert.Equal(t, metadata.Title, retrieved.Title)
	assert.Equal(t, metadata.Overview, retrieved.Overview)
	assert.Equal(t, metadata.Number, retrieved.Number)

	// Test listing season metadata
	metadataList, err := store.ListSeasonMetadata(ctx)
	assert.Nil(t, err)
	assert.Len(t, metadataList, 1)
	assert.Equal(t, metadata.Title, metadataList[0].Title)

	// Test deleting the season metadata
	err = store.DeleteSeasonMetadata(ctx, id)
	assert.Nil(t, err)

	// Verify deletion
	metadataList, err = store.ListSeasonMetadata(ctx)
	assert.Nil(t, err)
	assert.Empty(t, metadataList)

	// Test getting non-existent season metadata
	_, err = store.GetSeasonMetadata(ctx, where)
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestEpisodeMetadataStorage(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	assert.NotNil(t, store)

	// Test creating episode metadata
	metadata := model.EpisodeMetadata{
		TmdbID:   12345,
		Title:    "Test Episode",
		Overview: ptr("Test episode overview"),
		Runtime:  ptr(int32(45)),
	}

	id, err := store.CreateEpisodeMetadata(ctx, metadata)
	assert.Nil(t, err)
	assert.Greater(t, id, int64(0))

	// Test getting the episode metadata
	where := table.EpisodeMetadata.ID.EQ(sqlite.Int64(id))
	retrieved, err := store.GetEpisodeMetadata(ctx, where)
	assert.Nil(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, metadata.TmdbID, retrieved.TmdbID)
	assert.Equal(t, metadata.Title, retrieved.Title)
	assert.Equal(t, metadata.Overview, retrieved.Overview)
	assert.Equal(t, metadata.Runtime, retrieved.Runtime)

	// Test listing episode metadata
	metadataList, err := store.ListEpisodeMetadata(ctx)
	assert.Nil(t, err)
	assert.Len(t, metadataList, 1)
	assert.Equal(t, metadata.Title, metadataList[0].Title)

	// Test deleting the episode metadata
	err = store.DeleteEpisodeMetadata(ctx, id)
	assert.Nil(t, err)

	// Verify deletion
	metadataList, err = store.ListEpisodeMetadata(ctx)
	assert.Nil(t, err)
	assert.Empty(t, metadataList)

	// Test getting non-existent episode metadata
	_, err = store.GetEpisodeMetadata(ctx, where)
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

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
		// TmdbID is now in SeriesMetadata, not Series model
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
