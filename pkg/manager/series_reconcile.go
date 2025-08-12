package manager

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/pkg/library"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
	"go.uber.org/zap"
)

func (m MediaManager) ReconcileSeries(ctx context.Context) error {
	log := logger.FromCtx(ctx)

	dcs, err := m.ListDownloadClients(ctx)
	if err != nil {
		return err
	}

	indexers, err := m.ListIndexers(ctx)
	if err != nil {
		return err
	}

	log.Debug("listed indexers", zap.Int("count", len(indexers)))
	if len(indexers) == 0 {
		return errors.New("no indexers available")
	}

	snapshot := newReconcileSnapshot(indexers, dcs)

	err = m.ReconcileMissingSeries(ctx, snapshot)
	if err != nil {
		log.Error("failed to reconcile missing series", zap.Error(err))
	}

	err = m.ReconcileContinuingSeries(ctx, snapshot)
	if err != nil {
		log.Error("failed to reconcile continuing series", zap.Error(err))
	}

	err = m.ReconcileCompletedSeasons(ctx)
	if err != nil {
		log.Error("failed to reconcile completed seasons", zap.Error(err))
	}

	err = m.ReconcileCompletedSeries(ctx)
	if err != nil {
		log.Error("failed to reconcile completed series", zap.Error(err))
	}

	err = m.ReconcileDiscoveredEpisodes(ctx)
	if err != nil {
		log.Error("failed to reconcile discovered episodes", zap.Error(err))
	}

	return nil
}

func (m MediaManager) ReconcileMissingSeries(ctx context.Context, snapshot *ReconcileSnapshot) error {
	log := logger.FromCtx(ctx)

	if snapshot == nil {
		log.Warn("snapshot is nil, skipping reconcile")
		return nil
	}

	where := table.SeriesTransition.ToState.EQ(sqlite.String(string(storage.SeriesStateMissing))).
		AND(table.SeriesTransition.MostRecent.EQ(sqlite.Bool(true))).
		AND(table.Series.Monitored.EQ(sqlite.Int(1)))

	series, err := m.storage.ListSeries(ctx, where)
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			log.Error("failed to list missing series", zap.Error(err))
			return fmt.Errorf("couldn't list missing series: %w", err)
		}

		log.Debug("no missing series found")
		return nil
	}

	for _, s := range series {
		log.Debug("reconciling series", zap.Any("series", s.ID))
		err = m.reconcileMissingSeries(ctx, s, snapshot)
		if err != nil {
			log.Error("failed to reconcile series", zap.Error(err))
			continue
		}

		log.Debug("successfully reconciled series", zap.Any("series", s.ID))
	}

	return nil
}

func (m MediaManager) ReconcileContinuingSeries(ctx context.Context, snapshot *ReconcileSnapshot) error {
	log := logger.FromCtx(ctx)

	if snapshot == nil {
		log.Warn("snapshot is nil, skipping reconcile")
		return nil
	}

	where := table.SeriesTransition.ToState.IN(
		sqlite.String(string(storage.SeriesStateContinuing)),
		sqlite.String(string(storage.SeriesStateDownloading)),
	).AND(table.SeriesTransition.MostRecent.EQ(sqlite.Bool(true))).
		AND(table.Series.Monitored.EQ(sqlite.Int(1)))

	series, err := m.storage.ListSeries(ctx, where)
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			log.Error("failed to list continuing series", zap.Error(err))
			return fmt.Errorf("couldn't list continuing series: %w", err)
		}

		log.Debug("no continuing series found")
		return nil
	}

	for _, s := range series {
		log.Debug("reconciling continuing series", zap.Any("series", s.ID))
		err = m.reconcileContinuingSeries(ctx, s, snapshot)
		if err != nil {
			log.Error("failed to reconcile continuing series", zap.Error(err))
			continue
		}

		log.Debug("successfully reconciled continuing series", zap.Any("series", s.ID))
	}

	return nil
}

func (m MediaManager) reconcileContinuingSeries(ctx context.Context, series *storage.Series, snapshot *ReconcileSnapshot) error {
	log := logger.FromCtx(ctx)

	if series == nil {
		return fmt.Errorf("series is nil")
	}

	if series.Monitored == 0 {
		log.Debug("series is not monitored, skipping reconcile")
		return nil
	}

	qualityProfile, err := m.storage.GetQualityProfile(ctx, int64(series.QualityProfileID))
	if err != nil {
		log.Warnw("failed to find series qualityprofile", "quality_id", series.QualityProfileID)
		return err
	}

	seriesMetadata, err := m.storage.GetSeriesMetadata(ctx, table.SeriesMetadata.ID.EQ(sqlite.Int32(*series.SeriesMetadataID)))
	if err != nil {
		log.Debugw("failed to find series metadata", "meta_id", *series.SeriesMetadataID)
		return err
	}

	err = m.refreshSeriesEpisodes(ctx, series, seriesMetadata, snapshot)
	if err != nil {
		log.Warn("failed to refresh series episodes, continuing with existing episodes", zap.Error(err))
	}

	releases, err := m.SearchIndexers(ctx, snapshot.GetIndexerIDs(), TV_CATEGORIES, seriesMetadata.Title)
	if err != nil {
		log.Debugw("failed to search indexer", "indexers", snapshot.GetIndexerIDs(), zap.Error(err))
		return err
	}

	slices.SortFunc(releases, sortReleaseFunc())

	where := table.Season.SeriesID.EQ(sqlite.Int32(series.ID)).
		AND(table.Season.Monitored.EQ(sqlite.Int(1)))

	seasons, err := m.storage.ListSeasons(ctx, where)
	if err != nil {
		log.Error("failed to list seasons for continuing series", zap.Error(err))
		return fmt.Errorf("couldn't list seasons for continuing series: %w", err)
	}

	if len(seasons) == 0 {
		log.Debug("no seasons found, skipping reconcile")
		return nil
	}

	for _, season := range seasons {
		episodeWhere := table.Episode.SeasonID.EQ(sqlite.Int32(season.ID)).
			AND(table.EpisodeTransition.ToState.EQ(sqlite.String(string(storage.EpisodeStateMissing))))

		missingEpisodes, err := m.storage.ListEpisodes(ctx, episodeWhere)
		if err != nil {
			log.Error("failed to list missing episodes for continuing series", zap.Error(err))
			continue
		}

		if len(missingEpisodes) == 0 {
			log.Debug("no missing episodes in season", zap.Any("season", season.ID))
			continue
		}

		log.Debug("found missing episodes in continuing series",
			zap.Any("series", series.ID),
			zap.Any("season", season.ID),
			zap.Int("count", len(missingEpisodes)))

		seasonMetadata, err := m.storage.GetSeasonMetadata(ctx, table.SeasonMetadata.ID.EQ(sqlite.Int32(*season.SeasonMetadataID)))
		if err != nil {
			log.Debugw("failed to find season metadata", "meta_id", *season.SeasonMetadataID)
			continue
		}

		err = m.reconcileMissingEpisodes(ctx, seriesMetadata.Title, season.ID, seasonMetadata.Number, missingEpisodes, snapshot, qualityProfile, releases)
		if err != nil {
			log.Error("failed to reconcile missing episodes in continuing series", zap.Error(err))
			continue
		}

		log.Debug("successfully reconciled missing episodes in continuing series",
			zap.Any("series", series.ID),
			zap.Any("season", season.ID))
	}

	return nil
}

