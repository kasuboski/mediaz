package download

import (
	"testing"

	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/stretchr/testify/assert"
)

func TestDownloadClientFactory_NewDownloadClient(t *testing.T) {
	t.Run("transmission client", func(t *testing.T) {
		factory := NewDownloadClientFactory("mount")

		client, err := factory.NewDownloadClient(model.DownloadClient{
			Implementation: "transmission",
		})
		tc, ok := client.(*TransmissionClient)
		assert.True(t, ok, "client should be of type *TransmissionClient")

		assert.Equal(t, "mount", tc.mountPrefix)
		assert.Nil(t, err)
	})

	t.Run("sabnzbd client", func(t *testing.T) {
		factory := NewDownloadClientFactory()

		apiKey := "hello"
		client, err := factory.NewDownloadClient(model.DownloadClient{
			Implementation: "sabnzbd",
			APIKey:         &apiKey,
		})
		_, ok := client.(*SabnzbdClient)
		assert.True(t, ok, "client should be of type *SabnzbdClient")

		assert.Nil(t, err)
	})

	t.Run("sabnzbd client missing api key", func(t *testing.T) {
		factory := NewDownloadClientFactory()

		_, err := factory.NewDownloadClient(model.DownloadClient{
			Implementation: "sabnzbd",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing api key")
	})

	t.Run("unsupported client", func(t *testing.T) {
		factory := NewDownloadClientFactory()

		_, err := factory.NewDownloadClient(model.DownloadClient{
			Implementation: "my-client-implementation",
		})
		assert.Equal(t, "unsupported client implementation: my-client-implementation", err.Error())
	})

}
