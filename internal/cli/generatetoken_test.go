package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/suraciii/pages/internal/pages"
)

func TestGenerateTokenDefaultAndNamed(t *testing.T) {
	tokensFile := filepath.Join(t.TempDir(), "tokens.json")
	defaultOutput := captureStdout(t, func() int {
		return runGenerateToken([]string{"--tokens-file", tokensFile})
	})
	if defaultOutput == "" || strings.Contains(defaultOutput, ".") {
		t.Fatalf("default output = %q", defaultOutput)
	}

	namedOutput := captureStdout(t, func() int {
		return runGenerateToken([]string{"--tokens-file", tokensFile, "bumble"})
	})
	if !strings.HasPrefix(namedOutput, "bumble.") {
		t.Fatalf("named output = %q", namedOutput)
	}

	tokens, err := pages.ReadTokensFile(tokensFile)
	if err != nil {
		t.Fatalf("read tokens: %v", err)
	}
	if tokens.Token != defaultOutput || "bumble."+tokens.Identities["bumble"] != namedOutput {
		t.Fatalf("tokens = %+v", tokens)
	}
}

func TestGenerateTokenRequiresReplace(t *testing.T) {
	tokensFile := filepath.Join(t.TempDir(), "tokens.json")
	if status := runGenerateToken([]string{"--tokens-file", tokensFile}); status != 0 {
		t.Fatalf("first status = %d", status)
	}
	if status := runGenerateToken([]string{"--tokens-file", tokensFile}); status != 1 {
		t.Fatalf("duplicate status = %d", status)
	}
	if status := runGenerateToken([]string{"--tokens-file", tokensFile, "--replace"}); status != 0 {
		t.Fatalf("replace status = %d", status)
	}
}

func TestGenerateTokenConfigDirectory(t *testing.T) {
	configDir := filepath.Join(t.TempDir(), "pages-config")
	output := captureStdout(t, func() int {
		return runGenerateToken([]string{"--config-dir", configDir})
	})
	if output == "" {
		t.Fatal("generate-token produced no token")
	}
	if _, err := os.Stat(filepath.Join(configDir, "tokens.json")); err != nil {
		t.Fatalf("tokens file: %v", err)
	}
}

func TestGenerateTokenExactFileOverridesConfigDirectory(t *testing.T) {
	configDir := filepath.Join(t.TempDir(), "pages-config")
	exactFile := filepath.Join(t.TempDir(), "mounted", "tokens.json")
	output := captureStdout(t, func() int {
		return runGenerateToken([]string{
			"--config-dir", configDir,
			"--tokens-file", exactFile,
		})
	})
	if output == "" {
		t.Fatal("generate-token produced no token")
	}
	if _, err := os.Stat(exactFile); err != nil {
		t.Fatalf("exact tokens file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(configDir, "tokens.json")); !os.IsNotExist(err) {
		t.Fatalf("configuration directory tokens file error = %v", err)
	}
}

func TestPublishConfigDirectory(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(t.TempDir(), "pages-config")
	filePath := filepath.Join(t.TempDir(), "report.html")
	if err := os.WriteFile(filePath, []byte("<h1>ready</h1>"), 0o600); err != nil {
		t.Fatalf("write page: %v", err)
	}
	configPath := filepath.Join(configDir, "config.json")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("create config directory: %v", err)
	}
	config := `{"public-root":"` + root + `"}`
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	output, _, status := captureOutput(t, func() int {
		return runPublish([]string{"--config-dir", configDir, "--file", filePath, "--slug", "report"})
	})
	if status != 0 {
		t.Fatalf("status = %d, output = %q", status, output)
	}
	if _, err := os.Stat(filepath.Join(root, "report", "index.html")); err != nil {
		t.Fatalf("published page: %v", err)
	}
}

func TestSubcommandHelpUsesStdoutAndShowsDefaults(t *testing.T) {
	stdout, stderr, status := captureOutput(t, func() int {
		return runPublish([]string{"--help"})
	})
	if status != 0 {
		t.Fatalf("status = %d", status)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
	for _, text := range []string{"Usage: pages publish", "--timeout", "default 1m30s", "--config"} {
		if !strings.Contains(stdout, text) {
			t.Fatalf("stdout does not contain %q:\n%s", text, stdout)
		}
	}
}

func TestSubcommandRejectsSingleDashFlagAlias(t *testing.T) {
	_, stderr, status := captureOutput(t, func() int {
		return runPublish([]string{"-file", "page.html"})
	})
	if status != 2 || !strings.Contains(stderr, "use --file instead of -file") {
		t.Fatalf("status = %d, stderr = %q", status, stderr)
	}
}

func TestSubcommandErrorsStayOnStderrWhenHelpAppearsLater(t *testing.T) {
	stdout, stderr, status := captureOutput(t, func() int {
		return runPublish([]string{"--bad", "--help"})
	})
	if status != 2 || stdout != "" || !strings.Contains(stderr, "flag provided but not defined") {
		t.Fatalf("status = %d, stdout = %q, stderr = %q", status, stdout, stderr)
	}
}

func TestPublishAndServeRejectPositionalArguments(t *testing.T) {
	for name, run := range map[string]func() int{
		"publish": func() int { return runPublish([]string{"extra"}) },
		"serve":   func() int { return runServe([]string{"extra"}) },
	} {
		t.Run(name, func(t *testing.T) {
			_, stderr, status := captureOutput(t, run)
			if status != 2 || !strings.Contains(stderr, "positional arguments are not allowed") {
				t.Fatalf("status = %d, stderr = %q", status, stderr)
			}
		})
	}
}

func captureStdout(t *testing.T, run func() int) string {
	t.Helper()
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	original := os.Stdout
	os.Stdout = write
	status := run()
	os.Stdout = original
	if err := write.Close(); err != nil {
		t.Fatalf("close stdout pipe: %v", err)
	}
	data, err := io.ReadAll(read)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	read.Close()
	if status != 0 {
		t.Fatalf("status = %d", status)
	}
	return strings.TrimSpace(string(data))
}

func captureOutput(t *testing.T, run func() int) (string, string, int) {
	t.Helper()
	stdoutRead, stdoutWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stderrRead, stderrWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}
	originalStdout, originalStderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = stdoutWrite, stderrWrite
	status := run()
	os.Stdout, os.Stderr = originalStdout, originalStderr
	stdoutWrite.Close()
	stderrWrite.Close()
	stdout, err := io.ReadAll(stdoutRead)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	stderr, err := io.ReadAll(stderrRead)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	stdoutRead.Close()
	stderrRead.Close()
	return string(stdout), string(stderr), status
}
