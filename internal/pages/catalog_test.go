package pages

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/suraciii/pages/internal/filesystem"
)

func TestRefreshIndexesListsPagesAndIdentities(t *testing.T) {
	stager, fileSystem, publicRoot := newMemoryStager(t)
	publishStagedHTML(t, stager, "", "status", "status")
	publishStagedHTML(t, stager, "", "report", "report")
	publishStagedHTML(t, stager, "bumble", "overview", "overview")

	if err := RefreshIndexes(publicRoot, fileSystem); err != nil {
		t.Fatalf("refresh indexes: %v", err)
	}

	root := readCatalog(t, fileSystem, filepath.Join(publicRoot, "index.html"))
	assertContainsInOrder(t, root, `href="./report/"`, `href="./status/"`, `href="./@bumble/"`)
	if strings.Contains(root, ".pages") || strings.Contains(root, "overview") {
		t.Fatalf("root index leaked internal or nested entries: %s", root)
	}

	identity := readCatalog(t, fileSystem, filepath.Join(publicRoot, "@bumble", "index.html"))
	if !strings.Contains(identity, `href="../"`) || !strings.Contains(identity, `href="./overview/"`) {
		t.Fatalf("identity index links = %s", identity)
	}
}

func TestRefreshIndexesRendersEmptyScope(t *testing.T) {
	_, fileSystem, publicRoot := newMemoryStager(t)
	if err := RefreshIndexes(publicRoot, fileSystem); err != nil {
		t.Fatalf("refresh indexes: %v", err)
	}
	root := readCatalog(t, fileSystem, filepath.Join(publicRoot, "index.html"))
	if !strings.Contains(root, "No Pages published yet.") {
		t.Fatalf("empty root index = %s", root)
	}
}

func TestRefreshIndexesIgnoresInvalidAndIncompleteEntries(t *testing.T) {
	_, fileSystem, publicRoot := newMemoryStager(t)
	mustWriteMemoryFile(t, fileSystem, filepath.Join(publicRoot, "not-a-page", "asset.txt"), "asset")
	mustWriteMemoryFile(t, fileSystem, filepath.Join(publicRoot, "@Bad", "report", "index.html"), "bad identity")
	if err := fileSystem.MkdirAll(filepath.Join(publicRoot, ".hidden"), 0o755); err != nil {
		t.Fatalf("create hidden entry: %v", err)
	}

	if err := RefreshIndexes(publicRoot, fileSystem); err != nil {
		t.Fatalf("refresh indexes: %v", err)
	}
	root := readCatalog(t, fileSystem, filepath.Join(publicRoot, "index.html"))
	if strings.Contains(root, "not-a-page") || strings.Contains(root, "@Bad") || strings.Contains(root, ".hidden") {
		t.Fatalf("root index listed invalid entries: %s", root)
	}
}

func TestRefreshIndexesRecoversInterruptedReplacement(t *testing.T) {
	_, fileSystem, publicRoot := newMemoryStager(t)
	staging := filepath.Join(publicRoot, ".pages", catalogStagingName)
	if err := fileSystem.MkdirAll(staging, 0o700); err != nil {
		t.Fatalf("create catalog staging: %v", err)
	}
	mustWriteMemoryFile(t, fileSystem, filepath.Join(staging, "old-deadbeefdeadbeef-root"), "old index")

	if err := RefreshIndexes(publicRoot, fileSystem); err != nil {
		t.Fatalf("refresh indexes: %v", err)
	}
	if _, err := fileSystem.Stat(filepath.Join(staging, "old-deadbeefdeadbeef-root")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("old catalog remains: %v", err)
	}
	if _, err := fileSystem.Stat(filepath.Join(publicRoot, "index.html")); err != nil {
		t.Fatalf("root index missing after recovery: %v", err)
	}
}

func TestNewServerRefreshesExistingPages(t *testing.T) {
	fileSystem := filesystem.NewMemory("/workspace")
	publicRoot := "/public"
	mustWriteMemoryFile(t, fileSystem, filepath.Join(publicRoot, "report", "index.html"), "page")
	if _, err := NewServer(ServerConfig{
		PublicRoot:     publicRoot,
		Tokens:         Tokens{Token: "secret"},
		MaxUploadBytes: 1024,
		FileSystem:     fileSystem,
	}); err != nil {
		t.Fatalf("new server: %v", err)
	}
	root := readCatalog(t, fileSystem, filepath.Join(publicRoot, "index.html"))
	if !strings.Contains(root, `href="./report/"`) {
		t.Fatalf("root index = %s", root)
	}
}

func TestServerRefreshesPageIndexAfterUpload(t *testing.T) {
	server, publicRoot := newTestServer(t, Tokens{Token: "secret"}, 1024)
	if response := publishRequest(t, server, "report", "secret", []byte("page")); response.Code != 204 {
		t.Fatalf("status = %d, want 204", response.Code)
	}
	root := readCatalog(t, server.fileSystem, filepath.Join(publicRoot, "index.html"))
	if !strings.Contains(root, `href="./report/"`) {
		t.Fatalf("root index = %s", root)
	}
}

func readCatalog(t *testing.T, fileSystem filesystem.FS, path string) string {
	t.Helper()
	contents, err := fileSystem.ReadFile(path)
	if err != nil {
		t.Fatalf("read catalog %q: %v", path, err)
	}
	return string(contents)
}

func assertContainsInOrder(t *testing.T, value string, fragments ...string) {
	t.Helper()
	position := -1
	for _, fragment := range fragments {
		next := strings.Index(value, fragment)
		if next <= position {
			t.Fatalf("fragment %q is not after position %d in %s", fragment, position, value)
		}
		position = next
	}
}
