package sqlite

import (
	"context"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
)

// GetDownloadClient gets a stored download client given an id
func (s *SQLite) GetDownloadClient(ctx context.Context, id int64) (model.DownloadClient, error) {
	stmt := table.DownloadClient.SELECT(table.DownloadClient.AllColumns).FROM(table.DownloadClient).WHERE(table.DownloadClient.ID.EQ(sqlite.Int64(id)))
	var result model.DownloadClient
	err := stmt.QueryContext(ctx, s.db, &result)
	return result, err
}

// ListDownloadClients lists all stored download clients
func (s *SQLite) ListDownloadClients(ctx context.Context) ([]*model.DownloadClient, error) {
	items := make([]*model.DownloadClient, 0)
	stmt := table.DownloadClient.SELECT(table.DownloadClient.AllColumns).FROM(table.DownloadClient).ORDER_BY(table.DownloadClient.ID.ASC())
	err := stmt.QueryContext(ctx, s.db, &items)
	return items, err
}

// UpdateDownloadClient updates an existing download client
func (s *SQLite) UpdateDownloadClient(ctx context.Context, id int64, client model.DownloadClient) error {
	stmt := table.DownloadClient.UPDATE(
		table.DownloadClient.Type,
		table.DownloadClient.Implementation,
		table.DownloadClient.Scheme,
		table.DownloadClient.Host,
		table.DownloadClient.Port,
		table.DownloadClient.APIKey,
	).MODEL(client).WHERE(table.DownloadClient.ID.EQ(sqlite.Int64(id)))

	_, err := stmt.ExecContext(ctx, s.db)
	return err
}

// DeleteDownloadClient deletes a download client given an id
func (s *SQLite) DeleteDownloadClient(ctx context.Context, id int64) error {
	stmt := table.DownloadClient.DELETE().WHERE(table.DownloadClient.ID.EQ(sqlite.Int64(id)))
	_, err := s.handleDelete(ctx, stmt)
	return err
}

// CreateDownloadClient stores a new download client
func (s *SQLite) CreateDownloadClient(ctx context.Context, profile model.DownloadClient) (int64, error) {
	stmt := table.DownloadClient.INSERT(table.DownloadClient.AllColumns.Except(table.DownloadClient.ID)).MODEL(profile).RETURNING(table.DownloadClient.ID)
	result, err := s.handleInsert(ctx, stmt)
	if err != nil {
		return 0, err
	}

	inserted, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return inserted, nil
}
