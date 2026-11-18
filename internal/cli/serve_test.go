package cli

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/suraciii/pages/internal/filesystem"
)

func TestResolveConfigDirDefault(t *testing.T) {
	command := newTestCommand()
	wantDir := filepath.Join("/config", ".pages")
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
	status := runServe([]string{"--dest", "https://pages.example.com"}, command.runtime)
	if status != 2 || !strings.Contains(command.stderr.String(), "destination must be a local path") || !strings.Contains(command.stderr.String(), "Contract: design/cli.md") {
		t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
	}
}

func TestServeUsesDestinationEnvironment(t *testing.T) {
	command := newTestCommand()
	command.environment["PAGES_DESTINATION"] = "https://pages.example.com"
	status := runServe(nil, command.runtime)
	if status != 2 || !strings.Contains(command.stderr.String(), "destination must be a local path") {
		t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
	}
}

func TestServeDestinationFlagOverridesEnvironment(t *testing.T) {
	command := newTestCommand()
	command.environment["PAGES_DESTINATION"] = "https://pages.example.com"
	status := runServe([]string{"--dest", "/public", "--tokens-file", "/missing/tokens.json"}, command.runtime)
	if status != 1 || !strings.Contains(command.stderr.String(), "destination=/public") || !strings.Contains(command.stderr.String(), "test bind failure") {
		t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
	}
	if strings.Contains(command.stderr.String(), "destination must be a local path") {
		t.Fatalf("remote destination error with local flag: %q", command.stderr.String())
	}
}

func TestServeDefaultDestinationAndReadOnlyWarnings(t *testing.T) {
	command := newTestCommand()
	status := runServe([]string{"--tokens-file", "/missing/tokens.json"}, command.runtime)
	if status != 1 {
		t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
	}
	for _, want := range []string{
		"warning: no Tokens loaded; remote Uploads are disabled",
		"listen=127.0.0.1:3103",
		"destination=/workspace/pages-public",
		"tokens-file=/missing/tokens.json",
		"max-upload-bytes=10485760",
		"tokens=0",
		"tls=off",
		"csp=default",
		"test bind failure",
	} {
		if !strings.Contains(command.stderr.String(), want) {
			t.Fatalf("stderr does not contain %q:\n%s", want, command.stderr.String())
		}
	}
	if strings.Contains(command.stderr.String(), "plaintext") {
		t.Fatalf("loopback listener warned about plaintext: %q", command.stderr.String())
	}
}

func TestServeNonLoopbackWarnsPlaintext(t *testing.T) {
	command := newTestCommand()
	status := runServe([]string{"--tokens-file", "/missing/tokens.json", "--listen", ":443"}, command.runtime)
	if status != 1 || !strings.Contains(command.stderr.String(), "not loopback; Tokens and Page content travel in plaintext") {
		t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
	}
}

func TestServeIPv6LoopbackDoesNotWarnPlaintext(t *testing.T) {
	command := newTestCommand()
	status := runServe([]string{"--tokens-file", "/missing/tokens.json", "--listen", "[::1]:3103"}, command.runtime)
	if status != 1 || strings.Contains(command.stderr.String(), "plaintext") {
		t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
	}
}

func TestServeRejectsInvalidListenAddress(t *testing.T) {
	for _, address := range []string{
		"",
		"127.0.0.1",
		"127.0.0.1:0",
		"127.0.0.1:http",
		"127.0.0.1:65536",
		"unix:/tmp/pages.sock",
		"http://127.0.0.1:3103",
		"::1:3103",
		"[::1]",
	} {
		t.Run(address, func(t *testing.T) {
			command := newTestCommand()
			status := runServe([]string{"--listen", address, "--tokens-file", "/missing/tokens.json"}, command.runtime)
			if status != 2 || !strings.Contains(command.stderr.String(), "listen") {
				t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
			}
		})
	}
}

func TestServeRejectsIncompleteTLS(t *testing.T) {
	for name, args := range map[string][]string{
		"certificate only": {"--tls-cert", "/cert.pem"},
		"key only":         {"--tls-key", "/key.pem"},
		"empty pair":       {"--tls-cert", "", "--tls-key", ""},
	} {
		t.Run(name, func(t *testing.T) {
			command := newTestCommand()
			status := runServe(append(args, "--tokens-file", "/missing/tokens.json"), command.runtime)
			if status != 2 || !strings.Contains(command.stderr.String(), "--tls") {
				t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
			}
		})
	}

	command := newTestCommand()
	command.environment["PAGES_TLS_CERT"] = "/cert.pem"
	status := runServe([]string{"--tokens-file", "/missing/tokens.json"}, command.runtime)
	if status != 2 || !strings.Contains(command.stderr.String(), "--tls-cert and --tls-key must be set together") {
		t.Fatalf("environment pair status = %d, stderr = %q", status, command.stderr.String())
	}
}

