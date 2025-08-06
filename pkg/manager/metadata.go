package manager

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
	"github.com/kasuboski/mediaz/pkg/tmdb"
)

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

	metadata := FromMediaDetails(*det)

	id, err := m.storage.CreateMovieMetadata(ctx, metadata)
	if err != nil {
		return nil, err
	}
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

	return m.loadSeriesMetadata(ctx, tmdbID)
}

// RefreshSeriesMetadataFromTMDB always fetches fresh series metadata from TMDB, regardless of whether cached data exists.
// This is useful for discovering new episodes and seasons for continuing series.
func (m MediaManager) RefreshSeriesMetadataFromTMDB(ctx context.Context, tmdbID int) (*model.SeriesMetadata, error) {
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
		season := FromSeriesSeasons(s)
		season.SeriesID = int32(seriesMetadataID)

		existingSeason, err := m.storage.GetSeasonMetadata(ctx, table.SeasonMetadata.TmdbID.EQ(sqlite.Int(int64(season.TmdbID))))
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			return nil, err
		}

		seasonMetadataID := int64(0)
		if existingSeason != nil {
			seasonMetadataID = int64(existingSeason.ID)
		}
		if seasonMetadataID == 0 {
			seasonMetadataID, err = m.storage.CreateSeasonMetadata(ctx, season)
			if err != nil {
				return nil, err
			}
		}

		for _, episode := range s.Episodes {
			episodeMetadata := FromSeriesEpisodes(episode)
			episodeMetadata.SeasonID = int32(seasonMetadataID)

			_, err := m.storage.GetEpisodeMetadata(ctx, table.EpisodeMetadata.TmdbID.EQ(sqlite.Int(int64(episodeMetadata.TmdbID))))
			if err == nil {
				continue
			}
			if !errors.Is(err, storage.ErrNotFound) {
				return nil, err
			}

			_, err = m.storage.CreateEpisodeMetadata(ctx, episodeMetadata)
			if err != nil {
				return nil, err
			}
		}
	}

	return m.storage.GetSeriesMetadata(ctx, table.SeriesMetadata.ID.EQ(sqlite.Int(seriesMetadataID)))

}

func FromMediaDetails(det tmdb.MediaDetails) model.MovieMetadata {
	model := model.MovieMetadata{
		TmdbID:        int32(det.ID),
		ImdbID:        det.ImdbID,
		Images:        "",
		Title:         *det.Title,
		OriginalTitle: det.OriginalTitle,
		Runtime:       int32(*det.Runtime),
		Overview:      det.Overview,
	}
	if det.PosterPath != nil {
		model.Images = *det.PosterPath
	}

	if det.Genres != nil {
		names := []string{}
		for _, g := range *det.Genres {
			names = append(names, g.Name)
		}
		gs := strings.Join(names, ",")
		model.Genres = &gs
	}

	if det.Homepage != nil {
		model.Website = det.Homepage
	}

	if det.Popularity != nil {
		p := float64(*det.Popularity)
		model.Popularity = &p
	}

	if det.ProductionCompanies != nil && len(*det.ProductionCompanies) > 0 {
		studio := (*det.ProductionCompanies)[0].Name
		model.Studio = studio
	}

	if det.BelongsToCollection != nil {
		if collMap, ok := (*det.BelongsToCollection).(map[string]any); ok {
			if rawID, ok := collMap["id"].(float64); ok {
				id := int32(rawID)
				model.CollectionTmdbID = &id
			}
			if name, ok := collMap["name"].(string); ok {
				model.CollectionTitle = &name
			}
		}
	}

	if det.ReleaseDate != nil {
		releaseDate, err := parseTMDBDate(*det.ReleaseDate)
		if err == nil {
			model.ReleaseDate = releaseDate
		}

		if model.ReleaseDate != nil {
			y := int32(model.ReleaseDate.Year())
			model.Year = &y
		}
	}

	return model
}

func FromSeriesDetails(series tmdb.SeriesDetails) (model.SeriesMetadata, error) {
	airDate, err := parseTMDBDate(series.FirstAirDate)
	if err != nil {
		return model.SeriesMetadata{}, err
	}

	var poster *string
	if series.PosterPath != "" {
		p := series.PosterPath
		poster = &p
	}

	return model.SeriesMetadata{
		TmdbID:       int32(series.ID),
		Title:        series.Name,
		SeasonCount:  int32(series.NumberOfSeasons),
		EpisodeCount: int32(series.NumberOfEpisodes),
		FirstAirDate: airDate,
		PosterPath:   poster,
	}, nil

}

func FromSeriesSeasons(tmdbSeason tmdb.Season) model.SeasonMetadata {
	model := model.SeasonMetadata{
		TmdbID:   int32(tmdbSeason.ID),
		Title:    tmdbSeason.Name,
		Number:   int32(tmdbSeason.SeasonNumber),
		Overview: &tmdbSeason.Overview,
	}

	airDate, err := parseTMDBDate(tmdbSeason.AirDate)
	if err == nil {
		model.AirDate = airDate
	}

	return model
}

func FromSeriesEpisodes(episode tmdb.Episode) model.EpisodeMetadata {
	runtime := int32(episode.Runtime)
	model := model.EpisodeMetadata{
		TmdbID:   int32(episode.ID),
		Title:    episode.Name,
		Number:   int32(episode.EpisodeNumber),
		Runtime:  &runtime,
		Overview: &episode.Overview,
	}

	airDate, err := parseTMDBDate(episode.AirDate)
	if err == nil {
		model.AirDate = airDate
	}

	return model
}

func parseTMDBDate(date string) (*time.Time, error) {
	if date == "" {
		return nil, nil
	}
	t, err := time.Parse(tmdb.ReleaseDateFormat, date)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
