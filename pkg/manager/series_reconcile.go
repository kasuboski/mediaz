package manager

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/kasuboski/mediaz/pkg/download"
	"github.com/kasuboski/mediaz/pkg/indexer"
	"github.com/kasuboski/mediaz/pkg/io"
	"github.com/kasuboski/mediaz/pkg/library"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/ptr"
	"github.com/kasuboski/mediaz/pkg/storage"
	sqlcdb "github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"go.uber.org/zap"
)

func (m MediaManager) ReconcileSeries(ctx context.Context) error {
	log := logger.FromCtx(ctx)

	dcs, err := m.ListDownloadClients(ctx)
	if err != nil {
		return err
	}

	snapshot := newReconcileSnapshot(make([]sqlcdb.Indexer, 0), dcs)

	var allErrors error

	err = m.ReconcileMissingSeries(ctx, snapshot)
	if err != nil {
		log.Error("failed to reconcile missing series", zap.Error(err))
		allErrors = errors.Join(allErrors, err)
	}

	err = m.ReconcileDownloadingSeries(ctx, snapshot)
	if err != nil {
		log.Error("failed to reconcile downloading series", zap.Error(err))
		allErrors = errors.Join(allErrors, err)
	}

	err = m.ReconcileContinuingSeries(ctx, snapshot)
	if err != nil {
		log.Error("failed to reconcile continuing series", zap.Error(err))
		allErrors = errors.Join(allErrors, err)
	}

	err = m.ReconcileCompletedSeasons(ctx)
	if err != nil {
		log.Error("failed to reconcile completed seasons", zap.Error(err))
		allErrors = errors.Join(allErrors, err)
	}

	err = m.ReconcileCompletedSeries(ctx)
	if err != nil {
		log.Error("failed to reconcile completed series", zap.Error(err))
		allErrors = errors.Join(allErrors, err)
	}

	err = m.ReconcileDiscoveredEpisodes(ctx, snapshot)
	if err != nil {
		log.Error("failed to reconcile discovered episodes", zap.Error(err))
		allErrors = errors.Join(allErrors, err)
	}

	return allErrors
}

func (m MediaManager) ReconcileMissingSeries(ctx context.Context, snapshot *ReconcileSnapshot) error {
	log := logger.FromCtx(ctx)
	log.Debug("starting missing series reconciliation")

	if snapshot == nil {
		log.Warn("snapshot is nil, skipping reconcile")
		return nil
	}

	indexers, err := m.listIndexersInternal(ctx)
	if err != nil {
		return err
	}

	log.Debug("listed indexers", zap.Int("count", len(indexers)))
	if len(indexers) == 0 {
		log.Warn("skipping missing series reconciliation: no indexers available")
		return nil
	}

	snapshot = newReconcileSnapshot(indexers, snapshot.GetDownloadClients())

	monitored := true
	series, err := m.seriesStorage.ListSeries(ctx, storage.SeriesFilter{
		States:    []storage.SeriesState{storage.SeriesStateMissing},
		Monitored: &monitored,
	})
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			log.Error("failed to list missing series", zap.Error(err))
			return fmt.Errorf("couldn't list missing series: %w", err)
		}

		log.Debug("no missing series found")
		return nil
	}

	for _, s := range series {
		log.Debug("reconciling series", zap.Any("series", s.Series.ID))
		err = m.reconcileMissingSeries(ctx, s, snapshot)
		if err != nil {
			log.Error("failed to reconcile series", zap.Error(err))
			continue
		}

		log.Debug("successfully reconciled series", zap.Any("series", s.Series.ID))
	}

	return nil
}

func (m MediaManager) ReconcileContinuingSeries(ctx context.Context, snapshot *ReconcileSnapshot) error {
	log := logger.FromCtx(ctx)
	log.Debug("starting continuing series reconciliation")

	if snapshot == nil {
		log.Warn("snapshot is nil, skipping reconcile")
		return nil
	}

	// Process all continuing series (monitored and unmonitored) to refresh metadata
	// New episodes will inherit the series' monitored status, so only monitored episodes will be searched/downloaded
	series, err := m.seriesStorage.ListSeries(ctx, storage.SeriesFilter{
		States: []storage.SeriesState{storage.SeriesStateContinuing, storage.SeriesStateDownloading},
	})
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			log.Error("failed to list continuing series", zap.Error(err))
			return fmt.Errorf("couldn't list continuing series: %w", err)
		}

		log.Debug("no continuing series found")
		return nil
	}

	for _, s := range series {
		log.Debug("reconciling continuing series", zap.Any("series", s.Series.ID))
		err = m.reconcileContinuingSeries(ctx, s, snapshot)
		if err != nil {
			log.Error("failed to reconcile continuing series", zap.Error(err))
			continue
		}

		log.Debug("successfully reconciled continuing series", zap.Any("series", s.Series.ID))
	}

	return nil
}

func (m MediaManager) reconcileContinuingSeries(ctx context.Context, series sqlcdb.ListSeriesRow, snapshot *ReconcileSnapshot) error {
	log := logger.FromCtx(ctx)

	// Process both monitored and unmonitored series for metadata refresh
	// The series' monitored status will be inherited by new episodes

	seriesMetadata, err := m.seriesMetaStorage.GetSeriesMetadataByID(ctx, series.Series.SeriesMetadataID.Int64)
	if err != nil {
		log.Debug("failed to find series metadata", zap.Int64("meta_id", series.Series.SeriesMetadataID.Int64))
		return err
	}

	err = m.refreshSeriesEpisodes(ctx, series.Series, seriesMetadata, snapshot)
	if err != nil {
		log.Warn("failed to refresh series episodes", zap.Error(err))
		return err
	}

	log.Debug("successfully reconciled continuing series - refreshed episodes from TMDB",
		zap.Any("series", series.Series.ID))

	return nil
}

// refreshSeriesEpisodes fetches the latest episode metadata from TMDB and creates any new episodes
func (m MediaManager) refreshSeriesEpisodes(ctx context.Context, series sqlcdb.Series, seriesMetadata sqlcdb.SeriesMetadatum, snapshot *ReconcileSnapshot) error {
	log := logger.FromCtx(ctx)

	_, err := m.RefreshSeriesMetadataFromTMDB(ctx, seriesMetadata.TmdbID)
	if err != nil {
		log.Debug("failed to refresh series metadata", zap.Error(err))
		return err
	}

	seriesIDInt := series.ID
	existingSeasons, err := m.seriesStorage.ListSeasons(ctx, storage.SeasonFilter{SeriesID: &seriesIDInt})
	if err != nil {
		log.Error("failed to list existing seasons", zap.Error(err))
		return err
	}

	existingSeasonNumbers := make(map[int64]int64)
	existingSeasonsWithoutMetadata := make([]sqlcdb.ListSeasonsRow, 0)

	for _, season := range existingSeasons {
		if !season.Season.SeasonMetadataID.Valid {
			// Track seasons without metadata for later linking
			existingSeasonsWithoutMetadata = append(existingSeasonsWithoutMetadata, season)
			continue
		}

		seasonMetadata, err := m.seriesMetaStorage.GetSeasonMetadataByID(ctx, int64(season.Season.SeasonMetadataID.Int64))
		if err != nil {
			continue
		}

		existingSeasonNumbers[seasonMetadata.Number] = int64(season.Season.ID)
	}

	allSeasonMetadata, err := m.seriesMetaStorage.ListSeasonMetadataBySeriesMetadataID(ctx, seriesMetadata.ID)
	if err != nil {
		log.Error("failed to list season metadata", zap.Error(err))
		return err
	}

	for _, seasonMeta := range allSeasonMetadata {
		var seasonID int64
		var exists bool

		if seasonID, exists = existingSeasonNumbers[seasonMeta.Number]; !exists {
			// Use unified season creation to prevent duplicates
			seasonID, err = m.getOrCreateSeason(ctx, series.ID, seasonMeta.Number, ptr.To(seasonMeta.ID), storage.SeasonStateMissing)
			if err != nil {
				log.Error("failed to create new season", zap.Error(err))
				continue
			}

			log.Debug("created new season for continuing series",
				zap.Any("series", series.ID),
				zap.Int64("season_number", seasonMeta.Number))
		}

		existingEpisodes, err := m.seriesStorage.ListEpisodes(ctx, storage.EpisodeFilter{SeasonID: &seasonID})
		if err != nil {
			log.Error("failed to list existing episodes", zap.Error(err))
			continue
		}

		existingEpisodeNumbers := make(map[int64]bool)
		for _, episode := range existingEpisodes {
			existingEpisodeNumbers[episode.Episode.EpisodeNumber] = true
		}

		// Get season to access its metadata ID
		seasonEntity, err := m.seriesStorage.GetSeasonByID(ctx, seasonID)
		if err != nil || !seasonEntity.Season.SeasonMetadataID.Valid {
			log.Error("failed to get season or season has no metadata", zap.Error(err))
			continue
		}

		episodeMetadataList, err := m.seriesMetaStorage.ListEpisodeMetadataBySeasonMetadataID(ctx, seasonEntity.Season.SeasonMetadataID.Int64)
		if err != nil {
			log.Error("failed to list episode metadata", zap.Error(err))
			continue
		}

		for _, episodeMeta := range episodeMetadataList {
			if !existingEpisodeNumbers[episodeMeta.Number] {
				// New episodes inherit the series' monitored status
				// This ensures unmonitored series don't trigger searches/downloads
				episode := sqlcdb.Episode{
					EpisodeMetadataID: sql.NullInt64{Int64: episodeMeta.ID, Valid: true},
					SeasonID:          seasonID,
					Monitored:         series.Monitored,
					EpisodeNumber:     episodeMeta.Number,
				}

				episodeState := storage.EpisodeStateMissing
				if !episodeMeta.AirDate.Valid || !isReleased(snapshot.time, &episodeMeta.AirDate.Time) {
					episodeState = storage.EpisodeStateUnreleased
				}

				_, err := m.seriesStorage.CreateEpisode(ctx, episode, episodeState)
				if err != nil {
					log.Error("failed to create new episode",
						zap.Error(err),
						zap.Int64("episode_number", episodeMeta.Number))
					continue
				}

				log.Debug("created new episode for continuing series",
					zap.Any("series", series.ID),
					zap.Int64("season_number", seasonMeta.Number),
					zap.Int64("episode_number", episodeMeta.Number),
					zap.String("state", string(episodeState)))
			}
		}
	}

	// Link existing seasons without metadata to the appropriate metadata
	for _, season := range existingSeasonsWithoutMetadata {
		// Use the season number stored in the database (set during indexing)
		seasonNumber := season.Season.SeasonNumber

		// Find matching season metadata
		for _, seasonMeta := range allSeasonMetadata {
			if seasonMeta.Number == seasonNumber {
				err := m.seriesStorage.LinkSeasonMetadata(ctx, int64(season.Season.ID), seasonMeta.ID)
				if err != nil {
					log.Error("failed to link season metadata", zap.Error(err))
				} else {
					log.Debug("linked existing season to metadata",
						zap.Int64("season_id", season.Season.ID),
						zap.Int64("season_number", seasonNumber),
						zap.Int64("metadata_id", seasonMeta.ID))
					// Update the existingSeasonNumbers map
					existingSeasonNumbers[seasonNumber] = int64(season.Season.ID)
				}
				break
			}
		}
	}

	return nil
}

