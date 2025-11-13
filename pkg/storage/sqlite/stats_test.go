package sqlite

import (
	"context"
	"fmt"
	"testing"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetMovieStatsByState_Empty verifies correct handling when no movies exist
func TestGetMovieStatsByState_Empty(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)

	stats, err := store.GetMovieStatsByState(ctx)

	require.NoError(t, err)
	assert.Empty(t, stats)
}

// TestGetMovieStatsByState_SingleMovieSingleState tests basic functionality with minimal data
func TestGetMovieStatsByState_SingleMovieSingleState(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)

	createTestMovie(t, store, ctx, storage.MovieStateMissing)

	stats, err := store.GetMovieStatsByState(ctx)

	require.NoError(t, err)
	require.Len(t, stats, 1)
	assert.Equal(t, "missing", stats[0].State)
	assert.Equal(t, 1, stats[0].Count)
}

// TestGetMovieStatsByState_MultipleStates verifies aggregation across multiple states
func TestGetMovieStatsByState_MultipleStates(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)

	// Create test data - use valid state transitions
	// Missing -> Discovered -> Downloading -> Downloaded
	createTestMovie(t, store, ctx, storage.MovieStateMissing)
	createTestMovie(t, store, ctx, storage.MovieStateMissing)
	createTestMovie(t, store, ctx, storage.MovieStateMissing)

	// Move some to discovered
	createTestMovieInState(t, store, ctx, storage.MovieStateDiscovered)
	createTestMovieInState(t, store, ctx, storage.MovieStateDiscovered)

	// Move some to downloading
	createTestMovieInState(t, store, ctx, storage.MovieStateDownloading)

	stats, err := store.GetMovieStatsByState(ctx)

	require.NoError(t, err)

	// Convert to map for easier assertions
	stateMap := make(map[string]int)
	for _, stat := range stats {
		stateMap[stat.State] = stat.Count
	}

	assert.Equal(t, 3, stateMap["missing"])
	assert.Equal(t, 2, stateMap["discovered"])
	assert.Equal(t, 1, stateMap["downloading"])
}

// TestGetTVStatsByState_Empty verifies correct handling when no series exist
func TestGetTVStatsByState_Empty(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)

	stats, err := store.GetTVStatsByState(ctx)

	require.NoError(t, err)
	assert.Empty(t, stats)
}

// TestGetTVStatsByState_SingleSeriesSingleState tests basic functionality with minimal data
func TestGetTVStatsByState_SingleSeriesSingleState(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)

	createTestSeries(t, store, ctx, storage.SeriesStateMissing)

	stats, err := store.GetTVStatsByState(ctx)

	require.NoError(t, err)
	require.Len(t, stats, 1)
	assert.Equal(t, "missing", stats[0].State)
	assert.Equal(t, 1, stats[0].Count)
}

// TestGetTVStatsByState_MultipleStates verifies aggregation across multiple TV states
func TestGetTVStatsByState_MultipleStates(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)

	// Create series in various states using valid transitions
	createTestSeries(t, store, ctx, storage.SeriesStateMissing)
	createTestSeries(t, store, ctx, storage.SeriesStateMissing)

	createTestSeriesInState(t, store, ctx, storage.SeriesStateDiscovered)
	createTestSeriesInState(t, store, ctx, storage.SeriesStateDiscovered)
	createTestSeriesInState(t, store, ctx, storage.SeriesStateDiscovered)

	createTestSeriesInState(t, store, ctx, storage.SeriesStateDownloading)

	stats, err := store.GetTVStatsByState(ctx)

	require.NoError(t, err)

	stateMap := make(map[string]int)
	for _, stat := range stats {
		stateMap[stat.State] = stat.Count
	}

	assert.Equal(t, 2, stateMap["missing"])
	assert.Equal(t, 3, stateMap["discovered"])
	assert.Equal(t, 1, stateMap["downloading"])
}

// TestGetLibraryStats_Empty verifies behavior with no data
func TestGetLibraryStats_Empty(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)

	stats, err := store.GetLibraryStats(ctx)

	require.NoError(t, err)
	require.NotNil(t, stats)

	assert.Equal(t, 0, stats.Movies.Total)
	assert.Empty(t, stats.Movies.ByState)
	assert.Equal(t, 0, stats.TV.Total)
	assert.Empty(t, stats.TV.ByState)
}

