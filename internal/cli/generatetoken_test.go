package cli

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/suraciii/pages/internal/filesystem"
	"github.com/suraciii/pages/internal/pages"
)

type testCommand struct {
	runtime     commandRuntime
	fileSystem  *filesystem.Memory
	environment map[string]string
	stdout      *bytes.Buffer
	stderr      *bytes.Buffer
}

func newTestCommand() *testCommand {
	fileSystem := filesystem.NewMemory("/workspace")
	workDir, err := fileSystem.Abs(".")
	if err != nil {
		panic(err)
	}
	environment := make(map[string]string)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	return &testCommand{
		runtime: commandRuntime{
			fileSystem:       fileSystem,
			environment:      func(name string) string { return environment[name] },
			userConfigDir:    func() (string, error) { return "/config", nil },
			currentDirectory: func() (string, error) { return workDir, nil },
			stdout:           stdout,
			stderr:           stderr,
			version:          "v1.2.3",
		},
		fileSystem:  fileSystem,
		environment: environment,
		stdout:      stdout,
		stderr:      stderr,
	}
}

func (command *testCommand) writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := command.fileSystem.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create parent: %v", err)
	}
	if err := command.fileSystem.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func TestGenerateTokenDefaultAndNamed(t *testing.T) {
	command := newTestCommand()
	tokensFile := "/state/tokens.json"
	if status := runGenerateToken([]string{"--tokens-file", tokensFile}, command.runtime); status != 0 {
		t.Fatalf("default status = %d, stderr = %q", status, command.stderr.String())
	}
	defaultToken := strings.TrimSpace(command.stdout.String())
	if defaultToken == "" || strings.Contains(defaultToken, ".") {
		t.Fatalf("default output = %q", defaultToken)
	}

	command.stdout.Reset()
	if status := runGenerateToken([]string{"--tokens-file", tokensFile, "bumble"}, command.runtime); status != 0 {
		t.Fatalf("named status = %d, stderr = %q", status, command.stderr.String())
	}
	namedToken := strings.TrimSpace(command.stdout.String())
	if !strings.HasPrefix(namedToken, "bumble.") {
		t.Fatalf("named output = %q", namedToken)
	}

	tokens, err := pages.ReadTokensFile(command.fileSystem, tokensFile)
	if err != nil {
		t.Fatalf("read tokens: %v", err)
	}
	if tokens.Token != defaultToken || "bumble."+tokens.Identities["bumble"] != namedToken {
		t.Fatalf("tokens = %+v", tokens)
	}
}

func TestGenerateTokenRequiresReplace(t *testing.T) {
	command := newTestCommand()
	args := []string{"--tokens-file", "/state/tokens.json"}
	if status := runGenerateToken(args, command.runtime); status != 0 {
		t.Fatalf("first status = %d", status)
	}
	if status := runGenerateToken(args, command.runtime); status != 1 {
		t.Fatalf("duplicate status = %d", status)
	}
	if status := runGenerateToken(append(args, "--replace"), command.runtime); status != 0 {
		t.Fatalf("replace status = %d", status)
	}
}

func TestGenerateTokenConfigDirectory(t *testing.T) {
	command := newTestCommand()
	if status := runGenerateToken([]string{"--config-dir", "/settings"}, command.runtime); status != 0 {
		t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
	}
	if _, err := command.fileSystem.Stat("/settings/tokens.json"); err != nil {
		t.Fatalf("tokens file: %v", err)
	}
}

func TestGenerateTokenExactFileOverridesConfigDirectory(t *testing.T) {
	command := newTestCommand()
	if status := runGenerateToken([]string{"--config-dir", "/settings", "--tokens-file", "/mounted/tokens.json"}, command.runtime); status != 0 {
		t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
	}
	if _, err := command.fileSystem.Stat("/mounted/tokens.json"); err != nil {
		t.Fatalf("exact tokens file: %v", err)
	}
	if _, err := command.fileSystem.Stat("/settings/tokens.json"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("configuration directory tokens file error = %v", err)
	}
}

func TestPublishConfigDirectory(t *testing.T) {
	command := newTestCommand()
	command.writeFile(t, "/source/report.html", "<h1>ready</h1>")
	command.writeFile(t, "/settings/config.json", `{"destination":"/public"}`)
	status := runPublish([]string{"--config-dir", "/settings", "--file", "/source/report.html", "--slug", "report"}, command.runtime)
	if status != 0 {
		t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
	}
	if _, err := command.fileSystem.Stat("/public/report/index.html"); err != nil {
		t.Fatalf("published page: %v", err)
	}
}

func TestPublishExactConfigSkipsUserConfigDir(t *testing.T) {
	command := newTestCommand()
	command.runtime.userConfigDir = func() (string, error) { return "", errors.New("must not be called") }
	command.writeFile(t, "/source/report.html", "<h1>ready</h1>")
	command.writeFile(t, "/settings/config.json", `{"destination":"/public"}`)
	status := runPublish([]string{"--config", "/settings/config.json", "--file", "/source/report.html", "--slug", "report"}, command.runtime)
	if status != 0 || command.stderr.Len() != 0 {
		t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
	}
	if _, err := command.fileSystem.Stat("/public/report/index.html"); err != nil {
		t.Fatalf("published page: %v", err)
	}
}

