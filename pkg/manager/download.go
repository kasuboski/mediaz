package manager

import (
	"context"

	"github.com/kasuboski/mediaz/pkg/download"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
)

// AddDownloadClientRequest describes what is required to add a download client
type AddDownloadClientRequest struct {
	model.DownloadClient
}

func (m MediaManager) CreateDownloadClient(ctx context.Context, request AddDownloadClientRequest) (model.DownloadClient, error) {
	downloadClient := request.DownloadClient

	id, err := m.storage.CreateDownloadClient(ctx, request.DownloadClient)
	if err != nil {
		return downloadClient, err
	}

	downloadClient.ID = int32(id)
	return downloadClient, nil
}

func (m MediaManager) GetDownloadClient(ctx context.Context, id int64) (download.DownloadClient, error) {
	client, err := m.storage.GetDownloadClient(ctx, id)
	if err != nil {
		return nil, err
	}

	return m.factory.NewDownloadClient(client)
}

func (m MediaManager) ListDownloadClients(ctx context.Context) ([]*model.DownloadClient, error) {
	return m.storage.ListDownloadClients(ctx)
}

func (m MediaManager) DeleteDownloadClient(ctx context.Context, id int64) error {
	return m.storage.DeleteDownloadClient(ctx, id)
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
