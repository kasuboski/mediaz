package manager

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/kasuboski/mediaz/pkg/library"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/storage"
	sqlcdb "github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/kasuboski/mediaz/pkg/tmdb"
	"go.uber.org/zap"
)

// SeriesMetadataProvider provides series metadata operations needed by SeriesService.
// This decouples SeriesService from the full MediaManager, allowing the metadata
// subsystem to be extracted independently later.
type SeriesMetadataProvider interface {
	GetSeriesMetadata(ctx context.Context, tmdbID int64) (*sqlcdb.SeriesMetadatum, error)
}

// SeriesService handles series CRUD and TV detail lookups.
// Indexing and reconciliation remain on MediaManager for now.
type SeriesService struct {
	tmdb              tmdb.ITmdb
	library           library.Library
	seriesStorage     storage.SeriesStorage
	seriesMetaStorage storage.SeriesMetadataStorage
	qualityService    *QualityService
	metadataProvider  SeriesMetadataProvider
}

// NewSeriesService creates a SeriesService with the given dependencies.
func NewSeriesService(tmdbClient tmdb.ITmdb, lib library.Library, seriesStorage storage.SeriesStorage, seriesMetaStorage storage.SeriesMetadataStorage, qualityService *QualityService, metadataProvider SeriesMetadataProvider) *SeriesService {
	return &SeriesService{
		tmdb:              tmdbClient,
		library:           lib,
		seriesStorage:     seriesStorage,
		seriesMetaStorage: seriesMetaStorage,
		qualityService:    qualityService,
		metadataProvider:  metadataProvider,
	}
}

// ---------------------------------------------------------------------------
// CRUD Operations
// ---------------------------------------------------------------------------

