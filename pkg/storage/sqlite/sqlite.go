package sqlite

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	_ "github.com/mattn/go-sqlite3"
)

// SQLite wraps a *sql.DB and an embedded *sqlcdb.Queries that is bound to the
// base DB (not any transaction). Callers that open their own transaction via
// db.BeginTx must NOT call methods on the embedded Queries inside that
// transaction — those calls would run on the base DB and silently bypass the
// tx. Use txQueries(tx) to get a transaction-scoped *sqlcdb.Queries instead.
type SQLite struct {
	db   *sql.DB
	path string
	*sqlcdb.Queries
}

// txQueries returns a *sqlcdb.Queries scoped to tx so that sqlc-generated
// calls participate in the transaction rather than running on the base DB.
func (s *SQLite) txQueries(tx *sql.Tx) *sqlcdb.Queries {
	return s.Queries.WithTx(tx)
}

// seedSQL is the embedded default data (defaults.sql) applied after
// migrations. Statements use INSERT OR IGNORE, so rows backed by a key
// (quality_profile, quality_profile_item) are never duplicated. Seeding is
// deliberately not a migration so that DDL history stays separate from
// default data. seed guards the initial population transactionally because
// quality_definition has no uniqueness constraint on its seed values.
//
//go:embed defaults.sql
var seedSQL string

const (
	timestampFormat = "2006-01-02 15:04:05"

	// inMemoryPath selects a private in-memory SQLite database.
	inMemoryPath = ":memory:"

	// dsnParams configures every pooled SQLite connection:
	//
	//   - _foreign_keys=on: enforce foreign keys (SQLite defaults them off).
	//     The driver applies DSN params per connection, so every pooled
	//     connection enforces them.
	//   - _busy_timeout=5000: wait up to 5s for a locked database instead of
	//     failing immediately with SQLITE_BUSY.
	//   - _journal_mode=WAL and _synchronous=NORMAL: write-ahead logging with
	//     a fast-but-safe sync level; WAL allows concurrent readers with one
	//     writer.
	//   - _txlock=immediate: BEGIN IMMEDIATE for every transaction, so write
	//     transactions acquire the write lock up front instead of failing at
	//     COMMIT time with SQLITE_BUSY. Combined with the busy timeout this
	//     serializes writers cleanly under the shared WAL.
	dsnParams = "?_foreign_keys=on&_busy_timeout=5000&_journal_mode=WAL&_synchronous=NORMAL&_txlock=immediate"
)

// New creates a new sqlite database given a path to the database file. It
// opens the database, runs migrations (forward-only baseline), and applies
// the idempotent default seed.
func New(ctx context.Context, filePath string) (storage.Storage, error) {
	dsn := filePath + dsnParams
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	if filePath == inMemoryPath {
		// Every pooled connection that opens ":memory:" gets its own private
		// database. With more than one connection, concurrent callers could
		// silently see different (possibly un-migrated, un-seeded) databases,
		// so restrict the pool to a single shared connection.
		db.SetMaxOpenConns(1)
	}

	s := &SQLite{
		db:      db,
		path:    filePath,
		Queries: sqlcdb.New(db),
	}

	_, err = s.migrateAndReport()
	if err != nil {
		db.Close()
		return nil, err
	}
	if err := s.seed(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to seed database: %w", err)
	}

	return s, nil
}

// RunMigrations applies pending database migrations.
func (s *SQLite) RunMigrations(ctx context.Context) error {
	_, err := s.migrateAndReport()
	return err
}

// migrateAndReport runs migrations and reports whether they changed the
// database (i.e. the baseline was just applied, as opposed to the database
// already being at the baseline version).
func (s *SQLite) migrateAndReport() (bool, error) {
	return runMigrations(s.db, s.path)
}

// seed initializes defaults in one immediate transaction. Unlike profiles and
// profile items, quality definitions have no unique seed key. Use their presence
// as the population guard instead of blindly repeating INSERT OR IGNORE. The
// transaction makes this safe for simultaneous New calls and rolls back all
// defaults on failure, allowing the next New to retry after migration succeeded.
// Existing defaults (including operator customizations) are left untouched.
func (s *SQLite) seed(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var populated bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM quality_definition)`).Scan(&populated); err != nil {
		return err
	}
	if !populated {
		if _, err := tx.ExecContext(ctx, seedSQL); err != nil {
			return err
		}
	}
	return tx.Commit()
}