func TestServeRejectsMalformedEnvironmentValues(t *testing.T) {
	for name, testCase := range map[string]struct {
		environment string
		value       string
		want        string
	}{
		"upload limit": {"PAGES_MAX_UPLOAD_BYTES", "nope", "PAGES_MAX_UPLOAD_BYTES"},
		"listen":       {"PAGES_LISTEN_ADDR", "bad", "listen"},
		"policy":       {"PAGES_CSP", "default-src 'none'\nscript-src *", "one line"},
	} {
		t.Run(name, func(t *testing.T) {
			command := newTestCommand()
			command.environment[testCase.environment] = testCase.value
			status := runServe([]string{"--tokens-file", "/missing/tokens.json"}, command.runtime)
			if status != 2 || !strings.Contains(command.stderr.String(), testCase.want) {
				t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
			}
		})
	}
}

func TestServeFlagsOverrideMalformedEnvironment(t *testing.T) {
	command := newTestCommand()
	command.environment["PAGES_MAX_UPLOAD_BYTES"] = "nope"
	command.environment["PAGES_LISTEN_ADDR"] = "bad"
	command.environment["PAGES_CSP"] = "default-src 'none'\nscript-src *"
	status := runServe([]string{
		"--tokens-file", "/missing/tokens.json",
		"--listen", "127.0.0.1:3103",
		"--max-upload-bytes", "1024",
		"--content-security-policy", "default-src 'none'",
	}, command.runtime)
	if status != 1 || strings.Contains(command.stderr.String(), "PAGES_MAX_UPLOAD_BYTES") || strings.Contains(command.stderr.String(), "one line") {
		t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
	}
	if !strings.Contains(command.stderr.String(), "csp=custom") {
		t.Fatalf("custom CSP was not reported: %q", command.stderr.String())
	}
}

func TestServeRejectsInvalidContentSecurityPolicy(t *testing.T) {
	for name, value := range map[string]string{
		"empty":      "",
		"line break": "default-src 'none'\nscript-src *",
		"carriage":   "default-src 'none'\rscript-src *",
	} {
		t.Run(name, func(t *testing.T) {
			command := newTestCommand()
			status := runServe([]string{"--content-security-policy", value, "--tokens-file", "/missing/tokens.json"}, command.runtime)
			if status != 2 || !strings.Contains(command.stderr.String(), "policy") {
				t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
			}
		})
	}
}

func TestServeRejectsCredentialsInsidePublicRoot(t *testing.T) {
	for name, args := range map[string][]string{
		"tokens file": {"--tokens-file", "/public/tokens.json"},
		"certificate": {"--tokens-file", "/tokens.json", "--tls-cert", "/public/cert.pem", "--tls-key", "/tls/key.pem"},
		"private key": {"--tokens-file", "/tokens.json", "--tls-cert", "/tls/cert.pem", "--tls-key", "/public/key.pem"},
		"public root": {"--tokens-file", "/public"},
	} {
		t.Run(name, func(t *testing.T) {
			command := newTestCommand()
			args = append(args, "--destination", "/public")
			status := runServe(args, command.runtime)
			if status != 1 || !strings.Contains(command.stderr.String(), "inside the Public Root") {
				t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
			}
		})
	}
}

func TestServeEmptyDestinationUsesDefaultOverEnvironment(t *testing.T) {
	command := newTestCommand()
	command.environment["PAGES_DESTINATION"] = "https://pages.example.com"
	status := runServe([]string{"--destination", "", "--tokens-file", "/missing/tokens.json"}, command.runtime)
	if status != 1 || !strings.Contains(command.stderr.String(), "destination=/workspace/pages-public") {
		t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
	}
	if command.stdout.Len() != 0 {
		t.Fatalf("stdout = %q", command.stdout.String())
	}
}

func TestServeRejectsNonPositiveUploadLimit(t *testing.T) {
	for name, args := range map[string][]string{
		"flag zero":        {"--max-upload-bytes", "0"},
		"flag negative":    {"--max-upload-bytes", "-1"},
		"environment zero": {},
	} {
		t.Run(name, func(t *testing.T) {
			command := newTestCommand()
			if name == "environment zero" {
				command.environment["PAGES_MAX_UPLOAD_BYTES"] = "0"
			}
			status := runServe(append(args, "--tokens-file", "/missing/tokens.json"), command.runtime)
			if status != 2 || !strings.Contains(strings.ToLower(command.stderr.String()), "upload") {
				t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
			}
		})
	}
}

