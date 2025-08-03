package manager

type LibraryShow struct {
	Path       string `json:"path"`
	TMDBID     int32  `json:"tmdbID"`
	Title      string `json:"title"`
	PosterPath string `json:"poster_path"`
	Year       int32  `json:"year,omitempty"`
	State      string `json:"state"`
}
