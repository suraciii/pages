package cli

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
)

func printUsage(writer io.Writer) {
	fmt.Fprintln(writer, "pages publishes static HTML pages to a public site.")
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "Usage:")
	fmt.Fprintln(writer, "  pages serve              run the publish service")
	fmt.Fprintln(writer, "  pages publish            publish one page")
	fmt.Fprintln(writer, "  pages generate-token [identity]")
	fmt.Fprintln(writer, "                           issue an upload token")
	fmt.Fprintln(writer, "  pages skill              print agent instructions")
	fmt.Fprintln(writer, "  pages --version          print version")
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "Run \"pages <command> --help\" for command flags.")
}

// subcommandUsage reports one subcommand line and its flags with the
// long-form double-dash style.
func subcommandUsage(flags *flag.FlagSet, format string, args ...any) func() {
	return func() {
		fmt.Fprintf(flags.Output(), format+"\n", args...)
		flags.VisitAll(func(item *flag.Flag) {
			defaultValue := ""
			if item.DefValue != "" {
				defaultValue = fmt.Sprintf(" (default %s)", item.DefValue)
			}
			fmt.Fprintf(flags.Output(), "  --%s  %s%s\n", item.Name, item.Usage, defaultValue)
		})
	}
}

func parseFlags(flags *flag.FlagSet, args []string, stdout, stderr io.Writer) (int, bool) {
	flags.SetOutput(stderr)
	if invalid := singleDashFlag(flags, args); invalid != "" {
		fmt.Fprintf(stderr, "%s: use --%s instead of -%s\n", flags.Name(), invalid, invalid)
		flags.Usage()
		return 2, false
	}
	var output bytes.Buffer
	flags.SetOutput(&output)
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, _ = stdout.Write(output.Bytes())
			return 0, false
		}
		_, _ = stderr.Write(output.Bytes())
		return 2, false
	}
	flags.SetOutput(stderr)
	return 0, true
}

func singleDashFlag(flags *flag.FlagSet, args []string) string {
	for index := 0; index < len(args); index++ {
		argument := args[index]
		if argument == "--" || !strings.HasPrefix(argument, "-") {
			return ""
		}
		if argument == "-h" {
			continue
		}
		if !strings.HasPrefix(argument, "--") {
			return strings.TrimPrefix(strings.SplitN(argument, "=", 2)[0], "-")
		}
		nameValue := strings.TrimPrefix(argument, "--")
		name, _, hasValue := strings.Cut(nameValue, "=")
		item := flags.Lookup(name)
		if hasValue || item == nil {
			continue
		}
		boolFlag, isBool := item.Value.(interface{ IsBoolFlag() bool })
		if !isBool || !boolFlag.IsBoolFlag() {
			index++
		}
	}
	return ""
}

func flagWasSet(flags *flag.FlagSet, name string) bool {
	set := false
	flags.Visit(func(item *flag.Flag) {
		if item.Name == name {
			set = true
		}
	})
	return set
}
