package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// expectedTables is the full set of tables produced by the squashed baseline
// migration, in sqlite_master order.
var expectedTables = []string{
	"download_client",
	"episode",
	"episode_file",
	"episode_metadata",
	"episode_transition",
	"indexer",
	"indexer_source",
	"job",
	"job_transition",
	"movie",
	"movie_file",
	"movie_metadata",
	"movie_transition",
	"quality_definition",
	"quality_profile",
	"quality_profile_item",
	"schema_migrations",
	"season",
	"season_metadata",
	"season_transition",
	"series",
	"series_metadata",
	"series_transition",
}

// TestBootstrap_FreshFileDatabase verifies that New on a fresh file database
// migrates to the baseline and applies the default seed.
func TestBootstrap_FreshFileDatabase(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "mediaz.db")

	store, err := New(ctx, dbPath)
	require.NoError(t, err)
	s := store.(*SQLite)
	t.Cleanup(func() { require.NoError(t, s.db.Close()) })

	assert.Equal(t, expectedTables, listTables(t, s.db))

	version, dirty, err := s.GetMigrationVersion()
	require.NoError(t, err)
	assert.Equal(t, uint(1), version)
	assert.False(t, dirty)

	assertSeedData(t, s.db)

	// _foreign_keys=on is a DSN parameter applied per pooled connection, so it
	// must hold on every connection, not just the one that ran migrations.
	assertForeignKeysOnAllConnections(t, ctx, s.db)
}

// TestBootstrap_DoubleNewIsIdempotent verifies that opening the same file
// database twice neither re-runs migrations nor duplicates seed rows.
func TestBootstrap_DoubleNewIsIdempotent(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "mediaz.db")

	store, err := New(ctx, dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.(*SQLite).db.Close()) })
	before := seedSnapshot(t, store.(*SQLite).db)

	store2, err := New(ctx, dbPath)
	require.NoError(t, err)
	s2 := store2.(*SQLite)
	t.Cleanup(func() { require.NoError(t, s2.db.Close()) })

	assert.Equal(t, expectedTables, listTables(t, s2.db))
	assert.Equal(t, before, seedSnapshot(t, s2.db), "seed rows must be unchanged by a second New")

	version, dirty, err := s2.GetMigrationVersion()
	require.NoError(t, err)
	assert.Equal(t, uint(1), version)
	assert.False(t, dirty)
}

func TestBootstrap_ConcurrentNewIsIdempotent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	dbPath := filepath.Join(t.TempDir(), "mediaz.db")
	// Start from an applied but unseeded baseline to isolate seed races from
	// golang-migrate's independent bootstrap/version-table initialization.
	db, err := sql.Open("sqlite3", dbPath+dsnParams)
	require.NoError(t, err)
	_, err = runMigrations(db, dbPath)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	type result struct {
		s   *SQLite
		err error
	}
	results := make(chan result, 2)
	for i := 0; i < 2; i++ {
		go func() {
			store, err := New(ctx, dbPath)
			if err != nil {
				results <- result{err: err}
				return
			}
			results <- result{s: store.(*SQLite)}
		}()
	}
	for i := 0; i < 2; i++ {
		r := <-results
		require.NoError(t, r.err)
		t.Cleanup(func() { require.NoError(t, r.s.db.Close()) })
		assertSeedData(t, r.s.db)
	}
}

// TestBootstrap_InMemoryDatabase verifies bootstrap on :memory:, including
// that foreign keys are enforced on the single pooled connection.
func TestBootstrap_InMemoryDatabase(t *testing.T) {
	ctx := context.Background()

	store, err := New(ctx, inMemoryPath)
	require.NoError(t, err)
	s := store.(*SQLite)
	t.Cleanup(func() { require.NoError(t, s.db.Close()) })

	assert.Equal(t, expectedTables, listTables(t, s.db))
	assertSeedData(t, s.db)

	var enabled int
	require.NoError(t, s.db.QueryRow(`PRAGMA foreign_keys`).Scan(&enabled))
	assert.Equal(t, 1, enabled, "foreign_keys must be enforced on the in-memory database connection")
}

// TestBootstrap_InMemoryPool proves a second caller waits for the same
// connection rather than receiving a distinct empty in-memory database.
func TestBootstrap_InMemoryPool(t *testing.T) {
	ctx := context.Background()
	store, err := New(ctx, inMemoryPath)
	require.NoError(t, err)
	db := store.(*SQLite).db
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	require.Equal(t, 1, db.Stats().MaxOpenConnections)
	conn, err := db.Conn(ctx)
	require.NoError(t, err)
	defer conn.Close()
	waitCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	_, err = db.Conn(waitCtx)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.NoError(t, conn.Close())
	assert.Equal(t, expectedTables, listTables(t, db))
	assertSeedData(t, db)
}

