package library

import (
	"io/fs"
	"log"
	"regexp"
	"strings"
)

const (
	// moviePattern = "((\\w|\\s)+)(\\(\\d+\\))?\\s*({tmdb-\\w+})*\\.*\\w*"
	moviePattern = `^((\w|\s|')+)(\(\d+\))?\s*({tmdb-\w+})?\.?\w*$`
	showPattern  = `^(\w|\s|')+(\((\w|\s)+\))*\s*(\(\d+\))*\s*({tmdb-\d+})?-?\s*([sS]\d{1,2}[eE]\d{1,2})?\s*-?\s*(\w|\s|'|-)*\s*(\(\d+\))?\.?\w*$`
)

var (
	movieRegex = regexp.MustCompile(moviePattern)
	showRegex  = regexp.MustCompile(showPattern)
)

type Library struct {
	movies fs.FS
	tv     fs.FS
}

func New(movies fs.FS, tv fs.FS) Library {
	return Library{
		movies,
		tv,
	}
}

func (l *Library) FindMovies() ([]string, error) {
	movies := []string{}
	err := fs.WalkDir(l.movies, ".", func(path string, d fs.DirEntry, err error) error {
		log.Printf("movie walk: %s", path)
		if err != nil {
			// just skip this dir for now if there's an issue
			return fs.SkipDir
		}

		match := matchMovie(d.Name())
		nesting := levelsOfNesting(path)
		if d.IsDir() {
			if nesting > 0 || (!match && d.Name() != ".") {
				log.Printf("skipping dir: %s", d.Name())
				return fs.SkipDir
			}
			return nil
		}

		if !match || nesting == 0 {
			return nil
		}

		movies = append(movies, d.Name())

		return nil
	})

	if err != nil {
		return nil, err
	}

	return movies, nil
}

func (l *Library) FindEpisodes() []string {
	episodes := []string{}
	fs.WalkDir(l.tv, ".", func(path string, d fs.DirEntry, err error) error {
		log.Printf("episode walk: %s", path)
		if err != nil {
			// just skip this dir for now if there's an issue
			return fs.SkipDir
		}

		match := matchEpisode(d.Name())
		nesting := levelsOfNesting(path)
		if d.IsDir() {
			if strings.Contains(d.Name(), "Season ") {
				return nil
			}
			if nesting > 2 || (!match && d.Name() != ".") {
				log.Printf("skipping dir: %s", d.Name())
				return fs.SkipDir
			}
			return nil
		}

		if !match || nesting == 0 {
			return nil
		}

		episodes = append(episodes, d.Name())

		return nil
	})

	return episodes
}

func levelsOfNesting(path string) int {
	return strings.Count(path, "/")
}

func matchMovie(name string) bool {
	return movieRegex.MatchString(name)
}

func matchEpisode(name string) bool {
	return showRegex.MatchString(name)
}