func TestServeLoadsSuccessfulTLSAndStarts(t *testing.T) {
	command := newTestCommand()
	certificate, key := testCertificate(t, "localhost")
	command.writeFile(t, "/tls/cert.pem", string(certificate))
	command.writeFile(t, "/tls/key.pem", string(key))
	status := runServe([]string{
		"--tokens-file", "/missing/tokens.json",
		"--tls-cert", "/tls/cert.pem",
		"--tls-key", "/tls/key.pem",
	}, command.runtime)
	if status != 1 || !strings.Contains(command.stderr.String(), "tls=on") || !strings.Contains(command.stderr.String(), "test bind failure") {
		t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
	}
	if strings.Contains(command.stderr.String(), "plaintext") {
		t.Fatalf("TLS listener warned about plaintext: %q", command.stderr.String())
	}
}

func TestServeMissingCertificateFileFails(t *testing.T) {
	command := newTestCommand()
	status := runServe([]string{
		"--tokens-file", "/missing/tokens.json",
		"--tls-cert", "/missing/cert.pem",
		"--tls-key", "/missing/key.pem",
	}, command.runtime)
	if status != 1 || !strings.Contains(command.stderr.String(), "read certificate") {
		t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
	}
	if strings.Contains(command.stderr.String(), "listen=") {
		t.Fatalf("startup line appeared before the certificate failure: %q", command.stderr.String())
	}
}

type unreadableFileSystem struct {
	filesystem.FS
	path string
}

func (fileSystem unreadableFileSystem) Open(path string) (filesystem.File, error) {
	if path == fileSystem.path {
		return nil, fs.ErrPermission
	}
	return fileSystem.FS.Open(path)
}

func TestServeRejectsCredentialsThroughSymlinkedPublicRoot(t *testing.T) {
	command := newTestCommand()
	fileSystem := filesystem.NewMemory("/workspace")
	if err := fileSystem.MkdirAll("/real-root", 0o755); err != nil {
		t.Fatalf("create real root: %v", err)
	}
	command.runtime.fileSystem = evalSymlinkFileSystem{
		FS:    fileSystem,
		links: map[string]string{"/link-root": "/real-root"},
	}
	status := runServe([]string{"--destination", "/link-root", "--tokens-file", "/real-root/tokens.json"}, command.runtime)
	if status != 1 || !strings.Contains(command.stderr.String(), "inside the Public Root") {
		t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
	}
}

func TestServeTokensFileStates(t *testing.T) {
	t.Run("malformed", func(t *testing.T) {
		command := newTestCommand()
		command.writeFile(t, "/tokens.json", "not json")
		status := runServe([]string{"--tokens-file", "/tokens.json"}, command.runtime)
		if status != 1 || !strings.Contains(command.stderr.String(), "parse tokens file") {
			t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
		}
	})

	t.Run("empty", func(t *testing.T) {
		command := newTestCommand()
		command.writeFile(t, "/tokens.json", "")
		status := runServe([]string{"--tokens-file", "/tokens.json"}, command.runtime)
		if status != 1 || !strings.Contains(command.stderr.String(), "tokens=0") {
			t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
		}
	})

	t.Run("configured", func(t *testing.T) {
		command := newTestCommand()
		command.writeFile(t, "/tokens.json", `{"token":"default-secret","identities":{"bumble":"secret"}}`)
		status := runServe([]string{"--tokens-file", "/tokens.json"}, command.runtime)
		if status != 1 || !strings.Contains(command.stderr.String(), "tokens=2") || strings.Contains(command.stderr.String(), "remote Uploads are disabled") {
			t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
		}
		for _, secret := range []string{"default-secret", "secret"} {
			if strings.Contains(command.stderr.String(), secret) {
				t.Fatalf("startup output leaked a Token: %q", command.stderr.String())
			}
		}
	})
}

func TestServeRejectsUnreadableTokensFile(t *testing.T) {
	command := newTestCommand()
	command.writeFile(t, "/tokens.json", `{"token":"secret"}`)
	command.runtime.fileSystem = unreadableFileSystem{
		FS:   command.fileSystem,
		path: "/tokens.json",
	}
	status := runServe([]string{"--tokens-file", "/tokens.json"}, command.runtime)
	if status != 1 || !strings.Contains(command.stderr.String(), "read tokens file") {
		t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
	}
}

