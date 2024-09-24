package library

import (
	"fmt"
	"time"
)

type MovieFile struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Size int64  `json:"size"`
}

func (mf MovieFile) String() string {
	return fmt.Sprintf("name: %s, path: %s, size: %s", mf.Name, mf.Path, fileSizeToString(mf.Size))
}

func FromPath(path string) MovieFile {
	// Use the directory name to find the movie name
	name := dirName(path)
	return MovieFile{
		Name: sanitizeName(name),
		Path: path,
	}
}

type Movie struct {
	ID              string    `json:"id"`
	MovieFileID     string    `json:"movie_file_id"`
	MovieMetadataID string    `json:"movie_metadata_id"`
	Added           time.Time `json:"added"`
	LastSearch      time.Time `json:"last_search"`
}

type MovieMetadata struct {
	ID     string `json:"id"`
	TMDBID int    `json:"tmdb_id"`
	Images string `json:"images"`
	// Genres string `json:"genres"`
	Title        string    `json:"title"`
	LastInfoSync time.Time `json:"last_info_sync"`
	Runtime      int       `json:"runtime"`
	Overview     string    `json:"overview"`
}
