package cli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/inspect"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/models"
	"github.com/rahulnakmol/agentic-ayudha/internal/planning"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

func misses(context.Context, string) (bool, string, error) { return false, "coverage fail", nil }

func setLease(g *fake.GitHub, n int, holder string, exp time.Time) {
	g.Mu.Lock()
	defer g.Mu.Unlock()
	for _, c := range g.Issues[n].Comments {
		if leaseTag.MatchString(c.Body) {
			c.Body = leaseBody(holder, exp)
		}
	}
}

func comment(g *fake.GitHub, n int, prefix string) string {
	g.Mu.Lock()
	defer g.Mu.Unlock()
	for _, c := range g.Issues[n].Comments {
		if strings.HasPrefix(c.Body, prefix) {
			return c.Body
		}
	}
	return ""
}

// WP5.5 build: the lease (assignee, lock comment, label) refuses another
// live holder and is renewed after it expires; a floor miss on a checkpoint
// is a failed attempt retried as the same checkpoint; the attempts budget
// moves the story to status:blocked; --resume continues from the branch's
// last checkpoint trailer; the progress comment and the ledger state record
// it all.
func TestBuildLeaseAttemptsResume(t *testing.T) {
	g, env, opts, dir, bare, prN, _ := thinFixture(t)
	g.Mu.Lock()
	assignees := g.Issues[2].Assignees
	g.Mu.Unlock()
	if !contains(g.LabelNames(2), LeaseLabel) || len(assignees) != 1 || assignees[0].Login != "founder" || comment(g, 2, "<!-- aayudha:lease holder=founder@") == "" {
		t.Fatalf("lease after build: labels %v assignees %v", g.LabelNames(2), assignees)
	}
	// build moves the story from status:ready to status:in-progress, and its
	// later runs accept that (D-5.5c.17).
	if labels := g.LabelNames(2); !contains(labels, "status:in-progress") || contains(labels, "status:ready") {
		t.Fatalf("labels while building: %v", labels)
	}
	if p := comment(g, 2, progressMarker); !strings.Contains(p, "| 1/1 | 1 |") || !strings.Contains(p, "hold: floors hold") || !strings.Contains(p, "PR #"+strconv.Itoa(prN)) {
		t.Fatalf("progress comment:\n%s", p)
	}
	build := func(args ...string) (int, string) {
		code, out, errOut := run(t, env, opts, append([]string{"build", "2", "--repo", "o/r"}, args...)...)
		return code, out + errOut
	}
	setLease(g, 2, "other@laptop", time.Now().Add(time.Hour))
	if code, out := build(); code != ExitBlocked || !strings.Contains(out, "leased by other@laptop until") {
		t.Fatalf("build under another live lease: %d %s", code, out)
	}
	setLease(g, 2, "other@laptop", time.Now().Add(-time.Minute))
	wt := BuildWorktree(dir, 2)
	opts.CheckpointVerify = misses
	_ = os.WriteFile(filepath.Join(wt, "a.txt"), []byte("one\n"), 0o644)
	if code, out := build(); code != ExitFloor || !strings.Contains(out, "checkpoint 2/2: floor miss: coverage fail") {
		t.Fatalf("first miss: %d %s", code, out)
	}
	if c := comment(g, 2, "<!-- aayudha:lease holder=founder@"); c == "" {
		t.Fatal("the expired lease was not taken over")
	}
	_ = os.WriteFile(filepath.Join(wt, "a.txt"), []byte("two\n"), 0o644)
	if code, out := build(); code != ExitFloor || !strings.Contains(out, "checkpoint 2/2 (attempt 2)") {
		t.Fatalf("second miss: %d %s", code, out)
	}
	if msg := git(t, wt, "log", "-1", "--format=%B"); !strings.Contains(msg, "Aayudha-Checkpoint: 2/2") || !strings.Contains(msg, "Aayudha-Attempt: 2") {
		t.Fatalf("retry commit:\n%s", msg)
	}
	_ = os.WriteFile(filepath.Join(wt, "a.txt"), []byte("three\n"), 0o644)
	if code, out := build(); code != ExitBlocked || !strings.Contains(out, "attempts budget breached: 3 failed checkpoint attempts, budget 3") {
		t.Fatalf("third miss: %d %s", code, out)
	}
	if labels := g.LabelNames(2); !contains(labels, "status:blocked") || contains(labels, "status:ready") {
		t.Fatalf("labels after the budget breach: %v", labels)
	}
	if code, out := build(); code != ExitBlocked || !strings.Contains(out, "not labelled status:ready") {
		t.Fatalf("build while blocked: %d %s", code, out)
	}
	var st BuildState
	if err := json.Unmarshal([]byte(git(t, bare, "show", "ledger:"+BuildLedgerPath(2))), &st); err != nil || st.Attempts != 3 || len(st.Checkpoints) != 4 || !strings.HasPrefix(st.Blocked, "attempts budget") {
		t.Fatalf("ledger state: %v %+v", err, st)
	}
	if p := comment(g, 2, progressMarker); !strings.Contains(p, "**Blocked:** attempts budget breached") || !strings.Contains(p, "3 of 3 failed attempts") {
		t.Fatalf("progress comment when blocked:\n%s", p)
	}

	// A human makes it Ready again: a fresh budget, and --resume continues on another machine.
	if code, _, errOut := run(t, env, opts, "ready", "2", "--repo", "o/r"); code != 0 {
		t.Fatalf("ready again: %s", errOut)
	}
	git(t, wt, "push", "--quiet", "--force", "origin", "HEAD:refs/heads/story/2-show-empty-list")
	git(t, dir, "worktree", "remove", "--force", wt)
	if code, out := build(); code != ExitBlocked || !strings.Contains(out, "already exists on origin; `aayu build 2 --resume`") {
		t.Fatalf("build without --resume over a pushed branch: %d %s", code, out)
	}
	code, out, errOut := run(t, env, opts, "build", "2", "--resume", "--repo", "o/r", "--json")
	var res BuildResult
	_ = json.NewDecoder(strings.NewReader(out)).Decode(&res)
	if code != 0 || !res.Created || res.Resumed != "2/2" || res.Budget.Attempts != 0 {
		t.Fatalf("resume: %d %+v %s", code, res, errOut)
	}
	opts.CheckpointVerify = holds
	_ = os.WriteFile(filepath.Join(wt, "a.txt"), []byte("fixed\n"), 0o644)
	// --resume on the existing worktree reports the checkpoint it continues from.
	code, out, errOut = run(t, env, opts, "build", "2", "--resume", "--repo", "o/r", "--json")
	res = BuildResult{}
	_ = json.NewDecoder(strings.NewReader(out)).Decode(&res)
	if code != 0 || res.Resumed != "2/2" || res.Checkpoint != "2/2" || res.Verify != "hold" {
		t.Fatalf("after resume: %d %+v %s", code, res, errOut)
	}
}

// WP5.5 build budget: a second verify miss on a never layer blocks before the
// attempts budget, and metered spend over [autonomy.budget] cost_usd blocks at
// once, but only when aayu's own model calls are pay-as-you-go.
func TestBuildNeverLayerAndCost(t *testing.T) {
	g, env, opts, dir, _, _, _ := thinFixture(t)
	wt := BuildWorktree(dir, 2)
	opts.CheckpointVerify = misses
	_ = os.MkdirAll(filepath.Join(wt, "internal", "config"), 0o755)
	for i, want := range []int{ExitFloor, ExitBlocked} {
		_ = os.WriteFile(filepath.Join(wt, "internal", "config", "x.go"), []byte("package config // "+strconv.Itoa(i)+"\n"), 0o644)
		code, out, errOut := run(t, env, opts, "build", "2", "--repo", "o/r")
		if code != want {
			t.Fatalf("miss %d on L07: %d %s %s", i+1, code, out, errOut)
		}
		if i == 1 && !strings.Contains(out+errOut, "second verify failure on never layer L07") {
			t.Fatalf("second L07 miss: %s %s", out, errOut)
		}
	}
	if !contains(g.LabelNames(2), "status:blocked") {
		t.Fatalf("labels: %v", g.LabelNames(2))
	}
	if code, _, errOut := run(t, env, opts, "ready", "2", "--repo", "o/r"); code != 0 {
		t.Fatalf("ready again: %s", errOut)
	}
	// The cost limit (60 USD) applies only when aayu's own calls are
	// pay-as-you-go: a direct provider's key here. Spend is appended to the
	// clone's actions log, which build reads.
	logged := func(usd string) {
		t.Helper()
		p := filepath.Join(dir, ".aayudha", "telemetry", "actions.jsonl")
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = f.WriteString(`{"kind":"model.call","story":"#2","status":"done","cost_usd":` + usd + "}\n")
		_ = f.Close()
	}
	notBlocked := func(what string) {
		t.Helper()
		if _, out, errOut := run(t, env, opts, "build", "2", "--repo", "o/r"); strings.Contains(out+errOut, "cost budget breached") || contains(g.LabelNames(2), "status:blocked") {
			t.Fatalf("%s: blocked: %s %s", what, out, errOut)
		}
	}
	env["AAYU_SECRET_ANTHROPIC"] = "model-key"
	logged("60")
	notBlocked("metered, at 60 USD")
	logged("0.01")
	if code, out, errOut := run(t, env, opts, "build", "2", "--repo", "o/r"); code != ExitBlocked || !strings.Contains(out+errOut, "cost budget breached: metered spend 60.01 USD of 60 (subscription use not counted)") {
		t.Fatalf("metered, over the cost budget: %d %s %s", code, out, errOut)
	}
	// With no metered key or endpoint there is no cost limit, though the log
	// shows 100 USD for the story.
	if code, _, errOut := run(t, env, opts, "ready", "2", "--repo", "o/r"); code != 0 {
		t.Fatalf("ready again: %s", errOut)
	}
	delete(env, "AAYU_SECRET_ANTHROPIC")
	logged("39.99")
	_ = os.WriteFile(filepath.Join(wt, "notes.txt"), []byte("a checkpoint\n"), 0o644)
	_, out, errOut := run(t, env, opts, "build", "2", "--repo", "o/r")
	if strings.Contains(out+errOut, "cost budget breached") || contains(g.LabelNames(2), "status:blocked") ||
		!strings.Contains(out, "metered spend 100.00 USD, no cost limit: no pay-as-you-go model key or endpoint is configured (subscription use not counted)") {
		t.Fatalf("no metered key, 100 USD logged: %s %s", out, errOut)
	}
	// Spend a host reports in AAYU_COST_USD is metered: it counts, and the
	// limit applies to it.
	env["AAYU_COST_USD"] = "0.01"
	if code, out, errOut := run(t, env, opts, "build", "2", "--repo", "o/r"); code != ExitBlocked || !strings.Contains(out+errOut, "cost budget breached: metered spend 100.01 USD of 60 (subscription use not counted)") {
		t.Fatalf("AAYU_COST_USD reported, over the cost budget: %d %s %s", code, out, errOut)
	}
	// A cap set to 0 on purpose says so, rather than blaming a missing key;
	// a per_story_usd kept below cost_usd is the limit that stops the story.
	if code, _, errOut := run(t, env, opts, "ready", "2", "--repo", "o/r"); code != 0 {
		t.Fatalf("ready again: %s", errOut)
	}
	_ = os.WriteFile(filepath.Join(wt, "notes2.txt"), []byte("another checkpoint\n"), 0o644)
	_, out, errOut = run(t, env, opts, "build", "2", "--repo", "o/r", "--set", "autonomy.budget.cost_usd=0")
	if strings.Contains(out+errOut, "cost budget breached") || !strings.Contains(out, "metered spend 100.01 USD, no cost limit: autonomy.budget.cost_usd is 0 (subscription use not counted)") {
		t.Fatalf("cost_usd = 0: %s %s", out, errOut)
	}
	if code, out, errOut := run(t, env, opts, "build", "2", "--repo", "o/r", "--set", "budget.per_story_usd=25"); code != ExitBlocked || !strings.Contains(out+errOut, "cost budget breached: metered spend 100.01 USD of 25 (subscription use not counted)") {
		t.Fatalf("per_story_usd 25 below cost_usd 60: %d %s %s", code, out, errOut)
	}
}

