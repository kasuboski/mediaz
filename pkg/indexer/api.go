package indexer

import (
	"context"

	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
)

type IndexerSource interface {
	ListIndexers(ctx context.Context) ([]SourceIndexer, error)
	Search(ctx context.Context, indexerID int32, categories []int32, query string) ([]*prowlarr.ReleaseResource, error)
}

type SourceIndexer struct {
	ID         int32
	Name       string
	URI        string
	Priority   int32
	Categories []prowlarr.IndexerCategory
	Status     *prowlarr.IndexerStatusResource
}

type Factory interface {
	NewIndexerSource(config model.IndexerSource) (IndexerSource, error)
}
