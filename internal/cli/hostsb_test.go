package cli

import (
	"bytes"
	"context"
	"fmt"
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
	// pre-tool from the shared plugin names no host: it is detected, and a
	// human-only verb in a shell command is blocked (exit 2) with the command to hand over.
	code, _, errs := hook(`{"tool_name":"Bash","tool_input":{"command":"aayu ship 4"}}`, map[string]string{"CODEX_THREAD_ID": "t1"}, "pre-tool", "--skills", "0.0.0")
	if code != 2 || !strings.Contains(errs, "run `aayu ship 4` in their own terminal") {
		t.Errorf("pre-tool: exit %d %s", code, errs)
	}
	// The refusal's next step is the person's, never the breaker's `aayu resume` (D-3.2b.e).
	if !strings.Contains(errs, "next: a person runs `aayu ship 4` in their own terminal") || strings.Contains(errs, "resume") {
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
	ship := `{"tool_name":"Bash","tool_input":{"command":"aayu ship 4"}}`
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
	cursorShip := `{"tool_name":"Shell","tool_input":{"command":"aayu ship 4"},"cursor_version":"2.4.1","hook_event_name":"preToolUse"}`
	cursorHooks := filepath.Join(repo, ".cursor", "hooks.json")
	_ = os.MkdirAll(filepath.Dir(cursorHooks), 0o755)
	_ = os.WriteFile(cursorHooks, []byte(`{"version": 1, "hooks": {"beforeShellExecution": [{"command": "\"/usr/bin/aayu\" hook pre-tool --host cursor"}]}}`), 0o644)
	refusals := 0
	for _, h := range []struct{ event, host, skills string }{
		{cursorShip, "claude", ""}, // Claude Code's repository hook, imported
		{cursorShip, "", "0.0.0"},  // Claude Code's plugin hook, imported
		{`{"command":"aayu ship 4","cursor_version":"2.4.1","hook_event_name":"beforeShellExecution"}`, "cursor", ""},
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
