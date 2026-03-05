package indexer

import (
	"context"

	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
)

const (
	TypeTV    = "tv"
	TypeMovie = "movie"
)

type IndexerSource interface {
	ListIndexers(ctx context.Context) ([]SourceIndexer, error)
	Search(ctx context.Context, indexerID int32, categories []int32, opts SearchOptions) ([]*prowlarr.ReleaseResource, error)
}

type SourceIndexer struct {
	ID         int32
	Name       string
	URI        string
	Priority   int32
	Categories []prowlarr.IndexerCategory
	Status     *prowlarr.IndexerStatusResource
}

type SearchOptions struct {
	Query   string
	Season  *int32
	Episode *int32
	Type    *string
	TmdbID  *int32
}

type Factory interface {
	NewIndexerSource(config model.IndexerSource) (IndexerSource, error)
}
