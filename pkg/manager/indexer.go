package manager

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"

	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/model"
	"github.com/oapi-codegen/nullable"
	"go.uber.org/zap"
)

type IndexerStatus = prowlarr.IndexerStatusResource

type IndexerStore struct {
	prowlarr prowlarr.IProwlarr
	storage  storage.IndexerStorage
}

func NewIndexerStore(prowlarr prowlarr.IProwlarr, storage storage.IndexerStorage) IndexerStore {
	return IndexerStore{
		prowlarr,
		storage,
	}
}

type Indexer struct {
	Categories nullable.Nullable[[]prowlarr.IndexerCategory] `json:"categories,omitempty"`
	Status     nullable.Nullable[IndexerStatus]              `json:"status,omitempty"`
	Name       string                                        `json:"name"`
	ID         int32                                         `json:"id"`
	Priority   int32                                         `json:"priority"`
}

func (i Indexer) String() string {
	return fmt.Sprintf("%s (%d)", i.Name, i.ID)
}

func (i *IndexerStore) ListIndexers(ctx context.Context) ([]Indexer, error) {
	dbIndexers, err := i.storage.ListIndexers(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing indexers failed: %w", err)
	}
	ret := make([]Indexer, 0, len(dbIndexers))
	for _, v := range dbIndexers {
		indexer := fromStorageIndexer(*v)
		ret = append(ret, indexer)
	}
	sortIndexers(ret)
	return ret, nil
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

	var storeErr error
	for _, indexer := range indexers {
		ret, err := FromProwlarrIndexer(indexer)
		if err != nil {
			return err
		}
		model := toStorageIndexer(*ret, i.prowlarr.GetAPIURL(), i.prowlarr.GetAPIKey())
		_, err = i.storage.CreateIndexer(ctx, model)
		if err != nil {
			log.Errorw("error creating indexer", "error", err)
			errors.Join(storeErr, err)
		}
	}
	return storeErr
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
		log.Debug("unexpected response status", zap.String("status", resp.Status), zap.String("body", string(b)))
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

func fromStorageIndexer(mi model.Indexers) Indexer {
	return Indexer{
		ID:   mi.ID,
		Name: mi.Name,
		// TODO: Maybe we should store the category?
		Categories: nullable.NewNullNullable[[]prowlarr.IndexerCategory](),
		Priority:   mi.Priority,
		Status:     nullableStatus(nil),
	}
}

func toStorageIndexer(indexer Indexer, uri, key string) model.Indexers {
	return model.Indexers{
		ID:       indexer.ID,
		Name:     indexer.Name,
		Priority: indexer.Priority,
		URI:      uri,
		ApiKey:   &key,
	}
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
