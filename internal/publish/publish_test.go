package publish

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/suraciii/pages/internal/pages"
)

func TestResolveLocalModeDefaultsIdentityAndIgnoresUploadToken(t *testing.T) {
	t.Setenv("PAGES_PUBLIC_ROOT", filepath.Join(t.TempDir(), "public"))
	t.Setenv("PAGES_UPLOAD_TOKEN", "bumble.secret")
	resolved, err := Resolve(Input{File: "page.html", Slug: "report", Timeout: time.Minute})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved.Remote || resolved.Identity != "" || resolved.Token != "" {
		t.Fatalf("resolved = %+v, want local Default Identity", resolved)
	}
}

func TestResolveLocalNamedIdentity(t *testing.T) {
	t.Setenv("PAGES_PUBLIC_ROOT", filepath.Join(t.TempDir(), "public"))
	resolved, err := Resolve(Input{File: "page.html", Slug: "report", Identity: "bumble", Timeout: time.Minute})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved.Identity != "bumble" {
		t.Fatalf("identity = %q", resolved.Identity)
	}
}

func TestResolveLocalModeDefaultsToCurrentDirectory(t *testing.T) {
	t.Setenv("PAGES_PUBLIC_ROOT", "")
	t.Setenv("PAGES_REMOTE", "")
	resolved, err := Resolve(Input{File: "page.html", Slug: "report", Timeout: time.Minute})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	currentDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("get current directory: %v", err)
	}
	if resolved.Remote || resolved.LocalTarget != currentDirectory {
		t.Fatalf("resolved = %+v, want local target %q", resolved, currentDirectory)
	}
}

func TestResolveRemoteDefaultTokenFromEnvironment(t *testing.T) {
	t.Setenv("PAGES_REMOTE", "https://pages.example.com")
	t.Setenv("PAGES_UPLOAD_TOKEN", "default-secret")
	resolved, err := Resolve(Input{File: "page.html", Slug: "report", Identity: "bumble", Timeout: time.Minute})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved.Identity != "" || resolved.Token != "default-secret" {
		t.Fatalf("resolved = %+v", resolved)
	}
}

func TestResolveEnvironmentTokenIgnoresInvalidFlagIdentity(t *testing.T) {
	t.Setenv("PAGES_REMOTE", "https://pages.example.com")
	t.Setenv("PAGES_UPLOAD_TOKEN", "default-secret")
	resolved, err := Resolve(Input{File: "page.html", Slug: "report", Identity: "Bad", Timeout: time.Minute})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved.Identity != "" || resolved.Token != "default-secret" {
		t.Fatalf("resolved = %+v", resolved)
	}
}

func TestResolveRemoteNamedTokenFromEnvironment(t *testing.T) {
	t.Setenv("PAGES_REMOTE", "https://pages.example.com")
	t.Setenv("PAGES_UPLOAD_TOKEN", "bumble.secret")
	resolved, err := Resolve(Input{File: "page.html", Slug: "report", Timeout: time.Minute})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved.Identity != "bumble" || resolved.Token != "bumble.secret" {
		t.Fatalf("resolved = %+v", resolved)
	}
}

func TestResolveRemoteTokensFromConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	config := `{"remote":"https://pages.example.com","token":"default-secret","identities":{"bumble":"secret"}}`
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	for identity, wantToken := range map[string]string{"": "default-secret", "bumble": "bumble.secret"} {
		resolved, err := Resolve(Input{File: "page.html", Slug: "report", Identity: identity, Timeout: time.Minute, ConfigPath: configPath})
		if err != nil {
			t.Fatalf("resolve %q: %v", identity, err)
		}
		if resolved.Identity != identity || resolved.Token != wantToken {
			t.Fatalf("resolved %q = %+v", identity, resolved)
		}
	}
}

func TestResolveRemoteModeRequiresSelectedToken(t *testing.T) {
	t.Setenv("PAGES_REMOTE", "https://pages.example.com")
	_, err := Resolve(Input{File: "page.html", Slug: "report", Timeout: time.Minute})
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("err = %v, want usage error", err)
	}
}

func TestResolveNamedConfigMustExist(t *testing.T) {
	_, err := Resolve(Input{File: "page.html", Slug: "report", Timeout: time.Minute, ConfigPath: filepath.Join(t.TempDir(), "missing.json")})
	if err == nil || errors.Is(err, ErrUsage) {
		t.Fatalf("err = %v, want config read error", err)
	}
}

