package cli

import (
	_ "embed"
	"fmt"
	"io"
)

//go:embed skill/SKILL.md
var pagesSkill string

func runSkill(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = io.WriteString(stdout, pagesSkill)
		return 0
	}
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		printSkillUsage(stdout)
		return 0
	}
	fmt.Fprintln(stderr, "pages skill: arguments are not allowed")
	printSkillUsage(stderr)
	return 2
}

func printSkillUsage(writer io.Writer) {
	fmt.Fprintln(writer, "Usage: pages skill")
}
