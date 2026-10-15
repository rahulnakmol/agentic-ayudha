package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
)

func TestTour(t *testing.T) {
	if code, _, errs := run(t, map[string]string{"AAYU_AGENT": "1"}, Options{IsTerminal: func(any) bool { return true }}, "tour"); code != ExitBlocked || !strings.Contains(errs, "agents are refused") {
		t.Fatalf("agent: exit %d: %s", code, errs)
	}
	if code, _, errs := run(t, nil, Options{}, "tour"); code != ExitBlocked || !strings.Contains(errs, "next: aayu init") {
		t.Fatalf("no terminal: exit %d: %s", code, errs)
	}

	// A person on a terminal: tour runs the real init and install handlers
	// (stubbed here) with the host they chose, and celebrates Level 1 once.
	repo := gitInit(t, "dev", nil)
	var calls []string
	withHandler(t, "init", func(_ context.Context, e *Env, in Values) error {
		calls = append(calls, "init")
		_ = os.WriteFile(filepath.Join(repo, ".aayudha", "config.toml"), []byte("version = 1\n"), 0o644)
		return nil
	})
	withHandler(t, "install", func(_ context.Context, e *Env, in Values) error {
		calls = append(calls, "install --host "+e.Host+" apply="+map[bool]string{true: "true", false: "false"}[in.Bool("apply")])
		return nil
	})
	_ = os.MkdirAll(filepath.Join(repo, ".aayudha"), 0o755)
	var out, errb bytes.Buffer
	g := fake.New(t, "o/r", "User")
	opts := Options{Dir: repo, GitHubAPI: g.URL(), Exec: realGitWithLFS, LookPath: noTools, IsTerminal: func(any) bool { return true }, UIStatic: true,
		Getenv: func(k string) string { return map[string]string{"HOME": t.TempDir(), "AAYU_GITHUB_TOKEN": "t"}[k] }}
	code := Run(context.Background(), []string{"tour", "--plain"}, IO{In: strings.NewReader("cursor\n\n"), Out: &out, Err: &errb}, opts)
	if code != 0 || strings.Join(calls, "; ") != "init; install --host cursor apply=true" {
		t.Fatalf("exit %d, calls %v\n%s%s", code, calls, out.String(), errb.String())
	}
	for _, want := range []string{"Step 1 of 3", "Step 3 of 3: skills, MCP server, and hooks for cursor", "Next: aayu grill story"} {
		if !strings.Contains(strings.ReplaceAll(out.String(), "Next → ", "Next: "), want) {
			t.Errorf("stdout lacks %q:\n%s", want, out.String())
		}
	}
	if !strings.Contains(errb.String(), "Milestone: Level 1!") {
		t.Errorf("no Level 1 celebration:\n%s", errb.String())
	}
}

func TestCompletionScripts(t *testing.T) {
	for shell, want := range map[string]string{"bash": "__aayu_debug", "zsh": "#compdef aayu", "fish": "complete -c aayu", "powershell": "Register-ArgumentCompleter"} {
		code, out, errs := run(t, nil, Options{}, "completion", shell)
		if code != 0 || !strings.Contains(out, want) {
			t.Errorf("%s: exit %d, %d bytes, missing %q: %s", shell, code, len(out), want, errs)
		}
	}
	// The scripts ask the binary, which completes registry enums.
	for args, want := range map[string][]string{
		"grill":          {"story", "brand"},
		"install --host": {"amp", "cursor", "claude", "all"},
		"release 3 --to": {"rolling", "main"},
		"completion":     {"bash", "zsh", "fish", "powershell"},
	} {
		code, out, _ := run(t, nil, Options{}, append(append([]string{"__complete"}, strings.Fields(args)...), "")...)
		for _, w := range want {
			if code != 0 || !strings.Contains(out, w+"\n") {
				t.Errorf("complete %q: exit %d, missing %q:\n%s", args, code, w, out)
			}
		}
	}
}
