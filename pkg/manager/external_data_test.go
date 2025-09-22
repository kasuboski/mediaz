package manager

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSerializeExternalIDs(t *testing.T) {
	t.Run("serializes valid external IDs", func(t *testing.T) {
		imdbID := "tt1234567"
		tvdbID := 12345
		data := &ExternalIDsData{
			ImdbID: &imdbID,
			TvdbID: &tvdbID,
		}

		result, err := SerializeExternalIDs(data)
		require.NoError(t, err)
		require.NotNil(t, result)

		expected := `{"imdb_id":"tt1234567","tvdb_id":12345}`
		assert.Equal(t, expected, *result)
	})

	t.Run("handles nil input", func(t *testing.T) {
		result, err := SerializeExternalIDs(nil)
		require.NoError(t, err)
		assert.Nil(t, result)
	})
}

func TestDeserializeExternalIDs(t *testing.T) {
	t.Run("deserializes valid JSON", func(t *testing.T) {
		jsonStr := `{"imdb_id":"tt1234567","tvdb_id":12345}`

		result, err := DeserializeExternalIDs(&jsonStr)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "tt1234567", *result.ImdbID)
		assert.Equal(t, 12345, *result.TvdbID)
	})

	t.Run("handles nil input", func(t *testing.T) {
		result, err := DeserializeExternalIDs(nil)
		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("handles empty string", func(t *testing.T) {
		emptyStr := ""
		result, err := DeserializeExternalIDs(&emptyStr)
		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("handles invalid JSON", func(t *testing.T) {
		invalidJSON := `{"invalid":}`
		result, err := DeserializeExternalIDs(&invalidJSON)
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestSerializeWatchProviders(t *testing.T) {
	t.Run("serializes valid watch providers", func(t *testing.T) {
		logoPath := "/netflix.png"
		data := &WatchProvidersData{
			US: WatchProviderRegionData{
				Flatrate: []WatchProviderData{
					{
						ProviderID: 8,
						Name:       "Netflix",
						LogoPath:   &logoPath,
					},
				},
			},
		}

		result, err := SerializeWatchProviders(data)
		require.NoError(t, err)
		require.NotNil(t, result)

		expected := `{"US":{"flatrate":[{"provider_id":8,"provider_name":"Netflix","logo_path":"/netflix.png"}],"link":null}}`
		assert.Equal(t, expected, *result)
	})

	t.Run("handles nil input", func(t *testing.T) {
		result, err := SerializeWatchProviders(nil)
		require.NoError(t, err)
		assert.Nil(t, result)
	})
}

func TestDeserializeWatchProviders(t *testing.T) {
	t.Run("deserializes valid JSON", func(t *testing.T) {
		jsonStr := `{"US":{"flatrate":[{"provider_id":8,"provider_name":"Netflix","logo_path":"/netflix.png"}],"link":null}}`

		result, err := DeserializeWatchProviders(&jsonStr)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Len(t, result.US.Flatrate, 1)
		assert.Equal(t, 8, result.US.Flatrate[0].ProviderID)
		assert.Equal(t, "Netflix", result.US.Flatrate[0].Name)
		assert.Equal(t, "/netflix.png", *result.US.Flatrate[0].LogoPath)
	})

	t.Run("handles nil input", func(t *testing.T) {
		result, err := DeserializeWatchProviders(nil)
		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("handles empty string", func(t *testing.T) {
		emptyStr := ""
		result, err := DeserializeWatchProviders(&emptyStr)
		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("handles invalid JSON", func(t *testing.T) {
		invalidJSON := `{"invalid":}`
		result, err := DeserializeWatchProviders(&invalidJSON)
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}
