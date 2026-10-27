package sqlite

// End-to-end SQLite integration regression tests for the sqlc persistence
// migration (issue #181).
//
// These tests exercise the public storage API against a real in-memory SQLite
// database with the full migration chain applied. They guard the invariants
// the sqlc migration must preserve:
//
//  1. Database-level defaults and timestamps (CURRENT_TIMESTAMP columns) must
//     keep working, i.e. persistence code must leave default columns out of
//     INSERT statements instead of writing zero values over them.
//  2. The partial unique indexes on the transition tables must keep enforcing
//     at most one most_recent transition per parent and unique
//     (parent, sort_key) history entries.
//  3. State transitions are transactional: if the new transition row cannot be
//     inserted, the whole transaction must roll back, leaving the previous
//     transition intact (no flipped most_recent flags, no partial rows, no
//     orphaned parents).
//  4. DeleteIndexerSourceCascade removes a source and its child indexers
//     atomically: a failure part-way through must leave the source and all
//     children in place.
//
// Mid-transaction failures are simulated with test-only RAISE(ABORT) triggers
// created over the raw database handle; they are dropped when the test ends.
// Everything else goes through the storage API, never through generated
// query helpers.

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/pkg/ptr"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sqlcIntegrationStore returns a fully migrated in-memory store pinned to a
// single connection. Pinning guarantees the test-only triggers below are
// visible to every statement the store executes, and it makes transaction
// leaks loud: any base-DB query issued while a transaction holds the only
// connection blocks until the test timeout instead of silently hitting a
// different, empty in-memory database.
func sqlcIntegrationStore(t *testing.T) *SQLite {
	t.Helper()
	store := initSqlite(t, context.Background())
	s := store.(*SQLite)
	s.db.SetMaxOpenConns(1)
	return s
}

// installAbortTrigger installs a test-only trigger that aborts every <event>
// (e.g. INSERT) on <table>, simulating a mid-transaction failure.
func installAbortTrigger(t *testing.T, s *SQLite, trigger, table, event string) {
	t.Helper()
	stmt := fmt.Sprintf(
		`CREATE TRIGGER IF NOT EXISTS %s BEFORE %s ON %s BEGIN SELECT RAISE(ABORT, 'injected failure: %s %s'); END;`,
		trigger, event, table, event, table)
	_, err := s.db.Exec(stmt)
	require.NoError(t, err)
}

func dropTrigger(t *testing.T, s *SQLite, trigger string) {
	t.Helper()
	_, err := s.db.Exec(fmt.Sprintf(`DROP TRIGGER IF EXISTS %s`, trigger))
	require.NoError(t, err)
}

func movieTransitionRows(t *testing.T, s *SQLite, movieID int64) []sqlcdb.MovieTransition {
	t.Helper()
	rows, err := s.db.Query(
		`SELECT id, movie_id, to_state, from_state, most_recent, sort_key, download_client_id, download_id, created_at, updated_at
		 FROM movie_transition WHERE movie_id = ? ORDER BY sort_key`, movieID)
	require.NoError(t, err)
	defer rows.Close()

	var out []sqlcdb.MovieTransition
	for rows.Next() {
		var r sqlcdb.MovieTransition
		require.NoError(t, rows.Scan(&r.ID, &r.MovieID, &r.ToState, &r.FromState, &r.MostRecent, &r.SortKey,
			&r.DownloadClientID, &r.DownloadID, &r.CreatedAt, &r.UpdatedAt))
		out = append(out, r)
	}
	require.NoError(t, rows.Err())
	return out
}

func seriesTransitionRows(t *testing.T, s *SQLite, seriesID int64) []sqlcdb.SeriesTransition {
	t.Helper()
	rows, err := s.db.Query(
		`SELECT id, series_id, to_state, from_state, most_recent, sort_key, created_at, updated_at
		 FROM series_transition WHERE series_id = ? ORDER BY sort_key`, seriesID)
	require.NoError(t, err)
	defer rows.Close()

	var out []sqlcdb.SeriesTransition
	for rows.Next() {
		var r sqlcdb.SeriesTransition
		require.NoError(t, rows.Scan(&r.ID, &r.SeriesID, &r.ToState, &r.FromState, &r.MostRecent, &r.SortKey, &r.CreatedAt, &r.UpdatedAt))
		out = append(out, r)
	}
	require.NoError(t, rows.Err())
	return out
}