func TestServeRejectsPublicRootCreationFailure(t *testing.T) {
	command := newTestCommand()
	command.writeFile(t, "/public", "not a directory")
	status := runServe([]string{"--destination", "/public", "--tokens-file", "/missing/tokens.json"}, command.runtime)
	if status != 1 || !strings.Contains(command.stderr.String(), "create public root") {
		t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
	}
}

func TestServeLoadsInvalidCertificatePair(t *testing.T) {
	command := newTestCommand()
	command.writeFile(t, "/tls/cert.pem", "not a certificate")
	command.writeFile(t, "/tls/key.pem", "not a key")
	status := runServe([]string{
		"--tokens-file", "/missing/tokens.json",
		"--tls-cert", "/tls/cert.pem",
		"--tls-key", "/tls/key.pem",
	}, command.runtime)
	if status != 1 || !strings.Contains(command.stderr.String(), "load certificate pair") {
		t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
	}
}

func TestServeHelpDocumentsTheService(t *testing.T) {
	command := newTestCommand()
	status := runServe([]string{"--help"}, command.runtime)
	if status != 0 || command.stderr.Len() != 0 {
		t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
	}
	for _, want := range []string{
		"Usage: pages serve",
		"--tls-cert",
		"--content-security-policy",
		"/__pages/healthz",
		"pages-public/",
		"read-only",
		"Contract: design/cli.md",
		"revision 2026-11-18",
	} {
		if !strings.Contains(command.stdout.String(), want) {
			t.Fatalf("stdout does not contain %q:\n%s", want, command.stdout.String())
		}
	}
}

func TestServeDestinationAliasesReachStartup(t *testing.T) {
	for _, flagName := range []string{"--destination", "--dest"} {
		t.Run(flagName, func(t *testing.T) {
			command := newTestCommand()
			status := runServe([]string{flagName, "/public", "--tokens-file", "/missing/tokens.json"}, command.runtime)
			if status != 1 || !strings.Contains(command.stderr.String(), "destination=/public") || strings.Contains(command.stderr.String(), "tokens file must contain") {
				t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
			}
		})
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
			status := runGenerateToken(args, command.runtime)
			if name == "serve" {
				status = runServe(args, command.runtime)
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
	status := runPublish([]string{"--file", "report.html", "--slug", "report"}, command.runtime)
	if status != 0 || command.stderr.Len() != 0 {
		t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
	}
	workDir, err := command.runtime.currentDirectory()
	if err != nil {
		t.Fatalf("resolve work directory: %v", err)
	}
	if _, err := command.fileSystem.Stat(filepath.Join(workDir, "pages-public", "report", "index.html")); err != nil {
		t.Fatalf("published page: %v", err)
	}
	if _, err := command.fileSystem.Stat("/wrong/report/index.html"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("current-directory config was used: %v", err)
	}
}

func TestExplicitTokenFileSkipsUserConfigDir(t *testing.T) {
	command := newTestCommand()
	command.runtime.userConfigDir = func() (string, error) { return "", errors.New("must not be called") }
	if status := runGenerateToken([]string{"--tokens-file", "/mounted/tokens.json"}, command.runtime); status != 0 {
		t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
	}
	if _, err := command.fileSystem.Stat("/mounted/tokens.json"); err != nil {
		t.Fatalf("tokens file: %v", err)
	}
}

func TestServeExactTokenFileSkipsUserConfigDir(t *testing.T) {
	command := newTestCommand()
	command.runtime.userConfigDir = func() (string, error) { return "", errors.New("must not be called") }
	status := runServe([]string{"--tokens-file", "/missing/tokens.json"}, command.runtime)
	if status != 1 || strings.Contains(command.stderr.String(), "user config directory") || !strings.Contains(command.stderr.String(), "tokens=0") {
		t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
	}
}

func TestConfigDirectoryEnvironmentSkipsUserConfigDir(t *testing.T) {
	command := newTestCommand()
	command.environment["PAGES_CONFIG_DIR"] = "/mounted/pages-config"
	command.runtime.userConfigDir = func() (string, error) { return "", errors.New("must not be called") }
	if status := runGenerateToken(nil, command.runtime); status != 0 {
		t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
	}
	if _, err := command.fileSystem.Stat("/mounted/pages-config/tokens.json"); err != nil {
		t.Fatalf("tokens file: %v", err)
	}
}
