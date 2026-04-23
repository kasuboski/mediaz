package manager

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
	"github.com/kasuboski/mediaz/pkg/tmdb"
	"github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

// MetadataService owns TMDB interactions, metadata enrichment, and external data syncing.
type MetadataService struct {
	tmdb              tmdb.ITmdb
	movieMetaStorage  storage.MovieMetadataStorage
	seriesMetaStorage storage.SeriesMetadataStorage
}

// NewMetadataService creates a MetadataService with the given dependencies.
func NewMetadataService(tmbdClient tmdb.ITmdb, movieMetaStorage storage.MovieMetadataStorage, seriesMetaStorage storage.SeriesMetadataStorage) MetadataService {
	return MetadataService{
		tmdb:              tmbdClient,
		movieMetaStorage:  movieMetaStorage,
		seriesMetaStorage: seriesMetaStorage,
	}
}

func (ms MetadataService) GetMovieMetadata(ctx context.Context, tmdbID int) (*model.MovieMetadata, error) {
	res, err := ms.movieMetaStorage.GetMovieMetadata(ctx, table.MovieMetadata.TmdbID.EQ(sqlite.Int(int64(tmdbID))))
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			return nil, err
		}
		res, err = ms.loadMovieMetadata(ctx, tmdbID)
		if err != nil {
			return nil, err
		}
	}
	return res, err
}

func (ms MetadataService) UpdateMovieMetadataFromTMDB(ctx context.Context, tmdbID int) (*model.MovieMetadata, error) {
	log := logger.FromCtx(ctx)

	det, err := ms.tmdb.GetMovieDetails(ctx, tmdbID)
	if err != nil {
		log.Error("failed to get movie details", zap.Error(err))
		return nil, err
	}

	existing, err := ms.movieMetaStorage.GetMovieMetadata(ctx, table.MovieMetadata.TmdbID.EQ(sqlite.Int(int64(tmdbID))))
	if err != nil {
		log.Error("failed to get existing movie metadata", zap.Error(err))
		return nil, err
	}

	updated := FromMediaDetails(*det)
	updated.ID = existing.ID
	updated.TmdbID = existing.TmdbID

	err = ms.movieMetaStorage.UpdateMovieMetadata(ctx, updated)
	if err != nil {
		log.Error("failed to update movie metadata", zap.Error(err))
		return nil, err
	}

	return &updated, nil
}

func (ms MetadataService) loadMovieMetadata(ctx context.Context, tmdbID int) (*model.MovieMetadata, error) {
	det, err := ms.tmdb.GetMovieDetails(ctx, tmdbID)
	if err != nil {
		return nil, err
	}

	metadata := FromMediaDetails(*det)

	id, err := ms.movieMetaStorage.CreateMovieMetadata(ctx, metadata)
	if err != nil {
		return nil, err
	}
	metadata.ID = int32(id)
	return &metadata, nil
}

func (ms MetadataService) GetMovieMetadataByID(ctx context.Context, metadataID int32) (*model.MovieMetadata, error) {
	return ms.movieMetaStorage.GetMovieMetadata(ctx, table.MovieMetadata.ID.EQ(sqlite.Int(int64(metadataID))))
}

// GetSeriesMetadata gets all metadata around a series. If it does not exist, it will be created including seasons and episodes.
func (ms MetadataService) GetSeriesMetadata(ctx context.Context, tmdbID int) (*model.SeriesMetadata, error) {
	metadata, err := ms.seriesMetaStorage.GetSeriesMetadata(ctx, table.SeriesMetadata.TmdbID.EQ(sqlite.Int64(int64(tmdbID))))
	if err == nil {
		return metadata, nil
	}
	if !errors.Is(err, storage.ErrNotFound) {
		return nil, err
	}

	return ms.loadSeriesMetadata(ctx, tmdbID)
}

// RefreshSeriesMetadataFromTMDB refreshes series metadata with proper entity linking.
func (ms MetadataService) RefreshSeriesMetadataFromTMDB(ctx context.Context, tmdbID int) (*model.SeriesMetadata, error) {
	return ms.loadSeriesMetadata(ctx, tmdbID)
}

func (ms MetadataService) UpdateSeriesMetadataFromTMDB(ctx context.Context, tmdbID int) (*model.SeriesMetadata, error) {
	log := logger.FromCtx(ctx)

	details, err := ms.tmdb.GetSeriesDetails(ctx, tmdbID)
	if err != nil {
		log.Error("failed to get series details", zap.Error(err))
		return nil, err
	}

	existing, err := ms.seriesMetaStorage.GetSeriesMetadata(ctx, table.SeriesMetadata.TmdbID.EQ(sqlite.Int64(int64(tmdbID))))
	if err != nil {
		log.Error("failed to get existing series metadata", zap.Error(err))
		return nil, err
	}

	updated, err := FromSeriesDetails(*details)
	if err != nil {
		log.Error("failed to parse series details", zap.Error(err))
		return nil, err
	}

	updated.ID = existing.ID
	updated.TmdbID = existing.TmdbID

	if extIDs, err := ms.fetchExternalIDs(ctx, tmdbID); err == nil && extIDs != nil {
		updated.ExternalIds = extIDs
	}

	if watchProviders, err := ms.fetchWatchProviders(ctx, tmdbID); err == nil && watchProviders != nil {
		updated.WatchProviders = watchProviders
	}

	err = ms.seriesMetaStorage.UpdateSeriesMetadata(ctx, updated)
	if err != nil {
		log.Error("failed to update series metadata", zap.Error(err))
		return nil, err
	}

	return &updated, nil
}

