package io

import (
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

// Create copies a file from a source path to a target path
func (o *MediaFileSystem) Copy(source, target string) (int64, error) {
	sourceFile, err := o.Open(source)
	if err != nil {
		return 0, err
	}
	defer sourceFile.Close()

	targetFile, err := o.Create(target)
	if err != nil {
		return 0, err
	}
	defer targetFile.Close()

	return io.Copy(targetFile, sourceFile)
}

// IsSameFileSystem checks if a source and target are on the same file system
func (o *MediaFileSystem) IsSameFileSystem(source, target string) (bool, error) {
	var sourcePathSyscall, targetPathSyscall syscall.Stat_t
	if err := syscall.Stat(source, &sourcePathSyscall); err != nil {
		return false, fmt.Errorf("failed to stat download path: %v", err)
	}

	if err := syscall.Stat(target, &targetPathSyscall); err != nil {
		return false, fmt.Errorf("failed to stat destination path: %v", err)
	}

	return sourcePathSyscall.Dev == targetPathSyscall.Dev, nil
}

// WalkDir is a wrapper around fs.WalkDir
func (o *MediaFileSystem) WalkDir(fsys fs.FS, root string, fn fs.WalkDirFunc) error {
	return fs.WalkDir(fsys, root, fn)
}
