package main

import (
	"os"

	"github.com/suraciii/pages/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
