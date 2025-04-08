package manager

import (
	"context"
	"errors"
	"time"

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

// GetSeriesMetadata gets all metadata around a series, its seasons, and its episodes.
func (m MediaManager) GetSeriesMetadata(ctx context.Context, tmdbID int) (*storage.SeriesMetadata, error) {
	metadata, err := m.storage.GetSeriesMetadata(ctx, table.SeriesMetadata.TmdbID.EQ(sqlite.Int64(int64(tmdbID))))
	if err == nil {
		return metadata, nil
	}

	// anything other than a not found error is an internal error
	if !errors.Is(err, storage.ErrNotFound) {
		return nil, err
	}

	// load the metadata from tmdb if we dont have it
	return m.loadSeriesMetadata(ctx, tmdbID)
}

func (m MediaManager) loadSeriesMetadata(ctx context.Context, tmdbID int) (*storage.SeriesMetadata, error) {
	det, err := m.tmdb.GetSeriesDetails(ctx, tmdbID)
	if err != nil {
		return nil, err
	}

	return FromSeriesDetails(*det)
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
	releaseDate, err := parseTMDBDate(*det.ReleaseDate)
	if err != nil {
		return model.MovieMetadata{}, err
	}
	return model.MovieMetadata{
		TmdbID:        int32(det.ID),
		ImdbID:        det.ImdbID,
		Title:         *det.Title,
		OriginalTitle: det.OriginalTitle,
		Runtime:       int32(*det.Runtime),
		Overview:      det.Overview,
		ReleaseDate:   releaseDate,
	}, nil
}

func FromSeriesDetails(det tmdb.SeriesDetails) (*storage.SeriesMetadata, error) {
	seriesMetadata := model.SeriesMetadata{
		TmdbID:       int32(det.ID),
		Title:        det.Name,
		SeasonCount:  int32(det.NumberOfSeasons),
		EpisodeCount: int32(det.NumberOfEpisodes),
	}

	if det.FirstAirDate != "" {
		airDate, err := parseTMDBDate(det.FirstAirDate)
		if err != nil {
			return nil, err
		}

		seriesMetadata.FirstAirDate = airDate
	}

	var seasons []storage.SeasonMetadata
	for _, tmdbSeason := range det.Seasons {
		airDate, err := parseTMDBDate(tmdbSeason.AirDate)
		if err != nil {
			return nil, err
		}
		runtime := int32(tmdbSeason.Runtime)
		seasonMetadata := storage.SeasonMetadata{
			SeasonMetadata: model.SeasonMetadata{
				TmdbID:   int32(tmdbSeason.ID),
				Title:    tmdbSeason.Name,
				AirDate:  airDate,
				Number:   int32(tmdbSeason.SeasonNumber),
				Runtime:  &runtime,
				Overview: &tmdbSeason.Overview,
			},
		}

		for _, episode := range tmdbSeason.Episodes {
			airDate, err := parseTMDBDate(episode.AirDate)
			if err != nil {
				return nil, err
			}

			runtime := int32(episode.Runtime)
			episodeMetadata := model.EpisodeMetadata{
				TmdbID:   int32(episode.ID),
				Title:    episode.Name,
				AirDate:  airDate,
				Number:   int32(episode.EpisodeNumber),
				Runtime:  &runtime,
				Overview: &episode.Overview,
			}

			seasonMetadata.Episodes = append(seasonMetadata.Episodes, episodeMetadata)
		}

		seasons = append(seasons, seasonMetadata)
	}

	return &storage.SeriesMetadata{
		SeriesMetadata: seriesMetadata,
		SeasonMetadata: seasons,
	}, nil
}

func parseTMDBDate(date string) (*time.Time, error) {
	t, err := time.Parse(tmdb.ReleaseDateFormat, date)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
