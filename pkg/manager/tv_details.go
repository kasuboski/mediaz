package manager

import (
	"context"

	sqlcdb "github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/kasuboski/mediaz/pkg/tmdb"
)

// getTVMetadataAndDetails retrieves both series metadata and full TMDB details.
// Delegates to SeriesService.
func (m MediaManager) getTVMetadataAndDetails(ctx context.Context, tmdbID int64) (*sqlcdb.SeriesMetadatum, *tmdb.SeriesDetailsResponse, error) {
	return m.seriesService.getTVMetadataAndDetails(ctx, tmdbID)
}
