package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The marketplace plugin's hooks and server pass the skills version they carry
// and run aayu from PATH (WP3.2b): a mismatch is reported, never silently used.
func TestPluginHooksAndStatusLine(t *testing.T) {
	repo := gitInit(t, "dev", map[string]string{".aayudha/config.toml": "version = 1\n"})
	hook := func(stdin string, env map[string]string, args ...string) (int, string, string) {
		var out, errb bytes.Buffer
		opts := Options{Dir: repo, Now: func() time.Time { return time.Date(2026, 10, 15, 10, 0, 0, 0, time.UTC) },
			Getenv: func(k string) string { return env[k] }}
		code := Run(context.Background(), append([]string{"hook"}, args...), IO{In: strings.NewReader(stdin), Out: &out, Err: &errb}, opts)
		return code, out.String(), errb.String()
	}
	// Before any home screen: a hint, never a network call.
	if code, out, _ := hook("{}", nil, "status-line", "--host", "claude"); code != 0 || out != "aayudha | run aayu for the home screen\n" {
		t.Fatalf("status-line without a cache: exit %d %q", code, out)
	}
	cache := filepath.Join(repo, ".aayudha", "state", "journey.json")
	_ = os.MkdirAll(filepath.Dir(cache), 0o755)
	_ = os.WriteFile(cache, []byte(`{"ready":[{"number":1},{"number":2}],"awaiting_human":[{"number":4}],"next":{"command":"aayu ship 4"},"at":"2026-10-15T09:00:00Z"}`), 0o644)
	if _, out, _ := hook("{}", nil, "status-line", "--host", "claude"); out != "aayudha | Ready 2 | waiting for you 1 | next: aayu ship 4 | as of 1h0m0s ago\n" {
		t.Errorf("status-line: %q", out)
	}
	// Open Feedback without a fix layer surfaces on its own at session start
	// and in the status line, with the command that classifies it (D-7.7e).
	if _, out, _ := hook("{}", nil, "session-start", "--host", "claude"); out != "" {
		t.Errorf("session-start with nothing to classify: %q", out)
	}
	_ = os.WriteFile(cache, []byte(`{"unclassified_feedback":[{"number":190,"title":"Persona outputs present inferences as facts"},{"number":152,"title":"Feedback: brief eli5"}],"next":{"command":"aayu next"},"at":"2026-10-15T09:00:00Z"}`), 0o644)
	if _, out, _ := hook("{}", nil, "status-line", "--host", "claude"); !strings.Contains(out, "| Feedback to classify 2 |") {
		t.Errorf("status-line with unclassified Feedback: %q", out)
	}
	if _, out, _ := hook("{}", nil, "session-start", "--host", "claude"); !strings.HasPrefix(out, "aayudha: 2 open Feedback issue(s) without a fix layer (#190 Persona outputs present inferences as facts; #152 Feedback: brief eli5)") ||
		!strings.Contains(out, "`aayu retro --classify <issue> --layer codebase|check|prose|review") || !strings.Contains(out, "(as of 1h0m0s ago; `aayu` refreshes it)") {
		t.Errorf("session-start with unclassified Feedback: %q", out)
	}
	if _, out, _ := hook("{}", nil, "session-start", "--host", "cursor"); !strings.HasPrefix(out, `{"additional_context":"aayudha: 2 open Feedback issue(s) without a fix layer`) {
		t.Errorf("cursor session-start with unclassified Feedback: %q", out)
	}
	_ = os.Remove(cache)
	// pre-tool from the shared plugin names no host: it is detected, and a
	// human-only flag in a shell command is blocked (exit 2) with the command to hand over.
	code, _, errs := hook(`{"tool_name":"Bash","tool_input":{"command":"aayu ship 4 --solo-bypass"}}`, map[string]string{"CODEX_THREAD_ID": "t1"}, "pre-tool", "--skills", "0.0.0")
	if code != 2 || !strings.Contains(errs, "run `aayu ship 4 --solo-bypass` in their own terminal") {
		t.Errorf("pre-tool: exit %d %s", code, errs)
	}
	// The refusal's next step is the person's, never the breaker's `aayu resume` (D-3.2b.e).
	if !strings.Contains(errs, "next: a person runs `aayu ship 4 --solo-bypass` in their own terminal") || strings.Contains(errs, "resume") {
		t.Errorf("pre-tool next step: %s", errs)
	}
	// Codex's session starts with the request routing: the grill gate first (D-3.2b.b).
	if _, out, _ := hook("{}", nil, "session-start", "--host", "codex"); !strings.HasPrefix(out, "aayudha: before you change anything for a request, run `aayu grill story --assess") {
		t.Errorf("codex session-start: %q", out)
	}
	// Where the host runs the hooks `aayu install` registered, the plugin's
	// (they pass --skills) stand aside, so each runs once (D-3.2b.d). Codex
	// runs a project's hooks only in a project it trusts, so in an untrusted
	// one the plugin's hook still refuses (D-3.2b.h).
	_ = os.MkdirAll(filepath.Join(repo, ".codex"), 0o755)
	_ = os.WriteFile(filepath.Join(repo, ".codex", "hooks.json"), []byte(`{"hooks": {"PreToolUse": [{"hooks": [{"type": "command", "command": "\"/usr/bin/aayu\" hook pre-tool --host codex"}]}]}}`), 0o644)
	codexHome := t.TempDir()
	codex := map[string]string{"CODEX_HOME": codexHome}
	ship := `{"tool_name":"Bash","tool_input":{"command":"aayu ship 4 --solo-bypass"}}`
	if code, _, _ := hook(ship, codex, "pre-tool", "--host", "codex", "--skills", "0.0.0"); code != 2 {
		t.Errorf("the plugin's pre-tool stood aside in an untrusted Codex project, leaving no guard: exit %d", code)
	}
	_ = os.WriteFile(filepath.Join(codexHome, "config.toml"), []byte(fmt.Sprintf("[projects.%q]\ntrust_level = \"trusted\"\n", repo)), 0o644)
	if code, _, _ := hook(ship, codex, "pre-tool", "--host", "codex", "--skills", "0.0.0"); code != 0 {
		t.Errorf("the plugin's pre-tool ran beside the repository's: exit %d", code)
	}
	if code, _, _ := hook(ship, codex, "pre-tool", "--host", "codex"); code != 2 {
		t.Errorf("the repository's pre-tool stood aside: exit %d", code)
	}
	if _, out, _ := hook("{}", codex, "session-start", "--host", "codex", "--skills", "0.0.0"); !strings.Contains(out, "turns the plugin off for this repository") || strings.Contains(out, "grill") {
		t.Errorf("the plugin's session-start beside the repository's: %q", out)
	}
	// Cursor runs Claude Code's hooks too (Third-Party Imports), so with both
	// installed one shell command reaches Claude Code's pre-tool and Cursor's.
	// Exactly one refuses it, in Cursor's format (D-3.2b.j).
	_ = os.MkdirAll(filepath.Join(repo, ".claude"), 0o755)
	_ = os.WriteFile(filepath.Join(repo, ".claude", "settings.json"), []byte(`{"hooks": {"PreToolUse": [{"matcher": "*", "hooks": [{"type": "command", "command": "\"/usr/bin/aayu\" hook pre-tool --host claude"}]}]}}`), 0o644)
	cursorShip := `{"tool_name":"Shell","tool_input":{"command":"aayu ship 4 --solo-bypass"},"cursor_version":"2.4.1","hook_event_name":"preToolUse"}`
	cursorHooks := filepath.Join(repo, ".cursor", "hooks.json")
	_ = os.MkdirAll(filepath.Dir(cursorHooks), 0o755)
	_ = os.WriteFile(cursorHooks, []byte(`{"version": 1, "hooks": {"beforeShellExecution": [{"command": "\"/usr/bin/aayu\" hook pre-tool --host cursor"}]}}`), 0o644)
	refusals := 0
	for _, h := range []struct{ event, host, skills string }{
		{cursorShip, "claude", ""}, // Claude Code's repository hook, imported
		{cursorShip, "", "0.0.0"},  // Claude Code's plugin hook, imported
		{`{"command":"aayu ship 4 --solo-bypass","cursor_version":"2.4.1","hook_event_name":"beforeShellExecution"}`, "cursor", ""},
	} {
		args := []string{"pre-tool"}
		if h.host != "" {
			args = append(args, "--host", h.host)
		}
		if h.skills != "" {
			args = append(args, "--skills", h.skills)
		}
		code, out, _ := hook(h.event, nil, args...)
		if code == 2 {
			refusals++
		} else if out != `{"permission":"allow"}`+"\n" {
			t.Errorf("pre-tool %v in Cursor: exit %d %q", args, code, out)
		}
	}
	if refusals != 1 {
		t.Errorf("Cursor refused one shell command %d times", refusals)
	}
	// Without Cursor's own hooks, Claude Code's is the one that refuses, in Cursor's format.
	_ = os.Remove(cursorHooks)
	if code, out, _ := hook(cursorShip, nil, "pre-tool", "--host", "claude"); code != 2 || !strings.Contains(out, `"permission":"deny"`) {
		t.Errorf("Claude Code's pre-tool in Cursor alone: exit %d %q", code, out)
	}
	if code, _, _ := hook(ship, nil, "pre-tool", "--host", "claude"); code != 2 {
		t.Errorf("Claude Code's pre-tool in Claude Code: exit %d", code)
	}
	if code, _, _ := hook(`{"tool_name":"Bash","tool_input":{"command":"aayu build 4"}}`, nil, "pre-tool", "--host", "claude"); code != 0 {
		t.Errorf("build blocked: exit %d", code)
	}
	if _, out, _ := hook("{}", map[string]string{"CLAUDECODE": "1"}, "session-start", "--skills", "9.9.9"); !strings.Contains(out, "the claude plugin carries skills 9.9.9, but the aayu on PATH carries 0.0.0") {
		t.Errorf("session-start mismatch: %q", out)
	}
	if code, _, errs := run(t, nil, Options{Dir: repo}, "mcp", "--skills", "9.9.9"); code != ExitExternal || !strings.Contains(errs, "next: aayu self-update") {
		t.Errorf("mcp mismatch: exit %d %s", code, errs)
	}
}

