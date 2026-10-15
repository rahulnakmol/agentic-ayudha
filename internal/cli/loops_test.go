package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