func countRows(t *testing.T, s *SQLite, table string) int64 {
	t.Helper()
	var n int64
	err := s.db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM %s`, table)).Scan(&n)
	require.NoError(t, err)
	return n
}

func countMostRecent(t *testing.T, s *SQLite, table, parentColumn string, parentID int64) int64 {
	t.Helper()
	var n int64
	query := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE %s = ? AND most_recent = 1`, table, parentColumn)
	err := s.db.QueryRow(query, parentID).Scan(&n)
	require.NoError(t, err)
	return n
}

// TestSqlcIntegration_MigrationPreservesDefaultsAndTimestamps verifies that the
// database defaults that survive the sqlc migration still apply: parents get
// CURRENT_TIMESTAMP added dates, transition rows get database-generated
// created_at/updated_at timestamps, and history rows keep their timestamps
// when a transition supersedes them.
func TestSqlcIntegration_MigrationPreservesDefaultsAndTimestamps(t *testing.T) {
	ctx := context.Background()
	s := sqlcIntegrationStore(t)

	_, err := s.CreateDownloadClient(ctx, sqlcdb.DownloadClient{
		Type: "nzb", Implementation: "sabnzbd", Scheme: "http", Host: "localhost", Port: 8080,
	})
	require.NoError(t, err)

	path := "Defaults/defaults.mkv"
	movieID, err := s.CreateMovie(ctx, sqlcdb.Movie{Path: sql.NullString{String: path, Valid: true}, Monitored: 1, QualityProfileID: 1}, storage.MovieStateMissing)
	require.NoError(t, err)

	movie, err := s.GetMovie(ctx, movieID)
	require.NoError(t, err)

	// movie.added must come from the DEFAULT CURRENT_TIMESTAMP, not be NULL.
	require.True(t, movie.Movie.Added.Valid, "movie.added default must be applied")
	assert.False(t, movie.Movie.Added.Time.IsZero(), "movie.added default must be a real timestamp")

	// The initial transition row must carry database-generated timestamps.
	rows := movieTransitionRows(t, s, movieID)
	require.Len(t, rows, 1)
	initial := rows[0]
	assert.False(t, initial.CreatedAt.Time.IsZero(), "created_at default must be applied")
	assert.False(t, initial.UpdatedAt.Time.IsZero(), "updated_at default must be applied")
	assert.Equal(t, sqlcdb.MovieTransition{
		ID:         initial.ID,
		MovieID:    movieID,
		ToState:    "missing",
		MostRecent: true,
		SortKey:    1,
		CreatedAt:  initial.CreatedAt,
		UpdatedAt:  initial.UpdatedAt,
	}, initial)

	require.NoError(t, s.UpdateMovieState(ctx, movieID, storage.MovieStateDownloading,
		&storage.TransitionStateMetadata{
			DownloadClientID: ptr.To(int64(1)),
			DownloadID:       ptr.To("dl-42"),
		}))

	rows = movieTransitionRows(t, s, movieID)
	require.Len(t, rows, 2)

	// The superseded row keeps its history and its timestamps.
	assert.False(t, rows[0].MostRecent)
	assert.False(t, rows[0].UpdatedAt.Time.IsZero(), "superseded transition must keep a non-zero updated_at")

	// The new row must get its timestamps from the database defaults, and
	// updated_at must never trail created_at.
	latest := rows[1]
	assert.True(t, latest.MostRecent)
	assert.Equal(t, "downloading", latest.ToState)
	assert.Equal(t, "dl-42", latest.DownloadID.String)
	assert.Equal(t, int64(1), latest.DownloadClientID.Int64)
	assert.False(t, latest.CreatedAt.Time.IsZero(), "created_at default must be applied to new transitions")
	assert.False(t, latest.UpdatedAt.Time.IsZero(), "updated_at default must be applied to new transitions")
	assert.False(t, latest.UpdatedAt.Time.Before(latest.CreatedAt.Time),
		"updated_at must not be before created_at")
}

