package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

// movieJoinRow mirrors the shared movie+movie_transition select shape used by
// the movie lookup and list queries so one conversion covers them all.
type movieJoinRow struct {
	ID                  int64
	Path                sql.NullString
	Monitored           int64
	QualityProfileID    int64
	Added               sql.NullTime
	Tags                sql.NullString
	AddOptions          sql.NullString
	MovieFileID         sql.NullInt64
	MinimumAvailability int64
	MovieMetadataID     sql.NullInt64
	LastSearchTime      sql.NullTime
	ToState             string
	DownloadClientID    sql.NullInt64
	DownloadID          sql.NullString
}

// movieMetaJoinRow is the movie+movie_transition shape used when download
// client details are not selected (see GetMovieByMetadataID).
type movieMetaJoinRow struct {
	ID                  int64
	Path                sql.NullString
	Monitored           int64
	QualityProfileID    int64
	Added               sql.NullTime
	Tags                sql.NullString
	AddOptions          sql.NullString
	MovieFileID         sql.NullInt64
	MinimumAvailability int64
	MovieMetadataID     sql.NullInt64
	LastSearchTime      sql.NullTime
	ToState             string
}

// movieNullStr converts a nullable string pointer to a driver NullString.
func movieNullStr(v *string) sql.NullString {
	if v == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *v, Valid: true}
}

// movieNullI64 converts a nullable int32 pointer to a driver NullInt64.
func movieNullI64(v *int32) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*v), Valid: true}
}

// movieNullTime converts a nullable time pointer to a driver NullTime.
func movieNullTime(v *time.Time) sql.NullTime {
	if v == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *v, Valid: true}
}

func movieStrPtr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	s := v.String
	return &s
}

func movieI32Ptr(v sql.NullInt64) *int32 {
	if !v.Valid {
		return nil
	}
	i := int32(v.Int64)
	return &i
}

func movieTimePtr(v sql.NullTime) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time
	return &t
}

func movieNullF64(v *float64) sql.NullFloat64 {
	if v == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *v, Valid: true}
}

func movieF64Ptr(v sql.NullFloat64) *float64 {
	if !v.Valid {
		return nil
	}
	f := v.Float64
	return &f
}

func movieCreateParams(m model.Movie) sqlcdb.CreateMovieParams {
	return sqlcdb.CreateMovieParams{
		Path:                movieNullStr(m.Path),
		Monitored:           int64(m.Monitored),
		QualityProfileID:    int64(m.QualityProfileID),
		Tags:                movieNullStr(m.Tags),
		AddOptions:          movieNullStr(m.AddOptions),
		MovieFileID:         movieNullI64(m.MovieFileID),
		MinimumAvailability: int64(m.MinimumAvailability),
		MovieMetadataID:     movieNullI64(m.MovieMetadataID),
		LastSearchTime:      movieNullTime(m.LastSearchTime),
	}
}

func movieCreateWithAddedParams(m model.Movie) sqlcdb.CreateMovieWithAddedParams {
	return sqlcdb.CreateMovieWithAddedParams{
		Added:               movieNullTime(m.Added),
		Path:                movieNullStr(m.Path),
		Monitored:           int64(m.Monitored),
		QualityProfileID:    int64(m.QualityProfileID),
		Tags:                movieNullStr(m.Tags),
		AddOptions:          movieNullStr(m.AddOptions),
		MovieFileID:         movieNullI64(m.MovieFileID),
		MinimumAvailability: int64(m.MinimumAvailability),
		MovieMetadataID:     movieNullI64(m.MovieMetadataID),
		LastSearchTime:      movieNullTime(m.LastSearchTime),
	}
}

func movieTransitionParams(t storage.MovieTransition) sqlcdb.CreateMovieTransitionParams {
	return sqlcdb.CreateMovieTransitionParams{
		MovieID:          int64(t.MovieID),
		ToState:          t.ToState,
		FromState:        movieNullStr(t.FromState),
		MostRecent:       t.MostRecent,
		SortKey:          int64(t.SortKey),
		DownloadClientID: movieNullI64(t.DownloadClientID),
		DownloadID:       movieNullStr(t.DownloadID),
	}
}

