package cli

import (
	"strings"
	"testing"
)

// `pages publish --help` is the agent surface. It must carry the guidance an
// agent cannot derive from the flags: which mode to choose, what each mode
// requires, how to check the result, and the recovery for a missing Token or
// Identity. A regression here is a silent loss of the recovery path.
func TestPublishHelpCarriesAgentGuidance(t *testing.T) {
	command := newTestCommand()
	command.runtime.userConfigDir = func() (string, error) { return "", nil }
	status := run([]string{"publish", "--help"}, command.runtime)
	if status != 0 || command.stderr.Len() != 0 {
		t.Fatalf("status = %d, stderr = %q", status, command.stderr.String())
	}
	help := command.stdout.String()
	for _, want := range []string{
		// Which mode to choose, and what each mode needs.
		"Mode",
		"No Token, no\n  Identity, no service, no network",
		"PAGES_UPLOAD_TOKEN",
		// The recommended path for each mode.
		"Recommended path",
		"pages publish --file report.html --slug report --dest public",
		// How to check the result.
		"Result check",
		"Exit 0 and exactly one printed line",
		// The recovery for a missing Token or Identity.
		"Recovery",
		"A usage error exits 2",
		// The Identity rules, including the trap.
		"Identity",
		"identity.secret form\n  selects that Named Identity by itself",
		"must\n  not name a different one",
		// The two partial-effect failures.
		"the page is in place at <url>",
		"the page is in place at <dir>",
		// The flags themselves.
		"--file",
		"--slug",
		"--identity",
		// A pointer to the authoritative contract.
		"design/cli.md is the authoritative command contract",
	} {
		if !strings.Contains(help, want) {
			t.Errorf("help is missing %q", want)
		}
	}
}

// The guidance must reach stderr with a usage error, because an agent that
// got the command wrong reads the error, not a separate help run.
func TestPublishUsageErrorCarriesAgentGuidance(t *testing.T) {
	command := newTestCommand()
	command.runtime.userConfigDir = func() (string, error) { return "", nil }
	status := run([]string{"publish", "--slug", "report"}, command.runtime)
	if status != 2 {
		t.Fatalf("status = %d, want 2", status)
	}
	if !strings.Contains(command.stderr.String(), "Result check") {
		t.Fatalf("stderr = %q, want the agent guidance", command.stderr.String())
	}
}
