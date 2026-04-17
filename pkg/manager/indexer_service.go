package manager

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/pkg/cache"
	"github.com/kasuboski/mediaz/pkg/indexer"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/ptr"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
	"go.uber.org/zap"
)

type indexerCacheEntry struct {
	Indexers   []indexer.SourceIndexer
	SourceName string
	SourceURI  string
}

type AddIndexerSourceRequest struct {
	Name           string  `json:"name"`
	Implementation string  `json:"implementation"`
	Scheme         string  `json:"scheme"`
	Host           string  `json:"host"`
	Port           *int32  `json:"port,omitempty"`
	APIKey         *string `json:"apiKey,omitempty"`
	Enabled        bool    `json:"enabled"`
}

type UpdateIndexerSourceRequest struct {
	Name           string  `json:"name"`
	Implementation string  `json:"implementation"`
	Scheme         string  `json:"scheme"`
	Host           string  `json:"host"`
	Port           *int32  `json:"port,omitempty"`
	APIKey         *string `json:"apiKey,omitempty"`
	Enabled        bool    `json:"enabled"`
}

type IndexerSourceResponse struct {
	ID             int32  `json:"id"`
	Name           string `json:"name"`
	Implementation string `json:"implementation"`
	Scheme         string `json:"scheme"`
	Host           string `json:"host"`
	Port           *int32 `json:"port,omitempty"`
	Enabled        bool   `json:"enabled"`
}

type IndexerService struct {
	indexerStorage    storage.IndexerStorage
	indexerSrcStorage storage.IndexerSourceStorage
	indexerFactory    indexer.Factory
	indexerCache      *cache.Cache[int64, indexerCacheEntry]
}

func NewIndexerService(
	indexerStorage storage.IndexerStorage,
	indexerSrcStorage storage.IndexerSourceStorage,
	indexerFactory indexer.Factory,
) *IndexerService {
	return &IndexerService{
		indexerStorage:    indexerStorage,
		indexerSrcStorage: indexerSrcStorage,
		indexerFactory:    indexerFactory,
		indexerCache:      cache.New[int64, indexerCacheEntry](),
	}
}

func (is IndexerService) listIndexersInternal(ctx context.Context) ([]model.Indexer, error) {
	var all []model.Indexer

	keys := is.indexerCache.Keys()
	for _, sourceID := range keys {
		cached, ok := is.indexerCache.Get(sourceID)
		if !ok {
			continue
		}

		for _, idx := range cached.Indexers {
			all = append(all, model.Indexer{
				ID:              idx.ID,
				IndexerSourceID: ptr.To(int32(sourceID)),
				Name:            idx.Name,
				Priority:        idx.Priority,
				URI:             idx.URI,
				APIKey:          nil,
			})
		}
	}

	dbIndexers, err := is.indexerStorage.ListIndexers(ctx)
	if err != nil {
		return nil, err
	}

	for _, idx := range dbIndexers {
		if idx.IndexerSourceID == nil {
			all = append(all, *idx)
		}
	}

	return all, nil
}

func (is IndexerService) ListIndexers(ctx context.Context) ([]IndexerResponse, error) {
	var all []IndexerResponse

	keys := is.indexerCache.Keys()
	for _, sourceID := range keys {
		cached, ok := is.indexerCache.Get(sourceID)
		if !ok {
			continue
		}

		for _, idx := range cached.Indexers {
			all = append(all, IndexerResponse{
				ID:       idx.ID,
				Name:     idx.Name,
				Source:   cached.SourceName,
				Priority: idx.Priority,
				URI:      idx.URI,
			})
		}
	}

	dbIndexers, err := is.indexerStorage.ListIndexers(ctx)
	if err != nil {
		return nil, err
	}

	for _, idx := range dbIndexers {
		if idx.IndexerSourceID == nil {
			all = append(all, IndexerResponse{
				ID:       idx.ID,
				Name:     idx.Name,
				Source:   "Internal",
				Priority: idx.Priority,
				URI:      idx.URI,
			})
		}
	}

	return all, nil
}

