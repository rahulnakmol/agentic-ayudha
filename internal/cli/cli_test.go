package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// run invokes the CLI in-process with a fixed environment.
func run(t *testing.T, env map[string]string, opts Options, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	opts.Getenv = func(k string) string { return env[k] }
	for k, v := range env {
		opts.Environ = append(opts.Environ, k+"="+v)
	}
	if env["HOME"] == "" { // never read the developer's real user config
		home := t.TempDir()
		get := opts.Getenv
		opts.Getenv = func(k string) string {
			if k == "HOME" {
				return home
			}
			return get(k)
		}
	}
	if opts.Dir == "" {
		opts.Dir = t.TempDir()
	}
	code = Run(context.Background(), args, IO{In: strings.NewReader(""), Out: &out, Err: &errb}, opts)
	return code, out.String(), errb.String()
}

// withHandler temporarily implements a registry command for a test.
func withHandler(t *testing.T, name string, h Handler) {
	t.Helper()
	old, had := handlers[name]
	handlers[name] = h
	t.Cleanup(func() {
		if had {
			handlers[name] = old
		} else {
			delete(handlers, name)
		}
	})
}

func TestExitCodes(t *testing.T) {
	withHandler(t, "verify", func(context.Context, *Env, Values) error { return Errorf(ExitFloor, "coverage 89.9%% < 90%%") })
	withHandler(t, "inspect", func(context.Context, *Env, Values) error { return Errorf(ExitBudget, "per_pr_inspect_max reached") })
	withHandler(t, "status", func(context.Context, *Env, Values) error { return errors.New("unexpected nil map") })
	withHandler(t, "audit", func(context.Context, *Env, Values) error { return Errorf(ExitExternal, "GitHub: 502") })
	withHandler(t, "ready", func(context.Context, *Env, Values) error { return nil })

	agent := map[string]string{"AAYU_AGENT": "1"}
	tests := []struct {
		name string
		env  map[string]string
		args []string
		want int
	}{
		{"no arguments prints help", nil, nil, ExitOK},
		{"help", nil, []string{"help"}, ExitOK},
		{"version", nil, []string{"version"}, ExitOK},
		{"handler success", nil, []string{"ready", "12", "--check"}, ExitOK},
		{"--yes as a human", nil, []string{"ready", "12", "--yes"}, ExitOK},
		{"handler error without a code is internal", nil, []string{"status"}, ExitInternal},
		{"unknown command", nil, []string{"frobnicate"}, ExitUsage},
		{"unknown flag", nil, []string{"version", "--frobnicate"}, ExitUsage},
		{"too many arguments", nil, []string{"version", "extra"}, ExitUsage},
		{"missing required positional", nil, []string{"ready"}, ExitUsage},
		{"enum positional out of range", nil, []string{"config", "explode"}, ExitUsage},
		{"enum flag out of range", nil, []string{"release", "7", "--to", "prod"}, ExitUsage},
		{"issue reference malformed", nil, []string{"ready", "twelve"}, ExitUsage},
		{"pull request URL of an issue", nil, []string{"inspect", "https://github.com/o/r/issues/3"}, ExitUsage},
		{"bad bool flag value", nil, []string{"ready", "12", "--check=maybe"}, ExitUsage},
		{"unknown --host", nil, []string{"version", "--host", "vim"}, ExitUsage},
		{"malformed --repo", nil, []string{"version", "--repo", "not a repo"}, ExitUsage},
		{"unknown config key", nil, []string{"config", "validate", "--set", "budget.per_week_usd=1"}, ExitUsage},
		{"config validate", nil, []string{"config", "validate"}, ExitOK},
		{"later-milestone verb is not implemented yet", nil, []string{"build", "12"}, ExitUsage},
		{"floor miss", nil, []string{"verify"}, ExitFloor},
		{"--yes refused for agents", agent, []string{"ready", "12", "--yes"}, ExitBlocked},
		{"self-update refused for agents", agent, []string{"self-update"}, ExitBlocked},
		{"external error", nil, []string{"audit"}, ExitExternal},
		{"budget", nil, []string{"inspect", "#4"}, ExitBudget},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, _, stderr := run(t, tt.env, Options{}, tt.args...)
			if code != tt.want {
				t.Fatalf("aayu %s: exit %d, want %d (stderr: %s)", strings.Join(tt.args, " "), code, tt.want, stderr)
			}
		})
	}
}

func TestHelpNamesAayuAndListsRegistry(t *testing.T) {
	code, out, _ := run(t, nil, Options{}, "help")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(out, "aayu [command]") {
		t.Errorf("help does not print the aayu usage line:\n%s", out)
	}
	for _, c := range append(append([]verbs.Command{}, verbs.Verbs...), verbs.Plumbing...) {
		if !strings.Contains(out, "\n  "+c.Name+" ") {
			t.Errorf("help does not list %s", c.Name)
		}
	}
}

func TestNotImplementedNamesWP(t *testing.T) {
	_, _, stderr := run(t, nil, Options{}, "build", "12")
	if !strings.Contains(stderr, "not implemented yet (lands in WP5.5)") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestJSONOutput(t *testing.T) {
	code, out, _ := run(t, nil, Options{Build: BuildInfo{Version: "1.2.3", Commit: "abc"}}, "version", "--json")
	var v map[string]string
	if code != 0 || json.Unmarshal([]byte(out), &v) != nil || v["version"] != "1.2.3" || v["commit"] != "abc" {
		t.Fatalf("exit %d, stdout %q", code, out)
	}
	// Errors are machine-readable too, so MCP clients and hosts can branch on the code.
	code, out, _ = run(t, nil, Options{}, "build", "12", "--json")
	var e struct{ Error struct{ Code int } }
	if code != ExitUsage || json.Unmarshal([]byte(out), &e) != nil || e.Error.Code != ExitUsage {
		t.Fatalf("exit %d, stdout %q", code, out)
	}
}

func TestInputsReachHandler(t *testing.T) {
	var got Values
	withHandler(t, "install", func(_ context.Context, _ *Env, in Values) error { got = in; return nil })
	// install declares its own host input (with "all"), which reads the global --host flag.
	if code, _, stderr := run(t, nil, Options{}, "install", "--host", "all", "--apply"); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if got.String("host") != "all" || !got.Bool("apply") || got.Has("pin") {
		t.Fatalf("values = %v", got)
	}
}
