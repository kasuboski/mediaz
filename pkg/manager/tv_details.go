package manager

import (
	"context"

	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/tmdb"
)

// getTVMetadataAndDetails retrieves both series metadata and full TMDB details.
// Delegates to SeriesService.
func (m MediaManager) getTVMetadataAndDetails(ctx context.Context, tmdbID int) (*model.SeriesMetadata, *tmdb.SeriesDetailsResponse, error) {
	return m.seriesService.getTVMetadataAndDetails(ctx, tmdbID)
}