// AddSeriesToLibrary adds a series to be managed by mediaz.
func (s SeriesService) AddSeriesToLibrary(ctx context.Context, request AddSeriesRequest) (*sqlcdb.GetSeriesRow, error) {
	log := logger.FromCtx(ctx)

	qualityProfile, err := s.qualityService.GetQualityProfile(ctx, int64(request.QualityProfileID))
	if err != nil {
		log.Debug("failed to get quality profile", zap.Int64("id", request.QualityProfileID), zap.Error(err))
		return nil, err
	}

	seriesMetadata, err := s.metadataProvider.GetSeriesMetadata(ctx, request.TMDBID)
	if err != nil {
		log.Debug("failed to get series metadata", zap.Error(err))
		return nil, err
	}

	series, err := s.seriesStorage.GetSeriesByMetadataID(ctx, seriesMetadata.ID)
	// if we find the series we dont need to add it
	if err == nil {
		return &series, err
	}
	if !errors.Is(err, storage.ErrNotFound) {
		log.Warn("couldn't find series by metadata", zap.Int64("meta_id", seriesMetadata.ID), zap.Error(err))
		return nil, err
	}

	monitorNewSeasons := int64(0)
	if request.MonitorNewSeasons {
		monitorNewSeasons = 1
	}
	newSeries := sqlcdb.Series{
		SeriesMetadataID:  sql.NullInt64{Int64: seriesMetadata.ID, Valid: true},
		QualityProfileID:  qualityProfile.ID,
		Monitored:         1,
		Path:              sql.NullString{String: seriesMetadata.Title, Valid: true},
		MonitorNewSeasons: monitorNewSeasons,
	}

	var firstAirDate *time.Time
	if seriesMetadata.FirstAirDate.Valid {
		firstAirDate = &seriesMetadata.FirstAirDate.Time
	}
	state := initialSeriesState(firstAirDate)

	seriesID, err := s.seriesStorage.CreateSeries(ctx, newSeries, state)
	if err != nil {
		log.Error("failed to create new missing series", zap.Error(err))
		return nil, err
	}

	log.Debug("created new missing series", zap.Any("series", newSeries))

	// cleanupCreatedSeries removes the series on any child-creation failure.
	cleanupCreatedSeries := func() {
		if delErr := s.seriesStorage.DeleteSeries(ctx, seriesID); delErr != nil {
			log.Error("failed to cleanup created series after error", zap.Int64("series_id", seriesID), zap.Error(delErr))
		}
	}

	// Get series to access its metadata ID
	seriesEntity, err := s.seriesStorage.GetSeriesByID(ctx, seriesID)
	if err != nil || !seriesEntity.Series.SeriesMetadataID.Valid {
		log.Error("failed to get series or series has no metadata", zap.Error(err))
		cleanupCreatedSeries()
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("series has no metadata")
	}

	seasonMetadata, err := s.seriesMetaStorage.ListSeasonMetadataBySeriesMetadataID(ctx, seriesEntity.Series.SeriesMetadataID.Int64)
	if err != nil {
		cleanupCreatedSeries()
		return nil, err
	}

	for _, sm := range seasonMetadata {
		season := sqlcdb.Season{
			SeriesID:         seriesID,
			SeasonMetadataID: sql.NullInt64{Int64: sm.ID, Valid: true},
			Monitored:        1,
		}

		seasonID, err := s.seriesStorage.CreateSeason(ctx, season, storage.SeasonStateMissing)
		if err != nil {
			log.Error("failed to create season", zap.Error(err))
			cleanupCreatedSeries()
			return nil, err
		}

		log.Debug("created new missing season", zap.Any("season", season))

		// Get the season to access its metadata ID for proper episode metadata querying
		seasonEntity, err := s.seriesStorage.GetSeasonByID(ctx, seasonID)
		if err != nil || !seasonEntity.Season.SeasonMetadataID.Valid {
			log.Error("failed to get season or season has no metadata linked", zap.Error(err))
			cleanupCreatedSeries()
			if err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("season has no metadata")
		}

		episodesMetadata, err := s.seriesMetaStorage.ListEpisodeMetadataBySeasonMetadataID(ctx, seasonEntity.Season.SeasonMetadataID.Int64)
		if err != nil {
			log.Error("failed to list episode metadata", zap.Error(err))
			cleanupCreatedSeries()
			return nil, err
		}

		for _, e := range episodesMetadata {
			episode := sqlcdb.Episode{
				EpisodeMetadataID: sql.NullInt64{Int64: e.ID, Valid: true},
				SeasonID:          seasonID,
				Monitored:         1,
				EpisodeNumber:     e.Number,
			}

			_, err := s.seriesStorage.CreateEpisode(ctx, episode, storage.EpisodeStateMissing)
			if err != nil {
				log.Error("failed to create episode", zap.Error(err))
				cleanupCreatedSeries()
				return nil, err
			}

			log.Debug("created new missing episode", zap.Any("episode", episode))
		}
	}

	created, err := s.seriesStorage.GetSeriesByID(ctx, seriesID)
	if err != nil {
		log.Warn("failed to get created series", zap.Error(err))
		return nil, err
	}

	return &created, nil
}

// DeleteSeries removes a series and optionally its files from disk.
func (s SeriesService) DeleteSeries(ctx context.Context, seriesID int64, deleteDirectory bool) error {
	log := logger.FromCtx(ctx)

	series, err := s.seriesStorage.GetSeriesByID(ctx, seriesID)
	if err != nil {
		return fmt.Errorf("failed to get series: %w", err)
	}

	seasons, err := s.seriesStorage.ListSeasons(ctx, storage.SeasonFilter{SeriesID: &seriesID})
	if err != nil {
		log.Warn("failed to get seasons for cleanup", zap.Error(err))
	}

	// When deleting the directory, collect episode file IDs first, then attempt
	// filesystem deletion before touching the DB, to avoid DB inconsistency on failure.
	var episodeFileIDs []int64
	if deleteDirectory {
		for _, season := range seasons {
			seasonID := season.Season.ID
			episodes, err := s.seriesStorage.ListEpisodes(ctx, storage.EpisodeFilter{SeasonID: &seasonID})
			if err != nil {
				log.Warn("failed to get episodes for cleanup", zap.Int64("season_id", seasonID), zap.Error(err))
				continue
			}
			for _, episode := range episodes {
				if !episode.Episode.EpisodeFileID.Valid {
					continue
				}
				episodeFileIDs = append(episodeFileIDs, episode.Episode.EpisodeFileID.Int64)
			}
		}

		if !series.Series.Path.Valid {
			return fmt.Errorf("cannot delete directory: series path is nil")
		}

		if err := s.library.DeleteSeriesDirectory(ctx, series.Series.Path.String); err != nil {
			return fmt.Errorf("failed to delete series directory %s: %w", series.Series.Path.String, err)
		}

		// Only delete episode file DB rows after successful directory deletion.
		for _, fileID := range episodeFileIDs {
			if err := s.seriesStorage.DeleteEpisodeFile(ctx, fileID); err != nil {
				log.Warn("failed to delete episode file", zap.Int64("episode_file_id", fileID), zap.Error(err))
			}
		}
	}

	if err := s.seriesStorage.DeleteSeries(ctx, seriesID); err != nil {
		return fmt.Errorf("failed to delete series: %w", err)
	}

	log.Info("deleted series", zap.Int64("id", seriesID), zap.Bool("directory_deleted", deleteDirectory))
	return nil
}

