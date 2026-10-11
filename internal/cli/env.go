package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"time"

	"go.opentelemetry.io/otel/trace"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/secrets"
	"github.com/rahulnakmol/agentic-ayudha/internal/telemetry"
)

// IO is where a command reads and writes. Commands never touch os.Stdin,
// os.Stdout, or os.Stderr directly, so the same code serves the terminal, tests,
// and the MCP stdio server (WP3.3), where stdout carries protocol frames.
type IO struct {
	In  io.Reader
	Out io.Writer
	Err io.Writer
}

// BuildInfo is stamped into the binary at release time (-ldflags -X).
type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

// Options are the process-level dependencies of a run. Zero values use the real
// process environment; tests replace them.
type Options struct {
	Build      BuildInfo
	Getenv     func(string) string
	Environ    []string // KEY=value pairs for the config env layer; nil with a nil Getenv means os.Environ
	Dir        string   // working directory; "" means os.Getwd
	HTTP       *http.Client
	Now        func() time.Time
	Executable func() (string, error)
	SelfUpdate SelfUpdateOptions
	GitHubAPI  string                                                                      // default https://api.github.com
	GHToken    func(ctx context.Context) (string, error)                                   // `gh auth token`; nil runs gh
	Secrets    *secrets.Store                                                              // nil uses keychain, env, then the age file
	LookPath   func(string) (string, error)                                                // nil uses exec.LookPath (doctor)
	Exec       func(ctx context.Context, dir, name string, args ...string) (string, error) // nil runs the command (doctor, install)
	Assets     fs.FS                                                                       // embedded skills/ and hosts/ (root embed.go); install needs it
}

// Globals are the global flags shared by every command.
type Globals struct {
	DryRun  bool
	JSON    bool
	Repo    string
	Host    string
	Yes     bool
	Verbose bool
	CI      bool     // running in CI: GITHUB_TOKEN may be the human token; bot writes need the App token
	Set     []string // config overrides, key=value (the flags layer of WP1.2)
}

// Env is everything a command handler may use.
type Env struct {
	IO
	Globals
	Options
	config  *config.Loaded // loaded on first use by Config
	actions *actions.Log   // opened on first use by Actions

	telemetry *telemetry.Telemetry
	span      trace.Span
}

func newEnv(io IO, opts Options) (*Env, error) {
	if opts.Getenv == nil {
		opts.Getenv = os.Getenv
		if opts.Environ == nil {
			opts.Environ = os.Environ()
		}
	}
	if opts.Dir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		opts.Dir = wd
	}
	if opts.HTTP == nil {
		opts.HTTP = &http.Client{Timeout: 5 * time.Minute}
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Executable == nil {
		opts.Executable = os.Executable
	}
	if opts.Build.Version == "" {
		opts.Build.Version = "0.0.0-dev"
	}
	return &Env{IO: io, Options: opts}, nil
}

// Agent reports whether the process runs as an agent lane (AAYU_AGENT=1).
// Agents may not confirm on a human's behalf or replace the binary.
func (e *Env) Agent() bool { return e.Getenv("AAYU_AGENT") == "1" }

// Emit writes a command result: v as indented JSON with --json, otherwise text.
func (e *Env) Emit(v any, text func(w io.Writer) error) error {
	if e.JSON {
		enc := json.NewEncoder(e.Out)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		return enc.Encode(v)
	}
	return text(e.Out)
}

// Logf writes a diagnostic line to the error stream when --verbose is set.
func (e *Env) Logf(format string, args ...any) {
	if e.Verbose {
		fmt.Fprintf(e.Err, format+"\n", args...)
	}
}

// Handler implements a registry command. Returned errors without an exit code
// are internal errors (exit 1).
type Handler func(ctx context.Context, e *Env, in Values) error

// handlers maps registry command names to implementations. A registry command
// without a handler reports "not implemented yet" with exit code 2.
var handlers = map[string]Handler{}

func register(name string, h Handler) {
	if _, dup := handlers[name]; dup {
		panic("cli: duplicate handler " + name)
	}
	handlers[name] = h
}

// Values holds the parsed, validated inputs of one invocation, keyed by the
// registry input name. Unset optional inputs are absent.
type Values map[string]any

// String returns a string or enum input, or "" when unset.
func (v Values) String(name string) string { s, _ := v[name].(string); return s }

// Bool returns a bool input, or false when unset.
func (v Values) Bool(name string) bool { b, _ := v[name].(bool); return b }

// Int returns an int input, or 0 when unset.
func (v Values) Int(name string) int { n, _ := v[name].(int); return n }

// Has reports whether an input was given.
func (v Values) Has(name string) bool { _, ok := v[name]; return ok }
