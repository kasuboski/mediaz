package sqlite

import (
	"context"
	"database/sql"

	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

// CreateIndexerSource stores a new indexer source in the database
func (s *SQLite) CreateIndexerSource(ctx context.Context, source sqlcdb.IndexerSource) (int64, error) {
	return s.Queries.CreateIndexerSource(ctx, sqlcdb.CreateIndexerSourceParams{
		Name:           source.Name,
		Implementation: source.Implementation,
		Scheme:         source.Scheme,
		Host:           source.Host,
		Port:           source.Port,
		ApiKey:         source.ApiKey,
		Enabled:        source.Enabled,
	})
}

// GetIndexerSource gets a stored indexer source given an id
func (s *SQLite) GetIndexerSource(ctx context.Context, id int64) (sqlcdb.IndexerSource, error) {
	source, err := s.Queries.GetIndexerSource(ctx, id)
	if err != nil {
		return sqlcdb.IndexerSource{}, mapQueryErr(err, "failed to get indexer source")
	}
	return source, nil
}

// ListIndexerSources lists all stored indexer sources ordered by name
func (s *SQLite) ListIndexerSources(ctx context.Context) ([]sqlcdb.IndexerSource, error) {
	return s.Queries.ListIndexerSources(ctx)
}

// ListEnabledIndexerSources lists all stored indexer sources that are
// enabled, ordered by name
func (s *SQLite) ListEnabledIndexerSources(ctx context.Context) ([]sqlcdb.IndexerSource, error) {
	return s.Queries.ListEnabledIndexerSources(ctx)
}

// UpdateIndexerSource updates an existing indexer source. An invalid (NULL)
// ApiKey leaves the stored key untouched (COALESCE in the query).
func (s *SQLite) UpdateIndexerSource(ctx context.Context, id int64, source sqlcdb.IndexerSource) error {
	return s.Queries.UpdateIndexerSource(ctx, sqlcdb.UpdateIndexerSourceParams{
		Name:           source.Name,
		Implementation: source.Implementation,
		Scheme:         source.Scheme,
		Host:           source.Host,
		Port:           source.Port,
		Enabled:        source.Enabled,
		ApiKey:         source.ApiKey,
		ID:             id,
	})
}

// DeleteIndexerSource deletes an indexer source given an id. Child indexers
// cascade-delete via the schema FK.
func (s *SQLite) DeleteIndexerSource(ctx context.Context, id int64) error {
	return s.Queries.DeleteIndexerSource(ctx, id)
}

// DeleteIndexerSourceCascade deletes an indexer source and all associated
// child indexers atomically in a single database transaction.
func (s *SQLite) DeleteIndexerSourceCascade(ctx context.Context, id int64) error {
	log := logger.FromCtx(ctx)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		log.Errorw("failed to begin transaction", "id", id, "error", err)
		return err
	}

	q := s.txQueries(tx)

	// Delete all child indexers belonging to this source.
	if err := q.DeleteIndexersBySourceID(ctx, sql.NullInt64{Int64: id, Valid: true}); err != nil {
		log.Errorw("failed to delete child indexers", "id", id, "error", err)
		if rbErr := tx.Rollback(); rbErr != nil {
			log.Errorw("failed to rollback", "id", id, "error", rbErr)
		}
		return err
	}

	// Delete the source row itself.
	if err := q.DeleteIndexerSource(ctx, id); err != nil {
		log.Errorw("failed to delete indexer source", "id", id, "error", err)
		if rbErr := tx.Rollback(); rbErr != nil {
			log.Errorw("failed to rollback", "id", id, "error", rbErr)
		}
		return err
	}

	log.Debugw("deleted indexer source and child indexers", "id", id)
	return tx.Commit()
}
