package manager

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/pkg/pagination"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/stretchr/testify/require"
)

// Exercise history paging against SQLite, including the duration attached to
// the entry immediately before a page boundary (which depends on its successor).
func TestMovieHistoryPaginationUsesFullHistoryForDurationsAndCount(t *testing.T) {
	ctx := context.Background()
	store, db := newStampedActivityTestStorage(t, ctx)
	svc := &JobService{activityStorage: store}

	metadataID, err := store.CreateMovieMetadata(ctx, sqlcdb.MovieMetadatum{TmdbID: 700, Title: "History Movie", Images: `{"poster_path":"/history.jpg"}`})
	require.NoError(t, err)
	id, err := store.CreateMovie(ctx, sqlcdb.Movie{QualityProfileID: 1, Path: sql.NullString{String: "history", Valid: true}, Monitored: 1}, storage.MovieStateMissing)
	require.NoError(t, err)
	require.NoError(t, store.LinkMovieMetadata(ctx, id, metadataID))

	firstAt := time.Now().UTC().Add(-5 * time.Minute).Truncate(time.Second)
	stampMostRecentTransition(t, db, "movie_transition", "movie_id", id, firstAt)
	require.NoError(t, store.UpdateMovieState(ctx, id, storage.MovieStateDownloading, nil))
	// The middle row's duration is determined by this exact 10-second interval.
	stampMostRecentTransition(t, db, "movie_transition", "movie_id", id, firstAt.Add(10*time.Second))
	require.NoError(t, store.UpdateMovieState(ctx, id, storage.MovieStateDownloaded, nil))
	stampMostRecentTransition(t, db, "movie_transition", "movie_id", id, firstAt.Add(20*time.Second))

	first, err := svc.GetEntityTransitionHistory(ctx, "movie", id, pagination.Params{Page: 1, PageSize: 2})
	require.NoError(t, err)
	require.Len(t, first.History, 2)
	require.Equal(t, 3, first.Pagination.TotalItems)
	require.Equal(t, "10s", first.History[1].Duration)
	require.Equal(t, &EntityInfo{Type: "movie", ID: id, Title: "History Movie", PosterPath: `{"poster_path":"/history.jpg"}`}, first.Entity)

	second, err := svc.GetEntityTransitionHistory(ctx, "movie", id, pagination.Params{Page: 2, PageSize: 2})
	require.NoError(t, err)
	require.Len(t, second.History, 1)
	require.Equal(t, 3, second.Pagination.TotalItems)
	require.Equal(t, "downloaded", second.History[0].ToState)
	require.LessOrEqual(t, parseRenderedDuration(t, second.History[0].Duration), time.Since(firstAt.Add(20*time.Second))+time.Second)

	out, err := svc.GetEntityTransitionHistory(ctx, "movie", id, pagination.Params{Page: 99, PageSize: 2})
	require.NoError(t, err)
	require.Empty(t, out.History)
	require.Equal(t, first.Entity, out.Entity)
	require.Equal(t, 3, out.Pagination.TotalItems)
}
