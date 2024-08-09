package library

import (
	"io/fs"
	"log"
	"regexp"
)

const (
	moviePattern = "((\\w|\\s)+)(\\(\\d+\\))?\\s*({tmdb-\\w+})*\\.*\\w*"
	showPattern  = "((\\w|\\s)+)(\\(\\d+\\))?\\s*([sS]\\d{1,2}[eE]\\d{1,2})\\s*({tmdb-\\w+})*\\.*\\w*"
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

		match := movieRegex.MatchString(d.Name())
		// This probaby doesn't take into account we only want one level of nesting
		if d.IsDir() {
			if !match && d.Name() != "." {
				log.Printf("skipping dir: %s", d.Name())
				return fs.SkipDir
			}
			return nil
		}

		if !match {
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

		match := showRegex.MatchString(d.Name())
		if d.IsDir() {
			if !match && d.Name() != "." {
				log.Printf("skipping dir: %s", d.Name())
				return fs.SkipDir
			}
			return nil
		}

		if !match {
			return nil
		}

		episodes = append(episodes, d.Name())

		return nil
	})

	return episodes
}