func (m MediaManager) reconcileMissingSeries(ctx context.Context, series sqlcdb.ListSeriesRow, snapshot *ReconcileSnapshot) error {
	log := logger.FromCtx(ctx)

	if series.Series.Monitored == 0 {
		log.Debug("series is not monitored, skipping reconcile")
		return nil
	}

	qualityProfile, err := m.qualityService.ListQualityProfileRows(ctx, series.Series.QualityProfileID)
	if err != nil {
		log.Warn("failed to find series qualityprofile", zap.Int64("quality_id", series.Series.QualityProfileID))
		return err
	}

	seriesMetadata, err := m.seriesMetaStorage.GetSeriesMetadataByID(ctx, int64(series.Series.SeriesMetadataID.Int64))
	if err != nil {
		log.Debug("failed to find series metadata", zap.Int64("meta_id", series.Series.SeriesMetadataID.Int64))
		return err
	}

	releases, err := m.indexerService.SearchIndexers(ctx, snapshot.GetIndexerIDs(), TVCategories, indexer.SearchOptions{
		Query: seriesMetadata.Title,
		Type:  ptr.To(indexer.TypeTV),
	})
	if err != nil {
		log.Warn("some indexer sources failed during series search", zap.Any("indexers", snapshot.GetIndexerIDs()), zap.Error(err))
		if len(releases) == 0 {
			return err
		}
	}

	slices.SortFunc(releases, sortReleaseFunc())

	seriesIDInt := int64(series.Series.ID)
	missingMonitoredSeasons, err := m.seriesStorage.ListSeasons(ctx, storage.SeasonFilter{
		SeriesID: &seriesIDInt,
		States:   []storage.SeasonState{storage.SeasonStateMissing},
	})
	if err != nil {
		log.Error("failed to list missing seasons", zap.Error(err))
		return fmt.Errorf("couldn't list missing seasons: %w", err)
	}

	// the typed season filter cannot express monitored=1, so apply it here
	seasons := make([]sqlcdb.ListSeasonsRow, 0, len(missingMonitoredSeasons))
	for _, season := range missingMonitoredSeasons {
		if season.Season.Monitored == 1 {
			seasons = append(seasons, season)
		}
	}

	if len(seasons) == 0 {
		log.Debug("no seasons found, skipping reconcile")
		return nil
	}

	seasonIDs := make([]int64, 0, len(seasons))
	seasonMetadataIDs := make([]int64, 0, len(seasons))
	for _, season := range seasons {
		seasonIDs = append(seasonIDs, int64(season.Season.ID))
		if season.Season.SeasonMetadataID.Valid {
			seasonMetadataIDs = append(seasonMetadataIDs, season.Season.SeasonMetadataID.Int64)
		}
	}
	batchEpisodes, err := m.seriesStorage.ListEpisodesBySeasonIDs(ctx, seasonIDs)
	if err != nil {
		log.Error("failed to list episodes for seasons", zap.Error(err))
		return fmt.Errorf("couldn't list episodes for seasons: %w", err)
	}
	episodesBySeason := make(map[int64][]sqlcdb.ListEpisodesRow, len(seasons))
	episodeMetadataIDs := make([]int64, 0)
	for _, e := range batchEpisodes {
		episodesBySeason[e.Episode.SeasonID] = append(episodesBySeason[e.Episode.SeasonID], sqlcdb.ListEpisodesRow{Episode: e.Episode, State: e.State, DownloadID: e.DownloadID, DownloadClientID: e.DownloadClientID, IsEntireSeasonDownload: e.IsEntireSeasonDownload})
		if e.Episode.EpisodeMetadataID.Valid {
			episodeMetadataIDs = append(episodeMetadataIDs, e.Episode.EpisodeMetadataID.Int64)
		}
	}
	seasonMetadata, err := m.seriesMetaStorage.ListSeasonMetadataByIDs(ctx, seasonMetadataIDs)
	if err != nil {
		return fmt.Errorf("couldn't list season metadata: %w", err)
	}
	seasonMetadataByID := make(map[int64]sqlcdb.SeasonMetadatum, len(seasonMetadata))
	for _, meta := range seasonMetadata {
		seasonMetadataByID[meta.ID] = meta
	}
	episodeMetadata, err := m.seriesMetaStorage.ListEpisodeMetadataByIDs(ctx, episodeMetadataIDs)
	if err != nil {
		return fmt.Errorf("couldn't list episode metadata: %w", err)
	}
	episodeMetadataByID := make(map[int64]sqlcdb.EpisodeMetadatum, len(episodeMetadata))
	for _, meta := range episodeMetadata {
		episodeMetadataByID[meta.ID] = meta
	}

	// Check if all missing seasons only have unreleased episodes.
	allMissingSeasonsUnreleased := true
	for _, season := range seasons {
		for _, e := range episodesBySeason[int64(season.Season.ID)] {
			if e.State == string(storage.EpisodeStateMissing) && e.Episode.EpisodeMetadataID.Valid {
				if meta, ok := episodeMetadataByID[e.Episode.EpisodeMetadataID.Int64]; ok && meta.AirDate.Valid && isReleased(snapshot.time, &meta.AirDate.Time) {
					allMissingSeasonsUnreleased = false
					break
				}
			}
		}
		if !allMissingSeasonsUnreleased {
			break
		}
	}

	if allMissingSeasonsUnreleased {
		log.Info("all missing seasons only have unreleased episodes, reverting series state to continuing")
		err := m.updateSeriesState(ctx, int64(series.Series.ID), storage.SeriesStateContinuing, nil)
		if err != nil {
			log.Error("failed to update series state to continuing", zap.Error(err))
			return err
		}
		return nil
	}

	identity := seriesReleaseIdentity(seriesMetadata)
	for _, s := range seasons {
		log.Debug("reconciling season", zap.Any("season", s.Season.ID))
		err = m.reconcileMissingSeasonWithMetadata(ctx, seriesMetadata.Title, s, episodesBySeason, seasonMetadataByID, episodeMetadataByID, snapshot, qualityProfile, releases, identity)
		if err != nil {
			log.Error("failed to reconcile missing season", zap.Error(err))
			continue
		}
		log.Debug("successfully reconciled season", zap.Any("season", s.Season.ID))
	}

	return nil
}

func (m MediaManager) reconcileMissingSeason(ctx context.Context, seriesTitle string, season sqlcdb.ListSeasonsRow, snapshot *ReconcileSnapshot, qualityProfile []sqlcdb.ListQualityProfileRowsRow, releases []*prowlarr.ReleaseResource) error {
	return m.reconcileMissingSeasonWithMetadata(ctx, seriesTitle, season, nil, nil, nil, snapshot, qualityProfile, releases)
}

