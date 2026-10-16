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

	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
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

// S-7.1b: an event loop's guard is due whenever its own event started the run
// (in an agent session, or in Actions on that event), not on another event;
// halt still stops it with exit 4 before anything runs.
func TestLoopGuardEvent(t *testing.T) {
	g := statusTree(t)
	dir := checkout(t)
	_ = os.MkdirAll(filepath.Join(dir, "loops"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "loops", "triage-new.toml"), []byte("description = \"x\"\ncommand = [\"triage\", \"{number}\"]\non = \"issues.opened\"\n"), 0o644)
	output := filepath.Join(t.TempDir(), "output")
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir}
	guard := func(event, halt string) (int, string) {
		t.Helper()
		_ = os.Remove(output)
		env := map[string]string{"GITHUB_TOKEN": "workflow-token", "GITHUB_OUTPUT": output, "AAYUDHA_HALT": halt}
		if event != "" {
			env["GITHUB_ACTIONS"], env["GITHUB_EVENT_NAME"] = "true", event
		}
		code, stdout, _ := run(t, env, opts, "hook", "loop:triage-new", "--repo", "o/r")
		out, _ := os.ReadFile(output)
		return code, string(out) + stdout
	}
	if code, out := guard("", ""); code != 0 || !strings.Contains(out, "due=true") || !strings.Contains(out, "event: issues.opened") {
		t.Errorf("agent session: exit %d %q", code, out)
	}
	if code, out := guard("issues", ""); code != 0 || !strings.Contains(out, "due=true") {
		t.Errorf("Actions on issues: exit %d %q", code, out)
	}
	if code, out := guard("pull_request", ""); code != 0 || !strings.Contains(out, "due=false") {
		t.Errorf("Actions on another event: exit %d %q", code, out)
	}
	if code, _ := guard("", "founder at 2026-10-16: test"); code != ExitBlocked {
		t.Errorf("halted: exit %d, want %d", code, ExitBlocked)
	}
}

// S-7.1d: install --loops shows each detected host's steps for this
// repository only (Amp: set_schedule and create_github_subscription inputs
// with the project's ID); --record notes loops as set up, so the next run says
// active, and a changed loop says stale.
func TestInstallLoops(t *testing.T) {
	dir := checkout(t)
	_ = os.MkdirAll(filepath.Join(dir, "loops"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "loops", "digest.toml"), []byte("description = \"Digest.\"\ncommand = [\"status\", \"--digest\"]\ncron = \"5 9 * * 1-5\"\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "loops", "triage-new.toml"), []byte("description = \"Triage.\"\ncommand = [\"triage\"]\non = \"issues.opened\"\nfilter = { label = \"intake\" }\n"), 0o644)
	opts := Options{Dir: dir,
		LookPath: func(b string) (string, error) {
			if b == "amp" {
				return "/usr/bin/amp", nil
			}
			return "", os.ErrNotExist
		},
		Exec: func(_ context.Context, _, name string, args ...string) (string, error) {
			if strings.Join(append([]string{name}, args...), " ") == "amp projects get o/r --json" {
				return `{"id": "proj-7"}`, nil
			}
			return "", errors.New("not found")
		}}
	env := map[string]string{"AAYU_GITHUB_TOKEN": "t"}
	report := func(args ...string) loopsReport {
		t.Helper()
		code, out, errOut := run(t, env, opts, append([]string{"install", "--loops", "--repo", "o/r", "--json"}, args...)...)
		var r loopsReport
		if err := json.Unmarshal([]byte(out), &r); code != 0 || err != nil {
			t.Fatalf("install --loops %v: exit %d %v: %s", args, code, err, errOut)
		}
		return r
	}
	states := func(r loopsReport) string {
		var s []string
		for _, h := range r.Hosts {
			for _, st := range h.States {
				s = append(s, h.Host+":"+st.Loop+"="+st.Status)
			}
		}
		return strings.Join(s, " ")
	}
	r := report()
	if len(r.Hosts) != 1 || r.Hosts[0].Host != "amp" || !r.Hosts[0].Detected {
		t.Fatalf("detected hosts: %+v", r.Hosts)
	}
	steps := strings.Join(r.Hosts[0].Steps, "\n")
	if !strings.Contains(steps, "set_schedule with the input in loops/amp/digest.json") || !strings.Contains(steps, "create_github_subscription with the input in loops/amp/triage-new.json, adding project_id proj-7") {
		t.Errorf("amp steps:\n%s", steps)
	}
	if got := states(r); got != "amp:digest=missing amp:triage-new=missing" {
		t.Errorf("before: %s", got)
	}
	if !slices.Contains(r.Stale, "loops/amp/digest.json") {
		t.Errorf("unrendered files not reported: %v", r.Stale)
	}
	if code, _, errOut := run(t, env, opts, "install", "--loops", "--repo", "o/r", "--host", "amp", "--record", "digest=T-1,triage-new=sub-2"); code != 0 {
		t.Fatalf("record: exit %d: %s", code, errOut)
	}
	if got := states(report()); got != "amp:digest=active amp:triage-new=active" {
		t.Errorf("after recording: %s", got)
	}
	_ = os.WriteFile(filepath.Join(dir, "loops", "digest.toml"), []byte("description = \"Digest.\"\ncommand = [\"status\", \"--digest\"]\ncron = \"5 8 * * 1-5\"\n"), 0o644)
	if got := states(report()); got != "amp:digest=stale amp:triage-new=active" {
		t.Errorf("after the digest moved to 08:05: %s", got)
	}
	if code, _, _ := run(t, env, opts, "install", "--loops", "--repo", "o/r", "--host", "codex", "--record", "triage-new=x"); code != ExitUsage {
		t.Errorf("recording an event loop on Codex, which has no trigger for it: exit %d, want %d", code, ExitUsage)
	}
}

