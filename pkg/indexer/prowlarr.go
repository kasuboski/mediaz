package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
)

type ProwlarrIndexerSource struct {
	client prowlarr.IProwlarr
	config model.IndexerSource
}

func NewProwlarrSource(config model.IndexerSource) (*ProwlarrIndexerSource, error) {
	if config.APIKey == nil {
		return nil, fmt.Errorf("prowlarr source requires api_key")
	}

	url := fmt.Sprintf("%s://%s", config.Scheme, config.Host)
	if config.Port != nil && *config.Port != 80 && *config.Port != 443 {
		url = fmt.Sprintf("%s:%d", url, *config.Port)
	}

	client, err := prowlarr.New(url, *config.APIKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create prowlarr client: %w", err)
	}

	return &ProwlarrIndexerSource{
		client: client,
		config: config,
	}, nil
}

func (p *ProwlarrIndexerSource) ListIndexers(ctx context.Context) ([]SourceIndexer, error) {
	resp, err := p.client.GetAPIV1Indexer(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch indexers: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var prowlarrIndexers []prowlarr.IndexerResource
	if err := json.Unmarshal(body, &prowlarrIndexers); err != nil {
		return nil, err
	}

	indexers := make([]SourceIndexer, 0, len(prowlarrIndexers))
	for _, pi := range prowlarrIndexers {
		name, _ := pi.Name.Get()

		var uri string
		if pi.IndexerUrls.IsSpecified() {
			urls, _ := pi.IndexerUrls.Get()
			if len(urls) > 0 {
				uri = urls[0]
			}
		}

		var categories []prowlarr.IndexerCategory
		if pi.Capabilities.Categories.IsSpecified() {
			cats, _ := pi.Capabilities.Categories.Get()
			categories = cats
		}

		indexers = append(indexers, SourceIndexer{
			ID:         *pi.ID,
			Name:       name,
			URI:        uri,
			Priority:   *pi.Priority,
			Categories: categories,
			Status:     pi.Status,
		})
	}

	return indexers, nil
}

func (p *ProwlarrIndexerSource) Search(ctx context.Context, indexerID int32, categories []int32, query string) ([]*prowlarr.ReleaseResource, error) {
	resp, err := p.client.GetAPIV1Search(ctx, &prowlarr.GetAPIV1SearchParams{
		IndexerIds: &[]int32{indexerID},
		Query:      &query,
		Categories: &categories,
		Limit:      ptr(int32(100)),
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var releases []*prowlarr.ReleaseResource
	if err := json.Unmarshal(body, &releases); err != nil {
		return nil, err
	}

	return releases, nil
}

func ptr[T any](v T) *T {
	return &v
}
