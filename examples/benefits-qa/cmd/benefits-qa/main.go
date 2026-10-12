// Command benefits-qa serves the benefits-qa example (layer L02,
// docs/BUILD-PLAN.md WP8.1): the Ask page at /, POST /api/ask, and GET
// /healthz, over the embedded synthetic plan documents. With -events it
// appends every product event to an Application Insights export.
//
//	go run ./examples/benefits-qa/cmd/benefits-qa -addr 127.0.0.1:8787
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/examples/benefits-qa/analytics"
	"github.com/rahulnakmol/agentic-ayudha/examples/benefits-qa/api"
	"github.com/rahulnakmol/agentic-ayudha/examples/benefits-qa/ask"
	"github.com/rahulnakmol/agentic-ayudha/examples/benefits-qa/store"
	"github.com/rahulnakmol/agentic-ayudha/examples/benefits-qa/web"
)

// PIDHeader carries the server's process ID on /healthz.
const PIDHeader = "X-Benefits-Qa-Pid"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stderr, nil); err != nil {
		fmt.Fprintln(os.Stderr, "benefits-qa:", err)
		os.Exit(1)
	}
}

// run serves until ctx ends. ready, when set, receives the listening address.
func run(ctx context.Context, args []string, stderr io.Writer, ready chan<- string) error {
	fs := flag.NewFlagSet("benefits-qa", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addr := fs.String("addr", "127.0.0.1:8787", "listen address")
	tokens := fs.String("tokens", "packages/design-system/src/styles/tokens.css", "path of the design system's tokens.css")
	events := fs.String("events", "", "append product events to this Application Insights export (JSON lines)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if _, err := os.Stat(*tokens); err != nil {
		return fmt.Errorf("-tokens: %w", err)
	}
	s, err := store.Load()
	if err != nil {
		return err
	}
	var rec ask.Recorder = ask.Discard{}
	if *events != "" {
		f, err := os.OpenFile(*events, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
		rec = analytics.NewAppInsights(f)
	}
	svc := ask.New(s, rec, nil)
	mux := http.NewServeMux()
	api.Register(mux, svc)
	web.Register(mux, svc, *tokens)
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		return err
	}
	// /healthz names this process, so deploy/serve.sh stops only the instance it started.
	pid := strconv.Itoa(os.Getpid())
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			w.Header().Set(PIDHeader, pid)
		}
		mux.ServeHTTP(w, r)
	})
	srv := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: time.Minute}
	fmt.Fprintf(stderr, "benefits-qa: serving http://%s\n", ln.Addr())
	if ready != nil {
		ready <- ln.Addr().String()
	}
	shut := make(chan error, 1)
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		shut <- srv.Shutdown(sctx)
	}()
	if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return <-shut
}