// WP5.5 accept: a stale lease is reclaimed by gc (comment, label, and
// assignee), a stray lease label without a lock comment is removed, a live
// lease is left alone, and the worktree of a closed story is removed while an
// open story's is kept. --dry-run reports the same and changes nothing, and a
// story gc cannot read keeps its worktree.
func TestGCReclaimsStaleLease(t *testing.T) {
	g, env, opts, dir, _, _, _ := thinFixture(t)
	other := g.AddIssue("Other story", "x", "type:story", "status:ready", LeaseLabel) // a live lease
	g.Mu.Lock()
	other.Comments = append(other.Comments, &fake.Comment{ID: 501, Body: leaseBody("other@laptop", time.Now().Add(time.Hour))})
	g.Mu.Unlock()
	// A stray label: its lock comment's expiry does not parse, so it is no lock comment.
	stray := g.AddIssue("Stray story", "x", "type:story", "status:ready", LeaseLabel)
	g.Mu.Lock()
	stray.Comments = append(stray.Comments, &fake.Comment{ID: 502, Body: "<!-- aayudha:lease holder=someone@box expires=tomorrow -->"})
	g.Mu.Unlock()
	setLease(g, 2, "founder@old-laptop", time.Now().Add(-2*time.Hour))
	reclaimedStray := "lease #" + strconv.Itoa(stray.Number) + " reclaimed: labelled leased without a lock comment"
	code, out, errOut := run(t, env, opts, "gc", "--repo", "o/r", "--dry-run")
	if code != 0 || !strings.Contains(out, "lease #2 reclaimed: the lease of founder@old-laptop expired at") || !strings.Contains(out, reclaimedStray) ||
		!strings.HasSuffix(out, "--dry-run: nothing was changed\n") {
		t.Fatalf("gc --dry-run: %d %s %s", code, out, errOut)
	}
	if !contains(g.LabelNames(2), LeaseLabel) || !contains(g.LabelNames(stray.Number), LeaseLabel) || comment(g, 2, "<!-- aayudha:lease-released -->") != "" {
		t.Fatalf("gc --dry-run changed the leases: %v %v", g.LabelNames(2), g.LabelNames(stray.Number))
	}
	code, out, errOut = run(t, env, opts, "gc", "--repo", "o/r")
	if code != 0 || !strings.Contains(out, "lease #2 reclaimed: the lease of founder@old-laptop expired at") || !strings.Contains(out, "lease #"+strconv.Itoa(other.Number)+" live: other@laptop until") ||
		!strings.Contains(out, reclaimedStray) || !strings.Contains(out, "worktree #2 kept: the story is open") || strings.Contains(out, "--dry-run") {
		t.Fatalf("gc: %d %s %s", code, out, errOut)
	}
	if contains(g.LabelNames(stray.Number), LeaseLabel) {
		t.Fatalf("stray lease label left: %v", g.LabelNames(stray.Number))
	}
	g.Mu.Lock()
	assignees := g.Issues[2].Assignees
	g.Mu.Unlock()
	if contains(g.LabelNames(2), LeaseLabel) || len(assignees) != 0 || !strings.Contains(comment(g, 2, "<!-- aayudha:lease-released -->"), "reclaimed by `aayu gc`") {
		t.Fatalf("after gc: labels %v assignees %v", g.LabelNames(2), assignees)
	}
	if !contains(g.LabelNames(other.Number), LeaseLabel) {
		t.Fatal("gc reclaimed a live lease")
	}
	g.Mu.Lock()
	g.Issues[2].State = "closed"
	g.Mu.Unlock()
	// --dry-run names the worktree it would remove and leaves it; a story gc cannot read keeps its worktree.
	if code, out, errOut := run(t, env, opts, "gc", "--repo", "o/r", "--dry-run"); code != 0 || !strings.Contains(out, "worktree #2 removed: the story is closed") || !dirExists(BuildWorktree(dir, 2)) {
		t.Fatalf("gc --dry-run of a closed story's worktree: %d %s %s", code, out, errOut)
	}
	g.Mu.Lock()
	g.Fail = "GET /repos/o/r/issues/2"
	g.Mu.Unlock()
	if code, out, errOut := run(t, env, opts, "gc", "--repo", "o/r"); code != ExitExternal || !dirExists(BuildWorktree(dir, 2)) {
		t.Fatalf("gc when the story cannot be read: %d %s %s", code, out, errOut)
	}
	if code, out, errOut := run(t, env, opts, "gc", "--repo", "o/r"); code != 0 || !strings.Contains(out, "worktree #2 removed: the story is closed") {
		t.Fatalf("gc of a closed story's worktree: %d %s %s", code, out, errOut)
	}
	if _, err := os.Stat(BuildWorktree(dir, 2)); !os.IsNotExist(err) {
		t.Fatalf("worktree left: %v", err)
	}
}

// WP5.5 ship: the human pass comes from the [autonomy] approver in solo mode
// (a bot's approval is never one); a never layer needs the EM's plan-time
// review and the approval of a login that is a CODEOWNER of every never-layer
// path, whatever the streak; the ship record
// keeps the pass rule, the approval, and the streak, and the lease is
// released.
func TestShipPassRules(t *testing.T) {
	g, env, opts, dir, bare, prN, head := thinFixture(t)
	gateRun(t, g, dir, prN, head, []proof.Check{{Floor: "coverage", Result: proof.Pass, Value: 100, Threshold: 90, Tool: "aayudha", Version: "1"}}, "L02", "L07")
	g.Mu.Lock()
	g.Pulls[prN].Files = []string{"internal/cli/x.go", "internal/config/x.go", "internal/ledger/x.go"}
	g.Files = map[string]string{}
	g.Files[".github/CODEOWNERS"] = "/internal/config/ @owner1 @reviewer2\n/internal/ledger/ @reviewer2 @owner1\n"
	// The approver and the layers are the base branch's committed policy (#645).
	g.Files[".aayudha/config.toml"] = "version = 1\n[autonomy]\napprover = \"reviewer2\"\n"
	if b, err := os.ReadFile("../../LAYERS.md"); err == nil {
		g.Files["LAYERS.md"] = string(b)
	} else {
		t.Fatal(err)
	}
	g.Pulls[prN].Reviews = []fake.Review{review("aayudha-bot[bot]", "Bot", "APPROVED", head), review("reviewer", "User", "APPROVED", head)}
	g.Mu.Unlock()
	ship := func(args ...string) (int, string) {
		code, out, errOut := run(t, env, opts, append([]string{"ship", strconv.Itoa(prN), "--repo", "o/r"}, args...)...)
		return code, out + errOut
	}
	if code, out := ship(); code != ExitBlocked || !strings.Contains(out, "plan-time review is required") {
		t.Fatalf("never layer without the EM review: %d %s", code, out)
	}
	// With proof off, a narrow-set path holds the streak back, and the
	// refusal says so rather than naming a merge on proof (#680).
	g.Mu.Lock()
	g.Pulls[prN].Files = append(g.Pulls[prN].Files, ".aayudha/config.local.json")
	g.Mu.Unlock()
	if code, out := ship(); code != ExitBlocked || !strings.Contains(out, "plan-time review is required") || !strings.Contains(out, "; no merge on the streak: it touches .aayudha/config.local.json") || strings.Contains(out, "no merge on proof") {
		t.Fatalf("never layer without the EM review, a narrow-set path, proof off: %d %s", code, out)
	}
	g.Mu.Lock()
	g.Pulls[prN].Files = g.Pulls[prN].Files[:3]
	g.Mu.Unlock()
	// A solo-mode plan-review bypass satisfies the plan check (the human pass is checked next).
	bypass := PlanRecord{Story: 2, Footprint: []string{"L02", "L07"}, Review: planning.Review{Required: true, Layers: []string{"L07"},
		Bypass: &planning.ReviewBypass{As: "solo-bypass", Reviewer: "none (solo mode)", RecordedBy: "founder", Comment: "https://example.test/c/1"}}}
	b, _ := json.Marshal(bypass)
	if _, err := (&ledger.Writer{Dir: dir}).Write(context.Background(), ledger.Commit{Message: "plan bypass", Files: map[string][]byte{PlanLedgerPath(2): b}}); err != nil {
		t.Fatal(err)
	}
	if code, out := ship(); code != ExitBlocked || strings.Contains(out, "plan-time review is required") || !strings.Contains(out, "no human pass") {
		t.Fatalf("never layer with a plan-review bypass: %d %s", code, out)
	}
	rec := PlanRecord{Story: 2, Footprint: []string{"L02", "L07"}, Review: planning.Review{Required: true, Layers: []string{"L07"}, ApprovedBy: "founder", As: "solo", ApprovedAt: "2026-10-12T10:00:00Z"}}
	b, _ = json.Marshal(rec)
	if _, err := (&ledger.Writer{Dir: dir}).Write(context.Background(), ledger.Commit{Message: "plan", Files: map[string][]byte{PlanLedgerPath(2): b}}); err != nil {
		t.Fatal(err)
	}
	code, out := ship()
	if code != ExitBlocked || !strings.Contains(out, "from a bot identity and is not a human pass") || !strings.Contains(out, "approval by reviewer is not the human pass: it must come from reviewer2 (approver)") {
		t.Fatalf("bot and non-approver reviews: %d %s", code, out)
	}
	g.Mu.Lock()
	g.Files[".github/CODEOWNERS"] = "/internal/config/ @owner1\n/internal/ledger/ @reviewer2 @owner1\n"
	g.Pulls[prN].Reviews = append(g.Pulls[prN].Reviews, review("reviewer2", "User", "APPROVED", head))
	g.Mu.Unlock()
	if code, out := ship(); code != ExitBlocked || !strings.Contains(out, "does not cover never layer(s) L07: a CODEOWNER of internal/config/x.go, internal/ledger/x.go must approve (owners: owner1), whatever the streak") {
		t.Fatalf("approver who is no CODEOWNER of the never layer: %d %s", code, out)
	}
	g.Mu.Lock()
	g.Files[".github/CODEOWNERS"] = "/internal/config/ @owner1 @reviewer2\n/internal/ledger/ @reviewer2 @owner1\n"
	g.Mu.Unlock()
	g.Reset()
	if code, out := ship(); code != 0 || !strings.Contains(out, "human pass: review by reviewer2") {
		t.Fatalf("ship: %d %s", code, out)
	}
	if n := closedPullLists(g); n != 0 { // an approving review settles the pass: the streak is not read (#701)
		t.Errorf("a review-pass ship read the merged-PR history for the streak: %d list(s)", n)
	}
	var sr ShipRecord
	_ = json.Unmarshal([]byte(git(t, bare, "show", "ledger:proof/2/"+head+"/authoritative/ship.json")), &sr)
	if sr.HumanPass.ReviewID == 0 || sr.Pass.As != "approver" || strings.Join(sr.Pass.Never, ",") != "L07" || sr.PlanReview == nil || sr.PlanReview.ApprovedBy != "founder" ||
		sr.Streak.Threshold != 20 || sr.Streak.Autonomous || sr.Streak.Skipped != "the human pass is settled: review" {
		t.Fatalf("ship record: %+v", sr)
	}
	if contains(g.LabelNames(2), LeaseLabel) || !strings.Contains(comment(g, 2, "<!-- aayudha:lease-released -->"), "shipped in #"+strconv.Itoa(prN)) {
		t.Fatalf("lease after ship: %v", g.LabelNames(2))
	}
}

