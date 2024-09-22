package manager

import "github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"

// QualitySizes represents the file size cut offs for different qualities
// Sizes are MB/min
type QualitySizes struct {
	TrashID   string        `json:"trash_id"`
	Type      string        `json:"type"` // movie, series, anime
	Qualities []QualitySize `json:"qualities"`
}

type QualitySize struct {
	Quality   string  `json:"quality"`
	Min       float64 `json:"min"`
	Preferred uint64  `json:"preferred"`
	Max       uint64  `json:"max"`
}

// MeetsQualitySize checks if the given fileSize (MB) and runtime (min) fall within the QualitySize
func MeetsQualitySize(qs model.QualityDefinition, fileSize uint64, runtime uint64) bool {
	fileRatio := float64(fileSize) / float64(runtime)
	if fileRatio < float64(qs.MinSize) {
		return false
	}

	if fileRatio > float64(qs.MaxSize) {
		return false
	}

	return true
}