func TestLoadConfigRejectsUnknownField(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, []byte(`{"extra":"value"}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if _, err := LoadConfig(configPath); err == nil {
		t.Fatal("load succeeded, want unknown field error")
	}
}

func TestRunLocalPublishesDefaultAndNamedHTML(t *testing.T) {
	root := t.TempDir()
	publicRoot := filepath.Join(root, "public")
	pageFile := filepath.Join(root, "page.html")
	if err := os.WriteFile(pageFile, []byte("<!doctype html><h1>local</h1>"), 0o600); err != nil {
		t.Fatalf("write page: %v", err)
	}

	for identity, relative := range map[string]string{"": filepath.Join("report"), "bumble": filepath.Join("@bumble", "report")} {
		output, err := Run(&Resolved{File: pageFile, Slug: "report", Identity: identity, LocalTarget: publicRoot})
		if err != nil {
			t.Fatalf("run %q: %v", identity, err)
		}
		want := filepath.Join(publicRoot, relative) + "/"
		if output != want {
			t.Fatalf("output = %q, want %q", output, want)
		}
		page, err := os.ReadFile(filepath.Join(publicRoot, relative, "index.html"))
		if err != nil || string(page) != "<!doctype html><h1>local</h1>" {
			t.Fatalf("page = %q, %v", page, err)
		}
	}
}

func TestRunLocalPublishesDefaultZip(t *testing.T) {
	root := t.TempDir()
	publicRoot := filepath.Join(root, "public")
	zipFile := filepath.Join(root, "page.zip")
	writeTestZip(t, zipFile, map[string]string{"index.html": "<h1>zip</h1>", "img/chart.png": "png"})

	output, err := Run(&Resolved{File: zipFile, Slug: "report", LocalTarget: publicRoot})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(publicRoot, "report", "img", "chart.png")); err != nil {
		t.Fatalf("extracted asset missing: %v", err)
	}
	if output != filepath.Join(publicRoot, "report")+"/" {
		t.Fatalf("output = %q", output)
	}
}

func TestRunLocalHTMLReplacesCompleteZipPage(t *testing.T) {
	root := t.TempDir()
	publicRoot := filepath.Join(root, "public")
	zipFile := filepath.Join(root, "page.zip")
	htmlFile := filepath.Join(root, "page.html")
	writeTestZip(t, zipFile, map[string]string{"index.html": "zip", "img/stale.png": "stale"})
	if err := os.WriteFile(htmlFile, []byte("html"), 0o600); err != nil {
		t.Fatalf("write HTML: %v", err)
	}

	if _, err := Run(&Resolved{File: zipFile, Slug: "report", LocalTarget: publicRoot}); err != nil {
		t.Fatalf("publish zip: %v", err)
	}
	if _, err := Run(&Resolved{File: htmlFile, Slug: "report", LocalTarget: publicRoot}); err != nil {
		t.Fatalf("publish HTML: %v", err)
	}
	page, err := os.ReadFile(filepath.Join(publicRoot, "report", "index.html"))
	if err != nil || string(page) != "html" {
		t.Fatalf("page = %q, %v", page, err)
	}
	if _, err := os.Stat(filepath.Join(publicRoot, "report", "img", "stale.png")); !os.IsNotExist(err) {
		t.Fatalf("stale zip asset still exists: %v", err)
	}
}

func TestRunLocalLeavesOtherActiveStaging(t *testing.T) {
	root := t.TempDir()
	publicRoot := filepath.Join(root, "public")
	stager, err := pages.NewStager(publicRoot)
	if err != nil {
		t.Fatalf("new stager: %v", err)
	}
	active, err := stager.StageDir("bumble")
	if err != nil {
		t.Fatalf("create active staging: %v", err)
	}
	pageFile := filepath.Join(root, "page.html")
	if err := os.WriteFile(pageFile, []byte("page"), 0o600); err != nil {
		t.Fatalf("write page: %v", err)
	}

	if _, err := Run(&Resolved{File: pageFile, Slug: "report", LocalTarget: publicRoot}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if _, err := os.Stat(active); err != nil {
		t.Fatalf("other active staging was removed: %v", err)
	}
}

func TestRunLocalRejectsBadZipAndLeavesNoTrace(t *testing.T) {
	root := t.TempDir()
	publicRoot := filepath.Join(root, "public")
	zipFile := filepath.Join(root, "bad.zip")
	writeTestZip(t, zipFile, map[string]string{"report.html": "no index"})

	if _, err := Run(&Resolved{File: zipFile, Slug: "report", LocalTarget: publicRoot}); err == nil {
		t.Fatal("run succeeded, want zip validation error")
	}
	if _, err := os.Stat(filepath.Join(publicRoot, "report")); !os.IsNotExist(err) {
		t.Fatalf("page exists after rejected publish: %v", err)
	}
}

func writeTestZip(t *testing.T, path string, entries map[string]string) {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range entries {
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
	if err := os.WriteFile(path, buffer.Bytes(), 0o600); err != nil {
		t.Fatalf("write zip: %v", err)
	}
}
