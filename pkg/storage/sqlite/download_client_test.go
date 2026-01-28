package sqlite

import (
	"context"
	"testing"

	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDownloadClientStorage(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)

	clientOne := model.DownloadClient{
		Type:           "torrent",
		Implementation: "transmission",
		Host:           "transmission",
		Scheme:         "http",
		Port:           9091,
	}

	client1ID, err := store.CreateDownloadClient(ctx, clientOne)
	assert.Nil(t, err)
	assert.NotZero(t, client1ID)

	storedClient, err := store.GetDownloadClient(ctx, client1ID)
	assert.Nil(t, err)
	assert.Equal(t, clientOne.Type, storedClient.Type)
	assert.Equal(t, clientOne.Implementation, storedClient.Implementation)
	assert.Equal(t, clientOne.Host, storedClient.Host)
	assert.Equal(t, clientOne.Scheme, storedClient.Scheme)
	assert.Equal(t, clientOne.Port, storedClient.Port)

	clientTwo := model.DownloadClient{
		Type:           "usenet",
		Implementation: "something",
		Host:           "host",
		Scheme:         "http",
		Port:           8080,
	}

	client2ID, err := store.CreateDownloadClient(ctx, clientTwo)
	assert.Nil(t, err)
	assert.NotZero(t, client2ID)

	storedClient, err = store.GetDownloadClient(ctx, client2ID)
	assert.Nil(t, err)
	assert.Equal(t, clientTwo.Type, storedClient.Type)
	assert.Equal(t, clientTwo.Implementation, storedClient.Implementation)
	assert.Equal(t, clientTwo.Host, storedClient.Host)
	assert.Equal(t, clientTwo.Scheme, storedClient.Scheme)
	assert.Equal(t, clientTwo.Port, storedClient.Port)

	err = store.DeleteDownloadClient(ctx, client1ID)
	assert.Nil(t, err)

	err = store.DeleteDownloadClient(ctx, client2ID)
	assert.Nil(t, err)
}

func TestUpdateDownloadClient(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)

	apiKey := "original-key"
	client := model.DownloadClient{
		Type:           "usenet",
		Implementation: "sabnzbd",
		Scheme:         "http",
		Host:           "localhost",
		Port:           8080,
		APIKey:         &apiKey,
	}

	id, err := store.CreateDownloadClient(ctx, client)
	require.NoError(t, err)

	newApiKey := "updated-key"
	updatedClient := model.DownloadClient{
		Type:           "usenet",
		Implementation: "sabnzbd",
		Scheme:         "https",
		Host:           "sabnzbd.updated.com",
		Port:           443,
		APIKey:         &newApiKey,
	}

	err = store.UpdateDownloadClient(ctx, id, updatedClient)
	require.NoError(t, err)

	retrieved, err := store.GetDownloadClient(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, "https", retrieved.Scheme)
	assert.Equal(t, "sabnzbd.updated.com", retrieved.Host)
	assert.Equal(t, int32(443), retrieved.Port)
	assert.Equal(t, &newApiKey, retrieved.APIKey)
}
