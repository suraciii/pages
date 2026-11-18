package cli

import (
	"path/filepath"
	"testing"

	"github.com/suraciii/pages/internal/filesystem"
)

type evalSymlinkFileSystem struct {
	filesystem.FS
	links map[string]string
}

func (fileSystem evalSymlinkFileSystem) EvalSymlinks(path string) (string, error) {
	if target, found := fileSystem.links[path]; found {
		return target, nil
	}
	return fileSystem.FS.EvalSymlinks(path)
}

func TestValidateListenAddress(t *testing.T) {
	for address, loopback := range map[string]bool{
		"127.0.0.1:3103": true,
		"localhost:80":   true,
		"[::1]:443":      true,
		":443":           false,
		"0.0.0.0:80":     false,
		"example.com:80": false,
	} {
		got, err := validateListenAddress(address)
		if err != nil || got != loopback {
			t.Fatalf("validateListenAddress(%q) = %t, %v; want %t", address, got, err, loopback)
		}
	}
}

func TestPathWithin(t *testing.T) {
	const root = "/srv/pages/public"
	for target, inside := range map[string]bool{
		root:                       true,
		root + "/tokens.json":      true,
		root + "/tls/cert.pem":     true,
		"/srv/pages/publicity":     false,
		"/srv/pages":               false,
		"/srv/pages/public/../key": false,
	} {
		if got := pathWithin(root, filepath.Clean(target)); got != inside {
			t.Fatalf("pathWithin(%q, %q) = %t, want %t", root, target, got, inside)
		}
	}
}

func TestResolveAbsolutePathKeepsMissingSuffix(t *testing.T) {
	fileSystem := filesystem.NewMemory("/workspace")
	got, err := resolveAbsolutePath(fileSystem, "pages-public/missing")
	if err != nil {
		t.Fatalf("resolve path: %v", err)
	}
	if got != "/workspace/pages-public/missing" {
		t.Fatalf("resolved path = %q", got)
	}
}

func TestResolveAbsolutePathResolvesExistingSymlinks(t *testing.T) {
	fileSystem := filesystem.NewMemory("/workspace")
	if err := fileSystem.MkdirAll("/real", 0o755); err != nil {
		t.Fatalf("create real directory: %v", err)
	}
	wrapped := evalSymlinkFileSystem{
		FS:    fileSystem,
		links: map[string]string{"/link": "/real"},
	}
	got, err := resolveAbsolutePath(wrapped, "/link/missing/tokens.json")
	if err != nil {
		t.Fatalf("resolve path: %v", err)
	}
	want := filepath.Join("/real", "missing", "tokens.json")
	if got != want {
		t.Fatalf("resolved path = %q, want %q", got, want)
	}
}
