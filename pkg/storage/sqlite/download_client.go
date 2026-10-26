package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

// CreateDownloadClient stores a new download client
func (s *SQLite) CreateDownloadClient(ctx context.Context, client model.DownloadClient) (int64, error) {
	return s.Queries.CreateDownloadClient(ctx, sqlcdb.CreateDownloadClientParams{
		Type:           client.Type,
		Implementation: client.Implementation,
		Scheme:         client.Scheme,
		Host:           client.Host,
		Port:           int64(client.Port),
		ApiKey:         cfgNullString(client.APIKey),
	})
}

// GetDownloadClient gets a stored download client given an id
func (s *SQLite) GetDownloadClient(ctx context.Context, id int64) (model.DownloadClient, error) {
	r, err := s.Queries.GetDownloadClient(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.DownloadClient{}, storage.ErrNotFound
		}
		return model.DownloadClient{}, err
	}

	return downloadClientFromRow(r), nil
}

// ListDownloadClients lists all stored download clients ordered by id
func (s *SQLite) ListDownloadClients(ctx context.Context) ([]*model.DownloadClient, error) {
	rows, err := s.Queries.ListDownloadClients(ctx)
	if err != nil {
		return nil, err
	}

	clients := make([]*model.DownloadClient, 0, len(rows))
	for _, r := range rows {
		client := downloadClientFromRow(r)
		clients = append(clients, &client)
	}

	return clients, nil
}

// UpdateDownloadClient updates an existing download client
func (s *SQLite) UpdateDownloadClient(ctx context.Context, id int64, client model.DownloadClient) error {
	return s.Queries.UpdateDownloadClient(ctx, sqlcdb.UpdateDownloadClientParams{
		Type:           client.Type,
		Implementation: client.Implementation,
		Scheme:         client.Scheme,
		Host:           client.Host,
		Port:           int64(client.Port),
		ApiKey:         cfgNullString(client.APIKey),
		ID:             id,
	})
}

// DeleteDownloadClient deletes a download client given an id
func (s *SQLite) DeleteDownloadClient(ctx context.Context, id int64) error {
	return s.Queries.DeleteDownloadClient(ctx, id)
}

// downloadClientFromRow converts a sqlc row into the domain model.
func downloadClientFromRow(r sqlcdb.DownloadClient) model.DownloadClient {
	return model.DownloadClient{
		ID:             int32(r.ID),
		Type:           r.Type,
		Implementation: r.Implementation,
		Scheme:         r.Scheme,
		Host:           r.Host,
		Port:           int32(r.Port),
		APIKey:         cfgStringPtr(r.ApiKey),
	}
}