func movieFromJoinRow(r movieJoinRow) storage.Movie {
	return storage.Movie{
		Movie: model.Movie{
			ID:                  int32(r.ID),
			Path:                movieStrPtr(r.Path),
			Monitored:           int32(r.Monitored),
			QualityProfileID:    int32(r.QualityProfileID),
			Added:               movieTimePtr(r.Added),
			Tags:                movieStrPtr(r.Tags),
			AddOptions:          movieStrPtr(r.AddOptions),
			MovieFileID:         movieI32Ptr(r.MovieFileID),
			MinimumAvailability: int32(r.MinimumAvailability),
			MovieMetadataID:     movieI32Ptr(r.MovieMetadataID),
			LastSearchTime:      movieTimePtr(r.LastSearchTime),
		},
		State:            storage.MovieState(r.ToState),
		DownloadID:       r.DownloadID.String,
		DownloadClientID: int32(r.DownloadClientID.Int64),
	}
}

func movieFromMetaJoinRow(r movieMetaJoinRow) storage.Movie {
	return storage.Movie{
		Movie: model.Movie{
			ID:                  int32(r.ID),
			Path:                movieStrPtr(r.Path),
			Monitored:           int32(r.Monitored),
			QualityProfileID:    int32(r.QualityProfileID),
			Added:               movieTimePtr(r.Added),
			Tags:                movieStrPtr(r.Tags),
			AddOptions:          movieStrPtr(r.AddOptions),
			MovieFileID:         movieI32Ptr(r.MovieFileID),
			MinimumAvailability: int32(r.MinimumAvailability),
			MovieMetadataID:     movieI32Ptr(r.MovieMetadataID),
			LastSearchTime:      movieTimePtr(r.LastSearchTime),
		},
		State: storage.MovieState(r.ToState),
	}
}

// createMovieRow inserts the movie row, leaving the added column out so the
// database default timestamp applies when the caller did not provide one.
func (s *SQLite) createMovieRow(ctx context.Context, q *sqlcdb.Queries, m model.Movie) (int64, error) {
	if m.Added == nil || m.Added.IsZero() {
		return q.CreateMovie(ctx, movieCreateParams(m))
	}
	return q.CreateMovieWithAdded(ctx, movieCreateWithAddedParams(m))
}

// CreateMovie stores a movie and creates an initial transition state
func (s *SQLite) CreateMovie(ctx context.Context, movie storage.Movie, initialState storage.MovieState) (int64, error) {
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
	q := s.txQueries(tx)

	inserted, err := s.createMovieRow(ctx, q, movie.Movie)
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

	if err := q.CreateMovieTransition(ctx, movieTransitionParams(state)); err != nil {
		tx.Rollback()
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		tx.Rollback()
		return 0, err
	}

	return inserted, nil
}

func (s *SQLite) GetMovie(ctx context.Context, id int64) (*storage.Movie, error) {
	row, err := s.Queries.GetMovieByID(ctx, id)
	if err != nil {
		return nil, err
	}

	movie := movieFromJoinRow(movieJoinRow(row))
	return &movie, nil
}

func (s *SQLite) GetMovieByMovieFileID(ctx context.Context, fileID int64) (*storage.Movie, error) {
	row, err := s.Queries.GetMovieByFileID(ctx, sql.NullInt64{Int64: fileID, Valid: true})
	if err != nil {
		return nil, err
	}

	movie := movieFromJoinRow(movieJoinRow(row))
	return &movie, nil
}

// ListMovies lists the stored movies
func (s *SQLite) ListMovies(ctx context.Context) ([]*storage.Movie, error) {
	rows, err := s.Queries.ListMovies(ctx)
	if err != nil {
		return nil, err
	}

	movies := make([]*storage.Movie, 0, len(rows))
	for _, r := range rows {
		movie := movieFromJoinRow(movieJoinRow(r))
		movies = append(movies, &movie)
	}

	return movies, nil
}

