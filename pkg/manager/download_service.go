package manager

import (
	"context"

	"github.com/kasuboski/mediaz/pkg/download"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
)

type DownloadClientService struct {
	downloadStorage storage.DownloadClientStorage
	factory         download.Factory
}

func NewDownloadClientService(downloadStorage storage.DownloadClientStorage, factory download.Factory) *DownloadClientService {
	return &DownloadClientService{
		downloadStorage: downloadStorage,
		factory:         factory,
	}
}

func (ds DownloadClientService) newDownloadClient(cfg model.DownloadClient) (download.DownloadClient, error) {
	return ds.factory.NewDownloadClient(cfg)
}

type AddDownloadClientRequest struct {
	model.DownloadClient
}

type UpdateDownloadClientRequest struct {
	model.DownloadClient
}

func (ds DownloadClientService) CreateDownloadClient(ctx context.Context, request AddDownloadClientRequest) (model.DownloadClient, error) {
	downloadClient := request.DownloadClient

	id, err := ds.downloadStorage.CreateDownloadClient(ctx, request.DownloadClient)
	if err != nil {
		return downloadClient, err
	}

	downloadClient.ID = int32(id)
	return downloadClient, nil
}

func (ds DownloadClientService) UpdateDownloadClient(ctx context.Context, id int64, request UpdateDownloadClientRequest) (model.DownloadClient, error) {
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

func (ds DownloadClientService) TestDownloadClient(ctx context.Context, request AddDownloadClientRequest) error {
	client, err := ds.factory.NewDownloadClient(request.DownloadClient)
	if err != nil {
		return err
	}

	_, err = client.List(ctx)
	return err
}

func (ds DownloadClientService) GetDownloadClient(ctx context.Context, id int64) (download.DownloadClient, error) {
	client, err := ds.downloadStorage.GetDownloadClient(ctx, id)
	if err != nil {
		return nil, err
	}

	return ds.factory.NewDownloadClient(client)
}

func (ds DownloadClientService) ListDownloadClients(ctx context.Context) ([]*model.DownloadClient, error) {
	return ds.downloadStorage.ListDownloadClients(ctx)
}

func (ds DownloadClientService) DeleteDownloadClient(ctx context.Context, id int64) error {
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
