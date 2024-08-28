package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/table"
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

var (
	ErrNoRowsDeleted = errors.New("no rows deleted")
)

type SQLite struct {
	db *sql.DB
}

// New creates a new sqlite database given a path to the database file
func New(filePath string) (storage.Storage, error) {
	db, err := sql.Open("sqlite3", filePath)
	if err != nil {
		return nil, err
	}

	return SQLite{
		db: db,
	}, nil
}

// Init applies the provided schema file contents to the database
func (s SQLite) Init(ctx context.Context, schemas ...string) error {
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

// CreateIndexer stores a new indexer in the database
func (s SQLite) CreateIndexer(ctx context.Context, name string, priority int) (int64, error) {
	stmt := table.Indexers.INSERT(table.Indexers.Name, table.Indexers.Priority).VALUES(name, priority).ON_CONFLICT(table.Indexers.Name).DO_NOTHING()
	_, err := s.handleInsert(ctx, stmt)
	if err != nil {
		return 0, err
	}

	return 0, nil
}

// DeleteIndexer deletes a stored indexer given the indexer ID
func (s SQLite) DeleteIndexer(ctx context.Context, id int64) error {
	stmt := table.Indexers.DELETE().WHERE(table.Indexers.ID.EQ(sqlite.Int64(id))).RETURNING(table.Indexers.AllColumns)
	_, err := s.handleDelete(ctx, stmt)
	if err != nil {
		return err
	}

	return nil
}

func (s SQLite) handleInsert(ctx context.Context, stmt sqlite.InsertStatement) (sql.Result, error) {
	return s.handleStatement(ctx, stmt, 1)
}

func (s SQLite) handleDelete(ctx context.Context, stmt sqlite.DeleteStatement) (sql.Result, error) {
	return s.handleStatement(ctx, stmt, 1)
}

func (s SQLite) handleStatement(ctx context.Context, stmt sqlite.Statement, expectedRows int64) (sql.Result, error) {
	log := logger.FromCtx(ctx)

	var result sql.Result
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}

	query, args := stmt.Sql()
	result, err = tx.ExecContext(ctx, query, args)
	if err != nil {
		log.Debug("failed to execute statement", zap.String("query", stmt.DebugSql()), zap.Error(err))
		tx.Rollback()
		return result, err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		log.Debug("failed to get number of rows affected", zap.Error(err))
		tx.Rollback()
		return result, err
	}

	if rows != expectedRows {
		log.Debug("unexpected number of rows effected", zap.Int64("rows", rows), zap.Int64("expected rows", expectedRows), zap.Error(err))
		tx.Rollback()
		return result, ErrNoRowsDeleted
	}

	return result, tx.Commit()
}