func (is IndexerService) AddIndexer(ctx context.Context, request AddIndexerRequest) (IndexerResponse, error) {
	idx := request.Indexer

	if idx.Name == "" {
		return IndexerResponse{}, fmt.Errorf("%w: indexer name is required", ErrValidation)
	}

	id, err := is.indexerStorage.CreateIndexer(ctx, idx)
	if err != nil {
		return IndexerResponse{}, err
	}

	idx.ID = int32(id)

	return toIndexerResponse(idx), nil
}

func (is IndexerService) UpdateIndexer(ctx context.Context, id int32, request UpdateIndexerRequest) (IndexerResponse, error) {
	if request.Name == "" {
		return IndexerResponse{}, fmt.Errorf("%w: indexer name is required", ErrValidation)
	}

	idx := model.Indexer{
		ID:       id,
		Name:     request.Name,
		Priority: request.Priority,
		URI:      request.URI,
		APIKey:   request.APIKey,
	}

	if err := is.indexerStorage.UpdateIndexer(ctx, int64(id), idx); err != nil {
		return IndexerResponse{}, err
	}

	return toIndexerResponse(idx), nil
}

func (is IndexerService) DeleteIndexer(ctx context.Context, request DeleteIndexerRequest) error {
	if request.ID == nil {
		return fmt.Errorf("indexer id is required")
	}

	return is.indexerStorage.DeleteIndexer(ctx, int64(*request.ID))
}

func (is IndexerService) CreateIndexerSource(ctx context.Context, req AddIndexerSourceRequest) (IndexerSourceResponse, error) {
	source := model.IndexerSource{
		Name:           req.Name,
		Implementation: req.Implementation,
		Scheme:         req.Scheme,
		Host:           req.Host,
		Port:           req.Port,
		APIKey:         req.APIKey,
		Enabled:        req.Enabled,
	}

	id, err := is.indexerSrcStorage.CreateIndexerSource(ctx, source)
	if err != nil {
		return IndexerSourceResponse{}, err
	}

	source.ID = int32(id)

	if source.Enabled {
		is.RefreshIndexerSource(ctx, id)
	}

	return toIndexerSourceResponse(source), nil
}

func (is IndexerService) ListIndexerSources(ctx context.Context) ([]IndexerSourceResponse, error) {
	sources, err := is.indexerSrcStorage.ListIndexerSources(ctx)
	if err != nil {
		return nil, err
	}

	responses := make([]IndexerSourceResponse, len(sources))
	for i, src := range sources {
		responses[i] = toIndexerSourceResponse(*src)
	}

	return responses, nil
}

func (is IndexerService) GetIndexerSource(ctx context.Context, id int64) (IndexerSourceResponse, error) {
	source, err := is.indexerSrcStorage.GetIndexerSource(ctx, id)
	if err != nil {
		return IndexerSourceResponse{}, err
	}

	return toIndexerSourceResponse(source), nil
}

func (is IndexerService) UpdateIndexerSource(ctx context.Context, id int64, req UpdateIndexerSourceRequest) (IndexerSourceResponse, error) {
	existing, err := is.indexerSrcStorage.GetIndexerSource(ctx, id)
	if err != nil {
		return IndexerSourceResponse{}, err
	}

	apiKey := req.APIKey
	if apiKey == nil || *apiKey == "" {
		apiKey = existing.APIKey
	}

	source := model.IndexerSource{
		ID:             int32(id),
		Name:           req.Name,
		Implementation: req.Implementation,
		Scheme:         req.Scheme,
		Host:           req.Host,
		Port:           req.Port,
		APIKey:         apiKey,
		Enabled:        req.Enabled,
	}

	if err := is.indexerSrcStorage.UpdateIndexerSource(ctx, id, source); err != nil {
		return IndexerSourceResponse{}, err
	}

	if source.Enabled {
		is.RefreshIndexerSource(ctx, id)
	}

	return toIndexerSourceResponse(source), nil
}

