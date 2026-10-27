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

func TestEpisodeFileStorage(t *testing.T) {
	ctx := t.Context()
	store := initSqlite(t, ctx)
	file := sqlcdb.EpisodeFile{Quality: "HDTV-720p", Size: 1_000_000_000, RelativePath: sql.NullString{String: "Series/Season 1/episode.mkv", Valid: true}, OriginalFilePath: sql.NullString{String: "/downloads/episode.mkv", Valid: true}}
	id, err := store.CreateEpisodeFile(ctx, file)
	require.NoError(t, err)
	file.ID = id
	got, err := store.GetEpisodeFile(ctx, id)
	require.NoError(t, err)
	require.False(t, got.Added.IsZero())
	assert.WithinDuration(t, time.Now(), got.Added, 5*time.Second)
	file.Added = got.Added
	assert.Equal(t, file, got)
	byID, err := store.GetEpisodeFileByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, []sqlcdb.EpisodeFile{file}, byID)
	list, err := store.ListEpisodeFiles(ctx)
	require.NoError(t, err)
	assert.Equal(t, []sqlcdb.EpisodeFile{file}, list)
	require.NoError(t, store.DeleteEpisodeFile(ctx, id))
	list, err = store.ListEpisodeFiles(ctx)
	require.NoError(t, err)
	assert.Empty(t, list)
	_, err = store.GetEpisodeFile(ctx, id)
	assert.ErrorIs(t, err, storage.ErrNotFound)
	_, err = store.GetEpisodeFileByID(ctx, id)
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestUpdateEpisodeFile(t *testing.T) {
	ctx := t.Context()
	store := initSqlite(t, ctx)
	id, err := store.CreateEpisodeFile(ctx, sqlcdb.EpisodeFile{Quality: "HDTV-720p", Size: 100, RelativePath: sql.NullString{String: "old.mkv", Valid: true}})
	require.NoError(t, err)
	file, err := store.GetEpisodeFile(ctx, id)
	require.NoError(t, err)
	file.Quality = "Bluray-1080p"
	file.Size = 200
	file.RelativePath = sql.NullString{String: "new.mkv", Valid: true}
	file.OriginalFilePath = sql.NullString{String: "/downloads/new.mkv", Valid: true}
	file.Added = time.Date(2024, 5, 1, 12, 30, 0, 0, time.UTC)
	require.NoError(t, store.UpdateEpisodeFile(ctx, id, file))
	got, err := store.GetEpisodeFile(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, file, got)
	file.RelativePath = sql.NullString{}
	file.OriginalFilePath = sql.NullString{}
	require.NoError(t, store.UpdateEpisodeFile(ctx, id, file))
	got, err = store.GetEpisodeFile(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, file, got)
}

func TestCreateEpisodeFileWithExplicitID(t *testing.T) {
	ctx := t.Context()
	store := initSqlite(t, ctx)
	want := sqlcdb.EpisodeFile{ID: 501, Quality: "HDTV-720p", Size: 100, Added: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), RelativePath: sql.NullString{String: "episode.mkv", Valid: true}}
	id, err := store.CreateEpisodeFile(ctx, want)
	require.NoError(t, err)
	assert.Equal(t, want.ID, id)
	got, err := store.GetEpisodeFile(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, want, got)

	// A conflicting create must persist the new values, not perform the old
	// no-op update against the existing row's own columns.
	want.Quality = "Bluray-1080p"
	want.Size = 200
	want.RelativePath = sql.NullString{String: "replacement.mkv", Valid: true}
	upsertID, err := store.CreateEpisodeFile(ctx, want)
	require.NoError(t, err)
	assert.Equal(t, id, upsertID)
	got, err = store.GetEpisodeFile(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}
