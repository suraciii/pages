package pages

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/suraciii/pages/internal/filesystem"
)

func newMemoryStager(t *testing.T) (*Stager, *filesystem.Memory, string) {
	t.Helper()
	fileSystem := filesystem.NewMemory("/workspace")
	publicRoot := "/public"
	stager, err := NewStager(publicRoot, fileSystem)
	if err != nil {
		t.Fatalf("new stager: %v", err)
	}
	return stager, fileSystem, publicRoot
}

func TestNewStagerMakesPublicRootReadableAndStagingPrivate(t *testing.T) {
	fileSystem := filesystem.NewMemory("/workspace")
	publicRoot := "/public"
	if err := fileSystem.MkdirAll(publicRoot, 0o700); err != nil {
		t.Fatalf("create existing public root: %v", err)
	}
	if _, err := NewStager(publicRoot, fileSystem); err != nil {
		t.Fatalf("new stager: %v", err)
	}

	assertMode(t, fileSystem, publicRoot, 0o755)
	assertMode(t, fileSystem, filepath.Join(publicRoot, ".pages"), 0o700)
	assertMode(t, fileSystem, filepath.Join(publicRoot, ".pages", "staging"), 0o700)
}

func TestStagerRecoveryRestoresDisplacedVersion(t *testing.T) {
	stager, fileSystem, publicRoot := newMemoryStager(t)
	oldDir := filepath.Join(publicRoot, ".pages", "staging", "@bumble", "old-deadbeef-report")
	mustWriteMemoryFile(t, fileSystem, filepath.Join(oldDir, "index.html"), "old version")

	if err := stager.Recover(); err != nil {
		t.Fatalf("recover: %v", err)
	}
	assertPageContent(t, fileSystem, publicRoot, "bumble", "report", "old version")
	assertStagingEmpty(t, fileSystem, publicRoot)
}

func TestStagerRecoveryKeepsTargetAndRemovesOldDisplacedVersion(t *testing.T) {
	stager, fileSystem, publicRoot := newMemoryStager(t)
	mustWriteMemoryFile(t, fileSystem, filepath.Join(publicRoot, "@bumble", "report", "index.html"), "live version")
	if err := fileSystem.MkdirAll(filepath.Join(publicRoot, ".pages", "staging", "@bumble", "old-cafe-report"), 0o700); err != nil {
		t.Fatalf("create displaced dir: %v", err)
	}

	if err := stager.Recover(); err != nil {
		t.Fatalf("recover: %v", err)
	}
	assertPageContent(t, fileSystem, publicRoot, "bumble", "report", "live version")
	assertStagingEmpty(t, fileSystem, publicRoot)
}

func TestNewStagerLeavesActiveStagingAndRecoveryRemovesLeftovers(t *testing.T) {
	stager, fileSystem, publicRoot := newMemoryStager(t)
	staging := filepath.Join(publicRoot, ".pages", "staging", "@bumble")
	if err := fileSystem.MkdirAll(filepath.Join(staging, "new-cafe000"), 0o700); err != nil {
		t.Fatalf("create new dir: %v", err)
	}
	mustWriteMemoryFile(t, fileSystem, filepath.Join(staging, ".upload-cafe.zip"), "zip")

	if _, err := fileSystem.Stat(filepath.Join(staging, "new-cafe000")); err != nil {
		t.Fatalf("active staging removed by constructor: %v", err)
	}
	if err := stager.Recover(); err != nil {
		t.Fatalf("recover: %v", err)
	}
	assertStagingEmpty(t, fileSystem, publicRoot)
}

func TestStagerRecoveryRemovesInvalidIdentityDir(t *testing.T) {
	stager, fileSystem, publicRoot := newMemoryStager(t)
	if err := fileSystem.MkdirAll(filepath.Join(publicRoot, ".pages", "staging", "not a valid name"), 0o700); err != nil {
		t.Fatalf("create invalid identity dir: %v", err)
	}
	if err := stager.Recover(); err != nil {
		t.Fatalf("recover: %v", err)
	}
	assertStagingEmpty(t, fileSystem, publicRoot)
}

func TestStagerSwapHTMLFirstPublishAndReplace(t *testing.T) {
	stager, fileSystem, publicRoot := newMemoryStager(t)
	if err := fileSystem.MkdirAll(filepath.Join(publicRoot, "@bumble"), 0o700); err != nil {
		t.Fatalf("create existing identity directory: %v", err)
	}
	for _, content := range []string{"first", "second"} {
		if err := stager.SwapHTML("bumble", "hello", stageHTML(t, stager, content)); err != nil {
			t.Fatalf("swap: %v", err)
		}
	}
	assertPageContent(t, fileSystem, publicRoot, "bumble", "hello", "second")
	assertMode(t, fileSystem, filepath.Join(publicRoot, "@bumble"), 0o755)
	assertMode(t, fileSystem, filepath.Join(publicRoot, "@bumble", "hello"), 0o755)
	assertStagingEmpty(t, fileSystem, publicRoot)
}

func TestStagerDefaultAndNamedPagesDoNotOverlap(t *testing.T) {
	stager, fileSystem, publicRoot := newMemoryStager(t)
	for identity, content := range map[string]string{"": "default", "bumble": "named"} {
		staged := stageHTMLForIdentity(t, stager, identity, content)
		if err := stager.SwapHTML(identity, "report", staged); err != nil {
			t.Fatalf("swap %q: %v", identity, err)
		}
	}
	assertPageContent(t, fileSystem, publicRoot, "", "report", "default")
	assertPageContent(t, fileSystem, publicRoot, "bumble", "report", "named")
}