// refreshSeriesEpisodes fetches the latest episode metadata from TMDB and creates any new episodes
func (m MediaManager) refreshSeriesEpisodes(ctx context.Context, series *storage.Series, seriesMetadata *model.SeriesMetadata, snapshot *ReconcileSnapshot) error {
	log := logger.FromCtx(ctx)

	_, err := m.RefreshSeriesMetadataFromTMDB(ctx, int(seriesMetadata.TmdbID))
	if err != nil {
		log.Debug("failed to refresh series metadata from TMDB", zap.Error(err))
		return err
	}

	where := table.Season.SeriesID.EQ(sqlite.Int32(series.ID))
	existingSeasons, err := m.storage.ListSeasons(ctx, where)
	if err != nil {
		log.Error("failed to list existing seasons", zap.Error(err))
		return err
	}

	existingSeasonNumbers := make(map[int32]int64)
	existingSeasonsWithoutMetadata := make([]*storage.Season, 0)

	for _, season := range existingSeasons {
		if season.SeasonMetadataID == nil {
			// Track seasons without metadata for later linking
			existingSeasonsWithoutMetadata = append(existingSeasonsWithoutMetadata, season)
			continue
		}

		seasonMetadata, err := m.storage.GetSeasonMetadata(ctx, table.SeasonMetadata.ID.EQ(sqlite.Int32(*season.SeasonMetadataID)))
		if err != nil {
			continue
		}

		existingSeasonNumbers[seasonMetadata.Number] = int64(season.ID)
	}

	seasonMetadataWhere := table.SeasonMetadata.SeriesID.EQ(sqlite.Int64(int64(seriesMetadata.ID)))
	allSeasonMetadata, err := m.storage.ListSeasonMetadata(ctx, seasonMetadataWhere)
	if err != nil {
		log.Error("failed to list season metadata", zap.Error(err))
		return err
	}

	for _, seasonMeta := range allSeasonMetadata {
		var seasonID int64
		var exists bool

		if seasonID, exists = existingSeasonNumbers[seasonMeta.Number]; !exists {
			season := storage.Season{
				Season: model.Season{
					SeriesID:         int32(series.ID),
					SeasonMetadataID: ptr(seasonMeta.ID),
					Monitored:        1,
				},
			}

			seasonID, err = m.storage.CreateSeason(ctx, season, storage.SeasonStateMissing)
			if err != nil {
				log.Error("failed to create new season", zap.Error(err))
				continue
			}

			log.Debug("created new season for continuing series",
				zap.Any("series", series.ID),
				zap.Int32("season_number", seasonMeta.Number))
		}

		episodeWhere := table.Episode.SeasonID.EQ(sqlite.Int64(seasonID))
		existingEpisodes, err := m.storage.ListEpisodes(ctx, episodeWhere)
		if err != nil {
			log.Error("failed to list existing episodes", zap.Error(err))
			continue
		}

		existingEpisodeNumbers := make(map[int32]bool)
		for _, episode := range existingEpisodes {
			existingEpisodeNumbers[episode.EpisodeNumber] = true
		}

		episodeMetadataWhere := table.EpisodeMetadata.SeasonID.EQ(sqlite.Int64(seasonID))
		episodeMetadataList, err := m.storage.ListEpisodeMetadata(ctx, episodeMetadataWhere)
		if err != nil {
			log.Error("failed to list episode metadata", zap.Error(err))
			continue
		}

		for _, episodeMeta := range episodeMetadataList {
			if !existingEpisodeNumbers[episodeMeta.Number] {
				episode := storage.Episode{
					Episode: model.Episode{
						EpisodeMetadataID: ptr(episodeMeta.ID),
						SeasonID:          int32(seasonID),
						Monitored:         1,
						EpisodeNumber:     episodeMeta.Number,
					},
				}

				episodeState := storage.EpisodeStateMissing
				if !isReleased(snapshot.time, episodeMeta.AirDate) {
					episodeState = storage.EpisodeStateUnreleased
				}

				_, err := m.storage.CreateEpisode(ctx, episode, episodeState)
				if err != nil {
					log.Error("failed to create new episode",
						zap.Error(err),
						zap.Int32("episode_number", episodeMeta.Number))
					continue
				}

				log.Debug("created new episode for continuing series",
					zap.Any("series", series.ID),
					zap.Int32("season_number", seasonMeta.Number),
					zap.Int32("episode_number", episodeMeta.Number),
					zap.String("state", string(episodeState)))
			}
		}
	}

	// Link existing seasons without metadata to the appropriate metadata
	for _, season := range existingSeasonsWithoutMetadata {
		// Use the season number stored in the database (set during indexing)
		seasonNumber := season.SeasonNumber

		// Find matching season metadata
		for _, seasonMeta := range allSeasonMetadata {
			if seasonMeta.Number == seasonNumber {
				err := m.storage.LinkSeasonMetadata(ctx, int64(season.ID), seasonMeta.ID)
				if err != nil {
					log.Error("failed to link season metadata", zap.Error(err))
				} else {
					log.Debug("linked existing season to metadata",
						zap.Int32("season_id", season.ID),
						zap.Int32("season_number", seasonNumber),
						zap.Int32("metadata_id", seasonMeta.ID))
					// Update the existingSeasonNumbers map
					existingSeasonNumbers[seasonNumber] = int64(season.ID)
				}
				break
			}
		}
	}

	return nil
}

