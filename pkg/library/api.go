package library

import (
	"context"
	"regexp"
)

const (
	moviePattern = `^((?:\w|\s|')+)(\(\d+\))?\s*({tmdb-\w+})?([[:print:]])*\.?\w*$`
	showPattern  = `^(\w|\s|')+(\((\w|\s)+\))*\s*(\(\d+\))*\s*({tmdb-\d+})?-?\s*([sS]\d{1,2}[eE]\d{1,2})?\s*-?\s*(\w|\s|'|-)*\s*(\(\d+\))?\.?\w*$`
)

var (
	movieRegex      = regexp.MustCompile(moviePattern)
	showRegex       = regexp.MustCompile(showPattern)
	videoExtensions = []string{".mp4", ".avi", ".mkv", ".m4v", ".iso", ".ts", ".m2ts"}
)

type Library interface {
	FindMovies(ctx context.Context) ([]MovieFile, error)
	AddMovie(ctx context.Context, sourcePath string) (MovieFile, error)

	FindEpisodes(ctx context.Context) ([]string, error)
}
