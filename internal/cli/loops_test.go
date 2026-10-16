package cli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	aayudha "github.com/rahulnakmol/agentic-ayudha"
	"github.com/rahulnakmol/agentic-ayudha/internal/loops"
)

// WP7.1 accept: the Actions fallback's digest job (guard, then status
// --digest) on schedule pages exactly one digest into the file sink; the other
// UTC cron of the day is not due; setting halt makes the next scheduled run
// exit 4 with no writes; the budget stops it with exit 6.
func TestLoopDigestOnSchedule(t *testing.T) {
	g := statusTree(t)
	dir, _ := ledgerCheckout(t) // status reads proof coverage from the ledger in a checkout
	_ = os.MkdirAll(filepath.Join(dir, "loops"), 0o755)
	_ = os.MkdirAll(filepath.Join(dir, ".aayudha"), 0o755)
	digest, err := os.ReadFile(filepath.Join("..", "..", "loops", "digest.toml"))
	if err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(dir, "loops", "digest.toml"), digest, 0o644)
	cfg, err := os.ReadFile(filepath.Join("..", "..", ".aayudha", "config.toml")) // aayudha's own: [notify] channel = "file"
	if err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(dir, ".aayudha", "config.toml"), cfg, 0o644)
	event := filepath.Join(t.TempDir(), "event.json")
	output := filepath.Join(t.TempDir(), "output")
	now := time.Date(2026, 10, 12, 8, 5, 0, 0, time.UTC) // Monday 09:05 London (BST)
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir, Now: func() time.Time { return now }}
	env := func(halt string) map[string]string {
		return map[string]string{"GITHUB_TOKEN": "workflow-token", "GITHUB_ACTIONS": "true", "GITHUB_EVENT_NAME": "schedule",
			"GITHUB_EVENT_PATH": event, "GITHUB_OUTPUT": output, "AAYUDHA_HALT": halt, "AAYU_AGENT": "1"}
	}
	guard := func(fired, halt string, extra ...string) (int, string) {
		t.Helper()
		_ = os.WriteFile(event, []byte(`{"schedule":"`+fired+`"}`), 0o644)
		_ = os.Remove(output)
		code, _, errOut := run(t, env(halt), opts, append([]string{"hook", "loop:digest", "--ci", "--repo", "o/r"}, extra...)...)
		out, _ := os.ReadFile(output)
		return code, string(out) + errOut
	}
	outbox := filepath.Join(dir, filepath.FromSlash(loops.OutboxPath))
	pages := func() int {
		b, _ := os.ReadFile(outbox)
		return strings.Count(string(b), "\n")
	}

	if code, out := guard("5 9 * * 1-5", ""); code != 0 || out != "due=false\n" {
		t.Errorf("the GMT cron in summer: exit %d, %q", code, out)
	}
	code, out := guard("5 8 * * 1-5", "")
	if code != 0 || out != "due=true\n" {
		t.Fatalf("the BST cron: exit %d, %q", code, out)
	}
	if code, _, errOut := run(t, env(""), opts, "status", "--digest", "--ci", "--repo", "o/r"); code != 0 || pages() != 1 {
		t.Fatalf("status --digest: exit %d, %d pages: %s", code, pages(), errOut)
	}
	if b, _ := os.ReadFile(outbox); !strings.Contains(string(b), `"key":"status-digest:o/r"`) || !strings.Contains(string(b), "aayudha status digest for o/r") {
		t.Errorf("outbox: %s", b)
	}

	// Halted: the next run exits 4 at the guard, and the verb itself refuses too; nothing is written.
	g.Reset()
	if code, _ := guard("5 8 * * 1-5", "founder at 2026-10-12: test"); code != ExitBlocked {
		t.Errorf("halted guard: exit %d, want %d", code, ExitBlocked)
	}
	if code, _, _ := run(t, env("founder at 2026-10-12: test"), opts, "status", "--digest", "--ci", "--repo", "o/r"); code != ExitBlocked {
		t.Errorf("halted status --digest: exit %d, want %d", code, ExitBlocked)
	}
	if w := g.Writes(); len(w) != 0 || pages() != 1 {
		t.Errorf("halted run wrote %v; %d pages", w, pages())
	}

	// [budget] ci_minutes_month: two runs of 5.5 minutes are billed as 12; the boundary stops the loop.
	g.Mu.Lock()
	g.Static = map[string]any{"/repos/o/r/actions/workflows/aayudha-loops.yml/runs": map[string]any{"total_count": 2, "workflow_runs": []any{
		map[string]any{"id": 1, "status": "completed", "run_started_at": "2026-10-05T08:05:00Z", "updated_at": "2026-10-05T08:10:30Z"},
		map[string]any{"id": 2, "status": "completed", "run_started_at": "2026-10-06T08:05:00Z", "updated_at": "2026-10-06T08:10:30Z"},
	}}}
	g.Mu.Unlock()
	if code, out := guard("5 8 * * 1-5", "", "--set", "budget.ci_minutes_month=13"); code != 0 || out != "due=true\n" {
		t.Errorf("12 minutes used of 13: exit %d %q", code, out)
	}
	if code, _ := guard("5 8 * * 1-5", "", "--set", "budget.ci_minutes_month=12"); code != ExitBudget {
		t.Errorf("12 minutes used of 12: exit %d, want %d", code, ExitBudget)
	}
	// The [budget] breaker's pause file stops every host's run.
	_ = os.MkdirAll(filepath.Join(dir, ".aayudha", "state"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, ".aayudha", "state", "paused"), []byte("per_day_usd"), 0o644)
	if code, _ := guard("5 8 * * 1-5", ""); code != ExitBudget {
		t.Errorf("paused: exit %d, want %d", code, ExitBudget)
	}
}

