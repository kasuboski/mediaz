package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

// CreateIndexerSource stores a new indexer source in the database
func (s *SQLite) CreateIndexerSource(ctx context.Context, source model.IndexerSource) (int64, error) {
	return s.Queries.CreateIndexerSource(ctx, sqlcdb.CreateIndexerSourceParams{
		Name:           source.Name,
		Implementation: source.Implementation,
		Scheme:         source.Scheme,
		Host:           source.Host,
		Port:           cfgNullInt64FromInt32(source.Port),
		ApiKey:         cfgNullString(source.APIKey),
		Enabled:        source.Enabled,
	})
}

// GetIndexerSource gets a stored indexer source given an id
func (s *SQLite) GetIndexerSource(ctx context.Context, id int64) (model.IndexerSource, error) {
	r, err := s.Queries.GetIndexerSource(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.IndexerSource{}, storage.ErrNotFound
		}
		return model.IndexerSource{}, err
	}

	return indexerSourceFromRow(r), nil
}

// ListIndexerSources lists all stored indexer sources ordered by name
func (s *SQLite) ListIndexerSources(ctx context.Context) ([]*model.IndexerSource, error) {
	rows, err := s.Queries.ListIndexerSources(ctx)
	if err != nil {
		return nil, err
	}

	sources := make([]*model.IndexerSource, 0, len(rows))
	for _, r := range rows {
		source := indexerSourceFromRow(r)
		sources = append(sources, &source)
	}

	return sources, nil
}

// ListEnabledIndexerSources lists all stored indexer sources that are enabled,
// ordered by name
func (s *SQLite) ListEnabledIndexerSources(ctx context.Context) ([]*model.IndexerSource, error) {
	rows, err := s.Queries.ListEnabledIndexerSources(ctx)
	if err != nil {
		return nil, err
	}

	sources := make([]*model.IndexerSource, 0, len(rows))
	for _, r := range rows {
		source := indexerSourceFromRow(r)
		sources = append(sources, &source)
	}

	return sources, nil
}

// UpdateIndexerSource updates an existing indexer source. A nil APIKey leaves
// the stored key untouched (COALESCE in the query).
func (s *SQLite) UpdateIndexerSource(ctx context.Context, id int64, source model.IndexerSource) error {
	return s.Queries.UpdateIndexerSource(ctx, sqlcdb.UpdateIndexerSourceParams{
		Name:           source.Name,
		Implementation: source.Implementation,
		Scheme:         source.Scheme,
		Host:           source.Host,
		Port:           cfgNullInt64FromInt32(source.Port),
		Enabled:        source.Enabled,
		ApiKey:         cfgNullString(source.APIKey),
		ID:             id,
	})
}

// DeleteIndexerSource deletes an indexer source given an id
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

// indexerSourceFromRow converts a sqlc row into the domain model.
func indexerSourceFromRow(r sqlcdb.IndexerSource) model.IndexerSource {
	return model.IndexerSource{
		ID:             int32(r.ID),
		Name:           r.Name,
		Implementation: r.Implementation,
		Scheme:         r.Scheme,
		Host:           r.Host,
		Port:           cfgInt32Ptr(r.Port),
		APIKey:         cfgStringPtr(r.ApiKey),
		Enabled:        r.Enabled,
		CreatedAt:      cfgTimePtr(r.CreatedAt),
		UpdatedAt:      cfgTimePtr(r.UpdatedAt),
	}
}
