package pages

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"

	"github.com/suraciii/pages/internal/filesystem"
)

// Stager owns the public root: it maintains the .pages staging area,
// swaps pages into place, and recovers interrupted swaps at startup.
type Stager struct {
	publicRoot string
	stagingDir string
	fs         filesystem.FS
	rename     func(string, string) error

	lockMu sync.Mutex
	locks  map[string]*sync.Mutex
}

// NewStager creates the public root and its staging area.
func NewStager(publicRoot string) (*Stager, error) {
	return NewStagerWithFS(publicRoot, filesystem.OS)
}

// NewStagerWithFS creates a Stager with an explicit file-system capability.
func NewStagerWithFS(publicRoot string, fileSystem filesystem.FS) (*Stager, error) {
	if publicRoot == "" {
		return nil, errors.New("public root is required")
	}
	stagingDir := filepath.Join(publicRoot, ".pages", "staging")
	if err := fileSystem.MkdirAll(stagingDir, 0o700); err != nil {
		return nil, fmt.Errorf("create public root: %w", err)
	}
	stager := &Stager{
		publicRoot: publicRoot,
		stagingDir: stagingDir,
		fs:         fileSystem,
		rename:     fileSystem.Rename,
		locks:      make(map[string]*sync.Mutex),
	}
	return stager, nil
}

// StageDir creates a fresh staging directory for one identity. The caller
// removes it after a failed publish.
func (stager *Stager) StageDir(identity string) (string, error) {
	identityDir, err := stager.identityDir(identity)
	if err != nil {
		return "", err
	}
	newDir, err := stager.fs.MkdirTemp(identityDir, "new-*")
	if err != nil {
		return "", fmt.Errorf("create staging page: %w", err)
	}
	return newDir, nil
}

// UploadFile reserves a private file for a pending zip body. The caller
// removes it after the upload.
func (stager *Stager) UploadFile(identity string) (string, error) {
	identityDir, err := stager.identityDir(identity)
	if err != nil {
		return "", err
	}
	file, err := stager.fs.CreateTemp(identityDir, ".upload-*.zip")
	if err != nil {
		return "", fmt.Errorf("create upload body: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close upload body: %w", err)
	}
	return file.Name(), nil
}

// SwapHTML replaces the complete old Page, including assets from a prior zip.
func (stager *Stager) SwapHTML(identity, slug, stagedDir string) error {
	return stager.swapPage(identity, slug, stagedDir)
}

// SwapZip replaces a directory page with the two-rename sequence.
func (stager *Stager) SwapZip(identity, slug, stagedDir string) error {
	return stager.swapPage(identity, slug, stagedDir)
}

func (stager *Stager) swapPage(identity, slug, stagedDir string) error {
	lock := stager.lock(identity, slug)
	lock.Lock()
	defer lock.Unlock()

	targetDir := stager.pageDir(identity, slug)
	if err := stager.fs.MkdirAll(filepath.Dir(targetDir), 0o755); err != nil {
		return fmt.Errorf("create identity directory: %w", err)
	}

	displaced := ""
	if _, err := stager.fs.Lstat(targetDir); err == nil {
		displaced = filepath.Join(filepath.Dir(stagedDir), "old-"+randomHex()+"-"+slug)
		if err := stager.rename(targetDir, displaced); err != nil {
			return fmt.Errorf("displace old page: %w", err)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("inspect existing page: %w", err)
	}

	if err := stager.rename(stagedDir, targetDir); err != nil {
		if displaced != "" {
			if restoreErr := stager.rename(displaced, targetDir); restoreErr != nil {
				return fmt.Errorf("swap page into place: %w; restore old page: %v", err, restoreErr)
			}
		}
		return fmt.Errorf("swap page into place: %w", err)
	}
	if displaced != "" {
		_ = stager.fs.RemoveAll(displaced)
	}
	return nil
}

// Recover restores interrupted swaps: an old version whose target is
// missing moves back; every other staging leftover is removed.
func (stager *Stager) Recover() error {
	scopes, err := stager.fs.ReadDir(stager.stagingDir)
	if err != nil {
		return fmt.Errorf("scan staging area: %w", err)
	}
	for _, scopeEntry := range scopes {
		scopeDir := filepath.Join(stager.stagingDir, scopeEntry.Name())
		identity, valid := identityFromScope(scopeEntry.Name())
		if !valid {
			_ = stager.fs.RemoveAll(scopeDir)
			continue
		}
		entries, err := stager.fs.ReadDir(scopeDir)
		if err != nil {
			return fmt.Errorf("scan staging scope: %w", err)
		}
		for _, entry := range entries {
			name := entry.Name()
			path := filepath.Join(scopeDir, name)
			if !strings.HasPrefix(name, "old-") {
				_ = stager.fs.RemoveAll(path)
				continue
			}
			_, slug, found := strings.Cut(strings.TrimPrefix(name, "old-"), "-")
			if !found || !ValidName(slug) {
				_ = stager.fs.RemoveAll(path)
				continue
			}
			target := stager.pageDir(identity, slug)
			if _, err := stager.fs.Lstat(target); errors.Is(err, fs.ErrNotExist) {
				if err := stager.fs.MkdirAll(filepath.Dir(target), 0o755); err != nil {
					return fmt.Errorf("restore interrupted swap: %w", err)
				}
				if err := stager.fs.Rename(path, target); err != nil {
					return fmt.Errorf("restore interrupted swap: %w", err)
				}
			} else if err == nil {
				_ = stager.fs.RemoveAll(path)
			} else {
				return fmt.Errorf("inspect recovery target: %w", err)
			}
		}
		if remaining, err := stager.fs.ReadDir(scopeDir); err == nil && len(remaining) == 0 {
			_ = stager.fs.Remove(scopeDir)
		}
	}
	return nil
}

func (stager *Stager) identityDir(identity string) (string, error) {
	identityDir := filepath.Join(stager.stagingDir, stagingScope(identity))
	if err := stager.fs.MkdirAll(identityDir, 0o700); err != nil {
		return "", fmt.Errorf("create staging directory: %w", err)
	}
	return identityDir, nil
}

func (stager *Stager) pageDir(identity, slug string) string {
	if scope := IdentityScope(identity); scope != "" {
		return filepath.Join(stager.publicRoot, scope, slug)
	}
	return filepath.Join(stager.publicRoot, slug)
}

func stagingScope(identity string) string {
	if identity == "" {
		return "default"
	}
	return IdentityScope(identity)
}

func identityFromScope(scope string) (string, bool) {
	if scope == "default" {
		return "", true
	}
	if !strings.HasPrefix(scope, "@") {
		return "", false
	}
	identity := strings.TrimPrefix(scope, "@")
	return identity, ValidName(identity)
}

func (stager *Stager) lock(identity, slug string) *sync.Mutex {
	key := identity + "/" + slug
	stager.lockMu.Lock()
	defer stager.lockMu.Unlock()
	lock, found := stager.locks[key]
	if !found {
		lock = &sync.Mutex{}
		stager.locks[key] = lock
	}
	return lock
}

// randomHex returns 8 random bytes as hex, so the value never contains
// hyphens and stays parseable inside directory names.
func randomHex() string {
	random := make([]byte, 8)
	if _, err := rand.Read(random); err != nil {
		panic(fmt.Sprintf("generate random suffix: %v", err))
	}
	return hex.EncodeToString(random)
}