func (m MediaManager) reconcileMissingSeasonWithMetadata(ctx context.Context, seriesTitle string, season sqlcdb.ListSeasonsRow, episodesBySeason map[int64][]sqlcdb.ListEpisodesRow, seasonMetadataByID map[int64]sqlcdb.SeasonMetadatum, episodeMetadataByID map[int64]sqlcdb.EpisodeMetadatum, snapshot *ReconcileSnapshot, qualityProfile []sqlcdb.ListQualityProfileRowsRow, releases []*prowlarr.ReleaseResource, identities ...SeriesReleaseFilterParams) error {
	log := logger.FromCtx(ctx)
	log = log.With("reconcile loop", "missing series")
	metadata, ok := seasonMetadataByID[season.Season.SeasonMetadataID.Int64]
	var err error
	if seasonMetadataByID == nil {
		metadata, err = m.seriesMetaStorage.GetSeasonMetadataByID(ctx, season.Season.SeasonMetadataID.Int64)
		if err != nil {
			return err
		}
	} else if !ok {
		return fmt.Errorf("season metadata %d not preloaded", season.Season.SeasonMetadataID.Int64)
	}
	seasonIDInt := int64(season.Season.ID)
	episodes, ok := episodesBySeason[seasonIDInt]
	if episodesBySeason == nil {
		episodes, err = m.seriesStorage.ListEpisodes(ctx, storage.EpisodeFilter{SeasonID: &seasonIDInt})
		if err != nil {
			return fmt.Errorf("couldn't list missing episodes: %w", err)
		}
	}

	if len(episodes) == 0 {
		log.Debug("no episodes found, skipping reconcile")
		return nil
	}

	var allMissing = true
	var missingEpisodes []sqlcdb.ListEpisodesRow
	var allMissingUnreleased = true
	for _, e := range episodes {
		switch storage.EpisodeState(e.State) {
		case storage.EpisodeStateMissing:
			missingEpisodes = append(missingEpisodes, e)
			// Check if episode is unreleased
			if e.Episode.EpisodeMetadataID.Valid {
				var episodeMetadata sqlcdb.EpisodeMetadatum
				var metadataErr error
				if episodeMetadataByID == nil {
					episodeMetadata, metadataErr = m.seriesMetaStorage.GetEpisodeMetadataByID(ctx, int64(e.Episode.EpisodeMetadataID.Int64))
				} else {
					var found bool
					episodeMetadata, found = episodeMetadataByID[e.Episode.EpisodeMetadataID.Int64]
					if !found {
						continue
					}
				}
				if metadataErr == nil && episodeMetadata.AirDate.Valid && isReleased(snapshot.time, &episodeMetadata.AirDate.Time) {
					allMissingUnreleased = false
				}
			}
			continue
		default:
			allMissing = false
		}
	}

	log.Debug("found missing episodes", zap.Int("count", len(missingEpisodes)))

	if !allMissing {
		return m.reconcileMissingEpisodesWithMetadata(ctx, seriesTitle, season.Season.ID, metadata.Number, missingEpisodes, episodeMetadataByID, snapshot, qualityProfile, releases, identities...)
	}

	// If all missing episodes are unreleased, revert season state to continuing
	if allMissing && allMissingUnreleased {
		log.Info("all missing episodes are unreleased, reverting season state to continuing")
		err := m.updateSeasonState(ctx, int64(season.Season.ID), storage.SeasonStateContinuing, nil)
		if err != nil {
			log.Error("failed to update season state to continuing", zap.Error(err))
			return err
		}
		return nil
	}

	// Fetch episode metadata for missing episodes to calculate runtime
	var missingEpisodesMetadata []*sqlcdb.EpisodeMetadatum
	for _, e := range missingEpisodes {
		if e.Episode.EpisodeMetadataID.Valid {
			var episodeMetadata sqlcdb.EpisodeMetadatum
			var metadataErr error
			if episodeMetadataByID == nil {
				episodeMetadata, metadataErr = m.seriesMetaStorage.GetEpisodeMetadataByID(ctx, int64(e.Episode.EpisodeMetadataID.Int64))
			} else {
				var found bool
				episodeMetadata, found = episodeMetadataByID[e.Episode.EpisodeMetadataID.Int64]
				if !found {
					continue
				}
			}
			if metadataErr != nil {
				log.Warn("failed to get episode metadata for runtime calculation", zap.Error(metadataErr))
				continue
			}
			missingEpisodesMetadata = append(missingEpisodesMetadata, &episodeMetadata)
		}
	}

	runtime := getSeasonRuntime(missingEpisodesMetadata, len(episodes))
	log.Debug("considering releases for season pack", zap.Int("count", len(releases)))

	var chosenSeasonPackRelease *prowlarr.ReleaseResource
	seasonParams := SeriesReleaseFilterParams{
		Title:        seriesTitle,
		SeasonNumber: metadata.Number,
		Runtime:      runtime,
	}
	if len(identities) > 0 {
		seasonParams.TmdbID = identities[0].TmdbID
		seasonParams.IMDbID = identities[0].IMDbID
		seasonParams.TVDBID = identities[0].TVDBID
		seasonParams.Year = identities[0].Year
	}
	for _, r := range releases {
		if RejectSeasonReleaseFunc(ctx, seasonParams, qualityProfile, snapshot.GetProtocols())(r) {
			continue
		}

		chosenSeasonPackRelease = r
		break
	}

	if chosenSeasonPackRelease == nil {
		log.Debug("no season pack releases found, defaulting to individual episodes")
		return m.reconcileMissingEpisodesWithMetadata(ctx, seriesTitle, season.Season.ID, metadata.Number, missingEpisodes, episodeMetadataByID, snapshot, qualityProfile, releases, identities...)
	}

	log.Info("found season pack release", zap.Any("title", chosenSeasonPackRelease.Title), zap.String("proto", string(*chosenSeasonPackRelease.Protocol)))

	clientID, status, err := m.requestReleaseDownload(ctx, snapshot, chosenSeasonPackRelease)
	if err != nil {
		log.Debug("failed to request episode release download", zap.Error(err))
		return err
	}

	var allUpdated = true
	updatedAny := false
	for _, e := range missingEpisodes {
		updateEpisodeState := m.updateEpisodeState
		if episodeMetadataByID != nil {
			updateEpisodeState = m.updateEpisodeStateWithoutAggregate
		}
		err = updateEpisodeState(ctx, e, storage.EpisodeStateDownloading, &storage.TransitionStateMetadata{
			DownloadID:             &status.ID,
			DownloadClientID:       &clientID,
			IsEntireSeasonDownload: ptr.To(true),
		})
		if err != nil {
			allUpdated = false
			log.Error("failed to update episode state in seasons pack", zap.Error(err))
			continue
		}

		updatedAny = true
		log.Debug("successfully reconciled episode in seasons pack")
	}

	if episodeMetadataByID != nil && updatedAny {
		if err := m.evaluateReconciledSeasonAggregates(ctx, int64(season.Season.ID)); err != nil {
			log.Warn("failed to update aggregates after season pack reconciliation", zap.Error(err))
		}
	}

	if allUpdated {
		return m.updateSeasonState(ctx, int64(season.Season.ID), storage.SeasonStateDownloading, &storage.TransitionStateMetadata{
			DownloadID:             &status.ID,
			DownloadClientID:       &clientID,
			IsEntireSeasonDownload: ptr.To(true),
		})
	}

	return nil
}

func (m MediaManager) reconcileMissingEpisodes(ctx context.Context, seriesTitle string, seasonID int64, seasonNumber int64, episode []sqlcdb.ListEpisodesRow, snapshot *ReconcileSnapshot, qualityProfile []sqlcdb.ListQualityProfileRowsRow, releases []*prowlarr.ReleaseResource) error {
	return m.reconcileMissingEpisodesWithMetadata(ctx, seriesTitle, seasonID, seasonNumber, episode, nil, snapshot, qualityProfile, releases)
}

func (m MediaManager) reconcileMissingEpisodesWithMetadata(ctx context.Context, seriesTitle string, seasonID int64, seasonNumber int64, episode []sqlcdb.ListEpisodesRow, episodeMetadataByID map[int64]sqlcdb.EpisodeMetadatum, snapshot *ReconcileSnapshot, qualityProfile []sqlcdb.ListQualityProfileRowsRow, releases []*prowlarr.ReleaseResource, identities ...SeriesReleaseFilterParams) error {
	log := logger.FromCtx(ctx)

	var allUpdated = true
	updatedAny := false
	for _, e := range episode {
		updated, err := m.reconcileMissingEpisodeWithMetadata(ctx, seriesTitle, seasonNumber, e, episodeMetadataByID, snapshot, qualityProfile, releases, identities...)
		if !updated {
			allUpdated = false
		} else {
			updatedAny = true
		}
		if err != nil {
			log.Error("failed to reconcile missing episode", zap.Error(err))
			continue
		}
	}

	if episodeMetadataByID != nil {
		if updatedAny {
			return m.evaluateReconciledSeasonAggregates(ctx, int64(seasonID))
		}
		return nil
	}

	if !allUpdated {
		if updatedAny {
			return m.evaluateReconciledSeasonAggregates(ctx, int64(seasonID))
		}
		return nil
	}

	return m.updateSeasonState(ctx, int64(seasonID), storage.SeasonStateDownloading, nil)
}

