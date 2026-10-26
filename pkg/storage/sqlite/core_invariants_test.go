package sqlite

import (
	"database/sql"
	"sync"
	"testing"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConcurrentMovieTransitionsValidateLockedCurrentState(t *testing.T) {
	ctx := t.Context()
	store := initSqlite(t, ctx)
	id, err := store.CreateMovie(ctx, sqlcdb.Movie{QualityProfileID: 1}, storage.MovieStateMissing)
	require.NoError(t, err)
	const workers = 16
	start := make(chan struct{})
	results := make(chan error, workers)
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() { <-start; results <- store.UpdateMovieState(ctx, id, storage.MovieStateDownloading, nil) })
	}
	close(start)
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	require.Equal(t, 1, successes, "all writers must validate the state read under their transaction lock")
	current, err := store.GetMovie(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, string(storage.MovieStateDownloading), current.State)
	history, err := store.GetEntityTransitionsMovie(ctx, id)
	require.NoError(t, err)
	require.Len(t, history, 2)
	assert.Equal(t, sql.NullString{String: string(storage.MovieStateMissing), Valid: true}, history[1].FromState)
}

func TestMovieDownloadMetadataColumnsAreIndependentlyNullable(t *testing.T) {
	ctx := t.Context()
	store := initSqlite(t, ctx)
	id, err := store.CreateMovie(ctx, sqlcdb.Movie{QualityProfileID: 1}, storage.MovieStateMissing)
	require.NoError(t, err)
	before, err := store.GetMovie(ctx, id)
	require.NoError(t, err)
	downloadID := "download-without-client"
	require.NoError(t, store.UpdateMovieState(ctx, id, storage.MovieStateDownloading, &storage.TransitionStateMetadata{DownloadID: &downloadID}))
	expected := before
	expected.State = string(storage.MovieStateDownloading)
	expected.DownloadID = sql.NullString{String: downloadID, Valid: true}
	got, err := store.GetMovie(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, expected, got)
	require.NoError(t, store.UpdateMovieState(ctx, id, storage.MovieStateDownloaded, nil))
	expected.State = string(storage.MovieStateDownloaded)
	expected.DownloadID = sql.NullString{}
	got, err = store.GetMovie(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, expected, got)
}
