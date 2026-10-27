package manager

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/kasuboski/mediaz/pkg/cache"
	"github.com/kasuboski/mediaz/pkg/indexer"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/storage"
	sqlcdb "github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"go.uber.org/zap"
)

type indexerCacheEntry struct {
	Indexers   []indexer.SourceIndexer
	SourceName string
	SourceURI  string
}

type AddIndexerSourceRequest struct {
	Name           string  `json:"name" validate:"required"`
	Implementation string  `json:"implementation" validate:"required"`
	Scheme         string  `json:"scheme" validate:"required"`
	Host           string  `json:"host" validate:"required"`
	Port           *int32  `json:"port,omitempty"`
	APIKey         *string `json:"apiKey,omitempty"`
	Enabled        bool    `json:"enabled"`
}

type UpdateIndexerSourceRequest struct {
	Name           string  `json:"name" validate:"required"`
	Implementation string  `json:"implementation" validate:"required"`
	Scheme         string  `json:"scheme" validate:"required"`
	Host           string  `json:"host" validate:"required"`
	Port           *int32  `json:"port,omitempty"`
	APIKey         *string `json:"apiKey,omitempty"`
	Enabled        bool    `json:"enabled"`
}

type IndexerSourceResponse struct {
	ID             int64  `json:"id"`
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

func (is IndexerService) listIndexersInternal(ctx context.Context) ([]sqlcdb.Indexer, error) {
	var all []sqlcdb.Indexer

	keys := is.indexerCache.Keys()
	for _, sourceID := range keys {
		cached, ok := is.indexerCache.Get(sourceID)
		if !ok {
			continue
		}

		for _, idx := range cached.Indexers {
			all = append(all, sqlcdb.Indexer{
				ID:              int64(idx.ID),
				IndexerSourceID: sql.NullInt64{Int64: sourceID, Valid: true},
				Name:            idx.Name,
				Priority:        int64(idx.Priority),
				Uri:             idx.URI,
				ApiKey:          sql.NullString{},
				Categories:      categoriesJSON(categoryIDs(idx.Categories)),
			})
		}
	}

	dbIndexers, err := is.indexerStorage.ListIndexers(ctx)
	if err != nil {
		return nil, err
	}

	for _, idx := range dbIndexers {
		if !idx.IndexerSourceID.Valid {
			all = append(all, idx)
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
				ID:       int64(idx.ID),
				Name:     idx.Name,
				Source:   cached.SourceName,
				Priority: int64(idx.Priority),
				URI:      idx.URI,
			})
		}
	}

	dbIndexers, err := is.indexerStorage.ListIndexers(ctx)
	if err != nil {
		return nil, err
	}

	for _, idx := range dbIndexers {
		if !idx.IndexerSourceID.Valid {
			all = append(all, IndexerResponse{
				ID:       idx.ID,
				Name:     idx.Name,
				Source:   "Internal",
				Priority: idx.Priority,
				URI:      idx.Uri,
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

	idx.ID = id

	return toIndexerResponse(idx), nil
}

func (is IndexerService) UpdateIndexer(ctx context.Context, id int64, request UpdateIndexerRequest) (IndexerResponse, error) {
	if request.Name == "" {
		return IndexerResponse{}, fmt.Errorf("%w: indexer name is required", ErrValidation)
	}

	idx := sqlcdb.Indexer{
		ID:       int64(id),
		Name:     request.Name,
		Priority: int64(request.Priority),
		Uri:      request.URI,
		ApiKey:   nullStrIn(request.APIKey),
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
	source := sqlcdb.IndexerSource{
		Name:           req.Name,
		Implementation: req.Implementation,
		Scheme:         req.Scheme,
		Host:           req.Host,
		Port:           nullI64In(req.Port),
		ApiKey:         nullStrIn(req.APIKey),
		Enabled:        req.Enabled,
	}

	id, err := is.indexerSrcStorage.CreateIndexerSource(ctx, source)
	if err != nil {
		return IndexerSourceResponse{}, err
	}

	source.ID = id

	if !source.Enabled {
		is.indexerCache.Delete(id)
		return toIndexerSourceResponse(source), nil
	}

	if err := is.RefreshIndexerSource(ctx, id); err != nil {
		return IndexerSourceResponse{}, err
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
		responses[i] = toIndexerSourceResponse(src)
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

	apiKey := nullStrIn(req.APIKey)
	if !apiKey.Valid || apiKey.String == "" {
		apiKey = existing.ApiKey
	}

	source := sqlcdb.IndexerSource{
		ID:             id,
		Name:           req.Name,
		Implementation: req.Implementation,
		Scheme:         req.Scheme,
		Host:           req.Host,
		Port:           nullI64In(req.Port),
		ApiKey:         apiKey,
		Enabled:        req.Enabled,
	}

	if err := is.indexerSrcStorage.UpdateIndexerSource(ctx, id, source); err != nil {
		return IndexerSourceResponse{}, err
	}

	if !source.Enabled {
		is.indexerCache.Delete(id)
		return toIndexerSourceResponse(source), nil
	}

	if err := is.RefreshIndexerSource(ctx, id); err != nil {
		return IndexerSourceResponse{}, err
	}

	return toIndexerSourceResponse(source), nil
}

func (is IndexerService) DeleteIndexerSource(ctx context.Context, id int64) error {
	if err := is.indexerSrcStorage.DeleteIndexerSourceCascade(ctx, id); err != nil {
		return err
	}

	is.indexerCache.Delete(id)
	return nil
}

func (is IndexerService) TestIndexerSource(ctx context.Context, req AddIndexerSourceRequest) error {
	source := sqlcdb.IndexerSource{
		Name:           req.Name,
		Implementation: req.Implementation,
		Scheme:         req.Scheme,
		Host:           req.Host,
		Port:           nullI64In(req.Port),
		ApiKey:         nullStrIn(req.APIKey),
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

	for _, idx := range indexers {
		categories := categoryIDs(idx.Categories)
		if _, err := is.indexerStorage.CreateIndexer(ctx, sqlcdb.Indexer{
			Name:            idx.Name,
			Priority:        int64(idx.Priority),
			Uri:             idx.URI,
			IndexerSourceID: sql.NullInt64{Int64: id, Valid: true},
			Categories:      categoriesJSON(categories),
		}); err != nil {
			return fmt.Errorf("failed to persist indexer %q: %w", idx.Name, err)
		}
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

	sources, err := is.indexerSrcStorage.ListEnabledIndexerSources(ctx)
	if err != nil {
		return err
	}

	var allErrors error
	for _, src := range sources {
		if err := is.RefreshIndexerSource(ctx, int64(src.ID)); err != nil {
			log.Error("failed to refresh source", zap.Int64("sourceID", src.ID), zap.Error(err))
			allErrors = errors.Join(allErrors, err)
		}
	}

	return allErrors
}

// sourceSearchTimeout is the per-source timeout for indexer search operations.
// If any single source goroutine exceeds this duration it is cancelled and
// its timeout error is aggregated with any other source errors, preventing a
// slow or unresponsive source from hanging the entire SearchIndexers call.
const sourceSearchTimeout = 30 * time.Second

func (is IndexerService) SearchIndexers(ctx context.Context, indexers []int64, categories []int32, opts indexer.SearchOptions) ([]*prowlarr.ReleaseResource, error) {
	log := logger.FromCtx(ctx)

	sourceIndexers := make(map[int64][]int64)

	keys := is.indexerCache.Keys()
	for _, sourceID := range keys {
		cached, ok := is.indexerCache.Get(sourceID)
		if !ok {
			continue
		}

		for _, idx := range cached.Indexers {
			for _, id := range indexers {
				if int64(idx.ID) == id {
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
		go func(srcID int64, indexerIDs []int64) {
			defer wg.Done()
			ctxWithTimeout, cancel := context.WithTimeout(ctx, sourceSearchTimeout)
			defer cancel()
			releases, err := is.searchIndexerSource(ctxWithTimeout, srcID, indexerIDs, categories, opts)
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

	return allReleases, searchErr
}

func (is IndexerService) searchIndexerSource(ctx context.Context, sourceID int64, indexerIDs []int64, categories []int32, opts indexer.SearchOptions) ([]*prowlarr.ReleaseResource, error) {
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
	cached, _ := is.indexerCache.Get(sourceID)
	categoriesByID := make(map[int32][]int32, len(cached.Indexers))
	for _, idx := range cached.Indexers {
		categoriesByID[idx.ID] = categoryIDs(idx.Categories)
	}
	storedIndexers, err := is.indexerStorage.ListIndexers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list stored indexers: %w", err)
	}
	for _, idx := range cached.Indexers {
		for _, stored := range storedIndexers {
			if stored.IndexerSourceID.Valid && stored.IndexerSourceID.Int64 == sourceID && stored.Name == idx.Name {
				if stored.Categories != "" {
					var categories []int32
					if err := json.Unmarshal([]byte(stored.Categories), &categories); err != nil {
						return nil, fmt.Errorf("failed to decode categories for indexer %q: %w", idx.Name, err)
					}
					categoriesByID[idx.ID] = categories
				}
				break
			}
		}
	}
	for _, indexerID := range indexerIDs {
		if indexerID < 0 || indexerID > math.MaxInt32 {
			return nil, fmt.Errorf("indexer ID %d exceeds the Prowlarr API range", indexerID)
		}
		indexerCategories := categoriesByID[int32(indexerID)]
		if len(indexerCategories) == 0 {
			indexerCategories = categories
		}
		releases, err := source.Search(ctx, int32(indexerID), indexerCategories, opts)
		if err != nil {
			log.Error("indexer search failed",
				zap.Int64("indexerID", indexerID),
				zap.Error(err))
			continue
		}
		sourceReleases = append(sourceReleases, releases...)
	}

	return sourceReleases, nil
}

func categoryIDs(categories []prowlarr.IndexerCategory) []int32 {
	var ids []int32
	for _, category := range categories {
		if category.ID != nil {
			ids = append(ids, *category.ID)
		}
		if subcategories, err := category.SubCategories.Get(); err == nil {
			ids = append(ids, categoryIDs(subcategories)...)
		}
	}
	return ids
}

func categoriesJSON(categories []int32) string {
	if categories == nil {
		categories = []int32{}
	}
	encoded, _ := json.Marshal(categories)
	return string(encoded)
}

func toIndexerResponse(idx sqlcdb.Indexer) IndexerResponse {
	return IndexerResponse{
		ID:       idx.ID,
		Name:     idx.Name,
		Priority: idx.Priority,
		URI:      idx.Uri,
	}
}

func toIndexerSourceResponse(src sqlcdb.IndexerSource) IndexerSourceResponse {
	var port *int32
	if src.Port.Valid {
		value := int32(src.Port.Int64)
		port = &value
	}
	return IndexerSourceResponse{
		ID:             src.ID,
		Name:           src.Name,
		Implementation: src.Implementation,
		Scheme:         src.Scheme,
		Host:           src.Host,
		Port:           port,
		Enabled:        src.Enabled,
	}
}

// nullStrIn converts an optional string to a nullable string column value.
func nullStrIn(v *string) sql.NullString {
	if v == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *v, Valid: true}
}

// nullI64In converts an optional int32 to a nullable int64 column value.
func nullI64In(v *int32) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*v), Valid: true}
}

// nullTimeIn converts an optional time to a nullable time column value.
func nullTimeIn(v *time.Time) sql.NullTime {
	if v == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *v, Valid: true}
}

// nullTimeOut converts a nullable time column value to an optional time.
func nullTimeOut(v sql.NullTime) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time
	return &t
}