func (m MediaManager) reconcileMissingEpisode(ctx context.Context, seriesTitle string, seasonNumber int64, episode sqlcdb.ListEpisodesRow, snapshot *ReconcileSnapshot, qualityProfile []sqlcdb.ListQualityProfileRowsRow, releases []*prowlarr.ReleaseResource) (bool, error) {
	return m.reconcileMissingEpisodeWithMetadata(ctx, seriesTitle, seasonNumber, episode, nil, snapshot, qualityProfile, releases)
}

func (m MediaManager) reconcileMissingEpisodeWithMetadata(ctx context.Context, seriesTitle string, seasonNumber int64, episode sqlcdb.ListEpisodesRow, episodeMetadataByID map[int64]sqlcdb.EpisodeMetadatum, snapshot *ReconcileSnapshot, qualityProfile []sqlcdb.ListQualityProfileRowsRow, releases []*prowlarr.ReleaseResource, identities ...SeriesReleaseFilterParams) (bool, error) {
	log := logger.FromCtx(ctx)

	if snapshot == nil {
		log.Warn("snapshot is nil, skipping reconcile")
		return false, nil
	}

	var episodeMetadata sqlcdb.EpisodeMetadatum
	var err error
	if episodeMetadataByID == nil {
		episodeMetadata, err = m.seriesMetaStorage.GetEpisodeMetadataByID(ctx, int64(episode.Episode.EpisodeMetadataID.Int64))
	} else {
		var found bool
		episodeMetadata, found = episodeMetadataByID[episode.Episode.EpisodeMetadataID.Int64]
		if !found {
			return false, fmt.Errorf("episode metadata %d not preloaded", episode.Episode.EpisodeMetadataID.Int64)
		}
	}
	if err != nil {
		log.Debug("failed to find episode metadata", zap.Int64("meta_id", episode.Episode.EpisodeMetadataID.Int64))
		return false, err
	}

	if !episodeMetadata.AirDate.Valid || !isReleased(snapshot.time, &episodeMetadata.AirDate.Time) {
		log.Debug("episode is not yet released", zap.Any("air_date", episodeMetadata.AirDate))
		return false, nil
	}

	if !episodeMetadata.Runtime.Valid {
		log.Warn("episode runtime is nil, skipping reconcile")
		return false, nil
	}

	var chosenRelease *prowlarr.ReleaseResource
	episodeParams := SeriesReleaseFilterParams{
		Title:         seriesTitle,
		SeasonNumber:  seasonNumber,
		EpisodeNumber: episodeMetadata.Number,
		Runtime:       episodeMetadata.Runtime.Int64,
	}
	if len(identities) > 0 {
		episodeParams.TmdbID = identities[0].TmdbID
		episodeParams.IMDbID = identities[0].IMDbID
		episodeParams.TVDBID = identities[0].TVDBID
		episodeParams.Year = identities[0].Year
	}
	for _, r := range releases {
		if RejectEpisodeReleaseFunc(ctx, episodeParams, qualityProfile, snapshot.GetProtocols())(r) {
			continue
		}
		chosenRelease = r
	}

	if chosenRelease == nil {
		log.Debug("no valid releases found for episode, skipping reconcile")
		return false, nil
	}

	log.Info("found release", zap.Any("title", chosenRelease.Title), zap.String("proto", string(*chosenRelease.Protocol)))

	clientID, status, err := m.requestReleaseDownload(ctx, snapshot, chosenRelease)
	if err != nil {
		log.Debug("failed to request episode release download", zap.Error(err))
		return false, err
	}

	updateEpisodeState := m.updateEpisodeState
	if episodeMetadataByID != nil {
		updateEpisodeState = m.updateEpisodeStateWithoutAggregate
	}
	err = updateEpisodeState(ctx, episode, storage.EpisodeStateDownloading, &storage.TransitionStateMetadata{
		DownloadID:       &status.ID,
		DownloadClientID: &clientID,
	})
	if err != nil {
		log.Debug("failed to update episode state", zap.Error(err))
		return false, err
	}

	log.Debug("successfully reconciled episode")
	return true, nil
}

func (m MediaManager) updateEpisodeState(ctx context.Context, episode sqlcdb.ListEpisodesRow, state storage.EpisodeState, metadata *storage.TransitionStateMetadata) error {
	return m.updateEpisodeStateWithAggregate(ctx, episode, state, metadata, true)
}

func (m MediaManager) updateEpisodeStateWithoutAggregate(ctx context.Context, episode sqlcdb.ListEpisodesRow, state storage.EpisodeState, metadata *storage.TransitionStateMetadata) error {
	return m.updateEpisodeStateWithAggregate(ctx, episode, state, metadata, false)
}

func (m MediaManager) updateEpisodeStateWithAggregate(ctx context.Context, episode sqlcdb.ListEpisodesRow, state storage.EpisodeState, metadata *storage.TransitionStateMetadata, aggregate bool) error {
	log := logger.FromCtx(ctx).With("episode id", episode.Episode.ID, "from state", episode.State, "to state", state)

	if episode.State == string(state) {
		log.Debug("episode already in target state, skipping update")
		return nil
	}

	err := m.seriesStorage.UpdateEpisodeState(ctx, int64(episode.Episode.ID), state, metadata)
	if err != nil {
		log.Error("failed to update episode state", zap.Error(err))
		return err
	}

	log.Info("successfully updated episode state")

	if state != storage.EpisodeStateDownloaded && state != storage.EpisodeStateDownloading && state != storage.EpisodeStateCompleted || !aggregate {
		return nil
	}

	if err := m.evaluateAndUpdateSeasonState(ctx, episode.Episode.SeasonID); err != nil {
		log.Warn("failed to update season state after episode state change", zap.Error(err))
	}

	season, err := m.seriesStorage.GetSeasonByID(ctx, int64(episode.Episode.SeasonID))
	if err != nil {
		log.Warn("failed to get season for series state update", zap.Error(err))
		return nil
	}

	if err := m.evaluateAndUpdateSeriesState(ctx, season.Season.SeriesID); err != nil {
		log.Warn("failed to update series state after episode state change", zap.Error(err))
	}

	return nil
}

func (m MediaManager) evaluateReconciledSeasonAggregates(ctx context.Context, seasonID int64) error {
	if err := m.evaluateAndUpdateSeasonState(ctx, seasonID); err != nil {
		return err
	}
	season, err := m.seriesStorage.GetSeasonByID(ctx, seasonID)
	if err != nil {
		return err
	}
	return m.evaluateAndUpdateSeriesState(ctx, season.Season.SeriesID)
}

func (m MediaManager) updateSeasonState(ctx context.Context, id int64, state storage.SeasonState, metadata *storage.TransitionStateMetadata) error {
	log := logger.FromCtx(ctx).With("season id", id, "to state", state)

	season, err := m.seriesStorage.GetSeasonByID(ctx, id)
	if err != nil {
		log.Error("failed to get season for state check", zap.Error(err))
		return err
	}

	if season.State.String == string(state) {
		log.Debug("season already in target state, skipping update")
		return nil
	}

	err = m.seriesStorage.UpdateSeasonState(ctx, id, state, metadata)
	if err != nil {
		log.Error("failed to update season state", zap.Error(err))
		return err
	}

	log.Info("successfully updated season state")

	if state != storage.SeasonStateCompleted && state != storage.SeasonStateDownloading {
		return nil
	}

	if err := m.evaluateAndUpdateSeriesState(ctx, season.Season.SeriesID); err != nil {
		log.Warn("failed to update series state after season state change", zap.Error(err))
	}

	return nil
}

// updateSeriesState updates the series state and handles cascading updates
func (m MediaManager) updateSeriesState(ctx context.Context, id int64, state storage.SeriesState, metadata *storage.TransitionStateMetadata) error {
	log := logger.FromCtx(ctx).With("series id", id, "to state", state)
	err := m.seriesStorage.UpdateSeriesState(ctx, id, state, metadata)
	if err != nil {
		log.Error("failed to update series state", zap.Error(err))
		return err
	}

	log.Info("successfully updated series state")
	return nil
}