// gc keeps the worktree of a closed story that still holds work and says how
// to free it: uncommitted changes, then commits that are not on origin.
func TestGCKeepsBusyWorktree(t *testing.T) {
	g, env, opts, dir, _, _, _ := thinFixture(t)
	g.Mu.Lock()
	g.Issues[2].State = "closed"
	g.Mu.Unlock()
	wt := BuildWorktree(dir, 2)
	_ = os.WriteFile(filepath.Join(wt, "draft.txt"), []byte("wip\n"), 0o644)
	if code, out, errOut := run(t, env, opts, "gc", "--repo", "o/r"); code != 0 ||
		!strings.Contains(out, "worktree #2 kept: the story is closed, but it has uncommitted changes; commit or discard them, then run `aayu gc` again") {
		t.Fatalf("gc with uncommitted changes: %d %s %s", code, out, errOut)
	}
	git(t, wt, "add", "draft.txt")
	git(t, wt, "commit", "--quiet", "-m", "wip")
	if code, out, errOut := run(t, env, opts, "gc", "--repo", "o/r"); code != 0 ||
		!strings.Contains(out, "has 1 commit(s) not on origin; push them or delete the branch, then run `aayu gc` again") {
		t.Fatalf("gc with unpushed commits: %d %s %s", code, out, errOut)
	}
	if _, err := os.Stat(wt); err != nil {
		t.Fatalf("a busy worktree was removed: %v", err)
	}
}

// buildSetup is story #2 Ready on a ledger checkout, before any build run.
func buildSetup(t *testing.T) (g *fake.GitHub, env map[string]string, opts Options, dir, bare string) {
	t.Helper()
	g = readyTree(t)
	dir, bare = ledgerCheckout(t)
	git(t, dir, "config", "user.name", "Builder")
	git(t, dir, "config", "user.email", "builder@example.com")
	env = map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "PATH": os.Getenv("PATH")}
	opts = Options{GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir, CheckpointVerify: holds}
	return g, env, opts, dir, bare
}

// WP5.5 build as a person runs it: text output; a long title becomes a
// branch slug of at most 40 characters; a body edited after Ready is refused;
// --dry-run commits the checkpoint locally but writes nothing to GitHub or the
// ledger; the plan on the ledger sets N; the next run numbers its checkpoint
// after the branch's last trailer; a verify that cannot run is noted and is
// not a failed attempt; the model comes from [models.build.<host>]; the
// progress comment is edited in place; and a build past the wall-time budget
// is blocked.
func TestBuildTextDryRunPlanAndBudget(t *testing.T) {
	g, env, opts, dir, bare := buildSetup(t)
	g.Mu.Lock()
	g.Issues[2].Title = "Show the empty list message when a cart has nothing in it"
	g.Mu.Unlock()
	if code, _, errOut := run(t, env, opts, "ready", "2", "--repo", "o/r"); code != 0 {
		t.Fatalf("ready: %s", errOut)
	}
	if code, _, errOut := run(t, env, opts, "build", "99", "--repo", "o/r"); code != ExitExternal || !strings.Contains(errOut, "404") {
		t.Fatalf("build of an issue that does not exist: %d %s", code, errOut)
	}
	build := func(args ...string) (int, string) {
		code, out, errOut := run(t, env, opts, append([]string{"build", "2", "--repo", "o/r", "--host", "amp", "--set", "models.build.amp.model=claude-build-1"}, args...)...)
		return code, out + errOut
	}

	g.Mu.Lock()
	body := g.Issues[2].Body
	g.Issues[2].Body = body + "\nedited after Ready\n"
	g.Mu.Unlock()
	if code, out := build(); code != ExitBlocked || !strings.Contains(out, "its body changed after Ready (snapshot") {
		t.Fatalf("build after a body edit: %d %s", code, out)
	}
	g.Mu.Lock()
	g.Issues[2].Body = body
	g.Mu.Unlock()

	wt := BuildWorktree(dir, 2)
	branch := "story/2-show-the-empty-list-message-when-a-cart"
	code, out := build()
	if code != 0 || !strings.Contains(out, "#2 Show the empty list message when a cart has nothing in it: worktree "+wt+" on "+branch+" (leased by founder@") ||
		!strings.Contains(out, "next: write the change in "+wt) {
		t.Fatalf("build (worktree): %d %s", code, out)
	}

	plan := PlanRecord{Story: 2, Checkpoints: make([]planning.Checkpoint, 3)}
	b, _ := json.Marshal(plan)
	if _, err := (&ledger.Writer{Dir: dir}).Write(context.Background(), ledger.Commit{Message: "plan", Files: map[string][]byte{PlanLedgerPath(2): b}}); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(wt, "empty.txt"), []byte("Your cart is empty\n"), 0o644)
	g.Reset()
	if code, out := build("--dry-run"); code != 0 || !strings.Contains(out, "#2: checkpoint 1/3 committed") {
		t.Fatalf("build --dry-run: %d %s", code, out)
	}
	var st BuildState
	if w := g.Writes(); len(w) != 0 {
		t.Fatalf("--dry-run wrote to GitHub: %+v", w)
	}
	if err := json.Unmarshal([]byte(git(t, bare, "show", "ledger:"+BuildLedgerPath(2))), &st); err != nil || len(st.Checkpoints) != 0 {
		t.Fatalf("--dry-run changed the ledger state: %v %+v", err, st)
	}
	if out := git(t, bare, "branch", "--list", branch); out != "" {
		t.Fatalf("--dry-run pushed the branch: %s", out)
	}

	opts.CheckpointVerify = func(context.Context, string) (bool, string, error) {
		return false, "", errors.New("go: not found")
	}
	_ = os.WriteFile(filepath.Join(wt, "total.txt"), []byte("Total: 0\n"), 0o644)
	code, out = build()
	if code != 0 || !strings.Contains(out, "#2: checkpoint 2/3 committed") || !strings.Contains(out, "verify: error") ||
		!strings.Contains(out, "note: verify could not run on the checkpoint: go: not found") || !strings.Contains(out, "#2: branch "+branch+" pushed; PR #") {
		t.Fatalf("build with a verify error: %d %s", code, out)
	}
	if msg := git(t, wt, "log", "-1", "--format=%B"); !strings.Contains(msg, "Aayudha-Checkpoint: 2/3") || !strings.Contains(msg, "Aayudha-Model: claude-build-1") || !strings.Contains(msg, "Aayudha-Host: amp") {
		t.Fatalf("checkpoint commit:\n%s", msg)
	}
	if err := json.Unmarshal([]byte(git(t, bare, "show", "ledger:"+BuildLedgerPath(2))), &st); err != nil || st.Attempts != 0 || len(st.Checkpoints) != 1 || st.Checkpoints[0].Verify != "error" || st.Checkpoints[0].Model != "claude-build-1" {
		t.Fatalf("ledger state after a verify error (the checkpoint records its model for inspect, Bug #171): %v %+v", err, st)
	}

	opts.CheckpointVerify = holds
	_ = os.WriteFile(filepath.Join(wt, "total.txt"), []byte("Total: 0.00\n"), 0o644)
	if code, out := build(); code != 0 || !strings.Contains(out, "#2: checkpoint 3/3 committed") {
		t.Fatalf("third checkpoint: %d %s", code, out)
	}
	n := 0
	g.Mu.Lock()
	for _, c := range g.Issues[2].Comments {
		if strings.HasPrefix(c.Body, progressMarker) {
			n++
		}
	}
	g.Mu.Unlock()
	if p := comment(g, 2, progressMarker); n != 1 || !strings.Contains(p, "| 2/3 | 1 |") || !strings.Contains(p, "error: go: not found") || !strings.Contains(p, "| 3/3 | 1 |") {
		t.Fatalf("%d progress comment(s):\n%s", n, p)
	}

	opts.Now = func() time.Time { return time.Now().Add(9 * time.Hour) }
	if code, out := build(); code != ExitBlocked || !strings.Contains(out, "wall-time budget breached: 9.0 h since the build started, budget 8.0 h") {
		t.Fatalf("build past the wall-time budget: %d %s", code, out)
	}
	if !contains(g.LabelNames(2), "status:blocked") {
		t.Fatalf("labels after the wall-time breach: %v", g.LabelNames(2))
	}
}

