package cli

import (
	"fmt"
)

func Run(args []string) int {
	return run(args, productionCommandRuntime())
}

func run(args []string, runtime commandRuntime) int {
	if len(args) == 0 {
		printUsage(runtime.stderr)
		return 2
	}
	switch args[0] {
	case "--help", "-h":
		printUsage(runtime.stdout)
		return 0
	case "serve":
		return runServe(args[1:], runtime)
	case "publish":
		return runPublish(args[1:], runtime)
	case "generate-token":
		return runGenerateToken(args[1:], runtime)
	default:
		fmt.Fprintf(runtime.stderr, "pages: unknown command %q\n", args[0])
		printUsage(runtime.stderr)
		return 2
	}
}
