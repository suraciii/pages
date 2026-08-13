package filesystem

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Memory is an isolated mutable file system for tests and in-process use.
type Memory struct {
	mu      sync.RWMutex
	nodes   map[string]*memoryNode
	locks   map[string]*sync.Mutex
	counter uint64
	workDir string
}

type memoryNode struct {
	data    []byte
	mode    fs.FileMode
	modTime time.Time
}

// NewMemory creates an empty file system with the supplied absolute working
// directory.
func NewMemory(workDir string) *Memory {
	if workDir == "" {
		workDir = memoryRoot()
	} else if !filepath.IsAbs(workDir) {
		workDir = filepath.Join(memoryRoot(), workDir)
	}
	memory := &Memory{
		nodes:   make(map[string]*memoryNode),
		locks:   make(map[string]*sync.Mutex),
		workDir: filepath.Clean(workDir),
	}
	memory.nodes[memory.clean(string(filepath.Separator))] = &memoryNode{mode: fs.ModeDir | 0o755, modTime: time.Unix(0, 0)}
	_ = memory.MkdirAll(memory.workDir, 0o755)
	return memory
}

func memoryRoot() string {
	if filepath.Separator == '\\' {
		return `C:\`
	}
	return string(filepath.Separator)
}

func (memory *Memory) clean(path string) string {
	if path == "" {
		path = "."
	}
	if !filepath.IsAbs(path) {
		if len(path) > 0 && path[0] == byte(filepath.Separator) {
			path = filepath.Join(filepath.VolumeName(memory.workDir), path)
		} else {
			path = filepath.Join(memory.workDir, path)
		}
	}
	return filepath.Clean(path)
}

func (memory *Memory) Open(path string) (File, error) {
	return memory.OpenFile(path, os.O_RDONLY, 0)
}

func (memory *Memory) OpenFile(path string, flag int, mode fs.FileMode) (File, error) {
	path = memory.clean(path)
	memory.mu.Lock()
	defer memory.mu.Unlock()
	node, found := memory.nodes[path]
	if !found {
		if flag&os.O_CREATE == 0 {
			return nil, pathError("open", path, fs.ErrNotExist)
		}
		if err := memory.parentDirectoryLocked(path); err != nil {
			return nil, pathError("open", path, err)
		}
		node = &memoryNode{mode: mode.Perm(), modTime: time.Unix(0, 0)}
		memory.nodes[path] = node
	} else if flag&os.O_CREATE != 0 && flag&os.O_EXCL != 0 {
		return nil, pathError("open", path, fs.ErrExist)
	}
	if node.mode.IsDir() {
		return nil, pathError("open", path, errors.New("is a directory"))
	}
	if flag&os.O_TRUNC != 0 && flag&(os.O_WRONLY|os.O_RDWR) != 0 {
		node.data = nil
	}
	position := int64(0)
	if flag&os.O_APPEND != 0 {
		position = int64(len(node.data))
	}
	return &memoryFile{memory: memory, path: path, flag: flag, position: position}, nil
}

func (memory *Memory) ReadFile(path string) ([]byte, error) {
	path = memory.clean(path)
	memory.mu.RLock()
	defer memory.mu.RUnlock()
	node, found := memory.nodes[path]
	if !found {
		return nil, pathError("read", path, fs.ErrNotExist)
	}
	if node.mode.IsDir() {
		return nil, pathError("read", path, errors.New("is a directory"))
	}
	return append([]byte(nil), node.data...), nil
}

func (memory *Memory) WriteFile(path string, data []byte, mode fs.FileMode) error {
	file, err := memory.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func (memory *Memory) Stat(path string) (fs.FileInfo, error)  { return memory.stat("stat", path) }
func (memory *Memory) Lstat(path string) (fs.FileInfo, error) { return memory.stat("lstat", path) }

func (memory *Memory) stat(operation, path string) (fs.FileInfo, error) {
	path = memory.clean(path)
	memory.mu.RLock()
	defer memory.mu.RUnlock()
	node, found := memory.nodes[path]
	if !found {
		if err := memory.ancestorDirectoryLocked(path); err != nil {
			return nil, pathError(operation, path, err)
		}
		return nil, pathError(operation, path, fs.ErrNotExist)
	}
	return memoryInfo{name: filepath.Base(path), node: cloneNode(node)}, nil
}

func (memory *Memory) MkdirAll(path string, mode fs.FileMode) error {
	path = memory.clean(path)
	memory.mu.Lock()
	defer memory.mu.Unlock()
	return memory.mkdirAllLocked(path, mode)
}

func (memory *Memory) mkdirAllLocked(path string, mode fs.FileMode) error {
	missing := make([]string, 0)
	for current := path; ; current = filepath.Dir(current) {
		if node, found := memory.nodes[current]; found {
			if !node.mode.IsDir() {
				return pathError("mkdir", current, errors.New("not a directory"))
			}
			break
		}
		missing = append(missing, current)
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
	}
	for index := len(missing) - 1; index >= 0; index-- {
		memory.nodes[missing[index]] = &memoryNode{mode: fs.ModeDir | mode.Perm(), modTime: time.Unix(0, 0)}
	}
	return nil
}

func (memory *Memory) MkdirTemp(directory, pattern string) (string, error) {
	memory.mu.Lock()
	defer memory.mu.Unlock()
	if err := memory.parentIsDirectoryLocked(directory); err != nil {
		return "", pathError("mkdirtemp", directory, err)
	}
	path := memory.tempPathLocked(directory, pattern)
	memory.nodes[path] = &memoryNode{mode: fs.ModeDir | 0o700, modTime: time.Unix(0, 0)}
	return path, nil
}

func (memory *Memory) CreateTemp(directory, pattern string) (File, error) {
	memory.mu.Lock()
	if err := memory.parentIsDirectoryLocked(directory); err != nil {
		memory.mu.Unlock()
		return nil, pathError("createtemp", directory, err)
	}
	path := memory.tempPathLocked(directory, pattern)
	memory.nodes[path] = &memoryNode{mode: 0o600, modTime: time.Unix(0, 0)}
	memory.mu.Unlock()
	return &memoryFile{memory: memory, path: path, flag: os.O_RDWR}, nil
}

func (memory *Memory) tempPathLocked(directory, pattern string) string {
	memory.counter++
	name := strings.Replace(pattern, "*", fixedSuffix(memory.counter), 1)
	return memory.clean(filepath.Join(directory, name))
}

func fixedSuffix(value uint64) string {
	const digits = "0123456789abcdef"
	result := make([]byte, 16)
	for index := len(result) - 1; index >= 0; index-- {
		result[index] = digits[value&15]
		value >>= 4
	}
	return string(result)
}

func (memory *Memory) ReadDir(path string) ([]fs.DirEntry, error) {
	path = memory.clean(path)
	memory.mu.RLock()
	defer memory.mu.RUnlock()
	node, found := memory.nodes[path]
	if !found {
		return nil, pathError("readdir", path, fs.ErrNotExist)
	}
	if !node.mode.IsDir() {
		return nil, pathError("readdir", path, errors.New("not a directory"))
	}
	entries := make([]fs.DirEntry, 0)
	for candidate, child := range memory.nodes {
		if candidate != path && filepath.Dir(candidate) == path {
			entries = append(entries, memoryDirEntry{name: filepath.Base(candidate), node: cloneNode(child)})
		}
	}
	sort.Slice(entries, func(left, right int) bool { return entries[left].Name() < entries[right].Name() })
	return entries, nil
}

func (memory *Memory) Rename(oldPath, newPath string) error {
	oldPath, newPath = memory.clean(oldPath), memory.clean(newPath)
	memory.mu.Lock()
	defer memory.mu.Unlock()
	if _, found := memory.nodes[oldPath]; !found {
		return pathError("rename", oldPath, fs.ErrNotExist)
	}
	if err := memory.parentDirectoryLocked(newPath); err != nil {
		return pathError("rename", newPath, err)
	}
	if existing, found := memory.nodes[newPath]; found {
		if existing.mode.IsDir() && memory.hasChildrenLocked(newPath) {
			return pathError("rename", newPath, errors.New("directory not empty"))
		}
		delete(memory.nodes, newPath)
	}
	moved := make(map[string]*memoryNode)
	for path, node := range memory.nodes {
		if path == oldPath || strings.HasPrefix(path, oldPath+string(filepath.Separator)) {
			suffix := strings.TrimPrefix(path, oldPath)
			moved[newPath+suffix] = node
			delete(memory.nodes, path)
		}
	}
	for path, node := range moved {
		memory.nodes[path] = node
	}
	return nil
}

func (memory *Memory) Remove(path string) error {
	path = memory.clean(path)
	memory.mu.Lock()
	defer memory.mu.Unlock()
	node, found := memory.nodes[path]
	if !found {
		return pathError("remove", path, fs.ErrNotExist)
	}
	if node.mode.IsDir() && memory.hasChildrenLocked(path) {
		return pathError("remove", path, errors.New("directory not empty"))
	}
	delete(memory.nodes, path)
	return nil
}

func (memory *Memory) RemoveAll(path string) error {
	path = memory.clean(path)
	memory.mu.Lock()
	defer memory.mu.Unlock()
	for candidate := range memory.nodes {
		if candidate == path || strings.HasPrefix(candidate, path+string(filepath.Separator)) {
			delete(memory.nodes, candidate)
		}
	}
	return nil
}

func (memory *Memory) Abs(path string) (string, error) {
	return memory.clean(path), nil
}

func (memory *Memory) Lock(path string) (func(), error) {
	path = memory.clean(path)
	memory.mu.Lock()
	lock := memory.locks[path]
	if lock == nil {
		lock = &sync.Mutex{}
		memory.locks[path] = lock
	}
	memory.mu.Unlock()
	lock.Lock()
	return lock.Unlock, nil
}

func (memory *Memory) parentDirectoryLocked(path string) error {
	return memory.parentIsDirectoryLocked(filepath.Dir(path))
}

func (memory *Memory) ancestorDirectoryLocked(path string) error {
	for current := filepath.Dir(path); ; current = filepath.Dir(current) {
		if node, found := memory.nodes[current]; found {
			if !node.mode.IsDir() {
				return errors.New("not a directory")
			}
		}
		parent := filepath.Dir(current)
		if parent == current {
			return nil
		}
	}
}

func (memory *Memory) parentIsDirectoryLocked(path string) error {
	node, found := memory.nodes[memory.clean(path)]
	if !found {
		return fs.ErrNotExist
	}
	if !node.mode.IsDir() {
		return errors.New("not a directory")
	}
	return nil
}

func (memory *Memory) hasChildrenLocked(path string) bool {
	prefix := path + string(filepath.Separator)
	for candidate := range memory.nodes {
		if strings.HasPrefix(candidate, prefix) {
			return true
		}
	}
	return false
}

type memoryFile struct {
	memory   *Memory
	path     string
	flag     int
	position int64
	closed   bool
}

func (file *memoryFile) Read(buffer []byte) (int, error) {
	file.memory.mu.RLock()
	defer file.memory.mu.RUnlock()
	if file.closed {
		return 0, fs.ErrClosed
	}
	node, found := file.memory.nodes[file.path]
	if !found {
		return 0, fs.ErrNotExist
	}
	if file.position >= int64(len(node.data)) {
		return 0, io.EOF
	}
	count := copy(buffer, node.data[file.position:])
	file.position += int64(count)
	return count, nil
}

func (file *memoryFile) ReadAt(buffer []byte, offset int64) (int, error) {
	file.memory.mu.RLock()
	defer file.memory.mu.RUnlock()
	if file.closed {
		return 0, fs.ErrClosed
	}
	node, found := file.memory.nodes[file.path]
	if !found {
		return 0, fs.ErrNotExist
	}
	if offset >= int64(len(node.data)) {
		return 0, io.EOF
	}
	count := copy(buffer, node.data[offset:])
	if count < len(buffer) {
		return count, io.EOF
	}
	return count, nil
}

func (file *memoryFile) Write(buffer []byte) (int, error) {
	file.memory.mu.Lock()
	defer file.memory.mu.Unlock()
	if file.closed {
		return 0, fs.ErrClosed
	}
	if file.flag&(os.O_WRONLY|os.O_RDWR) == 0 {
		return 0, fs.ErrPermission
	}
	node, found := file.memory.nodes[file.path]
	if !found {
		return 0, fs.ErrNotExist
	}
	if file.flag&os.O_APPEND != 0 {
		file.position = int64(len(node.data))
	}
	end := int(file.position) + len(buffer)
	if end > len(node.data) {
		node.data = append(node.data, make([]byte, end-len(node.data))...)
	}
	copy(node.data[file.position:], buffer)
	file.position = int64(end)
	return len(buffer), nil
}

func (file *memoryFile) Seek(offset int64, whence int) (int64, error) {
	file.memory.mu.RLock()
	defer file.memory.mu.RUnlock()
	if file.closed {
		return 0, fs.ErrClosed
	}
	node, found := file.memory.nodes[file.path]
	if !found {
		return 0, fs.ErrNotExist
	}
	position := offset
	switch whence {
	case io.SeekCurrent:
		position = file.position + offset
	case io.SeekEnd:
		position = int64(len(node.data)) + offset
	case io.SeekStart:
	default:
		return 0, errors.New("invalid whence")
	}
	if position < 0 {
		return 0, errors.New("negative position")
	}
	file.position = position
	return position, nil
}

func (file *memoryFile) Close() error {
	file.closed = true
	return nil
}

func (file *memoryFile) Stat() (fs.FileInfo, error) { return file.memory.Stat(file.path) }
func (file *memoryFile) Chmod(mode fs.FileMode) error {
	file.memory.mu.Lock()
	defer file.memory.mu.Unlock()
	node, found := file.memory.nodes[file.path]
	if !found {
		return fs.ErrNotExist
	}
	node.mode = node.mode.Type() | mode.Perm()
	return nil
}
func (file *memoryFile) Name() string { return file.path }

type memoryInfo struct {
	name string
	node *memoryNode
}

func (info memoryInfo) Name() string       { return info.name }
func (info memoryInfo) Size() int64        { return int64(len(info.node.data)) }
func (info memoryInfo) Mode() fs.FileMode  { return info.node.mode }
func (info memoryInfo) ModTime() time.Time { return info.node.modTime }
func (info memoryInfo) IsDir() bool        { return info.node.mode.IsDir() }
func (info memoryInfo) Sys() any           { return nil }

type memoryDirEntry struct {
	name string
	node *memoryNode
}

func (entry memoryDirEntry) Name() string      { return entry.name }
func (entry memoryDirEntry) IsDir() bool       { return entry.node.mode.IsDir() }
func (entry memoryDirEntry) Type() fs.FileMode { return entry.node.mode.Type() }
func (entry memoryDirEntry) Info() (fs.FileInfo, error) {
	return memoryInfo{name: entry.name, node: cloneNode(entry.node)}, nil
}

func cloneNode(node *memoryNode) *memoryNode {
	copy := *node
	copy.data = append([]byte(nil), node.data...)
	return &copy
}

func pathError(operation, path string, err error) error {
	return &fs.PathError{Op: operation, Path: path, Err: err}
}
