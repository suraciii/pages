package cli

import (
	"flag"
	"fmt"

	"github.com/suraciii/pages/internal/pages"
)

func runGenerateToken(args []string) int {
	return runGenerateTokenWithRuntime(args, productionCommandRuntime())
}

func runGenerateTokenWithRuntime(args []string, runtime commandRuntime) int {
	flags := flag.NewFlagSet("pages generate-token", flag.ContinueOnError)
	flags.Usage = subcommandUsage(flags, "Usage: pages generate-token [flags] [identity]")
	configDir := flags.String("config-dir", runtime.environment("PAGES_CONFIG_DIR"), "configuration directory")
	tokensFile := flags.String("tokens-file", runtime.environment("PAGES_TOKENS_FILE"), "identity token JSON file")
	replace := flags.Bool("replace", false, "replace an existing identity entry")
	if status, ok := parseFlags(flags, args, runtime.stdout, runtime.stderr); !ok {
		return status
	}

	positional := flags.Args()
	if len(positional) > 1 {
		fmt.Fprintln(runtime.stderr, "pages generate-token: at most one identity argument is allowed")
		flags.Usage()
		return 2
	}
	identity := ""
	if len(positional) == 1 {
		identity = positional[0]
	}
	if identity != "" && !pages.ValidName(identity) {
		fmt.Fprintf(runtime.stderr, "pages generate-token: invalid identity %q\n", identity)
		return 2
	}
	if *tokensFile == "" {
		resolvedConfigDir, err := resolveConfigDir(runtime, *configDir)
		if err != nil {
			fmt.Fprintf(runtime.stderr, "pages generate-token: %v; use --config-dir or --tokens-file\n", err)
			return 1
		}
		*tokensFile = configFilePathNamed(resolvedConfigDir, "tokens.json")
	}

	token, err := pages.IssueTokenWithFS(runtime.fileSystem, *tokensFile, identity, *replace)
	if err != nil {
		fmt.Fprintf(runtime.stderr, "pages generate-token: %v\n", err)
		return 1
	}
	fmt.Fprintln(runtime.stdout, token)
	return 0
}
