package pages

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/suraciii/pages/internal/filesystem"
)

const maxZipEntries = 512

// errInvalidZip marks a validation failure; callers report it as a bad
// request without touching the existing page.
var errInvalidZip = errors.New("invalid zip")

// StageZip validates and extracts an archive through fileSystem. The
// uncompressed total must not exceed four times the upload limit.
func StageZip(fileSystem filesystem.FS, zipPath, newDir string, uploadLimit int64) error {
	file, err := fileSystem.Open(zipPath)
	if err != nil {
		return fmt.Errorf("%w: %v", errInvalidZip, err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("%w: %v", errInvalidZip, err)
	}
	archive, err := zip.NewReader(file, info.Size())
	if err != nil {
		return fmt.Errorf("%w: %v", errInvalidZip, err)
	}

	if len(archive.File) > maxZipEntries {
		return fmt.Errorf("%w: more than %d entries", errInvalidZip, maxZipEntries)
	}

	rootIndex := false
	seen := make(map[string]bool, len(archive.File))
	var uncompressed uint64
	limit := uint64(uploadLimit) * 4
	for _, entry := range archive.File {
		name := entry.Name
		mode := entry.Mode()
		canonicalName, err := canonicalZipName(name, mode.IsDir())
		if err != nil {
			return fmt.Errorf("%w: %v", errInvalidZip, err)
		}
		duplicateKey := strings.ToLower(canonicalName)
		if seen[duplicateKey] {
			return fmt.Errorf("%w: duplicate entry %q", errInvalidZip, name)
		}
		seen[duplicateKey] = true

		if mode.IsDir() {
			continue
		}
		if mode&os.ModeSymlink != 0 || mode&os.ModeType != 0 {
			return fmt.Errorf("%w: unsupported entry type %q", errInvalidZip, name)
		}
		if canonicalName == "index.html" {
			rootIndex = true
		}
		uncompressed += entry.UncompressedSize64
		if uncompressed > limit {
			return fmt.Errorf("%w: uncompressed total exceeds four times the upload limit", errInvalidZip)
		}
	}
	if !rootIndex {
		return fmt.Errorf("%w: index.html at the archive root is required", errInvalidZip)
	}

	for _, entry := range archive.File {
		if entry.Mode().IsDir() {
			continue
		}
		destination := filepath.Join(newDir, filepath.FromSlash(entry.Name))
		if err := fileSystem.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return fmt.Errorf("create zip entry directory: %w", err)
		}
		source, err := entry.Open()
		if err != nil {
			return fmt.Errorf("open zip entry: %w", err)
		}
		target, err := fileSystem.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			source.Close()
			return fmt.Errorf("create zip entry: %w", err)
		}
		_, copyError := io.Copy(target, source)
		source.Close()
		closeError := target.Close()
		if copyError != nil {
			return fmt.Errorf("extract zip entry: %w", copyError)
		}
		if closeError != nil {
			return fmt.Errorf("close zip entry: %w", closeError)
		}
	}
	return nil
}

func canonicalZipName(name string, isDir bool) (string, error) {
	if !utf8.ValidString(name) {
		return "", fmt.Errorf("entry name is not UTF-8")
	}
	if name == "" {
		return "", errors.New("empty entry name")
	}
	if strings.HasPrefix(name, "/") {
		return "", fmt.Errorf("absolute entry name %q", name)
	}
	if strings.HasPrefix(name, ".") {
		return "", fmt.Errorf("entry name %q starts with a dot", name)
	}
	if strings.Contains(name, "\\") {
		return "", fmt.Errorf("entry name %q contains a backslash", name)
	}
	if strings.HasSuffix(name, "/") && !isDir {
		return "", fmt.Errorf("entry name %q is a file with a trailing slash", name)
	}
	canonicalName := strings.TrimSuffix(name, "/")
	if canonicalName == "" {
		return "", errors.New("entry name is only a slash")
	}
	if path.Clean(canonicalName) != canonicalName {
		return "", fmt.Errorf("entry name %q is not canonical", name)
	}
	return canonicalName, nil
}
