package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/go-jet/jet/v2/qrm"
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

const (
	timestampFormat = "2006-01-02 15:04:05"
)

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

// CreateMovie stores a movie and creates an initial transition state
func (s SQLite) CreateMovie(ctx context.Context, movie storage.Movie, initialState storage.MovieState) (int64, error) {
	if movie.State == "" {
		movie.State = storage.MovieStateNew
	}

	err := movie.Machine().ToState(initialState)
	if err != nil {
		return 0, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}

	setColumns := make([]sqlite.Expression, len(table.Movie.MutableColumns))
	for i, c := range table.Movie.MutableColumns {
		setColumns[i] = c
	}
	// don't insert a zeroed ID
	insertColumns := table.Movie.MutableColumns
	if movie.ID != 0 {
		insertColumns = table.Movie.AllColumns
	}

	stmt := table.Movie.
		INSERT(insertColumns).
		MODEL(movie.Movie).
		RETURNING(table.Movie.ID).
		ON_CONFLICT(table.Movie.ID).
		DO_UPDATE(sqlite.SET(table.Movie.MutableColumns.SET(sqlite.ROW(setColumns...))))

	result, err := stmt.ExecContext(ctx, tx)
	if err != nil {
		return 0, err
	}

	inserted, err := result.LastInsertId()
	if err != nil {
		tx.Rollback()
		return 0, err
	}

	state := storage.MovieTransition{
		MovieID:    int32(inserted),
		ToState:    string(initialState),
		MostRecent: true,
		SortKey:    1,
	}

	transitionStmt := table.MovieTransition.
		INSERT(table.MovieTransition.AllColumns.
			Except(table.MovieTransition.ID, table.MovieTransition.CreatedAt, table.MovieTransition.UpdatedAt)).
		MODEL(state)

	_, err = transitionStmt.ExecContext(ctx, tx)
	if err != nil {
		tx.Rollback()
		return 0, err
	}

	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		return 0, err
	}

	return inserted, nil
}

func (s SQLite) GetMovie(ctx context.Context, id int64) (*storage.Movie, error) {
	stmt := sqlite.
		SELECT(
			table.Movie.AllColumns,
			table.MovieTransition.AllColumns).
		FROM(
			table.Movie.INNER_JOIN(
				table.MovieTransition,
				table.Movie.ID.EQ(table.MovieTransition.MovieID))).
		WHERE(
			table.Movie.ID.EQ(sqlite.Int(id)).
				AND(table.MovieTransition.MostRecent.EQ(sqlite.Bool(true))),
		)

	movie := new(storage.Movie)
	err := stmt.QueryContext(ctx, s.db, movie)
	return movie, err
}

// ListMovies lists the stored movies
func (s SQLite) ListMovies(ctx context.Context) ([]*storage.Movie, error) {
	movies := make([]*storage.Movie, 0)
	stmt := sqlite.
		SELECT(
			table.Movie.AllColumns,
			table.MovieTransition.ToState,
			table.MovieTransition.DownloadClientID,
			table.MovieTransition.DownloadID,
			table.MovieTransition.MostRecent).
		FROM(
			table.Movie.INNER_JOIN(
				table.MovieTransition,
				table.Movie.ID.EQ(table.MovieTransition.MovieID))).
		WHERE(
			table.MovieTransition.MostRecent.EQ(sqlite.Bool(true))).
		ORDER_BY(table.Movie.Added.ASC())

	err := stmt.QueryContext(ctx, s.db, &movies)
	return movies, err
}

