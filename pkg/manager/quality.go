package manager

import (
	"context"
	"fmt"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
)

// MeetsQualitySize checks if the given fileSize (MB) and runtime (min) fall within the QualitySize
func MeetsQualitySize(qs storage.QualityDefinition, fileSize uint64, runtime uint64) bool {
	fileRatio := float64(fileSize) / float64(runtime)

	if fileRatio < qs.MinSize {
		return false
	}

	if fileRatio > qs.MaxSize {
		return false
	}

	return true
}

type AddQualityDefinitionRequest struct {
	model.QualityDefinition
}

// AddQualityDefinition stores a new quality definition in the database
func (m MediaManager) AddQualityDefinition(ctx context.Context, request AddQualityDefinitionRequest) (model.QualityDefinition, error) {
	definition := request.QualityDefinition

	id, err := m.storage.CreateQualityDefinition(ctx, request.QualityDefinition)
	if err != nil {
		return definition, err
	}

	definition.ID = int32(id)
	return definition, nil
}

// DeleteQualityDefinitionRequest request to delete a quality definition
type DeleteQualityDefinitionRequest struct {
	ID *int `json:"id" yaml:"id"`
}

// AddQualityDefinition deletes a quality definition
func (m MediaManager) DeleteQualityDefinition(ctx context.Context, request DeleteQualityDefinitionRequest) error {
	if request.ID == nil {
		return fmt.Errorf("indexer id is required")
	}

	return m.storage.DeleteQualityDefinition(ctx, int64(*request.ID))
}

// ListQualityDefinitions list stored quality definitions
func (m MediaManager) ListQualityDefinitions(ctx context.Context) ([]*model.QualityDefinition, error) {
	return m.storage.ListQualityDefinitions(ctx)
}

// ListQualityDefinitions get a stored quality definitions
func (m MediaManager) GetQualityDefinition(ctx context.Context, id int64) (model.QualityDefinition, error) {
	return m.storage.GetQualityDefinition(ctx, id)
}

func (m MediaManager) GetQualityProfile(ctx context.Context, id int64) (storage.QualityProfile, error) {
	return m.storage.GetQualityProfile(ctx, id)
}

func (m MediaManager) ListEpisodeQualityProfiles(ctx context.Context, id int64) ([]*storage.QualityProfile, error) {
	where := table.QualityDefinition.MediaType.EQ(sqlite.String("episode"))
	return m.storage.ListQualityProfiles(ctx, where)
}

func (m MediaManager) ListMovieQualityProfiles(ctx context.Context, id int64) ([]*storage.QualityProfile, error) {
	where := table.QualityDefinition.MediaType.EQ(sqlite.String("movie"))
	return m.storage.ListQualityProfiles(ctx, where)
}

func (m MediaManager) ListQualityProfiles(ctx context.Context) ([]*storage.QualityProfile, error) {
	return m.storage.ListQualityProfiles(ctx)
}