// evaluateAndUpdateSeasonState evaluates all episodes in a season and updates the season state accordingly
func (m MediaManager) evaluateAndUpdateSeasonState(ctx context.Context, seasonID int64) error {
	log := logger.FromCtx(ctx).With("season_id", seasonID)

	currentSeason, err := m.seriesStorage.GetSeasonByID(ctx, int64(seasonID))
	if err != nil {
		log.Error("failed to get season for state evaluation", zap.Error(err))
		return err
	}

	seasonIDInt := int64(seasonID)
	episodes, err := m.seriesStorage.ListEpisodes(ctx, storage.EpisodeFilter{SeasonID: &seasonIDInt})
	if err != nil {
		log.Error("failed to list episodes for season state evaluation", zap.Error(err))
		return err
	}

	if len(episodes) == 0 {
		log.Debug("no episodes found for season, keeping current state")
		return nil
	}

	counts, newSeasonState := determineSeasonState(episodes)

	log.Debug("evaluating season state",
		zap.Int("total_episodes", len(episodes)),
		zap.Int("done", counts["done"]),
		zap.Int("downloading", counts["downloading"]),
		zap.Int("missing", counts["missing"]),
		zap.Int("unreleased", counts["unreleased"]),
		zap.String("current_state", string(currentSeason.State.String)),
		zap.String("new_state", string(newSeasonState)))

	if currentSeason.State.String == string(newSeasonState) {
		log.Debug("season already in target state, no update needed")
		return nil
	}

	return m.updateSeasonState(ctx, int64(seasonID), newSeasonState, nil)
}

// evaluateAndUpdateSeriesState evaluates all seasons in a series and updates the series state accordingly
func (m MediaManager) evaluateAndUpdateSeriesState(ctx context.Context, seriesID int64) error {
	log := logger.FromCtx(ctx).With("series_id", seriesID)

	series, err := m.seriesStorage.GetSeriesByID(ctx, int64(seriesID))
	if err != nil {
		log.Error("failed to get series for state evaluation", zap.Error(err))
		return err
	}

	var tmdbStatus string
	if series.Series.SeriesMetadataID.Valid {
		metadata, err := m.seriesMetaStorage.GetSeriesMetadataByID(ctx, int64(series.Series.SeriesMetadataID.Int64))
		if err == nil {
			tmdbStatus = metadata.Status
			log.Debug("retrieved TMDB status", zap.String("status", tmdbStatus))
		}
	}

	seriesIDInt := int64(seriesID)
	seasons, err := m.seriesStorage.ListSeasons(ctx, storage.SeasonFilter{SeriesID: &seriesIDInt})
	if err != nil {
		log.Error("failed to list seasons for series state evaluation", zap.Error(err))
		return err
	}

	if len(seasons) == 0 {
		log.Debug("no seasons found for series, keeping current state")
		return nil
	}

	counts, newSeriesState := determineSeriesState(seasons, tmdbStatus)

	log.Debug("evaluating series state",
		zap.Int("series_id", int(seriesID)),
		zap.Int("total_seasons", len(seasons)),
		zap.Int("completed", counts.completed),
		zap.Int("downloading", counts.downloading),
		zap.Int("missing", counts.missing),
		zap.Int("unreleased", counts.unreleased),
		zap.Int("discovered", counts.discovered),
		zap.Int("continuing", counts.continuing),
		zap.String("tmdb_status", tmdbStatus),
		zap.String("current_state", string(series.State)),
		zap.String("new_state", string(newSeriesState)))

	if series.State == string(newSeriesState) {
		log.Debug("series already in target state, no update needed")
		return nil
	}

	return m.updateSeriesState(ctx, int64(seriesID), newSeriesState, nil)
}

// ReconcileDiscoveredEpisodes processes episodes in the "discovered" state by linking them to their
// corresponding TMDB metadata hierarchy (series -> season -> episode). Discovered episodes have video
// files on disk that need to be associated with the proper metadata before transitioning to "completed" state.
// This runs for both monitored and unmonitored episodes to ensure metadata is populated for display.
func (m MediaManager) ReconcileDiscoveredEpisodes(ctx context.Context, snapshot *ReconcileSnapshot) error {
	log := logger.FromCtx(ctx)
	log.Debug("starting discovered episodes reconciliation")

	// Process all discovered episodes (both monitored and unmonitored) for metadata linking
	// Downloads and searches are handled separately and only for monitored series
	episodes, err := m.seriesStorage.ListEpisodes(ctx, storage.EpisodeFilter{
		States: []storage.EpisodeState{storage.EpisodeStateDiscovered},
	})
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			log.Error("failed to list discovered episodes", zap.Error(err))
			return fmt.Errorf("couldn't list discovered episodes: %w", err)
		}
		log.Debug("no discovered episodes found")
		return nil
	}

	if len(episodes) == 0 {
		return nil
	}
	// Linked series do not need file evidence. Cache either the result or error
	// on first use so an evidence failure cannot block their reconciliation.
	var filesLoaded bool
	var filesByFolder map[string][]string
	var filesErr error
	loadFiles := func() (map[string][]string, error) {
		if !filesLoaded {
			filesLoaded = true
			rows, err := m.seriesStorage.ListEpisodeFiles(ctx)
			if err != nil {
				filesErr = fmt.Errorf("failed to load discovered episode files: %w", err)
			} else {
				filesByFolder = seriesDiscoveryFiles(rows, m.config.Library.TVDir)
			}
		}
		return filesByFolder, filesErr
	}
	attempts := make(map[int64]error)
	for _, episode := range episodes {
		log.Debug("reconciling discovered episode", zap.Any("episode", episode.Episode.ID))
		err = m.reconcileDiscoveredEpisodeWithEvidence(ctx, snapshot, episode, loadFiles, attempts)
		if err != nil {
			log.Error("failed to reconcile discovered episode", zap.Error(err))
			continue
		}
		log.Debug("successfully reconciled discovered episode", zap.Any("episode", episode.Episode.ID))
	}

	return nil
}

func (m MediaManager) reconcileDiscoveredEpisode(ctx context.Context, snapshot *ReconcileSnapshot, episode sqlcdb.ListEpisodesRow) error {
	return m.reconcileDiscoveredEpisodeWithEvidence(ctx, snapshot, episode, nil, nil)
}

