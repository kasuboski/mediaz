package manager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/pkg/library"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/ptr"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
	"github.com/kasuboski/mediaz/pkg/tmdb"
	"go.uber.org/zap"
)

// SeriesMetadataProvider provides series metadata operations needed by SeriesService.
// This decouples SeriesService from the full MediaManager, allowing the metadata
// subsystem to be extracted independently later.
type SeriesMetadataProvider interface {
	GetSeriesMetadata(ctx context.Context, tmdbID int) (*model.SeriesMetadata, error)
}

// SeriesService handles series CRUD, indexing, and TV detail lookups.
// Reconciliation remains on MediaManager for now due to cross-cutting dependencies.
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
func (s SeriesService) AddSeriesToLibrary(ctx context.Context, request AddSeriesRequest) (*storage.Series, error) {
	log := logger.FromCtx(ctx)

	qualityProfile, err := s.qualityService.GetQualityProfile(ctx, int64(request.QualityProfileID))
	if err != nil {
		log.Debug("failed to get quality profile", zap.Int32("id", request.QualityProfileID), zap.Error(err))
		return nil, err
	}

	seriesMetadata, err := s.metadataProvider.GetSeriesMetadata(ctx, request.TMDBID)
	if err != nil {
		log.Debug("failed to get series metadata", zap.Error(err))
		return nil, err
	}

	series, err := s.seriesStorage.GetSeries(ctx, table.Series.ID.EQ(sqlite.Int32(seriesMetadata.ID)))
	// if we find the series we dont need to add it
	if err == nil {
		return series, err
	}
	if !errors.Is(err, storage.ErrNotFound) {
		log.Warn("couldn't find series by metadata", zap.Int32("meta_id", seriesMetadata.ID), zap.Error(err))
		return nil, err
	}

	monitorNewSeasons := int32(0)
	if request.MonitorNewSeasons {
		monitorNewSeasons = 1
	}
	series = &storage.Series{
		Series: model.Series{
			SeriesMetadataID:  &seriesMetadata.ID,
			QualityProfileID:  qualityProfile.ID,
			Monitored:         1,
			Path:              &seriesMetadata.Title,
			MonitorNewSeasons: monitorNewSeasons,
		},
	}

	state := initialSeriesState(seriesMetadata.FirstAirDate)

	seriesID, err := s.seriesStorage.CreateSeries(ctx, *series, state)
	if err != nil {
		log.Error("failed to create new missing series", zap.Error(err))
		return nil, err
	}

	log.Debug("created new missing series", zap.Any("series", series))

	// Get series to access its metadata ID
	seriesEntity, err := s.seriesStorage.GetSeries(ctx, table.Series.ID.EQ(sqlite.Int(seriesID)))
	if err != nil || seriesEntity.SeriesMetadataID == nil {
		log.Error("failed to get series or series has no metadata")
		return nil, fmt.Errorf("series has no metadata")
	}

	where := table.SeasonMetadata.SeriesMetadataID.EQ(sqlite.Int32(*seriesEntity.SeriesMetadataID))
	seasonMetadata, err := s.seriesMetaStorage.ListSeasonMetadata(ctx, where)
	if err != nil {
		return nil, err
	}

	for _, sm := range seasonMetadata {
		season := storage.Season{
			Season: model.Season{
				SeriesID:         int32(seriesID),
				SeasonMetadataID: ptr.To(sm.ID),
				Monitored:        1,
			},
		}

		seasonID, err := s.seriesStorage.CreateSeason(ctx, season, storage.SeasonStateMissing)
		if err != nil {
			log.Error("failed to create season", zap.Error(err))
			return nil, err
		}

		log.Debug("created new missing season", zap.Any("season", season))

		// Get the season to access its metadata ID for proper episode metadata querying
		seasonEntity, err := s.seriesStorage.GetSeason(ctx, table.Season.ID.EQ(sqlite.Int64(seasonID)))
		if err != nil || seasonEntity.SeasonMetadataID == nil {
			log.Error("failed to get season or season has no metadata linked")
			return nil, fmt.Errorf("season has no metadata")
		}

		where := table.EpisodeMetadata.SeasonMetadataID.EQ(sqlite.Int32(*seasonEntity.SeasonMetadataID))

		episodesMetadata, err := s.seriesMetaStorage.ListEpisodeMetadata(ctx, where)
		if err != nil {
			log.Error("failed to list episode metadata", zap.Error(err))
			return nil, err
		}

		for _, e := range episodesMetadata {
			episode := storage.Episode{
				Episode: model.Episode{
					EpisodeMetadataID: ptr.To(e.ID),
					SeasonID:          int32(seasonID),
					Monitored:         1,
					EpisodeNumber:     e.Number,
				},
			}

			_, err := s.seriesStorage.CreateEpisode(ctx, episode, storage.EpisodeStateMissing)
			if err != nil {
				log.Error("failed to create episode", zap.Error(err))
				return nil, err
			}

			log.Debug("created new missing episode", zap.Any("episode", episode))
		}
	}

	series, err = s.seriesStorage.GetSeries(ctx, table.Series.ID.EQ(sqlite.Int64(seriesID)))
	if err != nil {
		log.Warn("failed to get created series", zap.Error(err))
	}

	return series, err
}

