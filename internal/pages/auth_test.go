package pages

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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

func TestTokenIdentityDefaultAndNamed(t *testing.T) {
	for token, want := range map[string]string{
		"secret":            "",
		"secret-one":        "",
		"bumble.secret":     "bumble",
		"bumble.secret-one": "bumble",
	} {
		identity, err := TokenIdentity(token)
		if err != nil || identity != want {
			t.Fatalf("TokenIdentity(%q) = %q, %v; want %q", token, identity, err, want)
		}
	}
	for _, token := range []string{"", "secret+one", "bumble.secret.more", "Bad.secret", "bumble.secret+one", "bumble.", ".secret"} {
		if _, err := TokenIdentity(token); err == nil {
			t.Fatalf("TokenIdentity(%q) succeeded, want error", token)
		}
	}
}

func TestValidName(t *testing.T) {
	if !ValidName("bumble") || !ValidName("a") || !ValidName("report-2") || !ValidName("trail-") {
		t.Fatal("ValidName rejected a safe name")
	}
	for _, name := range []string{"", "-lead", "Upper", "with space", "a/b", "12345678901234567890123456789012345678901234567890123456789012345", "."} {
		if ValidName(name) {
			t.Fatalf("ValidName accepted %q", name)
		}
	}
}

func TestIdentityScope(t *testing.T) {
	if got := IdentityScope(""); got != "" {
		t.Fatalf("default scope = %q", got)
	}
	if got := IdentityScope("bumble"); got != "@bumble" {
		t.Fatalf("named scope = %q", got)
	}
}

func TestReadTokensFileMissingYieldsEmptyTokens(t *testing.T) {
	tokens, err := ReadTokensFile(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("read missing file: %v", err)
	}
	if tokens.Token != "" || len(tokens.Identities) != 0 {
		t.Fatalf("tokens = %+v, want empty", tokens)
	}
}

func TestReadTokensFileRejectsUnknownAndInvalidEntries(t *testing.T) {
	for name, body := range map[string]string{
		"unknown field":  `{"bumble":"secret"}`,
		"dotted default": `{"token":"with.dot"}`,
		"dotted named":   `{"identities":{"bumble":"with.dot"}}`,
		"symbol default": `{"token":"with+symbol"}`,
		"invalid name":   `{"identities":{"Bad":"secret"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "tokens.json")
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatalf("write tokens: %v", err)
			}
			if _, err := ReadTokensFile(path); err == nil {
				t.Fatal("read succeeded, want error")
			}
		})
	}
}

func TestWriteTokensFileCreatesAndKeepsScopes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokens.json")
	tokens := Tokens{Token: "default-secret", Identities: map[string]string{"bumble": "secret-one"}}
	if err := WriteTokensFile(path, tokens); err != nil {
		t.Fatalf("write: %v", err)
	}
	tokens.Identities["fizz"] = "secret-two"
	if err := WriteTokensFile(path, tokens); err != nil {
		t.Fatalf("write: %v", err)
	}

	reloaded, err := ReadTokensFile(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Token != "default-secret" || reloaded.Identities["bumble"] != "secret-one" || reloaded.Identities["fizz"] != "secret-two" {
		t.Fatalf("reloaded = %+v", reloaded)
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
	if err := WriteTokensFile(filepath.Join(t.TempDir(), "tokens.json"), Tokens{}); err == nil {
		t.Fatal("write succeeded, want error for empty Tokens")
	}
}

func TestIssueTokenCreatesPrivateParentDirectory(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "config", "pages")
	path := filepath.Join(parent, "tokens.json")
	if _, err := IssueToken(path, "", false); err != nil {
		t.Fatalf("issue token: %v", err)
	}
	info, err := os.Stat(parent)
	if err != nil {
		t.Fatalf("stat parent: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o700 {
		t.Fatalf("parent mode = %o, want 700", got)
	}
	info, err = os.Stat(path)
	if err != nil {
		t.Fatalf("stat tokens: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("tokens mode = %o, want 600", got)
	}
}

func TestIssueTokenConcurrentProcessesKeepEveryIdentity(t *testing.T) {
	if identity := os.Getenv("PAGES_TEST_ISSUE_IDENTITY"); identity != "" {
		if _, err := IssueToken(os.Getenv("PAGES_TEST_ISSUE_PATH"), identity, false); err != nil {
			t.Fatalf("issue token: %v", err)
		}
		return
	}

	const processCount = 12
	path := filepath.Join(t.TempDir(), "tokens.json")
	type child struct {
		command *exec.Cmd
		output  bytes.Buffer
	}
	children := make([]child, processCount)
	for index := range children {
		identity := "identity-" + strconv.Itoa(index)
		command := exec.Command(os.Args[0], "-test.run=^TestIssueTokenConcurrentProcessesKeepEveryIdentity$")
		command.Env = append(os.Environ(),
			"PAGES_TEST_ISSUE_IDENTITY="+identity,
			"PAGES_TEST_ISSUE_PATH="+path,
		)
		children[index].command = command
		command.Stdout = &children[index].output
		command.Stderr = &children[index].output
		if err := command.Start(); err != nil {
			t.Fatalf("start child %d: %v", index, err)
		}
	}
	for index := range children {
		if err := children[index].command.Wait(); err != nil {
			t.Fatalf("child %d: %v\n%s", index, err, children[index].output.String())
		}
	}

	tokens, err := ReadTokensFile(path)
	if err != nil {
		t.Fatalf("read tokens: %v", err)
	}
	if len(tokens.Identities) != processCount {
		t.Fatalf("identities = %d, want %d", len(tokens.Identities), processCount)
	}
}

func TestAuthenticateDefaultAndNamed(t *testing.T) {
	tokens := Tokens{Token: "default-secret", Identities: map[string]string{"bumble": "secret-one"}}
	for header, want := range map[string]string{
		"Bearer default-secret":    "",
		"Bearer bumble.secret-one": "bumble",
	} {
		identity, ok := tokens.Authenticate(header)
		if !ok || identity != want {
			t.Fatalf("authenticate %q = %q, %v; want %q", header, identity, ok, want)
		}
	}
	for _, header := range []string{"", "Basic default-secret", "Bearer wrong", "Bearer bumble.wrong", "Bearer fizz.secret-one", "Bearer bumble.secret-one.more"} {
		if _, ok := tokens.Authenticate(header); ok {
			t.Fatalf("authenticate accepted %q", header)
		}
	}
}