func (m MediaManager) reconcileMissingSeries(ctx context.Context, series *storage.Series, snapshot *ReconcileSnapshot) error {
	log := logger.FromCtx(ctx)

	if series == nil {
		return fmt.Errorf("series is nil")
	}

	if series.Monitored == 0 {
		log.Debug("series is not monitored, skipping reconcile")
		return nil
	}

	qualityProfile, err := m.storage.GetQualityProfile(ctx, int64(series.QualityProfileID))
	if err != nil {
		log.Warnw("failed to find series qualityprofile", "quality_id", series.QualityProfileID)
		return err
	}

	seriesMetadata, err := m.storage.GetSeriesMetadata(ctx, table.SeriesMetadata.ID.EQ(sqlite.Int32(*series.SeriesMetadataID)))
	if err != nil {
		log.Debugw("failed to find series metadata", "meta_id", *series.SeriesMetadataID)
		return err
	}

	releases, err := m.SearchIndexers(ctx, snapshot.GetIndexerIDs(), TV_CATEGORIES, seriesMetadata.Title)
	if err != nil {
		log.Debugw("failed to search indexer", "indexers", snapshot.GetIndexerIDs(), zap.Error(err))
		return err
	}

	slices.SortFunc(releases, sortReleaseFunc())

	where := table.Season.SeriesID.EQ(sqlite.Int32(series.ID)).
		AND(table.Season.Monitored.EQ(sqlite.Int(1))).
		AND(table.SeasonTransition.ToState.EQ(sqlite.String(string(storage.SeasonStateMissing))))

	seasons, err := m.storage.ListSeasons(ctx, where)
	if err != nil {
		log.Error("failed to list missing seasons", zap.Error(err))
		return fmt.Errorf("couldn't list missing seasons: %w", err)
	}

	if len(seasons) == 0 {
		log.Debug("no seasons found, skipping reconcile")
		return nil
	}

	for _, s := range seasons {
		log.Debug("reconciling season", zap.Any("season", s.ID))
		err = m.reconcileMissingSeason(ctx, seriesMetadata.Title, s, snapshot, qualityProfile, releases)
		if err != nil {
			log.Error("failed to reconcile missing season", zap.Error(err))
			continue
		}
		log.Debug("successfully reconciled season", zap.Any("season", s.ID))
	}

	return nil
}

func (m MediaManager) reconcileMissingSeason(ctx context.Context, seriesTitle string, season *storage.Season, snapshot *ReconcileSnapshot, qualityProfile storage.QualityProfile, releases []*prowlarr.ReleaseResource) error {
	log := logger.FromCtx(ctx)
	log = log.With("reconcile loop", "missing series")

	if season == nil {
		return fmt.Errorf("season is nil")
	}

	metadata, err := m.storage.GetSeasonMetadata(ctx, table.SeasonMetadata.ID.EQ(sqlite.Int32(*season.SeasonMetadataID)))
	if err != nil {
		log.Debugw("failed to find season metadata", "meta_id", *season.SeasonMetadataID)
		return err
	}

	where := table.Episode.SeasonID.EQ(sqlite.Int32(season.ID))
	episodes, err := m.storage.ListEpisodes(ctx, where)
	if err != nil {
		log.Error("failed to list missing episodes", zap.Error(err))
		return fmt.Errorf("couldn't list missing episodes: %w", err)
	}
	if len(episodes) == 0 {
		log.Debug("no episodes found, skipping reconcile")
		return nil
	}

	var allMissing = true
	var missingEpisodes []*storage.Episode
	for _, e := range episodes {
		switch e.State {
		case storage.EpisodeStateMissing:
			missingEpisodes = append(missingEpisodes, e)
			continue
		default:
			allMissing = false
		}
	}

	log.Debug("found missing episodes", zap.Int("count", len(missingEpisodes)))

	if !allMissing {
		return m.reconcileMissingEpisodes(ctx, seriesTitle, season.ID, metadata.Number, missingEpisodes, snapshot, qualityProfile, releases)
	}

	// Fetch episode metadata for missing episodes to calculate runtime
	var missingEpisodesMetadata []*model.EpisodeMetadata
	for _, e := range missingEpisodes {
		if e.EpisodeMetadataID != nil {
			episodeMetadata, err := m.storage.GetEpisodeMetadata(ctx, table.EpisodeMetadata.ID.EQ(sqlite.Int32(*e.EpisodeMetadataID)))
			if err != nil {
				log.Warn("failed to get episode metadata for runtime calculation", zap.Error(err))
				continue
			}
			missingEpisodesMetadata = append(missingEpisodesMetadata, episodeMetadata)
		}
	}

	runtime := getSeasonRuntime(missingEpisodesMetadata, len(episodes))
	log.Debug("considering releases for season pack", zap.Int("count", len(releases)))

	var chosenSeasonPackRelease *prowlarr.ReleaseResource
	for _, r := range releases {
		if RejectSeasonReleaseFunc(ctx, seriesTitle, metadata.Number, runtime, qualityProfile, snapshot.GetProtocols())(r) {
			continue
		}

		chosenSeasonPackRelease = r
		break
	}

	if chosenSeasonPackRelease == nil {
		log.Debug("no season pack releases found, defaulting to individual episodes")
		return m.reconcileMissingEpisodes(ctx, seriesTitle, season.ID, metadata.Number, missingEpisodes, snapshot, qualityProfile, releases)
	}

	log.Infow("found season pack release", "title", chosenSeasonPackRelease.Title, "proto", *chosenSeasonPackRelease.Protocol)

	clientID, status, err := m.requestReleaseDownload(ctx, snapshot, chosenSeasonPackRelease)
	if err != nil {
		log.Debug("failed to request episode release download", zap.Error(err))
		return err
	}

	var allUpdated = true
	for _, e := range missingEpisodes {
		err = m.updateEpisodeState(ctx, *e, storage.EpisodeStateDownloading, &storage.TransitionStateMetadata{
			DownloadID:             &status.ID,
			DownloadClientID:       &clientID,
			IsEntireSeasonDownload: ptr(true),
		})
		if err != nil {
			allUpdated = false
			log.Error("failed to update episode state in seasons pack", zap.Error(err))
			continue
		}

		log.Debug("successfully reconciled episode in seasons pack")
	}

	if allUpdated {
		return m.updateSeasonState(ctx, int64(season.ID), storage.SeasonStateDownloading, &storage.TransitionStateMetadata{
			DownloadID:             &status.ID,
			DownloadClientID:       &clientID,
			IsEntireSeasonDownload: ptr(true),
		})
	}

	return nil
}