// Migration can succeed before a failed seed; the next startup must retry.
func TestBootstrap_SeedAfterAlreadyAppliedBaseline(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "mediaz.db")
	db, err := sql.Open("sqlite3", dbPath+dsnParams)
	require.NoError(t, err)
	_, err = runMigrations(db, dbPath)
	require.NoError(t, err)
	require.NoError(t, db.Close())
	store, err := New(ctx, dbPath)
	require.NoError(t, err)
	s := store.(*SQLite)
	t.Cleanup(func() { require.NoError(t, s.db.Close()) })
	assertSeedData(t, s.db)
	before := seedSnapshot(t, s.db)
	require.NoError(t, s.seed(ctx))
	assert.Equal(t, before, seedSnapshot(t, s.db))
}

// TestBootstrap_ForeignKeysEnforced proves foreign keys are actually
// enforced: a transition row referencing a nonexistent movie must be
// rejected. (The successful seed is the positive control — it inserts
// quality_profile_item rows referencing quality_profile and quality_definition
// with foreign keys on.)
func TestBootstrap_ForeignKeysEnforced(t *testing.T) {
	ctx := context.Background()

	store, err := New(ctx, inMemoryPath)
	require.NoError(t, err)
	db := store.(*SQLite).db
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	_, err = db.Exec(`INSERT INTO movie_transition (movie_id, to_state, most_recent, sort_key)
		VALUES (999999, 'discovered', 1, 1)`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "FOREIGN KEY constraint failed")
}

// TestBootstrap_OldVersionedDatabaseRequiresRecreate simulates a database
// created by an earlier release (old migration series ended at version 7,
// now squashed away) and verifies that New fails loudly with the
// recreate-database guidance — without modifying or deleting the file.
func TestBootstrap_OldVersionedDatabaseRequiresRecreate(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "mediaz.db")

	store, err := New(ctx, dbPath)
	require.NoError(t, err)

	// Force the stored version to 7, as an old-release database would carry.
	_, err = store.(*SQLite).db.Exec(`UPDATE schema_migrations SET version = 7`)
	require.NoError(t, err)

	// Flush WAL and close the successful bootstrap before comparing file bytes.
	require.NoError(t, store.(*SQLite).db.Close())
	before, err := os.ReadFile(dbPath)
	require.NoError(t, err)

	store2, err := New(ctx, dbPath)
	require.Error(t, err)
	assert.Nil(t, store2)
	assert.Contains(t, err.Error(), "recreating the mediaz database")
	assert.Contains(t, err.Error(), "media files are NOT affected")
	assert.Contains(t, err.Error(), "version 7")
	assert.Contains(t, err.Error(), dbPath, "the error must include the database path")

	after, err := os.ReadFile(dbPath)
	require.NoError(t, err)
	assert.Equal(t, sha256.Sum256(before), sha256.Sum256(after),
		"a failed New must not modify the database file")

	_, err = os.Stat(dbPath)
	require.NoError(t, err, "mediaz must never delete the database file itself")
}

