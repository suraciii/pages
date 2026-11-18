package cli

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"time"

	"github.com/suraciii/pages/internal/publish"
)

// publishUsageNotes is the method guidance an agent or operator reads from
// `pages publish --help`. It answers the four questions the command alone
// cannot: which mode to choose, what each mode needs, how to check the
// result, and what to do when a Token or Identity is missing. It states the
// contract revision it was checked against.
const publishUsageNotes = `
Mode
  A local --dest path writes into a Public Root you own. No Token, no
  Identity, no service, no network. An absolute HTTP(S) --dest URL
  uploads through the Service and verifies the public URL.

Recommended path
  Local, with the default pages-public/ Public Root:
    pages publish --file report.html --slug report
    <current-directory>/pages-public/report/
  Remote over HTTPS, with a Token in the process environment, never on the
  command line:
    PAGES_UPLOAD_TOKEN=bumble.sek pages publish --file report.html --slug report \
      --dest https://pages.example.com
    https://pages.example.com/@bumble/report/
  A loopback HTTP Destination is still remote mode and needs a Token. Use
  HTTPS for a non-loopback Destination.

Identity
  Local mode needs no Token and no Identity. It never reads
  PAGES_UPLOAD_TOKEN; --identity only chooses the output directory.
  Remote mode needs a Token. A Token with no dot selects the Default
  Identity, whose URL has no @ segment. A Token in identity.secret form
  selects that Named Identity by itself, so do not also pass --identity.
  When PAGES_UPLOAD_TOKEN is set it decides the scope, and --identity must
  not name a different one: that combination is a usage error that exits 2
  and publishes nothing.

Result check
  Exit 0 and exactly one printed line: the Page directory in local mode, the
  verified public URL in remote mode. Any other exit is a failure.

Recovery
  A usage error exits 2 and its message names the input to fix. Publishing
  the same Slug again replaces the Page, so a repeat after a timeout is safe.
  Two failures happen after the Page is already in place, and the Page stays
  live in both:
    verify page: ... the page is in place at <url>; check the Service or its proxy
    refresh page index: ... the page is in place at <dir>; move the foreign
      file at index.html or index.json, then publish again

  Run pages serve --help for the Service side: listener, TLS, and tokens.

Contract: design/cli.md is the authoritative command contract, revision ` + commandContractRevision + `.
This help was checked against it.
`

func runPublish(args []string, runtime commandRuntime) int {
	flags := flag.NewFlagSet("pages publish", flag.ContinueOnError)
	flags.Usage = subcommandUsage(flags, "Usage: pages publish [flags]", publishUsageNotes)
	filePath := flags.String("file", "", "HTML or zip file to publish (required)")
	slug := flags.String("slug", "", "page slug (required)")
	destinationValue := destinationFlags(flags, false)
	identity := flags.String("identity", "", "named identity; must match the identity in PAGES_UPLOAD_TOKEN when that is set")
	timeout := flags.Duration("timeout", 90*time.Second, "upload and verification timeout")
	configDir := flags.String("config-dir", runtime.environment("PAGES_CONFIG_DIR"), "configuration directory")
	configFlag := flags.String("config", "", "config file")
	if status, ok := parseFlags(flags, args, runtime.stdout, runtime.stderr); !ok {
		return status
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(runtime.stderr, "pages publish: positional arguments are not allowed")
		flags.Usage()
		return 2
	}
	if legacy, found := legacyDestinationEnvironment(runtime.environment); found {
		fmt.Fprintf(runtime.stderr, "pages publish: %s is not supported; use PAGES_DESTINATION\n", legacy)
		return 2
	}
	if err := rejectDestinationAliasConflict(flags); err != nil {
		fmt.Fprintf(runtime.stderr, "pages publish: %v\n", err)
		flags.Usage()
		return 2
	}

	configPath := *configFlag
	if !flagWasSet(flags, "config") {
		resolvedConfigDir, err := resolveConfigDir(runtime, *configDir)
		if err == nil {
			configPath = configFilePath(resolvedConfigDir)
			if _, err := runtime.fileSystem.Stat(configPath); errors.Is(err, fs.ErrNotExist) {
				configPath = ""
			}
		}
	}

	resolved, err := publish.Resolve(publish.Input{
		File:        *filePath,
		Slug:        *slug,
		Destination: *destinationValue,
		Identity:    *identity,
		Timeout:     *timeout,
		ConfigPath:  configPath,
	}, runtime.publishRuntime())
	if err != nil {
		if errors.Is(err, publish.ErrUsage) {
			fmt.Fprintf(runtime.stderr, "pages publish: %v\n", err)
			flags.Usage()
			return 2
		}
		fmt.Fprintf(runtime.stderr, "pages publish: %v\n", err)
		return 1
	}

	result, err := publish.Run(resolved, runtime.publishRuntime())
	if err != nil {
		if errors.Is(err, publish.ErrUsage) {
			// Run reports the body rules as usage errors: an empty file and
			// a file that is not UTF-8. They exit 2 like every other usage
			// error, not 1 like a failed step.
			fmt.Fprintf(runtime.stderr, "pages publish: %v\n", err)
			flags.Usage()
			return 2
		}
		fmt.Fprintf(runtime.stderr, "pages publish: %v\n", err)
		return 1
	}
	fmt.Fprintln(runtime.stdout, result)
	return 0
}
