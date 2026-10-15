package cli

import (
	"bytes"
	"context"
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
