// Package filesystem defines the file operations used by pages.
package filesystem

import (
	"io"
	"io/fs"
)

// File is an open file used by page, zip, and configuration operations.
type File interface {
	io.Reader
	io.ReaderAt
	io.Writer
	io.Seeker
	io.Closer
	Stat() (fs.FileInfo, error)
	Chmod(fs.FileMode) error
	Name() string
}

// FS is the complete file-system capability used by pages.
type FS interface {
	Open(string) (File, error)
	OpenFile(string, int, fs.FileMode) (File, error)
	ReadFile(string) ([]byte, error)
	WriteFile(string, []byte, fs.FileMode) error
	Stat(string) (fs.FileInfo, error)
	Lstat(string) (fs.FileInfo, error)
	Chmod(string, fs.FileMode) error
	MkdirAll(string, fs.FileMode) error
	MkdirTemp(string, string) (string, error)
	CreateTemp(string, string) (File, error)
	ReadDir(string) ([]fs.DirEntry, error)
	Rename(string, string) error
	Remove(string) error
	RemoveAll(string) error
	Abs(string) (string, error)
	Lock(string) (func(), error)
}
