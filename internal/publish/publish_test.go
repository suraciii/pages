package publish

import (
	"archive/zip"
	"bytes"
	"errors"
	"io/fs"
	"path/filepath"
	"testing"
	"time"

	"github.com/suraciii/pages/internal/destination"
	"github.com/suraciii/pages/internal/filesystem"
	"github.com/suraciii/pages/internal/pages"
)

func testRuntime(environment map[string]string) (Runtime, *filesystem.Memory) {
	fileSystem := filesystem.NewMemory("/workspace")
	workDir, err := fileSystem.Abs(".")
	if err != nil {
		panic(err)
	}
	return Runtime{
		FileSystem: fileSystem,
		Environment: func(name string) string {
			return environment[name]
		},
		CurrentDirectory: func() (string, error) { return workDir, nil },
	}, fileSystem
}

func TestResolveLocalModeDefaultsIdentityAndIgnoresUploadToken(t *testing.T) {
	runtime, _ := testRuntime(map[string]string{
		"PAGES_DESTINATION":  "/public",
		"PAGES_UPLOAD_TOKEN": "bumble.secret",
	})
	resolved, err := ResolveWithRuntime(Input{File: "page.html", Slug: "report", Timeout: time.Minute}, runtime)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved.Destination.IsRemote() || resolved.Destination.LocalPath() != "/public" || resolved.Identity != "" || resolved.Token != "" {
		t.Fatalf("resolved = %+v, want local Default Identity", resolved)
	}
}

func TestResolveLocalNamedIdentity(t *testing.T) {
	runtime, _ := testRuntime(map[string]string{"PAGES_DESTINATION": "/public"})
	resolved, err := ResolveWithRuntime(Input{File: "page.html", Slug: "report", Identity: "bumble", Timeout: time.Minute}, runtime)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved.Identity != "bumble" {
		t.Fatalf("identity = %q", resolved.Identity)
	}
}

func TestResolveLocalModeDefaultsToCurrentDirectory(t *testing.T) {
	runtime, _ := testRuntime(nil)
	resolved, err := ResolveWithRuntime(Input{File: "page.html", Slug: "report", Timeout: time.Minute}, runtime)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	workDir, _ := runtime.FileSystem.Abs(".")
	if resolved.Destination.IsRemote() || resolved.Destination.LocalPath() != workDir {
		t.Fatalf("resolved = %+v, want %s", resolved, workDir)
	}
}

func TestResolveRemoteDefaultTokenFromEnvironment(t *testing.T) {
	runtime, _ := testRuntime(map[string]string{
		"PAGES_DESTINATION":  "https://pages.example.com",
		"PAGES_UPLOAD_TOKEN": "default-secret",
	})
	resolved, err := ResolveWithRuntime(Input{File: "page.html", Slug: "report", Identity: "bumble", Timeout: time.Minute}, runtime)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved.Identity != "" || resolved.Token != "default-secret" {
		t.Fatalf("resolved = %+v", resolved)
	}
}

func TestResolveEnvironmentTokenIgnoresInvalidFlagIdentity(t *testing.T) {
	runtime, _ := testRuntime(map[string]string{
		"PAGES_DESTINATION":  "https://pages.example.com",
		"PAGES_UPLOAD_TOKEN": "default-secret",
	})
	resolved, err := ResolveWithRuntime(Input{File: "page.html", Slug: "report", Identity: "Bad", Timeout: time.Minute}, runtime)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved.Identity != "" || resolved.Token != "default-secret" {
		t.Fatalf("resolved = %+v", resolved)
	}
}

func TestResolveRemoteNamedTokenFromEnvironment(t *testing.T) {
	runtime, _ := testRuntime(map[string]string{
		"PAGES_DESTINATION":  "https://pages.example.com",
		"PAGES_UPLOAD_TOKEN": "bumble.secret",
	})
	resolved, err := ResolveWithRuntime(Input{File: "page.html", Slug: "report", Timeout: time.Minute}, runtime)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved.Identity != "bumble" || resolved.Token != "bumble.secret" {
		t.Fatalf("resolved = %+v", resolved)
	}
}

