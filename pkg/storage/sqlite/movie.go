package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

// CreateMovie stores a movie and creates an initial transition state. When
// movie.Added is zero the column default (CURRENT_TIMESTAMP, UTC) applies.
func (s *SQLite) CreateMovie(ctx context.Context, movie sqlcdb.Movie, initialState storage.MovieState) (int64, error) {
	if err := storage.MovieStateNew.Machine().ToState(initialState); err != nil {
		return 0, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	q := s.txQueries(tx)

	params := sqlcdb.CreateMovieParams{
		Path:                movie.Path,
		Monitored:           movie.Monitored,
		QualityProfileID:    movie.QualityProfileID,
		Tags:                movie.Tags,
		AddOptions:          movie.AddOptions,
		MovieFileID:         movie.MovieFileID,
		MinimumAvailability: movie.MinimumAvailability,
		MovieMetadataID:     movie.MovieMetadataID,
		LastSearchTime:      movie.LastSearchTime,
	}

	var inserted int64
	if movie.Added.Valid && !movie.Added.Time.IsZero() {
		withAdded := sqlcdb.CreateMovieWithAddedParams{
			Added:               movie.Added,
			Path:                params.Path,
			Monitored:           params.Monitored,
			QualityProfileID:    params.QualityProfileID,
			Tags:                params.Tags,
			AddOptions:          params.AddOptions,
			MovieFileID:         params.MovieFileID,
			MinimumAvailability: params.MinimumAvailability,
			MovieMetadataID:     params.MovieMetadataID,
			LastSearchTime:      params.LastSearchTime,
		}
		inserted, err = q.CreateMovieWithAdded(ctx, withAdded)
	} else {
		inserted, err = q.CreateMovie(ctx, params)
	}
	if err != nil {
		tx.Rollback()
		return 0, err
	}

	if err := q.CreateMovieTransition(ctx, sqlcdb.CreateMovieTransitionParams{
		MovieID:    inserted,
		ToState:    string(initialState),
		MostRecent: true,
		SortKey:    1,
	}); err != nil {
		tx.Rollback()
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		tx.Rollback()
		return 0, err
	}

	return inserted, nil
}

func (s *SQLite) GetMovie(ctx context.Context, id int64) (sqlcdb.GetMovieByIDRow, error) {
	row, err := s.Queries.GetMovieByID(ctx, id)
	if err != nil {
		return sqlcdb.GetMovieByIDRow{}, mapQueryErr(err, "failed to get movie")
	}
	return row, nil
}

func (s *SQLite) GetMovieByMovieFileID(ctx context.Context, fileID int64) (sqlcdb.GetMovieByFileIDRow, error) {
	row, err := s.Queries.GetMovieByFileID(ctx, sql.NullInt64{Int64: fileID, Valid: true})
	if err != nil {
		return sqlcdb.GetMovieByFileIDRow{}, mapQueryErr(err, "failed to get movie by file id")
	}
	return row, nil
}

// ListMovies lists the stored movies ordered by metadata title.
func (s *SQLite) ListMovies(ctx context.Context) ([]sqlcdb.ListMoviesRow, error) {
	rows, err := s.Queries.ListMovies(ctx)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// ListLibraryMovies lists metadata-backed movies in stable title/id order.
func (s *SQLite) ListLibraryMovies(ctx context.Context, offset, limit int, states []storage.MovieState) ([]sqlcdb.ListLibraryMoviesRow, error) {
	if offset < 0 || limit <= 0 {
		return nil, fmt.Errorf("invalid library pagination: offset %d, limit %d", offset, limit)
	}
	stateStrings := make([]string, 0, len(states))
	for _, state := range states {
		stateStrings = append(stateStrings, string(state))
	}
	encoded, err := json.Marshal(stateStrings)
	if err != nil {
		return nil, err
	}
	return s.Queries.ListLibraryMovies(ctx, sqlcdb.ListLibraryMoviesParams{Offset: int64(offset), Limit: int64(limit), States: string(encoded)})
}
func (s *SQLite) CountLibraryMovies(ctx context.Context, states []storage.MovieState) (int64, error) {
	stateStrings := make([]string, 0, len(states))
	for _, state := range states {
		stateStrings = append(stateStrings, string(state))
	}
	encoded, err := json.Marshal(stateStrings)
	if err != nil {
		return 0, err
	}
	return s.Queries.CountLibraryMovies(ctx, string(encoded))
}
func (s *SQLite) CountLibraryMoviesByState(ctx context.Context) ([]sqlcdb.CountLibraryMoviesByStateRow, error) {
	return s.Queries.CountLibraryMoviesByState(ctx)
}

// ListMoviesByState lists movies whose current state is one of the given
// states. Calling it with no states applies no state filter.
func (s *SQLite) ListMoviesByState(ctx context.Context, states ...storage.MovieState) ([]sqlcdb.ListMoviesByStatesRow, error) {
	params := sqlcdb.ListMoviesByStatesParams{
		States: make([]string, 0, len(states)),
	}
	for _, state := range states {
		if state == storage.MovieStateNew {
			params.States = nil
			break
		}
		params.States = append(params.States, string(state))
	}
	if len(params.States) > 0 {
		params.StatesAny = sql.NullString{String: params.States[0], Valid: true}
	}

	rows, err := s.Queries.ListMoviesByStates(ctx, params)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// DeleteMovie removes a movie by id
func (s *SQLite) DeleteMovie(ctx context.Context, id int64) error {
	return s.Queries.DeleteMovie(ctx, id)
}

// UpdateMovieMovieFileID updates the movie file id for a movie
func (s *SQLite) UpdateMovieMovieFileID(ctx context.Context, id int64, fileID int64) error {
	return s.Queries.UpdateMovieMovieFileID(ctx, sqlcdb.UpdateMovieMovieFileIDParams{
		MovieFileID: sql.NullInt64{Int64: fileID, Valid: true},
		ID:          id,
	})
}

// UpdateMovieByID updates fields on a movie identified by id.
// Legacy parity: only the monitored column is persisted.
func (s *SQLite) UpdateMovieByID(ctx context.Context, id int64, movie sqlcdb.Movie) error {
	return s.Queries.UpdateMovieMonitored(ctx, sqlcdb.UpdateMovieMonitoredParams{
		Monitored: movie.Monitored,
		ID:        id,
	})
}

// UpdateMovieQualityProfile updates only the quality profile id for a movie
func (s *SQLite) UpdateMovieQualityProfile(ctx context.Context, id int64, qualityProfileID int64) error {
	return s.Queries.UpdateMovieQualityProfileID(ctx, sqlcdb.UpdateMovieQualityProfileIDParams{
		QualityProfileID: qualityProfileID,
		ID:               id,
	})
}

// UpdateMovieState updates the transition state of a movie. Metadata is
// optional and can be nil. The demote+insert pair runs in one transaction; on
// any failure nothing is written.
func (s *SQLite) UpdateMovieState(ctx context.Context, id int64, state storage.MovieState, metadata *storage.TransitionStateMetadata) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	q := s.txQueries(tx)
	movie, err := q.GetMovieByID(ctx, id)
	if err != nil {
		return mapQueryErr(err, "failed to get movie")
	}
	if err := storage.MovieState(movie.State).Machine().ToState(state); err != nil {
		return err
	}

	previousSortKey, err := q.DeactivateMovieTransition(ctx, id)
	if err != nil {
		tx.Rollback()
		return err
	}

	transition := sqlcdb.CreateMovieTransitionParams{
		MovieID:    id,
		FromState:  sql.NullString{String: movie.State, Valid: true},
		ToState:    string(state),
		MostRecent: true,
		SortKey:    previousSortKey + 1,
	}
	if metadata != nil {
		if metadata.DownloadClientID != nil {
			transition.DownloadClientID = sql.NullInt64{Int64: *metadata.DownloadClientID, Valid: true}
		}
		if metadata.DownloadID != nil {
			transition.DownloadID = sql.NullString{String: *metadata.DownloadID, Valid: true}
		}
	}

	if err := q.CreateMovieTransition(ctx, transition); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

// GetMovieByMetadataID checks if there's a movie already associated with the given metadata id
func (s *SQLite) GetMovieByMetadataID(ctx context.Context, metadataID int64) (sqlcdb.GetMovieByMetadataIDRow, error) {
	row, err := s.Queries.GetMovieByMetadataID(ctx, sql.NullInt64{Int64: metadataID, Valid: true})
	if err != nil {
		return sqlcdb.GetMovieByMetadataIDRow{}, mapQueryErr(err, "failed to lookup movie")
	}
	return row, nil
}

// GetMovieByPath gets a movie by path
func (s *SQLite) GetMovieByPath(ctx context.Context, path string) (sqlcdb.GetMovieByPathRow, error) {
	row, err := s.Queries.GetMovieByPath(ctx, sql.NullString{String: path, Valid: true})
	if err != nil {
		return sqlcdb.GetMovieByPathRow{}, mapQueryErr(err, "failed to lookup movie by path")
	}
	return row, nil
}

// LinkMovieMetadata links a movie with its metadata
func (s *SQLite) LinkMovieMetadata(ctx context.Context, movieID int64, metadataID int64) error {
	return s.Queries.LinkMovieMetadata(ctx, sqlcdb.LinkMovieMetadataParams{
		MovieMetadataID: sql.NullInt64{Int64: metadataID, Valid: true},
		ID:              movieID,
	})
}

// mapQueryErr converts sql.ErrNoRows to storage.ErrNotFound at the query
// boundary, wrapping everything else with the calling context.
func mapQueryErr(err error, msg string) error {
	if errors.Is(err, sql.ErrNoRows) {
		return storage.ErrNotFound
	}
	return fmt.Errorf("%s: %w", msg, err)
}
