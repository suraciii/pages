package pages

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/suraciii/pages/internal/filesystem"
)

func TestRefreshIndexesListsPagesAndIdentities(t *testing.T) {
	stager, fileSystem, publicRoot := newMemoryStager(t)
	publishStagedHTML(t, stager, "", "status", "status")
	publishStagedHTML(t, stager, "", "report", "report")
	publishStagedHTML(t, stager, "zeta", "overview", "overview")
	publishStagedHTML(t, stager, "default", "overview", "overview")
	publishStagedHTML(t, stager, "bumble", "overview", "overview")

	if err := RefreshIndexes(publicRoot, fileSystem); err != nil {
		t.Fatalf("refresh indexes: %v", err)
	}

	root := readCatalog(t, fileSystem, filepath.Join(publicRoot, "index.html"))
	assertContainsInOrder(t, root, `<nav aria-label="Identity navigation">`, `href="./">Default Identity</a>`, `href="./@bumble/"`, `href="./@default/"`, `href="./@zeta/"`,
		"<h2>Default Identity</h2>", `href="./report/"`, `href="./status/"`)
	if strings.Contains(root, `href="./default/"`) {
		t.Fatalf("default identity added a URL scope: %s", root)
	}
	if strings.Contains(root, ".pages") || strings.Contains(root, "overview") {
		t.Fatalf("root index leaked internal or nested entries: %s", root)
	}

	identity := readCatalog(t, fileSystem, filepath.Join(publicRoot, "@bumble", "index.html"))
	assertContainsInOrder(t, identity,
		`<nav aria-label="Identity navigation">`,
		`href="../">Default Identity</a>`,
		`href="./">@bumble/</a>`,
		`href="../@default/">@default/</a>`,
		`href="../@zeta/">@zeta/</a>`,
		`<nav aria-label="Breadcrumb">`,
		`href="../">Pages</a>`,
		`<h2>Pages</h2>`,
		`href="./overview/"`)
}

func TestRefreshIndexesShowsPublishedTimeNextToEachPage(t *testing.T) {
	stager, memory, publicRoot := newMemoryStager(t)
	publishStagedHTML(t, stager, "", "alpha", "alpha")
	publishStagedHTML(t, stager, "bumble", "overview", "overview")
	zone := time.FixedZone("UTC+8", 8*60*60)
	alphaTime := time.Date(2026, 10, 16, 21, 2, 13, 0, zone)
	overviewTime := time.Date(2026, 10, 15, 9, 41, 0, 0, zone)
	fileSystem := catalogTimeFS{
		FS: memory,
		times: map[string]time.Time{
			filepath.Join(publicRoot, "alpha", "index.html"):               alphaTime,
			filepath.Join(publicRoot, "@bumble", "overview", "index.html"): overviewTime,
		},
	}

	if err := RefreshIndexes(publicRoot, fileSystem); err != nil {
		t.Fatalf("refresh indexes: %v", err)
	}
	root := readCatalog(t, fileSystem, filepath.Join(publicRoot, "index.html"))
	assertContainsInOrder(t, root,
		`href="./alpha/">alpha/</a>`,
		`datetime="2026-10-16T21:02:13+08:00"`,
		`2026-10-16 21:02</time>`)
	identity := readCatalog(t, fileSystem, filepath.Join(publicRoot, "@bumble", "index.html"))
	assertContainsInOrder(t, identity,
		`href="./overview/">overview/</a>`,
		`datetime="2026-10-15T09:41:00+08:00"`,
		`2026-10-15 09:41</time>`)
}

func TestRefreshIndexesWritesPageRecordWithSameEntriesAsIndex(t *testing.T) {
	stager, memory, publicRoot := newMemoryStager(t)
	publishStagedHTML(t, stager, "", "alpha", "alpha")
	publishStagedHTML(t, stager, "bumble", "overview", "overview")
	zone := time.FixedZone("UTC+8", 8*60*60)
	fileSystem := catalogTimeFS{
		FS: memory,
		times: map[string]time.Time{
			filepath.Join(publicRoot, "alpha", "index.html"):               time.Date(2026, 10, 16, 21, 2, 13, 0, zone),
			filepath.Join(publicRoot, "@bumble", "overview", "index.html"): time.Date(2026, 10, 15, 9, 41, 0, 0, zone),
		},
	}

	if err := RefreshIndexes(publicRoot, fileSystem); err != nil {
		t.Fatalf("refresh indexes: %v", err)
	}
	rootRecord := readCatalog(t, fileSystem, filepath.Join(publicRoot, "index.json"))
	wantRoot := "{\n  \"pages\": [\n    {\"slug\": \"alpha\", \"published\": \"2026-10-16T21:02:13+08:00\"}\n  ]\n}\n"
	if rootRecord != wantRoot {
		t.Fatalf("root record = %q, want %q", rootRecord, wantRoot)
	}
	identityRecord := readCatalog(t, fileSystem, filepath.Join(publicRoot, "@bumble", "index.json"))
	wantIdentity := "{\n  \"pages\": [\n    {\"slug\": \"overview\", \"published\": \"2026-10-15T09:41:00+08:00\"}\n  ]\n}\n"
	if identityRecord != wantIdentity {
		t.Fatalf("identity record = %q, want %q", identityRecord, wantIdentity)
	}
}

