package library

import (
	"context"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/kasuboski/mediaz/pkg/io"
	"github.com/kasuboski/mediaz/pkg/logger"

	"go.uber.org/zap"
)

// FileSystem describes where media lives
type FileSystem struct {
	FS   fs.FS
	Path string
}

// MediaLibary describes the media that create a library
type MediaLibrary struct {
	io     io.FileIO
	movies FileSystem
	tv     FileSystem
}

// New creates a new library
func New(movies FileSystem, tv FileSystem, io io.FileIO) Library {
	return &MediaLibrary{
		movies: movies,
		tv:     tv,
		io:     io,
	}
}

// AddMovie adds a movie file from an absolute path to the movie library. If a directory does not exist for the movie, it will be created using the title provided.
// TODO: add option to delete source file if we succesfully copy
func (l *MediaLibrary) AddMovie(ctx context.Context, title, sourcePath string) (MovieFile, error) {
	log := logger.FromCtx(ctx)
	log = log.With("source path", sourcePath, "movie library path", l.movies.Path)

	var movieFile MovieFile

	ok, err := l.io.IsSameFileSystem(l.movies.Path, sourcePath)
	if err != nil {
		log.Debug("failed to determine if request path and library path share a file system", zap.Error(err))
		return movieFile, err
	}

	// downloads/file.mp4 -> /library/movies/batman begins/file.mp4
	targetDir := filepath.Join(l.movies.Path, title)
	targetPath := filepath.Join(targetDir, filepath.Base(sourcePath))
	log = log.With("target dir", targetDir, "target path", targetPath)

	err = l.io.MkdirAll(targetDir, os.ModePerm)
	if err != nil {
		log.Error("failed to create movie directory", zap.Error(err))
		return movieFile, err
	}

	// rename the file if we're on the same file system to the movie library path to avoid copying
	if ok {
		log.Debug("renaming file")
		err := l.io.Rename(sourcePath, targetPath)
		if err != nil {
			log.Error("failed to rename file", zap.Error(err))
			return movieFile, err
		}
	} else {
		log.Debug("copying file")
		_, err = l.io.Copy(sourcePath, targetPath)
		if err != nil {
			log.Error("failed to copy file", zap.Error(err))
			return movieFile, err
		}
	}

	file, err := l.io.Open(targetPath)
	if err != nil {
		return movieFile, err
	}
	defer file.Close()

	movieFile.Name = sanitizeName(filepath.Base(file.Name()))
	info, err := file.Stat()
	if err != nil {
		log.Error("failed to state file", zap.Error(err))
		return movieFile, err
	}

	movieFile.Size = info.Size()
	movieFile.Path = targetPath

	return movieFile, nil
}

// FindMovies lists media in the movie library
func (l *MediaLibrary) FindMovies(ctx context.Context) ([]MovieFile, error) {
	log := logger.FromCtx(ctx)

	movies := []MovieFile{}
	err := l.io.WalkDir(l.movies.FS, ".", func(path string, d fs.DirEntry, err error) error {
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

		movie := FromPath(path)
		info, err := d.Info()
		if err == nil {
			movie.Size = info.Size()
		}

		movies = append(movies, movie)

		return nil
	})

	if err != nil {
		return nil, err
	}

	return movies, nil
}

// FindEpisodes lists episodes in the tv library
func (l *MediaLibrary) FindEpisodes(ctx context.Context) ([]string, error) {
	log := logger.FromCtx(ctx)
	episodes := []string{}
	err := fs.WalkDir(l.tv.FS, ".", func(path string, d fs.DirEntry, err error) error {
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

func dirName(path string) string {
	dirPath := filepath.Dir(path)
	split := strings.Split(dirPath, string(os.PathSeparator))
	return split[len(split)-1]
}

func fileSizeToString(size int64) string {
	const unit = 1024

	if size < unit {
		return fmt.Sprintf("%d B", size)
	}

	exp := int(math.Log(float64(size)) / math.Log(unit))
	pre := "KMGTPE"[exp-1]
	return fmt.Sprintf("%.1f %cB", float64(size)/math.Pow(unit, float64(exp)), pre)
}
