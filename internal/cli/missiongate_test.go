package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/models"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
)

// Story #1001: the milestone gate on the missionWorld of Stories #997 and
// #998, with the dev tip on a fake GitHub and stub verify and eval runners.
// No model is called: ModelClient fails the test if anything asks for one.

type gateWorld struct {
	*missionWorld
	sha   string
	gmu   sync.Mutex
	vfail []string // verify checks that fail on the tip
	efail []string // eval cases that fail on the tip
	seats []string // the seats the eval runner was asked for
	calls int      // verify runs
}

// newGateWorld starts #889's mission with milestones ms ("MS1=#20,#21
// MS2=#22"), its stories as listed, and dev at a tip whose required checks
// build and lint pass.
func newGateWorld(t *testing.T, ms string, stories ...int) *gateWorld {
	g := &gateWorld{missionWorld: newMissionWorld(t)}
	m := g.missionWorld
	m.issue(889, "Agents pull the next story", "feature", 0, "")
	for _, n := range stories {
		m.issue(n, fmt.Sprintf("Story %d", n), "story", 889, "")
	}
	m.feature(889, stories...)
	m.mu.Lock()
	m.tool = "amp"
	m.mu.Unlock()
	if code, out, errOut := m.approve(889, "--milestones", ms); code != 0 {
		t.Fatalf("approve 889: exit %d: %s%s", code, out, errOut)
	}
	if code, out, errOut := m.mission(m.person, "rahulnakmol", "start", 889); code != 0 {
		t.Fatalf("start 889: exit %d: %s%s", code, out, errOut)
	}
	m.edit(func() { m.g.RequiredChecks = []string{"build", "lint", "protected-paths"} })
	g.tip(strings.Repeat("9f8e7d6", 6)[:40], "success")
	m.opts.ModelClient = func(models.Route, string) models.Client { t.Fatal("the gate called a model"); return nil }
	m.opts.GateVerify = func(_ context.Context, sha string) ([]GateCheck, error) {
		g.gmu.Lock()
		defer g.gmu.Unlock()
		g.calls++
		if sha != g.sha {
			t.Errorf("verify ran on %s, not the dev tip %s", sha, g.sha)
		}
		out := []GateCheck{{Check: "verify:mission.next-order", Result: gatePass, Detail: "passed"}}
		for _, c := range g.vfail {
			out = append(out, GateCheck{Check: "verify:" + c, Result: gateFail, Detail: "failed: two ship records"})
		}
		return out, nil
	}
	m.opts.GateEvals = func(_ context.Context, sha string, seats []string) ([]GateCheck, error) {
		g.gmu.Lock()
		defer g.gmu.Unlock()
		g.seats = seats
		out := []GateCheck{{Check: "eval:plan/small-story", Result: gatePass, Detail: "mean 4.40, pass^k 1.00"}}
		for _, c := range g.efail {
			out = append(out, GateCheck{Check: "eval:" + c, Result: gateFail, Detail: "mean 3.10, pass^k 0.00 (bars: mean 3.50, pass^k 0.80)"})
		}
		return out, nil
	}
	return g
}

// tip moves dev to sha, with build and lint at conclusion (the CODEOWNERS
// check is red on dev: the gate never judges it).
func (g *gateWorld) tip(sha, conclusion string) {
	g.sha = sha
	g.edit(func() {
		if g.g.RefFiles[sha] == nil { // dev's committed config moves with its tip
			g.g.RefFiles[sha] = g.g.RefFiles[devTip]
		}
		g.g.Refs["heads/dev"] = sha
		g.g.CheckRuns[sha] = []fake.CheckRun{
			{ID: 70, Name: "build", Status: "completed", Conclusion: conclusion, Suite: &fake.Suite{ID: 1}},
			{ID: 71, Name: "lint", Status: "completed", Conclusion: "success", Suite: &fake.Suite{ID: 1}},
			{ID: 72, Name: "protected-paths", Status: "completed", Conclusion: "failure", Suite: &fake.Suite{ID: 2}},
		}
	})
}

