package library

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type EpisodeFile struct {
	Name         string
	Size         int64
	RelativePath string
	AbsolutePath string

	SeriesName   string
	SeasonNumber int
}

var seasonDirRe = regexp.MustCompile(`(?i)^season\s*(\d+)`)

func EpisodeFileFromPath(path string) EpisodeFile {
	name := sanitizeName(filepath.Base(path))
	series := sanitizeName(dirName(filepath.Dir(path)))
	season := 0
	parent := dirName(path)
	if m := seasonDirRe.FindStringSubmatch(parent); len(m) == 2 {
		if n, err := strconv.Atoi(strings.TrimLeft(m[1], "0")); err == nil {
			season = n
		}
	}
	return EpisodeFile{
		Name:         name,
		RelativePath: path,
		SeriesName:   series,
		SeasonNumber: season,
	}
}
