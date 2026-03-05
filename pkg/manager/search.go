package manager

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/pkg/indexer"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
	"go.uber.org/zap"
)

func (m MediaManager) SearchIndexers(ctx context.Context, indexers, categories []int32, opts indexer.SearchOptions) ([]*prowlarr.ReleaseResource, error) {
	log := logger.FromCtx(ctx)

	sourceIndexers := make(map[int64][]int32)

	keys := m.indexerCache.Keys()
	for _, sourceID := range keys {
		cached, ok := m.indexerCache.Get(sourceID)
		if !ok {
			continue
		}

		for _, idx := range cached.Indexers {
			for _, id := range indexers {
				if idx.ID == id {
					sourceIndexers[sourceID] = append(sourceIndexers[sourceID], id)
				}
			}
		}
	}

	if len(sourceIndexers) == 0 {
		return nil, fmt.Errorf("no indexer sources found for requested indexers")
	}

	type result struct {
		releases []*prowlarr.ReleaseResource
		err      error
	}

	resultChan := make(chan result, len(sourceIndexers))
	var wg sync.WaitGroup

	for sourceID, idxIDs := range sourceIndexers {
		wg.Add(1)
		go func(srcID int64, indexerIDs []int32) {
			defer wg.Done()
			releases, err := m.searchIndexerSource(ctx, srcID, indexerIDs, categories, opts)
			resultChan <- result{releases: releases, err: err}
		}(sourceID, idxIDs)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	var allReleases []*prowlarr.ReleaseResource
	var searchErr error

	for res := range resultChan {
		if res.err != nil {
			log.Error("source search failed", zap.Error(res.err))
			searchErr = errors.Join(searchErr, res.err)
			continue
		}
		allReleases = append(allReleases, res.releases...)
	}

	if len(allReleases) == 0 && searchErr != nil {
		return nil, searchErr
	}

	return allReleases, nil
}

func (m MediaManager) searchIndexerSource(ctx context.Context, sourceID int64, indexerIDs, categories []int32, opts indexer.SearchOptions) ([]*prowlarr.ReleaseResource, error) {
	log := logger.FromCtx(ctx)

	sourceConfig, err := m.storage.GetIndexerSource(ctx, sourceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get source: %w", err)
	}

	source, err := m.indexerFactory.NewIndexerSource(sourceConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create source: %w", err)
	}

	var sourceReleases []*prowlarr.ReleaseResource
	for _, indexerID := range indexerIDs {
		releases, err := source.Search(ctx, indexerID, categories, opts)
		if err != nil {
			log.Error("indexer search failed",
				zap.Int32("indexerID", indexerID),
				zap.Error(err))
			continue
		}
		sourceReleases = append(sourceReleases, releases...)
	}

	return sourceReleases, nil
}

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
		zap.Int32s("indexer_ids", indexerIDs),
		zap.Int32s("categories", categories))

	releases, err := m.SearchIndexers(ctx, indexerIDs, categories, opts)
	if err != nil {
		log.Error("failed to search indexers", zap.Error(err))
		return nil, err
	}

	return releases, nil
}

func (m MediaManager) SearchForMovie(ctx context.Context, movieID int64) error {
	log := logger.FromCtx(ctx).With("movie_id", movieID)
	log.Debug("starting manual search for movie")

	movie, err := m.storage.GetMovie(ctx, movieID)
	if err != nil {
		log.Error("failed to get movie", zap.Error(err))
		return fmt.Errorf("movie not found: %w", err)
	}

	if movie.Monitored == 0 {
		log.Debug("movie is not monitored, cannot search")
		return fmt.Errorf("movie is not monitored")
	}

	snapshot, err := m.prepareSearchSnapshot(ctx)
	if err != nil {
		return err
	}

	err = m.reconcileMissingMovie(ctx, movie, snapshot)
	if err != nil {
		log.Error("failed to search for movie", zap.Error(err))
		return err
	}

	searchTime := now()
	err = m.storage.UpdateMovie(ctx, model.Movie{
		ID:             movie.ID,
		Monitored:      movie.Monitored,
		LastSearchTime: &searchTime,
	}, table.Movie.ID.EQ(sqlite.Int64(movieID)))
	if err != nil {
		log.Warn("failed to update last search time", zap.Error(err))
	}

	log.Debug("manual search completed for movie")
	return nil
}