// DeleteSeries removes a series and optionally its files from disk.
func (s SeriesService) DeleteSeries(ctx context.Context, seriesID int64, deleteDirectory bool) error {
	log := logger.FromCtx(ctx)

	series, err := s.seriesStorage.GetSeries(ctx, table.Series.ID.EQ(sqlite.Int64(seriesID)))
	if err != nil {
		return fmt.Errorf("failed to get series: %w", err)
	}

	seasons, err := s.seriesStorage.ListSeasons(ctx, table.Season.SeriesID.EQ(sqlite.Int64(seriesID)))
	if err != nil {
		log.Warn("failed to get seasons for cleanup", zap.Error(err))
	}

	for _, season := range seasons {
		episodes, err := s.seriesStorage.ListEpisodes(ctx, table.Episode.SeasonID.EQ(sqlite.Int64(int64(season.ID))))
		if err != nil {
			log.Warn("failed to get episodes for cleanup", zap.Int32("season_id", season.ID), zap.Error(err))
			continue
		}
		for _, episode := range episodes {
			if episode.EpisodeFileID == nil {
				continue
			}
			if err := s.seriesStorage.DeleteEpisodeFile(ctx, int64(*episode.EpisodeFileID)); err != nil {
				log.Warn("failed to delete episode file", zap.Int32("episode_file_id", *episode.EpisodeFileID), zap.Error(err))
			}
		}
	}

	if deleteDirectory {
		if series.Path == nil {
			return fmt.Errorf("cannot delete directory: series path is nil")
		}

		if err := s.library.DeleteSeriesDirectory(ctx, *series.Path); err != nil {
			return fmt.Errorf("failed to delete series directory %s: %w", *series.Path, err)
		}
	}

	if err := s.seriesStorage.DeleteSeries(ctx, seriesID); err != nil {
		return fmt.Errorf("failed to delete series: %w", err)
	}

	log.Info("deleted series", zap.Int64("id", seriesID), zap.Bool("directory_deleted", deleteDirectory))
	return nil
}

// UpdateSeriesMonitored toggles the monitored state of a series.
func (s SeriesService) UpdateSeriesMonitored(ctx context.Context, seriesID int64, monitored bool) (*storage.Series, error) {
	monitoredInt := int32(0)
	if monitored {
		monitoredInt = 1
	}

	seriesUpdate := model.Series{Monitored: monitoredInt}
	err := s.seriesStorage.UpdateSeries(ctx, seriesUpdate, table.Series.ID.EQ(sqlite.Int64(seriesID)))
	if err != nil {
		return nil, err
	}

	series, err := s.seriesStorage.GetSeries(ctx, table.Series.ID.EQ(sqlite.Int64(seriesID)))
	if err != nil {
		return nil, err
	}

	logger.FromCtx(ctx).Info("updated monitoring", zap.Int64("series_id", seriesID), zap.Bool("monitored", monitored))
	return series, nil
}

// ---------------------------------------------------------------------------
// TV Detail & Search
// ---------------------------------------------------------------------------