func (m MediaManager) reconcileDiscoveredEpisodeWithEvidence(ctx context.Context, snapshot *ReconcileSnapshot, episode sqlcdb.ListEpisodesRow, loadFiles func() (map[string][]string, error), attempts map[int64]error) error {
	log := logger.FromCtx(ctx).With("reconcile loop", "discovered", "episode id", episode.Episode.ID)

	log.Debug("starting episode reconciliation", zap.Int64("episode_id", episode.Episode.ID))

	season, err := m.seriesStorage.GetSeasonByID(ctx, int64(episode.Episode.SeasonID))
	if err != nil {
		log.Error("failed to get season for discovered episode", zap.Error(err))
		return fmt.Errorf("failed to get season: %w", err)
	}

	series, err := m.seriesStorage.GetSeriesByID(ctx, int64(season.Season.SeriesID))
	if err != nil {
		log.Error("failed to get series for discovered episode", zap.Error(err))
		return fmt.Errorf("failed to get series: %w", err)
	}

	if !series.Series.Path.Valid {
		log.Warn("series path is nil, skipping reconcile")
		return nil
	}

	// Step 1: Ensure series has metadata
	if !series.Series.SeriesMetadataID.Valid {
		if prior, tried := attempts[series.Series.ID]; tried && prior != nil {
			return nil // Leave all episodes of the same unresolved series discovered.
		}
		if loadFiles != nil {
			filesByFolder, loadErr := loadFiles()
			err = loadErr
			if err == nil {
				// Pass empty evidence explicitly to avoid the standalone loader.
				err = m.linkSeriesMetadata(ctx, series, log, filesByFolder[series.Series.Path.String])
			}
		} else {
			err = m.linkSeriesMetadata(ctx, series, log)
		}
		if attempts != nil {
			attempts[series.Series.ID] = err
		}
		if err != nil {
			log.Warn("failed to link series metadata, skipping episode reconciliation", zap.Error(err))
			return nil // Skip this episode rather than fail
		}

		// Reload series to get the updated metadata ID
		series, err = m.seriesStorage.GetSeriesByID(ctx, int64(series.Series.ID))
		if err != nil {
			return fmt.Errorf("failed to reload series after metadata linking: %w", err)
		}
	}

	// Step 2: Get series metadata and refresh from TMDB to ensure all season/episode metadata exists
	seriesMetadata, err := m.seriesMetaStorage.GetSeriesMetadataByID(ctx, int64(series.Series.SeriesMetadataID.Int64))
	if err != nil {
		log.Error("failed to get series metadata", zap.Error(err))
		return fmt.Errorf("failed to get series metadata: %w", err)
	}

	// Check if we need to refresh series metadata from TMDB
	// Only refresh if we don't have adequate season/episode metadata
	log.Debug("checking if series has adequate metadata for episode reconciliation",
		zap.Bool("series_has_metadata_id", series.Series.SeriesMetadataID.Valid))

	// Check if we have season metadata for the discovered file's season
	allSeasonMetadata, err := m.seriesMetaStorage.ListSeasonMetadataBySeriesMetadataID(ctx, series.Series.SeriesMetadataID.Int64)
	if err != nil {
		log.Warn("failed to check existing season metadata", zap.Error(err))
		return fmt.Errorf("failed to check season metadata: %w", err)
	}

	// Check if we need to refresh - either no season metadata exists for this specific season, or current season lacks metadata link
	var seasonMetadataExists bool
	for _, seasonMeta := range allSeasonMetadata {
		if seasonMeta.Number == season.Season.SeasonNumber {
			seasonMetadataExists = true
			break
		}
	}
	shouldRefresh := !seasonMetadataExists || !season.Season.SeasonMetadataID.Valid

	log.Debug("refresh decision",
		zap.Int("season_metadata_count", len(allSeasonMetadata)),
		zap.Bool("season_metadata_exists_for_season_number", seasonMetadataExists),
		zap.Bool("season_has_metadata_id", season.Season.SeasonMetadataID.Valid),
		zap.Bool("should_refresh", shouldRefresh))

	if shouldRefresh {
		if !seasonMetadataExists {
			log.Debug("no season metadata found for this season number, refreshing from TMDB")
		} else {
			log.Debug("season lacks metadata link, refreshing to link existing metadata")
		}

		err = m.refreshSeriesEpisodes(ctx, series.Series, seriesMetadata, snapshot)
		if err != nil {
			log.Warn("failed to refresh series metadata from TMDB, skipping episode reconciliation", zap.Error(err))
			return nil // Skip this episode rather than fail
		}

		// Reload season to get updated metadata link
		season, err = m.seriesStorage.GetSeasonByID(ctx, int64(episode.Episode.SeasonID))
		if err != nil {
			log.Error("failed to reload season after refresh", zap.Error(err))
			return fmt.Errorf("failed to reload season: %w", err)
		}
	}

	// Step 3: Get season metadata for the episode's season
	if !season.Season.SeasonMetadataID.Valid {
		log.Warn("season has no metadata ID, skipping reconcile")
		return nil
	}

	_, err = m.seriesMetaStorage.GetSeasonMetadataByID(ctx, season.Season.SeasonMetadataID.Int64)
	if err != nil {
		log.Error("failed to get season metadata", zap.Error(err))
		return fmt.Errorf("failed to get season metadata: %w", err)
	}

	// Step 4: Check if episode already has metadata linked
	if episode.Episode.EpisodeMetadataID.Valid {
		// Episode already has TMDB metadata linked, transition to completed
		log.Debug("episode already has TMDB metadata linked, transitioning to completed state")
		err = m.updateEpisodeState(ctx, episode, storage.EpisodeStateCompleted, nil)
		if err != nil {
			return fmt.Errorf("failed to update episode state to completed: %w", err)
		}
		log.Info("episode already properly linked, transitioned to completed")
		return nil
	}

	// Step 5: Episode needs metadata linking - get episode number from episode record or file
	var episodeNumber int
	if episode.Episode.EpisodeNumber > 0 {
		// Episode already has episode number set in database
		episodeNumber = int(episode.Episode.EpisodeNumber)
		log.Debug("using episode number from database", zap.Int("episode_number", episodeNumber))
	} else if episode.Episode.EpisodeFileID.Valid {
		// Get episode number from linked episode file
		episodeFile, err := m.seriesStorage.GetEpisodeFile(ctx, episode.Episode.EpisodeFileID.Int64)
		if err != nil {
			log.Error("failed to get episode file", zap.Error(err))
			return fmt.Errorf("failed to get episode file: %w", err)
		}

		// Parse episode number from stored file path (no filesystem operations)
		var filePath string
		if episodeFile.RelativePath.Valid {
			filePath = episodeFile.RelativePath.String
		} else if episodeFile.OriginalFilePath.Valid {
			filePath = episodeFile.OriginalFilePath.String
		} else {
			log.Warn("episode file has no path information")
			return nil
		}

		// Use library parsing to extract episode number from path
		parsedFile := library.EpisodeFileFromPath(filePath)
		if parsedFile.EpisodeNumber == 0 {
			log.Warn("could not parse episode number from file path", zap.String("path", filePath))
			return nil
		}
		episodeNumber = parsedFile.EpisodeNumber
		log.Debug("parsed episode number from file path", zap.Int("episode_number", episodeNumber))
	} else {
		log.Warn("episode has no episode number or linked episode file, cannot determine episode number")
		return nil
	}

	// Step 6: Try to match episode to existing TMDB metadata
	err = m.matchDiscoveredEpisodeToTMDBMetadataFromDB(ctx, episode, season.Season.ID, episodeNumber, log)
	if err != nil {
		log.Warn("failed to match discovered episode to TMDB metadata, skipping episode reconciliation", zap.Error(err))
		return nil
	}

	// Step 5: Transition episode to completed state since it's now properly linked
	err = m.updateEpisodeState(ctx, episode, storage.EpisodeStateCompleted, nil)
	if err != nil {
		return fmt.Errorf("failed to update episode state to completed: %w", err)
	}

	log.Info("successfully matched episode to TMDB metadata and marked as completed")
	return nil
}

// seriesDiscoveryFiles groups stored relative paths by their exact series folder,
// including files inside Season directories and excluding unrelated prefix names.
func seriesDiscoveryFiles(rows []sqlcdb.EpisodeFile, tvRoot string) map[string][]string {
	byFolder := make(map[string][]string)
	for _, row := range rows {
		path := row.RelativePath
		if !path.Valid {
			path = row.OriginalFilePath
			if path.Valid && filepath.IsAbs(path.String) {
				if tvRoot == "" {
					continue // No trusted root for an absolute original path.
				}
				root, err := filepath.Abs(tvRoot)
				if err != nil {
					continue
				}
				relative, err := filepath.Rel(root, path.String)
				if err != nil {
					continue
				}
				path.String = relative
			}
		}
		if !path.Valid || !filepath.IsLocal(path.String) {
			continue
		}
		path.String = filepath.Clean(path.String)
		if filepath.Dir(path.String) == "." {
			continue
		}
		folder := library.EpisodeFileFromPath(path.String).SeriesName
		byFolder[folder] = append(byFolder[folder], path.String)
	}
	return byFolder
}

func (m MediaManager) linkSeriesMetadata(ctx context.Context, series sqlcdb.GetSeriesRow, log *zap.SugaredLogger, filenameEvidence ...[]string) error {
	folder := series.Series.Path.String
	var files []string
	if len(filenameEvidence) > 0 {
		files = filenameEvidence[0]
	} else {
		rows, err := m.seriesStorage.ListEpisodeFiles(ctx)
		if err != nil {
			return fmt.Errorf("failed to load episode filename evidence: %w", err)
		}
		files = seriesDiscoveryFiles(rows, m.config.Library.TVDir)[folder]
	}
	evidence, err := ParseDiscoveryEvidence(folder, files, true)
	if err != nil {
		return fmt.Errorf("invalid series identity evidence: %w", err)
	}
	tmdbID := evidence.TMDBID
	if tmdbID == 0 {
		if evidence.Query == "" {
			return fmt.Errorf("no series title or explicit ID in discovery evidence")
		}
		resp, err := m.SearchTV(ctx, evidence.Query)
		if err != nil {
			return fmt.Errorf("failed to search for TV show: %w", err)
		}
		if resp == nil {
			return fmt.Errorf("no TMDB results found for series")
		}
		match := MatchDiscoveryCandidate(evidence, resp.Results, true)
		if match.Status != DiscoveryMatched {
			log.Warn("TV search did not uniquely identify series", zap.String("path", folder), zap.String("status", string(match.Status)))
			return fmt.Errorf("TV series identity is %s", match.Status)
		}
		tmdbID = *match.Candidate.ID
	}
	seriesMetadata, err := m.GetSeriesMetadata(ctx, int64(tmdbID))
	if err != nil {
		return fmt.Errorf("failed to get series metadata: %w", err)
	}
	other, err := m.seriesStorage.GetSeriesByMetadataID(ctx, seriesMetadata.ID)
	if err == nil && other.Series.ID != series.Series.ID {
		return fmt.Errorf("metadata already linked to another series")
	}
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return fmt.Errorf("failed checking existing series metadata link: %w", err)
	}
	err = m.seriesStorage.LinkSeriesMetadata(ctx, int64(series.Series.ID), seriesMetadata.ID)
	if err != nil {
		return fmt.Errorf("failed to link series metadata: %w", err)
	}

	log.Info("linked series to TMDB metadata", zap.Int64("series_metadata_id", seriesMetadata.ID))
	return nil
}

