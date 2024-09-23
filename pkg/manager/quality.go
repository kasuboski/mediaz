package manager

import (
	"log"

	"github.com/kasuboski/mediaz/pkg/storage"
)

// MeetsQualitySize checks if the given fileSize (MB) and runtime (min) fall within the QualitySize
func MeetsQualitySize(qs storage.QualityDefinition, fileSize uint64, runtime uint64) bool {
	log.Println("file size", fileSize)
	log.Println("file size", runtime)
	log.Println("quality name", qs.Name)
	log.Println("quality ratio min", qs.MinSize)
	log.Println("quality ratio max", qs.MaxSize)
	fileRatio := float64(fileSize) / float64(runtime)
	log.Println("file ratio", fileRatio)

	log.Println("comparing file ratio to min size", fileRatio, "<", qs.MinSize)
	if fileRatio < qs.MinSize {
		return false
	}

	log.Println("comparing file ratio to min size", fileRatio, ">", qs.MaxSize)
	if fileRatio > qs.MaxSize {
		return false
	}

	return true
}
