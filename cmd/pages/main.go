package main

import (
	"fmt"
	"os"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		printUsage(os.Stderr)
		return 2
	}
	switch args[0] {
	case "--help", "-h":
		printUsage(os.Stdout)
		return 0
	case "serve":
		return runServe(args[1:])
	case "publish":
		return runPublish(args[1:])
	case "generate-token":
		return runGenerateToken(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "pages: unknown command %q\n", args[0])
		printUsage(os.Stderr)
		return 2
	}
}
