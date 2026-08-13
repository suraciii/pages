package cli

import (
	"flag"
	"fmt"

	"github.com/suraciii/pages/internal/destination"
)

func destinationFlags(flags *flag.FlagSet) *string {
	value := ""
	flags.StringVar(&value, "destination", "", "local path or HTTP(S) URL (default current directory)")
	flags.StringVar(&value, "dest", "", "alias for --destination")
	return &value
}

func parseDestinationFlags(flags *flag.FlagSet, value string, localOnly bool, currentDirectory func() (string, error)) (destination.Value, error) {
	if err := rejectDestinationAliasConflict(flags); err != nil {
		return destination.Value{}, err
	}
	resolved, err := destination.Parse(value, currentDirectory)
	if err != nil {
		return destination.Value{}, err
	}
	if localOnly && resolved.IsRemote() {
		return destination.Value{}, fmt.Errorf("destination must be a local path")
	}
	return resolved, nil
}

func rejectDestinationAliasConflict(flags *flag.FlagSet) error {
	if flagWasSet(flags, "destination") && flagWasSet(flags, "dest") {
		return fmt.Errorf("--destination and --dest must not be used together")
	}
	return nil
}

func legacyDestinationEnvironment(environment func(string) string) (string, bool) {
	for _, name := range []string{"PAGES_REMOTE", "PAGES_PUBLIC_ROOT"} {
		if environment(name) != "" {
			return name, true
		}
	}
	return "", false
}
