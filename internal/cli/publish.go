package cli

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"time"

	"github.com/suraciii/pages/internal/publish"
)

func runPublish(args []string) int {
	return runPublishWithRuntime(args, productionCommandRuntime())
}

func runPublishWithRuntime(args []string, runtime commandRuntime) int {
	flags := flag.NewFlagSet("pages publish", flag.ContinueOnError)
	flags.Usage = subcommandUsage(flags, "Usage: pages publish [flags]")
	filePath := flags.String("file", "", "HTML or zip file to publish (required)")
	slug := flags.String("slug", "", "page slug (required)")
	destinationValue := destinationFlags(flags, runtime.environment)
	identity := flags.String("identity", "", "named identity; default identity when empty")
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

	resolved, err := publish.ResolveWithRuntime(publish.Input{
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

	result, err := publish.RunWithRuntime(resolved, runtime.publishRuntime())
	if err != nil {
		fmt.Fprintf(runtime.stderr, "pages publish: %v\n", err)
		return 1
	}
	fmt.Fprintln(runtime.stdout, result)
	return 0
}