// UpdateSeriesMonitored toggles the monitored state of a series.
func (s SeriesService) UpdateSeriesMonitored(ctx context.Context, seriesID int64, monitored bool) (*sqlcdb.GetSeriesRow, error) {
	monitoredInt := int64(0)
	if monitored {
		monitoredInt = 1
	}

	seriesUpdate := sqlcdb.Series{Monitored: monitoredInt}
	err := s.seriesStorage.UpdateSeriesByID(ctx, seriesID, seriesUpdate)
	if err != nil {
		return nil, err
	}

	series, err := s.seriesStorage.GetSeriesByID(ctx, seriesID)
	if err != nil {
		return nil, err
	}

	logger.FromCtx(ctx).Info("updated monitoring", zap.Int64("series_id", seriesID), zap.Bool("monitored", monitored))
	return &series, nil
}

// ---------------------------------------------------------------------------
// TV Detail & Search
// ---------------------------------------------------------------------------

// GetTVDetailByTMDBID retrieves detailed information for a single TV show by TMDB ID.
func (s SeriesService) GetTVDetailByTMDBID(ctx context.Context, tmdbID int64) (*TVDetailResult, error) {
	log := logger.FromCtx(ctx)

	// Get data from various sources
	metadata, seriesDetailsResponse, err := s.getTVMetadataAndDetails(ctx, tmdbID)
	if err != nil {
		log.Error("failed to get TV metadata and details", zap.Error(err), zap.Int64("tmdbID", tmdbID))
		return nil, err
	}

	// Get library information
	var series *sqlcdb.GetSeriesRow
	if srow, err := s.seriesStorage.GetSeriesByMetadataID(ctx, metadata.ID); err == nil {
		series = &srow
	} else if !errors.Is(err, storage.ErrNotFound) {
		log.Debug("error checking series library status", zap.Error(err), zap.Int64("metadataID", metadata.ID))
	}

	// Get seasons and episodes information for the consolidated response
	var seasons []SeasonResult
	if series != nil {
		seasonsData, err := s.getSeasonsWithEpisodes(ctx, series.Series.ID)
		if err != nil {
			log.Debug("failed to get seasons and episodes", zap.Error(err), zap.Int64("metadataID", metadata.ID))
			// Continue without seasons data - buildTVDetailResult will handle empty seasons
			seasons = []SeasonResult{}
		} else {
			seasons = seasonsData
		}
	} else {
		seasons = []SeasonResult{}
	}

	// Transform data into result
	result := s.buildTVDetailResult(metadata, seriesDetailsResponse, series, seasons)

	// Add stored external IDs from database
	var externalIDs *string
	if metadata.ExternalIds.Valid {
		externalIDs = &metadata.ExternalIds.String
	}
	if extIDs, err := DeserializeExternalIDs(externalIDs); err == nil && extIDs != nil {
		result.ExternalIDs = &ExternalIDs{ImdbID: extIDs.ImdbID, TvdbID: extIDs.TvdbID}
	}

	// Add stored watch providers from database
	var watchProviders *string
	if metadata.WatchProviders.Valid {
		watchProviders = &metadata.WatchProviders.String
	}
	if wpData, err := DeserializeWatchProviders(watchProviders); err == nil && wpData != nil {
		providers := make([]WatchProvider, 0, len(wpData.US.Flatrate))
		for _, p := range wpData.US.Flatrate {
			providers = append(providers, WatchProvider{
				ProviderID: p.ProviderID,
				Name:       p.Name,
				LogoPath:   p.LogoPath,
			})
		}
		result.WatchProviders = providers
	}

	return result, nil
}

