package manager

import (
	"context"
	"fmt"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/pkg/download"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
)

// DownloadService owns download client interactions, quality profile evaluation, and release selection.
// It depends only on storage.DownloadClientStorage and storage.QualityStorage.
type DownloadService struct {
	downloadStorage storage.DownloadClientStorage
	qualityStorage  storage.QualityStorage
	factory         download.Factory
}

func NewDownloadService(downloadStorage storage.DownloadClientStorage, qualityStorage storage.QualityStorage, factory download.Factory) *DownloadService {
	return &DownloadService{
		downloadStorage: downloadStorage,
		qualityStorage:  qualityStorage,
		factory:         factory,
	}
}

func (ds DownloadService) newDownloadClient(cfg model.DownloadClient) (download.DownloadClient, error) {
	return ds.factory.NewDownloadClient(cfg)
}

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

func (ds DownloadService) AddQualityDefinition(ctx context.Context, request AddQualityDefinitionRequest) (model.QualityDefinition, error) {
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

	id, err := ds.qualityStorage.CreateQualityDefinition(ctx, definition)
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
	CutoffQualityID *int32  `json:"cutoffQualityId,omitempty"`
	UpgradeAllowed  bool    `json:"upgradeAllowed"`
	QualityIDs      []int32 `json:"qualityIds"`
}

type UpdateQualityProfileRequest struct {
	Name            string  `json:"name"`
	CutoffQualityID *int32  `json:"cutoffQualityId,omitempty"`
	UpgradeAllowed  bool    `json:"upgradeAllowed"`
	QualityIDs      []int32 `json:"qualityIds"`
}

type DeleteQualityProfileRequest struct {
	ID *int `json:"id"`
}

func (ds DownloadService) DeleteQualityDefinition(ctx context.Context, request DeleteQualityDefinitionRequest) error {
	if request.ID == nil {
		return fmt.Errorf("indexer id is required")
	}

	return ds.qualityStorage.DeleteQualityDefinition(ctx, int64(*request.ID))
}

func (ds DownloadService) ListQualityDefinitions(ctx context.Context) ([]*model.QualityDefinition, error) {
	return ds.qualityStorage.ListQualityDefinitions(ctx)
}

func (ds DownloadService) GetQualityDefinition(ctx context.Context, id int64) (model.QualityDefinition, error) {
	return ds.qualityStorage.GetQualityDefinition(ctx, id)
}

func (ds DownloadService) GetQualityProfile(ctx context.Context, id int64) (storage.QualityProfile, error) {
	return ds.qualityStorage.GetQualityProfile(ctx, id)
}

func (ds DownloadService) ListEpisodeQualityProfiles(ctx context.Context) ([]*storage.QualityProfile, error) {
	where := table.QualityDefinition.MediaType.EQ(sqlite.String("episode"))
	return ds.qualityStorage.ListQualityProfiles(ctx, where)
}

func (ds DownloadService) ListMovieQualityProfiles(ctx context.Context) ([]*storage.QualityProfile, error) {
	where := table.QualityDefinition.MediaType.EQ(sqlite.String("movie"))
	return ds.qualityStorage.ListQualityProfiles(ctx, where)
}

func (ds DownloadService) ListQualityProfiles(ctx context.Context) ([]*storage.QualityProfile, error) {
	return ds.qualityStorage.ListQualityProfiles(ctx)
}

