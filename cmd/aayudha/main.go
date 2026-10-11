// Command aayudha (alias aayu) is the agentic-ayudha binary. This is the WP0.2
// stub; the cobra CLI frame replaces it in WP1.1.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// version is set at build time with -ldflags "-X main.version=<v>".
var version = "0.0.0-dev"

func main() {
	os.Exit(run(os.Args, os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	bin := "aayudha"
	if len(args) > 0 {
		bin = strings.TrimSuffix(filepath.Base(args[0]), ".exe")
	}
	if len(args) < 2 {
		usage(stdout, bin)
		return 0
	}
	switch cmd := args[1]; cmd {
	case "help", "-h", "--help":
		usage(stdout, bin)
		return 0
	case "version":
		fmt.Fprintf(stdout, "%s %s\n", bin, version)
		return 0
	default:
		if c, ok := verbs.Find(cmd); ok {
			fmt.Fprintf(stderr, "%s %s: not implemented yet (lands in %s)\n", bin, cmd, c.WP)
		} else {
			fmt.Fprintf(stderr, "%s %s: not implemented yet (unknown command; run '%s help')\n", bin, cmd, bin)
		}
		return 2
	}
}

func usage(w io.Writer, bin string) {
	width := 0
	for _, c := range append(verbs.Verbs[:len(verbs.Verbs):len(verbs.Verbs)], verbs.Plumbing...) {
		width = max(width, len(c.Name))
	}
	fmt.Fprintf(w, "Usage: %s <command> [arguments]\n\nVerbs:\n", bin)
	list(w, width, verbs.Verbs)
	fmt.Fprintf(w, "\nPlumbing commands:\n")
	list(w, width, verbs.Plumbing)
}

func list(w io.Writer, width int, cs []verbs.Command) {
	for _, c := range cs {
		fmt.Fprintf(w, "  %-*s  %s\n", width, c.Name, c.Summary)
	}
}