// GetSeriesDetails retrieves basic series details from TMDB.
func (s SeriesService) GetSeriesDetails(ctx context.Context, tmdbID int64) (sqlcdb.SeriesMetadatum, error) {
	var m sqlcdb.SeriesMetadatum
	det, err := s.tmdb.GetSeriesDetails(ctx, int(tmdbID))
	if err != nil {
		return m, err
	}

	m, err = FromSeriesDetails(*det)
	return m, err
}

// SearchTV queries TMDB for TV shows matching the given query.
func (s SeriesService) SearchTV(ctx context.Context, query string) (*SearchMediaResponse, error) {
	log := logger.FromCtx(ctx)
	if query == "" {
		log.Debug("search tv query is empty", zap.String("query", query))
		return nil, errors.New("query is empty")
	}

	res, err := s.tmdb.SearchTv(ctx, &tmdb.SearchTvParams{Query: query})
	if err != nil {
		log.Error("search tv failed request", zap.Error(err))
		return nil, err
	}
	defer res.Body.Close()

	log.Debug("search tv response", zap.Any("status", res.Status))
	result, err := parseMediaResult(res)
	if err != nil {
		log.Debug("error parsing tv show query result", zap.Error(err))
		return nil, err
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// Listing
// ---------------------------------------------------------------------------

// ListShowsInLibrary returns all tracked series enriched with metadata.
func (s SeriesService) ListShowsInLibrary(ctx context.Context) ([]LibraryShow, error) {
	series, err := s.seriesStorage.ListSeries(ctx, storage.SeriesFilter{})
	if err != nil {
		return nil, err
	}
	var shows []LibraryShow
	for _, sp := range series {
		// Skip series without metadata - they haven't been reconciled yet
		if !sp.Series.SeriesMetadataID.Valid {
			continue
		}

		meta, err := s.seriesMetaStorage.GetSeriesMetadataByID(ctx, sp.Series.SeriesMetadataID.Int64)
		if err != nil {
			return nil, err
		}
		ls := LibraryShow{State: string(sp.State)}
		if sp.Series.Path.Valid {
			ls.Path = sp.Series.Path.String
		}
		ls.TMDBID = meta.TmdbID
		ls.Title = meta.Title
		if meta.PosterPath.Valid {
			ls.PosterPath = meta.PosterPath.String
		}
		if meta.FirstAirDate.Valid {
			ls.Year = int32(meta.FirstAirDate.Time.Year())
		}
		shows = append(shows, ls)
	}
	return shows, nil
}

// ListSeasonsForSeries retrieves all seasons for a TV series by TMDB ID.
func (s SeriesService) ListSeasonsForSeries(ctx context.Context, tmdbID int64) ([]SeasonResult, error) {
	log := logger.FromCtx(ctx)

	// Ensure series metadata exists
	metadata, err := s.metadataProvider.GetSeriesMetadata(ctx, tmdbID)
	if err != nil {
		log.Error("failed to get series metadata", zap.Error(err), zap.Int64("tmdbID", tmdbID))
		return nil, err
	}

	// Find the series record that uses this metadata
	series, err := s.seriesStorage.GetSeriesByMetadataID(ctx, metadata.ID)
	if err != nil {
		log.Error("failed to get series", zap.Error(err), zap.Int64("metadataID", metadata.ID))
		return nil, err
	}

	// Query seasons with metadata join
	seriesID := series.Series.ID
	seasons, err := s.seriesStorage.ListSeasons(ctx, storage.SeasonFilter{SeriesID: &seriesID})
	if err != nil {
		log.Error("failed to list seasons", zap.Error(err), zap.Int64("seriesID", series.Series.ID))
		return nil, err
	}

	// Transform to response format with metadata lookup
	var results []SeasonResult
	for _, season := range seasons {
		// Get season metadata for rich data
		if !season.Season.SeasonMetadataID.Valid {
			log.Debug("season has no metadata ID, skipping", zap.Int64("seasonID", season.Season.ID))
			continue
		}

		seasonMeta, err := s.seriesMetaStorage.GetSeasonMetadataByID(ctx, season.Season.SeasonMetadataID.Int64)
		if err != nil {
			log.Error("failed to get season metadata", zap.Error(err), zap.Int64("seasonMetadataID", season.Season.SeasonMetadataID.Int64))
			continue
		}

		// Count episodes for this season
		seasonIDInt := season.Season.ID
		episodes, err := s.seriesStorage.ListEpisodes(ctx, storage.EpisodeFilter{SeasonID: &seasonIDInt})
		if err != nil {
			log.Debug("failed to count episodes for season", zap.Error(err), zap.Int64("seasonID", season.Season.ID))
		}

		result := SeasonResult{
			SeriesID:     season.Season.SeriesID,
			Number:       int32(seasonMeta.Number),
			Title:        seasonMeta.Title,
			TMDBID:       seasonMeta.TmdbID,
			Monitored:    season.Season.Monitored == 1,
			EpisodeCount: int32(len(episodes)),
		}

		// Add optional fields
		if seasonMeta.Overview.Valid {
			result.Overview = &seasonMeta.Overview.String
		}
		if seasonMeta.AirDate.Valid {
			airDateStr := seasonMeta.AirDate.Time.Format("2006-01-02")
			result.AirDate = &airDateStr
		}

		results = append(results, result)
	}

	return results, nil
}

// ListEpisodesForSeason retrieves all episodes for a season by TMDB ID and season number.
func (s SeriesService) ListEpisodesForSeason(ctx context.Context, tmdbID int64, seasonNumber int) ([]EpisodeResult, error) {
	log := logger.FromCtx(ctx)

	// Ensure series metadata exists
	seriesMetadata, err := s.metadataProvider.GetSeriesMetadata(ctx, int64(tmdbID))
	if err != nil {
		log.Error("failed to get series metadata", zap.Error(err), zap.Int64("tmdbID", tmdbID))
		return nil, err
	}

	// Find the series record that uses this metadata
	series, err := s.seriesStorage.GetSeriesByMetadataID(ctx, seriesMetadata.ID)
	if err != nil {
		log.Error("failed to get series", zap.Error(err), zap.Int64("metadataID", seriesMetadata.ID))
		return nil, err
	}

	// Find all seasons for this series
	seriesID := series.Series.ID
	seasons, err := s.seriesStorage.ListSeasons(ctx, storage.SeasonFilter{SeriesID: &seriesID})
	if err != nil {
		log.Error("failed to list seasons", zap.Error(err), zap.Int64("seriesID", series.Series.ID))
		return nil, err
	}

	// Find the season that matches the requested season number
	var candidateSeasons []sqlcdb.ListSeasonsRow
	var candidateMetas []*sqlcdb.SeasonMetadatum

	for _, season := range seasons {
		// Check if season has metadata with the right number
		if season.Season.SeasonMetadataID.Valid {
			meta, err := s.seriesMetaStorage.GetSeasonMetadataByID(ctx, season.Season.SeasonMetadataID.Int64)
			if err == nil && meta.Number == int64(seasonNumber) {
				candidateSeasons = append(candidateSeasons, season)
				candidateMetas = append(candidateMetas, &meta)
			}
		}
	}

	// If no seasons found with metadata, fall back to season_number matching
	if len(candidateSeasons) == 0 {
		for _, season := range seasons {
			if season.Season.SeasonNumber == int64(seasonNumber) {
				candidateSeasons = append(candidateSeasons, season)
				candidateMetas = append(candidateMetas, nil)
			}
		}
	}

	if len(candidateSeasons) == 0 {
		log.Error("season not found", zap.Int64("seriesID", seriesMetadata.ID), zap.Int("seasonNumber", seasonNumber))
		return nil, fmt.Errorf("season %d not found for series %d", seasonNumber, tmdbID)
	}

	// If we have multiple candidates, prefer the one with episodes
	var targetSeason *sqlcdb.ListSeasonsRow
	var seasonMeta *sqlcdb.SeasonMetadatum

	if len(candidateSeasons) == 1 {
		targetSeason = &candidateSeasons[0]
		seasonMeta = candidateMetas[0]
	} else {
		// Multiple candidates - check which one has episodes
		for i, season := range candidateSeasons {
			seasonIDInt := season.Season.ID
			episodes, err := s.seriesStorage.ListEpisodes(ctx, storage.EpisodeFilter{SeasonID: &seasonIDInt})
			if err == nil && len(episodes) > 0 {
				targetSeason = &candidateSeasons[i]
				seasonMeta = candidateMetas[i]
				break
			}
		}

		// If no season has episodes, just use the first one
		if targetSeason == nil {
			targetSeason = &candidateSeasons[0]
			seasonMeta = candidateMetas[0]
		}
	}

	// Query episodes for this season
	targetSeasonID := targetSeason.Season.ID
	episodes, err := s.seriesStorage.ListEpisodes(ctx, storage.EpisodeFilter{SeasonID: &targetSeasonID})
	if err != nil {
		log.Error("failed to list episodes", zap.Error(err), zap.Int64("seasonID", targetSeason.Season.ID))
		return nil, err
	}

	// Determine season number for response
	seasonNum := int32(seasonNumber)
	if seasonMeta != nil {
		seasonNum = int32(seasonMeta.Number)
	}

	results := make([]EpisodeResult, 0, len(episodes))
	metaMap := s.preloadEpisodeMetadata(ctx, episodes)
	for _, episode := range episodes {
		var episodeMeta *sqlcdb.EpisodeMetadatum
		if episode.Episode.EpisodeMetadataID.Valid {
			episodeMeta = metaMap[episode.Episode.EpisodeMetadataID.Int64]
		}
		results = append(results, buildEpisodeResult(episode, episodeMeta, series.Series.ID, seasonNum))
	}

	return results, nil
}

// getTVMetadataAndDetails retrieves both series metadata and full TMDB details.
func (s SeriesService) getTVMetadataAndDetails(ctx context.Context, tmdbID int64) (*sqlcdb.SeriesMetadatum, *tmdb.SeriesDetailsResponse, error) {
	// Get series metadata (creates if not exists)
	metadata, err := s.metadataProvider.GetSeriesMetadata(ctx, tmdbID)
	if err != nil {
		return nil, nil, err
	}

	// Get the full series details response from TMDB to access networks and status
	res, err := s.tmdb.TvSeriesDetails(ctx, int32(tmdbID), nil)
	if err != nil {
		return nil, nil, err
	}
	defer res.Body.Close()

	b, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, nil, err
	}

	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, nil, fmt.Errorf("unexpected TV series details status %s: %s", res.Status, string(b))
	}

	var seriesDetailsResponse tmdb.SeriesDetailsResponse
	if err := json.Unmarshal(b, &seriesDetailsResponse); err != nil {
		return nil, nil, err
	}
	return metadata, &seriesDetailsResponse, nil
}

