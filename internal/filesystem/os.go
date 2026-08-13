package filesystem

import (
	"io/fs"
	"os"
	"path/filepath"
)

// OS is the production file-system implementation.
var OS FS = osFS{}

type osFS struct{}

func (osFS) Open(path string) (File, error) { return os.Open(path) }
func (osFS) OpenFile(path string, flag int, mode fs.FileMode) (File, error) {
	return os.OpenFile(path, flag, mode)
}
func (osFS) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }
func (osFS) WriteFile(path string, data []byte, mode fs.FileMode) error {
	return os.WriteFile(path, data, mode)
}
func (osFS) Stat(path string) (fs.FileInfo, error)  { return os.Stat(path) }
func (osFS) Lstat(path string) (fs.FileInfo, error) { return os.Lstat(path) }
func (osFS) MkdirAll(path string, mode fs.FileMode) error {
	return os.MkdirAll(path, mode)
}
func (osFS) MkdirTemp(directory, pattern string) (string, error) {
	return os.MkdirTemp(directory, pattern)
}
func (osFS) CreateTemp(directory, pattern string) (File, error) {
	return os.CreateTemp(directory, pattern)
}
func (osFS) ReadDir(path string) ([]fs.DirEntry, error) { return os.ReadDir(path) }
func (osFS) Rename(oldPath, newPath string) error       { return os.Rename(oldPath, newPath) }
func (osFS) Remove(path string) error                   { return os.Remove(path) }
func (osFS) RemoveAll(path string) error                { return os.RemoveAll(path) }
func (osFS) Abs(path string) (string, error)            { return filepath.Abs(path) }

func (osFS) Lock(path string) (func(), error) {
	return lockOSFile(path)
}
