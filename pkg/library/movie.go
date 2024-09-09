package library

import "time"

type MovieFile struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
	Size string `json:"size"`
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
