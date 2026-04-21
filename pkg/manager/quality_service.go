package manager

import (
	"context"
	"fmt"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
)

type QualityService struct {
	qualityStorage storage.QualityStorage
}

func NewQualityService(qualityStorage storage.QualityStorage) *QualityService {
	return &QualityService{
		qualityStorage: qualityStorage,
	}
}

type AddQualityDefinitionRequest struct {
	Name          string  `json:"name" validate:"required"`
	Type          string  `json:"type" validate:"required,oneof=movie episode"`
	PreferredSize float64 `json:"preferredSize"`
	MinSize       float64 `json:"minSize"`
	MaxSize       float64 `json:"maxSize"`
}

type UpdateQualityDefinitionRequest struct {
	Name          string  `json:"name" validate:"required"`
	Type          string  `json:"type" validate:"required,oneof=movie episode"`
	PreferredSize float64 `json:"preferredSize"`
	MinSize       float64 `json:"minSize"`
	MaxSize       float64 `json:"maxSize"`
}

type DeleteQualityDefinitionRequest struct {
	ID *int `json:"id" yaml:"id" validate:"required,gt=0"`
}

func (qs QualityService) AddQualityDefinition(ctx context.Context, request AddQualityDefinitionRequest) (model.QualityDefinition, error) {
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

	id, err := qs.qualityStorage.CreateQualityDefinition(ctx, definition)
	if err != nil {
		return definition, err
	}

	definition.ID = int32(id)
	return definition, nil
}

func (qs QualityService) DeleteQualityDefinition(ctx context.Context, request DeleteQualityDefinitionRequest) error {
	if request.ID == nil {
		return fmt.Errorf("quality definition id is required")
	}

	return qs.qualityStorage.DeleteQualityDefinition(ctx, int64(*request.ID))
}

func (qs QualityService) ListQualityDefinitions(ctx context.Context) ([]*model.QualityDefinition, error) {
	return qs.qualityStorage.ListQualityDefinitions(ctx)
}

func (qs QualityService) GetQualityDefinition(ctx context.Context, id int64) (model.QualityDefinition, error) {
	return qs.qualityStorage.GetQualityDefinition(ctx, id)
}

func (qs QualityService) UpdateQualityDefinition(ctx context.Context, id int64, request UpdateQualityDefinitionRequest) (model.QualityDefinition, error) {
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

	err := qs.qualityStorage.UpdateQualityDefinition(ctx, id, definition)
	if err != nil {
		return model.QualityDefinition{}, err
	}
	return qs.qualityStorage.GetQualityDefinition(ctx, id)
}

type AddQualityProfileRequest struct {
	Name            string  `json:"name" validate:"required"`
	CutoffQualityID *int32  `json:"cutoffQualityId,omitempty"`
	UpgradeAllowed  bool    `json:"upgradeAllowed"`
	QualityIDs      []int32 `json:"qualityIds" validate:"required,min=1,dive,gt=0"`
}

type UpdateQualityProfileRequest struct {
	Name            string  `json:"name" validate:"required"`
	CutoffQualityID *int32  `json:"cutoffQualityId,omitempty"`
	UpgradeAllowed  bool    `json:"upgradeAllowed"`
	QualityIDs      []int32 `json:"qualityIds" validate:"required,min=1,dive,gt=0"`
}

type DeleteQualityProfileRequest struct {
	ID *int `json:"id" validate:"required,gt=0"`
}

func (qs QualityService) GetQualityProfile(ctx context.Context, id int64) (storage.QualityProfile, error) {
	return qs.qualityStorage.GetQualityProfile(ctx, id)
}

func (qs QualityService) ListEpisodeQualityProfiles(ctx context.Context) ([]*storage.QualityProfile, error) {
	where := table.QualityDefinition.MediaType.EQ(sqlite.String("episode"))
	return qs.qualityStorage.ListQualityProfiles(ctx, where)
}

func (qs QualityService) ListMovieQualityProfiles(ctx context.Context) ([]*storage.QualityProfile, error) {
	where := table.QualityDefinition.MediaType.EQ(sqlite.String("movie"))
	return qs.qualityStorage.ListQualityProfiles(ctx, where)
}

func (qs QualityService) ListQualityProfiles(ctx context.Context) ([]*storage.QualityProfile, error) {
	return qs.qualityStorage.ListQualityProfiles(ctx)
}