func (m MediaManager) reconcileMissingEpisodes(ctx context.Context, seriesTitle string, seasonID int32, seasonNumber int32, episode []*storage.Episode, snapshot *ReconcileSnapshot, qualityProfile storage.QualityProfile, releases []*prowlarr.ReleaseResource) error {
	log := logger.FromCtx(ctx)

	var allUpdated = true
	for _, e := range episode {
		updated, err := m.reconcileMissingEpisode(ctx, seriesTitle, seasonNumber, e, snapshot, qualityProfile, releases)
		if !updated {
			allUpdated = false
		}
		if err != nil {
			log.Error("failed to reconcile missing episode", zap.Error(err))
			continue
		}
	}

	if !allUpdated {
		return nil
	}

	return m.updateSeasonState(ctx, int64(seasonID), storage.SeasonStateDownloading, nil)
}

func (m MediaManager) reconcileMissingEpisode(ctx context.Context, seriesTitle string, seasonNumber int32, episode *storage.Episode, snapshot *ReconcileSnapshot, qualityProfile storage.QualityProfile, releases []*prowlarr.ReleaseResource) (bool, error) {
	log := logger.FromCtx(ctx)

	if episode == nil {
		log.Warn("episode is nil, skipping reconcile")
		return false, fmt.Errorf("episode is nil")
	}

	if snapshot == nil {
		log.Warn("snapshot is nil, skipping reconcile")
		return false, nil
	}

	episodeMetadata, err := m.storage.GetEpisodeMetadata(ctx, table.EpisodeMetadata.ID.EQ(sqlite.Int32(*episode.EpisodeMetadataID)))
	if err != nil {
		log.Debugw("failed to find episode metadata", "meta_id", *episode.EpisodeMetadataID)
		return false, err
	}

	if !isReleased(snapshot.time, episodeMetadata.AirDate) {
		log.Debug("episode is not yet released", zap.Any("air_date", episodeMetadata.AirDate))
		return false, nil
	}

	if episodeMetadata.Runtime == nil {
		log.Warn("episode runtime is nil, skipping reconcile")
		return false, nil
	}

	var chosenRelease *prowlarr.ReleaseResource
	for _, r := range releases {
		if RejectEpisodeReleaseFunc(ctx, seriesTitle, seasonNumber, episodeMetadata.Number, *episodeMetadata.Runtime, qualityProfile, snapshot.GetProtocols())(r) {
			continue
		}
		chosenRelease = r
	}

	if chosenRelease == nil {
		log.Debug("no valid releases found for episode, skipping reconcile")
		return false, nil
	}

	log.Infow("found release", "title", chosenRelease.Title, "proto", *chosenRelease.Protocol)

	clientID, status, err := m.requestReleaseDownload(ctx, snapshot, chosenRelease)
	if err != nil {
		log.Debug("failed to request episode release download", zap.Error(err))
		return false, err
	}

	err = m.updateEpisodeState(ctx, *episode, storage.EpisodeStateDownloading, &storage.TransitionStateMetadata{
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

func (m MediaManager) updateEpisodeState(ctx context.Context, episode storage.Episode, state storage.EpisodeState, metadata *storage.TransitionStateMetadata) error {
	log := logger.FromCtx(ctx).With("episode id", episode.ID, "from state", episode.State, "to state", state)

	if episode.State == state {
		log.Debug("episode already in target state, skipping update")
		return nil
	}

	err := m.storage.UpdateEpisodeState(ctx, int64(episode.ID), state, metadata)
	if err != nil {
		log.Error("failed to update episode state", zap.Error(err))
		return err
	}

	log.Info("successfully updated episode state")

	if state != storage.EpisodeStateDownloaded && state != storage.EpisodeStateDownloading && state != storage.EpisodeStateCompleted {
		return nil
	}

	if err := m.evaluateAndUpdateSeasonState(ctx, episode.SeasonID); err != nil {
		log.Warn("failed to update season state after episode state change", zap.Error(err))
	}

	season, err := m.storage.GetSeason(ctx, table.Season.ID.EQ(sqlite.Int32(episode.SeasonID)))
	if err != nil {
		log.Warn("failed to get season for series state update", zap.Error(err))
		return nil
	}

	if err := m.evaluateAndUpdateSeriesState(ctx, season.SeriesID); err != nil {
		log.Warn("failed to update series state after episode state change", zap.Error(err))
	}

	return nil
}

func (m MediaManager) updateSeasonState(ctx context.Context, id int64, state storage.SeasonState, metadata *storage.TransitionStateMetadata) error {
	log := logger.FromCtx(ctx).With("season id", id, "to state", state)

	season, err := m.storage.GetSeason(ctx, table.Season.ID.EQ(sqlite.Int64(id)))
	if err != nil {
		log.Error("failed to get season for state check", zap.Error(err))
		return err
	}

	if season.State == state {
		log.Debug("season already in target state, skipping update")
		return nil
	}

	err = m.storage.UpdateSeasonState(ctx, id, state, metadata)
	if err != nil {
		log.Error("failed to update season state", zap.Error(err))
		return err
	}

	log.Info("successfully updated season state")

	if state != storage.SeasonStateCompleted && state != storage.SeasonStateDownloading {
		return nil
	}

	if err := m.evaluateAndUpdateSeriesState(ctx, season.SeriesID); err != nil {
		log.Warn("failed to update series state after season state change", zap.Error(err))
	}

	return nil
}

func getSeasonRuntime(episodeMetadata []*model.EpisodeMetadata, totalSeasonEpisodes int) int32 {
	var runtime int32
	var consideredRuntimeCount int
	for _, meta := range episodeMetadata {
		if meta.Runtime != nil {
			runtime = runtime + *meta.Runtime
			consideredRuntimeCount++
		}
	}

	if consideredRuntimeCount == 0 {
		return 0
	}

	// If we have runtimes for some but not all episodes, calculate an average and apply it to the missing ones
	if consideredRuntimeCount < totalSeasonEpisodes {
		averageRuntime := runtime / int32(consideredRuntimeCount)
		missingRuntimes := (totalSeasonEpisodes - consideredRuntimeCount)
		runtime = runtime + (averageRuntime * int32(missingRuntimes))
	}

	return runtime
}

func determineSeasonState(episodes []*storage.Episode) (map[string]int, storage.SeasonState) {
	var done, downloading, missing, unreleased, discovered int
	for _, episode := range episodes {
		switch episode.State {
		case storage.EpisodeStateDownloaded, storage.EpisodeStateCompleted:
			done++
		case storage.EpisodeStateDownloading:
			downloading++
		case storage.EpisodeStateMissing:
			missing++
		case storage.EpisodeStateUnreleased:
			unreleased++
		case storage.EpisodeStateDiscovered:
			discovered++
		}
	}

	counts := map[string]int{
		"done":        done,
		"downloading": downloading,
		"missing":     missing,
		"unreleased":  unreleased,
		"discovered":  discovered,
	}

	switch {
	case len(episodes) == 0:
		return counts, storage.SeasonStateMissing
	case done == len(episodes):
		return counts, storage.SeasonStateCompleted
	case downloading > 0:
		return counts, storage.SeasonStateDownloading
	case discovered > 0 && (done > 0 || missing > 0 || downloading > 0):
		return counts, storage.SeasonStateContinuing
	case (done > 0 || missing > 0) && unreleased > 0:
		return counts, storage.SeasonStateContinuing
	case missing > 0 && unreleased == 0:
		return counts, storage.SeasonStateMissing
	case unreleased > 0 && done == 0 && missing == 0:
		return counts, storage.SeasonStateUnreleased
	case discovered > 0 && done == 0 && missing == 0 && downloading == 0 && unreleased == 0:
		return counts, storage.SeasonStateDiscovered
	default:
		return counts, storage.SeasonStateMissing
	}
}

// updateSeriesState updates the series state and handles cascading updates
func (m MediaManager) updateSeriesState(ctx context.Context, id int64, state storage.SeriesState, metadata *storage.TransitionStateMetadata) error {
	log := logger.FromCtx(ctx).With("series id", id, "to state", state)
	err := m.storage.UpdateSeriesState(ctx, id, state, metadata)
	if err != nil {
		log.Error("failed to update series state", zap.Error(err))
		return err
	}

	log.Info("successfully updated series state")
	return nil
}

// evaluateAndUpdateSeasonState evaluates all episodes in a season and updates the season state accordingly
func (m MediaManager) evaluateAndUpdateSeasonState(ctx context.Context, seasonID int32) error {
	log := logger.FromCtx(ctx).With("season_id", seasonID)

	currentSeason, err := m.storage.GetSeason(ctx, table.Season.ID.EQ(sqlite.Int32(seasonID)))
	if err != nil {
		log.Error("failed to get season for state evaluation", zap.Error(err))
		return err
	}

	where := table.Episode.SeasonID.EQ(sqlite.Int32(seasonID))
	episodes, err := m.storage.ListEpisodes(ctx, where)
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
		zap.String("current_state", string(currentSeason.State)),
		zap.String("new_state", string(newSeasonState)))

	if currentSeason.State == newSeasonState {
		log.Debug("season already in target state, no update needed")
		return nil
	}

	return m.updateSeasonState(ctx, int64(seasonID), newSeasonState, nil)
}

// evaluateAndUpdateSeriesState evaluates all seasons in a series and updates the series state accordingly
func (m MediaManager) evaluateAndUpdateSeriesState(ctx context.Context, seriesID int32) error {
	log := logger.FromCtx(ctx).With("series_id", seriesID)

	series, err := m.storage.GetSeries(ctx, table.Series.ID.EQ(sqlite.Int32(seriesID)))
	if err != nil {
		log.Error("failed to get series for state evaluation", zap.Error(err))
		return err
	}

	where := table.Season.SeriesID.EQ(sqlite.Int32(seriesID))
	seasons, err := m.storage.ListSeasons(ctx, where)
	if err != nil {
		log.Error("failed to list seasons for series state evaluation", zap.Error(err))
		return err
	}

	if len(seasons) == 0 {
		log.Debug("no seasons found for series, keeping current state")
		return nil
	}

	var completed, downloading, missing, unreleased, discovered, continuing int
	for _, season := range seasons {
		switch season.State {
		case storage.SeasonStateCompleted:
			completed++
		case storage.SeasonStateDownloading:
			downloading++
		case storage.SeasonStateMissing:
			missing++
		case storage.SeasonStateUnreleased:
			unreleased++
		case storage.SeasonStateDiscovered:
			discovered++
		case storage.SeasonStateContinuing:
			continuing++
		}
	}

	var newSeriesState storage.SeriesState
	switch {
	case completed == len(seasons):
		newSeriesState = storage.SeriesStateCompleted
	case downloading > 0:
		newSeriesState = storage.SeriesStateDownloading
	case discovered > 0 && (completed > 0 || missing > 0 || downloading > 0 || continuing > 0):
		newSeriesState = storage.SeriesStateContinuing
	case continuing > 0:
		newSeriesState = storage.SeriesStateContinuing
	case missing > 0 && unreleased == 0:
		newSeriesState = storage.SeriesStateMissing
	case unreleased > 0:
		newSeriesState = storage.SeriesStateUnreleased
	case discovered > 0 && completed == 0 && missing == 0 && downloading == 0 && continuing == 0 && unreleased == 0:
		newSeriesState = storage.SeriesStateDiscovered
	default:
		newSeriesState = storage.SeriesStateContinuing
	}

	log.Debug("evaluating series state",
		zap.Int("total_seasons", len(seasons)),
		zap.Int("completed", completed),
		zap.Int("downloading", downloading),
		zap.Int("missing", missing),
		zap.Int("unreleased", unreleased),
		zap.Int("discovered", discovered),
		zap.Int("continuing", continuing),
		zap.String("current_state", string(series.State)),
		zap.String("new_state", string(newSeriesState)))

	if series.State == newSeriesState {
		log.Debug("series already in target state, no update needed")
		return nil
	}

	return m.updateSeriesState(ctx, int64(seriesID), newSeriesState, nil)
}

// ReconcileDiscoveredEpisodes processes episodes in the "discovered" state by linking them to their
// corresponding TMDB metadata hierarchy (series -> season -> episode). Discovered episodes have video
// files on disk that need to be associated with the proper metadata before transitioning to "completed" state.
func (m MediaManager) ReconcileDiscoveredEpisodes(ctx context.Context) error {
	log := logger.FromCtx(ctx)

	where := table.EpisodeTransition.ToState.EQ(sqlite.String(string(storage.EpisodeStateDiscovered))).
		AND(table.EpisodeTransition.MostRecent.EQ(sqlite.Bool(true))).
		AND(table.Episode.Monitored.EQ(sqlite.Int(1)))

	episodes, err := m.storage.ListEpisodes(ctx, where)
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			log.Error("failed to list discovered episodes", zap.Error(err))
			return fmt.Errorf("couldn't list discovered episodes: %w", err)
		}
		log.Debug("no discovered episodes found")
		return nil
	}

	for _, episode := range episodes {
		log.Debug("reconciling discovered episode", zap.Any("episode", episode.ID))
		err = m.reconcileDiscoveredEpisode(ctx, episode)
		if err != nil {
			log.Error("failed to reconcile discovered episode", zap.Error(err))
			continue
		}
		log.Debug("successfully reconciled discovered episode", zap.Any("episode", episode.ID))
	}

	return nil
}