func TestResolveRemoteTokensFromConfig(t *testing.T) {
	runtime, fileSystem := testRuntime(nil)
	configPath := "/config/config.json"
	mustWriteFile(t, fileSystem, configPath, `{"destination":"https://pages.example.com","token":"default-secret","identities":{"bumble":"secret"}}`)

	for identity, wantToken := range map[string]string{"": "default-secret", "bumble": "bumble.secret"} {
		resolved, err := ResolveWithRuntime(Input{File: "page.html", Slug: "report", Identity: identity, Timeout: time.Minute, ConfigPath: configPath}, runtime)
		if err != nil {
			t.Fatalf("resolve %q: %v", identity, err)
		}
		if resolved.Identity != identity || resolved.Token != wantToken {
			t.Fatalf("resolved %q = %+v", identity, resolved)
		}
	}
}

func TestResolveDestinationPrecedence(t *testing.T) {
	runtime, fileSystem := testRuntime(map[string]string{"PAGES_DESTINATION": "/environment"})
	configPath := "/config/config.json"
	mustWriteFile(t, fileSystem, configPath, `{"destination":"/config"}`)

	for name, input := range map[string]Input{
		"flag":        {File: "page.html", Slug: "report", Destination: "/flag", Timeout: time.Minute, ConfigPath: configPath},
		"environment": {File: "page.html", Slug: "report", Timeout: time.Minute, ConfigPath: configPath},
	} {
		t.Run(name, func(t *testing.T) {
			resolved, err := ResolveWithRuntime(input, runtime)
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			want := "/environment"
			if name == "flag" {
				want = "/flag"
			}
			if resolved.Destination.LocalPath() != want {
				t.Fatalf("destination = %q, want %q", resolved.Destination.LocalPath(), want)
			}
		})
	}
}

func TestLoadConfigRejectsLegacyDestinationFields(t *testing.T) {
	for _, field := range []string{"remote", "public-root"} {
		t.Run(field, func(t *testing.T) {
			runtime, fileSystem := testRuntime(nil)
			configPath := "/config/config.json"
			mustWriteFile(t, fileSystem, configPath, `{"`+field+`":"/old"}`)
			if _, err := loadConfig(runtime.FileSystem, configPath); err == nil {
				t.Fatal("load succeeded, want unknown field error")
			}
		})
	}
}

func TestResolveRemoteModeRequiresSelectedToken(t *testing.T) {
	runtime, _ := testRuntime(map[string]string{"PAGES_DESTINATION": "https://pages.example.com"})
	_, err := ResolveWithRuntime(Input{File: "page.html", Slug: "report", Timeout: time.Minute}, runtime)
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("err = %v, want usage error", err)
	}
}

func TestResolveNamedConfigMustExist(t *testing.T) {
	runtime, _ := testRuntime(nil)
	_, err := ResolveWithRuntime(Input{File: "page.html", Slug: "report", Timeout: time.Minute, ConfigPath: "/config/missing.json"}, runtime)
	if err == nil || errors.Is(err, ErrUsage) {
		t.Fatalf("err = %v, want config read error", err)
	}
}

func TestLoadConfigRejectsUnknownField(t *testing.T) {
	runtime, fileSystem := testRuntime(nil)
	configPath := "/config/config.json"
	mustWriteFile(t, fileSystem, configPath, `{"extra":"value"}`)
	if _, err := loadConfig(runtime.FileSystem, configPath); err == nil {
		t.Fatal("load succeeded, want unknown field error")
	}
}

func TestRunLocalPublishesDefaultAndNamedHTML(t *testing.T) {
	runtime, fileSystem := testRuntime(nil)
	pageFile := "/source/page.html"
	mustWriteFile(t, fileSystem, pageFile, "<!doctype html><h1>local</h1>")

	for identity, relative := range map[string]string{"": "report", "bumble": filepath.Join("@bumble", "report")} {
		output, err := RunWithRuntime(&Resolved{File: pageFile, Slug: "report", Identity: identity, Destination: localDestination(t, "/public")}, runtime)
		if err != nil {
			t.Fatalf("run %q: %v", identity, err)
		}
		absolutePublicRoot, err := fileSystem.Abs("/public")
		if err != nil {
			t.Fatalf("resolve public root: %v", err)
		}
		want := filepath.Join(absolutePublicRoot, relative) + string(filepath.Separator)
		if output != want {
			t.Fatalf("output = %q, want %q", output, want)
		}
		page, err := fileSystem.ReadFile(filepath.Join("/public", relative, "index.html"))
		if err != nil || string(page) != "<!doctype html><h1>local</h1>" {
			t.Fatalf("page = %q, %v", page, err)
		}
	}
}

