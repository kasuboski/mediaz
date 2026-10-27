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

func TestSeriesMetadataStorage(t *testing.T) {
	ctx := t.Context()
	store := initSqlite(t, ctx)
	want := sqlcdb.SeriesMetadatum{TmdbID: 12345, Title: "Test Series", SeasonCount: 1, EpisodeCount: 1, Status: "Continuing", Overview: sql.NullString{String: "overview", Valid: true}, ExternalIds: sql.NullString{String: "{}", Valid: true}}
	id, err := store.CreateSeriesMetadata(ctx, want)
	require.NoError(t, err)
	want.ID = id
	got, err := store.GetSeriesMetadataByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, want, got)
	got, err = store.GetSeriesMetadataByTmdbID(ctx, want.TmdbID)
	require.NoError(t, err)
	assert.Equal(t, want, got)
	list, err := store.ListSeriesMetadata(ctx)
	require.NoError(t, err)
	assert.Equal(t, []sqlcdb.SeriesMetadatum{want}, list)
	list, err = store.ListSeriesMetadataByStatus(ctx, "Continuing")
	require.NoError(t, err)
	assert.Equal(t, []sqlcdb.SeriesMetadatum{want}, list)
	list, err = store.ListSeriesMetadataByStatus(ctx, "Ended")
	require.NoError(t, err)
	assert.Empty(t, list)
	require.NoError(t, store.DeleteSeriesMetadata(ctx, id))
	_, err = store.GetSeriesMetadataByID(ctx, id)
	assert.ErrorIs(t, err, storage.ErrNotFound)
	_, err = store.GetSeriesMetadataByTmdbID(ctx, want.TmdbID)
	assert.ErrorIs(t, err, storage.ErrNotFound)
	list, err = store.ListSeriesMetadata(ctx)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestUpdateSeriesMetadata(t *testing.T) {
	ctx := t.Context()
	store := initSqlite(t, ctx)
	want := sqlcdb.SeriesMetadatum{TmdbID: 42, Title: "Original", Status: "Continuing", Overview: sql.NullString{String: "old", Valid: true}}
	id, err := store.CreateSeriesMetadata(ctx, want)
	require.NoError(t, err)
	want.ID = id
	want.Title = "Updated"
	want.SeasonCount = 2
	want.EpisodeCount = 10
	want.Status = "Ended"
	want.Overview = sql.NullString{}
	require.NoError(t, store.UpdateSeriesMetadata(ctx, want))
	got, err := store.GetSeriesMetadataByID(ctx, id)
	require.NoError(t, err)
	require.True(t, got.LastInfoSync.Valid)
	assert.WithinDuration(t, time.Now(), got.LastInfoSync.Time, 5*time.Second)
	want.LastInfoSync = got.LastInfoSync
	assert.Equal(t, want, got)
}

func TestCreateSeriesMetadataUpsert(t *testing.T) {
	ctx := t.Context()
	store := initSqlite(t, ctx)
	original := sqlcdb.SeriesMetadatum{TmdbID: 42, Title: "Original", Status: "Continuing", Overview: sql.NullString{String: "original overview", Valid: true}, PosterPath: sql.NullString{String: "/original.jpg", Valid: true}}
	id, err := store.CreateSeriesMetadata(ctx, original)
	require.NoError(t, err)
	update := original
	update.Title = "Updated"
	update.SeasonCount = 3
	update.EpisodeCount = 30
	update.Status = "Ended"
	upsertID, err := store.CreateSeriesMetadata(ctx, update)
	require.NoError(t, err)
	assert.Equal(t, id, upsertID)
	got, err := store.GetSeriesMetadataByTmdbID(ctx, 42)
	require.NoError(t, err)
	update.ID = id
	require.True(t, got.LastInfoSync.Valid)
	update.LastInfoSync = got.LastInfoSync
	assert.Equal(t, update, got)
	list, err := store.ListSeriesMetadata(ctx)
	require.NoError(t, err)
	assert.Equal(t, []sqlcdb.SeriesMetadatum{got}, list)
}

func TestCreateSeriesMetadataWithExplicitID(t *testing.T) {
	ctx := t.Context()
	store := initSqlite(t, ctx)
	want := sqlcdb.SeriesMetadatum{ID: 777, TmdbID: 99, Title: "Explicit ID Series", Status: "Continuing"}
	id, err := store.CreateSeriesMetadata(ctx, want)
	require.NoError(t, err)
	assert.Equal(t, want.ID, id)
	got, err := store.GetSeriesMetadataByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestLinkSeriesMetadata(t *testing.T) {
	ctx := t.Context()
	store := initSqlite(t, ctx)
	id, err := store.CreateSeries(ctx, sqlcdb.Series{QualityProfileID: 1}, storage.SeriesStateMissing)
	require.NoError(t, err)
	before, err := store.GetSeriesByID(ctx, id)
	require.NoError(t, err)
	metadataID, err := store.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{TmdbID: 42, Title: "Series"})
	require.NoError(t, err)
	require.NoError(t, store.LinkSeriesMetadata(ctx, id, metadataID))
	before.Series.SeriesMetadataID = sql.NullInt64{Int64: metadataID, Valid: true}
	got, err := store.GetSeriesByMetadataID(ctx, metadataID)
	require.NoError(t, err)
	assert.Equal(t, before, got)
}
