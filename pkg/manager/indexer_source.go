package manager

import (
	"context"
	"errors"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/pkg/indexer"
	"github.com/kasuboski/mediaz/pkg/logger"
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

func (m MediaManager) CreateIndexerSource(ctx context.Context, req AddIndexerSourceRequest) (IndexerSourceResponse, error) {
	source := model.IndexerSource{
		Name:           req.Name,
		Implementation: req.Implementation,
		Scheme:         req.Scheme,
		Host:           req.Host,
		Port:           req.Port,
		APIKey:         req.APIKey,
		Enabled:        req.Enabled,
	}

	id, err := m.storage.CreateIndexerSource(ctx, source)
	if err != nil {
		return IndexerSourceResponse{}, err
	}

	source.ID = int32(id)

	if source.Enabled {
		m.RefreshIndexerSource(ctx, id)
	}

	return toIndexerSourceResponse(source), nil
}

func (m MediaManager) ListIndexerSources(ctx context.Context) ([]IndexerSourceResponse, error) {
	sources, err := m.storage.ListIndexerSources(ctx)
	if err != nil {
		return nil, err
	}

	responses := make([]IndexerSourceResponse, len(sources))
	for i, src := range sources {
		responses[i] = toIndexerSourceResponse(*src)
	}

	return responses, nil
}

func (m MediaManager) GetIndexerSource(ctx context.Context, id int64) (IndexerSourceResponse, error) {
	source, err := m.storage.GetIndexerSource(ctx, id)
	if err != nil {
		return IndexerSourceResponse{}, err
	}

	return toIndexerSourceResponse(source), nil
}

func (m MediaManager) UpdateIndexerSource(ctx context.Context, id int64, req UpdateIndexerSourceRequest) (IndexerSourceResponse, error) {
	existing, err := m.storage.GetIndexerSource(ctx, id)
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

	if err := m.storage.UpdateIndexerSource(ctx, id, source); err != nil {
		return IndexerSourceResponse{}, err
	}

	if source.Enabled {
		m.RefreshIndexerSource(ctx, id)
	}

	return toIndexerSourceResponse(source), nil
}

func (m MediaManager) DeleteIndexerSource(ctx context.Context, id int64) error {
	where := table.Indexer.IndexerSourceID.EQ(sqlite.Int64(id))
	indexers, err := m.storage.ListIndexers(ctx, where)
	if err != nil {
		return err
	}

	for _, idx := range indexers {
		if err := m.storage.DeleteIndexer(ctx, int64(idx.ID)); err != nil {
			return err
		}
	}

	m.indexerCache.Delete(id)

	return m.storage.DeleteIndexerSource(ctx, id)
}

func (m MediaManager) TestIndexerSource(ctx context.Context, req AddIndexerSourceRequest) error {
	source := model.IndexerSource{
		Name:           req.Name,
		Implementation: req.Implementation,
		Scheme:         req.Scheme,
		Host:           req.Host,
		Port:           req.Port,
		APIKey:         req.APIKey,
		Enabled:        true,
	}

	src, err := m.indexerFactory.NewIndexerSource(source)
	if err != nil {
		return err
	}

	_, err = src.ListIndexers(ctx)
	return err
}

func (m MediaManager) RefreshIndexerSource(ctx context.Context, id int64) error {
	log := logger.FromCtx(ctx)

	sourceConfig, err := m.storage.GetIndexerSource(ctx, id)
	if err != nil {
		return err
	}

	if !sourceConfig.Enabled {
		return nil
	}

	source, err := m.indexerFactory.NewIndexerSource(sourceConfig)
	if err != nil {
		return err
	}

	indexers, err := source.ListIndexers(ctx)
	if err != nil {
		return err
	}

	sourceURI := sourceConfig.Scheme + "://" + sourceConfig.Host

	m.indexerCache.Set(id, indexerCacheEntry{
		Indexers:   indexers,
		SourceName: sourceConfig.Name,
		SourceURI:  sourceURI,
	})

	log.Debug("refreshed indexers from source",
		zap.Int64("sourceID", id),
		zap.Int("count", len(indexers)))

	return nil
}

func (m MediaManager) RefreshAllIndexerSources(ctx context.Context) error {
	log := logger.FromCtx(ctx)

	sources, err := m.storage.ListIndexerSources(ctx, table.IndexerSource.Enabled.EQ(sqlite.Bool(true)))
	if err != nil {
		return err
	}

	var allErrors error
	for _, src := range sources {
		if err := m.RefreshIndexerSource(ctx, int64(src.ID)); err != nil {
			log.Error("failed to refresh source", zap.Int32("sourceID", src.ID), zap.Error(err))
			allErrors = errors.Join(allErrors, err)
		}
	}

	return allErrors
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