// S-7.1e: install --loops sets every loop up in the one tool the user picks
// from those detected; Amp's --apply starts one headless Amp thread per loop
// and records it; recording a loop in a second tool warns that it runs twice,
// and an empty ID forgets it; --run-now tests one loop with the tool's CLI.
func TestInstallLoops(t *testing.T) {
	dir := checkout(t)
	_ = os.MkdirAll(filepath.Join(dir, "loops"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "loops", "digest.toml"), []byte("description = \"Digest.\"\ncommand = [\"status\", \"--digest\"]\ncron = \"5 9 * * 1-5\"\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "loops", "triage-daily.toml"), []byte("description = \"Triage.\"\ncommand = [\"triage\"]\ncron = \"0 9 * * *\"\nhosts = [\"amp\", \"codex\"]\n"), 0o644)
	// The rendered Amp inputs, as init --gates writes them.
	files, err := func() (map[string][]byte, error) {
		ls, err := loops.Load(dir)
		if err != nil {
			return nil, err
		}
		return loops.Files(ls, "o/r", "dev", "UTC", "dev", Implemented)
	}()
	if err != nil {
		t.Fatal(err)
	}
	for p, b := range files {
		_ = os.MkdirAll(filepath.Join(dir, filepath.Dir(p)), 0o755)
		_ = os.WriteFile(filepath.Join(dir, p), b, 0o644)
	}
	var calls []string
	opts := Options{Dir: dir,
		LookPath: func(b string) (string, error) {
			if b == "amp" || b == "codex" {
				return "/usr/bin/" + b, nil
			}
			return "", os.ErrNotExist
		},
		Exec: func(_ context.Context, _, name string, args ...string) (string, error) {
			calls = append(calls, strings.Join(append([]string{name}, args...), " "))
			if strings.HasSuffix(name, "amp") && slices.Contains(args, "-ox") {
				return "Done.\nschedule s-" + strings.TrimPrefix(args[slices.Index(args, "--title")+1], "aayudha loop: ") + " on T-01a14215-c759-72bf-9a65-d2317a72a878", nil
			}
			if strings.HasSuffix(name, "amp") && args[0] == "-x" {
				return "loop triage: ran", nil
			}
			return "", errors.New("not found")
		}}
	env := map[string]string{"AAYU_GITHUB_TOKEN": "t"}
	report := func(stdin string, args ...string) (int, loopsReport, string) {
		t.Helper()
		code, out, errOut := runIn(t, env, opts, stdin, append([]string{"install", "--loops", "--repo", "o/r", "--json"}, args...)...)
		var r loopsReport
		_ = json.Unmarshal([]byte(out), &r)
		return code, r, errOut
	}
	states := func(r loopsReport) string {
		var s []string
		for _, st := range r.States {
			s = append(s, st.Loop+"="+st.Owner+":"+st.Status)
		}
		return strings.Join(s, " ")
	}
	// Two tools detected and no terminal to ask: name one.
	if code, _, errOut := report(""); code != ExitUsage || !strings.Contains(errOut, "detected: amp, codex") {
		t.Fatalf("two tools, no terminal: exit %d: %s", code, errOut)
	}
	// At a terminal it offers only the detected tools.
	term := opts
	term.IsTerminal = func(any) bool { return true }
	if code, _, errOut := runIn(t, env, term, "cursor\n", "install", "--loops", "--repo", "o/r"); code != ExitUsage || !strings.Contains(errOut, "Detected: amp, codex. [amp/codex]") {
		t.Errorf("an undetected pick: exit %d: %s", code, errOut)
	}
	if code, _, errOut := runIn(t, env, term, "codex\nn\n", "install", "--loops", "--repo", "o/r"); code != 0 || !strings.Contains(errOut, "codex/scheduled-tasks.md (2 loops)") || !strings.Contains(errOut, "Test the triage-daily loop now in codex") {
		t.Errorf("codex picked: exit %d: %s", code, errOut)
	}
	// Amp with --apply: one headless thread per loop sets its schedule, and each loop is recorded.
	code, r, errOut := report("", "--host", "amp", "--apply")
	if code != 0 || states(r) != "digest=amp:active triage-daily=amp:active" {
		t.Fatalf("amp --apply: exit %d %s: %s", code, states(r), errOut)
	}
	if n := len(slices.DeleteFunc(slices.Clone(calls), func(c string) bool {
		return !strings.HasPrefix(c, "/usr/bin/amp -ox --project o/r --no-archive-after-execute")
	})); n != 2 {
		t.Errorf("amp threads started: %d, calls %v", n, calls)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "loops", "active.json")); !strings.Contains(string(b), `"id": "schedule s-digest on T-01a14215`) {
		t.Errorf("active.json:\n%s", b)
	}
	// Recording triage-daily in Codex as well: it would run twice, so both say so.
	if code, out, _ := run(t, env, opts, "install", "--loops", "--repo", "o/r", "--host", "codex", "--record", "triage-daily=task-1"); code != 0 || !strings.Contains(out, "triage-daily is also recorded in amp: it runs twice") {
		t.Errorf("double record: exit %d: %s", code, out)
	}
	code, r, _ = report("", "--host", "codex")
	if code != 0 || states(r) != "digest=amp:active triage-daily=amp,codex:double" || len(r.Warnings) != 1 || !strings.Contains(r.Warnings[0], "records digest, triage-daily as set up in amp") {
		t.Errorf("codex with loops in amp: exit %d %s %v", code, states(r), r.Warnings)
	}
	if code, _, _ := run(t, env, opts, "install", "--loops", "--repo", "o/r", "--host", "codex", "--record", "triage-daily="); code != 0 {
		t.Errorf("forget: exit %d", code)
	}
	_ = os.WriteFile(filepath.Join(dir, "loops", "digest.toml"), []byte("description = \"Digest.\"\ncommand = [\"status\", \"--digest\"]\ncron = \"5 8 * * 1-5\"\n"), 0o644)
	if _, r, _ := report("", "--host", "amp"); states(r) != "digest=amp:stale triage-daily=amp:active" || r.RunNow != "aayu install --loops --host amp --run-now triage-daily" {
		t.Errorf("after forgetting codex and moving the digest: %s, run now %q", states(r), r.RunNow)
	}
	// Run now: the tool's own CLI runs the loop once, past "not due".
	calls = nil
	code, out, errOut := run(t, env, opts, "install", "--loops", "--repo", "o/r", "--host", "amp", "--run-now", "triage-daily")
	if calls = slices.DeleteFunc(calls, func(c string) bool { return !strings.Contains(c, " -x ") }); code != 0 || out != "loop triage: ran\n" ||
		len(calls) != 1 || !strings.HasPrefix(calls[0], "/usr/bin/amp -x This is a Run now test") {
		t.Errorf("run now: exit %d %q %v: %s", code, out, calls, errOut)
	}
	// D-7.1f: a loop that needs no agent runs in the Actions fallback. Still recorded in Amp, it runs twice until
	// Amp forgets it; then only Actions runs it, and the report says so.
	_ = os.WriteFile(filepath.Join(dir, "loops", "digest.toml"), []byte("description = \"Digest.\"\ncommand = [\"status\", \"--digest\"]\ncron = \"5 8 * * 1-5\"\nagent = false\n"), 0o644)
	if _, r, _ := report("", "--host", "amp"); states(r) != "digest=actions,amp:double triage-daily=amp:active" {
		t.Errorf("digest now agent = false, still in amp: %s", states(r))
	}
	if code, _, errOut := run(t, env, opts, "install", "--loops", "--repo", "o/r", "--host", "amp", "--record", "digest="); code != 0 {
		t.Errorf("forget a loop amp no longer runs: exit %d: %s", code, errOut)
	}
	if _, r, _ := report("", "--host", "amp"); states(r) != "digest=actions:active triage-daily=amp:active" || !slices.Equal(r.Actions, []string{"digest"}) {
		t.Errorf("digest in Actions only: %s, actions %v", states(r), r.Actions)
	}
}

