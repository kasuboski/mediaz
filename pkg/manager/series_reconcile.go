package manager

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/go-jet/jet/v2/sqlite"
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
	for _, season := range existingSeasons {
		if season.SeasonMetadataID == nil {
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

		// Get existing episodes for this season
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
						Runtime:           episodeMeta.Runtime,
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
	// if we didn't find any episodes we're done
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

	runtime := getSeasonRuntime(missingEpisodes, len(episodes))
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

	// Skip if already in the target state
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

	if state != storage.EpisodeStateDownloaded && state != storage.EpisodeStateDownloading {
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

	// Get current season to check state
	season, err := m.storage.GetSeason(ctx, table.Season.ID.EQ(sqlite.Int64(id)))
	if err != nil {
		log.Error("failed to get season for state check", zap.Error(err))
		return err
	}

	// Skip if already in the target state
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

func getSeasonRuntime(episodes []*storage.Episode, totalSeasonEpisodes int) int32 {
	var runtime int32
	var consideredRuntimeCount int
	for _, e := range episodes {
		if e.Runtime != nil {
			runtime = runtime + *e.Runtime
			consideredRuntimeCount++
		}
	}

	// try to estimate the remaining runtime based on the average of the other episodes
	if consideredRuntimeCount > 0 && consideredRuntimeCount < totalSeasonEpisodes {
		averageRuntime := runtime / int32(consideredRuntimeCount)
		runtime += averageRuntime * int32(totalSeasonEpisodes-consideredRuntimeCount)
	}

	return runtime
}

func determineSeasonState(episodes []*storage.Episode) (map[string]int, storage.SeasonState) {
	var downloaded, downloading, missing, unreleased int
	for _, episode := range episodes {
		switch episode.State {
		case storage.EpisodeStateDownloaded:
			downloaded++
		case storage.EpisodeStateDownloading:
			downloading++
		case storage.EpisodeStateMissing:
			missing++
		case storage.EpisodeStateUnreleased:
			unreleased++
		}
	}

	counts := map[string]int{
		"downloaded":  downloaded,
		"downloading": downloading,
		"missing":     missing,
		"unreleased":  unreleased,
	}

	switch {
	case len(episodes) == 0:
		return counts, storage.SeasonStateMissing
	case downloaded == len(episodes):
		return counts, storage.SeasonStateCompleted
	case downloading > 0:
		return counts, storage.SeasonStateDownloading
	case (downloaded > 0 || missing > 0) && unreleased > 0:
		return counts, storage.SeasonStateContinuing
	case missing > 0 && unreleased == 0:
		return counts, storage.SeasonStateMissing
	case unreleased > 0 && downloaded == 0 && missing == 0:
		return counts, storage.SeasonStateUnreleased
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

	// Get current season to check state
	currentSeason, err := m.storage.GetSeason(ctx, table.Season.ID.EQ(sqlite.Int32(seasonID)))
	if err != nil {
		log.Error("failed to get season for state evaluation", zap.Error(err))
		return err
	}

	// Get all episodes for this season
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
		zap.Int("downloaded", counts["downloaded"]),
		zap.Int("downloading", counts["downloading"]),
		zap.Int("missing", counts["missing"]),
		zap.Int("unreleased", counts["unreleased"]),
		zap.String("current_state", string(currentSeason.State)),
		zap.String("new_state", string(newSeasonState)))

	// Only update if the state is actually changing
	if currentSeason.State == newSeasonState {
		log.Debug("season already in target state, no update needed")
		return nil
	}

	return m.updateSeasonState(ctx, int64(seasonID), newSeasonState, nil)
}

// evaluateAndUpdateSeriesState evaluates all seasons in a series and updates the series state accordingly
func (m MediaManager) evaluateAndUpdateSeriesState(ctx context.Context, seriesID int32) error {
	log := logger.FromCtx(ctx).With("series_id", seriesID)

	// Get current series to check state
	series, err := m.storage.GetSeries(ctx, table.Series.ID.EQ(sqlite.Int32(seriesID)))
	if err != nil {
		log.Error("failed to get series for state evaluation", zap.Error(err))
		return err
	}

	// Get all seasons for this series
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

	// Count seasons by state
	var completed, downloading, missing, unreleased int
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
		}
	}

	var seriesMetadata *model.SeriesMetadata
	if series.SeriesMetadataID != nil {
		seriesMetadata, err = m.storage.GetSeriesMetadata(ctx, table.SeriesMetadata.ID.EQ(sqlite.Int32(*series.SeriesMetadataID)))
		if err != nil {
			log.Warn("failed to get series metadata for state evaluation", zap.Error(err))
		}
	}

	// Determine series state based on season states
	var newSeriesState storage.SeriesState
	switch {
	case completed == len(seasons):
		if seriesMetadata != nil && seriesMetadata.Status == "Ended" {
			newSeriesState = storage.SeriesStateEnded
			break
		}
		newSeriesState = storage.SeriesStateCompleted
	case downloading > 0:
		newSeriesState = storage.SeriesStateDownloading
	case missing > 0 && unreleased == 0:
		newSeriesState = storage.SeriesStateMissing
	case unreleased > 0:
		newSeriesState = storage.SeriesStateUnreleased
	default:
		newSeriesState = storage.SeriesStateContinuing
	}

	log.Debug("evaluating series state",
		zap.Int("total_seasons", len(seasons)),
		zap.Int("completed", completed),
		zap.Int("downloading", downloading),
		zap.Int("missing", missing),
		zap.Int("unreleased", unreleased),
		zap.String("current_state", string(series.State)),
		zap.String("new_state", string(newSeriesState)))

	// Only update if the state is actually changing
	if series.State == newSeriesState {
		log.Debug("series already in target state, no update needed")
		return nil
	}

	return m.updateSeriesState(ctx, int64(seriesID), newSeriesState, nil)
}

// ReconcileCompletedSeries evaluates and updates states for series that may have completed
func (m MediaManager) ReconcileCompletedSeries(ctx context.Context) error {
	log := logger.FromCtx(ctx)

	// Get all series that are currently in downloading or continuing state
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

	// Get all seasons that are currently in downloading or continuing state
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
