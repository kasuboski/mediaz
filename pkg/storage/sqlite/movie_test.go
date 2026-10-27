package sqlite

import (
	"database/sql"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/pkg/ptr"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMovieStorage(t *testing.T) {
	ctx := t.Context()
	store := initSqlite(t, ctx)
	metadataID, err := store.CreateMovieMetadata(ctx, sqlcdb.MovieMetadatum{TmdbID: 1234, Title: "Title"})
	require.NoError(t, err)
	file := sqlcdb.MovieFile{Quality: "HDTV-720p", Size: 1_000_000_000, RelativePath: sql.NullString{String: "Title/Title.mkv", Valid: true}}
	fileID, err := store.CreateMovieFile(ctx, file)
	require.NoError(t, err)
	file.ID = fileID
	movie := sqlcdb.Movie{Path: sql.NullString{String: "Title", Valid: true}, Monitored: 1, QualityProfileID: 1, MovieFileID: sql.NullInt64{Int64: fileID, Valid: true}, MovieMetadataID: sql.NullInt64{Int64: metadataID, Valid: true}}
	id, err := store.CreateMovie(ctx, movie, storage.MovieStateMissing)
	require.NoError(t, err)
	movie.ID = id
	got, err := store.GetMovie(ctx, id)
	require.NoError(t, err)
	require.True(t, got.Movie.Added.Valid)
	require.False(t, got.Movie.Added.Time.IsZero())
	movie.Added = got.Movie.Added
	assert.Equal(t, sqlcdb.GetMovieByIDRow{Movie: movie, State: string(storage.MovieStateMissing)}, got)
	for range 2 {
		list, err := store.ListMovies(ctx)
		require.NoError(t, err)
		assert.Equal(t, []sqlcdb.ListMoviesRow{{Movie: movie, State: string(storage.MovieStateMissing)}}, list)
	}
	clientID, err := store.CreateDownloadClient(ctx, sqlcdb.DownloadClient{Type: "torrent", Implementation: "transmission", Host: "localhost", Scheme: "http"})
	require.NoError(t, err)
	require.NoError(t, store.UpdateMovieState(ctx, id, storage.MovieStateDownloading, &storage.TransitionStateMetadata{DownloadID: ptr.To("123"), DownloadClientID: &clientID}))
	got, err = store.GetMovie(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, sqlcdb.GetMovieByIDRow{Movie: movie, State: string(storage.MovieStateDownloading), DownloadID: sql.NullString{String: "123", Valid: true}, DownloadClientID: sql.NullInt64{Int64: clientID, Valid: true}}, got)
	filtered, err := store.ListMoviesByState(ctx, storage.MovieStateDownloading)
	require.NoError(t, err)
	expectedFiltered := []sqlcdb.ListMoviesByStatesRow{{Movie: movie, State: got.State, DownloadID: got.DownloadID, DownloadClientID: got.DownloadClientID}}
	assert.Equal(t, expectedFiltered, filtered)
	filtered, err = store.ListMoviesByState(ctx)
	require.NoError(t, err)
	assert.Equal(t, expectedFiltered, filtered)
	filtered, err = store.ListMoviesByState(ctx, storage.MovieStateNew, storage.MovieStateDiscovered, storage.MovieStateMissing, storage.MovieStateDownloaded, storage.MovieStateDownloading)
	require.NoError(t, err)
	assert.Equal(t, expectedFiltered, filtered, "a matching fifth state must not be truncated")
	filtered, err = store.ListMoviesByState(ctx, storage.MovieStateMissing)
	require.NoError(t, err)
	assert.Empty(t, filtered)
	byMeta, err := store.GetMovieByMetadataID(ctx, metadataID)
	require.NoError(t, err)
	assert.Equal(t, movie, byMeta.Movie)
	byFile, err := store.GetMovieByMovieFileID(ctx, fileID)
	require.NoError(t, err)
	assert.Equal(t, movie, byFile.Movie)
	files, err := store.GetMovieFilesByMovieName(ctx, "Title")
	require.NoError(t, err)
	require.Len(t, files, 1)
	require.False(t, files[0].DateAdded.IsZero())
	file.DateAdded = files[0].DateAdded
	assert.Equal(t, []sqlcdb.MovieFile{file}, files)
	files, err = store.ListMovieFiles(ctx)
	require.NoError(t, err)
	assert.Equal(t, []sqlcdb.MovieFile{file}, files)
	require.NoError(t, store.DeleteMovie(ctx, id))
	list, err := store.ListMovies(ctx)
	require.NoError(t, err)
	assert.Empty(t, list)
	require.NoError(t, store.DeleteMovieFile(ctx, fileID))
	files, err = store.ListMovieFiles(ctx)
	require.NoError(t, err)
	assert.Empty(t, files)
}

