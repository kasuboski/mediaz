package library

import (
	"fmt"
	"path/filepath"
	"strings"
)

type EpisodeFile struct {
	Name         string `json:"name"`
	RelativePath string `json:"path"`
	AbsolutePath string `json:"absolutePath"`
	Size         int64  `json:"size"`
	SeriesTitle  string `json:"seriesTitle"`
	Season       int32  `json:"season"`
}

func (ef EpisodeFile) String() string {
	return fmt.Sprintf("name: %s, series: %s, season: %d, relative path: %s, size in bytes: %d",
		ef.Name, ef.SeriesTitle, ef.Season, ef.RelativePath, ef.Size)
}

// formatSeasonDirectory formats season number as "Season XX"
func formatSeasonDirectory(seasonNumber int32) string {
	return fmt.Sprintf("Season %02d", seasonNumber)
}

// formatEpisodeFilename creates a standardized episode filename
func formatEpisodeFilename(seriesTitle string, originalFilename string) string {
	ext := filepath.Ext(originalFilename)
	base := strings.TrimSuffix(filepath.Base(originalFilename), ext)

	// If the filename already contains the series title, use it as-is
	if strings.Contains(base, seriesTitle) {
		return originalFilename
	}

	// Otherwise, prepend the series title
	return fmt.Sprintf("%s - %s%s", seriesTitle, base, ext)
}