func (s *SQLite) ListMoviesByState(ctx context.Context, state storage.MovieState) ([]*storage.Movie, error) {
	rows, err := s.Queries.ListMoviesByState(ctx, string(state))
	if err != nil {
		return nil, err
	}

	movies := make([]*storage.Movie, 0, len(rows))
	for _, r := range rows {
		movie := movieFromJoinRow(movieJoinRow(r))
		movies = append(movies, &movie)
	}

	return movies, nil
}

// DeleteMovie removes a movie by id
func (s *SQLite) DeleteMovie(ctx context.Context, id int64) error {
	return s.Queries.DeleteMovie(ctx, id)
}

// UpdateMovieMovieFileID updates the movie file id for a movie
func (s *SQLite) UpdateMovieMovieFileID(ctx context.Context, id int64, fileID int64) error {
	return s.Queries.UpdateMovieMovieFileIDQuery(ctx, sqlcdb.UpdateMovieMovieFileIDQueryParams{
		MovieFileID: sql.NullInt64{Int64: fileID, Valid: true},
		ID:          id,
	})
}

// UpdateMovieByID updates fields on a movie identified by id.
// Legacy parity: the jet-era implementation only ever wrote the monitored
// column, so only monitored is persisted here as well.
func (s *SQLite) UpdateMovieByID(ctx context.Context, id int64, movie model.Movie) error {
	return s.Queries.UpdateMovieMonitored(ctx, sqlcdb.UpdateMovieMonitoredParams{
		Monitored: int64(movie.Monitored),
		ID:        id,
	})
}

// UpdateMovieQualityProfile updates only the quality profile id for a movie
func (s *SQLite) UpdateMovieQualityProfile(ctx context.Context, id int64, qualityProfileID int32) error {
	return s.Queries.UpdateMovieQualityProfileID(ctx, sqlcdb.UpdateMovieQualityProfileIDParams{
		QualityProfileID: int64(qualityProfileID),
		ID:               id,
	})
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

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	q := s.txQueries(tx)

	previous, err := q.DeactivateMovieTransition(ctx, sqlcdb.DeactivateMovieTransitionParams{
		UpdatedAt: time.Now().Format(timestampFormat),
		MovieID:   id,
	})
	if err != nil {
		tx.Rollback()
		return err
	}

	transition := storage.MovieTransition{
		MovieID:    int32(id),
		ToState:    string(state),
		MostRecent: true,
		SortKey:    int32(previous.SortKey) + 1,
	}

	if metadata != nil {
		if metadata.DownloadClientID != nil && metadata.DownloadID != nil {
			transition.DownloadClientID = metadata.DownloadClientID
			transition.DownloadID = metadata.DownloadID
		}
	}

	if err := q.CreateMovieTransition(ctx, movieTransitionParams(transition)); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

// GetMovieByMetadataID checks if there's a movie already associated with the given metadata id
func (s *SQLite) GetMovieByMetadataID(ctx context.Context, metadataID int) (*storage.Movie, error) {
	row, err := s.Queries.GetMovieByMetadataIDQuery(ctx, sql.NullInt64{Int64: int64(metadataID), Valid: true})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("failed to lookup movie: %w", err)
	}

	movie := movieFromMetaJoinRow(movieMetaJoinRow(row))
	return &movie, nil
}

// GetMovieByPath gets a movie by path
func (s *SQLite) GetMovieByPath(ctx context.Context, path string) (*storage.Movie, error) {
	row, err := s.Queries.GetMovieByPathQuery(ctx, sql.NullString{String: path, Valid: true})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, err
	}

	movie := movieFromJoinRow(movieJoinRow(row))
	return &movie, nil
}

// LinkMovieMetadata links a movie with its metadata
func (s *SQLite) LinkMovieMetadata(ctx context.Context, movieID int64, metadataID int32) error {
	return s.Queries.LinkMovieMetadata(ctx, sqlcdb.LinkMovieMetadataParams{
		MovieMetadataID: sql.NullInt64{Int64: int64(metadataID), Valid: true},
		ID:              movieID,
	})
}
