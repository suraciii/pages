package cli

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveConfigDirDefault(t *testing.T) {
	command := newTestCommand()
	wantDir := filepath.Join("/config", "pages")
	got, err := resolveConfigDir(command.runtime, "")
	if err != nil {
		t.Fatalf("resolve config directory: %v", err)
	}
	if got != wantDir {
		t.Fatalf("default config directory = %q, want %q", got, wantDir)
	}
}

func TestServeRejectsRemoteDestination(t *testing.T) {
	command := newTestCommand()
	status := runServeWithRuntime([]string{"--dest", "https://pages.example.com"}, command.runtime)
	if status != 2 || !strings.Contains(command.stderr.String(), "destination must be a local path") {
		t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
	}
}

func TestResolveConfigDirExplicitValueSkipsUserConfigDir(t *testing.T) {
	command := newTestCommand()
	calls := 0
	command.runtime.userConfigDir = func() (string, error) {
		calls++
		return "", errors.New("unavailable")
	}
	got, err := resolveConfigDir(command.runtime, "/mounted/pages-config")
	if err != nil || got != "/mounted/pages-config" || calls != 0 {
		t.Fatalf("directory = %q, calls = %d, error = %v", got, calls, err)
	}
}

func TestCommandsFailWhenUserConfigDirIsUnavailable(t *testing.T) {
	for _, name := range []string{"generate-token", "serve"} {
		t.Run(name, func(t *testing.T) {
			command := newTestCommand()
			command.runtime.userConfigDir = func() (string, error) { return "", errors.New("unavailable") }
			args := []string(nil)
			status := runGenerateTokenWithRuntime(args, command.runtime)
			if name == "serve" {
				status = runServeWithRuntime(args, command.runtime)
			}
			if status != 1 || !strings.Contains(command.stderr.String(), "determine user config directory") {
				t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
			}
			if _, err := command.fileSystem.Stat("/workspace/tokens.json"); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("current-directory tokens file error = %v", err)
			}
		})
	}
}

func TestPublishWithoutUserConfigDirDoesNotReadCurrentDirectoryConfig(t *testing.T) {
	command := newTestCommand()
	command.runtime.userConfigDir = func() (string, error) { return "", errors.New("unavailable") }
	command.writeFile(t, "report.html", "<h1>ready</h1>")
	command.writeFile(t, "config.json", `{"destination":"/wrong"}`)
	status := runPublishWithRuntime([]string{"--file", "report.html", "--slug", "report"}, command.runtime)
	if status != 0 || command.stderr.Len() != 0 {
		t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
	}
	workDir, err := command.runtime.currentDirectory()
	if err != nil {
		t.Fatalf("resolve work directory: %v", err)
	}
	if _, err := command.fileSystem.Stat(filepath.Join(workDir, "report", "index.html")); err != nil {
		t.Fatalf("published page: %v", err)
	}
	if _, err := command.fileSystem.Stat("/wrong/report/index.html"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("current-directory config was used: %v", err)
	}
}

func TestExplicitTokenFileSkipsUserConfigDir(t *testing.T) {
	command := newTestCommand()
	command.runtime.userConfigDir = func() (string, error) { return "", errors.New("must not be called") }
	if status := runGenerateTokenWithRuntime([]string{"--tokens-file", "/mounted/tokens.json"}, command.runtime); status != 0 {
		t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
	}
	if _, err := command.fileSystem.Stat("/mounted/tokens.json"); err != nil {
		t.Fatalf("tokens file: %v", err)
	}
}

func TestServeExactTokenFileSkipsUserConfigDir(t *testing.T) {
	command := newTestCommand()
	command.runtime.userConfigDir = func() (string, error) { return "", errors.New("must not be called") }
	status := runServeWithRuntime([]string{"--tokens-file", "/missing/tokens.json"}, command.runtime)
	if status != 1 || strings.Contains(command.stderr.String(), "user config directory") || !strings.Contains(command.stderr.String(), "tokens file must contain") {
		t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
	}
}

func TestConfigDirectoryEnvironmentSkipsUserConfigDir(t *testing.T) {
	command := newTestCommand()
	command.environment["PAGES_CONFIG_DIR"] = "/mounted/pages-config"
	command.runtime.userConfigDir = func() (string, error) { return "", errors.New("must not be called") }
	if status := runGenerateTokenWithRuntime(nil, command.runtime); status != 0 {
		t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
	}
	if _, err := command.fileSystem.Stat("/mounted/pages-config/tokens.json"); err != nil {
		t.Fatalf("tokens file: %v", err)
	}
}

func TestServeDestinationAliasDefaultsToCurrentDirectoryBeforeReadingTokens(t *testing.T) {
	for _, flagName := range []string{"--destination", "--dest"} {
		t.Run(flagName, func(t *testing.T) {
			command := newTestCommand()
			status := runServeWithRuntime([]string{flagName, "/public", "--tokens-file", "/missing/tokens.json"}, command.runtime)
			if status != 1 || !strings.Contains(command.stderr.String(), "tokens file must contain") {
				t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
			}
		})
	}
}
