package cli

import (
	"errors"
	"strings"
	"testing"
)

func TestSkillPrintsEmbeddedInstructions(t *testing.T) {
	command := newTestCommand()
	command.runtime.environment = func(string) string { panic("environment was read") }
	command.runtime.userConfigDir = func() (string, error) { return "", errors.New("user config directory was read") }
	command.runtime.currentDirectory = func() (string, error) { return "", errors.New("current directory was read") }

	status := run([]string{"skill"}, command.runtime)
	if status != 0 || command.stderr.Len() != 0 {
		t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
	}
	if command.stdout.String() != pagesSkill {
		t.Fatal("stdout does not match the embedded skill")
	}
	for _, text := range []string{"---\nname: pages\n", "# pages", "pages publish --file report.html --slug report"} {
		if !strings.Contains(command.stdout.String(), text) {
			t.Fatalf("stdout does not contain %q", text)
		}
	}
}

func TestSkillHelpUsesStdoutWithoutPrintingInstructions(t *testing.T) {
	for _, argument := range []string{"--help", "-h"} {
		t.Run(argument, func(t *testing.T) {
			command := newTestCommand()
			status := run([]string{"skill", argument}, command.runtime)
			if status != 0 || command.stderr.Len() != 0 || command.stdout.String() != "Usage: pages skill\n" {
				t.Fatalf("status = %d, stdout = %q, stderr = %q", status, command.stdout.String(), command.stderr.String())
			}
		})
	}
}

func TestSkillRejectsArguments(t *testing.T) {
	command := newTestCommand()
	status := run([]string{"skill", "extra"}, command.runtime)
	if status != 2 || command.stdout.Len() != 0 {
		t.Fatalf("status = %d, stdout = %q", status, command.stdout.String())
	}
	want := "pages skill: arguments are not allowed\nUsage: pages skill\n"
	if command.stderr.String() != want {
		t.Fatalf("stderr = %q, want %q", command.stderr.String(), want)
	}
}
