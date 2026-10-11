package cli

import (
	"context"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/rahulnakmol/agentic-ayudha/internal/telemetry"
)

// TracePath is the local trace file (WP1.8).
func (e *Env) TracePath() string { return filepath.Join(e.stateDir(), "telemetry", "traces.jsonl") }

// startTrace opens the run's span. Telemetry never fails a command: with an
// unreadable config it records to the local file only.
func (e *Env) startTrace(cmd *cobra.Command) {
	o := telemetry.Options{Path: e.TracePath(), HTTP: e.HTTP, Version: e.Build.Version}
	if l, err := e.Config(); err == nil {
		o.Disabled, o.OTLPEndpoint = l.Config.Telemetry.Disabled, l.Config.Telemetry.OTLPEndpoint
	}
	e.telemetry = telemetry.Start(o)
	ctx, span := e.telemetry.Tracer().Start(cmd.Context(), "aayu "+cmd.Name(), trace.WithAttributes(
		attribute.String("aayu.command", cmd.CommandPath()),
		attribute.String("aayu.host", e.Host),
		attribute.Bool("aayu.dry_run", e.DryRun),
		attribute.Bool("aayu.agent", e.Agent()),
		attribute.Bool("aayu.ci", e.CI),
	))
	e.span = span
	cmd.SetContext(ctx)
}

func (e *Env) endTrace(code int, err error) {
	if e.span == nil {
		return
	}
	telemetry.End(e.span, code, err)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if serr := e.telemetry.Shutdown(ctx); serr != nil {
		e.Logf("telemetry: %v", serr)
	}
}
