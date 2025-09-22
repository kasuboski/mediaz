package manager

import (
	"encoding/json"
)

// ExternalIDsData represents the external identifiers for a series
type ExternalIDsData struct {
	ImdbID *string `json:"imdb_id"`
	TvdbID *int    `json:"tvdb_id"`
}

// WatchProviderData represents a streaming provider
type WatchProviderData struct {
	ProviderID int     `json:"provider_id"`
	Name       string  `json:"provider_name"`
	LogoPath   *string `json:"logo_path"`
}

// WatchProvidersData represents the watch providers for a series
type WatchProvidersData struct {
	US WatchProviderRegionData `json:"US"`
}

// WatchProviderRegionData represents providers for a specific region
type WatchProviderRegionData struct {
	Flatrate []WatchProviderData `json:"flatrate"`
	Link     *string             `json:"link"`
}

// SerializeExternalIDs converts ExternalIDsData to JSON string for database storage
func SerializeExternalIDs(data *ExternalIDsData) (*string, error) {
	if data == nil {
		return nil, nil
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	result := string(jsonData)
	return &result, nil
}

// DeserializeExternalIDs converts JSON string from database to ExternalIDsData
func DeserializeExternalIDs(data *string) (*ExternalIDsData, error) {
	if data == nil || *data == "" {
		return nil, nil
	}

	var result ExternalIDsData
	err := json.Unmarshal([]byte(*data), &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

// SerializeWatchProviders converts WatchProvidersData to JSON string for database storage
func SerializeWatchProviders(data *WatchProvidersData) (*string, error) {
	if data == nil {
		return nil, nil
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	result := string(jsonData)
	return &result, nil
}

// DeserializeWatchProviders converts JSON string from database to WatchProvidersData
func DeserializeWatchProviders(data *string) (*WatchProvidersData, error) {
	if data == nil || *data == "" {
		return nil, nil
	}

	var result WatchProvidersData
	err := json.Unmarshal([]byte(*data), &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}