func validateQualityProfileCutoff(cutoffQualityID *int32, upgradeAllowed bool, qualityIDs []int32) error {
	if upgradeAllowed && cutoffQualityID == nil {
		return fmt.Errorf("cutoff quality must be specified when upgrades are allowed")
	}

	if cutoffQualityID == nil {
		return nil
	}

	for _, qid := range qualityIDs {
		if qid == *cutoffQualityID {
			return nil
		}
	}

	return fmt.Errorf("cutoff quality must be one of the selected qualities")
}

func (qs QualityService) AddQualityProfile(ctx context.Context, request AddQualityProfileRequest) (storage.QualityProfile, error) {
	if request.Name == "" {
		return storage.QualityProfile{}, fmt.Errorf("name is required")
	}
	if len(request.QualityIDs) == 0 {
		return storage.QualityProfile{}, fmt.Errorf("at least one quality must be selected")
	}

	if err := validateQualityProfileCutoff(request.CutoffQualityID, request.UpgradeAllowed, request.QualityIDs); err != nil {
		return storage.QualityProfile{}, err
	}

	profile := model.QualityProfile{
		Name:            request.Name,
		CutoffQualityID: request.CutoffQualityID,
		UpgradeAllowed:  request.UpgradeAllowed,
	}

	id, err := qs.qualityStorage.CreateQualityProfile(ctx, profile)
	if err != nil {
		return storage.QualityProfile{}, err
	}

	items := make([]model.QualityProfileItem, len(request.QualityIDs))
	for i, qualityID := range request.QualityIDs {
		items[i] = model.QualityProfileItem{
			ProfileID: int32(id),
			QualityID: qualityID,
		}
	}
	err = qs.qualityStorage.CreateQualityProfileItems(ctx, items)
	if err != nil {
		return storage.QualityProfile{}, err
	}

	return qs.qualityStorage.GetQualityProfile(ctx, id)
}

func (qs QualityService) UpdateQualityProfile(ctx context.Context, id int64, request UpdateQualityProfileRequest) (storage.QualityProfile, error) {
	if request.Name == "" {
		return storage.QualityProfile{}, fmt.Errorf("name is required")
	}
	if len(request.QualityIDs) == 0 {
		return storage.QualityProfile{}, fmt.Errorf("at least one quality must be selected")
	}

	if err := validateQualityProfileCutoff(request.CutoffQualityID, request.UpgradeAllowed, request.QualityIDs); err != nil {
		return storage.QualityProfile{}, err
	}

	existingProfile, err := qs.qualityStorage.GetQualityProfile(ctx, id)
	if err != nil {
		return storage.QualityProfile{}, err
	}

	profile := model.QualityProfile{
		ID:              existingProfile.ID,
		Name:            request.Name,
		CutoffQualityID: request.CutoffQualityID,
		UpgradeAllowed:  request.UpgradeAllowed,
	}

	err = qs.qualityStorage.UpdateQualityProfile(ctx, id, profile)
	if err != nil {
		return storage.QualityProfile{}, err
	}

	err = qs.qualityStorage.DeleteQualityProfileItemsByProfileID(ctx, id)
	if err != nil {
		return storage.QualityProfile{}, err
	}

	items := make([]model.QualityProfileItem, len(request.QualityIDs))
	for i, qualityID := range request.QualityIDs {
		items[i] = model.QualityProfileItem{
			ProfileID: int32(id),
			QualityID: qualityID,
		}
	}
	err = qs.qualityStorage.CreateQualityProfileItems(ctx, items)
	if err != nil {
		return storage.QualityProfile{}, err
	}

	return qs.qualityStorage.GetQualityProfile(ctx, id)
}

func (qs QualityService) DeleteQualityProfile(ctx context.Context, request DeleteQualityProfileRequest) error {
	if request.ID == nil {
		return fmt.Errorf("profile id is required")
	}
	return qs.qualityStorage.DeleteQualityProfile(ctx, int64(*request.ID))
}

func MeetsQualitySize(qs storage.QualityDefinition, fileSize uint64, runtime uint64) bool {
	if runtime == 0 {
		return false
	}
	if qs.MinSize < 0 {
		return false
	}
	if qs.MaxSize < 0 {
		return false
	}
	if qs.MinSize > qs.MaxSize {
		return false
	}

	fileRatio := float64(fileSize) / float64(runtime)

	if fileRatio < qs.MinSize {
		return false
	}

	if fileRatio > qs.MaxSize {
		return false
	}

	return true
}
