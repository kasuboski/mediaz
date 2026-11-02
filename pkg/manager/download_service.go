package manager

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/kasuboski/mediaz/pkg/download"
	"github.com/kasuboski/mediaz/pkg/indexer"
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
	if downloadClient.Implementation == "sabnzbd" {
		if !downloadClient.ApiKey.Valid || downloadClient.ApiKey.String == "" {
			return downloadClient, fmt.Errorf("%w: missing api key", ErrValidation)
		}
		if !downloadClient.ApiPath.Valid || downloadClient.ApiPath.String == "" {
			downloadClient.ApiPath = sql.NullString{String: "/api", Valid: true}
		}
		if err := download.ValidateSABConfig(downloadClient.Scheme, downloadClient.Host, downloadClient.Port, downloadClient.ApiPath.String); err != nil {
			return downloadClient, fmt.Errorf("%w: %v", ErrValidation, err)
		}
	}

	id, err := ds.downloadStorage.CreateDownloadClient(ctx, downloadClient)
	if err != nil {
		return downloadClient, err
	}

	downloadClient.ID = id
	return downloadClient, nil
}

func (ds DownloadClientService) UpdateDownloadClient(ctx context.Context, id int64, request UpdateDownloadClientRequest) (sqlcdb.DownloadClient, error) {
	needsKey := !request.ApiKey.Valid || request.ApiKey.String == ""
	needsPath := request.Implementation == "sabnzbd" && !request.ApiPath.Valid
	if needsKey || needsPath {
		existing, err := ds.downloadStorage.GetDownloadClient(ctx, id)
		if err != nil {
			return sqlcdb.DownloadClient{}, err
		}
		if needsKey {
			request.ApiKey = existing.ApiKey
		}
		if needsPath {
			request.ApiPath = existing.ApiPath
		}
	}
	if request.Implementation == "sabnzbd" {
		if request.ApiPath.Valid && request.ApiPath.String == "" {
			request.ApiPath = sql.NullString{String: "/api", Valid: true}
		}
		if !request.ApiKey.Valid || request.ApiKey.String == "" {
			return sqlcdb.DownloadClient{}, fmt.Errorf("%w: missing api key", ErrValidation)
		}
		if err := download.ValidateSABConfig(request.Scheme, request.Host, request.Port, request.ApiPath.String); err != nil {
			return sqlcdb.DownloadClient{}, fmt.Errorf("%w: %v", ErrValidation, err)
		}
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
	config := request.DownloadClient
	if config.Implementation == "sabnzbd" {
		loadedExisting := false
		if config.ID != 0 && (!config.ApiKey.Valid || config.ApiKey.String == "" || !config.ApiPath.Valid) {
			existing, err := ds.downloadStorage.GetDownloadClient(ctx, config.ID)
			if err != nil {
				return err
			}
			loadedExisting = true
			if !config.ApiKey.Valid || config.ApiKey.String == "" {
				config.ApiKey = existing.ApiKey
			}
			if !config.ApiPath.Valid {
				config.ApiPath = existing.ApiPath
			}
		}
		if !config.ApiKey.Valid || config.ApiKey.String == "" {
			return fmt.Errorf("%w: missing api key", ErrValidation)
		}
		if config.ApiPath.Valid && config.ApiPath.String == "" {
			config.ApiPath = sql.NullString{String: "/api", Valid: true}
		}
		if !loadedExisting && !config.ApiPath.Valid {
			config.ApiPath = sql.NullString{String: "/api", Valid: true}
		}
		if err := download.ValidateSABConfig(config.Scheme, config.Host, config.Port, config.ApiPath.String); err != nil {
			return fmt.Errorf("%w: %v", ErrValidation, err)
		}
	}
	client, err := ds.factory.NewDownloadClient(config)
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

func clientForProtocol(clients []sqlcdb.DownloadClient, proto indexer.DownloadProtocol) *sqlcdb.DownloadClient {
	for i := range clients {
		if clients[i].Type == string(proto) {
			return &clients[i]
		}
	}

	return nil
}
