package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/table"
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
	if err != nil {
		return err
	}

	return nil
}

// ListIndexer lists the stored indexers
func (s SQLite) ListIndexers(ctx context.Context) ([]*model.Indexers, error) {
	indexers := make([]*model.Indexers, 0)

	stmt := table.Indexers.SELECT(table.Indexers.AllColumns).FROM(table.Indexers).ORDER_BY(table.Indexers.Priority.DESC())
	err := stmt.QueryContext(ctx, s.db, &indexers)
	if err != nil {
		return nil, fmt.Errorf("failed to list indexers: %w", err)
	}
	return indexers, nil
}

// CreateMovie stores a movie
func (s SQLite) CreateMovie(ctx context.Context, movie model.Movies) (int32, error) {
	stmt := table.Movies.INSERT(table.Movies.MutableColumns).RETURNING(table.Movies.ID).MODEL(movie).ON_CONFLICT(table.Movies.ID).DO_NOTHING()
	result, err := s.handleInsert(ctx, stmt)
	if err != nil {
		return 0, err
	}

	inserted, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	// hope for the best I guess
	res := int32(inserted)
	return res, nil
}

// DeleteMovie removes a movie by id
func (s SQLite) DeleteMovie(ctx context.Context, id int64) error {
	stmt := table.Movies.DELETE().WHERE(table.Movies.ID.EQ(sqlite.Int64(id))).RETURNING(table.Movies.ID)
	_, err := s.handleDelete(ctx, stmt)
	if err != nil {
		return err
	}
	return nil
}

// ListMovies lists the stored movies
func (s SQLite) ListMovies(ctx context.Context) ([]*model.Movies, error) {
	movies := make([]*model.Movies, 0)
	stmt := table.Movies.SELECT(table.Movies.AllColumns).FROM(table.Movies).ORDER_BY(table.Movies.Added.ASC())
	err := stmt.QueryContext(ctx, s.db, &movies)
	if err != nil {
		return nil, fmt.Errorf("failed to list movies: %w", err)
	}

	return movies, nil
}

// CreateMovieFile stores a movie file
func (s SQLite) CreateMovieFile(ctx context.Context, file model.MovieFiles) (int32, error) {
	// Exclude DateAdded so that the default is used
	stmt := table.MovieFiles.INSERT(table.MovieFiles.MutableColumns.Except(table.MovieFiles.DateAdded)).RETURNING(table.MovieFiles.ID).MODEL(file)
	result, err := s.handleInsert(ctx, stmt)
	if err != nil {
		return 0, err
	}

	inserted, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	// hope for the best I guess
	res := int32(inserted)
	return res, nil
}

// DeleteMovieFile removes a movie file by id
func (s SQLite) DeleteMovieFile(ctx context.Context, id int64) error {
	stmt := table.MovieFiles.DELETE().WHERE(table.MovieFiles.ID.EQ(sqlite.Int64(id))).RETURNING(table.MovieFiles.ID)
	_, err := s.handleDelete(ctx, stmt)
	if err != nil {
		return err
	}
	return nil
}

// ListMovieFiles lists the stored movie files
func (s SQLite) ListMovieFiles(ctx context.Context) ([]*model.MovieFiles, error) {
	movieFiles := make([]*model.MovieFiles, 0)
	stmt := table.MovieFiles.SELECT(table.MovieFiles.AllColumns).FROM(table.MovieFiles).ORDER_BY(table.MovieFiles.ID.ASC())
	err := stmt.QueryContext(ctx, s.db, &movieFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to list movie files: %w", err)
	}

	return movieFiles, nil
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