// WP5.5 build without a CheckpointVerify stand-in runs `aayu verify` in the
// worktree: a floor that cannot run because nothing configures it is a floor
// miss, and once it passes, a floor whose tool is not installed here does not
// count as a miss (the merge gate runs it) but is named.
func TestBuildRunsVerify(t *testing.T) {
	_, env, opts, dir, bare := buildSetup(t)
	opts.CheckpointVerify = nil
	if code, _, errOut := run(t, env, opts, "ready", "2", "--repo", "o/r"); code != 0 {
		t.Fatalf("ready: %s", errOut)
	}
	if code, out, errOut := run(t, env, opts, "build", "2", "--repo", "o/r"); code != 0 {
		t.Fatalf("build (worktree): %d %s %s", code, out, errOut)
	}
	wt := BuildWorktree(dir, 2)
	state := func() CheckpointRecord {
		var st BuildState
		if err := json.Unmarshal([]byte(git(t, bare, "show", "ledger:"+BuildLedgerPath(2))), &st); err != nil || len(st.Checkpoints) == 0 {
			t.Fatalf("ledger state: %v %+v", err, st)
		}
		return st.Checkpoints[len(st.Checkpoints)-1]
	}
	_ = os.WriteFile(filepath.Join(wt, "empty.txt"), []byte("Your cart is empty\n"), 0o644)
	if code, out, errOut := run(t, env, opts, "build", "2", "--repo", "o/r"); code != ExitFloor || !strings.Contains(out+errOut, "floor miss: coverage blocked") {
		t.Fatalf("build without a coverage command: %d %s %s", code, out, errOut)
	}
	if cp := state(); cp.Verify != "miss" || !strings.HasPrefix(cp.Detail, "coverage blocked") {
		t.Fatalf("checkpoint record: %+v", cp)
	}

	report := filepath.Join(t.TempDir(), "cover.out")
	_ = os.WriteFile(report, []byte("mode: set\n"), 0o644)
	_ = os.WriteFile(filepath.Join(wt, "empty.txt"), []byte("Your cart is empty.\n"), 0o644)
	code, out, errOut := run(t, env, opts, "build", "2", "--repo", "o/r", "--set", "verify.floors.coverage.command=true", "--set", "verify.floors.coverage.report="+report)
	cp := state()
	if strings.Contains(cp.Detail, "coverage") {
		t.Fatalf("coverage still misses: %d %+v %s %s", code, cp, out, errOut)
	}
	var absent []string
	for _, tool := range []string{"semgrep", "gitleaks", "osv-scanner"} {
		if _, err := exec.LookPath(tool); err != nil {
			absent = append(absent, tool)
		}
	}
	if len(absent) == 3 && (code != 0 || cp.Verify != "hold" || !strings.HasPrefix(cp.Detail, "floors hold; not run here (tool not installed; the merge gate runs them): ") ||
		!strings.Contains(cp.Detail, "sast") || !strings.Contains(out, "verify: hold")) {
		t.Fatalf("build with the scanners not installed: %d %+v %s %s", code, cp, out, errOut)
	}

	// A checkpoint that breaks the repository's config: verify cannot run, which is noted, not a miss.
	_ = os.MkdirAll(filepath.Join(wt, ".aayudha"), 0o755)
	_ = os.WriteFile(filepath.Join(wt, ".aayudha", "config.toml"), []byte("version = [\n"), 0o644)
	code, out, errOut = run(t, env, opts, "build", "2", "--repo", "o/r")
	if cp := state(); code != 0 || cp.Verify != "error" || !strings.Contains(out, "note: verify could not run on the checkpoint: ") || !strings.Contains(cp.Detail, "config.toml") {
		t.Fatalf("build with a broken config: %d %+v %s %s", code, cp, out, errOut)
	}
}

// build in a repository without a ledger branch stops with exit 5 and says
// to run init.
func TestBuildWithoutLedger(t *testing.T) {
	_, env, opts, dir, _ := buildSetup(t)
	if code, _, errOut := run(t, env, opts, "ready", "2", "--repo", "o/r"); code != 0 {
		t.Fatalf("ready: %s", errOut)
	}
	git(t, dir, "push", "--quiet", "origin", "--delete", "ledger")
	if code, out, errOut := run(t, env, opts, "build", "2", "--repo", "o/r"); code != ExitExternal || !strings.Contains(errOut, "run `aayu init` to create it") {
		t.Fatalf("build without a ledger: %d %s %s", code, out, errOut)
	}
}

// WP5.5 build under a bot App: the lease holder is the App's [bot] login,
// which GitHub cannot assign, so the story gets the lock comment and label but
// no assignee. With neither the bot's token nor the human's (which the App's
// token broker needs), build exits 4. A bot-only session (AAYU_BOT_TOKEN and
// no human token, Decision #488) needs no broker.
func TestBuildUnderBotApp(t *testing.T) {
	g, env, opts, _, _ := buildSetup(t)
	if code, _, errOut := run(t, env, opts, "ready", "2", "--repo", "o/r"); code != 0 {
		t.Fatalf("ready: %s", errOut)
	}
	app := []string{"build", "2", "--repo", "o/r", "--json", "--set", "identity.app_id=42", "--set", "identity.app_slug=aayudha-bot"}
	if code, _, errOut := run(t, map[string]string{"PATH": os.Getenv("PATH")}, opts, app...); code != ExitBlocked || !strings.Contains(errOut, "no GitHub token found") {
		t.Fatalf("build under the App without any token: %d %s", code, errOut)
	}
	env["AAYU_BOT_TOKEN"] = "bot-token"
	code, out, errOut := run(t, env, opts, app...)
	var res BuildResult
	_ = json.NewDecoder(strings.NewReader(out)).Decode(&res)
	g.Mu.Lock()
	assignees := g.Issues[2].Assignees
	g.Mu.Unlock()
	if code != 0 || !strings.HasPrefix(res.Lease.Holder, "aayudha-bot[bot]@") || res.Lease.Assignee != "" || len(assignees) != 0 ||
		!contains(g.LabelNames(2), LeaseLabel) || comment(g, 2, "<!-- aayudha:lease holder=aayudha-bot[bot]@") == "" {
		t.Fatalf("build under the App: %d %+v assignees %v %s", code, res, assignees, errOut)
	}
}

// WP5.5: build takes the lease before any work, so a lease call that GitHub
// fails stops it with exit 5 and commits no checkpoint.
func TestBuildStopsWhenTheLeaseFails(t *testing.T) {
	g, env, opts, dir, _, _, head := thinFixture(t)
	wt := BuildWorktree(dir, 2)
	_ = os.WriteFile(filepath.Join(wt, "a.txt"), []byte("one\n"), 0o644)
	lockID := func() int64 {
		g.Mu.Lock()
		defer g.Mu.Unlock()
		for _, c := range g.Issues[2].Comments {
			if leaseTag.MatchString(c.Body) {
				return c.ID
			}
		}
		return 0
	}
	tests := []struct {
		name, fail string
		after      int // successful calls before the failure
		before     func()
	}{
		{"read the lock comment", "GET /repos/o/r/issues/2/comments", 1, nil}, // the first read is the Ready snapshot check
		{"renew the lock comment", "PATCH /repos/o/r/issues/comments/" + strconv.FormatInt(lockID(), 10), 0, nil},
		{"label the story", "POST /repos/o/r/issues/2/labels", 0, func() {
			g.Mu.Lock()
			g.Issues[2].Labels = slices.DeleteFunc(g.Issues[2].Labels, func(l fake.Label) bool { return l.Name == LeaseLabel })
			g.Mu.Unlock()
		}},
		{"assign the new holder", "POST /repos/o/r/issues/2/assignees", 0, func() { setLease(g, 2, "other@laptop", time.Now().Add(-time.Minute)) }},
		{"post the lock comment", "POST /repos/o/r/issues/2/comments", 0, func() {
			g.Mu.Lock()
			g.Issues[2].Comments = slices.DeleteFunc(g.Issues[2].Comments, func(c *fake.Comment) bool { return leaseTag.MatchString(c.Body) })
			g.Mu.Unlock()
		}},
	}
	for _, tt := range tests {
		if tt.before != nil {
			tt.before()
		}
		g.Mu.Lock()
		g.Fail, g.FailAfter = tt.fail, tt.after
		g.Mu.Unlock()
		code, out, errOut := run(t, env, opts, "build", "2", "--repo", "o/r")
		if code != ExitExternal || git(t, wt, "rev-parse", "HEAD") != head {
			t.Errorf("%s fails: %d %s %s", tt.name, code, out, errOut)
		}
	}
}

// gc with nothing to collect says so; it needs a repository and the human's
// GitHub token.
func TestGCNothingToCollect(t *testing.T) {
	g := fake.New(t, "o/r", "User")
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1}
	if code, out, errOut := run(t, map[string]string{"AAYU_GITHUB_TOKEN": "human-token"}, opts, "gc", "--repo", "o/r"); code != 0 || out != "gc: no leases or build worktrees\n" {
		t.Fatalf("gc with nothing to collect: %d %q %s", code, out, errOut)
	}
	if code, _, errOut := run(t, map[string]string{"AAYU_GITHUB_TOKEN": "human-token"}, opts, "gc"); code != ExitUsage || !strings.Contains(errOut, "no repository") {
		t.Fatalf("gc outside a checkout: %d %s", code, errOut)
	}
	if code, _, errOut := run(t, nil, opts, "gc", "--repo", "o/r"); code != ExitBlocked {
		t.Fatalf("gc without a token: %d %s", code, errOut)
	}
}

// When GitHub fails while build reads the Ready snapshot, the story may well
// be Ready: build exits 5 (external), not 4 (blocked), and says what failed.
func TestBuildSnapshotReadFailureIsExternal(t *testing.T) {
	g, env, opts, _, _ := buildSetup(t)
	if code, _, errOut := run(t, env, opts, "ready", "2", "--repo", "o/r"); code != 0 {
		t.Fatalf("ready: %s", errOut)
	}
	g.Mu.Lock()
	g.Fail = "GET /repos/o/r/issues/2/comments" // the first comments read is the snapshot's
	g.Mu.Unlock()
	if code, _, errOut := run(t, env, opts, "build", "2", "--repo", "o/r"); code != ExitExternal || !strings.Contains(errOut, "reading the Ready snapshot") {
		t.Fatalf("build with the snapshot read failing: exit %d %s; want %d", code, errOut, ExitExternal)
	}
}

