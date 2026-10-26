package sqlite

import (
	"context"
	"fmt"

	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

// CreateMovieMetadata creates the given movieMeta, returning the new id.
func (s *SQLite) CreateMovieMetadata(ctx context.Context, movieMeta sqlcdb.MovieMetadatum) (int64, error) {
	inserted, err := s.Queries.CreateMovieMetadata(ctx, sqlcdb.CreateMovieMetadataParams{
		TmdbID:             movieMeta.TmdbID,
		ImdbID:             movieMeta.ImdbID,
		Images:             movieMeta.Images,
		Genres:             movieMeta.Genres,
		Title:              movieMeta.Title,
		SortTitle:          movieMeta.SortTitle,
		CleanTitle:         movieMeta.CleanTitle,
		OriginalTitle:      movieMeta.OriginalTitle,
		CleanOriginalTitle: movieMeta.CleanOriginalTitle,
		OriginalLanguage:   movieMeta.OriginalLanguage,
		Status:             movieMeta.Status,
		LastInfoSync:       movieMeta.LastInfoSync,
		Runtime:            movieMeta.Runtime,
		InCinemas:          movieMeta.InCinemas,
		ReleaseDate:        movieMeta.ReleaseDate,
		Year:               movieMeta.Year,
		SecondaryYear:      movieMeta.SecondaryYear,
		Ratings:            movieMeta.Ratings,
		Recommendations:    movieMeta.Recommendations,
		Certification:      movieMeta.Certification,
		YoutubeTrailerID:   movieMeta.YoutubeTrailerID,
		Studio:             movieMeta.Studio,
		Overview:           movieMeta.Overview,
		Website:            movieMeta.Website,
		Popularity:         movieMeta.Popularity,
		CollectionTmdbID:   movieMeta.CollectionTmdbID,
		CollectionTitle:    movieMeta.CollectionTitle,
	})
	if err != nil {
		return 0, err
	}
	return inserted, nil
}

// UpdateMovieMetadata refreshes mutable metadata and stamps last_info_sync in UTC.
// The persistent id and external tmdb_id remain stable.
func (s *SQLite) UpdateMovieMetadata(ctx context.Context, metadata sqlcdb.MovieMetadatum) error {
	err := s.Queries.UpdateMovieMetadataQuery(ctx, sqlcdb.UpdateMovieMetadataQueryParams{
		ImdbID:             metadata.ImdbID,
		Images:             metadata.Images,
		Genres:             metadata.Genres,
		Title:              metadata.Title,
		SortTitle:          metadata.SortTitle,
		CleanTitle:         metadata.CleanTitle,
		OriginalTitle:      metadata.OriginalTitle,
		CleanOriginalTitle: metadata.CleanOriginalTitle,
		OriginalLanguage:   metadata.OriginalLanguage,
		Status:             metadata.Status,
		Runtime:            metadata.Runtime,
		InCinemas:          metadata.InCinemas,
		ReleaseDate:        metadata.ReleaseDate,
		Year:               metadata.Year,
		SecondaryYear:      metadata.SecondaryYear,
		Ratings:            metadata.Ratings,
		Recommendations:    metadata.Recommendations,
		Certification:      metadata.Certification,
		YoutubeTrailerID:   metadata.YoutubeTrailerID,
		Studio:             metadata.Studio,
		Overview:           metadata.Overview,
		Website:            metadata.Website,
		Popularity:         metadata.Popularity,
		CollectionTmdbID:   metadata.CollectionTmdbID,
		CollectionTitle:    metadata.CollectionTitle,
		ID:                 metadata.ID,
	})
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
func (s *SQLite) ListMovieMetadata(ctx context.Context) ([]sqlcdb.MovieMetadatum, error) {
	rows, err := s.Queries.ListMovieMetadata(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list movie metadata: %w", err)
	}
	return rows, nil
}

// GetMovieMetadataByID gets a movie metadata by its id
func (s *SQLite) GetMovieMetadataByID(ctx context.Context, id int64) (sqlcdb.MovieMetadatum, error) {
	row, err := s.Queries.GetMovieMetadataByID(ctx, id)
	if err != nil {
		return sqlcdb.MovieMetadatum{}, mapQueryErr(err, "failed to get movie metadata")
	}
	return row, nil
}

// GetMovieMetadataByTmdbID gets a movie metadata by its tmdb id
func (s *SQLite) GetMovieMetadataByTmdbID(ctx context.Context, tmdbID int64) (sqlcdb.MovieMetadatum, error) {
	row, err := s.Queries.GetMovieMetadataByTmdbID(ctx, tmdbID)
	if err != nil {
		return sqlcdb.MovieMetadatum{}, mapQueryErr(err, "failed to get movie metadata")
	}
	return row, nil
}