func TestRefreshIndexesRecordOrderMatchesIndexOrder(t *testing.T) {
	stager, memory, publicRoot := newMemoryStager(t)
	for _, slug := range []string{"alpha", "zeta", "beta"} {
		publishStagedHTML(t, stager, "", slug, slug)
	}
	scopeDir := publicRoot
	fileSystem := catalogTimeFS{
		FS: memory,
		times: map[string]time.Time{
			filepath.Join(scopeDir, "alpha", "index.html"): time.Unix(100, 0),
			filepath.Join(scopeDir, "beta", "index.html"):  time.Unix(200, 0),
			filepath.Join(scopeDir, "zeta", "index.html"):  time.Unix(200, 0),
		},
	}
	if err := RefreshIndexes(publicRoot, fileSystem); err != nil {
		t.Fatalf("refresh indexes: %v", err)
	}
	record := readCatalog(t, fileSystem, filepath.Join(publicRoot, "index.json"))
	index := readCatalog(t, fileSystem, filepath.Join(publicRoot, "index.html"))
	assertContainsInOrder(t, record, `"slug": "beta"`, `"slug": "zeta"`, `"slug": "alpha"`)
	assertContainsInOrder(t, index, `href="./beta/"`, `href="./zeta/"`, `href="./alpha/"`)
}

func TestRefreshIndexesRecordForEmptyScope(t *testing.T) {
	_, fileSystem, publicRoot := newMemoryStager(t)
	if err := RefreshIndexes(publicRoot, fileSystem); err != nil {
		t.Fatalf("refresh indexes: %v", err)
	}
	record := readCatalog(t, fileSystem, filepath.Join(publicRoot, "index.json"))
	want := "{\n  \"pages\": []\n}\n"
	if record != want {
		t.Fatalf("empty record = %q, want %q", record, want)
	}
}

func TestRefreshIndexesRefusesExistingNonPageRecord(t *testing.T) {
	stager, fileSystem, publicRoot := newMemoryStager(t)
	publishStagedHTML(t, stager, "", "report", "report")
	if err := RefreshIndexes(publicRoot, fileSystem); err != nil {
		t.Fatalf("initial refresh: %v", err)
	}
	indexBefore := readCatalog(t, fileSystem, filepath.Join(publicRoot, "index.html"))
	mustWriteMemoryFile(t, fileSystem, filepath.Join(publicRoot, "index.json"), "manual data")

	err := RefreshIndexes(publicRoot, fileSystem)
	if err == nil || !strings.Contains(err.Error(), "move the existing file first") {
		t.Fatalf("refresh error = %v, want refusal", err)
	}
	if indexAfter := readCatalog(t, fileSystem, filepath.Join(publicRoot, "index.html")); indexAfter != indexBefore {
		t.Fatalf("index changed before refusal")
	}
	if recordAfter := readCatalog(t, fileSystem, filepath.Join(publicRoot, "index.json")); recordAfter != "manual data" {
		t.Fatalf("record changed before refusal: %q", recordAfter)
	}
}