// gate runs aayu mission gate 889 as the Kanha in Amp.
func (g *gateWorld) gate(args ...string) (int, string, string) {
	g.t.Helper()
	return run(g.t, g.agent, g.opts, append([]string{"mission", "gate", "889", "--repo", "o/r"}, args...)...)
}

func (g *gateWorld) gateJSON(args ...string) GateReport {
	g.t.Helper()
	code, out, errOut := g.gate(append(args, "--json")...)
	var rep GateReport
	if code != 0 || json.Unmarshal([]byte(out), &rep) != nil {
		g.t.Fatalf("mission gate 889: exit %d: %s%s", code, out, errOut)
	}
	return rep
}

// runs are the lines of missions/889/gates.jsonl.
func (g *gateWorld) runs() []GateRun {
	raw, _ := exec.Command("git", "-C", g.bare, "show", "ledger:"+MissionGatesPath(889)).Output() // none yet: no runs
	var out []GateRun
	for _, l := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var r GateRun
		if json.Unmarshal([]byte(l), &r) == nil && r.Milestone != "" {
			out = append(out, r)
		}
	}
	return out
}

// fixPR opens fix PR n to dev with the gate fix line for check of MS1.
func (g *gateWorld) fixPR(n int, check, refs string) {
	pr := g.g.AddPR("fix(gate): "+check, refs+"\n\nAayudha-Gate-Fix: MS1 "+check+"\n", "rahulnakmol", fmt.Sprintf("fix/gate-%d", n), strings.Repeat("e", 40), "dev")
	g.edit(func() {
		is := g.g.Issues[pr.Number]
		delete(g.g.Issues, pr.Number)
		delete(g.g.Pulls, pr.Number)
		is.Number, pr.Number = n, n
		g.g.Issues[n], g.g.Pulls[n] = is, pr
	})
}

func (g *gateWorld) mergePR(n int) {
	g.edit(func() { g.g.Pulls[n].State, g.g.Pulls[n].Merged, g.g.Issues[n].State = "closed", true, "closed" })
}

// Criterion 1: when the last story of MS1 merges, the gate runs on the dev
// tip: the required checks (not the CODEOWNERS check), verify --check all,
// and the evals of the seats MS1 touched (L02: code-architect, L03:
// engineering-manager), and no model decides.
func TestGateRunsChecksVerifyAndEvals(t *testing.T) {
	g := newGateWorld(t, "MS1=#20,#21 MS2=#22", 20, 21, 22)
	g.edit(func() {
		is := g.g.Issues[21]
		i := slices.IndexFunc(is.Labels, func(l fake.Label) bool { return l.Name == "layer:L02" })
		is.Labels[i] = fake.Label{Name: "layer:L03"}
	})
	g.closeIssue(20)
	if code, out, errOut := g.gate(); code != 0 || g.calls != 0 || len(g.runs()) != 0 ||
		!strings.Contains(out, "MS1       building: 1 of 2 merged; the gate runs when its last story merges\n") {
		t.Fatalf("gate before MS1's last merge: exit %d, verify runs %d: %q%s", code, g.calls, out, errOut)
	}
	g.closeIssue(21)
	code, out, errOut := g.gate()
	if code != 0 || g.calls != 1 || !slices.Equal(g.seats, []string{"code-architect", "engineering-manager"}) {
		t.Fatalf("gate after MS1's last merge: exit %d, verify runs %d, eval seats %v: %s%s", code, g.calls, g.seats, out, errOut)
	}
	runs := g.runs()
	var names []string
	for _, ck := range runs[0].Checks {
		names = append(names, ck.Check+"="+ck.Result)
	}
	want := []string{"check:build=passed", "check:lint=passed", "verify:mission.next-order=passed", "eval:plan/small-story=passed"}
	if len(runs) != 1 || runs[0].DevSHA != g.sha || runs[0].Milestone != "MS1" || runs[0].Result != "passed" || !slices.Equal(names, want) {
		t.Fatalf("gate record: %+v, want checks %v", runs, want)
	}
	// The tip is unchanged: nothing runs or is written again.
	n := g.commits()
	if code, out, _ := g.gate(); code != 0 || g.calls != 1 || g.commits() != n || !strings.Contains(out, "MS1       passed on dev 9f8e7d6") {
		t.Fatalf("second gate on the same tip: exit %d, verify runs %d: %s", code, g.calls, out)
	}
}