// buildTVDetailResult transforms metadata and TMDB details into TVDetailResult.
func (s SeriesService) buildTVDetailResult(metadata *sqlcdb.SeriesMetadatum, details *tmdb.SeriesDetailsResponse, series *sqlcdb.GetSeriesRow, seasons []SeasonResult) *TVDetailResult {
	result := &TVDetailResult{
		TMDBID:        metadata.TmdbID,
		Title:         metadata.Title,
		SeasonCount:   int32(metadata.SeasonCount),
		EpisodeCount:  int32(metadata.EpisodeCount),
		LibraryStatus: "Not In Library", // Default status
	}

	if metadata.Overview.Valid {
		result.Overview = metadata.Overview.String
	}

	// Set poster path - prefer database over TMDB API to avoid unnecessary API calls
	if metadata.PosterPath.Valid && metadata.PosterPath.String != "" {
		result.PosterPath = metadata.PosterPath.String
	} else if details.PosterPath != "" {
		result.PosterPath = details.PosterPath
	}

	// Set backdrop path from TMDB API (not stored in database)
	if details.BackdropPath != "" {
		result.BackdropPath = &details.BackdropPath
	}

	// Format dates
	if metadata.FirstAirDate.Valid {
		firstAirDateStr := metadata.FirstAirDate.Time.Format("2006-01-02")
		result.FirstAirDate = &firstAirDateStr
	}
	if metadata.LastAirDate.Valid {
		lastAirDateStr := metadata.LastAirDate.Time.Format("2006-01-02")
		result.LastAirDate = &lastAirDateStr
	}
	// Next air date from TMDB next episode
	if details.NextEpisodeToAir.AirDate != "" {
		nextAir := details.NextEpisodeToAir.AirDate
		result.NextAirDate = &nextAir
	}

	// Status when available
	if details.Status != "" {
		status := details.Status
		result.Status = &status
	}

	// Original language
	if details.OriginalLanguage != "" {
		ol := details.OriginalLanguage
		result.OriginalLanguage = &ol
	}

	// Production countries (names when available)
	if len(details.ProductionCountries) > 0 {
		pcs := make([]string, 0, len(details.ProductionCountries))
		for _, pc := range details.ProductionCountries {
			if pc.Name != nil && *pc.Name != "" {
				pcs = append(pcs, *pc.Name)
			} else if pc.Iso31661 != nil && *pc.Iso31661 != "" {
				pcs = append(pcs, *pc.Iso31661)
			}
		}
		result.ProductionCountries = pcs
	}

	// Networks with optional logos
	if len(details.Networks) > 0 {
		networks := make([]NetworkInfo, 0, len(details.Networks))
		for _, n := range details.Networks {
			ni := NetworkInfo{Name: n.Name}
			if n.LogoPath != "" {
				lp := n.LogoPath
				ni.LogoPath = &lp
			}
			networks = append(networks, ni)
		}
		result.Networks = networks
	}

	// Extract genre names
	if len(details.Genres) > 0 {
		var genres []string
		for _, genre := range details.Genres {
			genres = append(genres, genre.Name)
		}
		result.Genres = genres
	}

	// Set additional fields from TMDB response
	if details.Adult {
		result.Adult = &details.Adult
	}
	if details.Popularity > 0 {
		pop := float64(details.Popularity)
		result.Popularity = &pop
	}
	// Map ratings when available
	if details.VoteAverage > 0 {
		va := float32(details.VoteAverage)
		result.VoteAverage = &va
	}
	if details.VoteCount > 0 {
		vc := int(details.VoteCount)
		result.VoteCount = &vc
	}

	// Set library status information if series exists
	if series != nil {
		seriesID := series.Series.ID
		qualityProfileID := series.Series.QualityProfileID
		var path *string
		if series.Series.Path.Valid {
			path = &series.Series.Path.String
		}
		monitored := series.Series.Monitored == 1
		monitorNewSeasons := series.Series.MonitorNewSeasons == 1
		result.ID = &seriesID
		result.LibraryStatus = series.State
		result.Path = path
		result.QualityProfileID = &qualityProfileID
		result.Monitored = &monitored
		result.MonitorNewSeasons = &monitorNewSeasons
	}

	// Add seasons information if available
	if len(seasons) > 0 {
		result.Seasons = seasons
	}

	return result
}

