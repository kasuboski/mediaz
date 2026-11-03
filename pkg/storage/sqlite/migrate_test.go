package sqlite

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMigrations_BaselineOnly verifies the embedded migration source includes
// the baseline through version 6 (durable TV episode admission).
func TestMigrations_BaselineOnly(t *testing.T) {
	version, err := maxMigrationSourceVersion()
	require.NoError(t, err)
	assert.Equal(t, uint(6), version)
}

// TestMigrations_RunnerIsIdempotent verifies the forward-only runner: the
// first run applies all migrations, subsequent runs are no-ops.
func TestMigrations_RunnerIsIdempotent(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	version, exists, err := storedMigrationVersion(db)
	require.NoError(t, err)
	assert.False(t, exists, "a fresh database has no schema_migrations table")
	assert.Equal(t, uint64(0), version)

	migrated, err := runMigrations(db, ":memory:")
	require.NoError(t, err)
	assert.True(t, migrated, "the first run applies the baseline")

	version, exists, err = storedMigrationVersion(db)
	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, uint64(6), version)

	migrated, err = runMigrations(db, ":memory:")
	require.NoError(t, err)
	assert.False(t, migrated, "a second run is a no-op")

	version, exists, err = storedMigrationVersion(db)
	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, uint64(6), version)
}

func TestMigrations_V3PreservesIndexerSource(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	defer db.Close()

	// Apply the real baseline, then construct the state immediately before migration 4.
	_, err = runMigrations(db, ":memory:")
	require.NoError(t, err)
	_, err = db.Exec(`ALTER TABLE indexer_source DROP COLUMN api_path`)
	require.NoError(t, err)
	_, err = db.Exec(`DROP TABLE tv_episode_admission`)
	require.NoError(t, err)
	// Undo migration 5 too so the resumed run can re-apply it cleanly.
	_, err = db.Exec(`ALTER TABLE season DROP COLUMN quality_profile_id`)
	require.NoError(t, err)
	_, err = db.Exec(`ALTER TABLE episode DROP COLUMN quality_profile_id`)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE schema_migrations SET version = 3`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO indexer_source (id, name, implementation, scheme, host, port, api_key, enabled)
		VALUES (42, 'Prowlarr', 'prowlarr', 'https', 'prowlarr.example', 9696, 'secret-key', 1)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO indexer (name, priority, uri, api_key, indexer_source_id)
		VALUES ('Existing indexer', 7, 'http://prowlarr.example/indexer/1', 'indexer-key', 42)`)
	require.NoError(t, err)

	migrated, err := runMigrations(db, ":memory:")
	require.NoError(t, err)
	assert.True(t, migrated)
	var id, port int64
	var name, implementation, scheme, host, apiKey string
	var enabled bool
	var apiPath sql.NullString
	require.NoError(t, db.QueryRow(`SELECT id, name, implementation, scheme, host, port, api_key, enabled, api_path FROM indexer_source WHERE id = 42`).Scan(&id, &name, &implementation, &scheme, &host, &port, &apiKey, &enabled, &apiPath))
	assert.Equal(t, int64(42), id)
	assert.Equal(t, "Prowlarr", name)
	assert.Equal(t, "prowlarr", implementation)
	assert.Equal(t, "https", scheme)
	assert.Equal(t, "prowlarr.example", host)
	assert.Equal(t, int64(9696), port)
	assert.Equal(t, "secret-key", apiKey)
	assert.True(t, enabled)
	assert.False(t, apiPath.Valid, "existing Prowlarr rows retain a null API path and use their built-in default")
	var indexerCount int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM indexer WHERE name = 'Existing indexer' AND indexer_source_id = 42`).Scan(&indexerCount))
	assert.Equal(t, 1, indexerCount)

	migrated, err = runMigrations(db, ":memory:")
	require.NoError(t, err)
	assert.False(t, migrated, "reapplying migration runner is a no-op")
	version, exists, err := storedMigrationVersion(db)
	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, uint64(6), version)
}

func TestMigrations_EmptyVersionTableResumes(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE schema_migrations (version uint64, dirty bool)`)
	require.NoError(t, err)
	migrated, err := runMigrations(db, ":memory:")
	require.NoError(t, err)
	assert.True(t, migrated)
	version, exists, err := storedMigrationVersion(db)
	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, uint64(6), version)
}

// TestMigrations_RecreateErrorMessage pins the operator-facing error for
// databases whose schema_migrations version exceeds the available baseline.
func TestMigrations_RecreateErrorMessage(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	_, err = runMigrations(db, "/data/mediaz.db")
	require.NoError(t, err)

	// Simulate a database created by the old seven-migration series.
	_, err = db.Exec(`UPDATE schema_migrations SET version = 7`)
	require.NoError(t, err)

	migrated, err := runMigrations(db, "/data/mediaz.db")
	require.Error(t, err)
	assert.False(t, migrated)
	assert.Contains(t, err.Error(), "/data/mediaz.db")
	assert.Contains(t, err.Error(), "recreating the mediaz database")
	assert.Contains(t, err.Error(), "delete the database file and restart")
	assert.Contains(t, err.Error(), "media files are NOT affected")
	assert.Contains(t, err.Error(), "version 7")
}
