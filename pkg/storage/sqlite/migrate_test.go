package sqlite

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMigrations_BaselineOnly verifies the embedded migration source contains
// exactly the squashed baseline (version 1).
func TestMigrations_BaselineOnly(t *testing.T) {
	version, err := maxMigrationSourceVersion()
	require.NoError(t, err)
	assert.Equal(t, uint(1), version)
}

// TestMigrations_RunnerIsIdempotent verifies the forward-only runner: the
// first run applies the baseline, subsequent runs are no-ops.
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
	assert.Equal(t, uint64(1), version)

	migrated, err = runMigrations(db, ":memory:")
	require.NoError(t, err)
	assert.False(t, migrated, "a second run is a no-op")

	version, exists, err = storedMigrationVersion(db)
	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, uint64(1), version)
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
	assert.Equal(t, uint64(1), version)
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