// TestBootstrap_PreMigrationDatabaseRequiresRecreate covers databases from
// before the migration system: mediaz tables exist but schema_migrations does
// not. Applying the baseline over them would silently keep old table shapes,
// so New must fail loudly instead.
func TestBootstrap_PreMigrationDatabaseRequiresRecreate(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "mediaz.db")

	db, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE quality_profile (id INTEGER PRIMARY KEY)`)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	store, err := New(ctx, dbPath)
	require.Error(t, err)
	assert.Nil(t, store)
	assert.Contains(t, err.Error(), "recreating the mediaz database")
	assert.Contains(t, err.Error(), dbPath, "the error must include the database path")

	_, err = os.Stat(dbPath)
	require.NoError(t, err, "mediaz must never delete the database file itself")
}

// listTables returns all non-internal table names in sqlite_master order.
func listTables(t *testing.T, db *sql.DB) []string {
	t.Helper()
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	require.NoError(t, err)
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		names = append(names, name)
	}
	require.NoError(t, rows.Err())
	return names
}

// assertSeedData checks the seeded defaults: 28 quality definitions (14 movie
// + 14 episode), 6 quality profiles with the expected cutoffs, and their
// items.
func assertSeedData(t *testing.T, db *sql.DB) {
	t.Helper()

	var definitions, profiles, items int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM quality_definition`).Scan(&definitions))
	assert.Equal(t, 28, definitions)
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM quality_profile`).Scan(&profiles))
	assert.Equal(t, 6, profiles)
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM quality_profile_item`).Scan(&items))
	assert.Equal(t, 27, items)

	var movieDefinitions, episodeDefinitions int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM quality_definition WHERE media_type = 'movie'`).Scan(&movieDefinitions))
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM quality_definition WHERE media_type = 'episode'`).Scan(&episodeDefinitions))
	assert.Equal(t, 14, movieDefinitions)
	assert.Equal(t, 14, episodeDefinitions)

	type profileRow struct {
		id      int64
		name    string
		cutoff  sql.NullInt64
		upgrade bool
	}
	rows, err := db.Query(`SELECT id, name, cutoff_quality_id, upgrade_allowed FROM quality_profile ORDER BY id`)
	require.NoError(t, err)
	defer rows.Close()

	var got []profileRow
	for rows.Next() {
		var p profileRow
		require.NoError(t, rows.Scan(&p.id, &p.name, &p.cutoff, &p.upgrade))
		got = append(got, p)
	}
	require.NoError(t, rows.Err())
	assert.Equal(t, []profileRow{
		{1, "Standard Definition", sql.NullInt64{Int64: 2, Valid: true}, true},
		{2, "High Definition", sql.NullInt64{Int64: 8, Valid: true}, true},
		{3, "Ultra High Definition", sql.NullInt64{Int64: 13, Valid: true}, false},
		{4, "Standard Definition", sql.NullInt64{Int64: 16, Valid: true}, true},
		{5, "High Definition", sql.NullInt64{Int64: 23, Valid: true}, true},
		{6, "Ultra High Definition", sql.NullInt64{Int64: 27, Valid: true}, false},
	}, got)

	itemCounts := map[int64]int{1: 2, 2: 6, 3: 5, 4: 2, 5: 7, 6: 5}
	rows2, err := db.Query(`SELECT profile_id, COUNT(*) FROM quality_profile_item GROUP BY profile_id ORDER BY profile_id`)
	require.NoError(t, err)
	defer rows2.Close()

	seen := map[int64]int{}
	for rows2.Next() {
		var profileID, count int64
		require.NoError(t, rows2.Scan(&profileID, &count))
		seen[profileID] = int(count)
	}
	require.NoError(t, rows2.Err())
	assert.Equal(t, itemCounts, seen)
}

// seedSnapshot renders every seeded row into a deterministic string so that
// before/after comparisons catch any duplication or mutation.
func seedSnapshot(t *testing.T, db *sql.DB) string {
	t.Helper()

	var sb strings.Builder
	queries := []string{
		`SELECT id, quality_id, name, preferred_size, min_size, max_size, media_type FROM quality_definition ORDER BY id`,
		`SELECT id, name, COALESCE(cutoff_quality_id, -1), upgrade_allowed FROM quality_profile ORDER BY id`,
		`SELECT id, profile_id, quality_id FROM quality_profile_item ORDER BY id`,
	}
	for _, query := range queries {
		rows, err := db.Query(query)
		require.NoError(t, err)
		columns, err := rows.Columns()
		require.NoError(t, err)

		for rows.Next() {
			values := make([]any, len(columns))
			targets := make([]any, len(columns))
			for i := range values {
				targets[i] = &values[i]
			}
			require.NoError(t, rows.Scan(targets...))
			fmt.Fprintln(&sb, values...)
		}
		require.NoError(t, rows.Err())
		require.NoError(t, rows.Close())
	}
	return sb.String()
}

// assertForeignKeysOnAllConnections checks out two distinct pooled
// connections and verifies PRAGMA foreign_keys is enabled on each.
func assertForeignKeysOnAllConnections(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()

	conns := make([]*sql.Conn, 0, 2)
	defer func() {
		for _, conn := range conns {
			require.NoError(t, conn.Close())
		}
	}()

	for i := 0; i < 2; i++ {
		conn, err := db.Conn(ctx)
		require.NoError(t, err)
		conns = append(conns, conn)

		var enabled int
		require.NoError(t, conn.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&enabled))
		assert.Equal(t, 1, enabled, "foreign_keys must be enforced on every pooled connection (connection %d)", i+1)
	}
}
