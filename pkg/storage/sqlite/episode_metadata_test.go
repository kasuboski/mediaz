package sqlite

import (
	"database/sql"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Metadata associations are exercised through generated Episode entities.
func TestEpisodeMetadataStorage(t *testing.T) {
	ctx := t.Context()
	store := initSqlite(t, ctx)
	q := sqlcdb.New(store.(*SQLite).db)
	seriesID := tvTestSeries(t, ctx, store)
	seriesMetadataID, err := store.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{TmdbID: 123, Title: "Series"})
	require.NoError(t, err)
	seasonMetadataID, err := q.UpsertSeasonMetadata(ctx, sqlcdb.UpsertSeasonMetadataParams{ID: sql.NullInt64{Int64: 77, Valid: true}, SeriesMetadataID: seriesMetadataID, Number: 1, TmdbID: 12345, Title: "Season One"})
	require.NoError(t, err)
	assert.Equal(t, int64(77), seasonMetadataID)
	seasonID, err := store.CreateSeason(ctx, sqlcdb.Season{SeriesID: seriesID, SeasonNumber: 1, SeasonMetadataID: sql.NullInt64{Int64: 77, Valid: true}}, storage.SeasonStateMissing)
	require.NoError(t, err)
	episodeMetadataID, err := q.UpsertEpisodeMetadata(ctx, sqlcdb.UpsertEpisodeMetadataParams{ID: sql.NullInt64{Int64: 88, Valid: true}, SeasonMetadataID: 77, Number: 1, TmdbID: 123456, Title: "Pilot", Overview: sql.NullString{String: "overview", Valid: true}, AirDate: sql.NullTime{Time: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true}, Runtime: sql.NullInt64{Int64: 45, Valid: true}, StillPath: sql.NullString{String: "/still.jpg", Valid: true}})
	require.NoError(t, err)
	assert.Equal(t, int64(88), episodeMetadataID)
	metadata, err := q.GetEpisodeMetadataByID(ctx, 88)
	require.NoError(t, err)
	assert.Equal(t, sqlcdb.EpisodeMetadatum{ID: 88, SeasonMetadataID: 77, Number: 1, TmdbID: 123456, Title: "Pilot", Overview: sql.NullString{String: "overview", Valid: true}, AirDate: sql.NullTime{Time: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true}, Runtime: sql.NullInt64{Int64: 45, Valid: true}, StillPath: sql.NullString{String: "/still.jpg", Valid: true}}, metadata)
	episode := sqlcdb.Episode{SeasonID: seasonID, EpisodeNumber: 1, Monitored: 1, EpisodeMetadataID: sql.NullInt64{Int64: 88, Valid: true}}
	id, err := store.CreateEpisode(ctx, episode, storage.EpisodeStateMissing)
	require.NoError(t, err)
	episode.ID = id
	got, err := store.GetEpisodeByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, sqlcdb.GetEpisodeByIDRow{Episode: episode, State: sql.NullString{String: string(storage.EpisodeStateMissing), Valid: true}}, got)
	list, err := store.ListEpisodes(ctx, storage.EpisodeFilter{SeasonID: &seasonID})
	require.NoError(t, err)
	assert.Equal(t, []sqlcdb.ListEpisodesRow{{Episode: episode, State: string(storage.EpisodeStateMissing)}}, list)

	refreshedID, err := q.UpsertEpisodeMetadata(ctx, sqlcdb.UpsertEpisodeMetadataParams{SeasonMetadataID: 77, Number: 1, TmdbID: 123456, Title: "Refreshed Pilot"})
	require.NoError(t, err)
	assert.Equal(t, episodeMetadataID, refreshedID)
	metadata, err = q.GetEpisodeMetadataByID(ctx, 88)
	require.NoError(t, err)
	assert.Equal(t, sqlcdb.EpisodeMetadatum{ID: 88, SeasonMetadataID: 77, Number: 1, TmdbID: 123456, Title: "Refreshed Pilot"}, metadata)
	got, err = store.GetEpisodeByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, episode, got.Episode)
}

func TestEpisodeStorage_LinkEpisodeMetadata(t *testing.T) {
	ctx := t.Context()
	store := initSqlite(t, ctx)
	q := sqlcdb.New(store.(*SQLite).db)
	id := tvTestEpisode(t, ctx, store, 1)
	before, err := store.GetEpisodeByID(ctx, id)
	require.NoError(t, err)
	require.False(t, before.Episode.EpisodeMetadataID.Valid)
	metadataID, err := store.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{TmdbID: 123, Title: "Series"})
	require.NoError(t, err)
	seasonMetadataID, err := q.UpsertSeasonMetadata(ctx, sqlcdb.UpsertSeasonMetadataParams{ID: sql.NullInt64{Int64: 77, Valid: true}, SeriesMetadataID: metadataID, Number: 1, TmdbID: 12345, Title: "Season One"})
	require.NoError(t, err)
	assert.Equal(t, int64(77), seasonMetadataID)
	episodeMetadataID, err := q.UpsertEpisodeMetadata(ctx, sqlcdb.UpsertEpisodeMetadataParams{ID: sql.NullInt64{Int64: 88, Valid: true}, SeasonMetadataID: 77, Number: 1, TmdbID: 123456, Title: "Pilot"})
	require.NoError(t, err)
	assert.Equal(t, int64(88), episodeMetadataID)
	require.NoError(t, store.LinkEpisodeMetadata(ctx, id, before.Episode.SeasonID, 88))
	before.Episode.EpisodeMetadataID = sql.NullInt64{Int64: 88, Valid: true}
	got, err := store.GetEpisodeByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, before, got)
}
