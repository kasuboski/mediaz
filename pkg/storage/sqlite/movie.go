package sqlite

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-jet/jet/v2/qrm"
	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
)

// CreateMovie stores a movie and creates an initial transition state
func (s *SQLite) CreateMovie(ctx context.Context, movie storage.Movie, initialState storage.MovieState) (int64, error) {
	if movie.State == "" {
		movie.State = storage.MovieStateNew
	}

	err := movie.Machine().ToState(initialState)
	if err != nil {
		return 0, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}

	setColumns := make([]sqlite.Expression, len(table.Movie.MutableColumns))
	for i, c := range table.Movie.MutableColumns {
		setColumns[i] = c
	}

	insertColumns := table.Movie.MutableColumns
	if movie.ID == 0 {
		insertColumns = insertColumns.Except(table.Movie.ID)
	}
	if movie.Added == nil || movie.Added.IsZero() {
		insertColumns = insertColumns.Except(table.Movie.Added)
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

func (s *SQLite) GetMovie(ctx context.Context, id int64) (*storage.Movie, error) {
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

func (s *SQLite) GetMovieByMovieFileID(ctx context.Context, fileID int64) (*storage.Movie, error) {
	stmt := sqlite.
		SELECT(
			table.Movie.AllColumns,
			table.MovieTransition.AllColumns).
		FROM(
			table.Movie.INNER_JOIN(
				table.MovieTransition,
				table.Movie.ID.EQ(table.MovieTransition.MovieID))).
		WHERE(
			table.Movie.MovieFileID.EQ(sqlite.Int(fileID)).
				AND(table.MovieTransition.MostRecent.EQ(sqlite.Bool(true))),
		)

	movie := new(storage.Movie)
	err := stmt.QueryContext(ctx, s.db, movie)
	return movie, err
}

// ListMovies lists the stored movies
func (s *SQLite) ListMovies(ctx context.Context) ([]*storage.Movie, error) {
	movies := make([]*storage.Movie, 0)
	stmt := sqlite.
		SELECT(
			table.Movie.AllColumns,
			table.MovieTransition.ToState,
			table.MovieTransition.DownloadClientID,
			table.MovieTransition.DownloadID,
			table.MovieTransition.MostRecent).
		FROM(
			table.Movie.
				INNER_JOIN(table.MovieTransition, table.Movie.ID.EQ(table.MovieTransition.MovieID)).
				LEFT_JOIN(table.MovieMetadata, table.Movie.MovieMetadataID.EQ(table.MovieMetadata.ID))).
		WHERE(
			table.MovieTransition.MostRecent.EQ(sqlite.Bool(true))).
		ORDER_BY(table.MovieMetadata.Title.ASC())

	err := stmt.QueryContext(ctx, s.db, &movies)
	return movies, err
}

func (s *SQLite) ListMoviesByState(ctx context.Context, state storage.MovieState) ([]*storage.Movie, error) {
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
func (s *SQLite) DeleteMovie(ctx context.Context, id int64) error {
	stmt := table.Movie.DELETE().WHERE(table.Movie.ID.EQ(sqlite.Int64(id))).RETURNING(table.Movie.ID)
	_, err := s.handleDelete(ctx, stmt)
	if err != nil {
		return err
	}
	return nil
}

// UpdateMovieMovieFileID updates the movie file id for a movie
func (s *SQLite) UpdateMovieMovieFileID(ctx context.Context, id int64, fileID int64) error {
	stmt := table.Movie.UPDATE().
		SET(table.Movie.MovieFileID.SET(sqlite.Int64(fileID))).WHERE(table.Movie.ID.EQ(sqlite.Int64(id)))
	_, err := s.handleStatement(ctx, stmt)
	return err
}

// UpdateMovie updates fields on a movie
func (s *SQLite) UpdateMovie(ctx context.Context, movie model.Movie, where ...sqlite.BoolExpression) error {
	stmt := table.Movie.UPDATE(table.Movie.Monitored).MODEL(movie)
	for _, w := range where {
		stmt = stmt.WHERE(w)
	}
	_, err := stmt.ExecContext(ctx, s.db)
	return err
}

// UpdateMovieQualityProfile updates only the quality profile id for a movie
func (s *SQLite) UpdateMovieQualityProfile(ctx context.Context, id int64, qualityProfileID int32) error {
	stmt := table.Movie.UPDATE().
		SET(table.Movie.QualityProfileID.SET(sqlite.Int32(qualityProfileID))).
		WHERE(table.Movie.ID.EQ(sqlite.Int64(id)))
	_, err := s.handleStatement(ctx, stmt)
	return err
}

// UpdateMovieState updates the transition state of a movie. Metadata is optional and can be nil
func (s *SQLite) UpdateMovieState(ctx context.Context, id int64, state storage.MovieState, metadata *storage.TransitionStateMetadata) error {
	movie, err := s.GetMovie(ctx, id)
	if err != nil {
		return err
	}

	err = movie.Machine().ToState(state)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
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

// GetMovieByMetadataID checks if there's a movie already associated with the given metadata id
func (s *SQLite) GetMovieByMetadataID(ctx context.Context, metadataID int) (*storage.Movie, error) {
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

// GetMovieByPath gets a movie by path
func (s *SQLite) GetMovieByPath(ctx context.Context, path string) (*storage.Movie, error) {
	stmt := table.Movie.
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
			table.Movie.Path.EQ(sqlite.String(path)).
				AND(table.MovieTransition.MostRecent.EQ(sqlite.Bool(true))))

	movie := new(storage.Movie)
	err := stmt.QueryContext(ctx, s.db, movie)
	return movie, err
}

// LinkMovieMetadata links a movie with its metadata
func (s *SQLite) LinkMovieMetadata(ctx context.Context, movieID int64, metadataID int32) error {
	stmt := table.Movie.UPDATE(table.Movie.MovieMetadataID).SET(metadataID).WHERE(table.Movie.ID.EQ(sqlite.Int64(movieID)))
	_, err := stmt.Exec(s.db)
	return err
}