// A foreign `index.json` that carries a `pages` key is not a Page Record and
// must be preserved, not replaced.
func TestRefreshIndexesRefusesForeignRecordWithPagesKey(t *testing.T) {
	for name, content := range map[string]string{
		"pages key":     `{"pages":[{"url":"/","title":"Home"}]}`,
		"nested pages":  `{"data":{"pages":["x"]}}`,
		"pages in text": `{"title":"pages"}`,
		"wrong prefix":  `{"identities":"","pages":[]}`,
	} {
		t.Run(name, func(t *testing.T) {
			stager, fileSystem, publicRoot := newMemoryStager(t)
			publishStagedHTML(t, stager, "", "report", "report")
			if err := RefreshIndexes(publicRoot, fileSystem); err != nil {
				t.Fatalf("initial refresh: %v", err)
			}
			if err := fileSystem.Remove(filepath.Join(publicRoot, "index.json")); err != nil {
				t.Fatalf("remove record: %v", err)
			}
			mustWriteMemoryFile(t, fileSystem, filepath.Join(publicRoot, "index.json"), content)

			err := RefreshIndexes(publicRoot, fileSystem)
			if err == nil || !strings.Contains(err.Error(), "move the existing file first") {
				t.Fatalf("refresh error = %v, want refusal", err)
			}
			if recordAfter := readCatalog(t, fileSystem, filepath.Join(publicRoot, "index.json")); recordAfter != content {
				t.Fatalf("record changed before refusal: %q", recordAfter)
			}
		})
	}
}

// A foreign `index.json` that embeds the pages key inside a nested object
// is not a Page Record and must be preserved: a Record starts with the
// marker, it does not merely contain it.
func TestRefreshIndexesRefusesRecordMarkerInsideNestedJSON(t *testing.T) {
	for name, content := range map[string]string{
		"nested object":  "{\"export\":{\n  \"pages\": [\"a\"]}}",
		"pages in value": "{\"note\":\"see {\\n  \\\"pages\\\": [\"}",
		"compact form":   `{"pages":[]}`,
		"leading blank":  "\n{\n  \"pages\": [\n  ]\n}\n",
	} {
		t.Run(name, func(t *testing.T) {
			stager, fileSystem, publicRoot := newMemoryStager(t)
			publishStagedHTML(t, stager, "", "report", "report")
			if err := RefreshIndexes(publicRoot, fileSystem); err != nil {
				t.Fatalf("initial refresh: %v", err)
			}
			if err := fileSystem.Remove(filepath.Join(publicRoot, "index.json")); err != nil {
				t.Fatalf("remove record: %v", err)
			}
			mustWriteMemoryFile(t, fileSystem, filepath.Join(publicRoot, "index.json"), content)

			err := RefreshIndexes(publicRoot, fileSystem)
			if err == nil || !strings.Contains(err.Error(), "move the existing file first") {
				t.Fatalf("refresh error = %v, want refusal", err)
			}
			if recordAfter := readCatalog(t, fileSystem, filepath.Join(publicRoot, "index.json")); recordAfter != content {
				t.Fatalf("record changed before refusal: %q", recordAfter)
			}
		})
	}
}

func TestRefreshIndexesRecoversInterruptedRecordReplacement(t *testing.T) {
	_, fileSystem, publicRoot := newMemoryStager(t)
	staging := filepath.Join(publicRoot, ".pages", catalogStagingName)
	if err := fileSystem.MkdirAll(staging, 0o700); err != nil {
		t.Fatalf("create catalog staging: %v", err)
	}
	mustWriteMemoryFile(t, fileSystem, filepath.Join(staging, "old-deadbeefdeadbeef-root-record"), "{\n  \"pages\": []\n}\n")

	if err := RefreshIndexes(publicRoot, fileSystem); err != nil {
		t.Fatalf("refresh indexes: %v", err)
	}
	if _, err := fileSystem.Stat(filepath.Join(staging, "old-deadbeefdeadbeef-root-record")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("old record remains: %v", err)
	}
	if _, err := fileSystem.Stat(filepath.Join(publicRoot, "index.json")); err != nil {
		t.Fatalf("record missing after recovery: %v", err)
	}
}