// TestSqlcIntegration_MovieFileDateAddedDefault verifies that movie_file
// keeps its database-generated date_added when created through the API.
func TestSqlcIntegration_MovieFileDateAddedDefault(t *testing.T) {
	ctx := context.Background()
	s := sqlcIntegrationStore(t)

	fileID, err := s.CreateMovieFile(ctx, sqlcdb.MovieFile{
		Quality:      "HDTV-720p",
		Size:         1_000_000_000,
		RelativePath: sql.NullString{String: "Defaults/defaults.mkv", Valid: true},
	})
	require.NoError(t, err)
	require.NotZero(t, fileID)

	files, err := s.ListMovieFiles(ctx)
	require.NoError(t, err)
	require.Len(t, files, 1)

	assert.False(t, files[0].DateAdded.IsZero(),
		"movie_file.date_added default must be applied, not zero/NULL")
	files[0].DateAdded = time.Time{}
	assert.Equal(t, sqlcdb.MovieFile{
		ID:           int64(fileID),
		Quality:      "HDTV-720p",
		Size:         1_000_000_000,
		RelativePath: sql.NullString{String: "Defaults/defaults.mkv", Valid: true},
		IndexerFlags: 0,
	}, files[0])
}

// TestSqlcIntegration_MovieLatestTransitionUniqueness verifies that the
// partial unique indexes on movie_transition survive the sqlc migration: one
// most_recent row per movie, unique (movie, sort_key) history, and that the
// constraint is scoped per movie.
func TestSqlcIntegration_MovieLatestTransitionUniqueness(t *testing.T) {
	ctx := context.Background()
	s := sqlcIntegrationStore(t)

	movieID, err := s.CreateMovie(ctx, sqlcdb.Movie{Path: sql.NullString{String: "Unique/one.mkv", Valid: true}, Monitored: 1, QualityProfileID: 1}, storage.MovieStateMissing)
	require.NoError(t, err)
	require.NoError(t, s.UpdateMovieState(ctx, movieID, storage.MovieStateDownloading,
		&storage.TransitionStateMetadata{DownloadID: ptr.To("dl-1")}))

	secondID, err := s.CreateMovie(ctx, sqlcdb.Movie{Path: sql.NullString{String: "Unique/two.mkv", Valid: true}, Monitored: 1, QualityProfileID: 1}, storage.MovieStateMissing)
	require.NoError(t, err)

	// Normal history: exactly one most_recent row per movie.
	assert.Equal(t, int64(1), countMostRecent(t, s, "movie_transition", "movie_id", movieID))
	assert.Equal(t, int64(1), countMostRecent(t, s, "movie_transition", "movie_id", secondID))

	// A second most_recent row for the same movie must violate the partial
	// unique index idx_movie_transitions_by_parent_most_recent.
	_, err = s.db.Exec(
		`INSERT INTO movie_transition (movie_id, to_state, most_recent, sort_key) VALUES (?, 'downloaded', 1, 99)`, movieID)
	require.ErrorContains(t, err, "UNIQUE")

	// A repeated sort key for the same movie must violate
	// idx_movie_transitions_by_parent_sort_key.
	_, err = s.db.Exec(
		`INSERT INTO movie_transition (movie_id, to_state, most_recent, sort_key) VALUES (?, 'downloaded', 0, 2)`, movieID)
	require.ErrorContains(t, err, "UNIQUE")

	// The constraints are per parent: another existing movie can use the same sort key.
	_, err = s.db.Exec(
		`INSERT INTO movie_transition (movie_id, to_state, most_recent, sort_key) VALUES (?, 'missing', 0, 2)`, secondID)
	assert.NoError(t, err)

	// The API-level history stays consistent: sort keys increase 1..N and the
	// latest transition is the most_recent one.
	rows := movieTransitionRows(t, s, movieID)
	require.Len(t, rows, 2)
	for i, row := range rows {
		assert.Equal(t, int64(i+1), row.SortKey)
	}
	assert.True(t, rows[1].MostRecent)
	assert.Equal(t, "downloading", rows[1].ToState)

	movie, err := s.GetMovie(ctx, movieID)
	require.NoError(t, err)
	assert.Equal(t, string(storage.MovieStateDownloading), movie.State)
}