func (s SQLite) ListMoviesByState(ctx context.Context, state storage.MovieState) ([]*storage.Movie, error) {
	stmt := sqlite.
		SELECT(
			table.Movie.AllColumns,
			table.MovieTransition.ToState,
			table.MovieTransition.DownloadClientID,
			table.MovieTransition.DownloadID).
		FROM(
			table.Movie.INNER_JOIN(
				table.MovieTransition,
				table.Movie.ID.EQ(table.MovieTransition.MovieID))).
		WHERE(
			table.MovieTransition.MostRecent.EQ(sqlite.Bool(true)).
				AND(table.MovieTransition.ToState.EQ(sqlite.String(string(state))))).
		ORDER_BY(table.Movie.Added.ASC())

	movies := make([]*storage.Movie, 0)
	err := stmt.QueryContext(ctx, s.db, &movies)
	return movies, err
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

// UpdateMovieMovieFileID updates the movie file id for a movie
func (s SQLite) UpdateMovieMovieFileID(ctx context.Context, id int64, fileID int64) error {
	stmt := table.Movie.UPDATE().
		SET(table.Movie.MovieFileID.SET(sqlite.Int64(fileID))).WHERE(table.Movie.ID.EQ(sqlite.Int64(id)))
	_, err := s.handleStatement(ctx, stmt)
	return err
}

// UpdateMovieState updates the transition state of a movie. Metadata is optional and can be nil
func (s SQLite) UpdateMovieState(ctx context.Context, id int64, state storage.MovieState, metadata *storage.MovieStateMetadata) error {
	movie, err := s.GetMovie(ctx, id)
	if err != nil {
		return err
	}

	err = movie.Machine().ToState(state)
	if err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	previousTransitionStmt := table.MovieTransition.
		UPDATE().
		SET(
			table.MovieTransition.MostRecent.SET(sqlite.Bool(false)),
			table.MovieTransition.UpdatedAt.SET(sqlite.TimestampExp(sqlite.String(time.Now().Format(timestampFormat))))).
		WHERE(
			table.MovieTransition.MovieID.EQ(sqlite.Int(id)).
				AND(table.MovieTransition.MostRecent.EQ(sqlite.Bool(true)))).
		RETURNING(table.MovieTransition.AllColumns)

	var previousTransition storage.MovieTransition
	err = previousTransitionStmt.QueryContext(ctx, tx, &previousTransition)
	if err != nil {
		tx.Rollback()
		return err
	}

	transition := storage.MovieTransition{
		MovieID:    int32(id),
		ToState:    string(state),
		MostRecent: true,
		SortKey:    previousTransition.SortKey + 1,
	}

	if metadata != nil {
		if metadata.DownloadClientID != nil && metadata.DownloadID != nil {
			transition.DownloadClientID = metadata.DownloadClientID
			transition.DownloadID = metadata.DownloadID
		}
	}

	newTransitionStmt := table.MovieTransition.
		INSERT(table.MovieTransition.AllColumns.
			Except(table.MovieTransition.ID, table.MovieTransition.CreatedAt, table.MovieTransition.UpdatedAt)).
		MODEL(transition).
		RETURNING(table.MovieTransition.AllColumns)

	_, err = newTransitionStmt.ExecContext(ctx, tx)
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

// GetMovieByTmdb checks if there's a movie already associated with the given tmdb id
func (s SQLite) GetMovieByMetadataID(ctx context.Context, metadataID int) (*storage.Movie, error) {
	movie := new(storage.Movie)

	stmt := table.Movie.
		SELECT(
			table.Movie.AllColumns,
			table.MovieTransition.ToState).
		FROM(
			table.Movie.INNER_JOIN(
				table.MovieTransition,
				table.Movie.ID.EQ(table.MovieTransition.MovieID))).
		WHERE(
			table.Movie.MovieMetadataID.EQ(sqlite.Int(int64(metadataID))).
				AND(table.MovieTransition.MostRecent.EQ(sqlite.Bool(true))),
		)

	err := stmt.QueryContext(ctx, s.db, movie)
	if err != nil {
		if errors.Is(err, qrm.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("failed to lookup movie: %w", err)
	}

	return movie, nil
}

func (s SQLite) GetMovieFiles(ctx context.Context, id int64) ([]*model.MovieFile, error) {
	stmt := table.MovieFile.
		SELECT(table.MovieFile.AllColumns).
		FROM(table.MovieFile).
		WHERE(table.MovieFile.MovieID.EQ(sqlite.Int64(id)))

	var result []*model.MovieFile
	err := stmt.QueryContext(ctx, s.db, &result)
	if err != nil {
		return result, err
	}

	if len(result) == 0 {
		return nil, storage.ErrNotFound
	}

	return result, err
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

// CreateMovieMetadata creates the given movieMeta
func (s SQLite) CreateMovieMetadata(ctx context.Context, movieMeta model.MovieMetadata) (int64, error) {
	stmt := table.MovieMetadata.INSERT(table.MovieMetadata.MutableColumns).MODEL(movieMeta).ON_CONFLICT(table.MovieMetadata.ID).DO_NOTHING()
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

// DeleteMovieMetadata deletes a movie metadata by id
func (s SQLite) DeleteMovieMetadata(ctx context.Context, id int64) error {
	stmt := table.MovieMetadata.DELETE().WHERE(table.MovieMetadata.ID.EQ(sqlite.Int64(id))).RETURNING(table.MovieMetadata.ID)
	_, err := s.handleDelete(ctx, stmt)
	if err != nil {
		return err
	}
	return nil
}

// ListMovieMetadata lists all movie metadata
func (s SQLite) ListMovieMetadata(ctx context.Context) ([]*model.MovieMetadata, error) {
	movies := make([]*model.MovieMetadata, 0)
	stmt := table.Movie.SELECT(table.MovieMetadata.AllColumns).FROM(table.MovieMetadata).ORDER_BY(table.MovieMetadata.LastInfoSync.ASC())
	err := stmt.QueryContext(ctx, s.db, &movies)
	if err != nil {
		return nil, fmt.Errorf("failed to list movies: %w", err)
	}

	return movies, nil
}

// GetMovieMetadata get a movie metadata for the given where
func (s SQLite) GetMovieMetadata(ctx context.Context, where sqlite.BoolExpression) (*model.MovieMetadata, error) {
	meta := &model.MovieMetadata{}
	stmt := table.Movie.SELECT(table.MovieMetadata.AllColumns).FROM(table.MovieMetadata).WHERE(where).LIMIT(1)
	err := stmt.QueryContext(ctx, s.db, meta)
	if err != nil {
		if errors.Is(err, qrm.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("failed to list movies: %w", err)
	}

	return meta, nil
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

func (s SQLite) CreateQualityProfileItem(ctx context.Context, item model.QualityProfileItem) (int64, error) {
	stmt := table.QualityProfileItem.INSERT(table.QualityProfileItem.AllColumns.Except(table.QualityProfileItem.ID)).RETURNING(table.QualityProfileItem.ID).MODEL(item)
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

// GetQualityProfileItem gets a quality item that belongs to a profile
func (s SQLite) GetQualityProfileItem(ctx context.Context, id int64) (model.QualityProfileItem, error) {
	stmt := table.QualityProfileItem.SELECT(table.QualityProfileItem.AllColumns).FROM(table.QualityProfileItem).WHERE(table.QualityProfileItem.ID.EQ(sqlite.Int64(id)))
	var result model.QualityProfileItem
	err := stmt.QueryContext(ctx, s.db, &result)
	return result, err
}

// ListQualityProfileItem lists all quality definitions
func (s SQLite) ListQualityProfileItems(ctx context.Context) ([]*model.QualityProfileItem, error) {
	items := make([]*model.QualityProfileItem, 0)
	stmt := table.Indexer.SELECT(table.QualityProfileItem.AllColumns).FROM(table.QualityProfileItem).ORDER_BY(table.QualityProfileItem.ID.ASC())
	err := stmt.QueryContext(ctx, s.db, &items)
	return items, err
}

// DeleteQualityDefinition deletes a quality
func (s SQLite) DeleteQualityProfileItem(ctx context.Context, id int64) error {
	stmt := table.QualityProfileItem.DELETE().WHERE(table.QualityProfileItem.ID.EQ(sqlite.Int64(id))).RETURNING(table.QualityProfileItem.AllColumns)
	_, err := s.handleDelete(ctx, stmt)
	return err
}

func (s SQLite) CreateQualityProfile(ctx context.Context, profile model.QualityProfile) (int64, error) {
	stmt := table.QualityProfile.INSERT(table.QualityProfile.AllColumns.Except(table.QualityProfile.ID)).MODEL(profile).RETURNING(table.QualityProfile.ID)
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
		table.QualityProfileItem.AllColumns,
		table.QualityDefinition.AllColumns,
	).FROM(
		table.QualityProfile.INNER_JOIN(
			table.QualityProfileItem, table.QualityProfileItem.ProfileID.EQ(table.QualityProfile.ID)).INNER_JOIN(
			table.QualityDefinition, table.QualityProfileItem.QualityID.EQ(table.QualityDefinition.ID)),
	).WHERE(table.QualityProfile.ID.EQ(sqlite.Int(id))).ORDER_BY(table.QualityDefinition.MinSize.DESC())

	var result storage.QualityProfile
	err := stmt.QueryContext(ctx, s.db, &result)
	return result, err
}

// ListQualityProfiles lists all quality profiles and their associated quality items
func (s SQLite) ListQualityProfiles(ctx context.Context) ([]*storage.QualityProfile, error) {
	stmt := sqlite.
		SELECT(
			table.QualityProfile.AllColumns,
			table.QualityDefinition.AllColumns,
		).
		FROM(
			table.QualityProfile.INNER_JOIN(
				table.QualityProfileItem, table.QualityProfileItem.ProfileID.EQ(table.QualityProfile.ID)).INNER_JOIN(
				table.QualityDefinition, table.QualityProfileItem.QualityID.EQ(table.QualityDefinition.ID)),
		).
		ORDER_BY(table.QualityDefinition.MinSize.DESC())

	result := make([]*storage.QualityProfile, 0)
	err := stmt.QueryContext(ctx, s.db, &result)
	return result, err
}

// DeleteQualityProfile delete a quality profile
func (s SQLite) DeleteQualityProfile(ctx context.Context, id int64) error {
	stmt := table.QualityProfile.DELETE().WHERE(table.QualityProfile.ID.EQ(sqlite.Int64(id))).RETURNING(table.QualityProfile.AllColumns)
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

// GetDownloadClient gets a stored download client given an id
func (s SQLite) GetDownloadClient(ctx context.Context, id int64) (model.DownloadClient, error) {
	stmt := table.DownloadClient.SELECT(table.DownloadClient.AllColumns).FROM(table.DownloadClient).WHERE(table.DownloadClient.ID.EQ(sqlite.Int64(id)))
	var result model.DownloadClient
	err := stmt.QueryContext(ctx, s.db, &result)
	return result, err
}

// ListDownloadClients lists all stored download clients
func (s SQLite) ListDownloadClients(ctx context.Context) ([]*model.DownloadClient, error) {
	items := make([]*model.DownloadClient, 0)
	stmt := table.Indexer.SELECT(table.DownloadClient.AllColumns).FROM(table.DownloadClient).ORDER_BY(table.DownloadClient.ID.ASC())
	err := stmt.QueryContext(ctx, s.db, &items)
	return items, err
}

// DeleteDownloadClient deletes a download client given an id
func (s SQLite) DeleteDownloadClient(ctx context.Context, id int64) error {
	stmt := table.DownloadClient.DELETE().WHERE(table.DownloadClient.ID.EQ(sqlite.Int64(id)))
	_, err := s.handleDelete(ctx, stmt)
	return err
}

// CreateDownloadClient stores a new download client
func (s SQLite) CreateDownloadClient(ctx context.Context, profile model.DownloadClient) (int64, error) {
	stmt := table.DownloadClient.INSERT(table.DownloadClient.AllColumns.Except(table.DownloadClient.ID)).MODEL(profile).RETURNING(table.DownloadClient.ID)
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
