package pages

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateSecretFormatAndUniqueness(t *testing.T) {
	first, err := GenerateSecret()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	second, err := GenerateSecret()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if first == second {
		t.Fatalf("two generated secrets are equal: %q", first)
	}
	if strings.Contains(first, ".") {
		t.Fatalf("secret %q contains a dot", first)
	}
}

func TestTokenIdentityAcceptsAndRejects(t *testing.T) {
	valid := []string{"bumble.secret", "bumble.secret-one"}
	for _, token := range valid {
		if identity, err := TokenIdentity(token); err != nil || identity != "bumble" {
			t.Fatalf("TokenIdentity(%q) = %q, %v", token, identity, err)
		}
	}
	invalid := []string{"", "bumble", "bumble.secret.more", "Bad.secret", "bumble.", ".secret"}
	for _, token := range invalid {
		if _, err := TokenIdentity(token); err == nil {
			t.Fatalf("TokenIdentity(%q) succeeded, want error", token)
		}
	}
}

func TestValidName(t *testing.T) {
	if !ValidName("bumble") || !ValidName("a") || !ValidName("report-2") || !ValidName("trail-") {
		t.Fatal("ValidName rejected a safe name")
	}
	invalid := []string{"", "-lead", "Upper", "with space", "a/b", "12345678901234567890123456789012345678901234567890123456789012345", "."}
	for _, name := range invalid {
		if ValidName(name) {
			t.Fatalf("ValidName accepted %q", name)
		}
	}
}

func TestReadTokensFileMissingYieldsEmptyMap(t *testing.T) {
	tokens, err := ReadTokensFile(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("read missing file: %v", err)
	}
	if len(tokens) != 0 {
		t.Fatalf("tokens = %v, want empty", tokens)
	}
}

func TestReadTokensFileRejectsInvalidEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokens.json")
	if err := os.WriteFile(path, []byte(`{"bumble": "with.dot"}`), 0o600); err != nil {
		t.Fatalf("write tokens: %v", err)
	}
	if _, err := ReadTokensFile(path); err == nil {
		t.Fatal("read succeeded, want error for a dotted secret")
	}
}

func TestWriteTokensFileCreatesAndKeepsEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokens.json")
	tokens := Tokens{"bumble": "secret-one"}
	if err := WriteTokensFile(path, tokens); err != nil {
		t.Fatalf("write: %v", err)
	}
	tokens["fizz"] = "secret-two"
	if err := WriteTokensFile(path, tokens); err != nil {
		t.Fatalf("write: %v", err)
	}

	reloaded, err := ReadTokensFile(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded["bumble"] != "secret-one" || reloaded["fizz"] != "secret-two" {
		t.Fatalf("reloaded = %v", reloaded)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode = %o, want 600", got)
	}
}

func TestWriteTokensFileRejectsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokens.json")
	if err := WriteTokensFile(path, Tokens{}); err == nil {
		t.Fatal("write succeeded, want error for an empty map")
	}
}

func TestAuthenticate(t *testing.T) {
	tokens := Tokens{"bumble": "secret-one"}
	header := "Bearer bumble.secret-one"
	if identity, ok := tokens.Authenticate(header); !ok || identity != "bumble" {
		t.Fatalf("authenticate = %q, %v", identity, ok)
	}
	rejected := []string{
		"",
		"Basic bumble.secret-one",
		"Bearer bumble.wrong",
		"Bearer fizz.secret-one",
		"Bearer bumble",
		"Bearer bumble.secret-one.more",
	}
	for _, header := range rejected {
		if _, ok := tokens.Authenticate(header); ok {
			t.Fatalf("authenticate accepted %q", header)
		}
	}
}
