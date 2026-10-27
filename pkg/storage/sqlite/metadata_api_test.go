package sqlite

import (
	"testing"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Local primary keys deliberately differ from TMDB IDs and season/episode
// numbers, so a query against the wrong identity column cannot pass.
func TestTVMetadataPublicLookupListAndDeleteAPIs(t *testing.T) {
	ctx := t.Context()
	store := initSqlite(t, ctx)
	series := sqlcdb.SeriesMetadatum{ID: 40, TmdbID: 900, Title: "Series", Status: "continuing"}
	seriesID, err := store.CreateSeriesMetadata(ctx, series)
	require.NoError(t, err)
	require.Equal(t, series.ID, seriesID)
	season := sqlcdb.SeasonMetadatum{ID: 50, SeriesMetadataID: seriesID, Number: 2, TmdbID: 901, Title: "Season"}
	seasonID, err := store.CreateSeasonMetadata(ctx, season)
	require.NoError(t, err)
	require.Equal(t, season.ID, seasonID)
	episode := sqlcdb.EpisodeMetadatum{ID: 60, SeasonMetadataID: seasonID, Number: 3, TmdbID: 902, Title: "Episode"}
	episodeID, err := store.CreateEpisodeMetadata(ctx, episode)
	require.NoError(t, err)
	require.Equal(t, episode.ID, episodeID)

	gotSeason, err := store.GetSeasonMetadataByID(ctx, seasonID)
	require.NoError(t, err)
	assert.Equal(t, season, gotSeason)
	gotSeason, err = store.GetSeasonMetadataByTmdbID(ctx, season.TmdbID)
	require.NoError(t, err)
	assert.Equal(t, season, gotSeason)
	seasons, err := store.ListSeasonMetadataBySeriesMetadataID(ctx, seriesID)
	require.NoError(t, err)
	assert.Equal(t, []sqlcdb.SeasonMetadatum{season}, seasons)

	gotEpisode, err := store.GetEpisodeMetadataByID(ctx, episodeID)
	require.NoError(t, err)
	assert.Equal(t, episode, gotEpisode)
	gotEpisode, err = store.GetEpisodeMetadataByTmdbID(ctx, episode.TmdbID)
	require.NoError(t, err)
	assert.Equal(t, episode, gotEpisode)
	gotEpisode, err = store.GetEpisodeMetadataBySeasonAndNumber(ctx, seasonID, episode.Number)
	require.NoError(t, err)
	assert.Equal(t, episode, gotEpisode)
	episodes, err := store.ListEpisodeMetadataBySeasonMetadataID(ctx, seasonID)
	require.NoError(t, err)
	assert.Equal(t, []sqlcdb.EpisodeMetadatum{episode}, episodes)
	episodes, err = store.ListEpisodeMetadataByIDs(ctx, []int64{-1, episodeID, 999})
	require.NoError(t, err)
	assert.Equal(t, []sqlcdb.EpisodeMetadatum{episode}, episodes)
	episodes, err = store.ListEpisodeMetadataByIDs(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, episodes)

	require.NoError(t, store.DeleteEpisodeMetadata(ctx, episodeID))
	_, err = store.GetEpisodeMetadataByID(ctx, episodeID)
	require.ErrorIs(t, err, storage.ErrNotFound)
	_, err = store.GetEpisodeMetadataByTmdbID(ctx, episode.TmdbID)
	require.ErrorIs(t, err, storage.ErrNotFound)
	_, err = store.GetEpisodeMetadataBySeasonAndNumber(ctx, seasonID, episode.Number)
	require.ErrorIs(t, err, storage.ErrNotFound)
	episodes, err = store.ListEpisodeMetadataBySeasonMetadataID(ctx, seasonID)
	require.NoError(t, err)
	assert.Empty(t, episodes)
	require.NoError(t, store.DeleteSeasonMetadata(ctx, seasonID))
	_, err = store.GetSeasonMetadataByID(ctx, seasonID)
	require.ErrorIs(t, err, storage.ErrNotFound)
	_, err = store.GetSeasonMetadataByTmdbID(ctx, season.TmdbID)
	require.ErrorIs(t, err, storage.ErrNotFound)
	seasons, err = store.ListSeasonMetadataBySeriesMetadataID(ctx, seriesID)
	require.NoError(t, err)
	assert.Empty(t, seasons)
	require.NoError(t, store.DeleteSeriesMetadata(ctx, seriesID))
}