// A parked Record backup for one Identity must restore that Identity's
// Record. A key scheme that reads the Identity name from the wrong position
// would create a fake Identity scope instead.
func TestRefreshIndexesRecordRecoveryTargetsNamedIdentity(t *testing.T) {
	stager, fileSystem, publicRoot := newMemoryStager(t)
	publishStagedHTML(t, stager, "bumble", "overview", "overview")
	if err := RefreshIndexes(publicRoot, fileSystem); err != nil {
		t.Fatalf("initial refresh: %v", err)
	}
	staging := filepath.Join(publicRoot, ".pages", catalogStagingName)
	if err := fileSystem.MkdirAll(staging, 0o700); err != nil {
		t.Fatalf("create catalog staging: %v", err)
	}
	parked := "{\n  \"pages\": [\n    {\"slug\": \"parked\", \"published\": \"2026-10-01T10:00:00+08:00\"}\n  ]\n}\n"
	mustWriteMemoryFile(t, fileSystem, filepath.Join(staging, "old-deadbeefdeadbeef-record-identity-bumble"), parked)
	if err := fileSystem.Remove(filepath.Join(publicRoot, "@bumble", "index.json")); err != nil {
		t.Fatalf("remove identity record: %v", err)
	}

	if err := RefreshIndexes(publicRoot, fileSystem); err != nil {
		t.Fatalf("refresh indexes: %v", err)
	}
	if _, err := fileSystem.Stat(filepath.Join(publicRoot, "@record-bumble")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("recovery created a fake identity scope: %v", err)
	}
	if _, err := fileSystem.Stat(filepath.Join(publicRoot, "@bumble", "index.json")); err != nil {
		t.Fatalf("identity record missing after recovery: %v", err)
	}
	if _, err := fileSystem.Stat(filepath.Join(publicRoot, "@bumble", "index.html")); err != nil {
		t.Fatalf("identity index missing after recovery: %v", err)
	}
}

func TestRefreshIndexesOrdersPagesByPublishedFileTime(t *testing.T) {
	for _, scope := range []struct {
		name     string
		identity string
	}{
		{name: "default"},
		{name: "named", identity: "bumble"},
	} {
		t.Run(scope.name, func(t *testing.T) {
			stager, memory, publicRoot := newMemoryStager(t)
			for _, slug := range []string{"alpha", "zeta", "beta"} {
				publishStagedHTML(t, stager, scope.identity, slug, slug)
			}
			scopeDir := filepath.Join(publicRoot, IdentityScope(scope.identity))
			alphaPath := filepath.Join(scopeDir, "alpha", "index.html")
			fileSystem := catalogTimeFS{
				FS: memory,
				times: map[string]time.Time{
					alphaPath: time.Unix(100, 0),
					filepath.Join(scopeDir, "beta", "index.html"): time.Unix(200, 0),
					filepath.Join(scopeDir, "zeta", "index.html"): time.Unix(200, 0),
				},
			}
			if err := RefreshIndexes(publicRoot, fileSystem); err != nil {
				t.Fatalf("refresh indexes: %v", err)
			}
			indexPath := filepath.Join(scopeDir, "index.html")
			index := readCatalog(t, fileSystem, indexPath)
			assertContainsInOrder(t, index, `href="./beta/"`, `href="./zeta/"`, `href="./alpha/"`)

			if err := RefreshIndexes(publicRoot, fileSystem); err != nil {
				t.Fatalf("refresh unchanged pages: %v", err)
			}
			if refreshed := readCatalog(t, fileSystem, indexPath); refreshed != index {
				t.Fatalf("refresh changed index for unchanged pages: before=%q after=%q", index, refreshed)
			}

			publishStagedHTML(t, stager, scope.identity, "alpha", "updated alpha")
			fileSystem.times[alphaPath] = time.Unix(300, 0)
			if err := RefreshIndexes(publicRoot, fileSystem); err != nil {
				t.Fatalf("refresh updated page: %v", err)
			}
			updated := readCatalog(t, fileSystem, indexPath)
			assertContainsInOrder(t, updated, `href="./alpha/"`, `href="./beta/"`, `href="./zeta/"`)
		})
	}
}

func TestRefreshIndexesRendersEmptyScope(t *testing.T) {
	_, fileSystem, publicRoot := newMemoryStager(t)
	if err := RefreshIndexes(publicRoot, fileSystem); err != nil {
		t.Fatalf("refresh indexes: %v", err)
	}
	root := readCatalog(t, fileSystem, filepath.Join(publicRoot, "index.html"))
	if !strings.Contains(root, "<h2>Default Identity</h2>") || !strings.Contains(root, "No Pages published yet.") {
		t.Fatalf("empty root index = %s", root)
	}
	if strings.Contains(root, `aria-label="Identity navigation"`) {
		t.Fatalf("empty root index has redundant identity navigation: %s", root)
	}
}

func TestRefreshIndexesRefusesExistingNonPageIndex(t *testing.T) {
	_, fileSystem, publicRoot := newMemoryStager(t)
	manualIndex := "<!doctype html><h1>existing home</h1>"
	mustWriteMemoryFile(t, fileSystem, filepath.Join(publicRoot, "index.html"), manualIndex)

	err := RefreshIndexes(publicRoot, fileSystem)
	if err == nil || !strings.Contains(err.Error(), "move the existing file first") {
		t.Fatalf("refresh error = %v, want refusal", err)
	}
	contents, readErr := fileSystem.ReadFile(filepath.Join(publicRoot, "index.html"))
	if readErr != nil || string(contents) != manualIndex {
		t.Fatalf("existing index = %q, %v", contents, readErr)
	}
}

