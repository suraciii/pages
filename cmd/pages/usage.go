package main

import (
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
	fmt.Fprintln(writer, "Run \"pages <command> -h\" for command flags.")
}
