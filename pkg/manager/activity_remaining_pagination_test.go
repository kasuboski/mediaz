package manager

import (
	"context"
	"database/sql"
	"testing"

	"github.com/kasuboski/mediaz/pkg/pagination"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/stretchr/testify/require"
)

// Each active collection has its own page and total; paging one collection
// must not change either of the others.
func TestActiveActivityIndependentPaginationRegression(t *testing.T) {
	ctx := context.Background()
	store := newActivityTestStorage(t, ctx)
	svc := &JobService{activityStorage: store}

	for i := int64(1); i <= 5; i++ {
		_, err := store.CreateMovieMetadata(ctx, sqlcdb.MovieMetadatum{TmdbID: i, Title: "Movie", Images: "[]"})
		require.NoError(t, err)
		id, err := store.CreateMovie(ctx, sqlcdb.Movie{QualityProfileID: 1, Path: sql.NullString{String: "movie", Valid: true}, Monitored: 1}, storage.MovieStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.LinkMovieMetadata(ctx, id, i))
		require.NoError(t, store.UpdateMovieState(ctx, id, storage.MovieStateDownloading, nil))
	}
	for i := int64(1); i <= 3; i++ {
		id, err := store.CreateSeries(ctx, sqlcdb.Series{QualityProfileID: 1, Monitored: 1}, storage.SeriesStateMissing)
		require.NoError(t, err)
		_, err = store.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{TmdbID: i, Title: "Series"})
		require.NoError(t, err)
		require.NoError(t, store.LinkSeriesMetadata(ctx, id, i))
		seasonID, err := store.CreateSeason(ctx, sqlcdb.Season{SeriesID: id, SeasonNumber: 1}, storage.SeasonStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.UpdateSeasonState(ctx, seasonID, storage.SeasonStateDownloading, nil))
	}
	for i := 0; i < 2; i++ {
		id, err := store.CreateJob(ctx, sqlcdb.Job{Type: "job"}, storage.JobStatePending)
		require.NoError(t, err)
		require.NoError(t, store.UpdateJobState(ctx, id, storage.JobStateRunning, nil))
	}

	resp, err := svc.GetActiveActivity(ctx, ActivePaginationParams{
		Movies: pagination.Params{Page: 2, PageSize: 2}, Series: pagination.Params{Page: 2, PageSize: 2}, Jobs: pagination.Params{Page: 2, PageSize: 1},
	})
	require.NoError(t, err)
	require.Len(t, resp.Movies, 2)
	require.Len(t, resp.Series, 1)
	require.Len(t, resp.Jobs, 1)
	require.Equal(t, ActiveActivityCounts{Movies: 5, Series: 3, Jobs: 2, Total: 10}, resp.Counts)
}

func TestActivityHistoryOutOfRangePreservesEntityRegression(t *testing.T) {
	ctx := context.Background()
	store := newActivityTestStorage(t, ctx)
	svc := &JobService{activityStorage: store}
	resp, err := svc.GetEntityTransitionHistory(ctx, "movie", 999, pagination.Params{Page: 2, PageSize: 2})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, pagination.Meta{Page: 2, PageSize: 2, TotalItems: 0, TotalPages: 0}, resp.Pagination)
	require.Empty(t, resp.History)
}