// getSeasonsWithEpisodes retrieves seasons and their episodes for a series.
func (s SeriesService) getSeasonsWithEpisodes(ctx context.Context, seriesID int64) ([]SeasonResult, error) {
	log := logger.FromCtx(ctx)

	// Query seasons for this series
	seriesIDInt := int64(seriesID)
	seasons, err := s.seriesStorage.ListSeasons(ctx, storage.SeasonFilter{SeriesID: &seriesIDInt})
	if err != nil {
		log.Error("failed to list seasons", zap.Error(err), zap.Int64("seriesID", seriesID))
		return nil, err
	}

	// Transform to response format with metadata and episodes
	var results []SeasonResult
	for _, season := range seasons {
		// Get season metadata for rich data
		if !season.Season.SeasonMetadataID.Valid {
			log.Debug("season has no metadata ID, skipping", zap.Int64("seasonID", season.Season.ID))
			continue
		}

		seasonMeta, err := s.seriesMetaStorage.GetSeasonMetadataByID(ctx, season.Season.SeasonMetadataID.Int64)
		if err != nil {
			log.Error("failed to get season metadata", zap.Error(err), zap.Int64("seasonMetadataID", season.Season.SeasonMetadataID.Int64))
			continue
		}

		// Get episodes for this season
		episodes, err := s.getEpisodesForSeason(ctx, season.Season.ID, season.Season.SeriesID, int32(seasonMeta.Number))
		if err != nil {
			log.Debug("failed to get episodes for season", zap.Error(err), zap.Int64("seasonID", season.Season.ID))
			// Continue with empty episodes array
			episodes = []EpisodeResult{}
		}

		result := SeasonResult{
			SeriesID:     season.Season.SeriesID,
			Number:       int32(seasonMeta.Number),
			Title:        seasonMeta.Title,
			TMDBID:       seasonMeta.TmdbID,
			Monitored:    season.Season.Monitored == 1,
			EpisodeCount: int32(len(episodes)),
			Episodes:     episodes,
		}

		// Add optional fields
		if seasonMeta.Overview.Valid {
			result.Overview = &seasonMeta.Overview.String
		}
		if seasonMeta.AirDate.Valid {
			airDateStr := seasonMeta.AirDate.Time.Format("2006-01-02")
			result.AirDate = &airDateStr
		}

		results = append(results, result)
	}

	return results, nil
}

