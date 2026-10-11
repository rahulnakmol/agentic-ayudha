// Command aayudha (alias aayu) is the agentic-ayudha binary. All behaviour lives
// in internal/cli; this is the only place that touches the process's standard
// streams and exit status.
package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/rahulnakmol/agentic-ayudha/internal/cli"
)

// Set at build time with -ldflags "-X main.version=<v> -X main.commit=<sha> -X main.date=<rfc3339>".
var (
	version = "0.0.0-dev"
	commit  = ""
	date    = ""
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	code := cli.Run(ctx, os.Args[1:], cli.IO{In: os.Stdin, Out: os.Stdout, Err: os.Stderr},
		cli.Options{Build: cli.BuildInfo{Version: version, Commit: commit, Date: date}})
	stop()
	os.Exit(code)
}