// TestSqlcIntegration_MovieTransitionInsertFailureRollsBackAtomically injects a
// failure into the insertion of a new movie transition and verifies the whole
// state transition rolls back: the previous transition stays most_recent, no
// partial rows are left behind, and the movie keeps its old state as seen
// through the API.
func TestSqlcIntegration_MovieTransitionInsertFailureRollsBackAtomically(t *testing.T) {
	ctx := context.Background()
	s := sqlcIntegrationStore(t)

	_, err := s.CreateDownloadClient(ctx, sqlcdb.DownloadClient{
		Type: "nzb", Implementation: "sabnzbd", Scheme: "http", Host: "localhost", Port: 8080,
	})
	require.NoError(t, err)

	path := "Rollback/rollback.mkv"
	movieID, err := s.CreateMovie(ctx, sqlcdb.Movie{Path: sql.NullString{String: path, Valid: true}, Monitored: 1, QualityProfileID: 1}, storage.MovieStateMissing)
	require.NoError(t, err)
	require.NoError(t, s.UpdateMovieState(ctx, movieID, storage.MovieStateDownloading,
		&storage.TransitionStateMetadata{DownloadClientID: ptr.To(int64(1)), DownloadID: ptr.To("dl-42")}))

	rowsBefore := movieTransitionRows(t, s, movieID)
	require.Len(t, rowsBefore, 2)

	installAbortTrigger(t, s, "sqlc_it_abort_movie_transition_insert", "movie_transition", "INSERT")
	t.Cleanup(func() { dropTrigger(t, s, "sqlc_it_abort_movie_transition_insert") })

	err = s.UpdateMovieState(ctx, movieID, storage.MovieStateDownloaded, nil)
	require.ErrorContains(t, err, "injected failure")

	// Not a single transition row may change: the previous transition must
	// still be most_recent and no new row may be inserted.
	assert.Equal(t, rowsBefore, movieTransitionRows(t, s, movieID))
	assert.Equal(t, int64(1), countMostRecent(t, s, "movie_transition", "movie_id", movieID))

	movie, err := s.GetMovie(ctx, movieID)
	require.NoError(t, err)
	assert.Equal(t, string(storage.MovieStateDownloading), movie.State)
	assert.Equal(t, "dl-42", movie.DownloadID.String)
	assert.Equal(t, int64(1), movie.DownloadClientID.Int64)

	// Once the failure is removed the same transition must succeed, proving
	// the failed attempt left no lasting corruption.
	dropTrigger(t, s, "sqlc_it_abort_movie_transition_insert")
	require.NoError(t, s.UpdateMovieState(ctx, movieID, storage.MovieStateDownloaded, nil))

	rows := movieTransitionRows(t, s, movieID)
	require.Len(t, rows, 3)
	assert.Equal(t, "downloaded", rows[2].ToState)
	assert.True(t, rows[2].MostRecent)
	assert.Equal(t, int64(3), rows[2].SortKey)
	assert.Equal(t, int64(1), countMostRecent(t, s, "movie_transition", "movie_id", movieID))
}