// D-7.1g: in a fresh repository, install --loops adds the default loops and every tool's render of them, so the
// Claude Code and Codex steps name files that exist; a product's copy carries no aayudha-only setup.
func TestInstallLoopsAddsDefaults(t *testing.T) {
	for _, host := range []string{"claude", "codex"} {
		dir := checkout(t)
		opts := Options{Dir: dir, Assets: aayudha.Assets, LookPath: func(string) (string, error) { return "", os.ErrNotExist }}
		code, out, errOut := run(t, map[string]string{"AAYU_GITHUB_TOKEN": "t"}, opts, "install", "--loops", "--repo", "o/r", "--host", host, "--json")
		var r loopsReport
		_ = json.Unmarshal([]byte(out), &r)
		if _, err := os.Stat(filepath.Join(dir, "loops", "triage-daily.toml")); code != 0 || err != nil || len(r.Stale) != 0 || len(r.States) == 0 {
			t.Fatalf("%s in a fresh repository: exit %d, stale %v, %d loops: %s", host, code, r.Stale, len(r.States), errOut)
		}
		prompts := map[string]string{"claude": "loops/claude/routines.md", "codex": "loops/codex/scheduled-tasks.md"}[host]
		if b, err := os.ReadFile(filepath.Join(dir, prompts)); err != nil || !strings.Contains(string(b), "aayudha triage-daily (o/r)") {
			t.Errorf("%s: %s: %v", host, prompts, err)
		}
		if b, _ := os.ReadFile(filepath.Join(dir, ".github", "workflows", loops.Workflow)); !strings.Contains(string(b), "loop:verify-sweep") || strings.Contains(string(b), "aayudha/floors") {
			t.Errorf("%s: the Actions fallback runs the sweep without aayudha's own setup:\n%s", host, b)
		}
	}
}
