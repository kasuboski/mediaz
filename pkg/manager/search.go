package manager

import (
	"context"
	"fmt"

	"github.com/kasuboski/mediaz/pkg/indexer"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
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

func (m MediaManager) executeSearch(ctx context.Context, snapshot *ReconcileSnapshot, categories []int32, opts indexer.SearchOptions) ([]*prowlarr.ReleaseResource, error) {
	log := logger.FromCtx(ctx)

	indexerIDs := snapshot.GetIndexerIDs()
	log.Debug("searching indexers",
		zap.Int64s("indexer_ids", indexerIDs),
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
	log := logger.FromCtx(ctx).With("series_id", seriesID)
	log.Debug("starting manual search for series")

	series, err := m.seriesStorage.GetSeriesByID(ctx, seriesID)
	if err != nil {
		log.Error("failed to get series", zap.Error(err))
		return fmt.Errorf("series not found: %w", err)
	}

	if series.Series.Monitored == 0 {
		log.Debug("series is not monitored, cannot search")
		return fmt.Errorf("series is not monitored")
	}

	seriesIDInt := series.Series.ID
	seasons, err := m.seriesStorage.ListSeasons(ctx, storage.SeasonFilter{SeriesID: &seriesIDInt})
	if err != nil {
		log.Error("failed to list seasons", zap.Error(err))
		return fmt.Errorf("failed to list seasons: %w", err)
	}

	var searchErrors []error
	for _, season := range seasons {
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

	searchTime := now()
	err = m.seriesStorage.UpdateSeriesByID(ctx, series.Series.ID, sqlcdb.Series{
		ID:             series.Series.ID,
		Monitored:      series.Series.Monitored,
		LastSearchTime: nullTimeIn(&searchTime),
	})
	if err != nil {
		log.Warn("failed to update series last search time", zap.Error(err))
	}

	if len(searchErrors) > 0 {
		log.Warn("some season searches failed", zap.Int("failed_count", len(searchErrors)))
		return fmt.Errorf("%d season searches failed", len(searchErrors))
	}

	log.Debug("manual search completed for series")
	return nil
}

func (m MediaManager) SearchForSeason(ctx context.Context, seasonID int64) error {
	log := logger.FromCtx(ctx).With("season_id", seasonID)
	log.Debug("starting manual search for season")

	season, err := m.seriesStorage.GetSeasonByID(ctx, seasonID)
	if err != nil {
		log.Error("failed to get season", zap.Error(err))
		return fmt.Errorf("season not found: %w", err)
	}

	if season.Season.Monitored == 0 {
		log.Debug("season is not monitored, cannot search")
		return fmt.Errorf("season is not monitored")
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

	if series.Series.QualityProfileID == 0 {
		log.Warn("series quality profile id is nil, skipping search")
		return fmt.Errorf("series has no quality profile")
	}

	qualityProfile, err := m.ListQualityProfileRows(ctx, series.Series.QualityProfileID)
	if err != nil {
		log.Error("failed to get quality profile", zap.Error(err))
		return fmt.Errorf("failed to get quality profile: %w", err)
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

	seasonNumber := int32(season.Season.SeasonNumber)
	searchType := indexer.TypeTV
	releases, err := m.executeSearch(ctx, snapshot, TVCategories, indexer.SearchOptions{
		Query:  seriesMetadata.Title,
		Season: &seasonNumber,
		Type:   &searchType,
	})
	if err != nil {
		return err
	}

	err = m.reconcileMissingSeason(ctx, seriesMetadata.Title, seasonListRow(season), snapshot, qualityProfile, releases)
	if err != nil {
		log.Error("failed to reconcile season", zap.Error(err))
		return err
	}

	log.Debug("manual search completed for season")
	return nil
}

func (m MediaManager) SearchForEpisode(ctx context.Context, episodeID int64) error {
	log := logger.FromCtx(ctx).With("episode_id", episodeID)
	log.Debug("starting manual search for episode")

	episode, err := m.seriesStorage.GetEpisodeByID(ctx, episodeID)
	if err != nil {
		log.Error("failed to get episode", zap.Error(err))
		return fmt.Errorf("episode not found: %w", err)
	}

	if episode.Episode.Monitored == 0 {
		log.Debug("episode is not monitored, cannot search")
		return fmt.Errorf("episode is not monitored")
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

	if series.Series.QualityProfileID == 0 {
		log.Warn("series quality profile id is nil, skipping search")
		return fmt.Errorf("series has no quality profile")
	}

	qualityProfile, err := m.ListQualityProfileRows(ctx, series.Series.QualityProfileID)
	if err != nil {
		log.Error("failed to get quality profile", zap.Error(err))
		return fmt.Errorf("failed to get quality profile: %w", err)
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

	seasonNumber := int32(season.Season.SeasonNumber)
	episodeNumber := int32(episode.Episode.EpisodeNumber)
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

	_, err = m.reconcileMissingEpisode(ctx, seriesMetadata.Title, int64(seasonNumber), episodeListRow(episode), snapshot, qualityProfile, releases)
	if err != nil {
		log.Error("failed to reconcile episode", zap.Error(err))
		return err
	}

	log.Debug("manual search completed for episode")
	return nil
}
