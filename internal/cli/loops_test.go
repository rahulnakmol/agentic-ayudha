package cli

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/loops"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof/fakes3"
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

// Bug #172: `init --gates` on a repository without loops/ renders the default proof-purge loop (its definition,
// the Actions job, the Amp schedule, and the Cursor automation), each run starting with the guard; aayudha's own
// generated loops carry it too.
func TestInitGatesRendersProofPurge(t *testing.T) {
	g := fake.New(t, "o/r", "User")
	dir := checkout(t)
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir, Build: BuildInfo{Version: "0.4.0"}}
	if code, _, errOut := run(t, map[string]string{"AAYU_GITHUB_TOKEN": "human-token"}, opts, "init", "--yes", "--gates"); code != 0 {
		t.Fatalf("init --gates: exit %d: %s", code, errOut)
	}
	files := snapshot(t, dir)
	if files["loops/proof-purge.toml"] != loops.Defaults["proof-purge"] || !strings.Contains(files["loops/amp/proof-purge.json"], "aayu hook loop:proof-purge") ||
		!strings.Contains(files["loops/cursor/aayudha-loops.tf"], "aayu proof purge") {
		t.Fatalf("init --gates did not write the proof-purge loop: %v", slices.Sorted(maps.Keys(files)))
	}
	own, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", loops.Workflow))
	if err != nil {
		t.Fatal(err)
	}
	for name, wf := range map[string]string{"init --gates": files[".github/workflows/"+loops.Workflow], "aayudha's go generate": string(own)} {
		guard, purge := strings.Index(wf, "run: aayudha hook loop:proof-purge"), strings.Index(wf, "aayudha proof purge --ci")
		if guard < 0 || purge < guard || !strings.Contains(wf, "# proof-purge: Daily at 03:40") || !strings.Contains(wf, "AAYU_SECRET_PROOF_MEDIA: ${{ secrets.AAYU_SECRET_PROOF_MEDIA }}") {
			t.Errorf("%s: the Actions loops lack a scheduled proof-purge job that starts with its guard", name)
		}
	}
}

// Bug #172: the proof-purge loop, run as its Actions job runs it (guard, then `proof purge` as the bot), deletes the
// media of a pack older than [proof] retention_days and tombstones it, and leaves a newer pack alone; while halted
// it exits 4 and purges nothing.
func TestLoopProofPurge(t *testing.T) {
	_, dir, _, env, opts, _ := proofFixture(t)
	s3 := fakes3.New(t, "sk")
	cfg := "version = 1\n[proof]\nmedia = \"s3://proof-bucket/acme?endpoint=" + s3.Server.URL + "\"\nretention_days = 30\n"
	_ = os.WriteFile(filepath.Join(dir, ".aayudha", "config.toml"), []byte(cfg), 0o644)
	_ = os.MkdirAll(filepath.Join(dir, "loops"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "loops", "proof-purge.toml"), []byte(loops.Defaults["proof-purge"]), 0o644)
	env["AAYU_SECRET_PROOF_MEDIA"] = "ak:sk"
	// Two packs of story #2, each with a screenshot in the bucket: one from June, one from 12 October.
	local := proof.LocalDir(dir, 2)
	for _, p := range []struct{ head, created string }{{headA, "2026-06-01T10:00:00Z"}, {strings.Repeat("b", 40), "2026-10-12T10:00:00Z"}} {
		pk, err := proof.LoadLocal(local)
		if err != nil {
			t.Fatal(err)
		}
		pk.PRHeadSHA, pk.CreatedAt = p.head, p.created
		files := map[string][]byte{}
		for i, a := range pk.Artifacts {
			files[a.Path] = []byte(a.Kind + " captured " + p.created) // a distinct capture per pack
			pk.Artifacts[i] = proof.NewArtifact(a.Path, a.Kind, files[a.Path])
		}
		if err := proof.WriteLocal(local, pk, files); err != nil {
			t.Fatal(err)
		}
		if code, _, errOut := run(t, env, opts, "proof", "push", "2", "--media", "--repo", "o/r"); code != 0 {
			t.Fatalf("push the %s pack: %s", p.created, errOut)
		}
	}
	if len(s3.Objects) != 2 {
		t.Fatalf("%d objects in the bucket, want the two screenshots", len(s3.Objects))
	}
	var newObj string
	for k, b := range s3.Objects {
		if strings.HasSuffix(string(b), "2026-10-12T10:00:00Z") {
			newObj = k
		}
	}

	event := filepath.Join(t.TempDir(), "event.json")
	_ = os.WriteFile(event, []byte(`{"schedule":"40 2 * * *"}`), 0o644) // 03:40 London in summer time
	opts.Now = func() time.Time { return time.Date(2026, 10, 15, 2, 40, 0, 0, time.UTC) }
	job := func(halt string) map[string]string {
		m := maps.Clone(env)
		maps.Copy(m, map[string]string{"GITHUB_TOKEN": "workflow-token", "AAYU_BOT_TOKEN": "bot-token", "GITHUB_ACTIONS": "true",
			"GITHUB_EVENT_NAME": "schedule", "GITHUB_EVENT_PATH": event, "GITHUB_OUTPUT": filepath.Join(t.TempDir(), "out"), "AAYUDHA_HALT": halt, "AAYU_AGENT": "1"})
		return m
	}

	// Halted: the guard and the command both exit 4; nothing is purged.
	halted := job("founder at 2026-10-15: test")
	if code, _, _ := run(t, halted, opts, "hook", "loop:proof-purge", "--ci", "--repo", "o/r"); code != ExitBlocked {
		t.Errorf("halted guard: exit %d, want %d", code, ExitBlocked)
	}
	if code, _, _ := run(t, halted, opts, "proof", "purge", "--ci", "--repo", "o/r"); code != ExitBlocked || len(s3.Objects) != 2 {
		t.Errorf("halted proof purge: exit %d, %d objects left, want %d and 2", code, len(s3.Objects), ExitBlocked)
	}

	// Due: the June pack's media go and its pack is tombstoned; the October pack is untouched.
	ok := job("")
	if code, _, errOut := run(t, ok, opts, "hook", "loop:proof-purge", "--ci", "--repo", "o/r"); code != 0 {
		t.Fatalf("guard: exit %d: %s", code, errOut)
	}
	if b, _ := os.ReadFile(ok["GITHUB_OUTPUT"]); string(b) != "due=true\n" {
		t.Fatalf("guard output %q, want due=true", b)
	}
	code, out, errOut := run(t, ok, opts, "proof", "purge", "--ci", "--repo", "o/r", "--json")
	if code != 0 || len(s3.Objects) != 1 || s3.Objects[newObj] == nil {
		t.Fatalf("proof purge: exit %d, objects %v (want only %s): %s %s", code, slices.Collect(maps.Keys(s3.Objects)), newObj, out, errOut)
	}
	var res proof.PurgeResult
	_ = json.Unmarshal([]byte(out), &res)
	if len(res.Packs) != 1 || !strings.Contains(res.Packs[0], headA) {
		t.Fatalf("tombstoned packs %v, want only the June pack (%s)", res.Packs, headA)
	}
}
