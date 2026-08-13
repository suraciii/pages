package cli

import (
	"flag"
	"fmt"
	"io"
)

func printUsage(writer io.Writer) {
	fmt.Fprintln(writer, "pages publishes static HTML pages to a public site.")
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "Usage:")
	fmt.Fprintln(writer, "  pages serve              run the publish service")
	fmt.Fprintln(writer, "  pages publish            publish one page")
	fmt.Fprintln(writer, "  pages generate-token <identity>")
	fmt.Fprintln(writer, "                           issue an upload token")
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "Run \"pages <command> --help\" for command flags.")
}

// subcommandUsage reports one subcommand line and its flags with the
// long-form double-dash style.
func subcommandUsage(flags *flag.FlagSet, format string, args ...any) func() {
	return func() {
		fmt.Fprintf(flags.Output(), format+"\n", args...)
		flags.VisitAll(func(item *flag.Flag) {
			fmt.Fprintf(flags.Output(), "  --%s  %s\n", item.Name, item.Usage)
		})
	}
}