func TestSQLite_UpdateMovieByID(t *testing.T) {
	ctx := t.Context()
	store := initSqlite(t, ctx)
	id, err := store.CreateMovie(ctx, sqlcdb.Movie{Monitored: 1, QualityProfileID: 1, Path: sql.NullString{String: "Title/Title.mkv", Valid: true}}, storage.MovieStateDiscovered)
	require.NoError(t, err)
	before, err := store.GetMovie(ctx, id)
	require.NoError(t, err)
	require.NoError(t, store.UpdateMovieByID(ctx, id, sqlcdb.Movie{Monitored: 0, QualityProfileID: 3, Path: sql.NullString{String: "changed", Valid: true}}))
	before.Movie.Monitored = 0
	got, err := store.GetMovie(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, before, got)
	require.NoError(t, store.UpdateMovieQualityProfile(ctx, id, 3))
	before.Movie.QualityProfileID = 3
	got, err = store.GetMovie(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, before, got)
	fileID, err := store.CreateMovieFile(ctx, sqlcdb.MovieFile{Quality: "HDTV-720p", RelativePath: sql.NullString{String: "Title.mkv", Valid: true}})
	require.NoError(t, err)
	require.NoError(t, store.UpdateMovieMovieFileID(ctx, id, fileID))
	before.Movie.MovieFileID = sql.NullInt64{Int64: fileID, Valid: true}
	got, err = store.GetMovie(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, before, got)
}

func TestSQLite_CreateMovieWithAdded(t *testing.T) {
	for _, added := range []sql.NullTime{{}, {Valid: true}, {Time: time.Date(2024, 5, 1, 12, 30, 0, 0, time.UTC), Valid: true}} {
		t.Run(added.Time.String(), func(t *testing.T) {
			ctx := t.Context()
			store := initSqlite(t, ctx)
			id, err := store.CreateMovie(ctx, sqlcdb.Movie{Monitored: 1, QualityProfileID: 1, Added: added}, storage.MovieStateDiscovered)
			require.NoError(t, err)
			got, err := store.GetMovie(ctx, id)
			require.NoError(t, err)
			require.True(t, got.Movie.Added.Valid)
			if !added.Time.IsZero() {
				assert.True(t, added.Time.Equal(got.Movie.Added.Time))
			} else {
				assert.WithinDuration(t, time.Now(), got.Movie.Added.Time, 5*time.Second)
			}
		})
	}
}

func TestSQLite_GetMovieByPath(t *testing.T) {
	ctx := t.Context()
	store := initSqlite(t, ctx)
	id, err := store.CreateMovie(ctx, sqlcdb.Movie{QualityProfileID: 1, Path: sql.NullString{String: "Some Movie/Some Movie.mkv", Valid: true}}, storage.MovieStateDiscovered)
	require.NoError(t, err)
	expected, err := store.GetMovie(ctx, id)
	require.NoError(t, err)
	got, err := store.GetMovieByPath(ctx, "Some Movie/Some Movie.mkv")
	require.NoError(t, err)
	assert.Equal(t, expected.Movie, got.Movie)
	_, err = store.GetMovieByPath(ctx, "missing")
	assert.ErrorIs(t, err, storage.ErrNotFound)
	_, err = store.GetMovieByMetadataID(ctx, 424242)
	assert.ErrorIs(t, err, storage.ErrNotFound)
}