// matchDiscoveredEpisodeToTMDBMetadataFromDB attempts to match a discovered episode to existing TMDB metadata
// using episode number parsed from stored file path (no filesystem scanning)
func (m MediaManager) matchDiscoveredEpisodeToTMDBMetadataFromDB(ctx context.Context, episode sqlcdb.ListEpisodesRow, seasonID int64, episodeNumber int, log *zap.SugaredLogger) error {
	logger := log.With(zap.Int("season_id", int(seasonID)), zap.Int("episode_number", episodeNumber))
	logger.Debug("matching discovered episode to TMDB metadata using parsed episode number")

	if episodeNumber == 0 {
		logger.Warn("episode number is 0", zap.Int64("episode_id", episode.Episode.ID))
		return fmt.Errorf("episode number is 0 for episode %d", episode.Episode.ID)
	}

	// Find episode metadata that matches the episode number for this specific season
	// After our migration, episode_metadata.season_id correctly references season entity IDs
	// Get season metadata ID first
	season, err := m.seriesStorage.GetSeasonByID(ctx, int64(seasonID))
	if err != nil {
		return fmt.Errorf("failed to get season: %w", err)
	}

	if !season.Season.SeasonMetadataID.Valid {
		return fmt.Errorf("season has no metadata linked")
	}

	episodeMeta, err := m.seriesMetaStorage.GetEpisodeMetadataBySeasonAndNumber(ctx, season.Season.SeasonMetadataID.Int64, int64(episodeNumber))

	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			logger.Debug("no TMDB episode metadata found for episode number in season",
				zap.Int("season_id", int(seasonID)),
				zap.Int("episode_number", episodeNumber))
			return fmt.Errorf("no TMDB episode metadata found")
		}
		return fmt.Errorf("failed to get episode metadata: %w", err)
	}

	// Check if another episode already has this metadata linked (due to UNIQUE constraint)
	// The typed interface has no get-by-metadata-id, so scan the season's episodes.
	seasonIDInt := int64(seasonID)
	seasonEpisodes, listErr := m.seriesStorage.ListEpisodes(ctx, storage.EpisodeFilter{SeasonID: &seasonIDInt})
	if listErr != nil {
		logger.Debug("error listing episodes to check for existing metadata link", zap.Error(listErr))
	}
	var existingEpisode *sqlcdb.ListEpisodesRow
	for _, ep := range seasonEpisodes {
		if ep.Episode.EpisodeMetadataID.Valid && ep.Episode.EpisodeMetadataID.Int64 == episodeMeta.ID {
			existingEpisode = &ep
			break
		}
	}
	if existingEpisode != nil {
		logger.Warn("another episode already linked to this TMDB metadata, skipping link")
		return nil
	}

	// Update the episode's episode metadata ID in the database
	err = m.seriesStorage.LinkEpisodeMetadata(ctx, int64(episode.Episode.ID), seasonID, episodeMeta.ID)
	if err != nil {
		return fmt.Errorf("failed to link episode metadata: %w", err)
	}

	// Update the in-memory object to reflect the database changes
	episode.Episode.EpisodeMetadataID = sql.NullInt64{Int64: episodeMeta.ID, Valid: true}

	log.Info("successfully matched discovered episode to TMDB metadata",
		zap.Int64("episode_metadata_id", episodeMeta.ID),
		zap.String("episode_title", episodeMeta.Title))

	return nil
}

// ReconcileCompletedSeries evaluates and updates states for series that may have completed
func (m MediaManager) ReconcileCompletedSeries(ctx context.Context) error {
	log := logger.FromCtx(ctx)
	log.Debug("starting completed series reconciliation")

	// Check completion status for all series (monitored and unmonitored) for accurate state tracking
	series, err := m.seriesStorage.ListSeries(ctx, storage.SeriesFilter{
		States: []storage.SeriesState{storage.SeriesStateDownloading, storage.SeriesStateContinuing},
	})
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			log.Error("failed to list series for completion check", zap.Error(err))
			return fmt.Errorf("couldn't list series for completion check: %w", err)
		}
		log.Debug("no series found for completion check")
		return nil
	}

	for _, s := range series {
		log.Debug("checking series completion", zap.Any("series", s.Series.ID))
		err = m.evaluateAndUpdateSeriesState(ctx, s.Series.ID)
		if err != nil {
			log.Error("failed to evaluate series state", zap.Error(err))
			continue
		}
	}

	return nil
}

// ReconcileCompletedSeasons evaluates and updates states for seasons that may have completed
func (m MediaManager) ReconcileCompletedSeasons(ctx context.Context) error {
	log := logger.FromCtx(ctx)
	log.Debug("starting completed seasons reconciliation")

	// Check completion status for all seasons (monitored and unmonitored) for accurate state tracking
	seasons, err := m.seriesStorage.ListSeasons(ctx, storage.SeasonFilter{
		States: []storage.SeasonState{storage.SeasonStateDownloading, storage.SeasonStateContinuing},
	})
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			log.Error("failed to list seasons for completion check", zap.Error(err))
			return fmt.Errorf("couldn't list seasons for completion check: %w", err)
		}
		log.Debug("no seasons found for completion check")
		return nil
	}

	for _, s := range seasons {
		log.Debug("checking season completion", zap.Any("season", s.Season.ID))
		err = m.evaluateAndUpdateSeasonState(ctx, s.Season.ID)
		if err != nil {
			log.Error("failed to evaluate season state", zap.Error(err))
			continue
		}
	}

	return nil
}

func (m MediaManager) ReconcileDownloadingSeries(ctx context.Context, snapshot *ReconcileSnapshot) error {
	log := logger.FromCtx(ctx)

	if snapshot == nil {
		log.Warn("snapshot is nil, skipping reconcile")
		return nil
	}

	episodes, err := m.seriesStorage.ListEpisodes(ctx, storage.EpisodeFilter{
		States: []storage.EpisodeState{storage.EpisodeStateDownloading},
	})
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			log.Error("failed to list downloading episodes", zap.Error(err))
			return err
		}
		log.Debug("no downloading episodes found")
		return nil
	}

	// Group episodes by season to process each season once
	seasonEpisodes := make(map[int64][]sqlcdb.ListEpisodesRow)
	for _, episode := range episodes {
		seasonEpisodes[episode.Episode.SeasonID] = append(seasonEpisodes[episode.Episode.SeasonID], episode)
	}

	// Process each season
	for seasonID, seasonEpisodeList := range seasonEpisodes {
		season, err := m.seriesStorage.GetSeasonByID(ctx, int64(seasonID))
		if err != nil {
			log.Error("failed to get season", zap.Error(err), zap.Int64("season_id", seasonID))
			continue
		}

		err = m.reconcileDownloadingSeason(ctx, season, seasonEpisodeList, snapshot)
		if err != nil {
			log.Error("failed to reconcile downloading season", zap.Error(err))
			continue
		}
		log.Debug("successfully reconciled season", zap.Int64("season_id", season.Season.ID))
	}

	return nil
}

func (m MediaManager) reconcileDownloadingSeason(ctx context.Context, season sqlcdb.GetSeasonByIDRow, episodes []sqlcdb.ListEpisodesRow, snapshot *ReconcileSnapshot) error {
	log := logger.FromCtx(ctx)
	log = log.With("season_id", season.Season.ID)

	// Check if any episode is marked as an entire season download
	isSeasonPack := false
	for _, episode := range episodes {
		if episode.IsEntireSeasonDownload.Valid && episode.IsEntireSeasonDownload.Bool {
			isSeasonPack = true
			break
		}
	}

	if isSeasonPack {
		log.Debug("processing season pack download")
		return m.reconcileSeasonPackDownload(ctx, episodes[0], episodes, snapshot)
	}

	log.Debug("processing individual episode downloads")
	for _, episode := range episodes {
		err := m.reconcileDownloadingEpisode(ctx, episode, snapshot)
		if err != nil {
			log.Error("failed to reconcile downloading episode", zap.Error(err))
			continue
		}
	}

	return nil
}

