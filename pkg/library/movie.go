package library

type Movie struct {
	Name   string `json:"name"`
	TMDBID string `json:"tmdbid"`
	Path   string `json:"path"`
	Size   string `json:"size"`
}

func FromPath(path string) Movie {
	// Use the directory name to find the movie name
	name := dirName(path)
	return Movie{
		Name: sanitizeName(name),
		Path: path,
	}
}
