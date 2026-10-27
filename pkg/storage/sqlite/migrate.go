package sqlite

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"os"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// databaseRequiresRecreateHint is the operator-facing guidance appended to
// every error caused by an unsupported pre-existing database. Mediaz never
// deletes or rewrites such files; the operator recreates the database
// deliberately.
const databaseRequiresRecreateHint = "this release requires recreating the mediaz database: delete the database file and restart mediaz (media files are NOT affected)"

// runMigrations applies pending database migrations to db. dbPath is only used
// to build operator-facing error messages.
//
// Migrations are forward-only against the squashed baseline
// (000001_baseline). Databases created by earlier releases carry a
// schema_migrations version from the old seven-migration series (e.g. 7) that
// no longer exists, and databases from before the migration system have no
// version table at all. Both fail loudly with the recreate hint instead of
// being silently baselined or upgraded over. No file is ever deleted
// automatically.
func runMigrations(db *sql.DB, dbPath string) (migrated bool, err error) {
	maxVersion, err := maxMigrationSourceVersion()
	if err != nil {
		return false, fmt.Errorf("failed to inspect migration source: %w", err)
	}

	current, exists, err := storedMigrationVersion(db)
	if err != nil {
		return false, fmt.Errorf("failed to check migration version: %w", err)
	}

	if exists && current > uint64(maxVersion) {
		return false, fmt.Errorf(
			"database %q is at migration version %d, but this release only provides migrations up to version %d: %s",
			dbPath, current, maxVersion, databaseRequiresRecreateHint)
	}

	if !exists {
		nonempty, err := hasUnversionedTables(db)
		if err != nil {
			return false, fmt.Errorf("failed to inspect database: %w", err)
		}
		if nonempty {
			// A fresh baseline must start with an empty database, not attempt
			// compatibility inference from individual application table shapes.
			return false, fmt.Errorf("database %q predates the migration system: %s", dbPath, databaseRequiresRecreateHint)
		}
	}

	sourceDriver, err := iofs.New(migrationFiles, "migrations")
	if err != nil {
		return false, fmt.Errorf("failed to create migration source: %w", err)
	}

	dbDriver, err := sqlite3.WithInstance(db, &sqlite3.Config{
		MigrationsTable: "schema_migrations",
		NoTxWrap:        false,
	})
	if err != nil {
		return false, fmt.Errorf("failed to create database driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", sourceDriver, "sqlite3", dbDriver)
	if err != nil {
		return false, fmt.Errorf("failed to create migrate instance: %w", err)
	}

	err = m.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return false, fmt.Errorf("failed to run migrations: %w", err)
	}

	// Report whether the baseline was applied; seed execution is independent
	// so a previous seed failure can recover on the next startup.
	return !errors.Is(err, migrate.ErrNoChange), nil
}

// maxMigrationSourceVersion returns the highest version available in the
// embedded migration source (1 while the series is squashed to the single
// baseline migration).
func maxMigrationSourceVersion() (uint, error) {
	source, err := iofs.New(migrationFiles, "migrations")
	if err != nil {
		return 0, fmt.Errorf("failed to create migration source: %w", err)
	}
	defer source.Close()

	first, err := source.First()
	if err != nil {
		return 0, fmt.Errorf("failed to read first migration: %w", err)
	}

	version := first
	for {
		next, err := source.Next(version)
		if errors.Is(err, os.ErrNotExist) {
			return version, nil
		}
		if err != nil {
			return 0, fmt.Errorf("failed to read next migration after version %d: %w", version, err)
		}
		version = next
	}
}

// storedMigrationVersion reads the version golang-migrate recorded in
// schema_migrations, and whether that table exists at all. Bootstrap
// introspection deliberately uses database/sql: this table is owned by
// golang-migrate, sits outside the sqlc application schema, and may not exist
// until the first migration runs.
func storedMigrationVersion(db *sql.DB) (version uint64, exists bool, err error) {
	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='schema_migrations'`).Scan(&count)
	if err != nil {
		return 0, false, err
	}
	if count == 0 {
		return 0, false, nil
	}

	var v sql.NullInt64
	err = db.QueryRow(`SELECT version FROM schema_migrations LIMIT 1`).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		// golang-migrate can leave an empty version table after interruption
		// before recording the baseline. Let the runner resume normally.
		return 0, true, nil
	}
	if err != nil {
		return 0, true, err
	}
	return uint64(v.Int64), true, nil
}

// hasUnversionedTables is a bootstrap safety check, not legacy schema
// detection: any nonempty unversioned database requires an explicit reset.
func hasUnversionedTables(db *sql.DB) (bool, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'`).Scan(&count)
	return count > 0, err
}

// GetMigrationVersion returns the current migration version and dirty state.
// schema_migrations is owned by golang-migrate, not the sqlc application schema.
func (s *SQLite) GetMigrationVersion() (version uint, dirty bool, err error) {
	var v sql.NullInt64
	var d bool
	query := `SELECT version, dirty FROM schema_migrations LIMIT 1`
	err = s.db.QueryRow(query).Scan(&v, &d)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return uint(v.Int64), d, nil
}
