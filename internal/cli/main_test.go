package cli

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestRootHelp(t *testing.T) {
	for _, argument := range []string{"--help", "-h"} {
		t.Run(argument, func(t *testing.T) {
			command := newTestCommand()
			command.runtime.userConfigDir = func() (string, error) { return "", errors.New("must not be called") }
			status := runWithRuntime([]string{argument}, command.runtime)
			if status != 0 || command.stderr.Len() != 0 || !strings.Contains(command.stdout.String(), "\nUsage:\n") {
				t.Fatalf("status = %d, stdout = %q, stderr = %q", status, command.stdout.String(), command.stderr.String())
			}
		})
	}
}

func TestRootUsageErrors(t *testing.T) {
	for name, args := range map[string][]string{
		"missing command": nil,
		"unknown command": {"unknown"},
	} {
		t.Run(name, func(t *testing.T) {
			command := newTestCommand()
			status := runWithRuntime(args, command.runtime)
			if status != 2 || command.stdout.Len() != 0 || !strings.Contains(command.stderr.String(), "\nUsage:\n") {
				t.Fatalf("status = %d, stdout = %q, stderr = %q", status, command.stdout.String(), command.stderr.String())
			}
		})
	}
}

func TestRootDispatchesMinimalLocalPublish(t *testing.T) {
	command := newTestCommand()
	command.writeFile(t, "report.html", "<h1>ready</h1>")
	status := runWithRuntime([]string{"publish", "--file", "report.html", "--slug", "report"}, command.runtime)
	workDir, err := command.runtime.currentDirectory()
	if err != nil {
		t.Fatalf("resolve work directory: %v", err)
	}
	wantOutput := filepath.Join(workDir, "report") + string(filepath.Separator)
	if status != 0 || command.stderr.Len() != 0 || strings.TrimSpace(command.stdout.String()) != wantOutput {
		t.Fatalf("status = %d, stdout = %q, stderr = %q", status, command.stdout.String(), command.stderr.String())
	}
	contents, err := command.fileSystem.ReadFile(filepath.Join(workDir, "report", "index.html"))
	if err != nil || string(contents) != "<h1>ready</h1>" {
		t.Fatalf("page = %q, %v", contents, err)
	}
	if _, err := command.fileSystem.Stat(filepath.Join(workDir, "config.json")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("current-directory config error = %v", err)
	}
}
