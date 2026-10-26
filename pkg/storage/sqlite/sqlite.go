package sqlite

import (
	"context"
	"database/sql"
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
	db *sql.DB
	*sqlcdb.Queries
}

// txQueries returns a *sqlcdb.Queries scoped to tx so that sqlc-generated
// calls participate in the transaction rather than running on the base DB.
func (s *SQLite) txQueries(tx *sql.Tx) *sqlcdb.Queries {
	return s.Queries.WithTx(tx)
}

const (
	timestampFormat = "2006-01-02 15:04:05"
)

// New creates a new sqlite database given a path to the database file.
func New(ctx context.Context, filePath string) (storage.Storage, error) {
	dsn := filePath + "?_busy_timeout=5000&_journal_mode=WAL&_synchronous=NORMAL&_txlock=immediate"
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}

	s := &SQLite{
		db:      db,
		Queries: sqlcdb.New(db),
	}

	if err := s.RunMigrations(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return s, nil
}

func (s *SQLite) RunMigrations(ctx context.Context) error {
	return runMigrations(s.db)
}

// Init applies the provided schema file contents to the database
func (s *SQLite) Init(ctx context.Context, schemas ...string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	for _, s := range schemas {
		_, err := tx.ExecContext(ctx, s)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}