func TestPublishDestinationFlags(t *testing.T) {
	for _, flagName := range []string{"--destination", "--dest"} {
		t.Run(flagName, func(t *testing.T) {
			command := newTestCommand()
			command.writeFile(t, "/source/report.html", "<h1>ready</h1>")
			status := runPublish([]string{flagName, "/public", "--file", "/source/report.html", "--slug", "report"}, command.runtime)
			if status != 0 || command.stderr.Len() != 0 {
				t.Fatalf("status = %d, stdout = %q, stderr = %q", status, command.stdout.String(), command.stderr.String())
			}
			if _, err := command.fileSystem.Stat("/public/report/index.html"); err != nil {
				t.Fatalf("published page: %v", err)
			}
		})
	}
}

func TestPublishDestinationEnvironmentOverridesConfig(t *testing.T) {
	command := newTestCommand()
	command.writeFile(t, "/source/report.html", "<h1>ready</h1>")
	command.writeFile(t, "/settings/config.json", `{"destination":"/config"}`)
	command.environment["PAGES_DESTINATION"] = "/environment"

	status := runPublish([]string{"--config", "/settings/config.json", "--file", "/source/report.html", "--slug", "report"}, command.runtime)
	if status != 0 {
		t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
	}
	if _, err := command.fileSystem.Stat("/environment/report/index.html"); err != nil {
		t.Fatalf("published page: %v", err)
	}
}

func TestPublishRemoteDestinationAliasUsesInjectedHTTP(t *testing.T) {
	command := newTestCommand()
	command.writeFile(t, "/source/report.html", "<h1>ready</h1>")
	command.environment["PAGES_UPLOAD_TOKEN"] = "secret"
	requests := 0
	command.runtime.httpClient = testDoer(func(request *http.Request) *http.Response {
		requests++
		if requests == 1 {
			if request.Method != http.MethodPost || request.URL.Path != "/report" {
				t.Errorf("upload request = %s %s", request.Method, request.URL.Path)
			}
			return cliHTTPResponse(http.StatusNoContent, "")
		}
		if request.Method != http.MethodGet || request.URL.Path != "/report/" {
			t.Errorf("verification request = %s %s", request.Method, request.URL.Path)
		}
		return cliHTTPResponse(http.StatusOK, "text/html")
	})

	status := runPublish([]string{"--dest", "https://pages.example.com", "--file", "/source/report.html", "--slug", "report"}, command.runtime)
	if status != 0 || requests != 2 || strings.TrimSpace(command.stdout.String()) != "https://pages.example.com/report/" {
		t.Fatalf("status = %d, requests = %d, stdout = %q, stderr = %q", status, requests, command.stdout.String(), command.stderr.String())
	}
}

type testDoer func(*http.Request) *http.Response

func (doer testDoer) Do(request *http.Request) (*http.Response, error) {
	return doer(request), nil
}

func cliHTTPResponse(status int, contentType string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     http.Header{"Content-Type": []string{contentType}},
		Body:       io.NopCloser(strings.NewReader("")),
	}
}

func TestDestinationAliasesCannotBeCombined(t *testing.T) {
	command := newTestCommand()
	status := runPublish([]string{"--destination", "/one", "--dest", "/two", "--file", "report.html", "--slug", "report"}, command.runtime)
	if status != 2 || !strings.Contains(command.stderr.String(), "must not be used together") {
		t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
	}
}

func TestPublishRejectsLegacyDestinationEnvironment(t *testing.T) {
	command := newTestCommand()
	command.environment["PAGES_REMOTE"] = "https://pages.example.com"
	status := runPublish([]string{"--file", "report.html", "--slug", "report"}, command.runtime)
	if status != 2 || !strings.Contains(command.stderr.String(), "use PAGES_DESTINATION") {
		t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
	}
}

func TestSubcommandHelpUsesStdoutAndShowsDefaults(t *testing.T) {
	command := newTestCommand()
	status := runPublish([]string{"--help"}, command.runtime)
	if status != 0 || command.stderr.Len() != 0 {
		t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
	}
	for _, text := range []string{"Usage: pages publish", "--timeout", "default 1m30s", "--destination", "--dest"} {
		if !strings.Contains(command.stdout.String(), text) {
			t.Fatalf("stdout does not contain %q:\n%s", text, command.stdout.String())
		}
	}
}

func TestSubcommandRejectsSingleDashFlagAlias(t *testing.T) {
	command := newTestCommand()
	status := runPublish([]string{"-file", "page.html"}, command.runtime)
	if status != 2 || !strings.Contains(command.stderr.String(), "use --file instead of -file") {
		t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
	}
}

func TestSubcommandErrorsStayOnStderrWhenHelpAppearsLater(t *testing.T) {
	command := newTestCommand()
	status := runPublish([]string{"--bad", "--help"}, command.runtime)
	if status != 2 || command.stdout.Len() != 0 || !strings.Contains(command.stderr.String(), "flag provided but not defined") {
		t.Fatalf("status = %d, stdout = %q, stderr = %q", status, command.stdout.String(), command.stderr.String())
	}
}

func TestPublishAndServeRejectPositionalArguments(t *testing.T) {
	for _, name := range []string{"publish", "serve"} {
		t.Run(name, func(t *testing.T) {
			command := newTestCommand()
			status := runPublish([]string{"extra"}, command.runtime)
			if name == "serve" {
				status = runServe([]string{"extra"}, command.runtime)
			}
			if status != 2 || !strings.Contains(command.stderr.String(), "positional arguments are not allowed") {
				t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
			}
		})
	}
}
