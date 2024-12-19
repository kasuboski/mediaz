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

func TestStatus_Finished(t *testing.T) {
	type fields struct {
		ID       string
		Name     string
		FilePath string
		Progress float64
		Speed    int64
		Size     int64
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		{
			name: "finished",
			fields: fields{
				Progress: 100,
			},
			want: true,
		},
		{
			name: "not finished",
			fields: fields{
				Progress: 12,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Status{
				ID:       tt.fields.ID,
				Name:     tt.fields.Name,
				FilePath: tt.fields.FilePath,
				Progress: tt.fields.Progress,
				Speed:    tt.fields.Speed,
				Size:     tt.fields.Size,
			}
			if got := s.Finished(); got != tt.want {
				t.Errorf("Status.Finished() = %v, want %v", got, tt.want)
			}
		})
	}
}
