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

// GetSeriesMetadata gets all metadata around a series. If it does not exist, it will be created including seasons and episodes.
func (m MediaManager) GetSeriesMetadata(ctx context.Context, tmdbID int) (*model.SeriesMetadata, error) {
	metadata, err := m.storage.GetSeriesMetadata(ctx, table.SeriesMetadata.TmdbID.EQ(sqlite.Int64(int64(tmdbID))))
	if err == nil {
		return metadata, nil
	}
	if !errors.Is(err, storage.ErrNotFound) {
		return nil, err
	}

	// load the metadata from tmdb if we dont have it
	return m.loadSeriesMetadata(ctx, tmdbID)
}

func (m MediaManager) loadSeriesMetadata(ctx context.Context, tmdbID int) (*model.SeriesMetadata, error) {
	details, err := m.tmdb.GetSeriesDetails(ctx, tmdbID)
	if err != nil {
		return nil, err
	}

	series, err := FromSeriesDetails(*details)
	if err != nil {
		return nil, err
	}

	seriesMetadataID, err := m.storage.CreateSeriesMetadata(ctx, series)
	if err != nil {
		return nil, err
	}

	for _, s := range details.Seasons {
		season, err := FromSeriesSeasons(s)
		if err != nil {
			return nil, err
		}

		season.SeriesID = int32(seriesMetadataID)

		seasonMetadataID, err := m.storage.CreateSeasonMetadata(ctx, season)
		if err != nil {
			return nil, err
		}

		for _, episode := range s.Episodes {
			episodeMetadata, err := FromSeriesEpisodes(episode)
			if err != nil {
				return nil, err
			}

			episodeMetadata.SeasonID = int32(seasonMetadataID)
			_, err = m.storage.CreateEpisodeMetadata(ctx, episodeMetadata)
			if err != nil {
				return nil, err
			}
		}
	}

	return m.storage.GetSeriesMetadata(ctx, table.SeriesMetadata.ID.EQ(sqlite.Int(seriesMetadataID)))

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

func FromSeriesDetails(series tmdb.SeriesDetails) (model.SeriesMetadata, error) {
	airDate, err := parseTMDBDate(series.FirstAirDate)
	if err != nil {
		return model.SeriesMetadata{}, err
	}

	return model.SeriesMetadata{
		TmdbID:       int32(series.ID),
		Title:        series.Name,
		SeasonCount:  int32(series.NumberOfSeasons),
		EpisodeCount: int32(series.NumberOfEpisodes),
		FirstAirDate: airDate,
	}, nil

}

func FromSeriesSeasons(tmdbSeason tmdb.Season) (model.SeasonMetadata, error) {
	airDate, err := parseTMDBDate(tmdbSeason.AirDate)
	if err != nil {
		return model.SeasonMetadata{}, err
	}

	return model.SeasonMetadata{
		TmdbID:   int32(tmdbSeason.ID),
		Title:    tmdbSeason.Name,
		AirDate:  airDate,
		Number:   int32(tmdbSeason.SeasonNumber),
		Overview: &tmdbSeason.Overview,
	}, nil
}

func FromSeriesEpisodes(episode tmdb.Episode) (model.EpisodeMetadata, error) {
	airDate, err := parseTMDBDate(episode.AirDate)
	if err != nil {
		return model.EpisodeMetadata{}, err
	}

	runtime := int32(episode.Runtime)
	return model.EpisodeMetadata{
		TmdbID:   int32(episode.ID),
		Title:    episode.Name,
		AirDate:  airDate,
		Number:   int32(episode.EpisodeNumber),
		Runtime:  &runtime,
		Overview: &episode.Overview,
	}, nil
}

func parseTMDBDate(date string) (*time.Time, error) {
	t, err := time.Parse(tmdb.ReleaseDateFormat, date)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
