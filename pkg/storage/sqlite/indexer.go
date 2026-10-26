package sqlite

import (
	"context"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

// CreateIndexer upserts an indexer by source and name, returning its stable id.
func (s *SQLite) CreateIndexer(ctx context.Context, indexer sqlcdb.Indexer) (int64, error) {
	return s.Queries.CreateIndexer(ctx, sqlcdb.CreateIndexerParams{
		Name:            indexer.Name,
		Priority:        indexer.Priority,
		Uri:             indexer.Uri,
		ApiKey:          indexer.ApiKey,
		IndexerSourceID: indexer.IndexerSourceID,
	})
}

// UpdateIndexer updates an indexer. An invalid (NULL) ApiKey leaves the stored
// key untouched (COALESCE in the query).
func (s *SQLite) UpdateIndexer(ctx context.Context, id int64, indexer sqlcdb.Indexer) error {
	return s.Queries.UpdateIndexer(ctx, sqlcdb.UpdateIndexerParams{
		Name:     indexer.Name,
		Priority: indexer.Priority,
		Uri:      indexer.Uri,
		ApiKey:   indexer.ApiKey,
		ID:       id,
	})
}

// DeleteIndexer deletes a stored indexer given the indexer ID
func (s *SQLite) DeleteIndexer(ctx context.Context, id int64) error {
	return s.Queries.DeleteIndexer(ctx, id)
}

// ListIndexers lists the stored indexers ordered by priority descending
func (s *SQLite) ListIndexers(ctx context.Context) ([]sqlcdb.Indexer, error) {
	return s.Queries.ListIndexers(ctx)
}
