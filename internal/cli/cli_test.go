package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/models"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// run invokes the CLI in-process with a fixed environment.
func run(t *testing.T, env map[string]string, opts Options, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	return runIn(t, env, opts, "", args...)
}

// runIn is run with stdin.
func runIn(t *testing.T, env map[string]string, opts Options, stdin string, args ...string) (code int, stdout, stderr string) {
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
	if opts.GHToken == nil { // never pick up the developer's real gh login
		opts.GHToken = func(context.Context) (string, error) { return "", errors.New("gh disabled in tests") }
	}
	if opts.GHAccounts == nil { // never pick up the developer's real gh accounts
		opts.GHAccounts = func(context.Context) (map[string]string, error) { return nil, nil }
	}
	if opts.GitCredential == nil { // never pick up the developer's real git credential helper
		opts.GitCredential = func(context.Context, string) (string, error) {
			return "", errors.New("git credentials disabled in tests")
		}
	}
	code = Run(context.Background(), args, IO{In: strings.NewReader(stdin), Out: &out, Err: &errb}, opts)
	return code, out.String(), errb.String()
}

// stdinTTY makes run's stdin a person's interactive terminal, as a human-only
// action needs when no agent marker is set (Bug #170); stdout and stderr stay
// pipes, so output is the plain text tests compare.
func stdinTTY(stream any) bool { _, ok := stream.(*strings.Reader); return ok }

// hostShells is the environment each host gives the shell commands its agent
// runs, without AAYU_AGENT (Bug #170): what a skill's shell call sees.
var hostShells = map[string]map[string]string{
	"amp":    {"AGENT": "amp", "AMP_THREAD_ID": "T-0000"},
	"cursor": {"CURSOR_AGENT": "1"},
	"claude": {"CLAUDECODE": "1"},
	"codex":  {"CODEX_THREAD_ID": "019a-thread"},
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
	withHandler(t, "review", func(context.Context, *Env, Values) error { return fmt.Errorf("lane A: %w", models.ErrBudget) })
	withHandler(t, "triage", func(context.Context, *Env, Values) error { return fmt.Errorf("edit body: %w", actions.ErrCapability) })

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
		{"budget breaker error from a lower layer", nil, []string{"review", "4"}, ExitBudget},
		{"capability refusal from the actions layer", nil, []string{"triage", "4"}, ExitBlocked},
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
	// Every verb has a handler since WP7.8; take one away to see the message.
	h := handlers["release"]
	delete(handlers, "release")
	t.Cleanup(func() { handlers["release"] = h })
	_, _, stderr := run(t, nil, Options{}, "release", "1", "--to", "rolling")
	if !strings.Contains(stderr, "not implemented yet (lands in WP7.8)") {
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

func TestModelsShowAndActionsChain(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	code, out, errs := run(t, nil, Options{Dir: repo}, "models", "show", "--host", "cursor", "--set", "models.review.cursor.model=gpt-6.1-sol")
	if code != 0 || !regexp.MustCompile(`review\s+cursor\s+grok-4\.6\s+xai\s+fallback`).MatchString(out) || !strings.Contains(out, "gpt-6.1-sol (openai) is not available on Cursor") {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	// The opt-in Copilot route is shown as the route real calls take.
	code, out, errs = run(t, nil, Options{Dir: repo}, "models", "show", "--host", "copilot", "--set", "models.ui-review.copilot.model=gemini-4-argon")
	if code != 0 || !regexp.MustCompile(`ui-review\s+copilot\s+gemini-4-argon\s+google\s+config\s+copilot copilot-cli\s+\(key: copilot\)`).MatchString(out) {
		t.Fatalf("Copilot route: exit %d\n%s%s", code, out, errs)
	}

	// The actions log written through Env.Actions verifies, and a tampered copy does not.
	e, _ := newEnv(IO{}, Options{Dir: repo, Getenv: func(string) string { return "" }})
	log, err := e.Actions()
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{"a", "b"} {
		if _, err := log.Do(context.Background(), actions.Action{Kind: "github.issue.comment", Payload: map[string]any{"body": body}},
			func(context.Context) (actions.Outcome, error) { return actions.Outcome{}, nil }); err != nil {
			t.Fatal(err)
		}
	}
	if code, out, _ := run(t, nil, Options{Dir: repo}, "actions", "verify-chain"); code != 0 || !strings.Contains(out, "2 records, chain intact") {
		t.Fatalf("verify-chain: exit %d: %s", code, out)
	}
	if code, out, _ := run(t, nil, Options{Dir: repo}, "actions", "grep", `"body":"b"`); code != 0 || strings.Count(out, "\n") != 1 {
		t.Fatalf("grep: exit %d: %s", code, out)
	}
	path := e.ActionsPath()
	b, _ := os.ReadFile(path)
	_ = os.WriteFile(path, bytes.Replace(b, []byte(`"body":"a"`), []byte(`"body":"A"`), 1), 0o644)
	if code, _, errs := run(t, nil, Options{Dir: repo}, "actions", "verify-chain"); code != ExitBlocked || !strings.Contains(errs, "line 1 (seq 1): content does not match") {
		t.Fatalf("tampered: exit %d: %s", code, errs)
	}

	// An agent lane cannot run verbs while the breaker has paused agent lanes.
	_ = os.MkdirAll(filepath.Dir(e.PauseFile()), 0o755)
	_ = os.WriteFile(e.PauseFile(), []byte("budget"), 0o644)
	if code, _, _ := run(t, map[string]string{"AAYU_AGENT": "1"}, Options{Dir: repo}, "status"); code != ExitBudget {
		t.Fatalf("paused agent: exit %d", code)
	}
}
