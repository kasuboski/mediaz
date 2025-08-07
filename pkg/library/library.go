package library

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
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

// AddMovie adds a movie file from an absolute path to the movie library.
// If a directory does not exist for the movie, it will be created using the title provided.
// This assumes the source path is not already relative to the library, i.e it was downloaded or discoverd outside of the library.
func (l *MediaLibrary) AddMovie(ctx context.Context, title, sourcePath string) (MovieFile, error) {
	log := logger.FromCtx(ctx)
	log = log.With("source path", sourcePath, "movie library path", l.movies.Path, "title", title)

	var movieFile MovieFile

	// downloads/file.mp4 -> /library/movies/batman begins/file.mp4
	targetDir := filepath.Join(l.movies.Path, title)
	targetPath := filepath.Join(targetDir, filepath.Base(sourcePath))

	fileInfo, actualTargetPath, err := l.moveFileToLibrary(ctx, sourcePath, targetPath, l.movies.Path)
	if err != nil {
		return movieFile, err
	}

	movieFile.Name = sanitizeName(filepath.Base(actualTargetPath))
	movieFile.Size = fileInfo.Size()
	movieFile.RelativePath = fmt.Sprintf("%s/%s", title, movieFile.Name)
	movieFile.AbsolutePath = actualTargetPath

	return movieFile, nil
}

// moveFileToLibrary is a common helper that handles the file system operations
// for moving files from downloads to library locations. Returns the file info and the actual target path used.
func (l *MediaLibrary) moveFileToLibrary(ctx context.Context, sourcePath, targetPath, libraryRoot string) (os.FileInfo, string, error) {
	log := logger.FromCtx(ctx)

	// Sanitize the target filename
	targetDir := filepath.Dir(targetPath)
	originalFilename := filepath.Base(targetPath)
	sanitizedFilename := sanitizeFilename(originalFilename)
	sanitizedTargetPath := filepath.Join(targetDir, sanitizedFilename)

	log = log.With("source path", sourcePath, "target path", sanitizedTargetPath, "library root", libraryRoot)

	ok, err := l.io.IsSameFileSystem(libraryRoot, sourcePath)
	if err != nil {
		log.Debug("failed to determine if request path and library path share a file system", zap.Error(err))
		return nil, "", err
	}

	// Create target directory
	err = l.io.MkdirAll(targetDir, os.ModePerm)
	if err != nil {
		log.Error("failed to create target directory", zap.Error(err))
		return nil, "", err
	}

	if ok {
		log.Debug("renaming file")
		err := l.io.Rename(sourcePath, sanitizedTargetPath)
		if err != nil {
			log.Error("failed to rename file", zap.Error(err))
			return nil, "", err
		}
	} else {
		log.Debug("copying file")
		_, err = l.io.Copy(sourcePath, sanitizedTargetPath)
		if err != nil {
			log.Error("failed to copy file", zap.Error(err))
			return nil, "", err
		}
	}

	file, err := l.io.Open(sanitizedTargetPath)
	if err != nil {
		return nil, "", err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		log.Error("failed to stat file", zap.Error(err))
		return nil, "", err
	}

	return info, sanitizedTargetPath, nil
}

// AddEpisode adds an episode file from an absolute path to the TV library.
// The episode will be organized into the proper series/season directory structure.
// This assumes the source path is not already relative to the library.
func (l *MediaLibrary) AddEpisode(ctx context.Context, seriesTitle string, seasonNumber int32, sourcePath string) (EpisodeFile, error) {
	log := logger.FromCtx(ctx)
	log = log.With("source path", sourcePath, "tv library path", l.tv.Path, "series", seriesTitle, "season", seasonNumber)

	var episodeFile EpisodeFile

	// downloads/episode.mp4 -> /library/tv/Series Name (Year)/Season XX/Series Name (Year) - sXXeXX - Episode Title.mp4
	seriesDir := filepath.Join(l.tv.Path, seriesTitle)
	seasonDir := filepath.Join(seriesDir, formatSeasonDirectory(seasonNumber))
	filename := formatEpisodeFilename(seriesTitle, filepath.Base(sourcePath))
	targetPath := filepath.Join(seasonDir, filename)

	fileInfo, actualTargetPath, err := l.moveFileToLibrary(ctx, sourcePath, targetPath, l.tv.Path)
	if err != nil {
		return episodeFile, err
	}

	episodeFile.Name = sanitizeName(filepath.Base(actualTargetPath))
	episodeFile.Size = fileInfo.Size()
	episodeFile.SeriesTitle = seriesTitle
	episodeFile.Season = seasonNumber
	episodeFile.RelativePath = fmt.Sprintf("%s/%s/%s", seriesTitle, formatSeasonDirectory(seasonNumber), episodeFile.Name)
	episodeFile.AbsolutePath = actualTargetPath

	return episodeFile, nil
} // FindMovies lists media in the movie library
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
	return slices.Contains(videoExtensions, strings.ToLower(ext))
}

// MovieNameFromFilepath builds a sanitized name from the path to a movie file
// Example /movies/My Movie/my-movie.mpv -> My Movie
func MovieNameFromFilepath(path string) string {
	dir := dirName(path)
	return sanitizeName(dir)
}

func sanitizeName(name string) string {
	return strings.Trim(strings.TrimSpace(name), "'")
}

func sanitizeFilename(filename string) string {
	// Get the file extension first
	ext := filepath.Ext(filename)
	nameWithoutExt := strings.TrimSuffix(filename, ext)

	// Remove or replace problematic characters
	sanitized := strings.ReplaceAll(nameWithoutExt, "/", "-")
	sanitized = strings.ReplaceAll(sanitized, "\\", "-")
	sanitized = strings.ReplaceAll(sanitized, ":", "-")
	sanitized = strings.ReplaceAll(sanitized, "*", "-")
	sanitized = strings.ReplaceAll(sanitized, "?", "-")
	sanitized = strings.ReplaceAll(sanitized, "\"", "-")
	sanitized = strings.ReplaceAll(sanitized, "<", "-")
	sanitized = strings.ReplaceAll(sanitized, ">", "-")
	sanitized = strings.ReplaceAll(sanitized, "|", "-")

	// Remove multiple consecutive dashes and trim
	for strings.Contains(sanitized, "--") {
		sanitized = strings.ReplaceAll(sanitized, "--", "-")
	}
	sanitized = strings.Trim(sanitized, "-")
	sanitized = strings.TrimSpace(sanitized)

	// Reassemble with extension
	return sanitized + ext
}

func dirName(path string) string {
	dirPath := filepath.Dir(path)
	split := strings.Split(dirPath, string(os.PathSeparator))
	return split[len(split)-1]
}
