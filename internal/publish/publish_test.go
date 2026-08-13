package publish

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResolveLocalModeFromEnvironment(t *testing.T) {
	t.Setenv("PAGES_PUBLIC_ROOT", "/srv/pages/public")
	resolved, err := Resolve(Input{File: "page.html", Slug: "report", Identity: "bumble", Timeout: time.Minute})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved.Remote {
		t.Fatalf("resolved = %+v, want local mode", resolved)
	}
	if resolved.LocalTarget != "/srv/pages/public" {
		t.Fatalf("local target = %q", resolved.LocalTarget)
	}
}

func TestResolveLocalModeFromConfigSingleToken(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	config := `{"public-root": "/srv/pages/public", "tokens": {"bumble": "secret"}}`
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	resolved, err := Resolve(Input{File: "page.html", Slug: "report", Timeout: time.Minute, ConfigPath: configPath})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved.Identity != "bumble" {
		t.Fatalf("identity = %q, want bumble", resolved.Identity)
	}
	if resolved.LocalTarget != "/srv/pages/public" {
		t.Fatalf("local target = %q", resolved.LocalTarget)
	}
	if resolved.Token != "bumble.secret" {
		t.Fatalf("token = %q, want bumble.secret", resolved.Token)
	}
}

func TestResolveLocalModeWithoutTargetIsUsageError(t *testing.T) {
	_, err := Resolve(Input{File: "page.html", Slug: "report", Identity: "bumble", Timeout: time.Minute})
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("err = %v, want usage error", err)
	}
}

func TestResolveRemoteModeRequiresToken(t *testing.T) {
	t.Setenv("PAGES_REMOTE", "https://pages.example.com")
	_, err := Resolve(Input{File: "page.html", Slug: "report", Timeout: time.Minute})
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("err = %v, want usage error", err)
	}
}

func TestResolveRemoteTokenFromConfigCombinesSecret(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	config := `{"remote": "https://pages.example.com", "tokens": {"bumble": "secret"}}`
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	resolved, err := Resolve(Input{File: "page.html", Slug: "report", Timeout: time.Minute, ConfigPath: configPath})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !resolved.Remote {
		t.Fatalf("resolved = %+v, want remote mode", resolved)
	}
	if resolved.Identity != "bumble" {
		t.Fatalf("identity = %q, want bumble", resolved.Identity)
	}
	if resolved.Token != "bumble.secret" {
		t.Fatalf("token = %q, want bumble.secret", resolved.Token)
	}
}

func TestResolveNamedConfigMustExist(t *testing.T) {
	_, err := Resolve(Input{
		File:       "page.html",
		Slug:       "report",
		Identity:   "bumble",
		Timeout:    time.Minute,
		ConfigPath: filepath.Join(t.TempDir(), "missing.json"),
	})
	if err == nil || errors.Is(err, ErrUsage) {
		t.Fatalf("err = %v, want config read error", err)
	}
}

func TestResolveEnvironmentTokenOverridesIdentity(t *testing.T) {
	t.Setenv("PAGES_UPLOAD_TOKEN", "bumble.secret")
	t.Setenv("PAGES_PUBLIC_ROOT", "/srv/pages/public")
	resolved, err := Resolve(Input{File: "page.html", Slug: "report", Identity: "fizz", Timeout: time.Minute})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved.Identity != "bumble" {
		t.Fatalf("identity = %q, want bumble from token", resolved.Identity)
	}
	if resolved.Token != "bumble.secret" {
		t.Fatalf("token = %q", resolved.Token)
	}
}

func TestRunLocalPublishesSingleHTML(t *testing.T) {
	root := t.TempDir()
	publicRoot := filepath.Join(root, "public")
	pageFile := filepath.Join(root, "page.html")
	if err := os.WriteFile(pageFile, []byte("<!doctype html><h1>local</h1>"), 0o600); err != nil {
		t.Fatalf("write page: %v", err)
	}

	output, err := Run(&Resolved{File: pageFile, Slug: "report", Identity: "bumble", LocalTarget: publicRoot})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	want := filepath.Join(publicRoot, "bumble", "report") + "/"
	if output != want {
		t.Fatalf("output = %q, want %q", output, want)
	}
	page, err := os.ReadFile(filepath.Join(publicRoot, "bumble", "report", "index.html"))
	if err != nil || string(page) != "<!doctype html><h1>local</h1>" {
		t.Fatalf("page = %q, %v", page, err)
	}
}

func TestRunLocalPublishesZip(t *testing.T) {
	root := t.TempDir()
	publicRoot := filepath.Join(root, "public")
	zipFile := filepath.Join(root, "page.zip")
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range map[string]string{"index.html": "<h1>zip</h1>", "img/chart.png": "png"} {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create entry: %v", err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatalf("write entry: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	if err := os.WriteFile(zipFile, buffer.Bytes(), 0o600); err != nil {
		t.Fatalf("write zip: %v", err)
	}

	output, err := Run(&Resolved{File: zipFile, Slug: "report", Identity: "bumble", LocalTarget: publicRoot})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(publicRoot, "bumble", "report", "img", "chart.png")); err != nil {
		t.Fatalf("extracted asset missing: %v", err)
	}
	if output != filepath.Join(publicRoot, "bumble", "report")+"/" {
		t.Fatalf("output = %q", output)
	}
}

func TestRunLocalRejectsBadZipAndLeavesNoTrace(t *testing.T) {
	root := t.TempDir()
	publicRoot := filepath.Join(root, "public")
	zipFile := filepath.Join(root, "bad.zip")
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	entry, err := writer.Create("report.html")
	if err != nil {
		t.Fatalf("create entry: %v", err)
	}
	if _, err := entry.Write([]byte("no index")); err != nil {
		t.Fatalf("write entry: %v", err)
	}
	writer.Close()
	if err := os.WriteFile(zipFile, buffer.Bytes(), 0o600); err != nil {
		t.Fatalf("write zip: %v", err)
	}

	if _, err := Run(&Resolved{File: zipFile, Slug: "report", Identity: "bumble", LocalTarget: publicRoot}); err == nil {
		t.Fatal("run succeeded, want zip validation error")
	}
	if _, err := os.Stat(filepath.Join(publicRoot, "bumble", "report")); !os.IsNotExist(err) {
		t.Fatalf("page exists after rejected publish: %v", err)
	}
	staging := filepath.Join(publicRoot, ".pages", "staging")
	if _, err := os.Stat(staging); !os.IsNotExist(err) {
		entries, _ := os.ReadDir(staging)
		for _, entry := range entries {
			if entry.Name() == "bumble" {
				inner, _ := os.ReadDir(filepath.Join(staging, "bumble"))
				if len(inner) > 0 {
					t.Fatalf("staging leftovers: %v", strings.Join(names(inner), ", "))
				}
			}
		}
	}
}

func names(entries []os.DirEntry) []string {
	result := make([]string, len(entries))
	for i, entry := range entries {
		result[i] = entry.Name()
	}
	return result
}
