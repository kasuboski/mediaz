package sqlite

import (
	"context"
	"database/sql"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
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
func (s SQLite) CreateIndexer(ctx context.Context, indexer model.Indexers) (int64, error) {
	stmt := table.Indexers.INSERT(table.Indexers.Name, table.Indexers.URI, table.Indexers.ApiKey, table.Indexers.Priority).MODEL(indexer).RETURNING(table.Indexers.AllColumns)
	result, err := s.handleInsert(ctx, stmt)
	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

// DeleteIndexer deletes a stored indexer given the indexer ID
func (s SQLite) DeleteIndexer(ctx context.Context, id int64) error {
	stmt := table.Indexers.DELETE().WHERE(table.Indexers.ID.EQ(sqlite.Int64(id))).RETURNING(table.Indexers.AllColumns)
	_, err := s.handleDelete(ctx, stmt)
	return err
}

// ListIndexer lists the stored indexers
func (s SQLite) ListIndexers(ctx context.Context) ([]*model.Indexers, error) {
	log := logger.FromCtx(ctx)

	indexers := make([]*model.Indexers, 0)

	stmt := table.Indexers.SELECT(table.Indexers.AllColumns).FROM(table.Indexers).ORDER_BY(table.Indexers.Priority.DESC())
	err := stmt.QueryContext(ctx, s.db, &indexers)
	if err != nil {
		log.Errorf("failed to list indexers: %w", err)
		return nil, err
	}

	return indexers, nil
}

// CreateIndexer stores a new indexer in the database
func (s SQLite) CreateQualityDefinition(ctx context.Context, definition model.QualityDefinitions) (int64, error) {
	stmt := table.QualityDefinitions.INSERT(table.QualityDefinitions.Name, table.QualityDefinitions.QualityId, table.QualityDefinitions.MinSize, table.QualityDefinitions.MaxSize).MODEL(definition).RETURNING(table.QualityDefinitions.AllColumns)
	result, err := s.handleInsert(ctx, stmt)
	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

// CreateIndexer stores a new indexer in the database
func (s SQLite) ListQualityDefinitions(ctx context.Context) ([]*model.QualityDefinitions, error) {
	definitions := make([]*model.QualityDefinitions, 0)
	stmt := table.Indexers.SELECT(table.QualityDefinitions.AllColumns).FROM(table.QualityDefinitions).ORDER_BY(table.QualityDefinitions.ID.ASC())
	err := stmt.QueryContext(ctx, s.db, &definitions)

	return definitions, err
}

// CreateIndexer stores a new indexer in the database
func (s SQLite) DeleteQualityDefinition(ctx context.Context, id int64) error {
	stmt := table.Indexers.DELETE().WHERE(table.QualityDefinitions.ID.EQ(sqlite.Int64(id))).RETURNING(table.QualityDefinitions.AllColumns)
	_, err := s.handleDelete(ctx, stmt)

	return err
}

func (s SQLite) handleInsert(ctx context.Context, stmt sqlite.InsertStatement) (sql.Result, error) {
	return s.handleStatement(ctx, stmt)
}

func (s SQLite) handleDelete(ctx context.Context, stmt sqlite.DeleteStatement) (sql.Result, error) {
	return s.handleStatement(ctx, stmt)
}

func (s SQLite) handleStatement(ctx context.Context, stmt sqlite.Statement) (sql.Result, error) {
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