// TestSqlcIntegration_CreateMovieRollbackLeavesNoPartialRows injects a failure
// into the initial movie transition insert and verifies CreateMovie leaves no
// half-created movie row behind.
func TestSqlcIntegration_CreateMovieRollbackLeavesNoPartialRows(t *testing.T) {
	ctx := context.Background()
	s := sqlcIntegrationStore(t)

	path := "Partial/partial.mkv"
	installAbortTrigger(t, s, "sqlc_it_abort_movie_transition_insert", "movie_transition", "INSERT")
	t.Cleanup(func() { dropTrigger(t, s, "sqlc_it_abort_movie_transition_insert") })

	_, err := s.CreateMovie(ctx, sqlcdb.Movie{Path: sql.NullString{String: path, Valid: true}, Monitored: 1, QualityProfileID: 1}, storage.MovieStateMissing)
	require.ErrorContains(t, err, "injected failure")

	assert.Zero(t, countRows(t, s, "movie"), "no movie row may survive a failed create")
	assert.Zero(t, countRows(t, s, "movie_transition"), "no transition row may survive a failed create")

	movies, err := s.ListMovies(ctx)
	require.NoError(t, err)
	assert.Empty(t, movies)
}

// TestSqlcIntegration_SeriesLatestTransitionUniquenessAndRollback repeats the
// uniqueness and rollback checks for series_transition, which the sqlc
// migration rebuilds without download client columns.
func TestSqlcIntegration_SeriesLatestTransitionUniquenessAndRollback(t *testing.T) {
	ctx := context.Background()
	s := sqlcIntegrationStore(t)

	series := sqlcdb.Series{Monitored: 1, QualityProfileID: 1}
	seriesID, err := s.CreateSeries(ctx, series, storage.SeriesStateMissing)
	require.NoError(t, err)
	require.NoError(t, s.UpdateSeriesState(ctx, seriesID, storage.SeriesStateDownloading, nil))

	// Uniqueness: one most_recent row, no duplicate sort keys, per-series scoping.
	assert.Equal(t, int64(1), countMostRecent(t, s, "series_transition", "series_id", seriesID))

	_, err = s.db.Exec(
		`INSERT INTO series_transition (series_id, to_state, most_recent, sort_key) VALUES (?, 'completed', 1, 99)`, seriesID)
	require.ErrorContains(t, err, "UNIQUE")

	_, err = s.db.Exec(
		`INSERT INTO series_transition (series_id, to_state, most_recent, sort_key) VALUES (?, 'completed', 0, 2)`, seriesID)
	require.ErrorContains(t, err, "UNIQUE")

	rowsBefore := seriesTransitionRows(t, s, seriesID)
	require.Len(t, rowsBefore, 2)

	// Rollback: a failing transition insert must leave history untouched.
	installAbortTrigger(t, s, "sqlc_it_abort_series_transition_insert", "series_transition", "INSERT")
	t.Cleanup(func() { dropTrigger(t, s, "sqlc_it_abort_series_transition_insert") })

	err = s.UpdateSeriesState(ctx, seriesID, storage.SeriesStateContinuing, nil)
	require.ErrorContains(t, err, "injected failure")

	assert.Equal(t, rowsBefore, seriesTransitionRows(t, s, seriesID))
	assert.Equal(t, int64(1), countMostRecent(t, s, "series_transition", "series_id", seriesID))

	seriesList, err := s.ListSeries(ctx, storage.SeriesFilter{})
	require.NoError(t, err)
	require.Len(t, seriesList, 1)
	assert.Equal(t, string(storage.SeriesStateDownloading), seriesList[0].State)

	// Recovery: after removing the trigger the transition succeeds.
	dropTrigger(t, s, "sqlc_it_abort_series_transition_insert")
	require.NoError(t, s.UpdateSeriesState(ctx, seriesID, storage.SeriesStateCompleted, nil))

	rows := seriesTransitionRows(t, s, seriesID)
	require.Len(t, rows, 3)
	assert.Equal(t, "completed", rows[2].ToState)
	assert.True(t, rows[2].MostRecent)
	assert.Equal(t, int64(3), rows[2].SortKey)
}

