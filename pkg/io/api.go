package io

import (
	"io"
	"io/fs"
	"os"
)

// FileIO is an interface for file io operations
type FileIO interface {
	Stat(target string) (os.FileInfo, error)
	Create(name string) (io.WriteCloser, error)
	IsSameFileSystem(source, target string) (bool, error)
	Open(name string) (*os.File, error)
	Rename(source, target string) error
	WalkDir(fsys fs.FS, root string, fn fs.WalkDirFunc) error
	Copy(source, target string) (int64, error)
	MkdirAll(name string, perm os.FileMode) error
}