// When opening the PR fails after the checkpoint was committed and pushed,
// the checkpoint is still recorded in build/<n>/state.json, so the next run
// continues after it instead of retrying the checkpoint it already passed.
func TestBuildRecordsCheckpointWhenPRCreationFails(t *testing.T) {
	g, env, opts, dir, bare := buildSetup(t)
	if code, _, errOut := run(t, env, opts, "ready", "2", "--repo", "o/r"); code != 0 {
		t.Fatalf("ready: %s", errOut)
	}
	build := func(file string) (int, BuildResult, string) {
		if file != "" {
			_ = os.WriteFile(filepath.Join(BuildWorktree(dir, 2), file), []byte(file+"\n"), 0o644)
		}
		code, out, errOut := run(t, env, opts, "build", "2", "--repo", "o/r", "--json")
		var res BuildResult
		_ = json.NewDecoder(strings.NewReader(out)).Decode(&res)
		return code, res, errOut
	}
	if code, _, errOut := build(""); code != 0 {
		t.Fatalf("build (worktree): %d %s", code, errOut)
	}
	opts.CheckpointVerify = misses
	if code, _, errOut := build("a.txt"); code != ExitFloor {
		t.Fatalf("checkpoint 1, attempt 1: %d %s", code, errOut)
	}
	opts.CheckpointVerify = holds
	g.Mu.Lock()
	g.Fail = "POST /repos/o/r/pulls"
	g.Mu.Unlock()
	if code, _, errOut := build("b.txt"); code != ExitExternal {
		t.Fatalf("checkpoint 1, attempt 2, with the PR create failing: %d %s", code, errOut)
	}
	var st BuildState
	if err := json.Unmarshal([]byte(git(t, bare, "show", "ledger:"+BuildLedgerPath(2))), &st); err != nil {
		t.Fatal(err)
	}
	last := st.Checkpoints[len(st.Checkpoints)-1]
	if head := git(t, BuildWorktree(dir, 2), "rev-parse", "HEAD"); last.Commit != head || last.K != 1 || last.Attempt != 2 || last.Verify != "hold" {
		t.Fatalf("ledger state after the failed PR create: last checkpoint %+v, head %s", last, head)
	}
	code, res, errOut := build("c.txt")
	if code != 0 || res.Checkpoint != "2/2" || res.Attempt != 1 || res.PR == 0 {
		t.Fatalf("next checkpoint: %d %+v %s", code, res, errOut)
	}
}

// A story titled with its issue form's prefix ("Story: …") gets a branch and
// a PR title without it (megaman: `feat: story: …`, `story/4-story-…`;
// D-5.5c.19).
func TestBuildDropsTheTypePrefix(t *testing.T) {
	g := readyTree(t)
	g.Mu.Lock()
	g.Issues[2].Title = "Story: Show empty list"
	g.Mu.Unlock()
	dir, _ := ledgerCheckout(t)
	git(t, dir, "config", "user.name", "Builder")
	git(t, dir, "config", "user.email", "builder@example.com")
	env := map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "PATH": os.Getenv("PATH")}
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir, CheckpointVerify: holds}
	if code, _, errOut := run(t, env, opts, "ready", "2", "--repo", "o/r"); code != 0 {
		t.Fatalf("ready: %s", errOut)
	}
	code, out, errOut := run(t, env, opts, "build", "2", "--repo", "o/r", "--json")
	var res BuildResult
	_ = json.NewDecoder(strings.NewReader(out)).Decode(&res)
	if code != 0 || res.Branch != "story/2-show-empty-list" {
		t.Fatalf("branch: %d %+v %s", code, res, errOut)
	}
	_ = os.WriteFile(filepath.Join(res.Worktree, "empty.txt"), []byte("Your cart is empty\n"), 0o644)
	code, out, errOut = run(t, env, opts, "build", "2", "--repo", "o/r", "--json")
	res = BuildResult{}
	_ = json.NewDecoder(strings.NewReader(out)).Decode(&res)
	g.Mu.Lock()
	pr := g.Pulls[res.PR]
	g.Mu.Unlock()
	if code != 0 || pr == nil || pr.Title != "feat: show empty list" || !strings.HasPrefix(git(t, res.Worktree, "log", "-1", "--format=%s"), "feat: show empty list (#2,") {
		t.Fatalf("PR title: %d %+v %s", code, pr, errOut)
	}
}

// D-5.5c.20: an agent does not pick up a Ready story the person did not ask
// for. build warns and stops before the lease; the person's words (--asked) or
// the person's own grill in the same session let it start.
func TestBuildHoldsAStoryThePersonDidNotAskFor(t *testing.T) {
	g := readyTree(t)
	dir, _ := ledgerCheckout(t)
	git(t, dir, "config", "user.name", "Builder")
	git(t, dir, "config", "user.email", "builder@example.com")
	person := map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "PATH": os.Getenv("PATH")}
	agent := func(session string) map[string]string {
		return map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "PATH": os.Getenv("PATH"), "CODEX_THREAD_ID": session}
	}
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir, CheckpointVerify: holds}
	if code, _, errOut := run(t, person, opts, "ready", "2", "--repo", "o/r"); code != 0 {
		t.Fatalf("ready: %s", errOut)
	}
	code, _, errOut := run(t, agent("s1"), opts, "build", "2", "--repo", "o/r")
	g.Mu.Lock()
	assignees := len(g.Issues[2].Assignees)
	g.Mu.Unlock()
	if code != ExitBlocked || !strings.Contains(errOut, `build #2 "Show empty list": the person has not asked for this story in this session (CODEX_THREAD_ID)`) ||
		!strings.Contains(errOut, `aayu build 2 --asked "<their words>"`) {
		t.Fatalf("unasked pickup: %d %s", code, errOut)
	}
	if labels := g.LabelNames(2); !contains(labels, "status:ready") || contains(labels, LeaseLabel) || assignees != 0 {
		t.Fatalf("held story was taken: labels %v assignees %d", labels, assignees)
	}
	if _, err := os.Stat(BuildWorktree(dir, 2)); err == nil {
		t.Fatal("held story got a worktree")
	}
	// The gate reads the human-only guard's marker list (hosts.AgentMarkers): a
	// cross-host convention holds too, and Cursor's CURSOR_TRACE_ID, which its
	// terminal sets for a person, does not (below).
	if code, _, errOut := run(t, map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "PATH": os.Getenv("PATH"), "AGENT": "goose"}, opts, "build", "2", "--repo", "o/r"); code != ExitBlocked || !strings.Contains(errOut, "in this session (AGENT)") {
		t.Fatalf("AGENT=goose pickup: %d %s", code, errOut)
	}
	if code, _, errOut := run(t, agent("s1"), opts, "build", "2", "--repo", "o/r", "--asked", "yes, build #2"); code != 0 || !contains(g.LabelNames(2), LeaseLabel) {
		t.Fatalf("asked pickup: %d %s", code, errOut)
	}
	if log, _ := os.ReadFile(filepath.Join(dir, ".aayudha", "telemetry", "actions.jsonl")); !strings.Contains(string(log), `"words":"yes, build #2"`) {
		t.Fatalf("the person's words are not in the actions log:\n%s", log)
	}
	// A story the person's grill filed passes in that session only.
	body := fixtureBody(t, "story", nil)
	code, out, errOut := run(t, agent("s1"), opts, "grill", "story", "--repo", "o/r", "--json", "--title", "Show totals", "--body", body, "--labels", "effort:S", "--create")
	var res GrillResult
	_ = json.NewDecoder(strings.NewReader(out)).Decode(&res)
	if code != 0 || res.Issue == 0 {
		t.Fatalf("grill: %d %s", code, errOut)
	}
	n := strconv.Itoa(res.Issue)
	if code, _, errOut := run(t, person, opts, "ready", n, "--repo", "o/r"); code != 0 {
		t.Fatalf("ready %s: %s", n, errOut)
	}
	if code, _, errOut := run(t, agent("s2"), opts, "build", n, "--repo", "o/r"); code != ExitBlocked || !strings.Contains(errOut, "has not asked") {
		t.Fatalf("another session's grilled story: %d %s", code, errOut)
	}
	if code, _, errOut := run(t, agent("s1"), opts, "build", n, "--repo", "o/r"); code != 0 {
		t.Fatalf("own grilled story: %d %s", code, errOut)
	}
	// The same holds across two Copilot sessions.
	copilot := func(session string) map[string]string {
		return map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "PATH": os.Getenv("PATH"), "COPILOT_AGENT_SESSION_ID": session}
	}
	code, out, errOut = run(t, copilot("c1"), opts, "grill", "story", "--repo", "o/r", "--json", "--title", "Show averages", "--body", body, "--labels", "effort:S", "--create")
	res = GrillResult{}
	_ = json.NewDecoder(strings.NewReader(out)).Decode(&res)
	if code != 0 || res.Issue == 0 {
		t.Fatalf("grill in Copilot: %d %s", code, errOut)
	}
	if code, _, errOut := run(t, person, opts, "ready", strconv.Itoa(res.Issue), "--repo", "o/r"); code != 0 {
		t.Fatalf("ready %d: %s", res.Issue, errOut)
	}
	if code, _, errOut := run(t, copilot("c2"), opts, "build", strconv.Itoa(res.Issue), "--repo", "o/r"); code != ExitBlocked || !strings.Contains(errOut, "has not asked") {
		t.Fatalf("another Copilot session's grilled story: %d %s", code, errOut)
	}
	if code, _, errOut := run(t, person, opts, "ready", "6", "--repo", "o/r"); code != 0 {
		t.Fatalf("ready 6: %s", errOut)
	}
	if code, _, errOut := run(t, map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "PATH": os.Getenv("PATH"), "CURSOR_TRACE_ID": "t"}, opts, "build", "6", "--repo", "o/r"); code != 0 {
		t.Fatalf("a person in Cursor's terminal: %d %s", code, errOut)
	}
}

