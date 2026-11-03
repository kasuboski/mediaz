package sqlite

import (
	"database/sql"
	"strconv"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	migratesqlite3 "github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMigrations_V5DownPreservesTVTree verifies that the migration 000005 down
// script removes the nullable quality_profile_id override columns from season
// and episode without losing any TV data, even with PRAGMA foreign_keys=ON and
// a fully populated series/season/episode tree with files and transitions.
//
// The down script must run through the actual migration mechanism (the
// embedded up/down SQL files, one transaction per migration) — not a duplicated
// copy of the SQL. The rebuilt schema must preserve the original table shape:
// all columns, constraints, indexes, and every referencing table's rows.
func TestMigrations_V5DownPreservesTVTree(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:?_foreign_keys=on&_busy_timeout=5000&_txlock=immediate")
	require.NoError(t, err)
	defer db.Close()
	db.SetMaxOpenConns(1)

	// Up to version 5 (adds the override columns), through the real runner.
	migrated, err := runMigrations(db, ":memory:")
	require.NoError(t, err)
	require.True(t, migrated)
	// This regression deliberately exercises V5 even after later migrations.
	m, err := newTestMigrate(db)
	require.NoError(t, err)
	require.NoError(t, m.Migrate(5))
	version, _, err := storedMigrationVersion(db)
	require.NoError(t, err)
	require.Equal(t, uint64(5), version)

	// Populate a realistic TV tree: series -> seasons -> episodes, with
	// metadata, a downloaded file, monitoring state, and state transitions.
	// One season and one episode carry quality overrides; the others inherit.
	// (The raw runner does not seed, so create the profile explicitly.)
	_, err = db.Exec(`INSERT INTO quality_profile (id, name, cutoff_quality_id, upgrade_allowed)
		VALUES (1, 'HD-1080p', NULL, 1)`)
	require.NoError(t, err)
	var profileID int64
	err = db.QueryRow(`SELECT id FROM quality_profile ORDER BY id LIMIT 1`).Scan(&profileID)
	require.NoError(t, err, "the seeded database must contain a quality profile")

	seed := []string{
		`INSERT INTO series_metadata (id, tmdb_id, title, season_count, episode_count, status)
		 VALUES (1, 1399, 'Game of Thrones', 2, 2, 'ended')`,
		`INSERT INTO series (id, path, monitored, quality_profile_id, series_metadata_id, monitor_new_seasons)
		 VALUES (1, '/tv/got', 1, ` + itoa(profileID) + `, 1, 0)`,
		`INSERT INTO season_metadata (id, series_metadata_id, number, tmdb_id, title)
		 VALUES (1, 1, 0, 3624, 'Specials'), (2, 1, 1, 3625, 'Season 1')`,
		`INSERT INTO season (id, series_id, season_number, season_metadata_id, monitored, quality_profile_id)
		 VALUES (1, 1, 0, 1, 0, ` + itoa(profileID) + `), (2, 1, 1, 2, 1, NULL)`,
		`INSERT INTO episode_metadata (id, season_metadata_id, number, tmdb_id, title)
		 VALUES (1, 1, 1, 63056, 'Winter Is Coming'), (2, 2, 1, 63057, 'Pilot')`,
		`INSERT INTO episode_file (id, quality, size, relative_path, original_file_path)
		 VALUES (1, 'HD-1080p', 942200000, '/tv/got/S01E01.mkv', '/downloads/S01E01.mkv')`,
		`INSERT INTO episode (id, season_id, episode_number, monitored, episode_metadata_id, episode_file_id, quality_profile_id)
		 VALUES (1, 1, 1, 1, 1, 1, ` + itoa(profileID) + `), (2, 2, 1, 0, 2, NULL, NULL)`,
		`INSERT INTO season_transition (id, season_id, to_state, from_state, most_recent, sort_key, download_id, is_entire_season_download)
		 VALUES (1, 1, 'downloaded', 'monitored', 1, 1, 'dl-1', 1)`,
		`INSERT INTO episode_transition (id, episode_id, to_state, from_state, most_recent, sort_key, download_id, is_entire_season_download)
		 VALUES (1, 1, 'downloaded', 'monitored', 1, 1, 'dl-1', 0)`,
	}
	for _, stmt := range seed {
		_, err := db.Exec(stmt)
		require.NoError(t, err, "seed statement failed: %s", stmt)
	}

	// Down one step (5 -> 4) through the real migration mechanism.
	require.NoError(t, migrateDownOneStep(db))
	version, _, err = storedMigrationVersion(db)
	require.NoError(t, err)
	require.Equal(t, uint64(4), version)

	// The override columns must be gone.
	assert.NotContains(t, tableColumns(t, db, "season"), "quality_profile_id")
	assert.NotContains(t, tableColumns(t, db, "episode"), "quality_profile_id")

	// The rebuilt tables must keep their original foreign keys. The season
	// table's own FKs (series_id -> series, season_metadata_id ->
	// season_metadata) must survive, and episode must still reference season,
	// episode_metadata, and episode_file.
	assertForeignKeys(t, db, "season", map[string]string{
		"series_id":          "series",
		"season_metadata_id": "season_metadata",
	})
	assertForeignKeys(t, db, "episode", map[string]string{
		"season_id":           "season",
		"episode_metadata_id": "episode_metadata",
		"episode_file_id":     "episode_file",
	})

	// The unique season index from the baseline must still exist.
	assertIndexExists(t, db, "idx_season_unique_series_number")
	assertIndexExists(t, db, "idx_season_transitions_by_parent_sort_key")
	assertIndexExists(t, db, "idx_episode_transitions_by_parent_sort_key")

	// Every row of the tree must have survived the rebuild — full row content,
	// not just counts.
	assertSeasonRows(t, db, [][]any{
		{int64(1), int64(1), int64(0), int64(1), int64(0)},
		{int64(2), int64(1), int64(1), int64(2), int64(1)},
	})
	assertEpisodeRows(t, db, [][]any{
		{int64(1), int64(1), int64(1), int64(1), int64(1), int64(1)},
		{int64(2), int64(2), int64(1), int64(0), int64(2), sql.NullInt64{}},
	})
	assertQueryRows(t, db, "SELECT id, quality, size, relative_path, original_file_path FROM episode_file", [][]any{
		{int64(1), "HD-1080p", int64(942200000), sql.NullString{String: "/tv/got/S01E01.mkv", Valid: true}, sql.NullString{String: "/downloads/S01E01.mkv", Valid: true}},
	})
	assertQueryRows(t, db, "SELECT id, season_id, to_state, from_state, most_recent, sort_key, download_id, is_entire_season_download FROM season_transition", [][]any{
		{int64(1), int64(1), "downloaded", sql.NullString{String: "monitored", Valid: true}, bool(true), int64(1), sql.NullString{String: "dl-1", Valid: true}, bool(true)},
	})
	assertQueryRows(t, db, "SELECT id, episode_id, to_state, from_state, most_recent, sort_key, download_id, is_entire_season_download FROM episode_transition", [][]any{
		{int64(1), int64(1), "downloaded", sql.NullString{String: "monitored", Valid: true}, bool(true), int64(1), sql.NullString{String: "dl-1", Valid: true}, bool(false)},
	})
	assertIntegrityClean(t, db, "after down")

	// Re-up must succeed and restore the nullable override columns with NULL
	// values (the data predates the feature), keeping every row intact.
	m, err = newTestMigrate(db)
	require.NoError(t, err)
	require.NoError(t, m.Migrate(5))
	version, _, err = storedMigrationVersion(db)
	require.NoError(t, err)
	require.Equal(t, uint64(5), version)

	assert.Contains(t, tableColumns(t, db, "season"), "quality_profile_id")
	assert.Contains(t, tableColumns(t, db, "episode"), "quality_profile_id")

	assertIntegrityClean(t, db, "after re-up")
	assertSeasonRows(t, db, [][]any{
		{int64(1), int64(1), int64(0), int64(1), int64(0)},
		{int64(2), int64(1), int64(1), int64(2), int64(1)},
	})
	assertEpisodeRows(t, db, [][]any{
		{int64(1), int64(1), int64(1), int64(1), int64(1), int64(1)},
		{int64(2), int64(2), int64(1), int64(0), int64(2), sql.NullInt64{}},
	})
	// Overrides reappear as NULL inheritance after a data round-trip.
	assertQueryRows(t, db, "SELECT id, quality_profile_id FROM season ORDER BY id", [][]any{
		{int64(1), sql.NullInt64{}}, {int64(2), sql.NullInt64{}},
	})
	assertQueryRows(t, db, "SELECT id, quality_profile_id FROM episode ORDER BY id", [][]any{
		{int64(1), sql.NullInt64{}}, {int64(2), sql.NullInt64{}},
	})

	// Bring later migrations back too, then verify runner idempotence.
	require.NoError(t, migrateUpAll(db))
	// The migration runner must report the database is current afterward.
	migrated, err = runMigrations(db, ":memory:")
	require.NoError(t, err)
	assert.False(t, migrated, "re-running migrations after down/up must be a no-op")
}

// migrateDownOneStep runs the actual embedded migration scripts to step the
// database down exactly one version, mirroring how runMigrations builds its
// migrate instance.
func migrateDownOneStep(db *sql.DB) error {
	m, err := newTestMigrate(db)
	if err != nil {
		return err
	}
	return m.Steps(-1)
}

// migrateUpAll runs the actual embedded migration scripts to bring the
// database back up to the latest version.
func migrateUpAll(db *sql.DB) error {
	m, err := newTestMigrate(db)
	if err != nil {
		return err
	}
	return m.Up()
}

func newTestMigrate(db *sql.DB) (*migrate.Migrate, error) {
	source, err := iofs.New(migrationFiles, "migrations")
	if err != nil {
		return nil, err
	}
	driver, err := migratesqlite3.WithInstance(db, &migratesqlite3.Config{
		MigrationsTable: "schema_migrations",
		NoTxWrap:        false,
	})
	if err != nil {
		return nil, err
	}
	return migrate.NewWithInstance("iofs", source, "sqlite3", driver)
}

// tableColumns returns the column names of a table via PRAGMA table_info.
func tableColumns(t *testing.T, db *sql.DB, table string) []string {
	t.Helper()
	rows, err := db.Query("PRAGMA table_info(" + table + ")")
	require.NoError(t, err)
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull, pk int
		var dflt sql.NullString
		require.NoError(t, rows.Scan(&cid, &name, &typ, &notNull, &dflt, &pk))
		cols = append(cols, name)
	}
	require.NoError(t, rows.Err())
	return cols
}

