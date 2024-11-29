package io

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"syscall"
)

var (
	_ FileIO = (*MediaFileSystem)(nil)
)

// MediaFilesystem is the default implementation of file io using the os package
type MediaFileSystem struct{}

// Stat is a wrapper around os.Stat
func (o *MediaFileSystem) Stat(target string) (os.FileInfo, error) {
	return os.Stat(target)
}

// Rename is a wrapper around os.Rename
func (o *MediaFileSystem) Rename(source, target string) error {
	return os.Rename(source, target)
}

// Open is a wrapper around os.Open
func (o *MediaFileSystem) Open(name string) (*os.File, error) {
	return os.Open(name)
}

// Create is a wrapper around os.Create
func (o *MediaFileSystem) Create(name string) (io.WriteCloser, error) {
	return os.Create(name)
}

// Create copies a file from a source path to a target path. The target file must not exist yet.
func (o *MediaFileSystem) Copy(source, target string) (int64, error) {
	sourceFile, err := o.Open(source)
	if err != nil {
		return 0, err
	}
	defer sourceFile.Close()

	_, err = o.Stat(target)
	if err == nil {
		return 0, errors.New("target file already exists")
	}

	if !errors.Is(err, os.ErrNotExist) {
		return 0, fmt.Errorf("unexpected error: %v", err)
	}

	targetFile, err := o.Create(target)
	if err != nil {
		return 0, err
	}
	defer targetFile.Close()

	return io.Copy(targetFile, sourceFile)
}

// IsSameFileSystem checks if a source and target are on the same file system.
func (o *MediaFileSystem) IsSameFileSystem(source, target string) (bool, error) {
	// Get source file stat
	sourceStat, err := os.Stat(source)
	if err != nil {
		return false, fmt.Errorf("failed to stat source path: %w", err)
	}

	sourceSys, ok := sourceStat.Sys().(*syscall.Stat_t)
	if !ok {
		return false, errors.New("source path: unexpected sys type")
	}

	// Get target file stat
	targetStat, err := os.Stat(target)
	if err != nil {
		return false, fmt.Errorf("failed to stat target path: %w", err)
	}

	targetSys, ok := targetStat.Sys().(*syscall.Stat_t)
	if !ok {
		return false, errors.New("target path: unexpected sys type")
	}

	// Compare device IDs
	return sourceSys.Dev == targetSys.Dev, nil
}

// WalkDir is a wrapper around fs.WalkDir
func (o *MediaFileSystem) WalkDir(fsys fs.FS, root string, fn fs.WalkDirFunc) error {
	return fs.WalkDir(fsys, root, fn)
}
