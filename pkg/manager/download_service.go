package manager

import (
	"context"

	"github.com/kasuboski/mediaz/pkg/download"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/storage"
	sqlcdb "github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
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

func (ds DownloadClientService) buildRuntimeDownloadClient(ctx context.Context, client sqlcdb.DownloadClient) (download.DownloadClient, error) {
	return ds.factory.NewDownloadClient(client)
}

type AddDownloadClientRequest struct {
	sqlcdb.DownloadClient
}

type UpdateDownloadClientRequest struct {
	sqlcdb.DownloadClient
}

func (ds DownloadClientService) CreateDownloadClient(ctx context.Context, request AddDownloadClientRequest) (sqlcdb.DownloadClient, error) {
	downloadClient := request.DownloadClient

	id, err := ds.downloadStorage.CreateDownloadClient(ctx, request.DownloadClient)
	if err != nil {
		return downloadClient, err
	}

	downloadClient.ID = id
	return downloadClient, nil
}

func (ds DownloadClientService) UpdateDownloadClient(ctx context.Context, id int64, request UpdateDownloadClientRequest) (sqlcdb.DownloadClient, error) {
	if !request.ApiKey.Valid || request.ApiKey.String == "" {
		existing, err := ds.downloadStorage.GetDownloadClient(ctx, id)
		if err != nil {
			return sqlcdb.DownloadClient{}, err
		}
		request.ApiKey = existing.ApiKey
	}

	downloadClient := request.DownloadClient
	downloadClient.ID = id

	err := ds.downloadStorage.UpdateDownloadClient(ctx, id, downloadClient)
	if err != nil {
		return sqlcdb.DownloadClient{}, err
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

func (ds DownloadClientService) GetDownloadClient(ctx context.Context, id int64) (sqlcdb.DownloadClient, error) {
	return ds.downloadStorage.GetDownloadClient(ctx, id)
}

func (ds DownloadClientService) ListDownloadClients(ctx context.Context) ([]sqlcdb.DownloadClient, error) {
	return ds.downloadStorage.ListDownloadClients(ctx)
}

func (ds DownloadClientService) DeleteDownloadClient(ctx context.Context, id int64) error {
	return ds.downloadStorage.DeleteDownloadClient(ctx, id)
}

func availableProtocols(clients []sqlcdb.DownloadClient) map[string]struct{} {
	ret := make(map[string]struct{})
	for _, c := range clients {
		ret[c.Type] = struct{}{}
	}

	return ret
}

func clientForProtocol(clients []sqlcdb.DownloadClient, proto prowlarr.DownloadProtocol) *sqlcdb.DownloadClient {
	for i := range clients {
		if clients[i].Type == string(proto) {
			return &clients[i]
		}
	}

	return nil
}
