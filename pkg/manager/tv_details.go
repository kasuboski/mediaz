package manager

import (
	"context"
	"encoding/json"
	"io"

	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/tmdb"
)

// getTVMetadataAndDetails retrieves both series metadata and full TMDB details
func (m MediaManager) getTVMetadataAndDetails(ctx context.Context, tmdbID int) (*model.SeriesMetadata, *tmdb.SeriesDetailsResponse, error) {
	// Get series metadata (creates if not exists)
	metadata, err := m.GetSeriesMetadata(ctx, tmdbID)
	if err != nil {
		return nil, nil, err
	}

	// Get the full series details response from TMDB to access networks and status
	res, err := m.tmdb.TvSeriesDetails(ctx, int32(tmdbID), nil)
	if err != nil {
		return nil, nil, err
	}
	defer res.Body.Close()

	b, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, nil, err
	}

	var seriesDetailsResponse tmdb.SeriesDetailsResponse
	if err := json.Unmarshal(b, &seriesDetailsResponse); err != nil {
		return nil, nil, err
	}
	return metadata, &seriesDetailsResponse, nil
}