func TestRefreshIndexesChecksAllTargetsBeforeReplacingAny(t *testing.T) {
	stager, fileSystem, publicRoot := newMemoryStager(t)
	publishStagedHTML(t, stager, "", "report", "report")
	publishStagedHTML(t, stager, "bumble", "overview", "overview")
	if err := RefreshIndexes(publicRoot, fileSystem); err != nil {
		t.Fatalf("initial refresh: %v", err)
	}
	rootBefore := readCatalog(t, fileSystem, filepath.Join(publicRoot, "index.html"))
	mustWriteMemoryFile(t, fileSystem, filepath.Join(publicRoot, "@bumble", "index.html"), "manual identity home")

	err := RefreshIndexes(publicRoot, fileSystem)
	if err == nil || !strings.Contains(err.Error(), "move the existing file first") {
		t.Fatalf("refresh error = %v, want refusal", err)
	}
	rootAfter := readCatalog(t, fileSystem, filepath.Join(publicRoot, "index.html"))
	if rootAfter != rootBefore {
		t.Fatalf("root index changed before refusal: before=%q after=%q", rootBefore, rootAfter)
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
	mustWriteMemoryFile(t, fileSystem, filepath.Join(staging, "old-deadbeefdeadbeef-root"), catalogMarker+"\nold index")

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

// A refresh whose output is byte-identical to the artifact on disk writes
// nothing: no rename reaches the target, so the artifact keeps its inode and
// a reader never sees a needless rewrite.
func TestRefreshIndexesSkipsByteIdenticalArtifacts(t *testing.T) {
	stager, fileSystem, publicRoot := newMemoryStager(t)
	publishStagedHTML(t, stager, "", "report", "report")
	if err := RefreshIndexes(publicRoot, fileSystem); err != nil {
		t.Fatalf("initial refresh: %v", err)
	}
	counting := &renameCountingFS{FS: fileSystem}
	if err := RefreshIndexes(publicRoot, counting); err != nil {
		t.Fatalf("second refresh: %v", err)
	}
	if counting.renames != 0 {
		t.Fatalf("second refresh performed %d renames, want 0", counting.renames)
	}
	entries, err := fileSystem.ReadDir(filepath.Join(publicRoot, ".pages", catalogStagingName))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("read catalog staging: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("staging holds %d entries after an unchanged refresh", len(entries))
	}
}

// A refresh that changes the Page set still replaces the artifact.
func TestRefreshIndexesReplacesChangedArtifact(t *testing.T) {
	stager, fileSystem, publicRoot := newMemoryStager(t)
	publishStagedHTML(t, stager, "", "report", "report")
	if err := RefreshIndexes(publicRoot, fileSystem); err != nil {
		t.Fatalf("initial refresh: %v", err)
	}
	counting := &renameCountingFS{FS: fileSystem}
	publishStagedHTML(t, stager, "", "later", "later")
	if err := RefreshIndexes(publicRoot, counting); err != nil {
		t.Fatalf("second refresh: %v", err)
	}
	if counting.renames == 0 {
		t.Fatal("second refresh replaced nothing, want a rename")
	}
	if index := readCatalog(t, fileSystem, filepath.Join(publicRoot, "index.html")); !strings.Contains(index, `href="./later/"`) {
		t.Fatalf("index does not list the new page: %s", index)
	}
}

type renameCountingFS struct {
	filesystem.FS
	renames int
}

func (fileSystem *renameCountingFS) Rename(oldPath, newPath string) error {
	fileSystem.renames++
	return fileSystem.FS.Rename(oldPath, newPath)
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

type catalogTimeFS struct {
	filesystem.FS
	times map[string]time.Time
}

func (fileSystem catalogTimeFS) Stat(path string) (fs.FileInfo, error) {
	info, err := fileSystem.FS.Stat(path)
	if err != nil {
		return nil, err
	}
	if modTime, found := fileSystem.times[path]; found {
		return catalogTimeInfo{FileInfo: info, modTime: modTime}, nil
	}
	return info, nil
}

type catalogTimeInfo struct {
	fs.FileInfo
	modTime time.Time
}

func (info catalogTimeInfo) ModTime() time.Time { return info.modTime }

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
