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
	Name          string  `json:"name"`
	Type          string  `json:"type"`
	PreferredSize float64 `json:"preferredSize"`
	MinSize       float64 `json:"minSize"`
	MaxSize       float64 `json:"maxSize"`
}

// AddQualityDefinition stores a new quality definition in the database
func (m MediaManager) AddQualityDefinition(ctx context.Context, request AddQualityDefinitionRequest) (model.QualityDefinition, error) {
	if request.Name == "" {
		return model.QualityDefinition{}, fmt.Errorf("name is required")
	}
	if request.Type == "" {
		return model.QualityDefinition{}, fmt.Errorf("type is required")
	}
	if request.MinSize >= request.MaxSize {
		return model.QualityDefinition{}, fmt.Errorf("min size must be less than max size")
	}

	definition := model.QualityDefinition{
		Name:          request.Name,
		MediaType:     request.Type,
		PreferredSize: request.PreferredSize,
		MinSize:       request.MinSize,
		MaxSize:       request.MaxSize,
	}

	id, err := m.storage.CreateQualityDefinition(ctx, definition)
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

type UpdateQualityDefinitionRequest struct {
	Name          string  `json:"name"`
	Type          string  `json:"type"`
	PreferredSize float64 `json:"preferredSize"`
	MinSize       float64 `json:"minSize"`
	MaxSize       float64 `json:"maxSize"`
}

type AddQualityProfileRequest struct {
	Name            string  `json:"name"`
	CutoffQualityID int32   `json:"cutoffQualityId"`
	UpgradeAllowed  bool    `json:"upgradeAllowed"`
	QualityIDs      []int32 `json:"qualityIds"`
}

type UpdateQualityProfileRequest struct {
	Name            string  `json:"name"`
	CutoffQualityID int32   `json:"cutoffQualityId"`
	UpgradeAllowed  bool    `json:"upgradeAllowed"`
	QualityIDs      []int32 `json:"qualityIds"`
}

type DeleteQualityProfileRequest struct {
	ID *int `json:"id"`
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

func (m MediaManager) ListEpisodeQualityProfiles(ctx context.Context) ([]*storage.QualityProfile, error) {
	where := table.QualityDefinition.MediaType.EQ(sqlite.String("episode"))
	return m.storage.ListQualityProfiles(ctx, where)
}

func (m MediaManager) ListMovieQualityProfiles(ctx context.Context) ([]*storage.QualityProfile, error) {
	where := table.QualityDefinition.MediaType.EQ(sqlite.String("movie"))
	return m.storage.ListQualityProfiles(ctx, where)
}

func (m MediaManager) ListQualityProfiles(ctx context.Context) ([]*storage.QualityProfile, error) {
	return m.storage.ListQualityProfiles(ctx)
}

func (m MediaManager) UpdateQualityDefinition(ctx context.Context, id int64, request UpdateQualityDefinitionRequest) (model.QualityDefinition, error) {
	if request.Name == "" {
		return model.QualityDefinition{}, fmt.Errorf("name is required")
	}
	if request.Type == "" {
		return model.QualityDefinition{}, fmt.Errorf("type is required")
	}
	if request.MinSize >= request.MaxSize {
		return model.QualityDefinition{}, fmt.Errorf("min size must be less than max size")
	}

	definition := model.QualityDefinition{
		ID:            int32(id),
		Name:          request.Name,
		MediaType:     request.Type,
		PreferredSize: request.PreferredSize,
		MinSize:       request.MinSize,
		MaxSize:       request.MaxSize,
	}

	err := m.storage.UpdateQualityDefinition(ctx, id, definition)
	if err != nil {
		return model.QualityDefinition{}, err
	}
	return m.storage.GetQualityDefinition(ctx, id)
}

func (m MediaManager) AddQualityProfile(ctx context.Context, request AddQualityProfileRequest) (storage.QualityProfile, error) {
	if request.Name == "" {
		return storage.QualityProfile{}, fmt.Errorf("name is required")
	}
	if len(request.QualityIDs) == 0 {
		return storage.QualityProfile{}, fmt.Errorf("at least one quality must be selected")
	}

	profile := model.QualityProfile{
		Name:            request.Name,
		CutoffQualityID: request.CutoffQualityID,
		UpgradeAllowed:  request.UpgradeAllowed,
	}

	id, err := m.storage.CreateQualityProfile(ctx, profile)
	if err != nil {
		return storage.QualityProfile{}, err
	}

	for _, qualityID := range request.QualityIDs {
		item := model.QualityProfileItem{
			ProfileID: int32(id),
			QualityID: qualityID,
		}
		_, err := m.storage.CreateQualityProfileItem(ctx, item)
		if err != nil {
			return storage.QualityProfile{}, err
		}
	}

	return m.storage.GetQualityProfile(ctx, id)
}

func (m MediaManager) UpdateQualityProfile(ctx context.Context, id int64, request UpdateQualityProfileRequest) (storage.QualityProfile, error) {
	if request.Name == "" {
		return storage.QualityProfile{}, fmt.Errorf("name is required")
	}

	existingProfile, err := m.storage.GetQualityProfile(ctx, id)
	if err != nil {
		return storage.QualityProfile{}, err
	}

	profile := model.QualityProfile{
		ID:              existingProfile.ID,
		Name:            request.Name,
		CutoffQualityID: request.CutoffQualityID,
		UpgradeAllowed:  request.UpgradeAllowed,
	}

	err = m.storage.UpdateQualityProfile(ctx, id, profile)
	if err != nil {
		return storage.QualityProfile{}, err
	}

	return m.storage.GetQualityProfile(ctx, id)
}

func (m MediaManager) DeleteQualityProfile(ctx context.Context, request DeleteQualityProfileRequest) error {
	if request.ID == nil {
		return fmt.Errorf("profile id is required")
	}
	return m.storage.DeleteQualityProfile(ctx, int64(*request.ID))
}
