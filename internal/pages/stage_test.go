package pages

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStagerRecoveryRestoresDisplacedVersion(t *testing.T) {
	root := t.TempDir()
	publicRoot := filepath.Join(root, "public")
	staging := filepath.Join(publicRoot, ".pages", "staging", "bumble")
	oldDir := filepath.Join(staging, "old-deadbeef-report")
	if err := os.MkdirAll(oldDir, 0o700); err != nil {
		t.Fatalf("create displaced dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "index.html"), []byte("old version"), 0o644); err != nil {
		t.Fatalf("write displaced page: %v", err)
	}

	if _, err := NewStager(publicRoot); err != nil {
		t.Fatalf("new stager: %v", err)
	}

	assertPageContent(t, publicRoot, "bumble", "report", "old version")
	assertStagingEmpty(t, publicRoot)
}

func TestStagerRecoveryKeepsTargetAndRemovesOldDisplacedVersion(t *testing.T) {
	root := t.TempDir()
	publicRoot := filepath.Join(root, "public")
	if err := os.MkdirAll(filepath.Join(publicRoot, "bumble", "report"), 0o755); err != nil {
		t.Fatalf("create target: %v", err)
	}
	if err := os.WriteFile(filepath.Join(publicRoot, "bumble", "report", "index.html"), []byte("live version"), 0o644); err != nil {
		t.Fatalf("write live page: %v", err)
	}
	staging := filepath.Join(publicRoot, ".pages", "staging", "bumble")
	if err := os.MkdirAll(filepath.Join(staging, "old-cafe-report"), 0o700); err != nil {
		t.Fatalf("create displaced dir: %v", err)
	}

	if _, err := NewStager(publicRoot); err != nil {
		t.Fatalf("new stager: %v", err)
	}

	assertPageContent(t, publicRoot, "bumble", "report", "live version")
	assertStagingEmpty(t, publicRoot)
}

func TestStagerRecoveryRemovesLeftovers(t *testing.T) {
	root := t.TempDir()
	publicRoot := filepath.Join(root, "public")
	staging := filepath.Join(publicRoot, ".pages", "staging", "bumble")
	if err := os.MkdirAll(filepath.Join(staging, "new-cafe000"), 0o700); err != nil {
		t.Fatalf("create new dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(staging, ".upload-cafe.zip"), []byte("zip"), 0o600); err != nil {
		t.Fatalf("create upload body: %v", err)
	}

	if _, err := NewStager(publicRoot); err != nil {
		t.Fatalf("new stager: %v", err)
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

	if _, err := NewStager(publicRoot); err != nil {
		t.Fatalf("new stager: %v", err)
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