// Appendix B D-7.1e: where no host trigger runs an event loop, the next session's SessionStart hook lists the
// GitHub events since the last session, once, for each event loop no agent host runs; a loop recorded as set up
// on a host is left to that host.
func TestSessionStartLoopEvents(t *testing.T) {
	dir := checkout(t)
	_ = os.MkdirAll(filepath.Join(dir, "loops"), 0o755)
	for name, body := range map[string]string{
		"triage-new":      "description = \"Triage.\"\ncommand = [\"triage\"]\non = \"issues.opened\"\nfilter = { label = \"intake\" }\n",
		"feedback":        "description = \"Feedback.\"\ncommand = [\"evolve\"]\non = \"issues.opened\"\nfilter = { label = \"type:feedback\" }\n",
		"ci-failed":       "description = \"Failed check.\"\ncommand = [\"status\", \"--work\"]\non = \"check_suite.completed\"\nfilter = { conclusion = \"failure\" }\n",
		"release-measure": "description = \"Measure.\"\ncommand = [\"measure\"]\non = \"release.published\"\n",
	} {
		_ = os.WriteFile(filepath.Join(dir, "loops", name+".toml"), []byte(body), 0o644)
	}
	g := fake.New(t, "o/r", "User")
	g.AddIssue("New intake", "", "intake").CreatedAt = "2026-10-16T08:00:00Z"
	g.AddIssue("Old intake", "", "intake").CreatedAt = "2026-10-14T08:00:00Z" // before the first session's day
	g.AddIssue("Feedback", "", "type:feedback").CreatedAt = "2026-10-16T08:00:00Z"
	g.AddPR("Fix", "", "dev1", "fix", strings.Repeat("c", 40), "dev").UpdatedAt = time.Date(2026, 10, 16, 8, 0, 0, 0, time.UTC)
	g.CheckRuns[strings.Repeat("c", 40)] = []fake.CheckRun{
		{Name: "build", Status: "completed", Conclusion: "failure", CompletedAt: "2026-10-16T09:00:00Z"},
		{Name: "lint", Status: "completed", Conclusion: "success", CompletedAt: "2026-10-16T09:00:00Z"},
		{Name: "docs", Status: "completed", Conclusion: "failure", CompletedAt: "2026-10-14T09:00:00Z"},
	}
	g.Releases = map[string]*fake.Release{
		"v1.0.0": {ID: 1, TagName: "v1.0.0", PublishedAt: "2026-10-16T09:30:00Z"},
		"v1.1.0": {ID: 2, TagName: "v1.1.0", Draft: true},
	}
	now := time.Date(2026, 10, 16, 10, 0, 0, 0, time.UTC)
	env := map[string]string{"AAYU_GITHUB_TOKEN": "t"}
	opts := Options{Dir: dir, GitHubAPI: g.URL(), GitHubGap: -1, Now: func() time.Time { return now }}
	if code, _, errOut := run(t, env, opts, "install", "--loops", "--repo", "o/r", "--host", "amp", "--record", "feedback=sub-9"); code != 0 {
		t.Fatalf("record: exit %d: %s", code, errOut)
	}
	code, out, errOut := run(t, env, opts, "hook", "session-start", "--host", "claude")
	if code != 0 {
		t.Fatalf("session-start: exit %d: %s", code, errOut)
	}
	for _, want := range []string{"since 2026-10-15T10:00:00Z", "Loop triage-new", "- issue #1 opened\n", `- pull request #4: check "build" completed with failure on ccccccc`,
		"- release v1.0.0 published\n", "run `aayu hook loop:release-measure`"} {
		if !strings.Contains(out, want) {
			t.Errorf("session-start lacks %q:\n%s", want, out)
		}
	}
	for _, not := range []string{"#2", "Loop feedback", `"lint"`, `"docs"`, "v1.1.0", "New intake"} {
		if strings.Contains(out, not) {
			t.Errorf("session-start reports %q (old, recorded on amp, not matching, a draft, or untrusted text):\n%s", not, out)
		}
	}
	if _, out, _ := run(t, env, opts, "hook", "session-start", "--host", "claude"); strings.Contains(out, "aayudha: since") {
		t.Errorf("the second session reports the same events again:\n%s", out)
	}
	now = now.Add(time.Hour)
	g.AddIssue("Newer intake", "", "intake").CreatedAt = "2026-10-16T10:30:00Z"
	_, out, _ = run(t, env, opts, "hook", "session-start", "--host", "cursor")
	var cursor struct {
		Context string `json:"additional_context"`
	}
	if err := json.Unmarshal([]byte(out), &cursor); err != nil || !strings.Contains(cursor.Context, "issue #5 opened") || strings.Contains(cursor.Context, "#1 opened") {
		t.Errorf("cursor session-start: %v %q", err, out)
	}
}