func (ms MetadataService) loadSeriesMetadata(ctx context.Context, tmdbID int) (*model.SeriesMetadata, error) {
	log := logger.FromCtx(ctx)
	details, err := ms.tmdb.GetSeriesDetails(ctx, tmdbID)
	if err != nil {
		log.Error("failed to get series details", zap.Error(err))
		return nil, err
	}

	series, err := FromSeriesDetails(*details)
	if err != nil {
		log.Error("failed to parse series details", zap.Error(err))
		return nil, err
	}

	if extIDs, err := ms.fetchExternalIDs(ctx, tmdbID); err == nil && extIDs != nil {
		series.ExternalIds = extIDs
	}

	if watchProviders, err := ms.fetchWatchProviders(ctx, tmdbID); err == nil && watchProviders != nil {
		series.WatchProviders = watchProviders
	}

	seriesMetadataID, err := ms.seriesMetaStorage.CreateSeriesMetadata(ctx, series)
	if err != nil {
		var sqliteErr sqlite3.Error
		if !errors.As(err, &sqliteErr) || sqliteErr.Code != sqlite3.ErrConstraint || sqliteErr.ExtendedCode != sqlite3.ErrConstraintUnique {
			log.Error("failed to create series metadata", zap.Error(err))
			return nil, err
		}

		existing, getErr := ms.seriesMetaStorage.GetSeriesMetadata(ctx, table.SeriesMetadata.TmdbID.EQ(sqlite.Int64(int64(series.TmdbID))))
		if getErr != nil {
			log.Error("failed to get existing series metadata", zap.NamedError("createErr", err), zap.NamedError("getErr", getErr))
			return nil, getErr
		}

		series.ID = existing.ID
		series.TmdbID = existing.TmdbID
		if updateErr := ms.seriesMetaStorage.UpdateSeriesMetadata(ctx, series); updateErr != nil {
			log.Error("failed to update existing series metadata", zap.Error(updateErr))
			return nil, updateErr
		}
		seriesMetadataID = int64(existing.ID)
	}

	for _, s := range details.Seasons {
		season := FromSeriesSeasons(s)
		season.SeriesMetadataID = int32(seriesMetadataID)

		existingSeason, err := ms.seriesMetaStorage.GetSeasonMetadata(ctx, table.SeasonMetadata.TmdbID.EQ(sqlite.Int(int64(season.TmdbID))))
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			log.Error("failed to get existing season metadata", zap.Error(err))
			return nil, err
		}

		seasonMetadataID := int64(0)
		if existingSeason != nil {
			seasonMetadataID = int64(existingSeason.ID)
		}
		if seasonMetadataID == 0 {
			seasonMetadataID, err = ms.seriesMetaStorage.CreateSeasonMetadata(ctx, season)
			if err != nil {
				log.Error("failed to create season metadata", zap.Error(err))
				return nil, err
			}
		}

		for _, episode := range s.Episodes {
			episodeMetadata := FromSeriesEpisodes(episode)
			episodeMetadata.SeasonMetadataID = int32(seasonMetadataID)

			_, err = ms.seriesMetaStorage.GetEpisodeMetadata(ctx, table.EpisodeMetadata.TmdbID.EQ(sqlite.Int(int64(episodeMetadata.TmdbID))))
			if err == nil {
				continue
			}
			if !errors.Is(err, storage.ErrNotFound) {
				log.Error("failed to get existing episode metadata", zap.Error(err))
				return nil, err
			}

			_, err = ms.seriesMetaStorage.CreateEpisodeMetadata(ctx, episodeMetadata)
			if err != nil {
				log.Error("failed to create episode metadata", zap.Error(err))
				return nil, err
			}
		}
	}

	return ms.seriesMetaStorage.GetSeriesMetadata(ctx, table.SeriesMetadata.ID.EQ(sqlite.Int(seriesMetadataID)))
}

func (ms MetadataService) fetchExternalIDs(ctx context.Context, tmdbID int) (*string, error) {
	log := logger.FromCtx(ctx)

	extIDsResp, err := ms.tmdb.TvSeriesExternalIds(ctx, int32(tmdbID))
	if err != nil {
		log.Debug("failed to fetch external IDs", zap.Error(err))
		return nil, nil
	}
	defer extIDsResp.Body.Close()

	if extIDsResp.StatusCode != 200 {
		return nil, nil
	}

	extIDsData, err := parseExternalIDs(extIDsResp)
	if err != nil {
		log.Debug("failed to parse external IDs", zap.Error(err))
		return nil, nil
	}

	serialized, err := SerializeExternalIDs(extIDsData)
	if err != nil {
		log.Debug("failed to serialize external IDs", zap.Error(err))
		return nil, nil
	}

	return serialized, nil
}