func (m MediaManager) reconcileDiscoveredEpisode(ctx context.Context, episode *storage.Episode) error {
	log := logger.FromCtx(ctx)

	if episode == nil {
		log.Warn("episode is nil, skipping reconcile")
		return nil
	}

	log = log.With("reconcile loop", "discovered", "episode id", episode.ID)
	log.Debug("starting episode reconciliation", zap.Int32("episode_id", episode.ID))

	season, err := m.storage.GetSeason(ctx, table.Season.ID.EQ(sqlite.Int32(episode.SeasonID)))
	if err != nil || season == nil {
		log.Error("failed to get season for discovered episode", zap.Error(err))
		return fmt.Errorf("failed to get season: %w", err)
	}

	series, err := m.storage.GetSeries(ctx, table.Series.ID.EQ(sqlite.Int32(season.SeriesID)))
	if err != nil || series == nil {
		log.Error("failed to get series for discovered episode", zap.Error(err))
		return fmt.Errorf("failed to get series: %w", err)
	}

	if series.Path == nil {
		log.Warn("series path is nil, skipping reconcile")
		return nil
	}

	// Step 1: Ensure series has metadata
	if series.SeriesMetadataID == nil {
		err = m.linkSeriesMetadata(ctx, series, log)
		if err != nil {
			log.Warn("failed to link series metadata, skipping episode reconciliation", zap.Error(err))
			return nil // Skip this episode rather than fail
		}

		// Reload series to get the updated metadata ID
		series, err = m.storage.GetSeries(ctx, table.Series.ID.EQ(sqlite.Int32(series.ID)))
		if err != nil {
			return fmt.Errorf("failed to reload series after metadata linking: %w", err)
		}
	}

	// Step 2: Get series metadata and refresh from TMDB to ensure all season/episode metadata exists
	seriesMetadata, err := m.storage.GetSeriesMetadata(ctx, table.SeriesMetadata.ID.EQ(sqlite.Int32(*series.SeriesMetadataID)))
	if err != nil {
		log.Error("failed to get series metadata", zap.Error(err))
		return fmt.Errorf("failed to get series metadata: %w", err)
	}

	// Create a dummy snapshot for refreshSeriesEpisodes (it only needs time)
	snapshot := &ReconcileSnapshot{time: now()}

	// Check if we need to refresh series metadata from TMDB
	// Only refresh if we don't have adequate season/episode metadata
	log.Debug("checking if series has adequate metadata for episode reconciliation",
		zap.Bool("series_has_metadata_id", series.SeriesMetadataID != nil))

	// Check if we have season metadata for the discovered file's season
	allSeasonMetadata, err := m.storage.ListSeasonMetadata(ctx,
		table.SeasonMetadata.SeriesID.EQ(sqlite.Int32(*series.SeriesMetadataID)))
	if err != nil {
		log.Warn("failed to check existing season metadata", zap.Error(err))
		return fmt.Errorf("failed to check season metadata: %w", err)
	}

	// Check if we need to refresh - either no season metadata exists, or current season lacks metadata link
	shouldRefresh := len(allSeasonMetadata) == 0 || season.SeasonMetadataID == nil

	log.Debug("refresh decision",
		zap.Int("season_metadata_count", len(allSeasonMetadata)),
		zap.Bool("season_has_metadata_id", season.SeasonMetadataID != nil),
		zap.Bool("should_refresh", shouldRefresh))

	if shouldRefresh {
		if len(allSeasonMetadata) == 0 {
			log.Debug("no season metadata found, refreshing from TMDB")
		} else {
			log.Debug("season lacks metadata link, refreshing to link existing metadata")
		}

		err = m.refreshSeriesEpisodes(ctx, series, seriesMetadata, snapshot)
		if err != nil {
			log.Warn("failed to refresh series metadata from TMDB, skipping episode reconciliation", zap.Error(err))
			return nil // Skip this episode rather than fail
		}

		// Reload season to get updated metadata link
		season, err = m.storage.GetSeason(ctx, table.Season.ID.EQ(sqlite.Int32(episode.SeasonID)))
		if err != nil || season == nil {
			log.Error("failed to reload season after refresh", zap.Error(err))
			return fmt.Errorf("failed to reload season: %w", err)
		}
	}

	// Step 3: Get season metadata for the episode's season
	if season.SeasonMetadataID == nil {
		log.Warn("season has no metadata ID, skipping reconcile")
		return nil
	}

	seasonMetadata, err := m.storage.GetSeasonMetadata(ctx,
		table.SeasonMetadata.ID.EQ(sqlite.Int32(*season.SeasonMetadataID)))
	if err != nil || seasonMetadata == nil {
		log.Error("failed to get season metadata", zap.Error(err))
		return fmt.Errorf("failed to get season metadata: %w", err)
	}

	// Step 4: Check if episode already has metadata linked
	if episode.EpisodeMetadataID != nil {
		// Episode already has TMDB metadata linked, transition to completed
		log.Debug("episode already has TMDB metadata linked, transitioning to completed state")
		err = m.updateEpisodeState(ctx, *episode, storage.EpisodeStateCompleted, nil)
		if err != nil {
			return fmt.Errorf("failed to update episode state to completed: %w", err)
		}
		log.Info("episode already properly linked, transitioned to completed")
		return nil
	}

	// Step 5: Episode needs metadata linking - get episode number from episode record or file
	var episodeNumber int
	if episode.EpisodeNumber > 0 {
		// Episode already has episode number set in database
		episodeNumber = int(episode.EpisodeNumber)
		log.Debug("using episode number from database", zap.Int("episode_number", episodeNumber))
	} else if episode.EpisodeFileID != nil {
		// Get episode number from linked episode file
		episodeFile, err := m.getEpisodeFileByID(ctx, *episode.EpisodeFileID)
		if err != nil {
			log.Error("failed to get episode file", zap.Error(err))
			return fmt.Errorf("failed to get episode file: %w", err)
		}

		if episodeFile == nil {
			log.Warn("episode file not found", zap.Int32("episode_file_id", *episode.EpisodeFileID))
			return nil
		}

		// Parse episode number from stored file path (no filesystem operations)
		var filePath string
		if episodeFile.RelativePath != nil {
			filePath = *episodeFile.RelativePath
		} else if episodeFile.OriginalFilePath != nil {
			filePath = *episodeFile.OriginalFilePath
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
	err = m.matchDiscoveredEpisodeToTMDBMetadataFromDB(ctx, episode, series, seasonMetadata, episodeNumber, log)
	if err != nil {
		log.Warn("failed to match discovered episode to TMDB metadata, skipping episode reconciliation", zap.Error(err))
		return nil
	}

	// Step 5: Transition episode to completed state since it's now properly linked
	err = m.updateEpisodeState(ctx, *episode, storage.EpisodeStateCompleted, nil)
	if err != nil {
		return fmt.Errorf("failed to update episode state to completed: %w", err)
	}

	log.Info("successfully matched episode to TMDB metadata and marked as completed")
	return nil
}

func (m MediaManager) linkSeriesMetadata(ctx context.Context, series *storage.Series, log *zap.SugaredLogger) error {
	searchTerm := pathToSearchTerm(*series.Path)
	searchResp, err := m.SearchTV(ctx, searchTerm)
	if err != nil {
		return fmt.Errorf("failed to search for TV show: %w", err)
	}

	if len(searchResp.Results) == 0 {
		log.Warn("no results found for TV show", zap.String("path", *series.Path), zap.String("search_term", searchTerm))
		return fmt.Errorf("no TMDB results found for series")
	}

	if len(searchResp.Results) > 1 {
		log.Debug("multiple results found for TV show", zap.String("path", *series.Path), zap.String("search_term", searchTerm), zap.Int("count", len(searchResp.Results)))
	}

	result := searchResp.Results[0]
	if result.ID == nil {
		return fmt.Errorf("TV show result has no ID")
	}

	seriesMetadata, err := m.GetSeriesMetadata(ctx, *result.ID)
	if err != nil {
		return fmt.Errorf("failed to get series metadata: %w", err)
	}

	err = m.storage.LinkSeriesMetadata(ctx, int64(series.ID), seriesMetadata.ID)
	if err != nil {
		return fmt.Errorf("failed to link series metadata: %w", err)
	}

	log.Info("linked series to TMDB metadata", zap.Int32("series_metadata_id", seriesMetadata.ID))
	return nil
}

// matchDiscoveredEpisodeToTMDBMetadataFromDB attempts to match a discovered episode to existing TMDB metadata
// using episode number parsed from stored file path (no filesystem scanning)
func (m MediaManager) matchDiscoveredEpisodeToTMDBMetadataFromDB(ctx context.Context, episode *storage.Episode, series *storage.Series, seasonMetadata *model.SeasonMetadata, episodeNumber int, log *zap.SugaredLogger) error {
	log.Debug("matching discovered episode to TMDB metadata using parsed episode number",
		zap.Int32("season_number", seasonMetadata.Number),
		zap.Int("episode_number", episodeNumber))

	if episodeNumber == 0 {
		log.Warn("episode number is 0", zap.Int32("episode_id", episode.ID))
		return fmt.Errorf("episode number is 0 for episode %d", episode.ID)
	}

	// Find episode metadata that matches the episode number
	episodeMeta, err := m.storage.GetEpisodeMetadata(ctx,
		table.EpisodeMetadata.SeasonID.EQ(sqlite.Int32(seasonMetadata.ID)).
			AND(table.EpisodeMetadata.Number.EQ(sqlite.Int32(int32(episodeNumber)))))

	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			log.Warn("no TMDB episode metadata found for episode number",
				zap.Int("episode_number", episodeNumber),
				zap.Int32("season_number", seasonMetadata.Number))
			return fmt.Errorf("no TMDB episode metadata for S%dE%d", seasonMetadata.Number, episodeNumber)
		}
		return fmt.Errorf("failed to get episode metadata: %w", err)
	}

	// Check if another episode already has this metadata linked (due to UNIQUE constraint)
	existingEpisode, err := m.storage.GetEpisode(ctx, table.Episode.EpisodeMetadataID.EQ(sqlite.Int32(episodeMeta.ID)))
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		log.Debug("error checking for existing episode with metadata", zap.Error(err))
	} else if existingEpisode != nil {
		log.Warn("another episode already linked to this TMDB metadata, skipping link",
			zap.Int32("existing_episode_id", existingEpisode.ID),
			zap.Int32("episode_metadata_id", episodeMeta.ID))
		return nil
	}

	// Update the episode's episode metadata ID in the database
	err = m.storage.LinkEpisodeMetadata(ctx, int64(episode.ID), episode.SeasonID, episodeMeta.ID)
	if err != nil {
		return fmt.Errorf("failed to link episode metadata: %w", err)
	}

	// Update the in-memory object to reflect the database changes
	episode.EpisodeMetadataID = &episodeMeta.ID

	log.Info("successfully matched discovered episode to TMDB metadata",
		zap.Int32("episode_metadata_id", episodeMeta.ID),
		zap.String("episode_title", episodeMeta.Title),
		zap.Int32("season_number", seasonMetadata.Number),
		zap.Int32("episode_number", episodeMeta.Number))

	return nil
}

