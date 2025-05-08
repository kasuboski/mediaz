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
		return m.reconcileMissingEpisodes(ctx, seriesTitle, metadata.Number, missingEpisodes, snapshot, qualityProfile, releases)
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
		return m.reconcileMissingEpisodes(ctx, seriesTitle, metadata.Number, missingEpisodes, snapshot, qualityProfile, releases)
	}

	log.Infow("found season pack release", "title", chosenSeasonPackRelease.Title, "proto", *chosenSeasonPackRelease.Protocol)

	clientID, status, err := m.requestReleaseDownload(ctx, snapshot, chosenSeasonPackRelease)
	if err != nil {
		log.Debug("failed to request episode release download", zap.Error(err))
		return err
	}

	for _, e := range missingEpisodes {
		err = m.updateEpisodeState(ctx, *e, storage.EpisodeStateDownloading, &storage.TransitionStateMetadata{
			DownloadID:             &status.ID,
			DownloadClientID:       &clientID,
			IsEntireSeasonDownload: ptr(true),
		})
		if err != nil {
			log.Error("failed to update episode state in seasons pack", zap.Error(err))
			continue
		}

		log.Debug("successfully reconciled episode in seasons pack")
	}

	return nil
}

func (m MediaManager) reconcileMissingEpisodes(ctx context.Context, seriesTitle string, seasonNumber int32, episode []*storage.Episode, snapshot *ReconcileSnapshot, qualityProfile storage.QualityProfile, releases []*prowlarr.ReleaseResource) error {
	log := logger.FromCtx(ctx)

	for _, e := range episode {
		if episode == nil {
			log.Warn("episode is nil, skipping reconcile")
			return fmt.Errorf("episode is nil")
		}

		if snapshot == nil {
			log.Warn("snapshot is nil, skipping reconcile")
			return nil
		}

		episodeMetadata, err := m.storage.GetEpisodeMetadata(ctx, table.EpisodeMetadata.ID.EQ(sqlite.Int32(*e.EpisodeMetadataID)))
		if err != nil {
			log.Debugw("failed to find episode metadata", "meta_id", *e.EpisodeMetadataID)
			return err
		}

		if episodeMetadata.Runtime == nil {
			log.Warn("episode runtime is nil, skipping reconcile")
			return nil
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
			continue
		}

		log.Infow("found release", "title", chosenRelease.Title, "proto", *chosenRelease.Protocol)

		clientID, status, err := m.requestReleaseDownload(ctx, snapshot, chosenRelease)
		if err != nil {
			log.Debug("failed to request episode release download", zap.Error(err))
			return err
		}

		err = m.updateEpisodeState(ctx, *e, storage.EpisodeStateDownloading, &storage.TransitionStateMetadata{
			DownloadID:       &status.ID,
			DownloadClientID: &clientID,
		})
		if err != nil {
			log.Debug("failed to update episode state", zap.Error(err))
			return err
		}

		log.Debug("successfully reconciled episode")
	}

	return nil
}

func (m MediaManager) updateEpisodeState(ctx context.Context, episode storage.Episode, state storage.EpisodeState, metadata *storage.TransitionStateMetadata) error {
	log := logger.FromCtx(ctx).With("episode id", episode.ID, "from state", episode.State, "to state", state)
	err := m.storage.UpdateEpisodeState(ctx, int64(episode.ID), state, metadata)
	if err != nil {
		log.Error("failed to update episode state", zap.Error(err))
		return err
	}

	log.Info("successfully updated episode state")
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

	// if we're missing some of the runtimes, we can try to estimate the remaining runtime based on the average of the other episodes
	// this could be pretty inaccurate in cases where we are missing more than a few runtimes, but it's better than nothing
	if consideredRuntimeCount > 0 && consideredRuntimeCount < totalSeasonEpisodes {
		averageRuntime := runtime / int32(consideredRuntimeCount)
		runtime += averageRuntime * int32(totalSeasonEpisodes-consideredRuntimeCount)
	}

	return runtime
}
