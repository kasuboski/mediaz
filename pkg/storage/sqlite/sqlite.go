package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/storage"
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

type SQLite struct {
	db *sql.DB
}

const (
	timestampFormat = "2006-01-02 15:04:05"
)

// New creates a new sqlite database given a path to the database file.
func New(ctx context.Context, filePath string) (storage.Storage, error) {
	// DSN parameters are applied to every connection opened by the pool.
	// busy_timeout and synchronous are per-connection settings and must be
	// set here rather than via one-shot PRAGMA calls. journal_mode=WAL is
	// persisted in the database file so it only needs to be set once, but
	// including it in the DSN is also correct and more explicit.
	dsn := filePath + "?_busy_timeout=5000&_journal_mode=WAL&_synchronous=NORMAL"
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}

	s := &SQLite{
		db: db,
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

func (s *SQLite) handleInsert(ctx context.Context, stmt sqlite.InsertStatement) (sql.Result, error) {
	return s.handleStatement(ctx, stmt)
}

func (s *SQLite) handleDelete(ctx context.Context, stmt sqlite.DeleteStatement) (sql.Result, error) {
	return s.handleStatement(ctx, stmt)
}

func (s *SQLite) handleStatement(ctx context.Context, stmt sqlite.Statement) (sql.Result, error) {
	log := logger.FromCtx(ctx)
	var result sql.Result

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		log.Debug("failed to init transaction", zap.Error(err))
		return result, err
	}

	result, err = stmt.ExecContext(ctx, tx)
	if err != nil {
		log.Debug("failed to execute statement", zap.String("query", stmt.DebugSql()), zap.Error(err))
		tx.Rollback()
		return result, err
	}

	return result, tx.Commit()
}