// Criterion 2: the gate passes, so the plan marks MS1 passed with the dev
// SHA, and show stops holding MS2's PRs behind it (ST16 reads the state).
func TestGatePassMarksMilestone(t *testing.T) {
	g := newGateWorld(t, "MS1=#20 MS2=#22", 20, 22)
	show := func() MissionShowReport {
		code, out, errOut := run(t, g.agent, g.opts, "mission", "show", "889", "--repo", "o/r", "--json")
		var rep MissionShowReport
		if code != 0 || json.Unmarshal([]byte(out), &rep) != nil {
			t.Fatalf("mission show: exit %d: %s%s", code, out, errOut)
		}
		return rep
	}
	if s := show(); !strings.HasSuffix(s.Milestones[1].State, "; its PRs merge after MS1's gate passes") {
		t.Fatalf("MS2 before MS1's gate: %q", s.Milestones[1].State)
	}
	g.closeIssue(20)
	rep := g.gateJSON()
	p, raw := g.plan(889)
	at := p.Milestones[0].Gate.At
	if p.Milestones[0].Gate != (MilestoneGate{State: GatePassed, DevSHA: g.sha, At: at}) || at == "" || p.Milestones[1].Gate.State != GateWaits ||
		rep.Milestones[0].State != GatePassed || !strings.Contains(raw, `"state": "passed",`+"\n"+`        "dev_sha": "`+g.sha+`"`) {
		t.Fatalf("plan after a passed gate:\n%s", raw)
	}
	if s := show(); s.Milestones[0].State != fmt.Sprintf("passed its gate on dev 9f8e7d6 at %s", at) || strings.Contains(s.Milestones[1].State, "merge after") {
		t.Fatalf("show after the pass: %+v", s.Milestones)
	}
	if rep.Milestones[1].Detail != "building: 0 of 1 merged; the gate runs when its last story merges" || len(rep.Fixes) != 0 {
		t.Fatalf("MS2 after MS1 passed: %+v", rep)
	}
}

// Criteria 3 and 5: the gate fails on a verify check and an eval. Each
// failed check gets one fix, started as one subthread with its brief; a
// second gate run starts neither again. Each fix PR says Refs #889, files no
// issue, and is in MS1, so MS1's gate runs again once both merge.
func TestGateFailOpensOneFixPRPerCheck(t *testing.T) {
	g := newGateWorld(t, "MS1=#20 MS2=#22", 20, 22)
	g.closeIssue(20)
	g.vfail, g.efail = []string{"inbox.ship-once"}, []string{"mission/stays-in-list"}
	rep := g.gateJSON()
	if len(rep.Fixes) != 2 || rep.Milestones[0].State != GateFailed {
		t.Fatalf("gate with two failed checks: %+v", rep)
	}
	for i, check := range []string{"verify:inbox.ship-once", "eval:mission/stays-in-list"} {
		fx := rep.Fixes[i]
		if fx.Action != "start" || fx.Check != check || fx.Milestone != "MS1" || fx.DevSHA != g.sha ||
			!strings.Contains(fx.Brief, "Its body says `Refs #889` and, on a line of its own, `Aayudha-Gate-Fix: MS1 "+check+"`") || !strings.Contains(fx.Brief, "File no issue.") {
			t.Fatalf("fix %d: %+v", i, fx)
		}
	}
	if got := g.claims(889); len(got) != 2 {
		t.Fatalf("claims of the two fixes: %v", got)
	}
	// One failed check gets one fix: the next run says the subthreads have them.
	rep = g.gateJSON()
	if rep.Fixes[0].Action != "started" || rep.Fixes[1].Action != "started" || len(g.claims(889)) != 2 || g.calls != 1 {
		t.Fatalf("second gate run: %+v, claims %v, verify runs %d", rep.Fixes, g.claims(889), g.calls)
	}
	g.g.Reset()
	g.fixPR(981, "verify:inbox.ship-once", "Refs #889")
	g.fixPR(982, "eval:mission/stays-in-list", "Refs #889")
	for _, n := range []string{"981", "982"} {
		if code, out, errOut := g.gate("--pr", n); code != 0 || !strings.Contains(out, "Fix PR #"+n+" is the fix of MS1") {
			t.Fatalf("gate --pr %s: exit %d: %s%s", n, code, out, errOut)
		}
	}
	for _, w := range g.g.Writes() {
		if w.Method == "POST" && strings.HasSuffix(w.Path, "/issues") {
			t.Fatalf("a fix filed an issue: %+v", w)
		}
	}
	if code, out, _ := g.gate(); code != 0 || !strings.Contains(out, "the gate runs again when its fixes merge: #981, #982") ||
		!strings.Contains(out, "fixing fix MS1 verify:inbox.ship-once · PR #981\nfixing fix MS1 eval:mission/stays-in-list · PR #982\n") {
		t.Fatalf("gate with both fixes open: exit %d: %s", code, out)
	}
	g.mergePR(981)
	g.tip(strings.Repeat("a", 40), "success")
	if code, out, _ := g.gate(); code != 0 || g.calls != 1 || !strings.Contains(out, "its fixes merge: #982") {
		t.Fatalf("gate with #982 open: exit %d, verify runs %d: %s", code, g.calls, out)
	}
	g.mergePR(982)
	g.vfail, g.efail = nil, nil
	if rep := g.gateJSON(); rep.Milestones[0].State != GatePassed || rep.DevSHA != strings.Repeat("a", 40) || len(rep.Fixes) != 0 {
		t.Fatalf("gate after both fixes merged: %+v", rep)
	}
}

