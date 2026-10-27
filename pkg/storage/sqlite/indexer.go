package sqlite

import (
	"context"
	"fmt"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

// CreateIndexer upserts sourced indexers by source and Prowlarr ID; internal indexers are inserted.
func (s *SQLite) CreateIndexer(ctx context.Context, indexer sqlcdb.Indexer) (int64, error) {
	if indexer.Categories == "" {
		indexer.Categories = `{"movie":[],"tv":[]}`
	}
	params := sqlcdb.CreateIndexerParams{
		Name:            indexer.Name,
		Priority:        indexer.Priority,
		Uri:             indexer.Uri,
		ApiKey:          indexer.ApiKey,
		IndexerSourceID: indexer.IndexerSourceID,
		Categories:      indexer.Categories,
		ProwlarrID:      indexer.ProwlarrID,
	}
	return s.Queries.CreateIndexer(ctx, params)
}

// UpdateIndexer updates an indexer. An invalid (NULL) ApiKey leaves the stored
// key untouched (COALESCE in the query).
func (s *SQLite) UpdateIndexer(ctx context.Context, id int64, indexer sqlcdb.Indexer) error {
	rows, err := s.Queries.UpdateIndexer(ctx, sqlcdb.UpdateIndexerParams{
		Name:       indexer.Name,
		Priority:   indexer.Priority,
		Uri:        indexer.Uri,
		ApiKey:     indexer.ApiKey,
		Categories: indexer.Categories,
		ID:         id,
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("update indexer %d: %w", id, storage.ErrNotFound)
	}
	return nil
}

// DeleteIndexer deletes a stored indexer given the indexer ID
func (s *SQLite) DeleteIndexer(ctx context.Context, id int64) error {
	rows, err := s.Queries.DeleteIndexer(ctx, id)
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("delete indexer %d: %w", id, storage.ErrNotFound)
	}
	return nil
}

// ListIndexers lists the stored indexers ordered by priority descending
func (s *SQLite) ListIndexers(ctx context.Context) ([]sqlcdb.Indexer, error) {
	return s.Queries.ListIndexers(ctx)
}
