package sqlite

import (
	"context"

	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

// CreateQualityDefinition store a new quality definition
func (s *SQLite) CreateQualityDefinition(ctx context.Context, definition sqlcdb.QualityDefinition) (int64, error) {
	return s.Queries.CreateQualityDefinition(ctx, sqlcdb.CreateQualityDefinitionParams{
		QualityID:     definition.QualityID,
		Name:          definition.Name,
		PreferredSize: definition.PreferredSize,
		MinSize:       definition.MinSize,
		MaxSize:       definition.MaxSize,
		MediaType:     definition.MediaType,
	})
}

// GetQualityDefinition gets a quality definition
func (s *SQLite) GetQualityDefinition(ctx context.Context, id int64) (sqlcdb.QualityDefinition, error) {
	definition, err := s.Queries.GetQualityDefinition(ctx, id)
	if err != nil {
		return sqlcdb.QualityDefinition{}, mapQueryErr(err, "failed to get quality definition")
	}
	return definition, nil
}

// ListQualityDefinitions lists all quality definitions ordered by id
func (s *SQLite) ListQualityDefinitions(ctx context.Context) ([]sqlcdb.QualityDefinition, error) {
	return s.Queries.ListQualityDefinitions(ctx)
}

// UpdateQualityDefinition updates a quality definition. The quality_id column
// is intentionally untouched.
func (s *SQLite) UpdateQualityDefinition(ctx context.Context, id int64, definition sqlcdb.QualityDefinition) error {
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
