package cli

import (
	"path/filepath"
	"testing"
)

func TestResolvedTokensFileDefault(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)
	t.Setenv("PAGES_CONFIG_DIR", "")
	t.Setenv("PAGES_TOKENS_FILE", "")
	want := filepath.Join(configDir, "pages", "tokens.json")
	if got := tokensFilePath(defaultConfigDir()); got != want {
		t.Fatalf("tokensFilePath(defaultConfigDir()) = %q, want %q", got, want)
	}
	if got := defaultConfigPath(); got != filepath.Join(configDir, "pages", "config.json") {
		t.Fatalf("defaultConfigPath() = %q", got)
	}
}

func TestResolvedTokensFileEnvironment(t *testing.T) {
	want := filepath.Join(t.TempDir(), "pages_tokens")
	t.Setenv("PAGES_TOKENS_FILE", want)
	if got := tokensFilePath(filepath.Join(t.TempDir(), "config")); got != want {
		t.Fatalf("tokensFilePath() = %q", got)
	}
}

func TestConfigDirectoryEnvironmentOverridesUserConfigDir(t *testing.T) {
	want := filepath.Join(t.TempDir(), "pages-config")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("PAGES_CONFIG_DIR", want)
	t.Setenv("PAGES_TOKENS_FILE", "")
	if got := defaultConfigDir(); got != want {
		t.Fatalf("defaultConfigDir() = %q, want %q", got, want)
	}
	if got := tokensFilePath(defaultConfigDir()); got != filepath.Join(want, "tokens.json") {
		t.Fatalf("tokensFilePath() = %q", got)
	}
}
