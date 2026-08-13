package cli

import "testing"

func TestResolvedTokensFileDefault(t *testing.T) {
	t.Setenv("PAGES_TOKENS_FILE", "")
	if got := resolvedTokensFile(); got != defaultTokensFile {
		t.Fatalf("resolvedTokensFile() = %q, want %q", got, defaultTokensFile)
	}
}

func TestResolvedTokensFileEnvironment(t *testing.T) {
	t.Setenv("PAGES_TOKENS_FILE", "/run/secrets/pages_tokens")
	if got := resolvedTokensFile(); got != "/run/secrets/pages_tokens" {
		t.Fatalf("resolvedTokensFile() = %q", got)
	}
}
