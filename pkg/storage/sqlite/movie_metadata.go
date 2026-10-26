package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

// mmCreateParams builds the movie metadata insert params. The id column is
// excluded so the autoincrement key is used.
func mmCreateParams(m model.MovieMetadata) sqlcdb.CreateMovieMetadataParams {
	return sqlcdb.CreateMovieMetadataParams{
		TmdbID:             int64(m.TmdbID),
		ImdbID:             movieNullStr(m.ImdbID),
		Images:             m.Images,
		Genres:             movieNullStr(m.Genres),
		Title:              m.Title,
		SortTitle:          movieNullStr(m.SortTitle),
		CleanTitle:         movieNullStr(m.CleanTitle),
		OriginalTitle:      movieNullStr(m.OriginalTitle),
		CleanOriginalTitle: movieNullStr(m.CleanOriginalTitle),
		OriginalLanguage:   int64(m.OriginalLanguage),
		Status:             int64(m.Status),
		LastInfoSync:       movieNullTime(m.LastInfoSync),
		Runtime:            int64(m.Runtime),
		InCinemas:          movieNullTime(m.InCinemas),
		ReleaseDate:        movieNullTime(m.ReleaseDate),
		Year:               movieNullI64(m.Year),
		SecondaryYear:      movieNullI64(m.SecondaryYear),
		Ratings:            movieNullStr(m.Ratings),
		Recommendations:    m.Recommendations,
		Certification:      movieNullStr(m.Certification),
		YoutubeTrailerID:   movieNullStr(m.YoutubeTrailerID),
		Studio:             movieNullStr(m.Studio),
		Overview:           movieNullStr(m.Overview),
		Website:            movieNullStr(m.Website),
		Popularity:         movieNullF64(m.Popularity),
		CollectionTmdbID:   movieNullI64(m.CollectionTmdbID),
		CollectionTitle:    movieNullStr(m.CollectionTitle),
	}
}

// mmUpdateParams builds the movie metadata update params. Legacy parity: the
// id, tmdb_id and last_info_sync columns are never updated.
func mmUpdateParams(m model.MovieMetadata) sqlcdb.UpdateMovieMetadataQueryParams {
	return sqlcdb.UpdateMovieMetadataQueryParams{
		ImdbID:             movieNullStr(m.ImdbID),
		Images:             m.Images,
		Genres:             movieNullStr(m.Genres),
		Title:              m.Title,
		SortTitle:          movieNullStr(m.SortTitle),
		CleanTitle:         movieNullStr(m.CleanTitle),
		OriginalTitle:      movieNullStr(m.OriginalTitle),
		CleanOriginalTitle: movieNullStr(m.CleanOriginalTitle),
		OriginalLanguage:   int64(m.OriginalLanguage),
		Status:             int64(m.Status),
		Runtime:            int64(m.Runtime),
		InCinemas:          movieNullTime(m.InCinemas),
		ReleaseDate:        movieNullTime(m.ReleaseDate),
		Year:               movieNullI64(m.Year),
		SecondaryYear:      movieNullI64(m.SecondaryYear),
		Ratings:            movieNullStr(m.Ratings),
		Recommendations:    m.Recommendations,
		Certification:      movieNullStr(m.Certification),
		YoutubeTrailerID:   movieNullStr(m.YoutubeTrailerID),
		Studio:             movieNullStr(m.Studio),
		Overview:           movieNullStr(m.Overview),
		Website:            movieNullStr(m.Website),
		Popularity:         movieNullF64(m.Popularity),
		CollectionTmdbID:   movieNullI64(m.CollectionTmdbID),
		CollectionTitle:    movieNullStr(m.CollectionTitle),
		ID:                 int64(m.ID),
	}
}

// mmRow mirrors the generated movie metadata row shape so conversions do not
// depend on the generated struct's name (which sqlc may singularize to
// MovieMetadatum or align to MovieMetadata via config). Struct conversion keeps
// this file working either way as long as the shape is unchanged.
type mmRow struct {
	ID                 int64
	TmdbID             int64
	ImdbID             sql.NullString
	Images             string
	Genres             sql.NullString
	Title              string
	SortTitle          sql.NullString
	CleanTitle         sql.NullString
	OriginalTitle      sql.NullString
	CleanOriginalTitle sql.NullString
	OriginalLanguage   int64
	Status             int64
	LastInfoSync       sql.NullTime
	Runtime            int64
	InCinemas          sql.NullTime
	ReleaseDate        sql.NullTime
	Year               sql.NullInt64
	SecondaryYear      sql.NullInt64
	Ratings            sql.NullString
	Recommendations    string
	Certification      sql.NullString
	YoutubeTrailerID   sql.NullString
	Studio             sql.NullString
	Overview           sql.NullString
	Website            sql.NullString
	Popularity         sql.NullFloat64
	CollectionTmdbID   sql.NullInt64
	CollectionTitle    sql.NullString
}