// assertForeignKeys asserts the table's foreign keys exactly: every listed
// column references the listed table and there are no other foreign keys.
func assertForeignKeys(t *testing.T, db *sql.DB, table string, want map[string]string) {
	t.Helper()
	rows, err := db.Query("PRAGMA foreign_key_list(" + table + ")")
	require.NoError(t, err)
	got := map[string]string{}
	for rows.Next() {
		var id, seq int
		var refTable, from, to string
		var onUpdate, onDelete, match string
		require.NoError(t, rows.Scan(&id, &seq, &refTable, &from, &to, &onUpdate, &onDelete, &match))
		got[from] = refTable
	}
	require.NoError(t, rows.Err())
	require.NoError(t, rows.Close())
	assert.Equal(t, want, got, "%s foreign keys changed by the migration", table)
}

// assertIndexExists asserts an index is still attached to its table.
func assertIndexExists(t *testing.T, db *sql.DB, name string) {
	t.Helper()
	var tbl string
	err := db.QueryRow(`SELECT tbl_name FROM sqlite_master WHERE type='index' AND name=?`, name).Scan(&tbl)
	require.NoError(t, err, "index %s must survive the migration", name)
}

// assertIntegrityClean asserts PRAGMA foreign_key_check and integrity_check
// both report no problems.
func assertIntegrityClean(t *testing.T, db *sql.DB, when string) {
	t.Helper()
	var violations int
	rows, err := db.Query("PRAGMA foreign_key_check")
	require.NoError(t, err)
	for rows.Next() {
		violations++
	}
	require.NoError(t, rows.Err())
	require.NoError(t, rows.Close())
	assert.Zero(t, violations, "foreign_key_check must be clean %s", when)

	var result string
	require.NoError(t, db.QueryRow("PRAGMA integrity_check").Scan(&result))
	assert.Equal(t, "ok", result, "integrity_check must be clean %s", when)
}