func TestStagerRestoresOldPageWhenInstallFails(t *testing.T) {
	stager, fileSystem, publicRoot := newMemoryStager(t)
	publishStagedHTML(t, stager, "bumble", "report", "old page")
	staged := stageHTMLForIdentity(t, stager, "bumble", "new page")
	originalRename := stager.rename
	renameCalls := 0
	stager.rename = func(source, target string) error {
		renameCalls++
		if renameCalls == 2 {
			return errors.New("install failed")
		}
		return originalRename(source, target)
	}
	if err := stager.SwapHTML("bumble", "report", staged); err == nil || !strings.Contains(err.Error(), "install failed") {
		t.Fatalf("swap error = %v", err)
	}
	assertPageContent(t, fileSystem, publicRoot, "bumble", "report", "old page")
	if _, err := fileSystem.Stat(filepath.Join(staged, "index.html")); err != nil {
		t.Fatalf("new staged page missing after failed install: %v", err)
	}
}

func TestStagerReportsInstallAndRestoreFailures(t *testing.T) {
	stager, fileSystem, publicRoot := newMemoryStager(t)
	publishStagedHTML(t, stager, "bumble", "report", "old page")
	staged := stageHTMLForIdentity(t, stager, "bumble", "new page")
	originalRename := stager.rename
	renameCalls := 0
	stager.rename = func(source, target string) error {
		renameCalls++
		switch renameCalls {
		case 2:
			return errors.New("install failed")
		case 3:
			return errors.New("restore failed")
		default:
			return originalRename(source, target)
		}
	}
	err := stager.SwapHTML("bumble", "report", staged)
	if err == nil || !strings.Contains(err.Error(), "install failed") || !strings.Contains(err.Error(), "restore failed") {
		t.Fatalf("swap error = %v", err)
	}
	entries, err := fileSystem.ReadDir(filepath.Join(publicRoot, ".pages", "staging", "@bumble"))
	if err != nil || len(entries) != 2 {
		t.Fatalf("staging entries = %v, %v", entries, err)
	}
	foundOld := false
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "old-") {
			content, readErr := fileSystem.ReadFile(filepath.Join(publicRoot, ".pages", "staging", "@bumble", entry.Name(), "index.html"))
			if readErr != nil || string(content) != "old page" {
				t.Fatalf("displaced old page = %q, %v", content, readErr)
			}
			foundOld = true
		}
	}
	if !foundOld {
		t.Fatal("displaced old page not found")
	}
}

func TestStagerRecoveryRestoresDefaultVersion(t *testing.T) {
	stager, fileSystem, publicRoot := newMemoryStager(t)
	oldDir := filepath.Join(publicRoot, ".pages", "staging", "default", "old-cafe-report")
	mustWriteMemoryFile(t, fileSystem, filepath.Join(oldDir, "index.html"), "default old")
	if err := stager.Recover(); err != nil {
		t.Fatalf("recover: %v", err)
	}
	assertPageContent(t, fileSystem, publicRoot, "", "report", "default old")
}

func TestStagerRecoveryPreservesOldVersionWhenTargetInspectionFails(t *testing.T) {
	stager, fileSystem, publicRoot := newMemoryStager(t)
	oldDir := filepath.Join(publicRoot, ".pages", "staging", "@bumble", "old-cafe-report")
	mustWriteMemoryFile(t, fileSystem, filepath.Join(oldDir, "index.html"), "old version")
	mustWriteMemoryFile(t, fileSystem, filepath.Join(publicRoot, "@bumble"), "not a directory")

	if err := stager.Recover(); err == nil || !strings.Contains(err.Error(), "inspect recovery target") {
		t.Fatalf("recover error = %v", err)
	}
	if _, err := fileSystem.Stat(filepath.Join(oldDir, "index.html")); err != nil {
		t.Fatalf("old version removed after inspection failure: %v", err)
	}
}

func stageHTML(t *testing.T, stager *Stager, content string) string {
	return stageHTMLForIdentity(t, stager, "bumble", content)
}

func stageHTMLForIdentity(t *testing.T, stager *Stager, identity, content string) string {
	t.Helper()
	staged, err := stager.StageDir(identity)
	if err != nil {
		t.Fatalf("stage: %v", err)
	}
	mustWriteMemoryFile(t, stager.fs, filepath.Join(staged, "index.html"), content)
	return staged
}

func publishStagedHTML(t *testing.T, stager *Stager, identity, slug, content string) {
	t.Helper()
	if err := stager.SwapHTML(identity, slug, stageHTMLForIdentity(t, stager, identity, content)); err != nil {
		t.Fatalf("publish staged page: %v", err)
	}
}

func mustWriteMemoryFile(t *testing.T, fileSystem filesystem.FS, path, content string) {
	t.Helper()
	if err := fileSystem.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create parent: %v", err)
	}
	if err := fileSystem.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func assertMode(t *testing.T, fileSystem filesystem.FS, path string, want fs.FileMode) {
	t.Helper()
	info, err := fileSystem.Stat(path)
	if err != nil {
		t.Fatalf("stat %q: %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("mode %q = %o, want %o", path, got, want)
	}
}

func assertNotExist(t *testing.T, fileSystem filesystem.FS, path string) {
	t.Helper()
	if _, err := fileSystem.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("path %q exists or returned unexpected error: %v", path, err)
	}
}