// Under GitHub Copilot, aayu cannot name the model: the checkpoint commit
// records every model Copilot reported in the session that ran build, most
// recent first, and a HydraFusion critic apart from them, for inspect's
// different-family rule. The build record holds one model, so with two
// families it names none: naming the latest would hide the other.
func TestBuildRecordsCopilotModels(t *testing.T) {
	_, env, opts, dir, bare := buildSetup(t)
	copilotHome := t.TempDir()
	env["COPILOT_HOME"] = copilotHome
	if code, _, errOut := run(t, env, opts, "ready", "2", "--repo", "o/r"); code != 0 {
		t.Fatalf("ready: %s", errOut)
	}
	session := filepath.Join(copilotHome, "session-state", "s-1")
	_ = os.MkdirAll(session, 0o755)
	_ = os.WriteFile(filepath.Join(session, "events.jsonl"), []byte(`{"type":"assistant.message","data":{"model":"gpt-6-luna"}}
{"type":"session.fusion_resolved","data":{"pattern":"critique","secondaryModel":"gpt-6-luna"}}
{"type":"assistant.message","data":{"model":"claude-opus-5.5","fusion":{"phaseKind":"draft","sourceModel":"claude-opus-5.5"}}}
`), 0o644)
	_ = os.MkdirAll(filepath.Join(dir, ".aayudha", "state"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, ".aayudha", "state", "copilot-session.json"), []byte(`{"session_id":"s-1"}`), 0o644)
	code, out, errOut := run(t, env, opts, "build", "2", "--repo", "o/r", "--json")
	var res BuildResult
	_ = json.NewDecoder(strings.NewReader(out)).Decode(&res)
	if code != 0 {
		t.Fatalf("build: %d %s", code, errOut)
	}
	_ = os.WriteFile(filepath.Join(res.Worktree, "empty.txt"), []byte("Your cart is empty\n"), 0o644)
	if code, _, errOut := run(t, env, opts, "build", "2", "--repo", "o/r", "--json"); code != 0 {
		t.Fatalf("build (checkpoint): %d %s", code, errOut)
	}
	msg := git(t, res.Worktree, "log", "-1", "--format=%B")
	if want := "Aayudha-Host: copilot\nAayudha-Model: claude-opus-5.5\nAayudha-Model: gpt-6-luna\nAayudha-Critic-Model: gpt-6-luna"; !strings.Contains(msg, want) {
		t.Fatalf("checkpoint commit lacks the Copilot trailers:\n%s", msg)
	}
	var st BuildState
	if err := json.Unmarshal([]byte(git(t, bare, "show", "ledger:"+BuildLedgerPath(2))), &st); err != nil || len(st.Checkpoints) != 1 || st.Checkpoints[0].Model != "" {
		t.Fatalf("build record: %v %+v", err, st.Checkpoints)
	}
}

// A checkpoint records the build model the host's plan picks when the build
// runs, so a later plan change cannot relabel who built it (Decision #469).
func TestBuilderModelRecordsThePlanDefault(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ host, set, want string }{
		{"cursor", "", "grok-4.7"},
		{"cursor", "hosts.cursor.plan=legacy", "claude-opus-5-5"},
		{"amp", "", ""}, // host-managed: Amp picks the model
	} {
		e, err := newEnv(IO{}, Options{Dir: repo, Getenv: func(string) string { return "" }})
		if err != nil {
			t.Fatal(err)
		}
		if c.set != "" {
			e.Set = []string{c.set}
		}
		e.Host = c.host
		if got := e.builderModel(); got != c.want {
			t.Errorf("%s %q: builder model %q, want %q", c.host, c.set, got, c.want)
		}
	}
}

// sessionReply is a reviewer's reply to a session review: fix with one P1 on
// a.go quoting `var <word> = 1`, or merge when word is empty; unmet makes a
// criterion unmet.
func sessionReply(word string, unmet ...int) models.Response {
	findings, disp := "[]", "merge"
	if word != "" {
		disp = "fix"
		findings = `[{"severity":"P1","category":"bug","file":"a.go","line":3,"quote":"var ` + word + ` = 1","title":"` + word + ` is wrong","detail":"d","fix":"f","confidence":0.9,"criterion":0}]`
	}
	verdict := func(n int) string {
		if slices.Contains(unmet, n) {
			return "unmet"
		}
		return "met"
	}
	return models.Response{Model: "grok-4.7", Text: `{"disposition":"` + disp + `","summary":"s","findings":` + findings + `,"criteria":[{"id":1,"verdict":"` + verdict(1) + `","evidence":"e"},{"id":2,"verdict":"` + verdict(2) + `","evidence":"e"}]}`}
}