// TestSqlcIntegration_IndexerSourceCascadeRollbackAtomicity verifies that
// DeleteIndexerSourceCascade removes a source and its child indexers
// atomically: an injected failure while deleting the source rolls back the
// already-deleted children, and a clean run deletes both.
func TestSqlcIntegration_IndexerSourceCascadeRollbackAtomicity(t *testing.T) {
	ctx := context.Background()
	s := sqlcIntegrationStore(t)

	sourceID, err := s.CreateIndexerSource(ctx, sqlcdb.IndexerSource{
		Name:           "prowlarr",
		Implementation: "prowlarr",
		Scheme:         "http",
		Host:           "prowlarr.local",
		Port:           sql.NullInt64{Int64: int64(9696), Valid: true},
		ApiKey:         sql.NullString{String: "source-key", Valid: true},
		Enabled:        true,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), sourceID)

	wantSource := sqlcdb.IndexerSource{
		ID:             int64(sourceID),
		Name:           "prowlarr",
		Implementation: "prowlarr",
		Scheme:         "http",
		Host:           "prowlarr.local",
		Port:           sql.NullInt64{Int64: int64(9696), Valid: true},
		ApiKey:         sql.NullString{String: "source-key", Valid: true},
		Enabled:        true,
	}

	_, err = s.CreateIndexer(ctx, sqlcdb.Indexer{
		Name:            "nzbgeek",
		Priority:        25,
		Uri:             "https://api.nzbgeek",
		ApiKey:          sql.NullString{String: "child-key-1", Valid: true},
		IndexerSourceID: sql.NullInt64{Int64: int64(sourceID), Valid: true},
	})
	require.NoError(t, err)

	_, err = s.CreateIndexer(ctx, sqlcdb.Indexer{
		Name:            "dzp",
		Priority:        50,
		Uri:             "https://api.dognzb.cr",
		ApiKey:          sql.NullString{String: "child-key-2", Valid: true},
		IndexerSourceID: sql.NullInt64{Int64: int64(sourceID), Valid: true},
	})
	require.NoError(t, err)

	wantChildren := []sqlcdb.Indexer{
		{
			ID:              1,
			Categories:      `{"movie":[],"tv":[]}`,
			Name:            "nzbgeek",
			Priority:        25,
			Uri:             "https://api.nzbgeek",
			ApiKey:          sql.NullString{String: "child-key-1", Valid: true},
			IndexerSourceID: sql.NullInt64{Int64: int64(sourceID), Valid: true},
		},
		{
			ID:              2,
			Categories:      `{"movie":[],"tv":[]}`,
			Name:            "dzp",
			Priority:        50,
			Uri:             "https://api.dognzb.cr",
			ApiKey:          sql.NullString{String: "child-key-2", Valid: true},
			IndexerSourceID: sql.NullInt64{Int64: int64(sourceID), Valid: true},
		},
	}

	// Failing to delete the source row must roll back the child deletes too.
	installAbortTrigger(t, s, "sqlc_it_abort_indexer_source_delete", "indexer_source", "DELETE")
	t.Cleanup(func() { dropTrigger(t, s, "sqlc_it_abort_indexer_source_delete") })

	err = s.DeleteIndexerSourceCascade(ctx, sourceID)
	require.ErrorContains(t, err, "injected failure")

	gotSource, err := s.GetIndexerSource(ctx, sourceID)
	require.NoError(t, err)
	// Timestamps are database-managed and intentionally not part of this
	// assertion; they are covered by the defaults tests above.
	gotSource.CreatedAt = sql.NullTime{}
	gotSource.UpdatedAt = sql.NullTime{}
	assert.Equal(t, wantSource, gotSource)

	children, err := s.ListIndexers(ctx)
	require.NoError(t, err)
	require.Len(t, children, 2)
	assert.ElementsMatch(t, wantChildren, children)

	// A clean cascade deletes the source and every child indexer.
	dropTrigger(t, s, "sqlc_it_abort_indexer_source_delete")
	require.NoError(t, s.DeleteIndexerSourceCascade(ctx, sourceID))

	sources, err := s.ListIndexerSources(ctx)
	require.NoError(t, err)
	assert.Empty(t, sources)

	children, err = s.ListIndexers(ctx)
	require.NoError(t, err)
	assert.Empty(t, children)
}
