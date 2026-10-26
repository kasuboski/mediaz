package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

// CreateQualityDefinition store a new quality definition
func (s *SQLite) CreateQualityDefinition(ctx context.Context, definition model.QualityDefinition) (int64, error) {
	return s.Queries.CreateQualityDefinition(ctx, sqlcdb.CreateQualityDefinitionParams{
		QualityID:     int64(definition.QualityID),
		Name:          definition.Name,
		PreferredSize: definition.PreferredSize,
		MinSize:       definition.MinSize,
		MaxSize:       definition.MaxSize,
		MediaType:     definition.MediaType,
	})
}

// GetQualityDefinition gets a quality definition
func (s *SQLite) GetQualityDefinition(ctx context.Context, id int64) (model.QualityDefinition, error) {
	r, err := s.Queries.GetQualityDefinition(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.QualityDefinition{}, storage.ErrNotFound
		}
		return model.QualityDefinition{}, err
	}

	return qualityDefinitionFromRow(r), nil
}

// ListQualityDefinitions lists all quality definitions ordered by id
func (s *SQLite) ListQualityDefinitions(ctx context.Context) ([]*model.QualityDefinition, error) {
	rows, err := s.Queries.ListQualityDefinitions(ctx)
	if err != nil {
		return nil, err
	}

	definitions := make([]*model.QualityDefinition, 0, len(rows))
	for _, r := range rows {
		definition := qualityDefinitionFromRow(r)
		definitions = append(definitions, &definition)
	}

	return definitions, nil
}

// UpdateQualityDefinition updates a quality definition. The quality_id column
// is intentionally untouched.
func (s *SQLite) UpdateQualityDefinition(ctx context.Context, id int64, definition model.QualityDefinition) error {
	return s.Queries.UpdateQualityDefinition(ctx, sqlcdb.UpdateQualityDefinitionParams{
		Name:          definition.Name,
		PreferredSize: definition.PreferredSize,
		MinSize:       definition.MinSize,
		MaxSize:       definition.MaxSize,
		MediaType:     definition.MediaType,
		ID:            id,
	})
}

// DeleteQualityDefinition deletes a quality definition
func (s *SQLite) DeleteQualityDefinition(ctx context.Context, id int64) error {
	return s.Queries.DeleteQualityDefinition(ctx, id)
}

// qualityDefinitionFromRow converts a sqlc row into the domain model.
func qualityDefinitionFromRow(r sqlcdb.QualityDefinition) model.QualityDefinition {
	return model.QualityDefinition{
		ID:            int32(r.ID),
		QualityID:     int32(r.QualityID),
		Name:          r.Name,
		PreferredSize: r.PreferredSize,
		MinSize:       r.MinSize,
		MaxSize:       r.MaxSize,
		MediaType:     r.MediaType,
	}
}
