package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/planning"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
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
	if code, out := build(); code != 0 || !strings.Contains(out, "checkpoint 2/2 committed") {
		t.Fatalf("after resume: %d %s", code, out)
	}
}

// WP5.5 build budget: a second verify miss on a never layer blocks before the
// attempts budget, and spend over [autonomy.budget] cost_usd blocks at once.
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
	env["AAYU_COST_USD"] = "30"
	if code, out, errOut := run(t, env, opts, "build", "2", "--repo", "o/r"); code != ExitBlocked || !strings.Contains(out+errOut, "cost budget breached: 30.00 USD spent, budget 25.00 USD") {
		t.Fatalf("over the cost budget: %d %s %s", code, out, errOut)
	}
}

// WP5.5 accept: a stale lease is reclaimed by gc (comment, label, and
// assignee), a live one is left alone, and the worktree of a closed story is
// removed while an open story's is kept.
func TestGCReclaimsStaleLease(t *testing.T) {
	g, env, opts, dir, _, _, _ := thinFixture(t)
	other := g.AddIssue("Other story", "x", "type:story", "status:ready", LeaseLabel) // a live lease
	g.Mu.Lock()
	other.Comments = append(other.Comments, &fake.Comment{ID: 501, Body: leaseBody("other@laptop", time.Now().Add(time.Hour))})
	g.Mu.Unlock()
	setLease(g, 2, "founder@old-laptop", time.Now().Add(-2*time.Hour))
	code, out, errOut := run(t, env, opts, "gc", "--repo", "o/r")
	if code != 0 || !strings.Contains(out, "lease #2 reclaimed: the lease of founder@old-laptop expired at") || !strings.Contains(out, "lease #"+strconv.Itoa(other.Number)+" live: other@laptop until") ||
		!strings.Contains(out, "worktree #2 kept: the story is open") {
		t.Fatalf("gc: %d %s %s", code, out, errOut)
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
	if code, out, errOut := run(t, env, opts, "gc", "--repo", "o/r"); code != 0 || !strings.Contains(out, "worktree #2 removed: the story is closed") {
		t.Fatalf("gc of a closed story's worktree: %d %s %s", code, out, errOut)
	}
	if _, err := os.Stat(BuildWorktree(dir, 2)); !os.IsNotExist(err) {
		t.Fatalf("worktree left: %v", err)
	}
}

// WP5.5 ship: the human pass comes from the [autonomy] approver in solo mode
// (a bot's approval is never one); a never layer needs the EM's plan-time
// review and a CODEOWNER's approval, whatever the streak; the ship record
// keeps the pass rule, the approval, and the streak, and the lease is
// released.
func TestShipPassRules(t *testing.T) {
	g, env, opts, dir, bare, prN, head := thinFixture(t)
	gateRun(t, g, dir, prN, head, []proof.Check{{Floor: "coverage", Result: proof.Pass, Value: 100, Threshold: 90, Tool: "aayudha", Version: "1"}}, "L02", "L07")
	g.Mu.Lock()
	g.Pulls[prN].Files = []string{"internal/cli/x.go", "internal/config/x.go"}
	g.Files = map[string]string{}
	g.Files[".github/CODEOWNERS"] = "/internal/config/ @owner1 @reviewer2\n"
	g.Pulls[prN].Reviews = []fake.Review{review("aayudha-bot[bot]", "Bot", "APPROVED", head), review("reviewer", "User", "APPROVED", head)}
	g.Mu.Unlock()
	ship := func(args ...string) (int, string) {
		code, out, errOut := run(t, env, opts, append([]string{"ship", strconv.Itoa(prN), "--repo", "o/r", "--set", "autonomy.approver=reviewer2"}, args...)...)
		return code, out + errOut
	}
	if code, out := ship(); code != ExitBlocked || !strings.Contains(out, "plan-time review is required") {
		t.Fatalf("never layer without the EM review: %d %s", code, out)
	}
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
	g.Files[".github/CODEOWNERS"] = "/internal/config/ @owner1\n"
	g.Pulls[prN].Reviews = append(g.Pulls[prN].Reviews, review("reviewer2", "User", "APPROVED", head))
	g.Mu.Unlock()
	if code, out := ship(); code != ExitBlocked || !strings.Contains(out, "does not cover never layer(s) L07: a CODEOWNER of internal/config/x.go must approve (owners: owner1), whatever the streak") {
		t.Fatalf("approver who is no CODEOWNER of the never layer: %d %s", code, out)
	}
	g.Mu.Lock()
	g.Files[".github/CODEOWNERS"] = "/internal/config/ @owner1 @reviewer2\n"
	g.Mu.Unlock()
	if code, out := ship(); code != 0 || !strings.Contains(out, "human pass: review by reviewer2") {
		t.Fatalf("ship: %d %s", code, out)
	}
	var sr ShipRecord
	_ = json.Unmarshal([]byte(git(t, bare, "show", "ledger:proof/2/"+head+"/authoritative/ship.json")), &sr)
	if sr.HumanPass.ReviewID == 0 || sr.Pass.As != "approver" || strings.Join(sr.Pass.Never, ",") != "L07" || sr.PlanReview == nil || sr.PlanReview.ApprovedBy != "founder" ||
		sr.Streak.Threshold != 20 || sr.Streak.Autonomous || len(sr.Streak.Layers) != 2 {
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