// TestGetLibraryStats_MoviesOnly verifies partial data handling with only movies
func TestGetLibraryStats_MoviesOnly(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)

	createTestMovie(t, store, ctx, storage.MovieStateMissing)
	createTestMovie(t, store, ctx, storage.MovieStateMissing)
	createTestMovie(t, store, ctx, storage.MovieStateMissing)
	createTestMovie(t, store, ctx, storage.MovieStateMissing)

	createTestMovieInState(t, store, ctx, storage.MovieStateDiscovered)
	createTestMovieInState(t, store, ctx, storage.MovieStateDiscovered)

	stats, err := store.GetLibraryStats(ctx)

	require.NoError(t, err)
	require.NotNil(t, stats)

	assert.Equal(t, 6, stats.Movies.Total)
	assert.Equal(t, 4, stats.Movies.ByState[storage.MovieStateMissing])
	assert.Equal(t, 2, stats.Movies.ByState[storage.MovieStateDiscovered])

	assert.Equal(t, 0, stats.TV.Total)
	assert.Empty(t, stats.TV.ByState)
}

// TestGetLibraryStats_TVOnly verifies partial data handling with only TV series
func TestGetLibraryStats_TVOnly(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)

	createTestSeries(t, store, ctx, storage.SeriesStateMissing)
	createTestSeries(t, store, ctx, storage.SeriesStateMissing)

	createTestSeriesInState(t, store, ctx, storage.SeriesStateDiscovered)
	createTestSeriesInState(t, store, ctx, storage.SeriesStateDiscovered)
	createTestSeriesInState(t, store, ctx, storage.SeriesStateDiscovered)

	stats, err := store.GetLibraryStats(ctx)

	require.NoError(t, err)
	require.NotNil(t, stats)

	assert.Equal(t, 0, stats.Movies.Total)
	assert.Empty(t, stats.Movies.ByState)

	assert.Equal(t, 5, stats.TV.Total)
	assert.Equal(t, 2, stats.TV.ByState[storage.SeriesStateMissing])
	assert.Equal(t, 3, stats.TV.ByState[storage.SeriesStateDiscovered])
}

// TestGetLibraryStats_Complete tests full integration with comprehensive dataset
func TestGetLibraryStats_Complete(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)

	// Create movies with valid transitions
	createTestMovie(t, store, ctx, storage.MovieStateMissing)
	createTestMovie(t, store, ctx, storage.MovieStateMissing)
	createTestMovie(t, store, ctx, storage.MovieStateMissing)

	createTestMovieInState(t, store, ctx, storage.MovieStateDiscovered)
	createTestMovieInState(t, store, ctx, storage.MovieStateDiscovered)

	createTestMovieInState(t, store, ctx, storage.MovieStateDownloading)

	// Create TV series with valid transitions
	createTestSeries(t, store, ctx, storage.SeriesStateMissing)
	createTestSeries(t, store, ctx, storage.SeriesStateMissing)

	createTestSeriesInState(t, store, ctx, storage.SeriesStateDiscovered)
	createTestSeriesInState(t, store, ctx, storage.SeriesStateDiscovered)

	createTestSeriesInState(t, store, ctx, storage.SeriesStateDownloading)

	stats, err := store.GetLibraryStats(ctx)

	require.NoError(t, err)
	require.NotNil(t, stats)

	// Verify movies
	assert.Equal(t, 6, stats.Movies.Total)
	assert.Equal(t, 3, stats.Movies.ByState[storage.MovieStateMissing])
	assert.Equal(t, 2, stats.Movies.ByState[storage.MovieStateDiscovered])
	assert.Equal(t, 1, stats.Movies.ByState[storage.MovieStateDownloading])

	// Verify TV
	assert.Equal(t, 5, stats.TV.Total)
	assert.Equal(t, 2, stats.TV.ByState[storage.SeriesStateMissing])
	assert.Equal(t, 2, stats.TV.ByState[storage.SeriesStateDiscovered])
	assert.Equal(t, 1, stats.TV.ByState[storage.SeriesStateDownloading])
}

// TestGetLibraryStats_StateTySafety verifies proper state types in map keys
func TestGetLibraryStats_StateTySafety(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)

	createTestMovie(t, store, ctx, storage.MovieStateMissing)
	createTestSeries(t, store, ctx, storage.SeriesStateMissing)

	stats, err := store.GetLibraryStats(ctx)

	require.NoError(t, err)

	// Verify map keys are proper types (not strings)
	for key := range stats.Movies.ByState {
		// This will compile only if key is of type MovieState
		_ = storage.MovieState(key)
	}

	for key := range stats.TV.ByState {
		// This will compile only if key is of type SeriesState
		_ = storage.SeriesState(key)
	}
}