func (ds DownloadService) UpdateQualityDefinition(ctx context.Context, id int64, request UpdateQualityDefinitionRequest) (model.QualityDefinition, error) {
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

	err := ds.qualityStorage.UpdateQualityDefinition(ctx, id, definition)
	if err != nil {
		return model.QualityDefinition{}, err
	}
	return ds.qualityStorage.GetQualityDefinition(ctx, id)
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

func (ds DownloadService) AddQualityProfile(ctx context.Context, request AddQualityProfileRequest) (storage.QualityProfile, error) {
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

	id, err := ds.qualityStorage.CreateQualityProfile(ctx, profile)
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
	err = ds.qualityStorage.CreateQualityProfileItems(ctx, items)
	if err != nil {
		return storage.QualityProfile{}, err
	}

	return ds.qualityStorage.GetQualityProfile(ctx, id)
}

func (ds DownloadService) UpdateQualityProfile(ctx context.Context, id int64, request UpdateQualityProfileRequest) (storage.QualityProfile, error) {
	if request.Name == "" {
		return storage.QualityProfile{}, fmt.Errorf("name is required")
	}
	if len(request.QualityIDs) == 0 {
		return storage.QualityProfile{}, fmt.Errorf("at least one quality must be selected")
	}

	if err := validateQualityProfileCutoff(request.CutoffQualityID, request.UpgradeAllowed, request.QualityIDs); err != nil {
		return storage.QualityProfile{}, err
	}

	existingProfile, err := ds.qualityStorage.GetQualityProfile(ctx, id)
	if err != nil {
		return storage.QualityProfile{}, err
	}

	profile := model.QualityProfile{
		ID:              existingProfile.ID,
		Name:            request.Name,
		CutoffQualityID: request.CutoffQualityID,
		UpgradeAllowed:  request.UpgradeAllowed,
	}

	err = ds.qualityStorage.UpdateQualityProfile(ctx, id, profile)
	if err != nil {
		return storage.QualityProfile{}, err
	}

	err = ds.qualityStorage.DeleteQualityProfileItemsByProfileID(ctx, id)
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
	err = ds.qualityStorage.CreateQualityProfileItems(ctx, items)
	if err != nil {
		return storage.QualityProfile{}, err
	}

	return ds.qualityStorage.GetQualityProfile(ctx, id)
}

func (ds DownloadService) DeleteQualityProfile(ctx context.Context, request DeleteQualityProfileRequest) error {
	if request.ID == nil {
		return fmt.Errorf("profile id is required")
	}
	return ds.qualityStorage.DeleteQualityProfile(ctx, int64(*request.ID))
}

// AddDownloadClientRequest describes what is required to add a download client
type AddDownloadClientRequest struct {
	model.DownloadClient
}

// UpdateDownloadClientRequest describes what is required to update a download client
type UpdateDownloadClientRequest struct {
	model.DownloadClient
}

func (ds DownloadService) CreateDownloadClient(ctx context.Context, request AddDownloadClientRequest) (model.DownloadClient, error) {
	downloadClient := request.DownloadClient

	id, err := ds.downloadStorage.CreateDownloadClient(ctx, request.DownloadClient)
	if err != nil {
		return downloadClient, err
	}

	downloadClient.ID = int32(id)
	return downloadClient, nil
}

func (ds DownloadService) UpdateDownloadClient(ctx context.Context, id int64, request UpdateDownloadClientRequest) (model.DownloadClient, error) {
	if request.APIKey == nil || (request.APIKey != nil && *request.APIKey == "") {
		existing, err := ds.downloadStorage.GetDownloadClient(ctx, id)
		if err != nil {
			return model.DownloadClient{}, err
		}
		request.APIKey = existing.APIKey
	}

	downloadClient := request.DownloadClient
	downloadClient.ID = int32(id)

	err := ds.downloadStorage.UpdateDownloadClient(ctx, id, downloadClient)
	if err != nil {
		return model.DownloadClient{}, err
	}

	return downloadClient, nil
}

func (ds DownloadService) TestDownloadClient(ctx context.Context, request AddDownloadClientRequest) error {
	client, err := ds.factory.NewDownloadClient(request.DownloadClient)
	if err != nil {
		return err
	}

	_, err = client.List(ctx)
	return err
}

func (ds DownloadService) GetDownloadClient(ctx context.Context, id int64) (download.DownloadClient, error) {
	client, err := ds.downloadStorage.GetDownloadClient(ctx, id)
	if err != nil {
		return nil, err
	}

	return ds.factory.NewDownloadClient(client)
}

func (ds DownloadService) ListDownloadClients(ctx context.Context) ([]*model.DownloadClient, error) {
	return ds.downloadStorage.ListDownloadClients(ctx)
}

func (ds DownloadService) DeleteDownloadClient(ctx context.Context, id int64) error {
	return ds.downloadStorage.DeleteDownloadClient(ctx, id)
}

func availableProtocols(clients []*model.DownloadClient) map[string]struct{} {
	ret := make(map[string]struct{})
	for _, c := range clients {
		ret[c.Type] = struct{}{}
	}

	return ret
}

func clientForProtocol(clients []*model.DownloadClient, proto prowlarr.DownloadProtocol) *model.DownloadClient {
	for _, c := range clients {
		if c.Type == string(proto) {
			return c
		}
	}

	return nil
}
