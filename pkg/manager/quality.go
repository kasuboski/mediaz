package manager

import (
	"github.com/kasuboski/mediaz/pkg/storage"
)

// MeetsQualitySize checks if the given fileSize (MB) and runtime (min) fall within the QualitySize
func MeetsQualitySize(qs storage.QualityDefinition, fileSize uint64, runtime uint64) bool {
	fileRatio := float64(fileSize) / float64(runtime)

	if fileRatio < qs.MinSize {
		return false
	}

	if fileRatio > qs.MaxSize {
		return false
	}

	return true
}
