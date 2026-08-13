package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/suraciii/pages/internal/pages"
)

func runGenerateToken(args []string) int {
	flags := flag.NewFlagSet("pages generate-token", flag.ExitOnError)
	tokensFile := flags.String("tokens-file", envOr("PAGES_TOKENS_FILE", ""), "identity token JSON file (required)")
	replace := flags.Bool("replace", false, "replace an existing identity entry")
	flags.Parse(args)

	positional := flags.Args()
	if len(positional) != 1 {
		fmt.Fprintln(os.Stderr, "pages generate-token: exactly one identity argument is required")
		flags.Usage()
		return 2
	}
	identity := positional[0]
	if *tokensFile == "" {
		fmt.Fprintln(os.Stderr, "pages generate-token: -tokens-file is required")
		flags.Usage()
		return 2
	}
	if !pages.ValidName(identity) {
		fmt.Fprintf(os.Stderr, "pages generate-token: invalid identity %q\n", identity)
		return 2
	}

	tokens, err := pages.ReadTokensFile(*tokensFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pages generate-token: %v\n", err)
		return 1
	}
	if _, exists := tokens[identity]; exists && !*replace {
		fmt.Fprintf(os.Stderr, "pages generate-token: identity %q already exists; use -replace to rotate\n", identity)
		return 1
	}

	secret, err := pages.GenerateSecret()
	if err != nil {
		fmt.Fprintf(os.Stderr, "pages generate-token: %v\n", err)
		return 1
	}
	tokens[identity] = secret
	if err := pages.WriteTokensFile(*tokensFile, tokens); err != nil {
		fmt.Fprintf(os.Stderr, "pages generate-token: %v\n", err)
		return 1
	}

	fmt.Printf("%s.%s\n", identity, secret)
	return 0
}
