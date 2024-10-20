package download

import (
	"testing"

	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/stretchr/testify/assert"
)

func TestDownloadClientFactory_NewDownloadClient(t *testing.T) {
	t.Run("transmission client", func(t *testing.T) {
		factory := NewDownloadClientFactory()

		client, err := factory.NewDownloadClient(model.DownloadClient{
			Implementation: "transmission",
		})
		_, ok := client.(*TransmissionClient)
		assert.True(t, ok, "client should be of type *TransmissionClient")

		assert.Nil(t, err)
	})

	t.Run("unsupported client", func(t *testing.T) {
		factory := NewDownloadClientFactory()

		_, err := factory.NewDownloadClient(model.DownloadClient{
			Implementation: "my-client-implementation",
		})
		assert.Equal(t, "unsupported client implementation: my-client-implementation", err.Error())
	})

}