// Decision #656, the session half: before each push build has a reviewer of
// another family review the branch. A fix holds the push; a decline with a
// reason releases it and goes into the PR body's self-attested summary; after
// inspect.MaxRounds rounds a push goes out with what is left open; a review
// that cannot run lets the push go out, and the summary says why.
func TestBuildSessionReviewLoop(t *testing.T) {
	g, env, opts, _, bare, prN, _ := thinFixture(t)
	model := &models.Fake{}
	var routes []string
	opts.ModelClient = func(r models.Route, key string) models.Client {
		routes = append(routes, r.BaseURL+" "+key)
		return model
	}
	// The same three names as the merge gate's code-review environment.
	env[inspect.ReviewModelURLVar] = "https://review.example/v1"
	env[inspect.ReviewModelVar] = "grok-4.7"
	env[inspect.ReviewModelKeyVar] = "review-key"
	wt := BuildWorktree(opts.Dir, 2)
	build := func(args ...string) (int, BuildResult, string) {
		code, out, errOut := run(t, env, opts, append([]string{"build", "2", "--repo", "o/r", "--json"}, args...)...)
		var res BuildResult
		_ = json.NewDecoder(strings.NewReader(out)).Decode(&res)
		return code, res, errOut
	}
	write := func(word string) {
		_ = os.WriteFile(filepath.Join(wt, "a.go"), []byte("package a\n\nvar "+word+" = 1\n"), 0o644)
	}
	pushed := func() string { return git(t, bare, "rev-parse", "refs/heads/story/2-show-empty-list") }
	body := func() string {
		g.Mu.Lock()
		defer g.Mu.Unlock()
		if g.Issues[prN].Body != g.Pulls[prN].Body {
			t.Fatalf("the PR's issue body differs from its pull request body:\n%s", g.Issues[prN].Body)
		}
		return g.Pulls[prN].Body
	}

	// Round 1 finds a P1: the checkpoint is committed but not pushed, and the
	// builder reads what to fix.
	model.Replies = []models.Response{sessionReply("first")}
	before := pushed()
	write("first")
	_ = os.MkdirAll(filepath.Join(wt, ".aayudha", "evidence"), 0o755)
	_ = os.WriteFile(filepath.Join(wt, ".aayudha", "evidence", "2.toml"), []byte("[[criterion]]\nid = 1\ntests = [\"./a:TestA\"]\n"), 0o644)
	id := inspect.FindingID("a.go", "bug", "first is wrong")
	code, out, errOut := run(t, env, opts, "build", "2", "--repo", "o/r")
	if code != ExitBlocked || pushed() != before || len(model.Requests) != 1 || model.Requests[0].Model != "grok-4.7" ||
		!slices.Equal(routes, []string{"https://review.example/v1 review-key"}) || model.Requests[0].ServiceTier != models.TierPriority || model.Requests[0].Effort != "high" ||
		!strings.Contains(out, "session review round 1 by grok-4.7: fix; not pushed yet") || !strings.Contains(out, "  "+id+" P1 a.go:3: first is wrong") ||
		!strings.Contains(out, "    fix: f") || !strings.Contains(out, "--decline") || !strings.Contains(model.Requests[0].Messages[0].Content, "criterion 1: ./a:TestA") {
		t.Fatalf("round 1: %d %s %s", code, out, errOut)
	}
	if code, _, errOut := build("--decline", "nope: why"); code != ExitUsage || !strings.Contains(errOut, "nope is not an open item") {
		t.Fatalf("decline of an unknown item: %d %s", code, errOut)
	}
	if code, _, errOut := build("--decline", id); code != ExitUsage || !strings.Contains(errOut, `write "<id>: <why>", one a line`) {
		t.Fatalf("decline without a reason: %d %s", code, errOut)
	}
	// The decline releases the push without calling the reviewer again.
	code, res, errOut := build("--decline", id+": the quoted line is the story's own example")
	if code != 0 || len(model.Requests) != 1 || pushed() == before {
		t.Fatalf("after the decline: %d %+v %s", code, res.Review, errOut)
	}
	b := body()
	for _, want := range []string{"Session review (self-attested)", "not a review that counts", "| 1 | `", "grok-4.7 (xai)", "**Declined by the builder, with its reason:**", "`" + id + "` P1 first is wrong (a.go:3): the quoted line is the story's own example", "Refs #2"} {
		if !strings.Contains(b, want) {
			t.Fatalf("PR body lacks %q:\n%s", want, b)
		}
	}
	// A run on the pushed head has nothing new to review.
	if code, _, errOut := build(); code != 0 || len(model.Requests) != 1 {
		t.Fatalf("a run on the pushed head: %d %s, %d calls", code, errOut, len(model.Requests))
	}

	// Three more rounds after that push: two hold it, the third pushes at the
	// cap. An unmet criterion is an item to fix too.
	for i, word := range []string{"second", "third", "fourth"} {
		model.Replies = []models.Response{sessionReply(word, 2)}
		before = pushed()
		write(word)
		code, res, errOut = build()
		if i < 2 && (code != ExitBlocked || pushed() != before || !slices.ContainsFunc(res.Review.Open, func(it sessionItem) bool { return it.ID == "C2" })) ||
			i == 2 && (code != 0 || pushed() == before) {
			t.Fatalf("round %d (%s): %d %+v %s", i+2, word, code, res.Review, errOut)
		}
	}
	if msg := git(t, wt, "log", "-1", "--format=%B"); !strings.Contains(msg, "Aayudha-Checkpoint: 3/3") || !strings.Contains(msg, "Aayudha-Attempt: 3") {
		t.Fatalf("a review fix is the same checkpoint's next attempt:\n%s", msg)
	}
	b = body()
	if !strings.Contains(b, "| 4 | `") || !strings.Contains(b, "**Left open at the round cap:**\n\n- `C2` criterion criterion 2 unmet\n- `"+inspect.FindingID("a.go", "bug", "fourth is wrong")+"` P1 fourth is wrong") ||
		!strings.Contains(b, "**Fixed (not raised by a later round):**") || strings.Count(b, sessionReviewStart) != 1 {
		t.Fatalf("PR body after the cap:\n%s", b)
	}

	// A review that cannot run does not hold the push.
	model.Err = errors.New("gateway down")
	before = pushed()
	write("fifth")
	code, res, errOut = build()
	if code != 0 || pushed() == before || res.Review == nil || !strings.Contains(res.Review.Skipped, "could not run") ||
		!strings.Contains(body(), "The last push went out without a session review: the review by grok-4.7 could not run") {
		t.Fatalf("a review that cannot run: %d %+v %s\n%s", code, res.Review, errOut, body())
	}

	// A rebuild with no finding holds the push too: the builder answers the
	// verdict itself.
	model.Err = nil
	model.Replies = []models.Response{{Model: "grok-4.7", Text: `{"disposition":"rebuild","summary":"the approach is wrong","findings":[],"criteria":[{"id":1,"verdict":"unclear","evidence":"no trace"},{"id":2,"verdict":"met","evidence":"e"}]}`}}
	_ = os.WriteFile(filepath.Join(wt, "b.txt"), []byte("rebuild\n"), 0o644)
	if code, res, errOut = build(); code != ExitBlocked || len(res.Review.Open) != 1 || res.Review.Open[0].ID != "C1" || res.Review.Open[0].Title != "criterion 1 unclear" {
		t.Fatalf("a rebuild with an unclear criterion: %d %+v %s", code, res.Review, errOut)
	}
	model.Replies = []models.Response{{Model: "grok-4.7", Text: `{"disposition":"rebuild","summary":"the approach is wrong","findings":[],"criteria":[]}`}}
	_ = os.WriteFile(filepath.Join(wt, "b.txt"), []byte("rebuild again\n"), 0o644)
	if code, res, errOut = build(); code != ExitBlocked || len(res.Review.Open) != 1 || res.Review.Open[0].Severity != "verdict" || res.Review.Open[0].Detail != "the approach is wrong" {
		t.Fatalf("a rebuild with nothing behind it: %d %+v %s", code, res.Review, errOut)
	}
	if code, _, errOut := build("--decline", res.Review.Open[0].ID+": the story asks for this approach"); code != 0 {
		t.Fatalf("declining the verdict: %d %s", code, errOut)
	}
	// A decline answers its round only: the first round's declined finding,
	// raised again on a later head, is open again.
	model.Replies = []models.Response{sessionReply("first")}
	_ = os.WriteFile(filepath.Join(wt, "b.txt"), []byte("first again\n"), 0o644)
	if code, res, errOut = build(); code != ExitBlocked || len(res.Review.Open) != 1 || res.Review.Open[0].ID != id {
		t.Fatalf("a declined finding raised again: %d %+v %s", code, res.Review, errOut)
	}
	if code, _, errOut := build("--decline", id+": still the story's own example"); code != 0 || !strings.Contains(body(), "`"+id+"` P1 first is wrong (a.go:3): still the story's own example") {
		t.Fatalf("declining it again: %d %s\n%s", code, errOut, body())
	}

	// A P2 that a later round raises again stays open, though inspect does not
	// post it twice.
	model.Err = nil
	p2 := func(word string) models.Response {
		r := sessionReply(word)
		r.Text = strings.Replace(r.Text, `"severity":"P1"`, `"severity":"P2"`, 1)
		return r
	}
	model.Replies = []models.Response{p2("sixth")}
	write("sixth")
	if code, res, errOut = build(); code != ExitBlocked || len(res.Review.Open) != 1 {
		t.Fatalf("a P2: %d %+v %s", code, res.Review, errOut)
	}
	p2id := res.Review.Open[0].ID
	_ = os.WriteFile(filepath.Join(wt, "b.txt"), []byte("unrelated\n"), 0o644)
	if code, res, errOut = build(); code != ExitBlocked || len(res.Review.Open) != 1 || res.Review.Open[0].ID != p2id {
		t.Fatalf("the P2 raised again: %d %+v %s", code, res.Review, errOut)
	}
	// A review that cannot run does not release a held push: the open item
	// still waits for a fix the reviewer sees, or a decline.
	model.Err = errors.New("gateway down")
	before = pushed()
	_ = os.WriteFile(filepath.Join(wt, "b.txt"), []byte("unseen fix\n"), 0o644)
	if code, res, errOut = build(); code != ExitBlocked || pushed() != before || len(res.Review.Open) != 1 || res.Review.Open[0].ID != p2id || !strings.Contains(res.Review.Skipped, "could not run") {
		t.Fatalf("a held push when the review cannot run: %d %+v %s", code, res.Review, errOut)
	}
	model.Err = nil

	// The spend the build's ledger record keeps still caps the PR when the
	// worktree's own state is gone, as on a resume elsewhere, with the merge
	// gate's spend: no new review call. The PR body's figure is not read.
	stateFile := filepath.Join(git(t, wt, "rev-parse", "--absolute-git-dir"), sessionReviewFile)
	_ = os.Remove(stateFile)
	paid := sessionReply("")
	paid.TokensIn = 100_000
	model.Replies = []models.Response{paid}
	_ = os.WriteFile(filepath.Join(wt, "b.txt"), []byte("paid\n"), 0o644)
	if code, res, errOut := build(); code != 0 || len(res.Review.Open) != 0 {
		t.Fatalf("a paid round that merges: %d %+v %s", code, res.Review, errOut)
	}
	var rec BuildState
	_ = json.Unmarshal([]byte(git(t, bare, "show", "ledger:"+BuildLedgerPath(2))), &rec)
	spent := rec.ReviewUSD
	if spent <= 0 || !strings.Contains(body(), "Spent on the session review: ") {
		t.Fatalf("no session review spend on the ledger (%v) or in the PR body:\n%s", spent, body())
	}
	g.Mu.Lock()
	g.Pulls[prN].Body = strings.ReplaceAll(g.Pulls[prN].Body, "Spent on the session review: ", "Spent: ")
	g.Issues[prN].Body = g.Pulls[prN].Body
	g.Mu.Unlock()
	if err := os.Remove(stateFile); err != nil {
		t.Fatal(err)
	}
	// The merge gate's inspect spends from the same cap: 1 USD on its record.
	addComment(g, prN, "github-actions[bot]", "Bot", inspect.Marker+"\n<!-- aayudha:inspect-state {\"diff_hash\":\"x\",\"cycles\":0,\"spent_usd\":1,\"hashes\":[]} -->\n")
	env["AAYU_BUDGET__PER_PR_INSPECT_MAX"] = strconv.FormatFloat(spent+1, 'f', -1, 64)
	calls := len(model.Requests)
	model.Replies = []models.Response{sessionReply("")}
	_ = os.WriteFile(filepath.Join(wt, "b.txt"), []byte("resumed\n"), 0o644)
	if code, res, errOut := build(); code != 0 || len(model.Requests) != calls || !strings.Contains(res.Review.Skipped, "per_pr_inspect_max") {
		t.Fatalf("a resume past the PR's review budget: %d %+v %s, %d calls", code, res.Review, errOut, len(model.Requests)-calls)
	}
	// A cap of 0 is no cap to inspect: the session makes no review call at
	// all, and a review endpoint it would refuse does not stop the push.
	env["AAYU_BUDGET__PER_PR_INSPECT_MAX"] = "0"
	env[inspect.ReviewModelURLVar] = "http://review.example/v1"
	_ = os.WriteFile(filepath.Join(wt, "b.txt"), []byte("uncapped\n"), 0o644)
	if code, res, errOut := build(); code != 0 || len(model.Requests) != calls || !strings.Contains(res.Review.Skipped, "per_pr_inspect_max is 0") {
		t.Fatalf("a zero per-PR cap: %d %+v %s, %d calls", code, res.Review, errOut, len(model.Requests)-calls)
	}
	delete(env, "AAYU_BUDGET__PER_PR_INSPECT_MAX")
	env[inspect.ReviewModelURLVar] = "https://review.example/v1"

	// A review that trips the budget breaker stops the build: nothing is pushed.
	costly := sessionReply("")
	costly.TokensIn = 50_000_000
	model.Replies = []models.Response{costly}
	env["AAYU_BUDGET__PER_DAY_USD"] = "1"
	before = pushed()
	_ = os.WriteFile(filepath.Join(wt, "b.txt"), []byte("budget\n"), 0o644)
	if code, _, errOut := build(); code != ExitBudget || pushed() != before {
		t.Fatalf("a tripped breaker: %d %s", code, errOut)
	}
	// The checkpoint it committed is on the ledger all the same.
	var bst BuildState
	if err := json.Unmarshal([]byte(git(t, bare, "show", "ledger:"+BuildLedgerPath(2))), &bst); err != nil ||
		bst.Checkpoints[len(bst.Checkpoints)-1].Commit != git(t, wt, "rev-parse", "HEAD") {
		t.Fatalf("the checkpoint before a review error is not recorded: %v %+v", err, bst.Checkpoints)
	}
	delete(env, "AAYU_BUDGET__PER_DAY_USD")
	_ = os.Remove(filepath.Join(opts.Dir, ".aayudha", "state", "paused"))

	// A review endpoint that is not https is refused before any call.
	env[inspect.ReviewModelURLVar] = "http://review.example/v1"
	calls = len(model.Requests)
	_ = os.WriteFile(filepath.Join(wt, "b.txt"), []byte("again\n"), 0o644)
	if code, _, errOut := build(); code != ExitUsage || !strings.Contains(errOut, inspect.ReviewModelURLVar) || len(model.Requests) != calls {
		t.Fatalf("an http endpoint: %d %s", code, errOut)
	}
}

