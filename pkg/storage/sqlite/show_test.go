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

func TestShowStorage(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	assert.NotNil(t, store)

	shows, err := store.ListShows(ctx)
	assert.Nil(t, err)
	assert.Empty(t, shows)
	time := time.Now()

	// Test creating a show
	show := model.Show{
		Monitored:        1,
		QualityProfileID: 1,
		Added:            &time,
	}

	id, err := store.CreateShow(ctx, show)
	assert.Nil(t, err)
	assert.Greater(t, id, int64(0))

	// Test getting the show
	retrieved, err := store.GetShow(ctx, id)
	assert.Nil(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, show.Monitored, retrieved.Monitored)
	assert.Equal(t, show.QualityProfileID, retrieved.QualityProfileID)

	// Test listing shows
	shows, err = store.ListShows(ctx)
	assert.Nil(t, err)
	assert.Len(t, shows, 1)
	assert.Equal(t, show.Monitored, shows[0].Monitored)

	// Test deleting the show
	err = store.DeleteShow(ctx, id)
	assert.Nil(t, err)

	// Verify deletion
	shows, err = store.ListShows(ctx)
	assert.Nil(t, err)
	assert.Empty(t, shows)

	// Test getting non-existent show
	_, err = store.GetShow(ctx, id)
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestSeasonStorage(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	assert.NotNil(t, store)

	// Create a show first
	show := model.Show{
		Monitored:        1,
		QualityProfileID: 1,
		Added:            ptr(time.Now()),
	}
	showID, err := store.CreateShow(ctx, show)
	require.Nil(t, err)

	// Test creating a season
	season := model.Season{
		ShowID: int32(showID),
	}

	id, err := store.CreateSeason(ctx, season)
	assert.Nil(t, err)
	assert.Greater(t, id, int64(0))

	// Test getting the season
	retrieved, err := store.GetSeason(ctx, id)
	assert.Nil(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, season.ShowID, retrieved.ShowID)

	// Test listing seasons
	seasons, err := store.ListSeasons(ctx, showID)
	assert.Nil(t, err)
	assert.Len(t, seasons, 1)
	assert.Equal(t, season.ShowID, seasons[0].ShowID)

	// Test deleting the season
	err = store.DeleteSeason(ctx, id)
	assert.Nil(t, err)

	// Verify deletion
	seasons, err = store.ListSeasons(ctx, showID)
	assert.Nil(t, err)
	assert.Empty(t, seasons)

	// Test getting non-existent season
	_, err = store.GetSeason(ctx, id)
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestEpisodeStorage(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	assert.NotNil(t, store)

	// Create a show and season first
	show := model.Show{
		Monitored:        1,
		QualityProfileID: 1,
		Added:            ptr(time.Now()),
	}
	showID, err := store.CreateShow(ctx, show)
	require.Nil(t, err)

	season := model.Season{
		ShowID: int32(showID),
	}
	seasonID, err := store.CreateSeason(ctx, season)
	require.Nil(t, err)

	// Test creating an episode
	episode := storage.Episode{
		Episode: model.Episode{
			SeasonID:      int32(seasonID),
			EpisodeNumber: 1,
			Monitored:     1,
		},
		State: storage.EpisodeState("wanted"),
	}

	id, err := store.CreateEpisode(ctx, episode)
	assert.Nil(t, err)
	assert.Greater(t, id, int64(0))

	// Test getting the episode
	retrieved, err := store.GetEpisode(ctx, id)
	assert.Nil(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, episode.SeasonID, retrieved.SeasonID)
	assert.Equal(t, episode.EpisodeNumber, retrieved.EpisodeNumber)
	assert.Equal(t, episode.Monitored, retrieved.Monitored)
	assert.Equal(t, episode.State, retrieved.State)

	// Test listing episodes
	episodes, err := store.ListEpisodes(ctx, seasonID)
	assert.Nil(t, err)
	assert.Len(t, episodes, 1)
	assert.Equal(t, episode.EpisodeNumber, episodes[0].EpisodeNumber)

	// Test listing episodes by state
	stateEpisodes, err := store.ListEpisodesByState(ctx, storage.EpisodeState("wanted"))
	assert.Nil(t, err)
	assert.Len(t, stateEpisodes, 1)
	assert.Equal(t, episode.State, stateEpisodes[0].State)

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
	episodes, err = store.ListEpisodes(ctx, seasonID)
	assert.Nil(t, err)
	assert.Empty(t, episodes)

	// Test getting non-existent episode
	_, err = store.GetEpisode(ctx, id)
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

func TestShowMetadataStorage(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	assert.NotNil(t, store)

	// Test creating show metadata
	metadata := model.ShowMetadata{
		TmdbID:       12345,
		Title:        "Test Show",
		SeasonCount:  1,
		EpisodeCount: 1,
		Status:       "Continuing",
	}

	id, err := store.CreateShowMetadata(ctx, metadata)
	assert.Nil(t, err)
	assert.Greater(t, id, int64(0))

	// Test getting the show metadata
	where := table.ShowMetadata.ID.EQ(sqlite.Int64(id))
	retrieved, err := store.GetShowMetadata(ctx, where)
	assert.Nil(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, metadata.TmdbID, retrieved.TmdbID)
	assert.Equal(t, metadata.Title, retrieved.Title)
	assert.Equal(t, metadata.SeasonCount, retrieved.SeasonCount)
	assert.Equal(t, metadata.EpisodeCount, retrieved.EpisodeCount)
	assert.Equal(t, metadata.Status, retrieved.Status)

	// Test listing show metadata
	metadataList, err := store.ListShowMetadata(ctx)
	assert.Nil(t, err)
	assert.Len(t, metadataList, 1)
	assert.Equal(t, metadata.Title, metadataList[0].Title)

	// Test deleting the show metadata
	err = store.DeleteShowMetadata(ctx, id)
	assert.Nil(t, err)

	// Verify deletion
	metadataList, err = store.ListShowMetadata(ctx)
	assert.Nil(t, err)
	assert.Empty(t, metadataList)

	// Test getting non-existent show metadata
	_, err = store.GetShowMetadata(ctx, where)
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestSeasonMetadataStorage(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	assert.NotNil(t, store)

	// Test creating season metadata
	metadata := model.SeasonMetadata{
		TmdbID:       12345,
		Title:        ptr("Season 1"),
		Overview:     ptr("Test season overview"),
		EpisodeCount: 10,
		Number:       1,
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
	assert.Equal(t, metadata.EpisodeCount, retrieved.EpisodeCount)
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
		Title:    ptr("Test Episode"),
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
