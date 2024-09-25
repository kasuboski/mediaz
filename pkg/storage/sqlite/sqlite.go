package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"log"

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
func (s SQLite) CreateIndexer(ctx context.Context, indexer model.Indexer) (int64, error) {
	stmt := table.Indexer.INSERT(table.Indexer.AllColumns.Except(table.Indexer.ID)).MODEL(indexer).ON_CONFLICT(table.Indexer.Name).DO_NOTHING().RETURNING(table.Indexer.AllColumns)
	result, err := s.handleInsert(ctx, stmt)
	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

// DeleteIndexer deletes a stored indexer given the indexer ID
func (s SQLite) DeleteIndexer(ctx context.Context, id int64) error {
	stmt := table.Indexer.DELETE().WHERE(table.Indexer.ID.EQ(sqlite.Int64(id))).RETURNING(table.Indexer.AllColumns)
	_, err := s.handleDelete(ctx, stmt)
	return err
}

// ListIndexer lists the stored indexers
func (s SQLite) ListIndexers(ctx context.Context) ([]*model.Indexer, error) {
	indexers := make([]*model.Indexer, 0)

	stmt := table.Indexer.SELECT(table.Indexer.AllColumns).FROM(table.Indexer).ORDER_BY(table.Indexer.Priority.DESC())
	err := stmt.QueryContext(ctx, s.db, &indexers)
	if err != nil {
		return nil, fmt.Errorf("failed to list indexers: %w", err)
	}

	return indexers, nil
}

// CreateMovie stores a movie
func (s SQLite) CreateMovie(ctx context.Context, movie model.Movie) (int64, error) {
	stmt := table.Movie.INSERT(table.Movie.MutableColumns).RETURNING(table.Movie.ID).MODEL(movie).ON_CONFLICT(table.Movie.ID).DO_NOTHING()
	result, err := s.handleInsert(ctx, stmt)
	if err != nil {
		return 0, err
	}

	inserted, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return inserted, nil

}

// DeleteMovie removes a movie by id
func (s SQLite) DeleteMovie(ctx context.Context, id int64) error {
	stmt := table.Movie.DELETE().WHERE(table.Movie.ID.EQ(sqlite.Int64(id))).RETURNING(table.Movie.ID)
	_, err := s.handleDelete(ctx, stmt)
	if err != nil {
		return err
	}
	return nil
}

// ListMovies lists the stored movies
func (s SQLite) ListMovies(ctx context.Context) ([]*model.Movie, error) {
	movies := make([]*model.Movie, 0)
	stmt := table.Movie.SELECT(table.Movie.AllColumns).FROM(table.Movie).ORDER_BY(table.Movie.Added.ASC())
	err := stmt.QueryContext(ctx, s.db, &movies)
	if err != nil {
		return nil, fmt.Errorf("failed to list movies: %w", err)
	}

	return movies, nil
}

// CreateMovieFile stores a movie file
func (s SQLite) CreateMovieFile(ctx context.Context, file model.MovieFile) (int64, error) {
	// Exclude DateAdded so that the default is used
	stmt := table.MovieFile.INSERT(table.MovieFile.MutableColumns.Except(table.MovieFile.DateAdded).Except(table.MovieFile.ID)).RETURNING(table.MovieFile.ID).MODEL(file)
	result, err := s.handleInsert(ctx, stmt)
	if err != nil {
		return 0, err
	}

	inserted, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return inserted, nil
}

// DeleteMovieFile removes a movie file by id
func (s SQLite) DeleteMovieFile(ctx context.Context, id int64) error {
	stmt := table.MovieFile.DELETE().WHERE(table.MovieFile.ID.EQ(sqlite.Int64(id))).RETURNING(table.MovieFile.ID)
	_, err := s.handleDelete(ctx, stmt)
	if err != nil {
		return err
	}
	return nil
}

// ListMovieFiles lists the stored movie files
func (s SQLite) ListMovieFiles(ctx context.Context) ([]*model.MovieFile, error) {
	movieFiles := make([]*model.MovieFile, 0)
	stmt := table.MovieFile.SELECT(table.MovieFile.AllColumns).FROM(table.MovieFile).ORDER_BY(table.MovieFile.ID.ASC())
	err := stmt.QueryContext(ctx, s.db, &movieFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to list movie files: %w", err)
	}

	return movieFiles, nil
}

// CreateQualityDefinition store a new quality definition
func (s SQLite) CreateQualityDefinition(ctx context.Context, definition model.QualityDefinition) (int64, error) {
	stmt := table.QualityDefinition.INSERT(table.QualityDefinition.AllColumns.Except(table.QualityDefinition.ID)).MODEL(definition).RETURNING(table.QualityDefinition.ID)
	result, err := s.handleInsert(ctx, stmt)
	if err != nil {
		return 0, err
	}

	inserted, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return inserted, nil
}

// GetQualityDefinition gets a quality definition
func (s SQLite) GetQualityDefinition(ctx context.Context, id int64) (model.QualityDefinition, error) {
	stmt := table.QualityDefinition.SELECT(table.QualityDefinition.AllColumns).FROM(table.QualityDefinition).WHERE(table.QualityDefinition.ID.EQ(sqlite.Int64(id))).ORDER_BY(table.QualityDefinition.ID.ASC())
	var result model.QualityDefinition
	err := stmt.QueryContext(ctx, s.db, &result)
	return result, err
}

// ListQualityDefinitions lists all quality definitions
func (s SQLite) ListQualityDefinitions(ctx context.Context) ([]*model.QualityDefinition, error) {
	definitions := make([]*model.QualityDefinition, 0)
	stmt := table.Indexer.SELECT(table.QualityDefinition.AllColumns).FROM(table.QualityDefinition).ORDER_BY(table.QualityDefinition.ID.ASC())
	err := stmt.QueryContext(ctx, s.db, &definitions)
	return definitions, err
}

// DeleteQualityDefinition deletes a quality
func (s SQLite) DeleteQualityDefinition(ctx context.Context, id int64) error {
	stmt := table.QualityDefinition.DELETE().WHERE(table.QualityDefinition.ID.EQ(sqlite.Int64(id))).RETURNING(table.QualityDefinition.AllColumns)
	_, err := s.handleDelete(ctx, stmt)
	return err
}

func (s SQLite) CreateProfileQualityItem(ctx context.Context, item model.ProfileQualityItem) (int64, error) {
	stmt := table.ProfileQualityItem.INSERT(table.ProfileQualityItem.AllColumns.Except(table.ProfileQualityItem.ID)).RETURNING(table.ProfileQualityItem.ID).MODEL(item)
	result, err := s.handleInsert(ctx, stmt)
	if err != nil {
		return 0, err
	}

	inserted, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return inserted, nil
}

// GetProfileQualityItem gets a quality item that belongs to a profile
func (s SQLite) GetProfileQualityItem(ctx context.Context, id int64) (model.ProfileQualityItem, error) {
	stmt := table.ProfileQualityItem.SELECT(table.QualityDefinition.AllColumns).FROM(table.ProfileQualityItem).WHERE(table.ProfileQualityItem.ID.EQ(sqlite.Int64(id)))
	var result model.ProfileQualityItem
	err := stmt.QueryContext(ctx, s.db, &result)
	return result, err
}

// ListProfileQualityItem lists all quality definitions
func (s SQLite) ListProfileQualityItems(ctx context.Context) ([]*model.ProfileQualityItem, error) {
	items := make([]*model.ProfileQualityItem, 0)
	stmt := table.Indexer.SELECT(table.ProfileQualityItem.AllColumns).FROM(table.ProfileQualityItem).ORDER_BY(table.ProfileQualityItem.ID.ASC())
	err := stmt.QueryContext(ctx, s.db, &items)
	return items, err
}

// DeleteQualityDefinition deletes a quality
func (s SQLite) DeleteProfileQualityItem(ctx context.Context, id int64) error {
	stmt := table.ProfileQualityItem.DELETE().WHERE(table.ProfileQualityItem.ID.EQ(sqlite.Int64(id))).RETURNING(table.ProfileQualityItem.AllColumns)
	_, err := s.handleDelete(ctx, stmt)
	return err
}

func (s SQLite) CreateQualityProfile(ctx context.Context, profile model.QualityProfile) (int64, error) {
	stmt := table.QualityProfile.INSERT(table.QualityProfile.AllColumns.Except(table.QualityProfile.ID)).MODEL(profile).RETURNING(table.QualityProfile.ID)
	log.Println(stmt.DebugSql())
	result, err := s.handleInsert(ctx, stmt)
	if err != nil {
		return 0, err
	}

	inserted, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return inserted, nil
}

// GetQualityProfile gets a quality profile and all associated quality items given a quality profile id
func (s SQLite) GetQualityProfile(ctx context.Context, id int64) (storage.QualityProfile, error) {
	stmt := sqlite.SELECT(
		table.QualityProfile.AllColumns,
		table.ProfileQualityItem.AllColumns,
		table.QualityDefinition.AllColumns,
	).FROM(
		table.QualityProfile.INNER_JOIN(
			table.ProfileQualityItem, table.ProfileQualityItem.ProfileID.EQ(table.QualityProfile.ID)).INNER_JOIN(
			table.QualityDefinition, table.ProfileQualityItem.QualityID.EQ(table.QualityDefinition.ID)),
	).WHERE(table.QualityProfile.ID.EQ(sqlite.Int(id))).ORDER_BY(table.QualityDefinition.MinSize.DESC())

	var result storage.QualityProfile
	err := stmt.QueryContext(ctx, s.db, &result)
	return result, err
}

// ListQualityProfiles lists all quality profiles and their associated quality items
func (s SQLite) ListQualityProfiles(ctx context.Context) ([]*storage.QualityProfile, error) {
	stmt := sqlite.SELECT(
		table.QualityProfile.AllColumns,
		table.QualityDefinition.AllColumns,
	).FROM(
		table.QualityProfile.INNER_JOIN(
			table.ProfileQualityItem, table.ProfileQualityItem.ProfileID.EQ(table.QualityProfile.ID)).INNER_JOIN(
			table.QualityDefinition, table.ProfileQualityItem.QualityID.EQ(table.QualityDefinition.ID)),
	).ORDER_BY(table.QualityDefinition.MinSize.DESC())

	result := make([]*storage.QualityProfile, 0)
	err := stmt.QueryContext(ctx, s.db, &result)
	return result, err
}

// DeleteQualityProfile delete a quality profile
func (s SQLite) DeleteQualityProfile(ctx context.Context, id int64) error {
	stmt := table.Indexer.DELETE().WHERE(table.QualityProfile.ID.EQ(sqlite.Int64(id))).RETURNING(table.QualityProfile.AllColumns)
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