// Criterion 4: fix PR #981 opens, and the plan gains one gate_fixes entry
// with its PR, milestone, check, dev SHA, the approved record and its hash,
// and the time. No issue and no triage record is written; a PR that is no
// fix of a failed check is refused.
func TestGateFixesEntryShape(t *testing.T) {
	g := newGateWorld(t, "MS1=#20 MS2=#22", 20, 22)
	g.closeIssue(20)
	g.vfail = []string{"inbox.ship-once"}
	g.gateJSON()
	g.fixPR(981, "verify:inbox.ship-once", "Refs #889")
	g.fixPR(983, "verify:inbox.ship-once", "Refs #889")
	g.fixPR(984, "verify:inbox.ship-once", "Refs #870")
	g.fixPR(985, "verify:mission.next-order", "Refs #889")
	g.g.Reset()
	before := git(t, g.bare, "ls-tree", "-r", "--name-only", "ledger")
	if code, out, errOut := g.gate("--pr", "981"); code != 0 {
		t.Fatalf("gate --pr 981: exit %d: %s%s", code, out, errOut)
	}
	p, _ := g.plan(889)
	sum := sha256.Sum256([]byte(git(t, g.bare, "show", "ledger:"+ready.ApprovalPath(889)) + "\n"))
	if len(p.GateFixes) != 1 {
		t.Fatalf("gate_fixes: %+v", p.GateFixes)
	}
	at := p.GateFixes[0].At
	if _, err := time.Parse(time.RFC3339, at); err != nil {
		t.Fatalf("at %q: %v", at, err)
	}
	b, _ := json.Marshal(p.GateFixes[0])
	want := fmt.Sprintf(`{"pr":981,"milestone":"MS1","check":"verify:inbox.ship-once","dev_sha":"%s","approval":{"record":"specs/889/approved.json","sha256":"%s"},"at":"%s"}`,
		g.sha, hex.EncodeToString(sum[:]), at)
	if string(b) != want {
		t.Fatalf("gate_fixes entry:\n got %s\nwant %s", b, want)
	}
	after := git(t, g.bare, "ls-tree", "-r", "--name-only", "ledger")
	if after != before || strings.Contains(after, "triage") {
		t.Fatalf("the fix wrote a ledger file besides the plan:\n%s", after)
	}
	for _, w := range g.g.Writes() {
		if strings.Contains(w.Path, "/issues") {
			t.Fatalf("the fix wrote to an issue: %+v", w)
		}
	}
	for n, why := range map[string]string{
		"983": "verify:inbox.ship-once of MS1 has its fix PR already, #981: one failed check gets one fix PR",
		"984": "#984 does not say `Refs #889`",
		"985": "verify:mission.next-order did not fail in the last gate of MS1 of #889: a fix PR comes only from a failed gate check (Decision #974)",
	} {
		if code, _, errOut := g.gate("--pr", n); code != ExitBlocked || !strings.Contains(errOut, why) {
			t.Errorf("gate --pr %s: exit %d: %s", n, code, errOut)
		}
	}
	if p, _ := g.plan(889); len(p.GateFixes) != 1 {
		t.Fatalf("a refused fix was recorded: %+v", p.GateFixes)
	}
}