// assertSeasonRows asserts the full content of the season table (in id order).
func assertSeasonRows(t *testing.T, db *sql.DB, want [][]any) {
	t.Helper()
	assertQueryRows(t, db,
		"SELECT id, series_id, season_number, season_metadata_id, monitored FROM season ORDER BY id", want)
}

// assertEpisodeRows asserts the full content of the episode table (in id order).
func assertEpisodeRows(t *testing.T, db *sql.DB, want [][]any) {
	t.Helper()
	assertQueryRows(t, db,
		"SELECT id, season_id, episode_number, monitored, episode_metadata_id, episode_file_id FROM episode ORDER BY id", want)
}

// assertQueryRows asserts an exact ordered row set for a query. Values compare
// as database/sql types (int64, string, sql.NullInt64).
func assertQueryRows(t *testing.T, db *sql.DB, query string, want [][]any) {
	t.Helper()
	rows, err := db.Query(query)
	require.NoError(t, err)
	defer rows.Close()
	var got [][]any
	for rows.Next() {
		dest := make([]any, len(want[0]))
		ptrs := make([]any, len(want[0]))
		for i := range dest {
			ptrs[i] = &dest[i]
		}
		require.NoError(t, rows.Scan(ptrs...))
		got = append(got, dest)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, len(want), len(got), "row count mismatch for query: %s", query)
	for i := range want {
		for j := range want[i] {
			assert.Equal(t, normalizeNull(want[i][j]), normalizeNull(got[i][j]), "row %d column %d of query: %s", i, j, query)
		}
	}
}

// normalizeNull maps every SQL NULL representation (driver nil, typed Null*
// structs with Valid=false) to a single comparable value.
func normalizeNull(v any) any {
	switch n := v.(type) {
	case nil:
		return "<null>"
	case sql.NullString:
		if !n.Valid {
			return "<null>"
		}
		return n.String
	case sql.NullInt64:
		if !n.Valid {
			return "<null>"
		}
		return n.Int64
	case sql.NullFloat64:
		if !n.Valid {
			return "<null>"
		}
		return n.Float64
	case sql.NullBool:
		if !n.Valid {
			return "<null>"
		}
		return n.Bool
	case sql.NullTime:
		if !n.Valid {
			return "<null>"
		}
		return n.Time
	default:
		return v
	}
}

// itoa formats an int64 for SQL interpolation of internal test constants.
func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}
