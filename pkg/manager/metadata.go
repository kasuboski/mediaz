package manager

import (
	"context"
	"errors"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/pkg/library"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
	"github.com/kasuboski/mediaz/pkg/tmdb"
)

// IndexMovies finds metadata for each movie in the library
func (m MediaManager) IndexMovies(ctx context.Context) error {
	log := logger.FromCtx(ctx)
	movies, err := m.ListMoviesInLibrary(ctx)
	if err != nil {
		return err
	}

	for _, mov := range movies {
		resp, err := m.SearchMovie(ctx, mov.Name)
		if err != nil {
			return err
		}
		if len(resp.Results) == 0 {
			log.Warn("no movie metadata", "name", mov.Name)
			continue
		}
		res := resp.Results[0]
		log.Debugw("metadata", "id", res.ID, "title", res.Title)
	}
	return nil
}

func (m MediaManager) GetMovieMetadata(ctx context.Context, tmdbID int) (*model.MovieMetadata, error) {
	res := &model.MovieMetadata{}
	res, err := m.storage.GetMovieMetadata(ctx, table.MovieMetadata.TmdbID.EQ(sqlite.Int(int64(tmdbID))))
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			return nil, err
		}
		res, err = m.loadMovieMetadata(ctx, tmdbID)
		if err != nil {
			return nil, err
		}

	}

	return res, err
}

func (m MediaManager) loadMovieMetadata(ctx context.Context, tmdbID int) (*model.MovieMetadata, error) {
	det, err := m.tmdb.GetMovieDetails(ctx, tmdbID)
	if err != nil {
		return nil, err
	}

	metadata, err := FromMediaDetails(*det)
	if err != nil {
		return nil, err
	}

	id, err := m.storage.CreateMovieMetadata(ctx, metadata)
	if err != nil {
		return nil, err
	}
	// sigh cast int id
	metadata.ID = int32(id)
	return &metadata, nil
}

func FromSearchMediaResult(resp SearchMediaResult) library.MovieMetadata {
	return library.MovieMetadata{
		TMDBID:   *resp.ID,
		Images:   *resp.PosterPath,
		Title:    *resp.Title,
		Overview: *resp.Overview,
	}
}

func FromMediaDetails(det tmdb.MediaDetails) (model.MovieMetadata, error) {
	return model.MovieMetadata{
		TmdbID:        int32(det.ID),
		ImdbID:        det.ImdbID,
		Title:         *det.Title,
		OriginalTitle: det.OriginalTitle,
		Runtime:       int32(*det.Runtime),
		Overview:      det.Overview,
	}, nil
}
