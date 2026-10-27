package indexer

import (
	"context"

	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

const (
	TypeTV    = "tv"
	TypeMovie = "movie"
)

// RowID identifies an indexer row in Mediaz's SQLite database.
type RowID int64

// ProwlarrID identifies an indexer within one Prowlarr source, not a SQLite row.
type ProwlarrID int32

type IndexerSource interface {
	ListIndexers(ctx context.Context) ([]SourceIndexer, error)
	Search(ctx context.Context, indexerID ProwlarrID, categories []int32, opts SearchOptions) ([]*prowlarr.ReleaseResource, error)
}

type SourceIndexer struct {
	ID         ProwlarrID
	Name       string
	URI        string
	Priority   int32
	Categories []prowlarr.IndexerCategory
	Status     *prowlarr.IndexerStatusResource
}

type SearchOptions struct {
	Query   string
	Season  *int64
	Episode *int64
	Type    *string
	TmdbID  *int32
}

type Factory interface {
	NewIndexerSource(config sqlcdb.IndexerSource) (IndexerSource, error)
}
