package manager

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/storage"
	sqlcdb "github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
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

func (ms MetadataService) GetMovieMetadata(ctx context.Context, tmdbID int64) (*sqlcdb.MovieMetadatum, error) {
	res, err := ms.movieMetaStorage.GetMovieMetadataByTmdbID(ctx, tmdbID)
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			return nil, err
		}
		return ms.loadMovieMetadata(ctx, tmdbID)
	}
	return &res, nil
}

func (ms MetadataService) UpdateMovieMetadataFromTMDB(ctx context.Context, tmdbID int64) (*sqlcdb.MovieMetadatum, error) {
	log := logger.FromCtx(ctx)

	det, err := ms.tmdb.GetMovieDetails(ctx, int(tmdbID))
	if err != nil {
		log.Error("failed to get movie details", zap.Error(err))
		return nil, err
	}

	existing, err := ms.movieMetaStorage.GetMovieMetadataByTmdbID(ctx, tmdbID)
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

func (ms MetadataService) loadMovieMetadata(ctx context.Context, tmdbID int64) (*sqlcdb.MovieMetadatum, error) {
	det, err := ms.tmdb.GetMovieDetails(ctx, int(tmdbID))
	if err != nil {
		return nil, err
	}

	metadata := FromMediaDetails(*det)

	id, err := ms.movieMetaStorage.CreateMovieMetadata(ctx, metadata)
	if err != nil {
		return nil, err
	}
	metadata.ID = id
	return &metadata, nil
}

func (ms MetadataService) GetMovieMetadataByID(ctx context.Context, metadataID int64) (*sqlcdb.MovieMetadatum, error) {
	meta, err := ms.movieMetaStorage.GetMovieMetadataByID(ctx, metadataID)
	if err != nil {
		return nil, err
	}
	return &meta, nil
}

// GetSeriesMetadata gets all metadata around a series. If it does not exist, it will be created including seasons and episodes.
func (ms MetadataService) GetSeriesMetadata(ctx context.Context, tmdbID int64) (*sqlcdb.SeriesMetadatum, error) {
	metadata, err := ms.seriesMetaStorage.GetSeriesMetadataByTmdbID(ctx, tmdbID)
	if err == nil {
		return &metadata, nil
	}
	if !errors.Is(err, storage.ErrNotFound) {
		return nil, err
	}

	return ms.loadSeriesMetadata(ctx, tmdbID)
}

// RefreshSeriesMetadataFromTMDB refreshes series metadata with proper entity linking.
func (ms MetadataService) RefreshSeriesMetadataFromTMDB(ctx context.Context, tmdbID int64) (*sqlcdb.SeriesMetadatum, error) {
	return ms.loadSeriesMetadata(ctx, tmdbID)
}

func (ms MetadataService) UpdateSeriesMetadataFromTMDB(ctx context.Context, tmdbID int64) (*sqlcdb.SeriesMetadatum, error) {
	log := logger.FromCtx(ctx)

	details, err := ms.tmdb.GetSeriesDetails(ctx, int(tmdbID))
	if err != nil {
		log.Error("failed to get series details", zap.Error(err))
		return nil, err
	}

	existing, err := ms.seriesMetaStorage.GetSeriesMetadataByTmdbID(ctx, tmdbID)
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
		updated.ExternalIds = sql.NullString{String: *extIDs, Valid: true}
	}

	if watchProviders, err := ms.fetchWatchProviders(ctx, tmdbID); err == nil && watchProviders != nil {
		updated.WatchProviders = sql.NullString{String: *watchProviders, Valid: true}
	}

	err = ms.seriesMetaStorage.UpdateSeriesMetadata(ctx, updated)
	if err != nil {
		log.Error("failed to update series metadata", zap.Error(err))
		return nil, err
	}

	return &updated, nil
}

