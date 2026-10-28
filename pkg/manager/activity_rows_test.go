package manager

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/pkg/pagination"
	"github.com/kasuboski/mediaz/pkg/storage/mocks"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestActivityTransitionItemsStableTies(t *testing.T) {
	ctx := context.Background()
	store := mocks.NewMockStorage(gomock.NewController(t))
	svc := &JobService{activityStorage: store}
	stamp := sql.NullTime{Time: time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC), Valid: true}
	// Deliberately scrambled equal timestamps exercise the cross-query ordering,
	// which is not the ordering produced by any one generated SQL query.
	store.EXPECT().GetMovieTransitionItems(ctx, sqlcdb.GetMovieTransitionItemsParams{Start: "start", End: "end", Lim: 0}).Return([]sqlcdb.GetMovieTransitionItemsRow{
		{ID: 3, EntityType: "movie", EntityID: 30, CreatedAt: stamp},
		{ID: 5, EntityType: "movie", EntityID: 50, CreatedAt: stamp},
	}, nil)
	store.EXPECT().GetSeasonTransitionItems(ctx, sqlcdb.GetSeasonTransitionItemsParams{Start: "start", End: "end", Lim: 0}).Return([]sqlcdb.GetSeasonTransitionItemsRow{
		{ID: 9, EntityType: "season", EntityID: 90, CreatedAt: stamp},
	}, nil)
	store.EXPECT().GetEpisodeTransitionItems(ctx, sqlcdb.GetEpisodeTransitionItemsParams{Start: "start", End: "end", Lim: 0}).Return([]sqlcdb.GetEpisodeTransitionItemsRow{
		{ID: 2, EntityType: "episode", EntityID: 20, CreatedAt: stamp},
	}, nil)
	store.EXPECT().GetJobTransitionItems(ctx, sqlcdb.GetJobTransitionItemsParams{Start: "start", End: "end", Lim: 0}).Return([]sqlcdb.GetJobTransitionItemsRow{
		{ID: 4, EntityType: "job", EntityID: 40, CreatedAt: stamp},
	}, nil)
	rows, err := svc.fetchAndSortTransitionItems(ctx, "start", "end")
	require.NoError(t, err)
	assert.Equal(t, []*TransitionItem{
		{ID: 2, EntityType: "episode", EntityID: 20, CreatedAt: stamp.Time},
		{ID: 4, EntityType: "job", EntityID: 40, CreatedAt: stamp.Time},
		{ID: 5, EntityType: "movie", EntityID: 50, CreatedAt: stamp.Time},
		{ID: 3, EntityType: "movie", EntityID: 30, CreatedAt: stamp.Time},
		{ID: 9, EntityType: "season", EntityID: 90, CreatedAt: stamp.Time},
	}, rows)
}

func TestActivityEmptyPublicCollections(t *testing.T) {
	ctx := context.Background()
	svc := &JobService{activityStorage: newActivityTestStorage(t, ctx)}
	timeline, err := svc.GetActivityTimeline(ctx, 1, pagination.Params{})
	require.NoError(t, err)
	assert.Equal(t, &TimelineResponse{Timeline: []*TimelineEntry{}, Transitions: []*TransitionItem{}}, timeline)
	active, err := svc.GetActiveActivity(ctx, ActivePaginationParams{Movies: pagination.Params{Page: 1, PageSize: 50}, Series: pagination.Params{Page: 1, PageSize: 50}, Jobs: pagination.Params{Page: 1, PageSize: 50}})
	require.NoError(t, err)
	assert.Equal(t, &ActiveActivityResponse{Movies: []*ActiveMovie{}, Series: []*ActiveSeries{}, Jobs: []*ActiveJob{}, Pagination: ActiveActivityPagination{Movies: pagination.Meta{Page: 1, PageSize: 50, TotalItems: 0, TotalPages: 0}, Series: pagination.Meta{Page: 1, PageSize: 50, TotalItems: 0, TotalPages: 0}, Jobs: pagination.Meta{Page: 1, PageSize: 50, TotalItems: 0, TotalPages: 0}}}, active)
	history, err := svc.GetEntityTransitionHistory(ctx, "movie", 999, pagination.Params{Page: 1, PageSize: 50})
	require.NoError(t, err)
	assert.Equal(t, &HistoryResponse{History: []*HistoryEntry{}, Pagination: pagination.Meta{Page: 1, PageSize: 50, TotalItems: 0, TotalPages: 0}}, history)
}

func TestActivityActiveRowsUseTransitionTimestamps(t *testing.T) {
	stamp := time.Date(2020, 2, 3, 4, 5, 6, 0, time.UTC)
	rows := []sqlcdb.ListDownloadingMoviesRow{{
		ID: 10, TmdbID: 20, Title: "Movie", Year: sql.NullInt64{Int64: 2025, Valid: true}, Images: `{"poster_path":"/poster.jpg"}`,
		ToState: "downloading", SortKey: 1, CreatedAt: sql.NullTime{Time: stamp, Valid: true}, DownloadID: sql.NullString{String: "download", Valid: true},
		DcID: sql.NullInt64{Int64: 30, Valid: true}, DcHost: sql.NullString{String: "localhost", Valid: true}, DcPort: sql.NullInt64{Int64: 8080, Valid: true},
	}}
	movies := transformActiveMovies(rows)
	require.Len(t, movies, 1)
	assert.Equal(t, []*ActiveMovie{{ID: 10, TMDBID: 20, Title: "Movie", Year: 2025, PosterPath: "/poster.jpg", State: "downloading", StateSince: stamp,
		Duration: formatDuration(time.Since(stamp)), DownloadID: "download", DownloadClient: &DownloadClientInfo{ID: 30, Host: "localhost", Port: 8080}}}, movies)
	series := transformActiveSeries([]sqlcdb.ListDownloadingSeriesRow{{ID: 40, TmdbID: sql.NullInt64{Int64: 50, Valid: true}, Title: sql.NullString{String: "Series", Valid: true},
		PosterPath: sql.NullString{String: "/series.jpg", Valid: true}, ToState: "downloading", SortKey: 1, CreatedAt: sql.NullTime{Time: stamp, Valid: true}, SeasonNumber: 3}})
	assert.Equal(t, []*ActiveSeries{{ID: 40, TMDBID: 50, Title: "Series", PosterPath: "/series.jpg", State: "downloading", StateSince: stamp, Duration: formatDuration(time.Since(stamp))}}, series)
}

func TestActivityHistoryDurations(t *testing.T) {
	start := time.Now().Add(-5 * time.Minute).Truncate(time.Second)
	entries := []*HistoryEntry{{SortKey: 1, ToState: "missing", CreatedAt: start}, {SortKey: 2, ToState: "downloading", CreatedAt: start.Add(time.Minute)}}
	assert.Equal(t, []*HistoryEntry{
		{SortKey: 1, ToState: "missing", CreatedAt: start, Duration: "1m"},
		{SortKey: 2, ToState: "downloading", CreatedAt: start.Add(time.Minute), Duration: formatDuration(time.Since(start.Add(time.Minute)))},
	}, calculateDurations(entries))
}

func TestActivityTimelineRejectsNegativePagination(t *testing.T) {
	svc := &JobService{}
	for _, args := range [][2]int{{-1, 0}, {0, -1}} {
		_, err := svc.getTransitionsByDate(context.Background(), time.Now(), time.Now(), args[0], args[1])
		require.ErrorContains(t, err, "nonnegative")
	}
}