func (m MediaManager) SearchForSeries(ctx context.Context, seriesID int64) error {
	log := logger.FromCtx(ctx).With("series_id", seriesID)
	log.Debug("starting manual search for series")

	series, err := m.storage.GetSeries(ctx, table.Series.ID.EQ(sqlite.Int32(int32(seriesID))))
	if err != nil {
		log.Error("failed to get series", zap.Error(err))
		return fmt.Errorf("series not found: %w", err)
	}

	if series.Monitored == 0 {
		log.Debug("series is not monitored, cannot search")
		return fmt.Errorf("series is not monitored")
	}

	seasons, err := m.storage.ListSeasons(ctx, table.Season.SeriesID.EQ(sqlite.Int32(series.ID)))
	if err != nil {
		log.Error("failed to list seasons", zap.Error(err))
		return fmt.Errorf("failed to list seasons: %w", err)
	}

	var searchErrors []error
	for _, season := range seasons {
		if season.Monitored == 0 {
			log.Debug("skipping unmonitored season", zap.Int32("season_id", season.ID))
			continue
		}

		err = m.SearchForSeason(ctx, int64(season.ID))
		if err != nil {
			log.Warn("failed to search for season", zap.Error(err), zap.Int32("season_id", season.ID))
			searchErrors = append(searchErrors, err)
		}
	}

	searchTime := now()
	err = m.storage.UpdateSeries(ctx, model.Series{
		ID:             series.ID,
		Monitored:      series.Monitored,
		LastSearchTime: &searchTime,
	}, table.Series.ID.EQ(sqlite.Int32(series.ID)))
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

	season, err := m.storage.GetSeason(ctx, table.Season.ID.EQ(sqlite.Int32(int32(seasonID))))
	if err != nil {
		log.Error("failed to get season", zap.Error(err))
		return fmt.Errorf("season not found: %w", err)
	}

	if season.Monitored == 0 {
		log.Debug("season is not monitored, cannot search")
		return fmt.Errorf("season is not monitored")
	}

	snapshot, err := m.prepareSearchSnapshot(ctx)
	if err != nil {
		return err
	}

	series, err := m.storage.GetSeries(ctx, table.Series.ID.EQ(sqlite.Int32(season.SeriesID)))
	if err != nil {
		log.Error("failed to get series for season", zap.Error(err))
		return fmt.Errorf("failed to get series: %w", err)
	}

	if series.QualityProfileID == 0 {
		log.Warn("series quality profile id is nil, skipping search")
		return fmt.Errorf("series has no quality profile")
	}

	qualityProfile, err := m.storage.GetQualityProfile(ctx, int64(series.QualityProfileID))
	if err != nil {
		log.Error("failed to get quality profile", zap.Error(err))
		return fmt.Errorf("failed to get quality profile: %w", err)
	}

	if series.SeriesMetadataID == nil {
		log.Error("series has no metadata ID")
		return fmt.Errorf("series has no metadata")
	}

	seriesMetadata, err := m.storage.GetSeriesMetadata(ctx, table.SeriesMetadata.ID.EQ(sqlite.Int32(*series.SeriesMetadataID)))
	if err != nil {
		log.Error("failed to get series metadata", zap.Error(err))
		return fmt.Errorf("failed to get series metadata: %w", err)
	}

	if seriesMetadata.Title == "" {
		log.Error("series metadata has empty title")
		return fmt.Errorf("series has no title")
	}

	searchType := indexer.TypeTV
	releases, err := m.executeSearch(ctx, snapshot, TV_CATEGORIES, indexer.SearchOptions{
		Query:  seriesMetadata.Title,
		Season: &season.SeasonNumber,
		Type:   &searchType,
	})
	if err != nil {
		return err
	}

	err = m.reconcileMissingSeason(ctx, seriesMetadata.Title, season, snapshot, qualityProfile, releases)
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

	episode, err := m.storage.GetEpisode(ctx, table.Episode.ID.EQ(sqlite.Int32(int32(episodeID))))
	if err != nil {
		log.Error("failed to get episode", zap.Error(err))
		return fmt.Errorf("episode not found: %w", err)
	}

	if episode.Monitored == 0 {
		log.Debug("episode is not monitored, cannot search")
		return fmt.Errorf("episode is not monitored")
	}

	season, err := m.storage.GetSeason(ctx, table.Season.ID.EQ(sqlite.Int32(episode.SeasonID)))
	if err != nil {
		log.Error("failed to get season for episode", zap.Error(err))
		return fmt.Errorf("season not found: %w", err)
	}

	snapshot, err := m.prepareSearchSnapshot(ctx)
	if err != nil {
		return err
	}

	series, err := m.storage.GetSeries(ctx, table.Series.ID.EQ(sqlite.Int32(season.SeriesID)))
	if err != nil {
		log.Error("failed to get series for episode", zap.Error(err))
		return fmt.Errorf("failed to get series: %w", err)
	}

	if series.QualityProfileID == 0 {
		log.Warn("series quality profile id is nil, skipping search")
		return fmt.Errorf("series has no quality profile")
	}

	qualityProfile, err := m.storage.GetQualityProfile(ctx, int64(series.QualityProfileID))
	if err != nil {
		log.Error("failed to get quality profile", zap.Error(err))
		return fmt.Errorf("failed to get quality profile: %w", err)
	}

	if series.SeriesMetadataID == nil {
		log.Error("series has no metadata ID")
		return fmt.Errorf("series has no metadata")
	}

	seriesMetadata, err := m.storage.GetSeriesMetadata(ctx, table.SeriesMetadata.ID.EQ(sqlite.Int32(*series.SeriesMetadataID)))
	if err != nil {
		log.Error("failed to get series metadata", zap.Error(err))
		return fmt.Errorf("failed to get series metadata: %w", err)
	}

	if seriesMetadata.Title == "" {
		log.Error("series metadata has empty title")
		return fmt.Errorf("series has no title")
	}

	searchType := indexer.TypeTV
	releases, err := m.executeSearch(ctx, snapshot, TV_CATEGORIES, indexer.SearchOptions{
		Query:   seriesMetadata.Title,
		Season:  &season.SeasonNumber,
		Episode: &episode.EpisodeNumber,
		Type:    &searchType,
	})
	if err != nil {
		return err
	}

	_, err = m.reconcileMissingEpisode(ctx, seriesMetadata.Title, season.SeasonNumber, episode, snapshot, qualityProfile, releases)
	if err != nil {
		log.Error("failed to reconcile episode", zap.Error(err))
		return err
	}

	log.Debug("manual search completed for episode")
	return nil
}
