package manager

import (
	"context"
	"fmt"
	"github.com/kasuboski/mediaz/pkg/indexer"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/storage"
	sqlcdb "github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"go.uber.org/zap"
)

func (m MediaManager) prepareSearchSnapshot(ctx context.Context) (*ReconcileSnapshot, error) {
	log := logger.FromCtx(ctx)

	dcs, err := m.ListDownloadClients(ctx)
	if err != nil {
		log.Error("failed to list download clients", zap.Error(err))
		return nil, err
	}

	indexers, err := m.listIndexersInternal(ctx)
	if err != nil {
		log.Error("failed to list indexers", zap.Error(err))
		return nil, err
	}

	if len(indexers) == 0 {
		log.Warn("no indexers available for search")
		return nil, fmt.Errorf("no indexers available")
	}

	return newReconcileSnapshot(indexers, dcs), nil
}

func (m MediaManager) executeSearch(ctx context.Context, snapshot *ReconcileSnapshot, categories []int32, opts indexer.SearchOptions) ([]*indexer.Release, error) {
	log := logger.FromCtx(ctx)

	indexerIDs := snapshot.GetIndexerIDs()
	log.Debug("searching indexers",
		zap.Any("indexer_ids", indexerIDs),
		zap.Int32s("categories", categories))

	releases, err := m.indexerService.SearchIndexers(ctx, indexerIDs, categories, opts)
	if err != nil {
		log.Warn("some indexer sources failed during search", zap.Error(err))
		if len(releases) == 0 {
			return nil, err
		}
	}

	return releases, nil
}

func (m MediaManager) SearchForMovie(ctx context.Context, movieID int64) error {
	log := logger.FromCtx(ctx).With("movie_id", movieID)
	log.Debug("starting manual search for movie")

	movie, err := m.movieStorage.GetMovie(ctx, movieID)
	if err != nil {
		log.Error("failed to get movie", zap.Error(err))
		return fmt.Errorf("movie not found: %w", err)
	}

	if movie.Movie.Monitored == 0 {
		log.Debug("movie is not monitored, cannot search")
		return fmt.Errorf("movie is not monitored")
	}

	snapshot, err := m.prepareSearchSnapshot(ctx)
	if err != nil {
		return err
	}

	err = m.reconcileMissingMovie(ctx, sqlcdb.ListMoviesByStatesRow{
		Movie: movie.Movie, State: movie.State, DownloadID: movie.DownloadID, DownloadClientID: movie.DownloadClientID,
	}, snapshot)
	if err != nil {
		log.Error("failed to search for movie", zap.Error(err))
		return err
	}

	searchTime := now()
	err = m.movieStorage.UpdateMovieByID(ctx, movieID, sqlcdb.Movie{
		ID:             movie.Movie.ID,
		Monitored:      movie.Movie.Monitored,
		LastSearchTime: nullTimeIn(&searchTime),
	})
	if err != nil {
		log.Warn("failed to update last search time", zap.Error(err))
	}

	log.Debug("manual search completed for movie")
	return nil
}

func (m MediaManager) SearchForSeries(ctx context.Context, seriesID int64) error {
	ctx = withTVScope(ctx, "series")
	log := logger.FromCtx(ctx).With("series_id", seriesID)
	log.Debug("starting explicit request-once search for series")

	// Explicit series requests bypass the series monitoring gate but still
	// respect season and episode monitoring choices; monitoring flags are
	// never mutated here.
	series, err := m.seriesStorage.GetSeriesByID(ctx, seriesID)
	if err != nil {
		log.Error("failed to get series", zap.Error(err))
		return fmt.Errorf("series not found: %w", err)
	}

	seriesIDInt := series.Series.ID
	seasons, err := m.seriesStorage.ListSeasons(ctx, storage.SeasonFilter{SeriesID: &seriesIDInt})
	if err != nil {
		log.Error("failed to list seasons", zap.Error(err))
		return fmt.Errorf("failed to list seasons: %w", err)
	}

	var searchErrors []error
	for _, season := range seasons {
		// Explicit series scope respects the stored season choices.
		if season.Season.Monitored == 0 {
			log.Debug("skipping unmonitored season", zap.Int64("season_id", season.Season.ID))
			continue
		}

		err = m.SearchForSeason(ctx, season.Season.ID)
		if err != nil {
			log.Warn("failed to search for season", zap.Error(err), zap.Int64("season_id", season.Season.ID))

			searchErrors = append(searchErrors, err)
		}
	}

	// Request-once must not write back its pre-search monitoring snapshot.
	// UpdateSeriesByID only updates monitoring, not LastSearchTime.

	if len(searchErrors) > 0 {
		log.Warn("some season searches failed", zap.Int("failed_count", len(searchErrors)))
		return fmt.Errorf("%d season searches failed", len(searchErrors))
	}

	log.Debug("manual search completed for series")
	return nil
}