func (ms MetadataService) fetchWatchProviders(ctx context.Context, tmdbID int) (*string, error) {
	log := logger.FromCtx(ctx)

	wpResp, err := ms.tmdb.TvSeriesWatchProviders(ctx, int32(tmdbID))
	if err != nil {
		log.Debug("failed to fetch watch providers", zap.Error(err))
		return nil, nil
	}
	defer wpResp.Body.Close()

	if wpResp.StatusCode != 200 {
		return nil, nil
	}

	wpData, err := parseWatchProviders(wpResp)
	if err != nil {
		log.Debug("failed to parse watch providers", zap.Error(err))
		return nil, nil
	}

	serialized, err := SerializeWatchProviders(wpData)
	if err != nil {
		log.Debug("failed to serialize watch providers", zap.Error(err))
		return nil, nil
	}

	return serialized, nil
}

func (ms MetadataService) RefreshSeriesMetadata(ctx context.Context, tmdbIDs ...int) error {
	log := logger.FromCtx(ctx).With("ids", tmdbIDs)
	log.Debug("refreshing series metadata")

	var errs []error

	if len(tmdbIDs) == 0 {
		allSeries, err := ms.seriesMetaStorage.ListSeriesMetadata(ctx, table.SeriesMetadata.Status.EQ(sqlite.String("")))
		if err != nil {
			log.Error("failed to list series with empty status", zap.Error(err))
			return err
		}

		for _, series := range allSeries {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			_, err := ms.UpdateSeriesMetadataFromTMDB(ctx, int(series.TmdbID))
			if err != nil {
				log.Error("failed to refresh series metadata", zap.Int32("tmdb_id", series.TmdbID), zap.Error(err))
				errs = append(errs, err)
			}
		}

		return errors.Join(errs...)
	}

	for _, id := range tmdbIDs {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		_, err := ms.UpdateSeriesMetadataFromTMDB(ctx, id)
		if err != nil {
			log.Error("failed to refresh series metadata", zap.Int("tmdb_id", id), zap.Error(err))
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func (ms MetadataService) RefreshMovieMetadata(ctx context.Context, tmdbIDs ...int) error {
	log := logger.FromCtx(ctx)

	var errs []error

	if len(tmdbIDs) == 0 {
		allMovies, err := ms.movieMetaStorage.ListMovieMetadata(ctx)
		if err != nil {
			log.Error("failed to list movie metadata", zap.Error(err))
			return err
		}

		for _, movie := range allMovies {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			_, err := ms.UpdateMovieMetadataFromTMDB(ctx, int(movie.TmdbID))
			if err != nil {
				log.Error("failed to refresh movie metadata", zap.Int32("tmdb_id", movie.TmdbID), zap.Error(err))
				errs = append(errs, err)
			}
		}

		return errors.Join(errs...)
	}

	for _, id := range tmdbIDs {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		_, err := ms.UpdateMovieMetadataFromTMDB(ctx, id)
		if err != nil {
			log.Error("failed to refresh movie metadata", zap.Int("tmdb_id", id), zap.Error(err))
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
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
		Overview:     &series.Overview,
		Status:       series.Status,
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

	if episode.StillPath != "" {
		model.StillPath = &episode.StillPath
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

// parseExternalIDs parses TMDB external IDs response
func parseExternalIDs(resp *http.Response) (*ExternalIDsData, error) {
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result ExternalIDsData
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// parseWatchProviders parses TMDB watch providers response
func parseWatchProviders(resp *http.Response) (*WatchProvidersData, error) {
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	type tmdbProvider struct {
		ProviderID   *int    `json:"provider_id"`
		ProviderName *string `json:"provider_name"`
		LogoPath     *string `json:"logo_path"`
	}
	type tmdbRegion struct {
		Flatrate []tmdbProvider `json:"flatrate"`
		Link     *string        `json:"link"`
	}
	type tmdbRoot struct {
		Results map[string]tmdbRegion `json:"results"`
	}

	var tmdbData tmdbRoot
	if err := json.Unmarshal(b, &tmdbData); err != nil {
		return nil, err
	}

	result := &WatchProvidersData{}
	if us, ok := tmdbData.Results["US"]; ok {
		providers := make([]WatchProviderData, 0, len(us.Flatrate))
		for _, p := range us.Flatrate {
			if p.ProviderID != nil && p.ProviderName != nil {
				providers = append(providers, WatchProviderData{
					ProviderID: *p.ProviderID,
					Name:       *p.ProviderName,
					LogoPath:   p.LogoPath,
				})
			}
		}
		result.US = WatchProviderRegionData{
			Flatrate: providers,
			Link:     us.Link,
		}
	}
	return result, nil
}