func TestRunLocalPublishesDefaultZip(t *testing.T) {
	runtime, fileSystem := testRuntime(nil)
	zipFile := "/source/page.zip"
	writeTestZip(t, fileSystem, zipFile, map[string]string{"index.html": "<h1>zip</h1>", "img/chart.png": "png"})

	output, err := RunWithRuntime(&Resolved{File: zipFile, Slug: "report", Destination: localDestination(t, "/public")}, runtime)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if _, err := fileSystem.Stat("/public/report/img/chart.png"); err != nil {
		t.Fatalf("extracted asset missing: %v", err)
	}
	absolutePublicRoot, err := fileSystem.Abs("/public")
	if err != nil {
		t.Fatalf("resolve public root: %v", err)
	}
	if output != filepath.Join(absolutePublicRoot, "report")+string(filepath.Separator) {
		t.Fatalf("output = %q", output)
	}
}

func TestRunLocalHTMLReplacesCompleteZipPage(t *testing.T) {
	runtime, fileSystem := testRuntime(nil)
	zipFile := "/source/page.zip"
	htmlFile := "/source/page.html"
	writeTestZip(t, fileSystem, zipFile, map[string]string{"index.html": "zip", "img/stale.png": "stale"})
	mustWriteFile(t, fileSystem, htmlFile, "html")

	if _, err := RunWithRuntime(&Resolved{File: zipFile, Slug: "report", Destination: localDestination(t, "/public")}, runtime); err != nil {
		t.Fatalf("publish zip: %v", err)
	}
	if _, err := RunWithRuntime(&Resolved{File: htmlFile, Slug: "report", Destination: localDestination(t, "/public")}, runtime); err != nil {
		t.Fatalf("publish HTML: %v", err)
	}
	page, err := fileSystem.ReadFile("/public/report/index.html")
	if err != nil || string(page) != "html" {
		t.Fatalf("page = %q, %v", page, err)
	}
	if _, err := fileSystem.Stat("/public/report/img/stale.png"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("stale zip asset still exists: %v", err)
	}
}

func TestRunLocalLeavesOtherActiveStaging(t *testing.T) {
	runtime, fileSystem := testRuntime(nil)
	stager, err := pages.NewStagerWithFS("/public", fileSystem)
	if err != nil {
		t.Fatalf("new stager: %v", err)
	}
	active, err := stager.StageDir("bumble")
	if err != nil {
		t.Fatalf("create active staging: %v", err)
	}
	mustWriteFile(t, fileSystem, "/source/page.html", "page")

	if _, err := RunWithRuntime(&Resolved{File: "/source/page.html", Slug: "report", Destination: localDestination(t, "/public")}, runtime); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if _, err := fileSystem.Stat(active); err != nil {
		t.Fatalf("other active staging was removed: %v", err)
	}
}

func TestRunLocalRejectsBadZipAndLeavesNoTrace(t *testing.T) {
	runtime, fileSystem := testRuntime(nil)
	writeTestZip(t, fileSystem, "/source/bad.zip", map[string]string{"report.html": "no index"})

	if _, err := RunWithRuntime(&Resolved{File: "/source/bad.zip", Slug: "report", Destination: localDestination(t, "/public")}, runtime); err == nil {
		t.Fatal("run succeeded, want zip validation error")
	}
	if _, err := fileSystem.Stat("/public/report"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("page exists after rejected publish: %v", err)
	}
}

func localDestination(t *testing.T, path string) destination.Value {
	t.Helper()
	value, err := destination.ParseWithCurrentDirectory(path, func() (string, error) { return "/workspace", nil })
	if err != nil {
		t.Fatalf("parse destination: %v", err)
	}
	return value
}

func mustWriteFile(t *testing.T, fileSystem filesystem.FS, path, content string) {
	t.Helper()
	if err := fileSystem.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create parent: %v", err)
	}
	if err := fileSystem.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func writeTestZip(t *testing.T, fileSystem filesystem.FS, path string, entries map[string]string) {
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
	if err := fileSystem.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create zip parent: %v", err)
	}
	if err := fileSystem.WriteFile(path, buffer.Bytes(), 0o600); err != nil {
		t.Fatalf("write zip: %v", err)
	}
}
