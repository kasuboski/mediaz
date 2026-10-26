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

func TestMovieMetadataStorage(t *testing.T) {
	ctx := t.Context()
	store := initSqlite(t, ctx)
	want := sqlcdb.MovieMetadatum{TmdbID: 1234, Title: "Title", ImdbID: sql.NullString{String: "tt1234", Valid: true}, Year: sql.NullInt64{Int64: 2024, Valid: true}, Overview: sql.NullString{String: "An overview", Valid: true}, Popularity: sql.NullFloat64{Float64: 4.5, Valid: true}, ReleaseDate: sql.NullTime{Time: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true}}
	id, err := store.CreateMovieMetadata(ctx, want)
	require.NoError(t, err)
	want.ID = id
	got, err := store.GetMovieMetadataByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, want, got)
	got, err = store.GetMovieMetadataByTmdbID(ctx, want.TmdbID)
	require.NoError(t, err)
	assert.Equal(t, want, got)
	list, err := store.ListMovieMetadata(ctx)
	require.NoError(t, err)
	assert.Equal(t, []sqlcdb.MovieMetadatum{want}, list)
	_, err = store.GetMovieMetadataByID(ctx, id+100)
	assert.ErrorIs(t, err, storage.ErrNotFound)
	_, err = store.GetMovieMetadataByTmdbID(ctx, 99)
	assert.ErrorIs(t, err, storage.ErrNotFound)
	require.NoError(t, store.DeleteMovieMetadata(ctx, id))
	list, err = store.ListMovieMetadata(ctx)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestSQLite_UpdateMovieMetadata(t *testing.T) {
	ctx := t.Context()
	store := initSqlite(t, ctx)
	want := sqlcdb.MovieMetadatum{TmdbID: 1234, Title: "Original", Overview: sql.NullString{String: "old", Valid: true}}
	id, err := store.CreateMovieMetadata(ctx, want)
	require.NoError(t, err)
	want.ID = id
	update := want
	update.TmdbID = 999
	update.Title = "Updated"
	update.Overview = sql.NullString{}
	update.Year = sql.NullInt64{Int64: 2025, Valid: true}
	require.NoError(t, store.UpdateMovieMetadata(ctx, update))
	want.Title = update.Title
	want.Overview = update.Overview
	want.Year = update.Year
	got, err := store.GetMovieMetadataByID(ctx, id)
	require.NoError(t, err)
	require.True(t, got.LastInfoSync.Valid)
	assert.WithinDuration(t, time.Now(), got.LastInfoSync.Time, 5*time.Second)
	want.LastInfoSync = got.LastInfoSync
	assert.Equal(t, want, got)
}
