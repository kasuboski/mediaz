package manager

import (
	"testing"

	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/stretchr/testify/assert"
)

func TestAvailableProtocols(t *testing.T) {
	clients := []*model.DownloadClient{
		{Type: "usenet"},
		{Type: "torrent"},
		{Type: "usenet"},
		{Type: "usenet"},
		{Type: "torrent"},
	}

	actual := availableProtocols(clients)
	assert.NotEmpty(t, actual)
	assert.Len(t, actual, 2)

	actual = availableProtocols([]*model.DownloadClient{})
	assert.Empty(t, actual)
}

func TestClientForProtocol(t *testing.T) {
	clients := []*model.DownloadClient{
		{ID: 1, Type: "usenet"},
		{ID: 2, Type: "torrent"},
		{ID: 3, Type: "usenet"},
		{ID: 4, Type: "usenet"},
		{ID: 5, Type: "torrent"},
	}

	t.Run("find torrent", func(t *testing.T) {
		actual := clientForProtocol(clients, prowlarr.DownloadProtocolTorrent)
		assert.NotNil(t, actual)
		assert.Equal(t, int32(2), actual.ID)
	})
	t.Run("find usenet", func(t *testing.T) {
		actual := clientForProtocol(clients, prowlarr.DownloadProtocolUsenet)
		assert.NotNil(t, actual)
		assert.Equal(t, int32(1), actual.ID)
	})

	t.Run("not found", func(t *testing.T) {
		actual := clientForProtocol([]*model.DownloadClient{{ID: 1, Type: "usenet"}}, prowlarr.DownloadProtocolTorrent)
		assert.Nil(t, actual)
	})

	t.Run("empty", func(t *testing.T) {
		actual := clientForProtocol([]*model.DownloadClient{}, prowlarr.DownloadProtocolTorrent)
		assert.Nil(t, actual)
	})
}