// mmFromRow converts a generated movie metadata row into the domain model.
func mmFromRow(r mmRow) model.MovieMetadata {
	return model.MovieMetadata{
		ID:                 int32(r.ID),
		TmdbID:             int32(r.TmdbID),
		ImdbID:             movieStrPtr(r.ImdbID),
		Images:             r.Images,
		Genres:             movieStrPtr(r.Genres),
		Title:              r.Title,
		SortTitle:          movieStrPtr(r.SortTitle),
		CleanTitle:         movieStrPtr(r.CleanTitle),
		OriginalTitle:      movieStrPtr(r.OriginalTitle),
		CleanOriginalTitle: movieStrPtr(r.CleanOriginalTitle),
		OriginalLanguage:   int32(r.OriginalLanguage),
		Status:             int32(r.Status),
		LastInfoSync:       movieTimePtr(r.LastInfoSync),
		Runtime:            int32(r.Runtime),
		InCinemas:          movieTimePtr(r.InCinemas),
		ReleaseDate:        movieTimePtr(r.ReleaseDate),
		Year:               movieI32Ptr(r.Year),
		SecondaryYear:      movieI32Ptr(r.SecondaryYear),
		Ratings:            movieStrPtr(r.Ratings),
		Recommendations:    r.Recommendations,
		Certification:      movieStrPtr(r.Certification),
		YoutubeTrailerID:   movieStrPtr(r.YoutubeTrailerID),
		Studio:             movieStrPtr(r.Studio),
		Overview:           movieStrPtr(r.Overview),
		Website:            movieStrPtr(r.Website),
		Popularity:         movieF64Ptr(r.Popularity),
		CollectionTmdbID:   movieI32Ptr(r.CollectionTmdbID),
		CollectionTitle:    movieStrPtr(r.CollectionTitle),
	}
}

// CreateMovieMetadata creates the given movieMeta
func (s *SQLite) CreateMovieMetadata(ctx context.Context, movieMeta model.MovieMetadata) (int64, error) {
	result, err := s.Queries.CreateMovieMetadata(ctx, mmCreateParams(movieMeta))
	if err != nil {
		return 0, err
	}

	inserted, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return inserted, nil
}

// UpdateMovieMetadata updates an existing movie metadata record
func (s *SQLite) UpdateMovieMetadata(ctx context.Context, metadata model.MovieMetadata) error {
	err := s.Queries.UpdateMovieMetadataQuery(ctx, mmUpdateParams(metadata))
	if err != nil {
		return fmt.Errorf("failed to update movie metadata: %w", err)
	}

	return nil
}

// DeleteMovieMetadata deletes a movie metadata by id
func (s *SQLite) DeleteMovieMetadata(ctx context.Context, id int64) error {
	return s.Queries.DeleteMovieMetadata(ctx, id)
}

// ListMovieMetadata lists all movie metadata
func (s *SQLite) ListMovieMetadata(ctx context.Context) ([]*model.MovieMetadata, error) {
	rows, err := s.Queries.ListMovieMetadata(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list movies: %w", err)
	}

	metadata := make([]*model.MovieMetadata, 0, len(rows))
	for _, r := range rows {
		meta := mmFromRow(mmRow(r))
		metadata = append(metadata, &meta)
	}

	return metadata, nil
}

// GetMovieMetadataByID gets a movie metadata by its id
func (s *SQLite) GetMovieMetadataByID(ctx context.Context, id int64) (*model.MovieMetadata, error) {
	row, err := s.Queries.GetMovieMetadataByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("failed to list movies: %w", err)
	}

	meta := mmFromRow(mmRow(row))
	return &meta, nil
}

// GetMovieMetadataByTmdbID gets a movie metadata by its tmdb id
func (s *SQLite) GetMovieMetadataByTmdbID(ctx context.Context, tmdbID int64) (*model.MovieMetadata, error) {
	row, err := s.Queries.GetMovieMetadataByTmdbID(ctx, tmdbID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("failed to list movies: %w", err)
	}

	meta := mmFromRow(mmRow(row))
	return &meta, nil
}