// preloadEpisodeMetadata fetches all episode metadata for the given IDs in a single query,
// returning a map keyed by metadata ID.
func (s SeriesService) preloadEpisodeMetadata(ctx context.Context, episodes []sqlcdb.ListEpisodesRow) map[int64]*sqlcdb.EpisodeMetadatum {
	ids := make([]int64, 0, len(episodes))
	for _, ep := range episodes {
		if ep.Episode.EpisodeMetadataID.Valid {
			ids = append(ids, ep.Episode.EpisodeMetadataID.Int64)
		}
	}
	if len(ids) == 0 {
		return nil
	}

	metas, err := s.seriesMetaStorage.ListEpisodeMetadataByIDs(ctx, ids)
	if err != nil {
		log := logger.FromCtx(ctx)
		log.Error("failed to batch fetch episode metadata", zap.Error(err))
		return nil
	}

	result := make(map[int64]*sqlcdb.EpisodeMetadatum, len(metas))
	for i := range metas {
		result[metas[i].ID] = &metas[i]
	}
	return result
}

// getEpisodesForSeason retrieves episodes for a specific season.
func (s SeriesService) getEpisodesForSeason(ctx context.Context, seasonID int64, seriesID int64, seasonNumber int32) ([]EpisodeResult, error) {
	log := logger.FromCtx(ctx)

	// Query episodes for this season
	seasonIDInt := int64(seasonID)
	episodes, err := s.seriesStorage.ListEpisodes(ctx, storage.EpisodeFilter{SeasonID: &seasonIDInt})
	if err != nil {
		log.Error("failed to list episodes", zap.Error(err), zap.Int64("seasonID", seasonID))
		return nil, err
	}

	results := make([]EpisodeResult, 0, len(episodes))
	metaMap := s.preloadEpisodeMetadata(ctx, episodes)
	for _, episode := range episodes {
		var episodeMeta *sqlcdb.EpisodeMetadatum
		if episode.Episode.EpisodeMetadataID.Valid {
			episodeMeta = metaMap[episode.Episode.EpisodeMetadataID.Int64]
		}
		results = append(results, buildEpisodeResult(episode, episodeMeta, seriesID, seasonNumber))
	}

	return results, nil
}

// REMOVED: ensureSeries stays on MediaManager (series_index.go) for IndexerService extraction
