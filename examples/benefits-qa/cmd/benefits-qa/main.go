// Command benefits-qa serves the benefits-qa example (layer L02,
// docs/BUILD-PLAN.md WP8.1): the Ask page at /, POST /api/ask, and GET
// /healthz, over the embedded synthetic plan documents. With -events it
// appends every product event to an Application Insights export; with
// -telemetry it writes OpenTelemetry traces (OTLP JSON) and JSON logs that
// carry trace IDs, tagged with the revision it was built from, so `aayu
// investigate` can read production behaviour by deployed revision.
//
//	go run ./examples/benefits-qa/cmd/benefits-qa -addr 127.0.0.1:8787
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"syscall"
	"time"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/rahulnakmol/agentic-ayudha/examples/benefits-qa/analytics"
	"github.com/rahulnakmol/agentic-ayudha/examples/benefits-qa/api"
	"github.com/rahulnakmol/agentic-ayudha/examples/benefits-qa/ask"
	"github.com/rahulnakmol/agentic-ayudha/examples/benefits-qa/store"
	"github.com/rahulnakmol/agentic-ayudha/examples/benefits-qa/web"
	"github.com/rahulnakmol/agentic-ayudha/internal/telemetry"
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
	telemetryDir := fs.String("telemetry", "", "append OpenTelemetry traces (traces.jsonl, OTLP JSON) and JSON logs (logs.jsonl) to this directory")
	revision := fs.String("revision", buildRevision(), "the deployed revision, recorded as service.version (default: the commit the binary was built from)")
	environment := fs.String("environment", "local", "the deployment environment, recorded as deployment.environment.name")
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
	var routes http.Handler = mux
	if *telemetryDir != "" {
		obs, stop, err := observe(*telemetryDir, *revision, *environment, mux)
		if err != nil {
			return err
		}
		defer stop()
		routes = obs
	}
	// /healthz names this process, so deploy/serve.sh stops only the instance it started.
	pid := strconv.Itoa(os.Getpid())
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			w.Header().Set(PIDHeader, pid)
		}
		routes.ServeHTTP(w, r)
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

// observe sets up the OpenTelemetry tracer and the JSON logger over dir and
// wraps mux with api.Observe. stop flushes the spans and closes the files.
func observe(dir, revision, environment string, mux *http.ServeMux) (http.Handler, func(), error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, nil, err
	}
	logs, err := os.OpenFile(filepath.Join(dir, "logs.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, nil, err
	}
	resource := []attribute.KeyValue{attribute.String("service.name", "benefits-qa"), attribute.String("service.version", revision),
		attribute.String("deployment.environment.name", environment)}
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(telemetry.NewFileExporter(filepath.Join(dir, "traces.jsonl"), resource...)))
	log := slog.New(slog.NewJSONHandler(logs, nil)).With("service.name", "benefits-qa", "service.version", revision)
	stop := func() {
		_ = tp.Shutdown(context.Background())
		_ = logs.Close()
	}
	return api.Observe(mux, mux, tp.Tracer("benefits-qa"), log), stop, nil
}

// buildRevision is the commit the binary was built from (Go records it when
// it builds inside a git checkout), or "unknown".
func buildRevision() string {
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			if s.Key == "vcs.revision" {
				return s.Value
			}
		}
	}
	return "unknown"
}