// A checkpoint the session review holds is kept on its own ref, never the PR
// branch, and --resume on another machine continues from it.
func TestBuildSessionReviewHeldCheckpointResumes(t *testing.T) {
	_, env, opts, dir, bare, _, _ := thinFixture(t)
	model := &models.Fake{Replies: []models.Response{sessionReply("held")}}
	opts.ModelClient = func(models.Route, string) models.Client { return model }
	env[inspect.ReviewModelURLVar] = "https://review.example/v1"
	env[inspect.ReviewModelVar] = "grok-4.7"
	env[inspect.ReviewModelKeyVar] = "review-key"
	wt := BuildWorktree(opts.Dir, 2)
	branch := "story/2-show-empty-list"
	before := git(t, bare, "rev-parse", "refs/heads/"+branch)
	_ = os.WriteFile(filepath.Join(wt, "a.go"), []byte("package a\n\nvar held = 1\n"), 0o644)
	if code, _, errOut := run(t, env, opts, "build", "2", "--repo", "o/r"); code != ExitBlocked {
		t.Fatalf("a held round: %d %s", code, errOut)
	}
	head := git(t, wt, "rev-parse", "HEAD")
	if got := git(t, bare, "rev-parse", heldRef(branch)); got != head || git(t, bare, "rev-parse", "refs/heads/"+branch) != before {
		t.Fatalf("held ref %s, branch moved: want %s on the held ref only", got, head)
	}
	gone := func() {
		t.Helper()
		git(t, dir, "worktree", "remove", "--force", wt)
		git(t, dir, "branch", "-D", branch)
	}
	gone()
	if code, _, errOut := run(t, env, opts, "build", "2", "--resume", "--repo", "o/r"); code != 0 || git(t, wt, "rev-parse", "HEAD") != head {
		t.Fatalf("resume from the held checkpoint: %d %s, head %s", code, errOut, git(t, wt, "rev-parse", "HEAD"))
	}
	// The story branch was never pushed, only the held ref: a build without
	// --resume says so, and --resume continues from the held ref.
	gone()
	git(t, bare, "update-ref", "-d", "refs/heads/"+branch)
	if code, out, errOut := run(t, env, opts, "build", "2", "--repo", "o/r"); code != ExitBlocked || !strings.Contains(out+errOut, "--resume") {
		t.Fatalf("a held-only story without --resume: %d %s %s", code, out, errOut)
	}
	if code, _, errOut := run(t, env, opts, "build", "2", "--resume", "--repo", "o/r"); code != 0 || git(t, wt, "rev-parse", "HEAD") != head {
		t.Fatalf("resume from a held-only checkpoint: %d %s, head %s", code, errOut, git(t, wt, "rev-parse", "HEAD"))
	}
	// Once the reviewer is satisfied the branch is pushed and the held ref is
	// gone, so a later rewrite of the branch cannot send a resume back to it.
	model.Replies = []models.Response{sessionReply("")}
	_ = os.WriteFile(filepath.Join(wt, "a.go"), []byte("package a\n\nvar fixed = 1\n"), 0o644)
	if code, _, errOut := run(t, env, opts, "build", "2", "--repo", "o/r"); code != 0 || git(t, bare, "rev-parse", "refs/heads/"+branch) != git(t, wt, "rev-parse", "HEAD") {
		t.Fatalf("the push after the hold: %d %s", code, errOut)
	}
	if refs := git(t, bare, "for-each-ref", "refs/aayudha/held/"); refs != "" {
		t.Fatalf("the held ref outlives the push: %s", refs)
	}
}

// A reviewer of the builder's family never reviews in the session: the push
// goes out unreviewed, and the PR body says so.
func TestBuildSessionReviewRefusesSameFamily(t *testing.T) {
	g, env, opts, _, bare, prN, _ := thinFixture(t)
	model := &models.Fake{Replies: []models.Response{sessionReply("")}}
	opts.ModelClient = func(models.Route, string) models.Client { return model }
	env["AAYU_MODEL"] = "grok-4.7"
	env["AAYU_INSPECT__PAID_REVIEWER"] = "grok-4.7"
	env["AAYU_SECRET_XAI"] = "xai-key"
	_ = os.WriteFile(filepath.Join(BuildWorktree(opts.Dir, 2), "a.txt"), []byte("x\n"), 0o644)
	// The open PR cannot be read before the review: nothing is pushed.
	g.Mu.Lock()
	g.Fail = "GET /repos/o/r/pulls"
	g.Mu.Unlock()
	if code, _, errOut := run(t, env, opts, "build", "2", "--repo", "o/r"); code != ExitExternal {
		t.Fatalf("the PR list fails: %d %s", code, errOut)
	}
	// The verified checkpoint reaches the ledger all the same.
	var bst BuildState
	if err := json.Unmarshal([]byte(git(t, bare, "show", "ledger:"+BuildLedgerPath(2))), &bst); err != nil || len(bst.Checkpoints) == 0 ||
		bst.Checkpoints[len(bst.Checkpoints)-1].Commit != git(t, BuildWorktree(opts.Dir, 2), "rev-parse", "HEAD") {
		t.Fatalf("the checkpoint before a PR-list failure is not recorded: %v %+v", err, bst.Checkpoints)
	}
	// The PR body cannot be updated: the push went out, and build says so.
	g.Mu.Lock()
	g.Fail = "PATCH /repos/o/r/pulls/" + strconv.Itoa(prN)
	g.Mu.Unlock()
	if code, _, errOut := run(t, env, opts, "build", "2", "--repo", "o/r"); code != ExitExternal || !strings.Contains(errOut, "500") {
		t.Fatalf("a failed body update: %d %s", code, errOut)
	}
	_ = os.WriteFile(filepath.Join(BuildWorktree(opts.Dir, 2), "a.txt"), []byte("y\n"), 0o644)
	code, out, errOut := run(t, env, opts, "build", "2", "--repo", "o/r", "--json")
	var res BuildResult
	_ = json.NewDecoder(strings.NewReader(out)).Decode(&res)
	g.Mu.Lock()
	b := g.Pulls[prN].Body
	g.Mu.Unlock()
	if code != 0 || len(model.Requests) != 0 || res.Review == nil || !strings.Contains(res.Review.Skipped, "no reviewer model of a family other than the builder's") ||
		!strings.Contains(b, "The last push went out without a session review: no reviewer model of a family other than the builder's") {
		t.Fatalf("same family: %d %+v %s\n%s", code, res.Review, errOut, b)
	}
}

// The PR body declares the builder (AGENTS.md) and the session review picks a
// reviewer of none of the declared families: build's own Claude line keeps a
// Claude reviewer out, and a person's added GPT line keeps OpenAI out too. The
// person's text and line survive the next push, and build's lines come back.
func TestBuildSessionReviewReadsDeclarations(t *testing.T) {
	g, env, opts, _, _, prN, _ := thinFixture(t)
	model := &models.Fake{Replies: []models.Response{sessionReply("")}}
	opts.ModelClient = func(models.Route, string) models.Client { return model }
	env[inspect.ReviewModelURLVar] = "https://review.example/v1"
	env[inspect.ReviewModelKeyVar] = "review-key"
	env[inspect.ReviewModelVar] = "claude-sonnet-5-5" // the builder's family: refused
	env["AAYU_EFFORT"] = "medium"
	body := func() string { g.Mu.Lock(); defer g.Mu.Unlock(); return g.Pulls[prN].Body }
	if b := body(); !strings.Contains(b, "\nAayudha-Model: claude-opus-5-5\n") {
		t.Fatalf("the PR body does not declare the builder's model:\n%s", b)
	}
	wt := BuildWorktree(opts.Dir, 2)
	family := func() verbs.Family {
		t.Helper()
		if len(model.Requests) == 0 {
			t.Fatal("no reviewer was called")
		}
		return verbs.FamilyOf(model.Requests[len(model.Requests)-1].Model)
	}
	_ = os.WriteFile(filepath.Join(wt, "a.txt"), []byte("one\n"), 0o644)
	if code, out, errOut := run(t, env, opts, "build", "2", "--repo", "o/r"); code != 0 || family() == verbs.FamilyAnthropic {
		t.Fatalf("a Claude-built PR: %d %s %s, reviewer %v", code, out, errOut, family())
	}
	// A person rewrites the body and declares GPT as well.
	g.Mu.Lock()
	g.Pulls[prN].Body = "Refs #2\n\nA person's note.\n\nAayudha-Model: gpt-6.1-sol\n"
	g.Mu.Unlock()
	_ = os.WriteFile(filepath.Join(wt, "a.txt"), []byte("two\n"), 0o644)
	if code, out, errOut := run(t, env, opts, "build", "2", "--repo", "o/r"); code != 0 || family() == verbs.FamilyAnthropic || family() == verbs.FamilyOpenAI {
		t.Fatalf("Claude and GPT declared: %d %s %s, reviewer %v", code, out, errOut, family())
	}
	b := body()
	for _, want := range []string{"A person's note.", "\nAayudha-Model: gpt-6.1-sol\n", "\nAayudha-Model: claude-opus-5-5\n", "\nAayudha-Effort: medium\n", "the reviewers are of none of [anthropic openai]"} {
		if !strings.Contains(b, want) {
			t.Fatalf("PR body lacks %q:\n%s", want, b)
		}
	}
	// A held round's head, retried after a declaration names the reviewer's
	// own family, is reviewed again by another reviewer: the first one's
	// cached result is not the new reviewer's.
	model.Replies = []models.Response{sessionReply("held")}
	_ = os.WriteFile(filepath.Join(wt, "a.txt"), []byte("three\n"), 0o644)
	if code, _, errOut := run(t, env, opts, "build", "2", "--repo", "o/r"); code != ExitBlocked {
		t.Fatalf("a held round: %d %s", code, errOut)
	}
	first, calls := model.Requests[len(model.Requests)-1].Model, len(model.Requests)
	g.Mu.Lock()
	g.Pulls[prN].Body += "\nAayudha-Model: " + first + "\n"
	g.Mu.Unlock()
	model.Replies = []models.Response{sessionReply("")}
	if code, out, errOut := run(t, env, opts, "build", "2", "--repo", "o/r"); code != 0 || len(model.Requests) != calls+1 || family() == verbs.FamilyOf(first) {
		t.Fatalf("the held head after %s is declared: %d %s %s, %d new calls, reviewer %v", first, code, out, errOut, len(model.Requests)-calls, family())
	}
}

// The review context comes from the committed head only: never a file outside
// the repository, an uncommitted one, or a symlink.
func TestGitSourceReadsOnlyCommittedFiles(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "--quiet")
	_ = os.MkdirAll(filepath.Join(dir, "p"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "p", "a.go"), []byte("package p\n"), 0o644)
	_ = os.Symlink("/etc/hostname", filepath.Join(dir, "p", "link.go"))
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "--quiet", "-m", "c")
	_ = os.WriteFile(filepath.Join(dir, "p", "new.go"), []byte("package p\n"), 0o644)
	src, ctx := gitSource(dir), context.Background()
	if b, err := src.File(ctx, "p/a.go"); err != nil || string(b) != "package p\n" {
		t.Fatalf("committed file: %q %v", b, err)
	}
	for _, p := range []string{"p/link.go", "p/new.go", "../x.go", "/etc/hostname"} {
		if _, err := src.File(ctx, p); err == nil {
			t.Fatalf("%s was read", p)
		}
	}
	if fs, err := src.Dir(ctx, "p"); err != nil || !slices.Equal(fs, []string{"p/a.go"}) {
		t.Fatalf("dir p: %v %v", fs, err)
	}
	if fs, err := src.Dir(ctx, "."); err != nil || len(fs) != 0 {
		t.Fatalf("dir .: %v %v", fs, err)
	}
	if _, err := src.Dir(ctx, ".."); err == nil {
		t.Fatal("a directory outside the repository was listed")
	}
}
