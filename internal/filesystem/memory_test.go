package filesystem

import (
	"path/filepath"
	"testing"
)

func TestMemoryResolvesRelativePathsAgainstWorkDirectory(t *testing.T) {
	fileSystem := NewMemory("/workspace")
	if err := fileSystem.WriteFile("report.html", []byte("ready"), 0o600); err != nil {
		t.Fatalf("write relative file: %v", err)
	}
	workDir, err := fileSystem.Abs(".")
	if err != nil {
		t.Fatalf("resolve work directory: %v", err)
	}
	wantPath := filepath.Join(workDir, "report.html")
	contents, err := fileSystem.ReadFile(wantPath)
	if err != nil || string(contents) != "ready" {
		t.Fatalf("absolute read = %q, %v", contents, err)
	}
	path, err := fileSystem.Abs("report.html")
	if err != nil || path != wantPath {
		t.Fatalf("absolute path = %q, %v", path, err)
	}
}
