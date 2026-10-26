package manager

import (
	"context"
	"database/sql"
	"testing"

	"github.com/kasuboski/mediaz/pkg/download"
	downloadMocks "github.com/kasuboski/mediaz/pkg/download/mocks"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/storage/mocks"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestAvailableProtocols(t *testing.T) {
	clients := []sqlcdb.DownloadClient{
		{Type: "usenet"},
		{Type: "torrent"},
		{Type: "usenet"},
		{Type: "usenet"},
		{Type: "torrent"},
	}

	actual := availableProtocols(clients)
	assert.NotEmpty(t, actual)
	assert.Len(t, actual, 2)

	actual = availableProtocols([]sqlcdb.DownloadClient{})
	assert.Empty(t, actual)
}

func TestClientForProtocol(t *testing.T) {
	clients := []sqlcdb.DownloadClient{
		{ID: 1, Type: "usenet"},
		{ID: 2, Type: "torrent"},
		{ID: 3, Type: "usenet"},
		{ID: 4, Type: "usenet"},
		{ID: 5, Type: "torrent"},
	}

	t.Run("find torrent", func(t *testing.T) {
		actual := clientForProtocol(clients, prowlarr.DownloadProtocolTorrent)
		assert.NotNil(t, actual)
		assert.Equal(t, int64(2), actual.ID)
	})
	t.Run("find usenet", func(t *testing.T) {
		actual := clientForProtocol(clients, prowlarr.DownloadProtocolUsenet)
		assert.NotNil(t, actual)
		assert.Equal(t, int64(1), actual.ID)
	})

	t.Run("not found", func(t *testing.T) {
		actual := clientForProtocol([]sqlcdb.DownloadClient{{ID: 1, Type: "usenet"}}, prowlarr.DownloadProtocolTorrent)
		assert.Nil(t, actual)
	})

	t.Run("empty", func(t *testing.T) {
		actual := clientForProtocol([]sqlcdb.DownloadClient{}, prowlarr.DownloadProtocolTorrent)
		assert.Nil(t, actual)
	})
}

func TestUpdateDownloadClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	store := mocks.NewMockStorage(ctrl)
	ds := NewDownloadClientService(store, nil)

	t.Run("update with new API key", func(t *testing.T) {
		newApiKey := "new-api-key"
		request := UpdateDownloadClientRequest{
			DownloadClient: sqlcdb.DownloadClient{
				Type:           "usenet",
				Implementation: "sabnzbd",
				Scheme:         "https",
				Host:           "sabnzbd.example.com",
				Port:           443,
				ApiKey:         sql.NullString{String: newApiKey, Valid: true},
			},
		}

		store.EXPECT().UpdateDownloadClient(ctx, int64(1), gomock.Any()).Return(nil)

		result, err := ds.UpdateDownloadClient(ctx, 1, request)
		require.NoError(t, err)
		assert.Equal(t, int64(1), result.ID)
		assert.Equal(t, "usenet", result.Type)
		assert.Equal(t, sql.NullString{String: newApiKey, Valid: true}, result.ApiKey)
	})

	t.Run("update preserves existing API key when not provided", func(t *testing.T) {
		existingApiKey := "existing-key"
		existingClient := sqlcdb.DownloadClient{
			ID:             1,
			Type:           "usenet",
			Implementation: "sabnzbd",
			Scheme:         "http",
			Host:           "localhost",
			Port:           8080,
			ApiKey:         sql.NullString{String: existingApiKey, Valid: true},
		}

		emptyKey := ""
		request := UpdateDownloadClientRequest{
			DownloadClient: sqlcdb.DownloadClient{
				Type:           "usenet",
				Implementation: "sabnzbd",
				Scheme:         "https",
				Host:           "sabnzbd.example.com",
				Port:           443,
				ApiKey:         sql.NullString{String: emptyKey, Valid: true},
			},
		}

		store.EXPECT().GetDownloadClient(ctx, int64(1)).Return(existingClient, nil)
		store.EXPECT().UpdateDownloadClient(ctx, int64(1), gomock.Any()).DoAndReturn(
			func(ctx context.Context, id int64, client sqlcdb.DownloadClient) error {
				assert.Equal(t, sql.NullString{String: existingApiKey, Valid: true}, client.ApiKey)
				return nil
			},
		)

		result, err := ds.UpdateDownloadClient(ctx, 1, request)
		require.NoError(t, err)
		assert.Equal(t, sql.NullString{String: existingApiKey, Valid: true}, result.ApiKey)
	})

	t.Run("update preserves API key when nil", func(t *testing.T) {
		existingApiKey := "existing-key"
		existingClient := sqlcdb.DownloadClient{
			ID:             1,
			Type:           "usenet",
			Implementation: "sabnzbd",
			Scheme:         "http",
			Host:           "localhost",
			Port:           8080,
			ApiKey:         sql.NullString{String: existingApiKey, Valid: true},
		}

		request := UpdateDownloadClientRequest{
			DownloadClient: sqlcdb.DownloadClient{
				Type:           "usenet",
				Implementation: "sabnzbd",
				Scheme:         "https",
				Host:           "sabnzbd.example.com",
				Port:           443,
				ApiKey:         sql.NullString{},
			},
		}

		store.EXPECT().GetDownloadClient(ctx, int64(1)).Return(existingClient, nil)
		store.EXPECT().UpdateDownloadClient(ctx, int64(1), gomock.Any()).DoAndReturn(
			func(ctx context.Context, id int64, client sqlcdb.DownloadClient) error {
				assert.Equal(t, sql.NullString{String: existingApiKey, Valid: true}, client.ApiKey)
				return nil
			},
		)

		result, err := ds.UpdateDownloadClient(ctx, 1, request)
		require.NoError(t, err)
		assert.Equal(t, sql.NullString{String: existingApiKey, Valid: true}, result.ApiKey)
	})
}

func TestTestDownloadClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	factory := downloadMocks.NewMockFactory(ctrl)
	ds := NewDownloadClientService(nil, factory)

	t.Run("successful connection test", func(t *testing.T) {
		client := downloadMocks.NewMockDownloadClient(ctrl)
		request := AddDownloadClientRequest{
			DownloadClient: sqlcdb.DownloadClient{
				Type:           "torrent",
				Implementation: "transmission",
				Scheme:         "http",
				Host:           "localhost",
				Port:           9091,
				ApiKey:         sql.NullString{},
			},
		}

		factory.EXPECT().NewDownloadClient(request.DownloadClient).Return(client, nil)
		client.EXPECT().List(ctx).Return([]download.Status{}, nil)

		err := ds.TestDownloadClient(ctx, request)
		assert.NoError(t, err)
	})

	t.Run("failed connection test", func(t *testing.T) {
		client := downloadMocks.NewMockDownloadClient(ctrl)
		request := AddDownloadClientRequest{
			DownloadClient: sqlcdb.DownloadClient{
				Type:           "torrent",
				Implementation: "transmission",
				Scheme:         "http",
				Host:           "invalid-host",
				Port:           9091,
				ApiKey:         sql.NullString{},
			},
		}

		factory.EXPECT().NewDownloadClient(request.DownloadClient).Return(client, nil)
		client.EXPECT().List(ctx).Return(nil, assert.AnError)

		err := ds.TestDownloadClient(ctx, request)
		assert.Error(t, err)
	})
}
