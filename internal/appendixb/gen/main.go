// Command gen checks Appendix B's rows under docs/appendix-b/ and keeps the
// generated end of docs/BUILD-PLAN.md (internal/appendixb). It is run from the
// repository root by `go generate ./...` through the directive in embed.go.
// With -print it prints the whole appendix as tables instead.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/rahulnakmol/agentic-ayudha/internal/appendixb"
)

func main() {
	printTable := flag.Bool("print", false, "print Appendix B as tables, rendered from "+appendixb.Dir)
	flag.Parse()
	if err := run(*printTable); err != nil {
		fmt.Fprintln(os.Stderr, "appendixb gen:", err)
		os.Exit(1)
	}
}

func run(printTable bool) error {
	if printTable {
		secs, err := appendixb.Load(".")
		if err != nil {
			return err
		}
		_, err = fmt.Print(appendixb.Render(secs))
		return err
	}
	return appendixb.Generate(".", os.Stdout, os.Getenv("GITHUB_ACTIONS") == "true")
}