func (is IndexerService) DeleteIndexerSource(ctx context.Context, id int64) error {
	where := table.Indexer.IndexerSourceID.EQ(sqlite.Int64(id))
	indexers, err := is.indexerStorage.ListIndexers(ctx, where)
	if err != nil {
		return err
	}

	for _, idx := range indexers {
		if err := is.indexerStorage.DeleteIndexer(ctx, int64(idx.ID)); err != nil {
			return err
		}
	}

	is.indexerCache.Delete(id)

	return is.indexerSrcStorage.DeleteIndexerSource(ctx, id)
}

func (is IndexerService) TestIndexerSource(ctx context.Context, req AddIndexerSourceRequest) error {
	source := model.IndexerSource{
		Name:           req.Name,
		Implementation: req.Implementation,
		Scheme:         req.Scheme,
		Host:           req.Host,
		Port:           req.Port,
		APIKey:         req.APIKey,
		Enabled:        true,
	}

	src, err := is.indexerFactory.NewIndexerSource(source)
	if err != nil {
		return err
	}

	_, err = src.ListIndexers(ctx)
	return err
}

func (is IndexerService) RefreshIndexerSource(ctx context.Context, id int64) error {
	log := logger.FromCtx(ctx)

	sourceConfig, err := is.indexerSrcStorage.GetIndexerSource(ctx, id)
	if err != nil {
		return err
	}

	if !sourceConfig.Enabled {
		return nil
	}

	source, err := is.indexerFactory.NewIndexerSource(sourceConfig)
	if err != nil {
		return err
	}

	indexers, err := source.ListIndexers(ctx)
	if err != nil {
		return err
	}

	sourceURI := sourceConfig.Scheme + "://" + sourceConfig.Host

	is.indexerCache.Set(id, indexerCacheEntry{
		Indexers:   indexers,
		SourceName: sourceConfig.Name,
		SourceURI:  sourceURI,
	})

	log.Debug("refreshed indexers from source",
		zap.Int64("sourceID", id),
		zap.Int("count", len(indexers)))

	return nil
}

func (is IndexerService) RefreshAllIndexerSources(ctx context.Context) error {
	log := logger.FromCtx(ctx)

	sources, err := is.indexerSrcStorage.ListIndexerSources(ctx, table.IndexerSource.Enabled.EQ(sqlite.Bool(true)))
	if err != nil {
		return err
	}

	var allErrors error
	for _, src := range sources {
		if err := is.RefreshIndexerSource(ctx, int64(src.ID)); err != nil {
			log.Error("failed to refresh source", zap.Int32("sourceID", src.ID), zap.Error(err))
			allErrors = errors.Join(allErrors, err)
		}
	}

	return allErrors
}

func (is IndexerService) SearchIndexers(ctx context.Context, indexers, categories []int32, opts indexer.SearchOptions) ([]*prowlarr.ReleaseResource, error) {
	log := logger.FromCtx(ctx)

	sourceIndexers := make(map[int64][]int32)

	keys := is.indexerCache.Keys()
	for _, sourceID := range keys {
		cached, ok := is.indexerCache.Get(sourceID)
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
			releases, err := is.searchIndexerSource(ctx, srcID, indexerIDs, categories, opts)
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

func (is IndexerService) searchIndexerSource(ctx context.Context, sourceID int64, indexerIDs, categories []int32, opts indexer.SearchOptions) ([]*prowlarr.ReleaseResource, error) {
	log := logger.FromCtx(ctx)

	sourceConfig, err := is.indexerSrcStorage.GetIndexerSource(ctx, sourceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get source: %w", err)
	}

	source, err := is.indexerFactory.NewIndexerSource(sourceConfig)
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

func toIndexerResponse(idx model.Indexer) IndexerResponse {
	return IndexerResponse{
		ID:       idx.ID,
		Name:     idx.Name,
		Priority: idx.Priority,
		URI:      idx.URI,
	}
}

func toIndexerSourceResponse(src model.IndexerSource) IndexerSourceResponse {
	return IndexerSourceResponse{
		ID:             src.ID,
		Name:           src.Name,
		Implementation: src.Implementation,
		Scheme:         src.Scheme,
		Host:           src.Host,
		Port:           src.Port,
		Enabled:        src.Enabled,
	}
}
