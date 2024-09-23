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
	Added           time.Time `json:"added"`
	LastSearch      time.Time `json:"last_search"`
	ID              string    `json:"id"`
	MovieFileID     string    `json:"movie_file_id"`
	MovieMetadataID string    `json:"movie_metadata_id"`
}

type MovieMetadata struct {
	LastInfoSync time.Time `json:"last_info_sync"`
	ID           string    `json:"id"`
	Images       string    `json:"images"`
	Title        string    `json:"title"`
	Overview     string    `json:"overview"`
	TMDBID       int       `json:"tmdb_id"`
	Runtime      int       `json:"runtime"`
}
