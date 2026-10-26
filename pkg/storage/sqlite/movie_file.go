package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

// mfFromRow converts a generated movie file row into the domain model.
func mfFromRow(r sqlcdb.MovieFile) model.MovieFile {
	return model.MovieFile{
		ID:               int32(r.ID),
		Quality:          r.Quality,
		Size:             r.Size,
		DateAdded:        r.DateAdded,
		SceneName:        movieStrPtr(r.SceneName),
		MediaInfo:        movieStrPtr(r.MediaInfo),
		ReleaseGroup:     movieStrPtr(r.ReleaseGroup),
		RelativePath:     movieStrPtr(r.RelativePath),
		Edition:          movieStrPtr(r.Edition),
		Languages:        r.Languages,
		IndexerFlags:     int32(r.IndexerFlags),
		OriginalFilePath: movieStrPtr(r.OriginalFilePath),
	}
}

// GetMovieFilesByMovieName gets all movie files given a movie name
// It assumes the movie name is the prefix of the relative path for a movie file
func (s *SQLite) GetMovieFilesByMovieName(ctx context.Context, name string) ([]*model.MovieFile, error) {
	rows, err := s.Queries.GetMovieFilesByMovieName(ctx, sql.NullString{String: name, Valid: true})
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		return nil, storage.ErrNotFound
	}

	files := make([]*model.MovieFile, 0, len(rows))
	for _, r := range rows {
		file := mfFromRow(r)
		files = append(files, &file)
	}

	return files, nil
}

// CreateMovieFile stores a movie file. The id and date_added columns are
// omitted from the insert so the autoincrement key and the database default
// timestamp are used.
func (s *SQLite) CreateMovieFile(ctx context.Context, file model.MovieFile) (int64, error) {
	inserted, err := s.Queries.CreateMovieFile(ctx, sqlcdb.CreateMovieFileParams{
		Quality:          file.Quality,
		Size:             file.Size,
		SceneName:        movieNullStr(file.SceneName),
		MediaInfo:        movieNullStr(file.MediaInfo),
		ReleaseGroup:     movieNullStr(file.ReleaseGroup),
		RelativePath:     movieNullStr(file.RelativePath),
		Edition:          movieNullStr(file.Edition),
		Languages:        file.Languages,
		IndexerFlags:     int64(file.IndexerFlags),
		OriginalFilePath: movieNullStr(file.OriginalFilePath),
	})
	if err != nil {
		return 0, err
	}

	return inserted, nil
}

// DeleteMovieFile removes a movie file by id
func (s *SQLite) DeleteMovieFile(ctx context.Context, id int64) error {
	return s.Queries.DeleteMovieFile(ctx, id)
}

// ListMovieFiles lists all movie files
func (s *SQLite) ListMovieFiles(ctx context.Context) ([]*model.MovieFile, error) {
	rows, err := s.Queries.ListMovieFiles(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list movie files: %w", err)
	}

	files := make([]*model.MovieFile, 0, len(rows))
	for _, r := range rows {
		file := mfFromRow(r)
		files = append(files, &file)
	}

	return files, nil
}