// Cursor starts the aayudha MCP server with only HOME, LOGNAME, PATH, SHELL,
// TERM, USER, and PWD (D-3.2b.p), so a token this shell finds through
// AAYU_GITHUB_TOKEN never reaches it. doctor resolves the token as that
// server would and names the fix; the server's own error says why.
func TestCursorMCPToken(t *testing.T) {
	repo, home, opts := installEnv(t)
	opts.GHToken = func(context.Context) (string, error) { return "", errors.New("gh: not logged in") }
	opts.HTTP = &http.Client{Transport: roundTrip(func(*http.Request) (*http.Response, error) { return nil, errors.New("offline") })}
	env := map[string]string{"HOME": home, "AAYU_GITHUB_TOKEN": "ghp_shell"}
	if code, out, errs := run(t, env, opts, "install", "--host", "cursor", "--apply", "--yes"); code != 0 {
		t.Fatalf("install: exit %d\n%s%s", code, out, errs)
	}
	_, out, _ := run(t, env, opts, "doctor")
	if !strings.Contains(out, "cursor: Cursor starts the aayudha MCP server with only HOME") || !strings.Contains(out, "comes from AAYU_GITHUB_TOKEN") {
		t.Errorf("doctor does not say the MCP server in Cursor finds no token:\n%s", out)
	}
	// With the token where the server finds it too, the line goes.
	opts.GHToken = func(context.Context) (string, error) { return "gho_store", nil }
	if _, out, _ := run(t, env, opts, "doctor"); strings.Contains(out, "Cursor starts the aayudha MCP server") {
		t.Errorf("doctor warns although gh's token reaches the server:\n%s", out)
	}
	e, err := newEnv(IO{In: strings.NewReader(""), Out: io.Discard, Err: io.Discard}, Options{Dir: repo, GHToken: func(context.Context) (string, error) { return "", errors.New("no") }, MCP: true, Getenv: func(k string) string { return map[string]string{"HOME": home}[k] }})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Resolver().Human(context.Background()); err == nil || !strings.Contains(err.Error(), "Cursor passes it only HOME") {
		t.Errorf("the MCP server's no-token error: %v", err)
	}
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// Copilot's pre-tool hook credits a build to its session only when the hook
// lets the call through: a build refused because the repository is halted
// leaves no note for a later build to read.
func TestCopilotPreToolRecordsOnlyAllowedBuilds(t *testing.T) {
	repo := gitInit(t, "dev", map[string]string{".aayudha/config.toml": "version = 1\n"})
	hook := func(env map[string]string) int {
		var out, errb bytes.Buffer
		opts := Options{Dir: repo, Getenv: func(k string) string { return env[k] }}
		ev := `{"sessionId":"s-1","cwd":"/r","toolName":"bash","toolArgs":{"command":"aayu build 4"}}`
		return Run(context.Background(), []string{"hook", "pre-tool", "--host", "copilot"}, IO{In: strings.NewReader(ev), Out: &out, Err: &errb}, opts)
	}
	note := filepath.Join(repo, ".aayudha", "state", "copilot-session.json")
	if code := hook(map[string]string{HaltVariable: "incident 7"}); code != 2 {
		t.Fatalf("halted pre-tool: exit %d", code)
	}
	if _, err := os.Stat(note); err == nil {
		t.Fatal("a refused build was credited to the Copilot session")
	}
	if code := hook(map[string]string{}); code != 0 {
		t.Fatalf("pre-tool: exit %d", code)
	}
	if _, err := os.Stat(note); err != nil {
		t.Fatalf("an allowed build left no note: %v", err)
	}
}