// Criterion 6: #981 spent 14 USD, #889 has 40 USD left, and #982 is
// estimated at 64 USD (the per-story cap). Show counts the 14 USD; #982
// waits, and one inbox card shows the spend and the estimate and offers the
// raise and keeping it waiting. A raise is a person's, with a reason.
func TestGateFixCountsAgainstBudget(t *testing.T) {
	stories := []int{20, 21, 22, 23, 24, 25, 26, 27, 28, 29}
	g := newGateWorld(t, "MS1=#20,#21,#22,#23,#24,#25,#26,#27,#28,#29 MS2=#30", append(slices.Clone(stories), 30)...)
	appr := git(t, g.bare, "show", "ledger:"+ready.ApprovalPath(889))
	putLedger(t, g.bare, ready.ApprovalPath(889), strings.Replace(appr, `"per_story_usd": 60`, `"per_story_usd": 64`, 1))
	for _, n := range stories {
		g.closeIssue(n)
	}
	g.vfail, g.efail = []string{"inbox.ship-once"}, []string{"mission/stays-in-list"}
	g.gateJSON()
	g.fixPR(981, "verify:inbox.ship-once", "Refs #889")
	g.fixPR(982, "eval:mission/stays-in-list", "Refs #889")
	g.setPlan(889, func(p *MissionPlan) {
		p.Budget = 694
		p.GateFixes = []GateFix{{PR: 981, Milestone: "MS1", Check: "verify:inbox.ship-once", DevSHA: g.sha, At: "2026-10-26T10:00:00Z"}}
	})
	log := filepath.Join(g.dir, ".aayudha", "telemetry", "actions.jsonl")
	_ = os.MkdirAll(filepath.Dir(log), 0o755)
	if err := os.WriteFile(log, []byte(`{"kind":"model.call","story":"o/r#981","status":"done","cost_usd":14}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, errOut := run(t, g.agent, g.opts, "mission", "show", "889", "--repo", "o/r", "--json")
	var show MissionShowReport
	if code != 0 || json.Unmarshal([]byte(out), &show) != nil || show.Budget.Used.Value != 654 || show.Budget.Left.Value != 40 ||
		!strings.HasSuffix(show.Budget.Used.Source, "; gate fix PRs at their measured spend: #981 $14") {
		t.Fatalf("mission show: exit %d: %+v%s", code, show.Budget, errOut)
	}
	code, out, errOut = g.gate("--pr", "982")
	if code != ExitBudget || !strings.Contains(out, "Fix PR #982 of MS1 eval:mission/stays-in-list waits: it would pass #889's budget ($654 used + $64, the fix's estimate, > $694).") {
		t.Fatalf("gate --pr 982 over the budget: exit %d: %s%s", code, out, errOut)
	}
	if p, _ := g.plan(889); len(p.GateFixes) != 2 || p.GateFixes[1].PR != 982 || p.GateFixes[1].Waits != "budget" {
		t.Fatalf("gate_fixes: %+v", p.GateFixes)
	}
	if rep := g.gateJSON(); rep.Fixes[1].Action != "waits" || rep.Fixes[1].Reason != "fix PR #982 would pass #889's budget; a person decides in aayu inbox" {
		t.Fatalf("gate lists #982: %+v", rep.Fixes)
	}
	cards := func() []InboxItem {
		code, out, errOut := run(t, g.agent, g.opts, "inbox", "--list", "--json", "--repo", "o/r")
		var l inboxList
		if code != 0 || json.Unmarshal([]byte(out), &l) != nil {
			t.Fatalf("inbox --list: exit %d: %s%s", code, out, errOut)
		}
		return slices.DeleteFunc(l.Items, func(it InboxItem) bool { return it.Kind != InboxMission })
	}
	g.gate("--pr", "982") // asks once: the same ask is not posted again
	c := cards()
	if len(c) != 1 || c[0].Title != "Fix PR #982 would pass #889's budget" || c[0].Number != 889 ||
		c[0].Why != "#889 used $654 of its $694 budget, so $40 is left, and fix PR #982 is estimated at $64." ||
		!slices.Contains(c[0].Details, "Fix PR #981 (MS1 verify:inbox.ship-once) spent $14") ||
		!slices.Contains(c[0].Details, "Estimate of #982: $64, the per-story cap, until #872 measures spend") ||
		len(c[0].Options) != 2 || c[0].Options[0].Label != "Raise the budget to $720, with a reason" || c[0].Options[1].Label != "Keep #982 waiting" ||
		c[0].Options[0].Recommended || c[0].Options[1].Recommended {
		t.Fatalf("budget card: %+v", c)
	}
	if code, _, errOut := run(t, g.agent, g.opts, "mission", "budget", "889", "--to", "720", "--reason", "x", "--repo", "o/r"); code != ExitBlocked ||
		!strings.Contains(errOut, "mission budget is a person's decision and is refused for an agent lane (AMP_THREAD_ID)") {
		t.Fatalf("agent budget raise: exit %d: %s", code, errOut)
	}
	code, out, errOut = g.mission(g.person, "rahulnakmol", "budget", 889, "--to", "720", "--reason", "the fix is needed for MS1")
	if code != 0 || g.codes() != 1 || !strings.Contains(out, "raised the budget of #889 to $720; fix PRs that go on: #982") {
		t.Fatalf("budget raise: exit %d, device codes %d: %s%s", code, g.codes(), out, errOut)
	}
	p, _ := g.plan(889)
	if p.Budget != 720 || p.GateFixes[1].Waits != "" || p.Controls[len(p.Controls)-1].Reason != "the fix is needed for MS1" || len(cards()) != 0 {
		t.Fatalf("after the raise: budget %v, fixes %+v, controls %+v", p.Budget, p.GateFixes, p.Controls)
	}
}

// Criterion 7: MS1's gate failed, and MS2's stories still start; an open
// fix PR takes one agents slot. Show holds MS2's PRs behind MS1's gate.
func TestFailedGateDoesNotHoldStarts(t *testing.T) {
	g := newGateWorld(t, "MS1=#20 MS2=#22,#23", 20, 22, 23)
	g.closeIssue(20)
	g.vfail = []string{"inbox.ship-once"}
	g.gateJSON()
	g.fixPR(981, "verify:inbox.ship-once", "Refs #889")
	if code, out, errOut := g.gate("--pr", "981"); code != 0 {
		t.Fatalf("gate --pr 981: exit %d: %s%s", code, out, errOut)
	}
	putLedger(t, g.bare, MissionClaimsPath(889), "")                  // the fix's claim expired; its open PR holds the slot
	commitOnDev(g.g, ".aayudha/config.toml", "[build]\nagents = 2\n") // the gate's tip shares dev's committed files
	g.status(22, "ready", "")
	g.status(23, "ready", "")
	if code, out, errOut := g.next(889); code != 0 || out != "start #22\nwaits #23: no free agent (2 of 2 busy after these starts)\n" {
		t.Fatalf("mission next with MS1's gate failed: exit %d: %q%s", code, out, errOut)
	}
	code, out, errOut := run(t, g.agent, g.opts, "mission", "show", "889", "--repo", "o/r")
	if code != 0 || !strings.Contains(out, "MS1       failed its gate on dev 9f8e7d6: its fix PRs run, and the gate runs again when they merge") ||
		!strings.Contains(out, "MS2       building: 0 of 2 merged; the gate runs when its last story merges; its PRs merge after MS1's gate passes") {
		t.Fatalf("mission show: exit %d: %s%s", code, out, errOut)
	}
}

// Criterion 8: MS1's gate fails a third time ([mission] max_no_go = 3): the
// mission stops, and one inbox card lists each run with its check and time,
// gives no recommendation, and offers to run the gate once more or stop.
// Once more is a person's, after the presence check.
func TestGateStopsAtMaxNoGo(t *testing.T) {
	g := newGateWorld(t, "MS1=#20 MS2=#22", 20, 22)
	g.closeIssue(20)
	g.status(22, "ready", "")
	for i, check := range []string{"inbox.ship-once", "inbox.ship-once", "mission.stays"} {
		g.vfail = []string{check}
		g.tip(strings.Repeat(fmt.Sprint(i+1), 40), "success")
		rep := g.gateJSON()
		if i < 2 {
			g.fixPR(981+i, "verify:"+check, "Refs #889")
			if code, _, errOut := g.gate("--pr", fmt.Sprint(981+i)); code != 0 {
				t.Fatalf("fix %d: exit %d: %s", i, code, errOut)
			}
			g.mergePR(981 + i)
			continue
		}
		if rep.Milestones[0].State != GateStopped || rep.Milestones[0].Detail != "MS1 of #889 failed its gate 3 times: its stories wait" ||
			rep.Fixes[0].Action != "waits" || !strings.HasPrefix(rep.Fixes[0].Reason, "the mission is stopped") {
			t.Fatalf("third failed gate: %+v", rep)
		}
	}
	p, _ := g.plan(889)
	if p.Milestones[0].Gate.State != GateStopped || p.Pause == nil || p.Pause.Reason != "MS1 of #889 failed its gate 3 times: its stories wait" {
		t.Fatalf("plan after the third failure: %+v %+v", p.Milestones[0].Gate, p.Pause)
	}
	if code, _, errOut := g.next(889); code != ExitBlocked || !strings.Contains(errOut, "The mission of #889 is paused: MS1 of #889 failed its gate 3 times: its stories wait. Nothing starts.") {
		t.Fatalf("mission next after the stop: exit %d: %s", code, errOut)
	}
	if code, _, errOut := g.gate(); code != ExitBlocked || !strings.Contains(errOut, "A person runs the gate once more, or stops the mission, in aayu inbox.") {
		t.Fatalf("the Kanha's gate after the stop: exit %d: %s", code, errOut)
	}
	code, out, errOut := run(t, g.agent, g.opts, "inbox", "--list", "--json", "--repo", "o/r")
	var l inboxList
	if code != 0 || json.Unmarshal([]byte(out), &l) != nil {
		t.Fatalf("inbox --list: exit %d: %s%s", code, out, errOut)
	}
	cards := slices.DeleteFunc(l.Items, func(it InboxItem) bool { return it.Kind != InboxMission })
	runs := g.runs()
	wantLines := []string{
		fmt.Sprintf("Run 1 at %s on dev 1111111: verify:inbox.ship-once failed", runs[0].At),
		fmt.Sprintf("Run 2 at %s on dev 2222222: verify:inbox.ship-once failed", runs[1].At),
		fmt.Sprintf("Run 3 at %s on dev 3333333: verify:mission.stays failed", runs[2].At),
	}
	if len(cards) != 1 || cards[0].Title != "MS1 of #889 failed its gate 3 times: its stories wait" ||
		cards[0].Why != "The gate of MS1 failed 3 times, so the mission stopped and asks you." || !slices.Equal(cards[0].Details, wantLines) ||
		len(cards[0].Options) != 2 || cards[0].Options[0].Label != "Run the gate once more" || cards[0].Options[1].Label != "Stop the mission, with a reason" ||
		cards[0].Options[0].Recommended || cards[0].Options[1].Recommended || strings.Contains(cards[0].Counsel, "I recommend") {
		t.Fatalf("no-go card: %+v", cards)
	}
	if code, _, errOut := g.gate("--once-more"); code != ExitBlocked || !strings.Contains(errOut, "mission gate --once-more is a person's decision and is refused for an agent lane (AMP_THREAD_ID)") {
		t.Fatalf("an agent's once more: exit %d: %s", code, errOut)
	}
	g.vfail = nil
	code, out, errOut = g.mission(g.person, "rahulnakmol", "gate", 889, "--once-more")
	p, _ = g.plan(889)
	if code != 0 || g.codes() != 1 || p.Milestones[0].Gate.State != GatePassed || p.Pause != nil || p.Controls[len(p.Controls)-1].Action != "gate once more" {
		t.Fatalf("once more: exit %d, device codes %d, gate %+v, pause %+v: %s%s", code, g.codes(), p.Milestones[0].Gate, p.Pause, out, errOut)
	}
	l = inboxList{}
	if code, out, _ := run(t, g.agent, g.opts, "inbox", "--list", "--json", "--repo", "o/r"); code != 0 || json.Unmarshal([]byte(out), &l) != nil ||
		slices.ContainsFunc(l.Items, func(it InboxItem) bool { return it.Kind == InboxMission }) {
		t.Fatalf("the card after the pass: %s", out)
	}
	if code, out, errOut := g.next(889); code != 0 || out != "start #22\n" {
		t.Fatalf("mission next after the pass: exit %d: %q%s", code, out, errOut)
	}
}

// Criterion 9: a required check cancelled, or failed with a 5xx from
// GitHub, is rerun once, and the gate records why; a second failure fails
// the gate, and a pass after the rerun passes it.
func TestGateRerunsRunnerErrorOnce(t *testing.T) {
	g := newGateWorld(t, "MS1=#20 MS2=#22", 20, 22)
	g.closeIssue(20)
	var reruns []int64
	g.edit(func() { g.g.OnRerunJob = func(id int64) { reruns = append(reruns, id) } })
	g.tip(strings.Repeat("c", 40), "cancelled")
	rep := g.gateJSON()
	if !slices.Equal(reruns, []int64{70}) || rep.Milestones[0].State != GateWaits ||
		rep.Milestones[0].Detail != "pending: build is cancelled on ccccccc; aayu reran it once (runner error: cancelled)" || g.calls != 0 {
		t.Fatalf("gate on a cancelled check: reruns %v: %+v", reruns, rep.Milestones[0])
	}
	if r := g.runs(); len(r) != 1 || r[0].Result != gatePending || r[0].Checks[0].Rerun != "runner error: cancelled" {
		t.Fatalf("the rerun's record: %+v", r)
	}
	// The rerun is cancelled again: that is a failed gate, and nothing reruns twice.
	g.edit(func() {
		runs := g.g.CheckRuns[g.sha]
		g.g.CheckRuns[g.sha] = append([]fake.CheckRun{{ID: 73, Name: "build", Status: "completed", Conclusion: "cancelled", Suite: &fake.Suite{ID: 1}}}, runs[1:]...)
	})
	rep = g.gateJSON()
	if len(reruns) != 1 || rep.Milestones[0].State != GateFailed || rep.Fixes[0].Check != "check:build" ||
		rep.Milestones[0].Checks[0].Detail != "build is cancelled on ccccccc again after its one rerun (runner error: cancelled)" {
		t.Fatalf("second runner error: reruns %v: %+v", reruns, rep)
	}
	// A 5xx from GitHub in the job's log reruns it once; a pass after the rerun passes the gate.
	g.setPlan(889, func(p *MissionPlan) { p.Milestones[0].Gate = MilestoneGate{State: GateWaits} })
	putLedger(t, g.bare, MissionGatesPath(889), "")
	g.tip(strings.Repeat("d", 40), "failure")
	g.edit(func() {
		g.g.JobLogs = map[int64]string{70: "go: downloading modules\nerror: GitHub returned HTTP 502 Bad Gateway\n"}
	})
	if rep := g.gateJSON(); !slices.Equal(reruns, []int64{70, 70}) || !strings.Contains(rep.Milestones[0].Detail, "runner error: its log names a 5xx from GitHub") {
		t.Fatalf("5xx in the log: reruns %v: %+v", reruns, rep.Milestones[0])
	}
	g.edit(func() { g.g.CheckRuns[g.sha][0].ID, g.g.CheckRuns[g.sha][0].Conclusion = 74, "success" })
	if rep := g.gateJSON(); rep.Milestones[0].State != GatePassed {
		t.Fatalf("pass after the rerun: %+v", rep.Milestones[0])
	}
}