func (m MediaManager) reconcileSeasonPackDownload(ctx context.Context, episode sqlcdb.ListEpisodesRow, episodes []sqlcdb.ListEpisodesRow, snapshot *ReconcileSnapshot) error {
	log := logger.FromCtx(ctx)
	log = log.With("season pack", "download", "episode id", episode.Episode.ID)

	if !episode.DownloadClientID.Valid || !episode.DownloadID.Valid || episode.DownloadID.String == "" {
		log.Warn("episode missing download client or download ID")
		return nil
	}

	dc := snapshot.GetDownloadClient(episode.DownloadClientID.Int64)
	if dc == nil {
		log.Warn("episode download client not found in snapshot", zap.Int64("download client id", episode.DownloadClientID.Int64))
		return nil
	}

	downloadClient, err := m.downloadClientService.buildRuntimeDownloadClient(ctx, *dc)
	if err != nil {
		log.Warn("failed to create download client", zap.Error(err))
		return err
	}

	status, err := downloadClient.Get(ctx, download.GetRequest{
		ID: episode.DownloadID.String,
	})
	if err != nil {
		log.Warn("failed to get download status", zap.Error(err))
		return err
	}

	log.Debug("download status", zap.Any("status", status))
	if !status.Done {
		log.Debug("download not finished")
		return nil
	}

	season, err := m.seriesStorage.GetSeasonByID(ctx, int64(episode.Episode.SeasonID))
	if err != nil {
		log.Error("failed to get season", zap.Error(err))
		return err
	}

	seasonMetadata, err := m.seriesMetaStorage.GetSeasonMetadataByID(ctx, int64(season.Season.SeasonMetadataID.Int64))
	if err != nil {
		log.Error("failed to get season metadata", zap.Error(err))
		return err
	}

	series, err := m.seriesStorage.GetSeriesByID(ctx, int64(season.Season.SeriesID))
	if err != nil {
		log.Error("failed to get series", zap.Error(err))
		return err
	}

	seriesMetadata, err := m.seriesMetaStorage.GetSeriesMetadataByID(ctx, int64(series.Series.SeriesMetadataID.Int64))
	if err != nil {
		log.Error("failed to get series metadata", zap.Error(err))
		return err
	}

	// For each file in the season pack, match it to an episode and link it
	for _, filePath := range status.FilePaths {
		matchedEpisode := m.matchEpisodeFileToEpisode(ctx, filePath, episodes)
		if matchedEpisode == nil {
			log.Warn("could not match file to episode, skipping", zap.String("file", filePath))
			continue
		}

		err = m.addEpisodeFileToLibrary(ctx, seriesMetadata.Title, seasonMetadata.Number, filePath, *matchedEpisode)
		if err != nil {
			log.Warn("failed to add episode file to library", zap.Error(err))
			continue
		}
	}

	// Update all episodes in the season pack to downloaded
	for _, ep := range episodes {
		err = m.updateEpisodeState(ctx, ep, storage.EpisodeStateDownloaded, nil)
		if err != nil {
			log.Error("failed to update episode state", zap.Error(err))
			continue
		}
	}

	return nil
}

func (m MediaManager) reconcileDownloadingEpisode(ctx context.Context, episode sqlcdb.ListEpisodesRow, snapshot *ReconcileSnapshot) error {
	log := logger.FromCtx(ctx)
	log = log.With("reconcile loop", "downloading episode", "episode id", episode.Episode.ID)

	if !episode.DownloadClientID.Valid || !episode.DownloadID.Valid || episode.DownloadID.String == "" {
		log.Warn("episode missing download client or download ID")
		return nil
	}

	dc := snapshot.GetDownloadClient(episode.DownloadClientID.Int64)
	if dc == nil {
		log.Warn("episode download client not found in snapshot", zap.Int64("download client id", episode.DownloadClientID.Int64))
		return nil
	}

	downloadClient, err := m.downloadClientService.buildRuntimeDownloadClient(ctx, *dc)
	if err != nil {
		log.Warn("failed to create download client", zap.Error(err))
		return err
	}

	status, err := downloadClient.Get(ctx, download.GetRequest{
		ID: episode.DownloadID.String,
	})
	if err != nil {
		log.Warn("failed to get download status", zap.Error(err))
		return err
	}

	log.Debug("download status", zap.Any("status", status))
	if !status.Done {
		log.Debug("download not finished")
		return nil
	}

	episodeMetadata, err := m.seriesMetaStorage.GetEpisodeMetadataByID(ctx, int64(episode.Episode.EpisodeMetadataID.Int64))
	if err != nil {
		log.Error("failed to get episode metadata", zap.Error(err))
		return err
	}

	season, err := m.seriesStorage.GetSeasonByID(ctx, int64(episode.Episode.SeasonID))
	if err != nil {
		log.Error("failed to get season", zap.Error(err))
		return err
	}

	seasonMetadata, err := m.seriesMetaStorage.GetSeasonMetadataByID(ctx, int64(season.Season.SeasonMetadataID.Int64))
	if err != nil {
		log.Error("failed to get season metadata", zap.Error(err))
		return err
	}

	series, err := m.seriesStorage.GetSeriesByID(ctx, int64(season.Season.SeriesID))
	if err != nil {
		log.Error("failed to get series", zap.Error(err))
		return err
	}

	seriesMetadata, err := m.seriesMetaStorage.GetSeriesMetadataByID(ctx, int64(series.Series.SeriesMetadataID.Int64))
	if err != nil {
		log.Error("failed to get series metadata", zap.Error(err))
		return err
	}

	log.Debug("processing individual episode download")
	return m.processIndividualEpisodeDownload(ctx, episode, status, seriesMetadata, seasonMetadata, episodeMetadata)
}

func (m MediaManager) processIndividualEpisodeDownload(ctx context.Context, episode sqlcdb.ListEpisodesRow, status download.Status, seriesMetadata sqlcdb.SeriesMetadatum, seasonMetadata sqlcdb.SeasonMetadatum, episodeMetadata sqlcdb.EpisodeMetadatum) error {
	log := logger.FromCtx(ctx)
	log = log.With("episode id", episode.Episode.ID, "series", seriesMetadata.Title, "season", seasonMetadata.Number, "episode", episodeMetadata.Number)

	if len(status.FilePaths) == 0 {
		log.Warn("no files found in completed download")
		return nil
	}

	for _, filePath := range status.FilePaths {
		err := m.addEpisodeFileToLibrary(ctx, seriesMetadata.Title, seasonMetadata.Number, filePath, episode)
		if err != nil {
			log.Error("failed to add episode file to library", zap.Error(err))
			return err
		}
		log.Debug("successfully added episode file to library", zap.String("file", filePath))
	}

	return m.updateEpisodeState(ctx, episode, storage.EpisodeStateDownloaded, nil)
}

func (m MediaManager) addEpisodeFileToLibrary(ctx context.Context, seriesTitle string, seasonNumber int64, filePath string, episode sqlcdb.ListEpisodesRow) error {
	log := logger.FromCtx(ctx)
	log = log.With("series", seriesTitle, "season", seasonNumber, "episodes")

	if episode.Episode.EpisodeFileID.Valid {
		log.Debug("episode already has file linked, skipping", zap.Int64("episode_id", episode.Episode.ID))
		return nil
	}

	ef, err := m.library.AddEpisode(ctx, seriesTitle, seasonNumber, filePath)
	if err != nil {
		if !errors.Is(err, io.ErrFileExists) {
			log.Error("failed to add episode to library", zap.Error(err))
			return err
		}

		log.Debug("file already exists in library, creating record for existing file")
	}

	log.Debug("episode in library", zap.String("from", filePath), zap.String("to", ef.RelativePath))

	episodeFileID, err := m.seriesStorage.CreateEpisodeFile(ctx, sqlcdb.EpisodeFile{
		Size:             ef.Size,
		RelativePath:     sql.NullString{String: ef.RelativePath, Valid: true},
		OriginalFilePath: sql.NullString{String: filePath, Valid: true},
	})
	if err != nil {
		log.Error("failed to create episode file record", zap.Error(err))
		return err
	}

	err = m.seriesStorage.UpdateEpisodeEpisodeFileID(ctx, int64(episode.Episode.ID), episodeFileID)
	if err != nil {
		log.Error("failed to link episode to file", zap.Error(err), zap.Int64("episode_id", episode.Episode.ID))
		return err
	}

	log.Debug("linked episode to file", zap.Int64("episode_id", episode.Episode.ID), zap.String("path", ef.RelativePath))

	return nil
}

// matchEpisodeFileToEpisode matches a downloaded file to a specific episode using the library package's
// episode extraction logic. Returns the matched episode or nil if no match is found.
func (m MediaManager) matchEpisodeFileToEpisode(ctx context.Context, filePath string, episodes []sqlcdb.ListEpisodesRow) *sqlcdb.ListEpisodesRow {
	log := logger.FromCtx(ctx)
	log = log.With("file_path", filePath, "candidate_episodes", len(episodes))

	// Use the library package to extract episode information from the file path
	episodeFile := library.EpisodeFileFromPath(filePath)

	log.Debug("extracted episode info from file",
		zap.String("series_name", episodeFile.SeriesName),
		zap.Int("season_number", episodeFile.SeasonNumber),
		zap.Int("episode_number", episodeFile.EpisodeNumber))

	// If we couldn't extract episode number, we can't match
	if episodeFile.EpisodeNumber == 0 {
		log.Warn("could not extract episode number from file path")
		return nil
	}

	// Look for an episode that matches the extracted episode number
	for _, episode := range episodes {
		if !episode.Episode.EpisodeMetadataID.Valid {
			continue // Skip episodes without metadata
		}

		// Get the episode metadata to check the episode number
		episodeMetadata, err := m.seriesMetaStorage.GetEpisodeMetadataByID(ctx, int64(episode.Episode.EpisodeMetadataID.Int64))
		if err != nil {
			log.Warn("failed to get episode metadata", zap.Error(err), zap.Int64("episode_id", episode.Episode.ID))
			continue
		}

		// Check if the episode numbers match
		if int64(episodeFile.EpisodeNumber) == episodeMetadata.Number {
			log.Debug("matched file to episode",
				zap.Int64("episode_id", episode.Episode.ID),
				zap.Int64("episode_number", episodeMetadata.Number))
			return &episode
		}
	}

	log.Warn("no matching episode found",
		zap.Int("file_episode_number", episodeFile.EpisodeNumber))
	return nil
}
