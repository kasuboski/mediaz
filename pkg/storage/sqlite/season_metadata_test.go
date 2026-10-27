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

// Season metadata is linked through the generated Season entity.
func TestSeasonMetadataStorage(t *testing.T) {
	ctx := t.Context()
	store := initSqlite(t, ctx)
	q := sqlcdb.New(store.(*SQLite).db)
	seriesID := tvTestSeries(t, ctx, store)
	seriesMetadataID, err := store.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{TmdbID: 123, Title: "Series"})
	require.NoError(t, err)
	airDate := sql.NullTime{Time: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true}
	metadataID, err := q.UpsertSeasonMetadata(ctx, sqlcdb.UpsertSeasonMetadataParams{ID: sql.NullInt64{Int64: 77, Valid: true}, SeriesMetadataID: seriesMetadataID, Number: 1, TmdbID: 12345, Title: "Season One", Overview: sql.NullString{String: "overview", Valid: true}, AirDate: airDate})
	require.NoError(t, err)
	assert.Equal(t, int64(77), metadataID)
	metadata, err := q.GetSeasonMetadataByID(ctx, 77)
	require.NoError(t, err)
	assert.Equal(t, sqlcdb.SeasonMetadatum{ID: 77, SeriesMetadataID: seriesMetadataID, Number: 1, TmdbID: 12345, Title: "Season One", Overview: sql.NullString{String: "overview", Valid: true}, AirDate: airDate}, metadata)
	season := sqlcdb.Season{SeriesID: seriesID, SeasonNumber: 1, Monitored: 1, SeasonMetadataID: sql.NullInt64{Int64: 77, Valid: true}}
	id, err := store.CreateSeason(ctx, season, storage.SeasonStateMissing)
	require.NoError(t, err)
	season.ID = id
	got, err := store.GetSeasonByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, sqlcdb.GetSeasonByIDRow{Season: season, State: sql.NullString{String: string(storage.SeasonStateMissing), Valid: true}}, got)
	byNumber, err := store.GetSeasonBySeriesIDAndNumber(ctx, seriesID, 1)
	require.NoError(t, err)
	assert.Equal(t, season, byNumber.Season)
	list, err := store.ListSeasons(ctx, storage.SeasonFilter{SeriesID: &seriesID})
	require.NoError(t, err)
	assert.Equal(t, []sqlcdb.ListSeasonsRow{{Season: season, State: string(storage.SeasonStateMissing)}}, list)

	// Refresh metadata by its external identity without changing the season's
	// stored link; every mutable metadata field must reflect the new values.
	refreshedID, err := q.UpsertSeasonMetadata(ctx, sqlcdb.UpsertSeasonMetadataParams{SeriesMetadataID: seriesMetadataID, Number: 1, TmdbID: 12345, Title: "Refreshed"})
	require.NoError(t, err)
	assert.Equal(t, metadataID, refreshedID)
	metadata, err = q.GetSeasonMetadataByID(ctx, 77)
	require.NoError(t, err)
	assert.Equal(t, sqlcdb.SeasonMetadatum{ID: 77, SeriesMetadataID: seriesMetadataID, Number: 1, TmdbID: 12345, Title: "Refreshed"}, metadata)
	got, err = store.GetSeasonByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, season, got.Season)
}

func TestLinkSeasonMetadata(t *testing.T) {
	ctx := t.Context()
	store := initSqlite(t, ctx)
	q := sqlcdb.New(store.(*SQLite).db)
	seriesID := tvTestSeries(t, ctx, store)
	id, err := store.CreateSeason(ctx, sqlcdb.Season{SeriesID: seriesID, SeasonNumber: 1}, storage.SeasonStateMissing)
	require.NoError(t, err)
	before, err := store.GetSeasonByID(ctx, id)
	require.NoError(t, err)
	require.False(t, before.Season.SeasonMetadataID.Valid)
	metadataID, err := store.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{TmdbID: 123, Title: "Series"})
	require.NoError(t, err)
	seasonMetadataID, err := q.UpsertSeasonMetadata(ctx, sqlcdb.UpsertSeasonMetadataParams{ID: sql.NullInt64{Int64: 78, Valid: true}, SeriesMetadataID: metadataID, Number: 1, TmdbID: 12345, Title: "Season One"})
	require.NoError(t, err)
	assert.Equal(t, int64(78), seasonMetadataID)
	require.NoError(t, store.LinkSeasonMetadata(ctx, id, 78))
	before.Season.SeasonMetadataID = sql.NullInt64{Int64: 78, Valid: true}
	got, err := store.GetSeasonByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, before, got)
}