// GetTVDetailByTMDBID retrieves detailed information for a single TV show by TMDB ID.
func (s SeriesService) GetTVDetailByTMDBID(ctx context.Context, tmdbID int) (*TVDetailResult, error) {
	log := logger.FromCtx(ctx)

	// Get data from various sources
	metadata, seriesDetailsResponse, err := s.getTVMetadataAndDetails(ctx, tmdbID)
	if err != nil {
		log.Error("failed to get TV metadata and details", zap.Error(err), zap.Int("tmdbID", tmdbID))
		return nil, err
	}

	// Get library information
	series, err := s.seriesStorage.GetSeries(ctx, table.Series.SeriesMetadataID.EQ(sqlite.Int32(metadata.ID)))
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		log.Debug("error checking series library status", zap.Error(err), zap.Int32("metadataID", metadata.ID))
	}

	// Get seasons and episodes information for the consolidated response
	var seasons []SeasonResult
	if series != nil {
		seasonsData, err := s.getSeasonsWithEpisodes(ctx, series.ID)
		if err != nil {
			log.Debug("failed to get seasons and episodes", zap.Error(err), zap.Int32("metadataID", metadata.ID))
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
	if extIDs, err := DeserializeExternalIDs(metadata.ExternalIds); err == nil && extIDs != nil {
		result.ExternalIDs = &ExternalIDs{ImdbID: extIDs.ImdbID, TvdbID: extIDs.TvdbID}
	}

	// Add stored watch providers from database
	if wpData, err := DeserializeWatchProviders(metadata.WatchProviders); err == nil && wpData != nil {
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
func (s SeriesService) GetSeriesDetails(ctx context.Context, tmdbID int) (model.SeriesMetadata, error) {
	var m model.SeriesMetadata
	det, err := s.tmdb.GetSeriesDetails(ctx, tmdbID)
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
	series, err := s.seriesStorage.ListSeries(ctx)
	if err != nil {
		return nil, err
	}
	shows := filterAndMap(series, func(sp *storage.Series) (LibraryShow, bool) {
		// Skip series without metadata - they haven't been reconciled yet
		if sp.SeriesMetadataID == nil {
			return LibraryShow{}, false
		}
		ls := LibraryShow{State: string(sp.State)}
		if sp.Path != nil {
			ls.Path = *sp.Path
		}
		meta, err := s.seriesMetaStorage.GetSeriesMetadata(ctx, table.SeriesMetadata.ID.EQ(sqlite.Int(int64(*sp.SeriesMetadataID))))
		if err != nil || meta == nil {
			return LibraryShow{}, false
		}
		ls.TMDBID = meta.TmdbID
		ls.Title = meta.Title
		if meta.PosterPath != nil {
			ls.PosterPath = *meta.PosterPath
		}
		if meta.FirstAirDate != nil {
			ls.Year = int32(meta.FirstAirDate.Year())
		}
		return ls, true
	})
	return shows, nil
}

// ListSeasonsForSeries retrieves all seasons for a TV series by TMDB ID.
func (s SeriesService) ListSeasonsForSeries(ctx context.Context, tmdbID int) ([]SeasonResult, error) {
	log := logger.FromCtx(ctx)

	// Ensure series metadata exists
	metadata, err := s.metadataProvider.GetSeriesMetadata(ctx, tmdbID)
	if err != nil {
		log.Error("failed to get series metadata", zap.Error(err), zap.Int("tmdbID", tmdbID))
		return nil, err
	}

	// Find the series record that uses this metadata
	series, err := s.seriesStorage.GetSeries(ctx,
		table.Series.SeriesMetadataID.EQ(sqlite.Int32(metadata.ID)))
	if err != nil {
		log.Error("failed to get series", zap.Error(err), zap.Int32("metadataID", metadata.ID))
		return nil, err
	}

	// Query seasons with metadata join
	seasons, err := s.seriesStorage.ListSeasons(ctx,
		table.Season.SeriesID.EQ(sqlite.Int32(series.ID)))
	if err != nil {
		log.Error("failed to list seasons", zap.Error(err), zap.Int32("seriesID", series.ID))
		return nil, err
	}

	// Transform to response format with metadata lookup
	var results []SeasonResult
	for _, season := range seasons {
		// Get season metadata for rich data
		if season.SeasonMetadataID == nil {
			log.Debug("season has no metadata ID, skipping", zap.Int32("seasonID", season.ID))
			continue
		}

		seasonMeta, err := s.seriesMetaStorage.GetSeasonMetadata(ctx,
			table.SeasonMetadata.ID.EQ(sqlite.Int32(*season.SeasonMetadataID)))
		if err != nil {
			log.Error("failed to get season metadata", zap.Error(err), zap.Int32("seasonMetadataID", *season.SeasonMetadataID))
			continue
		}

		// Count episodes for this season
		episodes, err := s.seriesStorage.ListEpisodes(ctx,
			table.Episode.SeasonID.EQ(sqlite.Int32(season.ID)))
		if err != nil {
			log.Debug("failed to count episodes for season", zap.Error(err), zap.Int32("seasonID", season.ID))
		}

		result := SeasonResult{
			SeriesID:     season.SeriesID,
			Number:       seasonMeta.Number,
			Title:        seasonMeta.Title,
			TMDBID:       seasonMeta.TmdbID,
			Monitored:    season.Monitored == 1,
			EpisodeCount: int32(len(episodes)),
		}

		// Add optional fields
		if seasonMeta.Overview != nil {
			result.Overview = seasonMeta.Overview
		}
		if seasonMeta.AirDate != nil {
			airDateStr := seasonMeta.AirDate.Format("2006-01-02")
			result.AirDate = &airDateStr
		}

		results = append(results, result)
	}

	return results, nil
}

// ListEpisodesForSeason retrieves all episodes for a season by TMDB ID and season number.
func (s SeriesService) ListEpisodesForSeason(ctx context.Context, tmdbID int, seasonNumber int) ([]EpisodeResult, error) {
	log := logger.FromCtx(ctx)

	// Ensure series metadata exists
	seriesMetadata, err := s.metadataProvider.GetSeriesMetadata(ctx, tmdbID)
	if err != nil {
		log.Error("failed to get series metadata", zap.Error(err), zap.Int("tmdbID", tmdbID))
		return nil, err
	}

	// Find the series record that uses this metadata
	series, err := s.seriesStorage.GetSeries(ctx,
		table.Series.SeriesMetadataID.EQ(sqlite.Int32(seriesMetadata.ID)))
	if err != nil {
		log.Error("failed to get series", zap.Error(err), zap.Int32("metadataID", seriesMetadata.ID))
		return nil, err
	}

	// Find all seasons for this series
	seasons, err := s.seriesStorage.ListSeasons(ctx,
		table.Season.SeriesID.EQ(sqlite.Int32(series.ID)))
	if err != nil {
		log.Error("failed to list seasons", zap.Error(err), zap.Int32("seriesID", series.ID))
		return nil, err
	}

	// Find the season that matches the requested season number
	var candidateSeasons []*storage.Season
	var candidateMetas []*model.SeasonMetadata

	for _, season := range seasons {
		// Check if season has metadata with the right number
		if season.SeasonMetadataID != nil {
			meta, err := s.seriesMetaStorage.GetSeasonMetadata(ctx,
				table.SeasonMetadata.ID.EQ(sqlite.Int32(*season.SeasonMetadataID)))
			if err == nil && meta.Number == int32(seasonNumber) {
				candidateSeasons = append(candidateSeasons, season)
				candidateMetas = append(candidateMetas, meta)
			}
		}
	}

	// If no seasons found with metadata, fall back to season_number matching
	if len(candidateSeasons) == 0 {
		for _, season := range seasons {
			if season.SeasonNumber == int32(seasonNumber) {
				candidateSeasons = append(candidateSeasons, season)
				candidateMetas = append(candidateMetas, nil)
			}
		}
	}

	if len(candidateSeasons) == 0 {
		log.Error("season not found", zap.Int32("seriesID", seriesMetadata.ID), zap.Int("seasonNumber", seasonNumber))
		return nil, fmt.Errorf("season %d not found for series %d", seasonNumber, tmdbID)
	}

	// If we have multiple candidates, prefer the one with episodes
	var targetSeason *storage.Season
	var seasonMeta *model.SeasonMetadata

	if len(candidateSeasons) == 1 {
		targetSeason = candidateSeasons[0]
		seasonMeta = candidateMetas[0]
	} else {
		// Multiple candidates - check which one has episodes
		for i, season := range candidateSeasons {
			episodes, err := s.seriesStorage.ListEpisodes(ctx,
				table.Episode.SeasonID.EQ(sqlite.Int32(season.ID)))
			if err == nil && len(episodes) > 0 {
				targetSeason = season
				seasonMeta = candidateMetas[i]
				break
			}
		}

		// If no season has episodes, just use the first one
		if targetSeason == nil {
			targetSeason = candidateSeasons[0]
			seasonMeta = candidateMetas[0]
		}
	}

	// Query episodes for this season
	episodes, err := s.seriesStorage.ListEpisodes(ctx,
		table.Episode.SeasonID.EQ(sqlite.Int32(targetSeason.ID)))
	if err != nil {
		log.Error("failed to list episodes", zap.Error(err), zap.Int32("seasonID", targetSeason.ID))
		return nil, err
	}

	// Determine season number for response
	seasonNum := int32(seasonNumber)
	if seasonMeta != nil {
		seasonNum = seasonMeta.Number
	}

	results := make([]EpisodeResult, 0, len(episodes))
	metaMap := s.preloadEpisodeMetadata(ctx, episodes)
	for _, episode := range episodes {
		var episodeMeta *model.EpisodeMetadata
		if episode.EpisodeMetadataID != nil {
			episodeMeta = metaMap[*episode.EpisodeMetadataID]
		}
		results = append(results, buildEpisodeResult(episode, episodeMeta, series.ID, seasonNum))
	}

	return results, nil
}

// ---------------------------------------------------------------------------
// Library Indexing
// ---------------------------------------------------------------------------

// IndexSeriesLibrary indexes the tv library directory for new files that are not yet monitored.
// The episodes are then stored with a state of discovered.
func (s SeriesService) IndexSeriesLibrary(ctx context.Context) error {
	log := logger.FromCtx(ctx).With("indexer", "series")

	discoveredFiles, err := s.library.FindEpisodes(ctx)
	if err != nil {
		return err
	}

	if len(discoveredFiles) == 0 {
		log.Debug("no files discovered")
		return nil
	}

	episodeFiles, err := s.seriesStorage.ListEpisodeFiles(ctx)
	if err != nil {
		return err
	}

	for _, discoveredFile := range discoveredFiles {
		matchedID, matchedPath := matchEpisodeFile(discoveredFile, episodeFiles, log)
		if err := s.upsertEpisodeFile(ctx, discoveredFile, matchedID, matchedPath); err != nil {
			log.Error("failed to upsert episode file", zap.Error(err))
		}
	}

	// pull the updated episode file list in case we added anything above
	episodeFiles, err = s.seriesStorage.ListEpisodeFiles(ctx)
	if err != nil {
		return err
	}

	for _, f := range episodeFiles {
		if f == nil || f.RelativePath == nil {
			continue
		}
		// rely on discovered EpisodeFile data; library provides series/season parsing
		var df library.EpisodeFile
		for _, d := range discoveredFiles {
			if d.RelativePath == *f.RelativePath {
				df = d
				break
			}
		}
		if df.SeriesName == "" {
			log.Warn("skipping episode file because series name is empty after matching", zap.String("episode_file_relative_path", *f.RelativePath))
			continue
		}

		// Check if this specific episode file already has an associated episode
		existingEpisode, err := s.seriesStorage.GetEpisodeByEpisodeFileID(ctx, int64(f.ID))
		if err == nil && existingEpisode != nil {
			log.Debug("episode file already has associated episode",
				zap.Int32("file_id", f.ID),
				zap.Int32("episode_id", existingEpisode.ID))
			continue
		}
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			log.Debug("error checking existing episode for file", zap.Error(err))
			continue
		}

		seriesID, err := s.ensureSeries(ctx, df.SeriesName)
		if err != nil {
			log.Error("couldn't ensure series for discovered file", zap.Error(err))
			continue
		}

		seasonID, err := s.getOrCreateSeason(ctx, seriesID, int32(df.SeasonNumber), nil, storage.SeasonStateDiscovered)
		if err != nil {
			log.Error("couldn't ensure season for discovered file", zap.Error(err))
			continue
		}

		// ensure episode exists; parse episode number from the discovered file data
		// Link the episode to the episode file so reconciliation can find the file details
		episode := storage.Episode{Episode: model.Episode{
			SeasonID:      int32(seasonID),
			Monitored:     0,
			EpisodeFileID: &f.ID,
			EpisodeNumber: int32(df.EpisodeNumber),
		}}
		_, err = s.seriesStorage.CreateEpisode(ctx, episode, storage.EpisodeStateDiscovered)
		if err != nil {
			log.Warn("failed to create new episode", zap.Error(err))
			continue
		}

		log.Debug("successfully indexed new episode", zap.Int32("ID", episode.ID), zap.Int("number", df.EpisodeNumber), zap.Int64("Season ID", seasonID))
	}

	return nil
}

// ---------------------------------------------------------------------------
// Internal Helpers
// ---------------------------------------------------------------------------

// getTVMetadataAndDetails retrieves both series metadata and full TMDB details.
func (s SeriesService) getTVMetadataAndDetails(ctx context.Context, tmdbID int) (*model.SeriesMetadata, *tmdb.SeriesDetailsResponse, error) {
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

	var seriesDetailsResponse tmdb.SeriesDetailsResponse
	if err := json.Unmarshal(b, &seriesDetailsResponse); err != nil {
		return nil, nil, err
	}
	return metadata, &seriesDetailsResponse, nil
}

// buildTVDetailResult transforms metadata and TMDB details into TVDetailResult.
func (s SeriesService) buildTVDetailResult(metadata *model.SeriesMetadata, details *tmdb.SeriesDetailsResponse, series *storage.Series, seasons []SeasonResult) *TVDetailResult {
	result := &TVDetailResult{
		TMDBID:        metadata.TmdbID,
		Title:         metadata.Title,
		SeasonCount:   metadata.SeasonCount,
		EpisodeCount:  metadata.EpisodeCount,
		LibraryStatus: "Not In Library", // Default status
	}

	if metadata.Overview != nil {
		result.Overview = *metadata.Overview
	}

	// Set poster path - prefer database over TMDB API to avoid unnecessary API calls
	if metadata.PosterPath != nil && *metadata.PosterPath != "" {
		result.PosterPath = *metadata.PosterPath
	} else if details.PosterPath != "" {
		result.PosterPath = details.PosterPath
	}

	// Set backdrop path from TMDB API (not stored in database)
	if details.BackdropPath != "" {
		result.BackdropPath = &details.BackdropPath
	}

	// Format dates
	if metadata.FirstAirDate != nil {
		firstAirDateStr := metadata.FirstAirDate.Format("2006-01-02")
		result.FirstAirDate = &firstAirDateStr
	}
	if metadata.LastAirDate != nil {
		lastAirDateStr := metadata.LastAirDate.Format("2006-01-02")
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
		result.ID = &series.ID
		result.LibraryStatus = string(series.State)
		result.Path = series.Path
		result.QualityProfileID = &series.QualityProfileID
		monitored := series.Monitored == 1
		result.Monitored = &monitored
		monitorNewSeasons := series.MonitorNewSeasons == 1
		result.MonitorNewSeasons = &monitorNewSeasons
	}

	// Add seasons information if available
	if len(seasons) > 0 {
		result.Seasons = seasons
	}

	return result
}

// getSeasonsWithEpisodes retrieves seasons and their episodes for a series.
func (s SeriesService) getSeasonsWithEpisodes(ctx context.Context, seriesID int32) ([]SeasonResult, error) {
	log := logger.FromCtx(ctx)

	// Query seasons for this series
	seasons, err := s.seriesStorage.ListSeasons(ctx,
		table.Season.SeriesID.EQ(sqlite.Int32(seriesID)))
	if err != nil {
		log.Error("failed to list seasons", zap.Error(err), zap.Int32("seriesID", seriesID))
		return nil, err
	}

	// Transform to response format with metadata and episodes
	var results []SeasonResult
	for _, season := range seasons {
		// Get season metadata for rich data
		if season.SeasonMetadataID == nil {
			log.Debug("season has no metadata ID, skipping", zap.Int32("seasonID", season.ID))
			continue
		}

		seasonMeta, err := s.seriesMetaStorage.GetSeasonMetadata(ctx,
			table.SeasonMetadata.ID.EQ(sqlite.Int32(*season.SeasonMetadataID)))
		if err != nil {
			log.Error("failed to get season metadata", zap.Error(err), zap.Int32("seasonMetadataID", *season.SeasonMetadataID))
			continue
		}

		// Get episodes for this season
		episodes, err := s.getEpisodesForSeason(ctx, season.ID, season.SeriesID, seasonMeta.Number)
		if err != nil {
			log.Debug("failed to get episodes for season", zap.Error(err), zap.Int32("seasonID", season.ID))
			// Continue with empty episodes array
			episodes = []EpisodeResult{}
		}

		result := SeasonResult{
			SeriesID:     season.SeriesID,
			Number:       seasonMeta.Number,
			Title:        seasonMeta.Title,
			TMDBID:       seasonMeta.TmdbID,
			Monitored:    season.Monitored == 1,
			EpisodeCount: int32(len(episodes)),
			Episodes:     episodes,
		}

		// Add optional fields
		if seasonMeta.Overview != nil {
			result.Overview = seasonMeta.Overview
		}
		if seasonMeta.AirDate != nil {
			airDateStr := seasonMeta.AirDate.Format("2006-01-02")
			result.AirDate = &airDateStr
		}

		results = append(results, result)
	}

	return results, nil
}

// preloadEpisodeMetadata fetches all episode metadata for the given IDs in a single query,
// returning a map keyed by metadata ID.
func (s SeriesService) preloadEpisodeMetadata(ctx context.Context, episodes []*storage.Episode) map[int32]*model.EpisodeMetadata {
	ids := make([]sqlite.Expression, 0, len(episodes))
	for _, ep := range episodes {
		if ep.EpisodeMetadataID != nil {
			ids = append(ids, sqlite.Int32(*ep.EpisodeMetadataID))
		}
	}
	if len(ids) == 0 {
		return nil
	}

	metas, err := s.seriesMetaStorage.ListEpisodeMetadata(ctx, table.EpisodeMetadata.ID.IN(ids...))
	if err != nil {
		log := logger.FromCtx(ctx)
		log.Error("failed to batch fetch episode metadata", zap.Error(err))
		return nil
	}

	result := make(map[int32]*model.EpisodeMetadata, len(metas))
	for _, meta := range metas {
		result[meta.ID] = meta
	}
	return result
}

// getEpisodesForSeason retrieves episodes for a specific season.
func (s SeriesService) getEpisodesForSeason(ctx context.Context, seasonID int32, seriesID int32, seasonNumber int32) ([]EpisodeResult, error) {
	log := logger.FromCtx(ctx)

	// Query episodes for this season
	episodes, err := s.seriesStorage.ListEpisodes(ctx,
		table.Episode.SeasonID.EQ(sqlite.Int32(seasonID)))
	if err != nil {
		log.Error("failed to list episodes", zap.Error(err), zap.Int32("seasonID", seasonID))
		return nil, err
	}

	results := make([]EpisodeResult, 0, len(episodes))
	metaMap := s.preloadEpisodeMetadata(ctx, episodes)
	for _, episode := range episodes {
		var episodeMeta *model.EpisodeMetadata
		if episode.EpisodeMetadataID != nil {
			episodeMeta = metaMap[*episode.EpisodeMetadataID]
		}
		results = append(results, buildEpisodeResult(episode, episodeMeta, seriesID, seasonNumber))
	}

	return results, nil
}

// ensureSeries gets or creates a series by path name.
func (s SeriesService) ensureSeries(ctx context.Context, seriesName string) (int64, error) {
	log := logger.FromCtx(ctx)

	series, err := s.seriesStorage.GetSeries(ctx, table.Series.Path.EQ(sqlite.String(seriesName)))
	if errors.Is(err, storage.ErrNotFound) || series == nil {
		log.Debug("episode file does not have associated series, creating new series")
		seriesModel := storage.Series{Series: model.Series{Path: &seriesName, Monitored: 0}}
		seriesID, err := s.seriesStorage.CreateSeries(ctx, seriesModel, storage.SeriesStateDiscovered)
		if err != nil {
			return 0, err
		}
		log.Debug("created new discovered series", zap.Int64("series id", seriesID))
		return seriesID, nil
	}
	if err != nil {
		return 0, err
	}

	seriesID := int64(series.ID)
	log.Debug("using existing series", zap.Int64("series id", seriesID))
	return seriesID, nil
}

// getOrCreateSeason gets or creates a season, with optional metadata linking.
func (s SeriesService) getOrCreateSeason(ctx context.Context, seriesID int64, seasonNumber int32, seasonMetadataID *int32, initialState storage.SeasonState) (int64, error) {
	log := logger.FromCtx(ctx).With(
		zap.Int64("series_id", seriesID),
		zap.Int32("season_number", seasonNumber),
	)

	// First try to find existing season by series_id + season_number (our unique constraint)
	season, err := s.seriesStorage.GetSeason(ctx,
		table.Season.SeriesID.EQ(sqlite.Int64(seriesID)).
			AND(table.Season.SeasonNumber.EQ(sqlite.Int32(seasonNumber))))

	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return 0, err
	}

	if season != nil {
		// Season exists, update metadata link if provided and missing
		if seasonMetadataID != nil && season.SeasonMetadataID == nil {
			err = s.seriesStorage.LinkSeasonMetadata(ctx, int64(season.ID), *seasonMetadataID)
			if err != nil {
				log.Error("failed to link season metadata", zap.Error(err))
				return 0, err
			}
			log.Debug("linked existing season to metadata",
				zap.Int64("season_id", int64(season.ID)),
				zap.Int32("season_metadata_id", *seasonMetadataID))
		}
		return int64(season.ID), nil
	}

	// Season doesn't exist, create new one with metadata link if available
	newSeason := storage.Season{
		Season: model.Season{
			SeriesID:         int32(seriesID),
			SeasonNumber:     seasonNumber,
			SeasonMetadataID: seasonMetadataID,
			Monitored:        0,
		},
	}

	seasonID, err := s.seriesStorage.CreateSeason(ctx, newSeason, initialState)
	if err != nil {
		return 0, err
	}

	log.Debug("created new season",
		zap.Int64("season_id", seasonID),
		zap.String("initial_state", string(initialState)),
		zap.Any("season_metadata_id", seasonMetadataID))

	return seasonID, nil
}

// upsertEpisodeFile creates a new episode file or updates the absolute path if it has changed.
func (s *SeriesService) upsertEpisodeFile(ctx context.Context, discoveredFile library.EpisodeFile, matchedID int32, matchedPath string) error {
	log := logger.FromCtx(ctx).With(
		zap.String("relative_path", discoveredFile.RelativePath),
		zap.String("discovered_absolute_path", discoveredFile.AbsolutePath),
		zap.Int32("matched_id", matchedID))

	if matchedID == 0 {
		ef := modelEpisodeFile(discoveredFile)
		log.Debug("creating new episode file", zap.Int64("size", discoveredFile.Size))
		_, err := s.seriesStorage.CreateEpisodeFile(ctx, ef)
		if err != nil {
			return fmt.Errorf("couldn't store episode file: %w", err)
		}
		log.Debug("successfully created episode file")
		return nil
	}

	log.Debug("episode file matched existing record", zap.String("matched_path", matchedPath))

	if matchedPath == "" {
		log.Debug("skipping update: matched path is empty")
		return nil
	}

	if discoveredFile.AbsolutePath == "" {
		log.Debug("skipping update: discovered absolute path is empty")
		return nil
	}

	if strings.EqualFold(matchedPath, discoveredFile.AbsolutePath) {
		log.Debug("absolute paths match, no update needed")
		return nil
	}

	log.Info("updating episode file absolute path",
		zap.String("old_absolute_path", matchedPath),
		zap.String("new_absolute_path", discoveredFile.AbsolutePath))

	existingFile, err := s.seriesStorage.GetEpisodeFile(ctx, matchedID)
	if err != nil {
		return fmt.Errorf("failed to get episode file for update: %w", err)
	}

	existingFile.OriginalFilePath = &discoveredFile.AbsolutePath
	err = s.seriesStorage.UpdateEpisodeFile(ctx, matchedID, *existingFile)
	if err != nil {
		return fmt.Errorf("failed to update episode file absolute path: %w", err)
	}

	log.Debug("successfully updated episode file absolute path")

	return nil
}

// modelEpisodeFile creates a model.EpisodeFile from a library EpisodeFile.
func modelEpisodeFile(df library.EpisodeFile) model.EpisodeFile {
	return model.EpisodeFile{
		OriginalFilePath: &df.AbsolutePath,
		RelativePath:     &df.RelativePath,
		Size:             df.Size,
	}
}

// matchEpisodeFile attempts to match a discovered file against existing tracked episode files.
func matchEpisodeFile(discoveredFile library.EpisodeFile, episodeFiles []*model.EpisodeFile, log *zap.SugaredLogger) (int32, string) {
	for _, ef := range episodeFiles {
		if ef == nil {
			continue
		}

		hasRelativePath := ef.RelativePath != nil && *ef.RelativePath != ""
		if hasRelativePath && strings.EqualFold(*ef.RelativePath, discoveredFile.RelativePath) {
			log.Debug("discovered file relative path matches monitored episode file relative path",
				zap.String("discovered file relative path", discoveredFile.RelativePath),
				zap.String("monitored file relative path", *ef.RelativePath))
			var path string
			if ef.OriginalFilePath != nil {
				path = *ef.OriginalFilePath
			}
			return ef.ID, path
		}

		hasAbsolutePath := ef.OriginalFilePath != nil && *ef.OriginalFilePath != ""
		hasDiscoveredAbsolutePath := discoveredFile.AbsolutePath != ""
		if hasAbsolutePath && hasDiscoveredAbsolutePath && strings.EqualFold(*ef.OriginalFilePath, discoveredFile.AbsolutePath) {
			log.Debug("discovered file absolute path matches monitored episode file original path",
				zap.String("discovered file absolute path", discoveredFile.AbsolutePath),
				zap.String("monitored file original path", *ef.OriginalFilePath))
			return ef.ID, *ef.OriginalFilePath
		}
	}

	return 0, ""
}

