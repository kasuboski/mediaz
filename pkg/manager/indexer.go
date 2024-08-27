package manager

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"sync"

	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/oapi-codegen/nullable"
	"go.uber.org/zap"
)

type IndexerStatus = prowlarr.IndexerStatusResource

type IndexerStore struct {
	prowlarr prowlarr.ClientInterface
	mutex    *sync.RWMutex
	indexers map[int32]*Indexer
}

func NewIndexerStore(prowlarr prowlarr.ClientInterface) IndexerStore {
	return IndexerStore{
		indexers: make(map[int32]*Indexer),
		prowlarr: prowlarr,
		mutex:    new(sync.RWMutex),
	}
}

type Indexer struct {
	Categories nullable.Nullable[[]prowlarr.IndexerCategory] `json:"categories,omitempty"`
	Status     nullable.Nullable[IndexerStatus]              `json:"status,omitempty"`
	Name       string                                        `json:"name"`
	ID         int32                                         `json:"id"`
	Priority   int32                                         `json:"priority"`
}

func (i *IndexerStore) ListIndexers(ctx context.Context) []Indexer {
	i.mutex.RLock()
	defer i.mutex.RUnlock()

	ret := make([]Indexer, 0, len(i.indexers))
	for _, v := range i.indexers {
		ret = append(ret, *v)
	}
	sortIndexers(ret)
	return ret
}

func (i *IndexerStore) FetchIndexers(ctx context.Context) error {
	log := logger.FromCtx(ctx)

	resp, err := i.prowlarr.GetAPIV1Indexer(ctx)
	if err != nil {
		log.Debug("failed to list indexers", zap.Error(err))
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var indexers []prowlarr.IndexerResource
	err = json.Unmarshal(b, &indexers)
	if err != nil {
		return err
	}

	i.mutex.Lock()
	defer i.mutex.Unlock()

	for _, indexer := range indexers {
		ret, err := FromProwlarrIndexer(indexer)
		if err != nil {
			return err
		}

		i.indexers[ret.ID] = ret
	}
	return nil
}

func (i *IndexerStore) searchIndexer(ctx context.Context, indexer int32, categories []int32, query string) ([]*prowlarr.ReleaseResource, error) {
	log := logger.FromCtx(ctx)

	resp, err := i.prowlarr.GetAPIV1Search(ctx, &prowlarr.GetAPIV1SearchParams{
		IndexerIds: &[]int32{indexer},
		Query:      &query,
		Categories: &categories,
		Limit:      intPtr(100),
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		log.Debug("unexpected response status", zap.Any("status", resp.Status), zap.String("body", string(b)))
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	var releases []*prowlarr.ReleaseResource
	err = json.Unmarshal(b, &releases)
	return releases, err
}

func intPtr(in int) *int32 {
	ret := int32(in)
	return &ret
}

func FromProwlarrIndexer(prowlarr prowlarr.IndexerResource) (*Indexer, error) {
	name, err := prowlarr.Name.Get()
	if err != nil {
		return nil, fmt.Errorf("indexer didn't have a name: %w", err)
	}
	return &Indexer{
		ID:         *prowlarr.ID,
		Name:       name,
		Categories: prowlarr.Capabilities.Categories,
		Priority:   *prowlarr.Priority,
		Status:     nullableStatus(prowlarr.Status),
	}, nil
}

func nullableStatus(s *prowlarr.IndexerStatusResource) nullable.Nullable[IndexerStatus] {
	if s == nil {
		return nullable.NewNullNullable[IndexerStatus]()
	}

	return nullable.NewNullableWithValue(*s)
}

func sortIndexers(indexers []Indexer) {
	slices.SortFunc(indexers, func(a, b Indexer) int {
		return cmp.Compare(a.Priority, b.Priority)
	})
}