func (m MediaManager) SearchForSeason(ctx context.Context, seasonID int64) error {
	if ctx.Value(tvRequestScope{}) == nil {
		ctx = withTVScope(ctx, "season")
	}
	log := logger.FromCtx(ctx).With("season_id", seasonID)
	log.Debug("starting explicit request-once search for season")

	season, err := m.seriesStorage.GetSeasonByID(ctx, seasonID)
	if err != nil {
		log.Error("failed to get season", zap.Error(err))
		return fmt.Errorf("season not found: %w", err)
	}

	// An active season download already covers this target; do not enqueue
	// a duplicate pack, and let the existing download continue processing.
	if storage.SeasonState(season.State.String) == storage.SeasonStateDownloading && (season.DownloadID.Valid || season.DownloadClientID.Valid) {
		log.Debug("season already downloading, skipping request")
		return nil
	}

	snapshot, err := m.prepareSearchSnapshot(ctx)
	if err != nil {
		return err
	}

	series, err := m.seriesStorage.GetSeriesByID(ctx, season.Season.SeriesID)
	if err != nil {
		log.Error("failed to get series for season", zap.Error(err))
		return fmt.Errorf("failed to get series: %w", err)
	}

	if !series.Series.SeriesMetadataID.Valid {
		log.Error("series has no metadata ID")
		return fmt.Errorf("series has no metadata")
	}

	seriesMetadata, err := m.seriesMetaStorage.GetSeriesMetadataByID(ctx, series.Series.SeriesMetadataID.Int64)
	if err != nil {
		log.Error("failed to get series metadata", zap.Error(err))
		return fmt.Errorf("failed to get series metadata: %w", err)
	}

	if seriesMetadata.Title == "" {
		log.Error("series metadata has empty title")
		return fmt.Errorf("series has no title")
	}

	seasonNumber := season.Season.SeasonNumber
	searchType := indexer.TypeTV
	releases, err := m.executeSearch(ctx, snapshot, TVCategories, indexer.SearchOptions{
		Query:  seriesMetadata.Title,
		Season: &seasonNumber,
		Type:   &searchType,
	})
	if err != nil {
		return err
	}

	// Explicit season scope bypasses series/season monitoring but includes
	// only monitored child episodes; effective quality profiles are resolved
	// per episode (episode > season > series).
	err = m.reconcileMissingSeasonWithMetadata(ctx, seriesMetadata.Title, series.Series, seasonListRow(season), nil, nil, nil, snapshot, nil, releases, seriesReleaseIdentity(seriesMetadata))
	if err != nil {
		log.Error("failed to reconcile season", zap.Error(err))
		return err
	}

	log.Debug("explicit season request completed")
	return nil
}

func (m MediaManager) SearchForEpisode(ctx context.Context, episodeID int64) error {
	ctx = withTVScope(ctx, "episode")
	log := logger.FromCtx(ctx).With("episode_id", episodeID)
	log.Debug("starting explicit request-once search for episode")

	episode, err := m.seriesStorage.GetEpisodeByID(ctx, episodeID)
	if err != nil {
		log.Error("failed to get episode", zap.Error(err))
		return fmt.Errorf("episode not found: %w", err)
	}

	// Request-once targets exactly this episode regardless of monitoring
	// flags (which are never mutated), but only when it is still missing;
	// downloading/downloaded/importing targets are not enqueued again.
	if storage.EpisodeState(episode.State.String) != storage.EpisodeStateMissing || hasActiveDownload(episodeListRow(episode)) {
		log.Debug("episode is not a missing target, skipping request",
			zap.String("state", episode.State.String))
		return nil
	}

	season, err := m.seriesStorage.GetSeasonByID(ctx, episode.Episode.SeasonID)
	if err != nil {
		log.Error("failed to get season for episode", zap.Error(err))
		return fmt.Errorf("season not found: %w", err)
	}

	snapshot, err := m.prepareSearchSnapshot(ctx)
	if err != nil {
		return err
	}

	series, err := m.seriesStorage.GetSeriesByID(ctx, season.Season.SeriesID)
	if err != nil {
		log.Error("failed to get series for episode", zap.Error(err))
		return fmt.Errorf("failed to get series: %w", err)
	}

	if !series.Series.SeriesMetadataID.Valid {
		log.Error("series has no metadata ID")
		return fmt.Errorf("series has no metadata")
	}

	seriesMetadata, err := m.seriesMetaStorage.GetSeriesMetadataByID(ctx, series.Series.SeriesMetadataID.Int64)
	if err != nil {
		log.Error("failed to get series metadata", zap.Error(err))
		return fmt.Errorf("failed to get series metadata: %w", err)
	}

	if seriesMetadata.Title == "" {
		log.Error("series metadata has empty title")
		return fmt.Errorf("series has no title")
	}

	seasonNumber := season.Season.SeasonNumber
	episodeNumber := episode.Episode.EpisodeNumber
	searchType := indexer.TypeTV
	releases, err := m.executeSearch(ctx, snapshot, TVCategories, indexer.SearchOptions{
		Query:   seriesMetadata.Title,
		Season:  &seasonNumber,
		Episode: &episodeNumber,
		Type:    &searchType,
	})
	if err != nil {
		return err
	}

	// The effective persistent profile (episode > season > series) is
	// resolved and validated inside the reconciler; no silent fallback.
	_, err = m.reconcileMissingEpisodeWithMetadata(ctx, seriesMetadata.Title, series.Series.QualityProfileID, season.Season.QualityProfileID, seasonNumber, episodeListRow(episode), nil, snapshot, nil, releases, seriesReleaseIdentity(seriesMetadata))
	if err != nil {
		log.Error("failed to reconcile episode", zap.Error(err))
		return err
	}

	log.Debug("explicit episode request completed")
	return nil
}
