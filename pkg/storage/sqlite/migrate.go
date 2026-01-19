package sqlite

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// runMigrations executes pending database migrations
func runMigrations(db *sql.DB) error {
	isLegacy, err := isLegacyDatabase(db)
	if err != nil {
		return fmt.Errorf("failed to check database type: %w", err)
	}

	sourceDriver, err := iofs.New(migrationFiles, "migrations")
	if err != nil {
		return fmt.Errorf("failed to create migration source: %w", err)
	}

	dbDriver, err := sqlite3.WithInstance(db, &sqlite3.Config{
		MigrationsTable: "schema_migrations",
		NoTxWrap:        true,
	})
	if err != nil {
		return fmt.Errorf("failed to create database driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", sourceDriver, "sqlite3", dbDriver)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}

	if isLegacy {
		if err := baselineLegacyDatabase(m); err != nil {
			return fmt.Errorf("failed to baseline legacy database: %w", err)
		}
	}

	err = m.Up()
	if err == migrate.ErrNoChange {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// isLegacyDatabase checks if the database was created without the migration system
func isLegacyDatabase(db *sql.DB) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='schema_migrations'`
	err := db.QueryRow(query).Scan(&count)
	if err != nil {
		return false, err
	}

	if count > 0 {
		return false, nil
	}

	query = `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='quality_profile'`
	err = db.QueryRow(query).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// baselineLegacyDatabase marks a legacy database as being at version 1
func baselineLegacyDatabase(m *migrate.Migrate) error {
	// Force the version to 1 without running migration 1
	// This tells the migration system that migration 1 has already been applied
	if err := m.Force(1); err != nil {
		return fmt.Errorf("failed to force version 1: %w", err)
	}
	return nil
}

// GetMigrationVersion returns the current migration version and dirty state
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
