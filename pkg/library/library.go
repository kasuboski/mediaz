package library

import (
	"context"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kasuboski/mediaz/pkg/logger"
)

const (
	moviePattern = `^((\w|\s|')+)(\(\d+\))?\s*({tmdb-\w+})?(\w|\s|'|-)*\.?\w*$`
	showPattern  = `^(\w|\s|')+(\((\w|\s)+\))*\s*(\(\d+\))*\s*({tmdb-\d+})?-?\s*([sS]\d{1,2}[eE]\d{1,2})?\s*-?\s*(\w|\s|'|-)*\s*(\(\d+\))?\.?\w*$`
)

var (
	movieRegex      = regexp.MustCompile(moviePattern)
	showRegex       = regexp.MustCompile(showPattern)
	videoExtensions = []string{".mp4", ".avi", ".mkv", ".m4v"}
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

func (l *Library) FindMovies(ctx context.Context) ([]string, error) {
	log := logger.FromCtx(ctx)

	movies := []string{}
	err := fs.WalkDir(l.movies, ".", func(path string, d fs.DirEntry, err error) error {
		log.Debugw("movie walk", "path", path)
		if err != nil {
			// just skip this dir for now if there's an issue
			return fs.SkipDir
		}

		match := matchMovie(d.Name())
		nesting := levelsOfNesting(path)
		if d.IsDir() {
			if nesting > 0 || (!match && d.Name() != ".") {
				log.Debugw("skipping", "dir", d.Name())
				return fs.SkipDir
			}
			return nil
		}

		if !match || nesting == 0 || !isVideoFile(path) {
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

func (l *Library) FindEpisodes(ctx context.Context) ([]string, error) {
	log := logger.FromCtx(ctx)
	episodes := []string{}
	err := fs.WalkDir(l.tv, ".", func(path string, d fs.DirEntry, err error) error {
		log.Debugw("episode walk", "path", path)
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
				log.Debugw("skipping", "dir", d.Name())
				return fs.SkipDir
			}
			return nil
		}

		if !match || nesting == 0 || !isVideoFile(path) {
			return nil
		}

		episodes = append(episodes, d.Name())

		return nil
	})

	if err != nil {
		return nil, err
	}

	return episodes, nil
}

func levelsOfNesting(path string) int {
	return strings.Count(path, "/")
}

func matchMovie(name string) bool {
	return movieRegex.MatchString(sanitizeName(name))
}

func matchEpisode(name string) bool {
	return showRegex.MatchString(sanitizeName(name))
}

func isVideoFile(name string) bool {
	ext := filepath.Ext(name)
	for _, e := range videoExtensions {
		if strings.ToLower(ext) == e {
			return true
		}
	}

	return false
}

func sanitizeName(name string) string {
	return strings.Trim(strings.TrimSpace(name), "'")
}
