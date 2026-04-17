package manager

import (
	"context"

	"github.com/kasuboski/mediaz/pkg/storage"
)

// IndexSeriesLibrary indexes the tv library directory for new files.
// Delegates to SeriesService.
func (m MediaManager) IndexSeriesLibrary(ctx context.Context) error {
	return m.seriesService.IndexSeriesLibrary(ctx)
}

// ensureSeries is called by reconciliation via MediaManager.
func (m MediaManager) ensureSeries(ctx context.Context, seriesName string) (int64, error) {
	return m.seriesService.ensureSeries(ctx, seriesName)
}

// getOrCreateSeason is called by reconciliation via MediaManager.
func (m MediaManager) getOrCreateSeason(ctx context.Context, seriesID int64, seasonNumber int32, seasonMetadataID *int32, initialState storage.SeasonState) (int64, error) {
	return m.seriesService.getOrCreateSeason(ctx, seriesID, seasonNumber, seasonMetadataID, initialState)
}
