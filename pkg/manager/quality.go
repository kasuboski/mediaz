package manager

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
	Preferred int     `json:"preferred"`
	Max       int     `json:"max"`
}

// MeetsQualitySize checks if the given fileSize (MB) and runtime (min) fall within the QualitySize
func MeetsQualitySize(qs QualitySize, fileSize int, runtime int) bool {
	fileRatio := float64(fileSize) / float64(runtime)
	if fileRatio < qs.Min {
		return false
	}

	if fileRatio > float64(qs.Max) {
		return false
	}

	return true
}
