package library

type Movie struct {
	Name   string
	TMDBID string
	Path   string
}

func FromPath(path string) Movie {
	// Use the directory name to find the movie name
	name := dirName(path)
	return Movie{
		Name: sanitizeName(name),
		Path: path,
	}
}