// getEpisodeFileByID retrieves an episode file by its ID from the database
func (m MediaManager) getEpisodeFileByID(ctx context.Context, fileID int32) (*model.EpisodeFile, error) {
	episodeFiles, err := m.storage.ListEpisodeFiles(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list episode files: %w", err)
	}

	for _, ef := range episodeFiles {
		if ef.ID == fileID {
			return ef, nil
		}
	}

	return nil, nil // Not found, but not an error
}

// ReconcileCompletedSeries evaluates and updates states for series that may have completed
func (m MediaManager) ReconcileCompletedSeries(ctx context.Context) error {
	log := logger.FromCtx(ctx)

	where := table.SeriesTransition.ToState.IN(
		sqlite.String(string(storage.SeriesStateDownloading)),
		sqlite.String(string(storage.SeriesStateContinuing)),
	).AND(table.SeriesTransition.MostRecent.EQ(sqlite.Bool(true))).
		AND(table.Series.Monitored.EQ(sqlite.Int(1)))

	series, err := m.storage.ListSeries(ctx, where)
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			log.Error("failed to list series for completion check", zap.Error(err))
			return fmt.Errorf("couldn't list series for completion check: %w", err)
		}
		log.Debug("no series found for completion check")
		return nil
	}

	for _, s := range series {
		log.Debug("checking series completion", zap.Any("series", s.ID))
		err = m.evaluateAndUpdateSeriesState(ctx, s.ID)
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

	where := table.SeasonTransition.ToState.IN(
		sqlite.String(string(storage.SeasonStateDownloading)),
		sqlite.String(string(storage.SeasonStateContinuing)),
	).AND(table.SeasonTransition.MostRecent.EQ(sqlite.Bool(true))).
		AND(table.Season.Monitored.EQ(sqlite.Int(1)))

	seasons, err := m.storage.ListSeasons(ctx, where)
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			log.Error("failed to list seasons for completion check", zap.Error(err))
			return fmt.Errorf("couldn't list seasons for completion check: %w", err)
		}
		log.Debug("no seasons found for completion check")
		return nil
	}

	for _, s := range seasons {
		log.Debug("checking season completion", zap.Any("season", s.ID))
		err = m.evaluateAndUpdateSeasonState(ctx, s.ID)
		if err != nil {
			log.Error("failed to evaluate season state", zap.Error(err))
			continue
		}
	}

	return nil
}
