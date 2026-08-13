package pages

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStagerRecoveryRestoresDisplacedVersion(t *testing.T) {
	root := t.TempDir()
	publicRoot := filepath.Join(root, "public")
	staging := filepath.Join(publicRoot, ".pages", "staging", "@bumble")
	oldDir := filepath.Join(staging, "old-deadbeef-report")
	if err := os.MkdirAll(oldDir, 0o700); err != nil {
		t.Fatalf("create displaced dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "index.html"), []byte("old version"), 0o644); err != nil {
		t.Fatalf("write displaced page: %v", err)
	}

	stager, err := NewStager(publicRoot)
	if err != nil {
		t.Fatalf("new stager: %v", err)
	}
	if err := stager.Recover(); err != nil {
		t.Fatalf("recover: %v", err)
	}

	assertPageContent(t, publicRoot, "bumble", "report", "old version")
	assertStagingEmpty(t, publicRoot)
}

func TestStagerRecoveryKeepsTargetAndRemovesOldDisplacedVersion(t *testing.T) {
	root := t.TempDir()
	publicRoot := filepath.Join(root, "public")
	if err := os.MkdirAll(filepath.Join(publicRoot, "@bumble", "report"), 0o755); err != nil {
		t.Fatalf("create target: %v", err)
	}
	if err := os.WriteFile(filepath.Join(publicRoot, "@bumble", "report", "index.html"), []byte("live version"), 0o644); err != nil {
		t.Fatalf("write live page: %v", err)
	}
	staging := filepath.Join(publicRoot, ".pages", "staging", "@bumble")
	if err := os.MkdirAll(filepath.Join(staging, "old-cafe-report"), 0o700); err != nil {
		t.Fatalf("create displaced dir: %v", err)
	}

	stager, err := NewStager(publicRoot)
	if err != nil {
		t.Fatalf("new stager: %v", err)
	}
	if err := stager.Recover(); err != nil {
		t.Fatalf("recover: %v", err)
	}

	assertPageContent(t, publicRoot, "bumble", "report", "live version")
	assertStagingEmpty(t, publicRoot)
}

func TestNewStagerLeavesActiveStagingAndRecoveryRemovesLeftovers(t *testing.T) {
	root := t.TempDir()
	publicRoot := filepath.Join(root, "public")
	staging := filepath.Join(publicRoot, ".pages", "staging", "@bumble")
	if err := os.MkdirAll(filepath.Join(staging, "new-cafe000"), 0o700); err != nil {
		t.Fatalf("create new dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(staging, ".upload-cafe.zip"), []byte("zip"), 0o600); err != nil {
		t.Fatalf("create upload body: %v", err)
	}

	stager, err := NewStager(publicRoot)
	if err != nil {
		t.Fatalf("new stager: %v", err)
	}
	if _, err := os.Stat(filepath.Join(staging, "new-cafe000")); err != nil {
		t.Fatalf("active staging removed by constructor: %v", err)
	}
	if err := stager.Recover(); err != nil {
		t.Fatalf("recover: %v", err)
	}
	assertStagingEmpty(t, publicRoot)
}

func TestStagerRecoveryRemovesInvalidIdentityDir(t *testing.T) {
	root := t.TempDir()
	publicRoot := filepath.Join(root, "public")
	staging := filepath.Join(publicRoot, ".pages", "staging")
	if err := os.MkdirAll(filepath.Join(staging, "not a valid name"), 0o700); err != nil {
		t.Fatalf("create invalid identity dir: %v", err)
	}

	stager, err := NewStager(publicRoot)
	if err != nil {
		t.Fatalf("new stager: %v", err)
	}
	if err := stager.Recover(); err != nil {
		t.Fatalf("recover: %v", err)
	}
	assertStagingEmpty(t, publicRoot)
}

func TestStagerSwapHTMLFirstPublishAndReplace(t *testing.T) {
	root := t.TempDir()
	publicRoot := filepath.Join(root, "public")
	stager, err := NewStager(publicRoot)
	if err != nil {
		t.Fatalf("new stager: %v", err)
	}

	for _, content := range []string{"first", "second"} {
		staged, err := stager.StageDir("bumble")
		if err != nil {
			t.Fatalf("stage: %v", err)
		}
		if err := os.WriteFile(filepath.Join(staged, "index.html"), []byte(content), 0o644); err != nil {
			t.Fatalf("write staged page: %v", err)
		}
		if err := stager.SwapHTML("bumble", "hello", staged); err != nil {
			t.Fatalf("swap: %v", err)
		}
	}
	assertPageContent(t, publicRoot, "bumble", "hello", "second")
	assertStagingEmpty(t, publicRoot)
}

func TestStagerDefaultAndNamedPagesDoNotOverlap(t *testing.T) {
	publicRoot := filepath.Join(t.TempDir(), "public")
	stager, err := NewStager(publicRoot)
	if err != nil {
		t.Fatalf("new stager: %v", err)
	}

	for identity, content := range map[string]string{"": "default", "bumble": "named"} {
		staged, err := stager.StageDir(identity)
		if err != nil {
			t.Fatalf("stage %q: %v", identity, err)
		}
		if err := os.WriteFile(filepath.Join(staged, "index.html"), []byte(content), 0o644); err != nil {
			t.Fatalf("write staged page: %v", err)
		}
		if err := stager.SwapHTML(identity, "report", staged); err != nil {
			t.Fatalf("swap %q: %v", identity, err)
		}
	}

	assertPageContent(t, publicRoot, "", "report", "default")
	assertPageContent(t, publicRoot, "bumble", "report", "named")
}

func TestStagerRestoresOldPageWhenInstallFails(t *testing.T) {
	publicRoot := filepath.Join(t.TempDir(), "public")
	stager, err := NewStager(publicRoot)
	if err != nil {
		t.Fatalf("new stager: %v", err)
	}
	publishStagedHTML(t, stager, "bumble", "report", "old page")
	staged := stageHTML(t, stager, "bumble", "new page")

	renameCalls := 0
	stager.rename = func(source, target string) error {
		renameCalls++
		if renameCalls == 2 {
			return errors.New("install failed")
		}
		return os.Rename(source, target)
	}
	if err := stager.SwapHTML("bumble", "report", staged); err == nil || !strings.Contains(err.Error(), "install failed") {
		t.Fatalf("swap error = %v", err)
	}
	assertPageContent(t, publicRoot, "bumble", "report", "old page")
	if _, err := os.Stat(filepath.Join(staged, "index.html")); err != nil {
		t.Fatalf("new staged page missing after failed install: %v", err)
	}
}

func TestStagerReportsInstallAndRestoreFailures(t *testing.T) {
	publicRoot := filepath.Join(t.TempDir(), "public")
	stager, err := NewStager(publicRoot)
	if err != nil {
		t.Fatalf("new stager: %v", err)
	}
	publishStagedHTML(t, stager, "bumble", "report", "old page")
	staged := stageHTML(t, stager, "bumble", "new page")

	renameCalls := 0
	stager.rename = func(source, target string) error {
		renameCalls++
		switch renameCalls {
		case 2:
			return errors.New("install failed")
		case 3:
			return errors.New("restore failed")
		default:
			return os.Rename(source, target)
		}
	}
	err = stager.SwapHTML("bumble", "report", staged)
	if err == nil || !strings.Contains(err.Error(), "install failed") || !strings.Contains(err.Error(), "restore failed") {
		t.Fatalf("swap error = %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(publicRoot, ".pages", "staging", "@bumble", "old-*-report", "index.html"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("displaced old page matches = %v, %v", matches, err)
	}
	content, err := os.ReadFile(matches[0])
	if err != nil || string(content) != "old page" {
		t.Fatalf("displaced old page = %q, %v", content, err)
	}
}

func stageHTML(t *testing.T, stager *Stager, identity, content string) string {
	t.Helper()
	staged, err := stager.StageDir(identity)
	if err != nil {
		t.Fatalf("stage: %v", err)
	}
	if err := os.WriteFile(filepath.Join(staged, "index.html"), []byte(content), 0o644); err != nil {
		t.Fatalf("write staged page: %v", err)
	}
	return staged
}

func publishStagedHTML(t *testing.T, stager *Stager, identity, slug, content string) {
	t.Helper()
	if err := stager.SwapHTML(identity, slug, stageHTML(t, stager, identity, content)); err != nil {
		t.Fatalf("publish staged page: %v", err)
	}
}

func TestStagerRecoveryRestoresDefaultVersion(t *testing.T) {
	publicRoot := filepath.Join(t.TempDir(), "public")
	oldDir := filepath.Join(publicRoot, ".pages", "staging", "default", "old-cafe-report")
	if err := os.MkdirAll(oldDir, 0o700); err != nil {
		t.Fatalf("create displaced dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "index.html"), []byte("default old"), 0o644); err != nil {
		t.Fatalf("write displaced page: %v", err)
	}

	stager, err := NewStager(publicRoot)
	if err != nil {
		t.Fatalf("new stager: %v", err)
	}
	if err := stager.Recover(); err != nil {
		t.Fatalf("recover: %v", err)
	}
	assertPageContent(t, publicRoot, "", "report", "default old")
}

func TestStagerRecoveryPreservesOldVersionWhenTargetInspectionFails(t *testing.T) {
	publicRoot := filepath.Join(t.TempDir(), "public")
	oldDir := filepath.Join(publicRoot, ".pages", "staging", "@bumble", "old-cafe-report")
	if err := os.MkdirAll(oldDir, 0o700); err != nil {
		t.Fatalf("create displaced dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "index.html"), []byte("old version"), 0o644); err != nil {
		t.Fatalf("write displaced page: %v", err)
	}
	if err := os.WriteFile(filepath.Join(publicRoot, "@bumble"), []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("write invalid identity path: %v", err)
	}

	stager, err := NewStager(publicRoot)
	if err != nil {
		t.Fatalf("new stager: %v", err)
	}
	if err := stager.Recover(); err == nil || !strings.Contains(err.Error(), "inspect recovery target") {
		t.Fatalf("recover error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(oldDir, "index.html")); err != nil {
		t.Fatalf("old version removed after inspection failure: %v", err)
	}
}
