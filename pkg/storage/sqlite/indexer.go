package sqlite

import (
	"context"
	"fmt"

	"github.com/kasuboski/mediaz/pkg/storage/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

// CreateIndexer stores a new indexer in the database. If an indexer with the
// same (source, name) pair already exists the insert does nothing.
func (s *SQLite) CreateIndexer(ctx context.Context, indexer model.Indexer) (int64, error) {
	result, err := s.Queries.CreateIndexer(ctx, sqlcdb.CreateIndexerParams{
		Name:            indexer.Name,
		Priority:        int64(indexer.Priority),
		Uri:             indexer.URI,
		ApiKey:          cfgNullString(indexer.APIKey),
		IndexerSourceID: cfgNullInt64FromInt32(indexer.IndexerSourceID),
	})
	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

// UpdateIndexer updates an indexer. A nil APIKey leaves the stored key
// untouched (COALESCE in the query).
func (s *SQLite) UpdateIndexer(ctx context.Context, id int64, indexer model.Indexer) error {
	return s.Queries.UpdateIndexer(ctx, sqlcdb.UpdateIndexerParams{
		Name:     indexer.Name,
		Priority: int64(indexer.Priority),
		Uri:      indexer.URI,
		ApiKey:   cfgNullString(indexer.APIKey),
		ID:       id,
	})
}

// DeleteIndexer deletes a stored indexer given the indexer ID
func (s *SQLite) DeleteIndexer(ctx context.Context, id int64) error {
	return s.Queries.DeleteIndexer(ctx, id)
}

// ListIndexers lists the stored indexers ordered by priority descending
func (s *SQLite) ListIndexers(ctx context.Context) ([]*model.Indexer, error) {
	rows, err := s.Queries.ListIndexers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list indexers: %w", err)
	}

	indexers := make([]*model.Indexer, 0, len(rows))
	for _, r := range rows {
		indexers = append(indexers, &model.Indexer{
			ID:              int32(r.ID),
			Name:            r.Name,
			Priority:        int32(r.Priority),
			URI:             r.Uri,
			APIKey:          cfgStringPtr(r.ApiKey),
			IndexerSourceID: cfgInt32Ptr(r.IndexerSourceID),
		})
	}

	return indexers, nil
}
