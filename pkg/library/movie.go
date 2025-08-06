package library

import (
	"fmt"
)

type MovieFile struct {
	Name         string `json:"name"`
	RelativePath string `json:"path"`
	AbsolutePath string `json:"absolutePath"`
	Size         int64  `json:"size"`
}

func (mf MovieFile) String() string {
	return fmt.Sprintf("name: %s, relative path: %s, size in bytes: %d", mf.Name, mf.RelativePath, mf.Size)
}

func FromPath(path string) MovieFile {
	// Use the directory name to find the movie name
	return MovieFile{
		Name:         MovieNameFromFilepath(path),
		RelativePath: path,
	}
}
