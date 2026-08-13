package pages

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Stager owns the public root: it maintains the .pages staging area,
// swaps pages into place, and recovers interrupted swaps at startup.
type Stager struct {
	publicRoot string
	stagingDir string

	lockMu sync.Mutex
	locks  map[string]*sync.Mutex
}

// NewStager creates the public root and its staging area and recovers
// interrupted swaps.
func NewStager(publicRoot string) (*Stager, error) {
	if publicRoot == "" {
		return nil, errors.New("public root is required")
	}
	stagingDir := filepath.Join(publicRoot, ".pages", "staging")
	if err := os.MkdirAll(stagingDir, 0o700); err != nil {
		return nil, fmt.Errorf("create public root: %w", err)
	}
	stager := &Stager{
		publicRoot: publicRoot,
		stagingDir: stagingDir,
		locks:      make(map[string]*sync.Mutex),
	}
	if err := stager.Recover(); err != nil {
		return nil, err
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
	newDir, err := os.MkdirTemp(identityDir, "new-*")
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
	file, err := os.CreateTemp(identityDir, ".upload-*.zip")
	if err != nil {
		return "", fmt.Errorf("create upload body: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close upload body: %w", err)
	}
	return file.Name(), nil
}

// SwapHTML atomically replaces the index.html of a single-file page.
func (stager *Stager) SwapHTML(identity, slug, stagedDir string) error {
	lock := stager.lock(identity, slug)
	lock.Lock()
	defer lock.Unlock()

	targetDir := filepath.Join(stager.publicRoot, identity, slug)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("create page directory: %w", err)
	}
	if err := os.Rename(filepath.Join(stagedDir, "index.html"), filepath.Join(targetDir, "index.html")); err != nil {
		return fmt.Errorf("atomically replace page: %w", err)
	}
	_ = os.RemoveAll(stagedDir)
	return nil
}

// SwapZip replaces a directory page with the two-rename sequence.
func (stager *Stager) SwapZip(identity, slug, stagedDir string) error {
	lock := stager.lock(identity, slug)
	lock.Lock()
	defer lock.Unlock()

	targetDir := filepath.Join(stager.publicRoot, identity, slug)
	if err := os.MkdirAll(filepath.Dir(targetDir), 0o755); err != nil {
		return fmt.Errorf("create identity directory: %w", err)
	}

	displaced := ""
	if _, err := os.Lstat(targetDir); err == nil {
		displaced = filepath.Join(filepath.Dir(stagedDir), "old-"+randomHex()+"-"+slug)
		if err := os.Rename(targetDir, displaced); err != nil {
			return fmt.Errorf("displace old page: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect existing page: %w", err)
	}

	if err := os.Rename(stagedDir, targetDir); err != nil {
		if displaced != "" {
			_ = os.Rename(displaced, targetDir)
		}
		return fmt.Errorf("swap page into place: %w", err)
	}
	if displaced != "" {
		_ = os.RemoveAll(displaced)
	}
	return nil
}

// Recover restores interrupted swaps: an old version whose target is
// missing moves back; every other staging leftover is removed.
func (stager *Stager) Recover() error {
	identities, err := os.ReadDir(stager.stagingDir)
	if err != nil {
		return fmt.Errorf("scan staging area: %w", err)
	}
	for _, identityEntry := range identities {
		identityDir := filepath.Join(stager.stagingDir, identityEntry.Name())
		if !ValidName(identityEntry.Name()) {
			_ = os.RemoveAll(identityDir)
			continue
		}
		entries, err := os.ReadDir(identityDir)
		if err != nil {
			return fmt.Errorf("scan staging identity: %w", err)
		}
		for _, entry := range entries {
			name := entry.Name()
			path := filepath.Join(identityDir, name)
			if !strings.HasPrefix(name, "old-") {
				_ = os.RemoveAll(path)
				continue
			}
			_, slug, found := strings.Cut(strings.TrimPrefix(name, "old-"), "-")
			if !found || !ValidName(slug) {
				_ = os.RemoveAll(path)
				continue
			}
			target := filepath.Join(stager.publicRoot, identityEntry.Name(), slug)
			if _, err := os.Lstat(target); os.IsNotExist(err) {
				if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
					return fmt.Errorf("restore interrupted swap: %w", err)
				}
				if err := os.Rename(path, target); err != nil {
					return fmt.Errorf("restore interrupted swap: %w", err)
				}
			} else {
				_ = os.RemoveAll(path)
			}
		}
		if remaining, err := os.ReadDir(identityDir); err == nil && len(remaining) == 0 {
			_ = os.Remove(identityDir)
		}
	}
	return nil
}

func (stager *Stager) identityDir(identity string) (string, error) {
	identityDir := filepath.Join(stager.stagingDir, identity)
	if err := os.MkdirAll(identityDir, 0o700); err != nil {
		return "", fmt.Errorf("create staging directory: %w", err)
	}
	return identityDir, nil
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
