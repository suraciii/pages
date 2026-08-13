package cli

import (
	"flag"
	"fmt"
	"os"

	"github.com/suraciii/pages/internal/pages"
)

func runGenerateToken(args []string) int {
	flags := flag.NewFlagSet("pages generate-token", flag.ContinueOnError)
	flags.Usage = subcommandUsage(flags, "Usage: pages generate-token [flags] [identity]")
	tokensFile := flags.String("tokens-file", resolvedTokensFile(), "identity token JSON file")
	replace := flags.Bool("replace", false, "replace an existing identity entry")
	if status, ok := parseFlags(flags, args); !ok {
		return status
	}

	positional := flags.Args()
	if len(positional) > 1 {
		fmt.Fprintln(os.Stderr, "pages generate-token: at most one identity argument is allowed")
		flags.Usage()
		return 2
	}
	identity := ""
	if len(positional) == 1 {
		identity = positional[0]
	}
	if identity != "" && !pages.ValidName(identity) {
		fmt.Fprintf(os.Stderr, "pages generate-token: invalid identity %q\n", identity)
		return 2
	}

	token, err := pages.IssueToken(*tokensFile, identity, *replace)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pages generate-token: %v\n", err)
		return 1
	}
	fmt.Println(token)
	return 0
}
