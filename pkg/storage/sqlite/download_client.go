package sqlite

import (
	"context"

	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

// CreateDownloadClient stores a new download client
func (s *SQLite) CreateDownloadClient(ctx context.Context, client sqlcdb.DownloadClient) (int64, error) {
	return s.Queries.CreateDownloadClient(ctx, sqlcdb.CreateDownloadClientParams{
		Type:           client.Type,
		Implementation: client.Implementation,
		Scheme:         client.Scheme,
		Host:           client.Host,
		Port:           client.Port,
		ApiKey:         client.ApiKey,
	})
}

// GetDownloadClient gets a stored download client given an id
func (s *SQLite) GetDownloadClient(ctx context.Context, id int64) (sqlcdb.DownloadClient, error) {
	client, err := s.Queries.GetDownloadClient(ctx, id)
	if err != nil {
		return sqlcdb.DownloadClient{}, mapQueryErr(err, "failed to get download client")
	}
	return client, nil
}

// ListDownloadClients lists all stored download clients ordered by id
func (s *SQLite) ListDownloadClients(ctx context.Context) ([]sqlcdb.DownloadClient, error) {
	return s.Queries.ListDownloadClients(ctx)
}

// UpdateDownloadClient updates an existing download client. An invalid (NULL)
// ApiKey leaves the stored key untouched (COALESCE in the query).
func (s *SQLite) UpdateDownloadClient(ctx context.Context, id int64, client sqlcdb.DownloadClient) error {
	return s.Queries.UpdateDownloadClient(ctx, sqlcdb.UpdateDownloadClientParams{
		Type:           client.Type,
		Implementation: client.Implementation,
		Scheme:         client.Scheme,
		Host:           client.Host,
		Port:           client.Port,
		ApiKey:         client.ApiKey,
		ID:             id,
	})
}

// DeleteDownloadClient deletes a download client given an id. Transitions
// referencing the client have their download_client_id set to NULL (ON DELETE
// SET NULL) so history survives.
func (s *SQLite) DeleteDownloadClient(ctx context.Context, id int64) error {
	return s.Queries.DeleteDownloadClient(ctx, id)
}