// TestGetLibraryStats_TotalCalculationAccuracy verifies Total equals sum of ByState counts
func TestGetLibraryStats_TotalCalculationAccuracy(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)

	// Create diverse dataset
	createTestMovie(t, store, ctx, storage.MovieStateMissing)
	createTestMovie(t, store, ctx, storage.MovieStateMissing)
	createTestMovieInState(t, store, ctx, storage.MovieStateDiscovered)
	createTestMovieInState(t, store, ctx, storage.MovieStateDiscovered)
	createTestMovieInState(t, store, ctx, storage.MovieStateDownloading)

	createTestSeries(t, store, ctx, storage.SeriesStateMissing)
	createTestSeriesInState(t, store, ctx, storage.SeriesStateDiscovered)
	createTestSeriesInState(t, store, ctx, storage.SeriesStateDiscovered)
	createTestSeriesInState(t, store, ctx, storage.SeriesStateDownloading)

	stats, err := store.GetLibraryStats(ctx)

	require.NoError(t, err)

	// Verify movie total equals sum of states
	movieSum := 0
	for _, count := range stats.Movies.ByState {
		movieSum += count
	}
	assert.Equal(t, stats.Movies.Total, movieSum, "Movies total should equal sum of ByState counts")

	// Verify TV total equals sum of states
	tvSum := 0
	for _, count := range stats.TV.ByState {
		tvSum += count
	}
	assert.Equal(t, stats.TV.Total, tvSum, "TV total should equal sum of ByState counts")
}

// TestGetMovieStatsByState_ContextCancellation verifies graceful handling of cancelled context
func TestGetMovieStatsByState_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	store := initSqlite(t, ctx)

	cancel()

	_, err := store.GetMovieStatsByState(ctx)

	assert.Error(t, err)
}

// TestGetTVStatsByState_ContextCancellation verifies graceful handling of cancelled context
func TestGetTVStatsByState_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	store := initSqlite(t, ctx)

	cancel()

	_, err := store.GetTVStatsByState(ctx)

	assert.Error(t, err)
}

// TestGetLibraryStats_ContextCancellation verifies graceful handling with parallel goroutines
func TestGetLibraryStats_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	store := initSqlite(t, ctx)

	cancel()

	_, err := store.GetLibraryStats(ctx)

	assert.Error(t, err)
}

// Helper functions

var movieCounter int
var seriesCounter int

// createTestMovie creates a movie in the specified initial state
func createTestMovie(t *testing.T, store storage.Storage, ctx context.Context, initialState storage.MovieState) int64 {
	t.Helper()
	movieCounter++
	path := fmt.Sprintf("path/to/movie_%d", movieCounter)
	movie := storage.Movie{
		Movie: model.Movie{
			Path:      &path,
			Monitored: 1,
		},
	}
	id, err := store.CreateMovie(ctx, movie, initialState)
	require.NoError(t, err)
	return id
}

// createTestMovieInState creates a movie and transitions it to the target state
func createTestMovieInState(t *testing.T, store storage.Storage, ctx context.Context, targetState storage.MovieState) int64 {
	t.Helper()
	// Create in missing state first (safe starting point)
	id := createTestMovie(t, store, ctx, storage.MovieStateMissing)

	// Transition to target state if needed
	if targetState != storage.MovieStateMissing {
		err := store.UpdateMovieState(ctx, id, targetState, nil)
		require.NoError(t, err)
	}
	return id
}

// createTestSeries creates a series in the specified initial state
func createTestSeries(t *testing.T, store storage.Storage, ctx context.Context, initialState storage.SeriesState) int64 {
	t.Helper()
	seriesCounter++
	path := fmt.Sprintf("path/to/series_%d", seriesCounter)
	series := storage.Series{
		Series: model.Series{
			Path:             &path,
			Monitored:        1,
			QualityProfileID: 1,
		},
	}
	id, err := store.CreateSeries(ctx, series, initialState)
	require.NoError(t, err)
	return id
}

// createTestSeriesInState creates a series and transitions it to the target state
func createTestSeriesInState(t *testing.T, store storage.Storage, ctx context.Context, targetState storage.SeriesState) int64 {
	t.Helper()
	// Create in missing state first (safe starting point)
	id := createTestSeries(t, store, ctx, storage.SeriesStateMissing)

	// Transition to target state if needed
	if targetState != storage.SeriesStateMissing {
		err := store.UpdateSeriesState(ctx, id, targetState, nil)
		require.NoError(t, err)
	}
	return id
}
