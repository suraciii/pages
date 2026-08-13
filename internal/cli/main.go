package cli

import (
	"fmt"
)

func Run(args []string) int {
	return runWithRuntime(args, productionCommandRuntime())
}

func runWithRuntime(args []string, runtime commandRuntime) int {
	if len(args) == 0 {
		printUsage(runtime.stderr)
		return 2
	}
	switch args[0] {
	case "--help", "-h":
		printUsage(runtime.stdout)
		return 0
	case "serve":
		return runServeWithRuntime(args[1:], runtime)
	case "publish":
		return runPublishWithRuntime(args[1:], runtime)
	case "generate-token":
		return runGenerateTokenWithRuntime(args[1:], runtime)
	default:
		fmt.Fprintf(runtime.stderr, "pages: unknown command %q\n", args[0])
		printUsage(runtime.stderr)
		return 2
	}
}