func (ms MetadataService) loadSeriesMetadata(ctx context.Context, tmdbID int64) (*sqlcdb.SeriesMetadatum, error) {
	log := logger.FromCtx(ctx)
	details, err := ms.tmdb.GetSeriesDetails(ctx, int(tmdbID))
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
		series.ExternalIds = sql.NullString{String: *extIDs, Valid: true}
	}

	if watchProviders, err := ms.fetchWatchProviders(ctx, tmdbID); err == nil && watchProviders != nil {
		series.WatchProviders = sql.NullString{String: *watchProviders, Valid: true}
	}

	seriesMetadataID, err := ms.seriesMetaStorage.CreateSeriesMetadata(ctx, series)
	if err != nil {
		var sqliteErr sqlite3.Error
		if !errors.As(err, &sqliteErr) || sqliteErr.Code != sqlite3.ErrConstraint || sqliteErr.ExtendedCode != sqlite3.ErrConstraintUnique {
			log.Error("failed to create series metadata", zap.Error(err))
			return nil, err
		}

		existing, getErr := ms.seriesMetaStorage.GetSeriesMetadataByTmdbID(ctx, series.TmdbID)
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
		seriesMetadataID = existing.ID
	}

	for _, s := range details.Seasons {
		season := FromSeriesSeasons(s)
		season.SeriesMetadataID = seriesMetadataID

		seasonMetadataID := int64(0)
		if existingSeason, err := ms.seriesMetaStorage.GetSeasonMetadataByTmdbID(ctx, season.TmdbID); err == nil {
			seasonMetadataID = existingSeason.ID
		} else if !errors.Is(err, storage.ErrNotFound) {
			log.Error("failed to get existing season metadata", zap.Error(err))
			return nil, err
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
			episodeMetadata.SeasonMetadataID = seasonMetadataID

			_, err = ms.seriesMetaStorage.GetEpisodeMetadataByTmdbID(ctx, episodeMetadata.TmdbID)
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

	created, err := ms.seriesMetaStorage.GetSeriesMetadataByID(ctx, seriesMetadataID)
	if err != nil {
		return nil, err
	}
	return &created, nil
}

func (ms MetadataService) fetchExternalIDs(ctx context.Context, tmdbID int64) (*string, error) {
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

func (ms MetadataService) fetchWatchProviders(ctx context.Context, tmdbID int64) (*string, error) {
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

func (ms MetadataService) RefreshSeriesMetadata(ctx context.Context, tmdbIDs ...int64) error {
	log := logger.FromCtx(ctx).With("ids", tmdbIDs)
	log.Debug("refreshing series metadata")

	var errs []error

	if len(tmdbIDs) == 0 {
		allSeries, err := ms.seriesMetaStorage.ListSeriesMetadataByStatus(ctx, "")
		if err != nil {
			log.Error("failed to list series with empty status", zap.Error(err))
			return err
		}

		for _, series := range allSeries {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			_, err := ms.UpdateSeriesMetadataFromTMDB(ctx, series.TmdbID)
			if err != nil {
				log.Error("failed to refresh series metadata", zap.Int64("tmdb_id", series.TmdbID), zap.Error(err))
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
			log.Error("failed to refresh series metadata", zap.Int64("tmdb_id", id), zap.Error(err))
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func (ms MetadataService) RefreshMovieMetadata(ctx context.Context, tmdbIDs ...int64) error {
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
			_, err := ms.UpdateMovieMetadataFromTMDB(ctx, movie.TmdbID)
			if err != nil {
				log.Error("failed to refresh movie metadata", zap.Int64("tmdb_id", movie.TmdbID), zap.Error(err))
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
			log.Error("failed to refresh movie metadata", zap.Int64("tmdb_id", id), zap.Error(err))
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func FromMediaDetails(det tmdb.MediaDetails) sqlcdb.MovieMetadatum {
	model := sqlcdb.MovieMetadatum{
		TmdbID:  int64(det.ID),
		Images:  "",
		Title:   *det.Title,
		Runtime: int64(*det.Runtime),
	}
	if det.ImdbID != nil {
		model.ImdbID = sql.NullString{String: *det.ImdbID, Valid: true}
	}
	if det.OriginalTitle != nil {
		model.OriginalTitle = sql.NullString{String: *det.OriginalTitle, Valid: true}
	}
	if det.Overview != nil {
		model.Overview = sql.NullString{String: *det.Overview, Valid: true}
	}
	if det.PosterPath != nil {
		model.Images = *det.PosterPath
	}

	if det.Genres != nil {
		names := []string{}
		for _, g := range *det.Genres {
			names = append(names, g.Name)
		}
		model.Genres = sql.NullString{String: strings.Join(names, ","), Valid: true}
	}

	if det.Homepage != nil {
		model.Website = sql.NullString{String: *det.Homepage, Valid: true}
	}

	if det.Popularity != nil {
		model.Popularity = sql.NullFloat64{Float64: float64(*det.Popularity), Valid: true}
	}

	if det.ProductionCompanies != nil && len(*det.ProductionCompanies) > 0 {
		studio := (*det.ProductionCompanies)[0].Name
		if studio != nil {
			model.Studio = sql.NullString{String: *studio, Valid: true}
		}
	}

	if det.BelongsToCollection != nil {
		if collMap, ok := (*det.BelongsToCollection).(map[string]any); ok {
			if rawID, ok := collMap["id"].(float64); ok {
				model.CollectionTmdbID = sql.NullInt64{Int64: int64(rawID), Valid: true}
			}
			if name, ok := collMap["name"].(string); ok {
				model.CollectionTitle = sql.NullString{String: name, Valid: true}
			}
		}
	}

	if det.ReleaseDate != nil {
		releaseDate, err := parseTMDBDate(*det.ReleaseDate)
		if err == nil && releaseDate != nil {
			model.ReleaseDate = sql.NullTime{Time: *releaseDate, Valid: true}
		}

		if model.ReleaseDate.Valid {
			model.Year = sql.NullInt64{Int64: int64(model.ReleaseDate.Time.Year()), Valid: true}
		}
	}

	return model
}

func FromSeriesDetails(series tmdb.SeriesDetails) (sqlcdb.SeriesMetadatum, error) {
	airDate, err := parseTMDBDate(series.FirstAirDate)
	if err != nil {
		return sqlcdb.SeriesMetadatum{}, err
	}

	poster := sql.NullString{}
	if series.PosterPath != "" {
		poster = sql.NullString{String: series.PosterPath, Valid: true}
	}

	var firstAirDate sql.NullTime
	if airDate != nil {
		firstAirDate = sql.NullTime{Time: *airDate, Valid: true}
	}
	return sqlcdb.SeriesMetadatum{
		TmdbID:       int64(series.ID),
		Title:        series.Name,
		SeasonCount:  int64(series.NumberOfSeasons),
		EpisodeCount: int64(series.NumberOfEpisodes),
		FirstAirDate: firstAirDate,
		PosterPath:   poster,
		Overview:     sql.NullString{String: series.Overview, Valid: true},
		Status:       series.Status,
	}, nil
}

func FromSeriesSeasons(tmdbSeason tmdb.Season) sqlcdb.SeasonMetadatum {
	model := sqlcdb.SeasonMetadatum{
		TmdbID:   int64(tmdbSeason.ID),
		Title:    tmdbSeason.Name,
		Number:   int64(tmdbSeason.SeasonNumber),
		Overview: sql.NullString{String: tmdbSeason.Overview, Valid: true},
	}

	if airDate, err := parseTMDBDate(tmdbSeason.AirDate); err == nil && airDate != nil {
		model.AirDate = sql.NullTime{Time: *airDate, Valid: true}
	}

	return model
}

func FromSeriesEpisodes(episode tmdb.Episode) sqlcdb.EpisodeMetadatum {
	model := sqlcdb.EpisodeMetadatum{
		TmdbID:   int64(episode.ID),
		Title:    episode.Name,
		Number:   int64(episode.EpisodeNumber),
		Runtime:  sql.NullInt64{Int64: int64(episode.Runtime), Valid: true},
		Overview: sql.NullString{String: episode.Overview, Valid: true},
	}

	if airDate, err := parseTMDBDate(episode.AirDate); err == nil && airDate != nil {
		model.AirDate = sql.NullTime{Time: *airDate, Valid: true}
	}

	if episode.StillPath != "" {
		model.StillPath = sql.NullString{String: episode.StillPath, Valid: true}
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
