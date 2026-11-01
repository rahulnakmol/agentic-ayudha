package cli

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/delegate"
	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/models"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
	"github.com/rahulnakmol/agentic-ayudha/internal/verify"
)

// Story #1001: the milestone gate on the missionWorld of Stories #997 and
// #998, with the dev tip on a fake GitHub and stub verify and eval runners.
// No model is called: ModelClient fails the test if anything asks for one.

type gateWorld struct {
	*missionWorld
	sha   string
	gmu   sync.Mutex
	verr  error    // the verify runner's error
	vfail []string // verify checks that fail on the tip
	efail []string // eval cases that fail on the tip
	seats []string // the seats the eval runner was asked for
	calls int      // verify runs
}

// newGateWorld starts #889's mission with milestones ms ("MS1=#20,#21
// MS2=#22"), its stories as listed, and dev at a tip whose required checks
// build and lint pass.
func newGateWorld(t *testing.T, ms string, stories ...int) *gateWorld {
	return newBugsGateWorld(t, ms, stories, nil)
}

// newBugsGateWorld is newGateWorld with Bugs too, and with no --milestones when ms is "".
func newBugsGateWorld(t *testing.T, ms string, stories, bugs []int) *gateWorld {
	g := &gateWorld{missionWorld: newMissionWorld(t)}
	m := g.missionWorld
	m.issue(889, "Agents pull the next story", "feature", 0, "")
	for _, n := range stories {
		m.issue(n, fmt.Sprintf("Story %d", n), "story", 889, "")
	}
	for _, n := range bugs {
		m.issue(n, fmt.Sprintf("Bug %d", n), "bug", 889, "")
	}
	m.feature(889, stories...)
	m.mu.Lock()
	m.tool = "amp"
	m.mu.Unlock()
	var args []string
	if ms != "" {
		args = []string{"--milestones", ms}
	}
	if code, out, errOut := m.approve(889, args...); code != 0 {
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
		if g.verr != nil {
			return nil, g.verr
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

// eachRequestFails runs cmd, then runs it again once for each GitHub
// request it made, with that request failing with a 500 and the ledger as
// it was: each run exits 5 with GitHub's error and reports no success. The
// ledger is then as it was before, so the test goes on from that state.
func (g *gateWorld) eachRequestFails(cmd func() (int, string, string)) {
	t := g.t
	t.Helper()
	tip := git(t, g.bare, "rev-parse", "ledger")
	g.gmu.Lock()
	calls := g.calls
	g.gmu.Unlock()
	comments := map[int][]*fake.Comment{} // the asks a run posts or closes
	g.edit(func() {
		for n, is := range g.g.Issues {
			comments[n] = slices.Clone(is.Comments)
		}
	})
	reset := func() {
		git(t, g.bare, "update-ref", "refs/heads/ledger", tip)
		g.edit(func() {
			for n, is := range g.g.Issues {
				is.Comments = slices.Clone(comments[n])
			}
		})
	}
	g.g.Reset()
	if code, out, errOut := cmd(); code != 0 && code != ExitBudget {
		t.Fatalf("the clean run: exit %d: %s%s", code, out, errOut)
	}
	g.g.Mu.Lock()
	reqs := slices.Clone(g.g.Requests)
	g.g.Mu.Unlock()
	seen := map[string]int{}
	for _, r := range reqs {
		k := r.Method + " " + r.Path
		if strings.HasPrefix(r.Path, "/login/") || r.Path == "/user" || strings.Contains(r.Path, "/actions/variables/") { // the presence check and the halt guard have their own tests
			continue
		}
		n := seen[k]
		seen[k]++
		reset()
		seenK, fired := 0, false
		g.edit(func() {
			g.g.Hook = func(method, path string) { // the n-th request k fails
				if method+" "+path == k && !fired {
					if seenK == n {
						g.g.Fail, g.g.FailAfter, fired = k, 0, true
					}
					seenK++
				}
			}
		})
		code, out, errOut := cmd()
		g.edit(func() { g.g.Hook, g.g.Fail = nil, "" })
		if !fired {
			t.Errorf("%s (time %d) was not requested again", k, n+1)
		}
		if code != ExitExternal || !strings.Contains(errOut, "500") {
			t.Errorf("%s (time %d) failing: exit %d: %s%s", k, n+1, code, out, errOut)
		}
	}
	reset()
	g.gmu.Lock()
	g.calls = calls
	g.gmu.Unlock()
}

// answer answers #889's mission card in aayu inbox as @rahulnakmol, with
// keys (an option, then y or the answer's line), past the cards before it.
func (g *gateWorld) answer(keys string) (int, string, string) {
	g.t.Helper()
	code, out, errOut := run(g.t, g.person, g.opts, "inbox", "--list", "--json", "--repo", "o/r")
	var l inboxList
	if code != 0 || json.Unmarshal([]byte(out), &l) != nil {
		g.t.Fatalf("inbox --list: exit %d: %s%s", code, out, errOut)
	}
	at := slices.IndexFunc(l.Items, func(it InboxItem) bool { return it.Kind == InboxMission })
	if at < 0 || at >= l.NeedsYou {
		g.t.Fatalf("no mission card: %+v", l.Items)
	}
	g.device("rahulnakmol")
	return runIn(g.t, g.person, g.opts, strings.Repeat("s ", at)+keys+"\nq", "inbox", "--plain", "--repo", "o/r")
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
	// A sweep that cannot run is no gate result: the command fails, and no run is recorded.
	g.verr = Errorf(ExitExternal, "verify: the sweep cannot start")
	if code, _, errOut := g.gate(); code != ExitExternal || !strings.Contains(errOut, "the sweep cannot start") || len(g.runs()) != 0 {
		t.Fatalf("gate when verify cannot run: exit %d: %s", code, errOut)
	}
	g.verr, g.calls = nil, 0
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
	// A plan Story #997 wrote has each gate as a string: it reads as a gate that waits.
	raw := git(t, g.bare, "show", "ledger:"+MissionPlanPath(889))
	old := strings.ReplaceAll(raw, `"gate": {`+"\n"+`        "state": "waits"`+"\n"+`      }`, `"gate": "runs when its last story merges"`)
	if strings.Count(old, `"gate": "runs when its last story merges"`) != 2 {
		t.Fatalf("the plan's gates:\n%s", raw)
	}
	putLedger(t, g.bare, MissionPlanPath(889), old)
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
	g.status(20, "merged", "") // ship leaves #20 open until its release (Bug #1169)
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
	g.eachRequestFails(func() (int, string, string) { return g.gate() })
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
	// Two fix PRs race for the room of one: #981 is recorded while #982's run reads the plan, so #982's write
	// sees it on the ledger tip, posts its budget card once, and waits ($60 merged + $60 for open #981 + $60 > $170).
	g.setPlan(889, func(p *MissionPlan) { p.Budget = 170 })
	comments := len(g.g.Issues[889].Comments)
	fired := false
	g.edit(func() {
		g.g.Hook = func(method, path string) {
			if !fired && method == "GET" && path == "/repos/o/r/pulls/982" {
				fired = true
				g.setPlan(889, func(p *MissionPlan) {
					p.GateFixes = append(p.GateFixes, GateFix{PR: 981, Milestone: "MS1", Check: "verify:inbox.ship-once", DevSHA: g.sha, At: "2026-10-26T10:00:00Z"})
				})
			}
		}
	})
	code, out, errOut := g.gate("--pr", "982")
	g.edit(func() { g.g.Hook = nil })
	p, _ := g.plan(889)
	asks := 0
	for _, c := range g.g.Issues[889].Comments[comments:] {
		if strings.HasPrefix(c.Body, MissionAskMarker+`{"id":"budget-982"`) {
			asks++
		}
	}
	if code != ExitBudget || !fired || !strings.Contains(out, "($120 used + $60, the fix's estimate, > $170)") || asks != 1 ||
		len(p.GateFixes) != 2 || p.GateFixes[0].Waits != "" || p.GateFixes[1].PR != 982 || p.GateFixes[1].Waits != "budget" {
		t.Fatalf("gate --pr 982 racing #981: exit %d, asks %d, fixes %+v: %s%s", code, asks, p.GateFixes, out, errOut)
	}
	g.edit(func() { g.g.Issues[889].Comments = g.g.Issues[889].Comments[:comments] })
	g.setPlan(889, func(p *MissionPlan) { p.Budget, p.GateFixes = 600, nil }) // room for both open fixes at their cap
	g.eachRequestFails(func() (int, string, string) { return g.gate("--pr", "981") })
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
	g.eachRequestFails(func() (int, string, string) { return g.gate() })
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
	// --dry-run lists the fix and writes nothing.
	n := g.commits()
	if code, out, errOut := g.gate("--dry-run"); code != 0 || g.commits() != n || len(g.claims(889)) != 0 ||
		!strings.Contains(out, "start fix MS1 verify:inbox.ship-once\n") || !strings.HasSuffix(out, "--dry-run: nothing written, no check rerun, and no fix claimed\n") {
		t.Fatalf("gate --dry-run: exit %d, claims %v: %s%s", code, g.claims(889), out, errOut)
	}
	g.gateJSON()
	g.fixPR(981, "verify:inbox.ship-once", "Refs #889")
	g.fixPR(983, "verify:inbox.ship-once", "Refs #889")
	g.fixPR(984, "verify:inbox.ship-once", "Refs #870")
	g.fixPR(985, "verify:mission.next-order", "Refs #889")
	g.fixPR(986, "verify:inbox.ship-once", "Refs #889")
	g.fixPR(987, "verify:inbox.ship-once", "Refs #889")
	g.edit(func() {
		g.g.Pulls[986].State = "closed"
		g.g.Pulls[987].Body = "Refs #889\n\nFixes the ship-once check.\n"
	})
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
		"986": "#986 is not an open PR to dev: a gate fix is one",
		"987": "#987 names no failed check: its body needs the line `Aayudha-Gate-Fix: <milestone> <check>`",
	} {
		if code, _, errOut := g.gate("--pr", n); code != ExitBlocked || !strings.Contains(errOut, why) {
			t.Errorf("gate --pr %s: exit %d: %s", n, code, errOut)
		}
	}
	if p, _ := g.plan(889); len(p.GateFixes) != 1 {
		t.Fatalf("a refused fix was recorded: %+v", p.GateFixes)
	}
	// The fix merged, but dev did not move: the gate judged this tip already, so it does not run again.
	g.mergePR(981)
	calls := g.calls
	if code, out, _ := g.gate(); code != 0 || g.calls != calls || !strings.Contains(out, "MS1       failed on dev 9f8e7d6, the tip now\n") {
		t.Fatalf("gate on the tip it failed: exit %d, verify runs %d: %s", code, g.calls, out)
	}
}

// No gate runs, and no fix or budget is recorded, for a Feature with no
// plan or a mission no one started; a paused mission runs no gate, and
// --once-more needs a stopped gate.
func TestGateRefusesWithoutARunningMission(t *testing.T) {
	g := newGateWorld(t, "MS1=#20 MS2=#22", 20, 22)
	g.issue(901, "Feature 901", "feature", 0, "")
	g.issue(60, "Story 60", "story", 901, "")
	g.feature(901, 60)
	refused := func(env map[string]string, f int, why string, args ...string) {
		t.Helper()
		n := g.commits()
		if code, out, errOut := run(t, env, g.opts, append([]string{"mission", args[0], fmt.Sprint(f), "--repo", "o/r"}, args[1:]...)...); code != ExitBlocked ||
			!strings.Contains(errOut, why) || g.commits() != n {
			t.Errorf("mission %v %d: exit %d: %s%s", args, f, code, out, errOut)
		}
	}
	refused(g.agent, 901, "#901 has no mission plan: a person approves it to build first. No gate runs.", "gate")
	if code, out, errOut := g.approve(901); code != 0 {
		t.Fatalf("approve 901: exit %d: %s%s", code, out, errOut)
	}
	refused(g.agent, 901, "The mission of #901 has not started: a person starts it. No gate runs.", "gate")
	refused(g.agent, 901, "#901 has no started mission, so it has no gate fix", "gate", "--pr", "981")
	refused(g.person, 901, "#901 has no started mission, so it has no budget to change", "budget", "--keep", "981")
	refused(g.person, 889, "No gate of #889 is stopped: the gate runs on its own when a milestone's last story merges.", "gate", "--once-more")
	g.setPlan(889, func(p *MissionPlan) {
		p.Pause = &MissionPause{Reason: "a migration needs a person", By: "rahulnakmol", At: "2026-10-26T12:00:00Z"}
	})
	g.closeIssue(20)
	refused(g.agent, 889, "The mission of #889 is paused: a migration needs a person. No gate runs.", "gate")
	if g.calls != 0 || len(g.runs()) != 0 {
		t.Fatalf("a refused gate ran: verify runs %d, gate runs %v", g.calls, g.runs())
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
	show := func() *MissionBudgetView {
		code, out, errOut := run(t, g.agent, g.opts, "mission", "show", "889", "--repo", "o/r", "--json")
		var rep MissionShowReport
		if code != 0 || json.Unmarshal([]byte(out), &rep) != nil || rep.Budget == nil {
			t.Fatalf("mission show: exit %d: %s%s", code, out, errOut)
		}
		return rep.Budget
	}
	g.eachRequestFails(func() (int, string, string) {
		return run(t, g.agent, g.opts, "mission", "show", "889", "--repo", "o/r")
	})
	// While #981 is open it may still spend its cap: it counts at least at $64, so a second fix cannot take the same $40.
	if b := show(); b.Used.Value != 704 || !strings.HasSuffix(b.Used.Source, "; gate fix PRs: #981 $64 (open: at least the per-story cap)") {
		t.Fatalf("mission show with #981 open: %+v", b)
	}
	g.mergePR(981)
	if b := show(); b.Used.Value != 654 || b.Left.Value != 40 || !strings.HasSuffix(b.Used.Source, "; gate fix PRs: #981 $14 measured") {
		t.Fatalf("mission show after #981 merged: %+v", b)
	}
	// A running story holds its cap too: with room for #982 but #30 in progress, #982 waits (--dry-run writes nothing).
	g.setPlan(889, func(p *MissionPlan) { p.Budget = 720 })
	g.status(30, "in-progress", "")
	if code, out, errOut := g.gate("--pr", "982", "--dry-run"); code != ExitBudget || !strings.Contains(out, "($718 used + $64, the fix's estimate, > $720)") {
		t.Fatalf("gate --pr 982 with #30 running: exit %d: %s%s", code, out, errOut)
	}
	g.status(30, "ready", "")
	if code, out, errOut := g.gate("--pr", "982", "--dry-run"); code != 0 || !strings.Contains(out, "Fix PR #982 is the fix of MS1") {
		t.Fatalf("gate --pr 982 with room and nothing running: exit %d: %s%s", code, out, errOut)
	}
	g.setPlan(889, func(p *MissionPlan) { p.Budget = 694 })
	g.eachRequestFails(func() (int, string, string) { return g.gate("--pr", "982") })
	// The card cannot be posted: the fix is not recorded, so the next --pr asks again.
	g.edit(func() { g.g.Fail = "POST /repos/o/r/issues/889/comments" })
	if code, _, errOut := g.gate("--pr", "982"); code != ExitExternal {
		t.Fatalf("gate --pr 982 when the card cannot be posted: exit %d: %s", code, errOut)
	}
	if p, _ := g.plan(889); len(p.GateFixes) != 1 {
		t.Fatalf("a fix whose card was not posted was recorded: %+v", p.GateFixes)
	}
	// The card posts, but the ledger refuses the write: the next --pr records the fix and does not post the card again.
	hooks := t.TempDir()
	if err := os.WriteFile(filepath.Join(hooks, "pre-receive"), []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, g.bare, "config", "core.hooksPath", hooks)
	if code, _, errOut := g.gate("--pr", "982"); code != ExitExternal {
		t.Fatalf("gate --pr 982 when the ledger refuses the write: exit %d: %s", code, errOut)
	}
	git(t, g.bare, "config", "--unset", "core.hooksPath")
	asks := func() int {
		g.g.Mu.Lock()
		defer g.g.Mu.Unlock()
		n := 0
		for _, c := range g.g.Issues[889].Comments {
			if strings.HasPrefix(c.Body, MissionAskMarker+`{"id":"budget-982"`) {
				n++
			}
		}
		return n
	}
	if n := asks(); n != 1 {
		t.Fatalf("budget asks of #982 after the refused write: %d", n)
	}
	code, out, errOut := g.gate("--pr", "982")
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
	if len(c) != 1 || asks() != 1 || c[0].Title != "Fix PR #982 would pass #889's budget" || c[0].Number != 889 || c[0].short() != "fix over budget" ||
		c[0].Why != "#889 used $654 of its $694 budget, so $40 is left, and fix PR #982 is estimated at $64." ||
		!slices.Contains(c[0].Details, "Fix PR #981 (MS1 verify:inbox.ship-once) spent $14") ||
		!slices.Contains(c[0].Details, "Estimate of #982: $64, the per-story cap, until #872 measures spend") ||
		len(c[0].Options) != 2 || c[0].Options[0].Label != "Raise the budget to $720, with a reason" || c[0].Options[1].Label != "Keep #982 waiting" ||
		c[0].Options[0].Recommended || c[0].Options[1].Recommended {
		t.Fatalf("budget card: %+v", c)
	}
	// A shell with a host's agent marker asks for the device code (Bug #1182): another account's code keeps the card open.
	g.device("mallory")
	if code, _, errOut := run(t, g.agent, g.opts, "mission", "budget", "889", "--keep", "982", "--repo", "o/r"); code != ExitBlocked || g.codes() != 1 ||
		!strings.Contains(errOut, "approved by @mallory, but aayu acts as @rahulnakmol") || len(cards()) != 1 {
		t.Fatalf("budget --keep in a marked shell: exit %d, device codes %d: %s", code, g.codes(), errOut)
	}
	// Keep #982 waiting: its card closes, and it still waits.
	g.eachRequestFails(func() (int, string, string) {
		return run(t, g.person, g.opts, "mission", "budget", "889", "--keep", "982", "--repo", "o/r")
	})
	if code, out, errOut := g.answer("2 y"); code != 0 || !strings.Contains(out, "Fix PR #982 of #889 keeps waiting; the budget stays $694.\n") || len(cards()) != 0 {
		t.Fatalf("the card's keep (aayu mission budget 889 --keep 982): exit %d: %s%s", code, out, errOut)
	}
	if p, _ := g.plan(889); p.GateFixes[1].Waits != "budget" {
		t.Fatalf("a kept fix stopped waiting: %+v", p.GateFixes[1])
	}
	for args, why := range map[string]string{"--to 600 --reason x": "give a budget above $694", "--to 720": "needs --reason", "--keep 981": "#981 is no gate fix of #889 that waits for the budget"} {
		if code, _, errOut := run(t, g.person, g.opts, append([]string{"mission", "budget", "889", "--repo", "o/r"}, strings.Fields(args)...)...); code == 0 || !strings.Contains(errOut, why) {
			t.Errorf("budget %s: exit %d: %s", args, code, errOut)
		}
	}
	if code, _, errOut := run(t, g.lane, g.opts, "mission", "budget", "889", "--to", "720", "--reason", "x", "--repo", "o/r"); code != ExitBlocked ||
		!strings.Contains(errOut, "mission budget is a person's decision and is refused for an agent lane (AAYU_AGENT=1)") {
		t.Fatalf("agent budget raise: exit %d: %s", code, errOut)
	}
	g.eachRequestFails(func() (int, string, string) {
		return g.mission(g.person, "rahulnakmol", "budget", 889, "--to", "720", "--reason", "the fix is needed for MS1")
	})
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
	g.vfail = []string{"inbox.ship-once", "mission.stays"}
	// A fix takes an agents slot as a story does: with one agent and #22 in progress, both fixes wait.
	commitOnDev(g.g, ".aayudha/config.toml", "[build]\nagents = 1\n")
	g.status(22, "in-progress", "")
	actions := func() string {
		var out []string
		for _, fx := range g.gateJSON().Fixes {
			out = append(out, fx.Action+" "+fx.Check+" "+fx.Reason)
		}
		return strings.Join(out, "; ")
	}
	if got := actions(); got != "waits verify:inbox.ship-once no free agent (1 of 1 busy); waits verify:mission.stays no free agent (1 of 1 busy)" || len(g.claims(889)) != 0 {
		t.Fatalf("gate with no free agent: %s, claims %v", got, g.claims(889))
	}
	// #22 is done: the first fix takes the slot, and the second waits for it.
	g.status(22, "ready", "")
	if got := actions(); !strings.HasPrefix(got, "start verify:inbox.ship-once ; waits verify:mission.stays no free agent (1 of 1 busy)") {
		t.Fatalf("gate with one free agent: %s", got)
	}
	g.fixPR(981, "verify:inbox.ship-once", "Refs #889")
	if code, out, errOut := g.gate("--pr", "981"); code != 0 {
		t.Fatalf("gate --pr 981: exit %d: %s%s", code, out, errOut)
	}
	// The fix's claim is gone, and its open PR still holds the slot.
	putLedger(t, g.bare, MissionClaimsPath(889), "")
	if got := actions(); got != "fixing verify:inbox.ship-once ; waits verify:mission.stays no free agent (1 of 1 busy)" {
		t.Fatalf("gate with fix PR #981 open: %s", got)
	}
	g.eachRequestFails(func() (int, string, string) { return g.next(889) })
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
		if i == 2 {
			g.eachRequestFails(func() (int, string, string) { return g.gate() })
			// The card cannot be posted: nothing is recorded, so the next run stops the mission and posts it.
			g.edit(func() { g.g.Fail = "POST /repos/o/r/issues/889/comments" })
			if code, _, errOut := g.gate(); code != ExitExternal || len(g.runs()) != 2 {
				t.Fatalf("gate when the card cannot be posted: exit %d, runs %d: %s", code, len(g.runs()), errOut)
			}
		}
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
	if code, out, errOut := run(t, g.agent, g.opts, "mission", "show", "889", "--repo", "o/r"); code != 0 ||
		!strings.Contains(out, "MS1       stopped: its gate failed on dev 3333333, and the mission asks a person · #20 merged\n") {
		t.Fatalf("mission show after the stop: exit %d: %s%s", code, out, errOut)
	}
	g.eachRequestFails(func() (int, string, string) {
		return run(t, g.agent, g.opts, "inbox", "--list", "--json", "--repo", "o/r")
	})
	if code, _, errOut := g.gate(); code != ExitBlocked || !strings.Contains(errOut, "A person runs aayu mission gate 889 --once-more, or stops the mission.") {
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
	if len(cards) != 1 || cards[0].Title != "MS1 of #889 failed its gate 3 times: its stories wait" || cards[0].short() != "gate stopped" ||
		cards[0].Why != "The gate of MS1 failed 3 times, so the mission stopped and asks you." || !slices.Equal(cards[0].Details, wantLines) ||
		len(cards[0].Options) != 2 || cards[0].Options[0].Label != "Run the gate once more" || cards[0].Options[1].Label != "Stop the mission, with a reason" ||
		cards[0].Options[0].Recommended || cards[0].Options[1].Recommended || strings.Contains(cards[0].Counsel, "I recommend") {
		t.Fatalf("no-go card: %+v", cards)
	}
	// Anyone may comment: another account's close marker neither hides the card nor stops the gate closing it.
	g.edit(func() {
		g.g.Issues[889].Comments = append(g.g.Issues[889].Comments, &fake.Comment{ID: 7001, Body: askClosed(noGoAskID("MS1")) + "\nclosed"})
		g.g.Issues[889].Comments[len(g.g.Issues[889].Comments)-1].User.Login = "mallory"
	})
	if err := (&Env{}).missionAct(t.Context(), cards[0], InboxAction{N: 1, Command: "aayu ship 1"}); err == nil || !strings.Contains(err.Error(), `"aayu ship 1" is not a mission step`) {
		t.Fatalf("an option that is no mission step: %v", err)
	}
	if code, _, errOut := run(t, g.lane, g.opts, "mission", "gate", "889", "--repo", "o/r", "--once-more"); code != ExitBlocked ||
		!strings.Contains(errOut, "mission gate --once-more is a person's decision and is refused for an agent lane (AAYU_AGENT=1)") {
		t.Fatalf("an agent's once more: exit %d: %s", code, errOut)
	}
	g.vfail = nil
	g.eachRequestFails(func() (int, string, string) { return g.mission(g.person, "rahulnakmol", "gate", 889, "--once-more") })
	code, out, errOut = g.answer("1 y") // the card's first option: aayu mission gate 889 --once-more
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

// Criterion 8, the card's other option: "Stop the mission, with a reason"
// runs aayu mission stop (Story #1002) with the person's line. Their stop
// takes over the gate's, the card closes, and a pass of the gate no longer
// resumes the mission: only the person's resume does.
func TestGateStopOptionStopsTheMission(t *testing.T) {
	g := newGateWorld(t, "MS1=#20 MS2=#22", 20, 22)
	g.closeIssue(20)
	g.status(22, "ready", "")
	commitOnDev(g.g, ".aayudha/config.toml", "[mission]\nmax_no_go = 1\n")
	g.vfail = []string{"inbox.ship-once"}
	if rep := g.gateJSON(); rep.Milestones[0].State != GateStopped {
		t.Fatalf("first failed gate with max_no_go 1: %+v", rep.Milestones[0])
	}
	if code, out, errOut := g.answer("2 the gate keeps failing on the same check"); code != 0 || !strings.Contains(out, "stopped the mission of #889") {
		t.Fatalf("the card's stop: exit %d: %s%s", code, out, errOut)
	}
	p, _ := g.plan(889)
	if p.Pause == nil || p.Pause.By != "rahulnakmol" || p.Pause.Reason != "the gate keeps failing on the same check" || p.Controls[len(p.Controls)-1].Action != "stop" {
		t.Fatalf("plan after the card's stop: pause %+v, controls %+v", p.Pause, p.Controls)
	}
	l := inboxList{}
	if code, out, _ := run(t, g.agent, g.opts, "inbox", "--list", "--json", "--repo", "o/r"); code != 0 || json.Unmarshal([]byte(out), &l) != nil ||
		slices.ContainsFunc(l.Items, func(it InboxItem) bool { return it.Kind == InboxMission }) {
		t.Fatalf("the card after the stop: %s", out)
	}
	g.vfail = nil
	if code, out, errOut := g.mission(g.person, "rahulnakmol", "gate", 889, "--once-more"); code != 0 {
		t.Fatalf("once more after the stop: exit %d: %s%s", code, out, errOut)
	}
	if p, _ := g.plan(889); p.Milestones[0].Gate.State != GatePassed || p.Pause == nil || p.Pause.By != "rahulnakmol" {
		t.Fatalf("a pass lifted the person's stop: gate %+v, pause %+v", p.Milestones[0].Gate, p.Pause)
	}
	if code, out, errOut := g.mission(g.person, "rahulnakmol", "resume", 889); code != 0 || !strings.Contains(out, "start #22") {
		t.Fatalf("resume (it starts what can start): exit %d: %s%s", code, out, errOut)
	}
}

// [mission] max_no_go is dev's committed value: the embedded default (3)
// does not lower a committed 4, and a person's --set does.
func TestGateMaxNoGoIsDevs(t *testing.T) {
	g := newGateWorld(t, "MS1=#20 MS2=#22", 20, 22)
	g.closeIssue(20)
	commitOnDev(g.g, ".aayudha/config.toml", "[mission]\nmax_no_go = 4\n")
	g.vfail = []string{"inbox.ship-once"}
	var two strings.Builder
	for i := 1; i <= 2; i++ {
		b, _ := json.Marshal(GateRun{Milestone: "MS1", DevSHA: strings.Repeat(fmt.Sprint(i), 40), At: "2026-10-26T10:00:00Z", Result: gateFail,
			Checks: []GateCheck{{Check: "verify:inbox.ship-once", Result: gateFail}}})
		two.Write(append(b, '\n'))
	}
	third := func(sha string, args ...string) GateMSState {
		putLedger(t, g.bare, MissionGatesPath(889), two.String())
		g.setPlan(889, func(p *MissionPlan) { p.Milestones[0].Gate = MilestoneGate{State: GateWaits} })
		g.tip(sha, "success")
		return g.gateJSON(args...).Milestones[0]
	}
	if ms := third(strings.Repeat("a", 40)); ms.State != GateFailed || !strings.Contains(ms.Detail, "(3 of 4 failed runs before the mission stops)") {
		t.Fatalf("third failure with a committed max_no_go of 4: %+v", ms)
	}
	if ms := third(strings.Repeat("b", 40), "--set", "mission.max_no_go=3"); ms.State != GateStopped {
		t.Fatalf("third failure with --set mission.max_no_go=3: %+v", ms)
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
	// A required check that still runs: the gate waits, runs nothing else, and records nothing.
	g.tip(strings.Repeat("b", 40), "")
	g.edit(func() { g.g.CheckRuns[g.sha][0].Status = "in_progress" })
	if rep := g.gateJSON(); rep.Milestones[0].Detail != "pending: build is in_progress on bbbbbbb" || g.calls != 0 || len(g.runs()) != 0 {
		t.Fatalf("gate with build running: %+v", rep.Milestones[0])
	}
	g.tip(strings.Repeat("c", 40), "cancelled")
	g.eachRequestFails(func() (int, string, string) { return g.gate() })
	reruns = nil
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
	// A token that cannot read Actions passes when no cancelled run needs the
	// read: protected-paths, which the gate does not judge, and a check no rule
	// requires are cancelled; a lint run from an app the rule does not pin is too.
	g.edit(func() {
		g.g.CheckRuns[g.sha][0].ID, g.g.CheckRuns[g.sha][0].Conclusion = 74, "success"
		g.g.CheckRuns[g.sha][2].Conclusion = "cancelled"
		g.g.CheckRuns[g.sha] = append(g.g.CheckRuns[g.sha], fake.CheckRun{ID: 75, Name: "screen-reader", Status: "completed", Conclusion: "cancelled", Suite: &fake.Suite{ID: 3}},
			fake.CheckRun{ID: 76, Name: "lint", Status: "completed", Conclusion: "cancelled", App: &fake.App{ID: 999}, Suite: &fake.Suite{ID: 4}})
		g.g.RequiredChecks = []string{"build", "lint@0", "protected-paths"}
		g.g.Hook = func(method, path string) { // every Actions read fails, as for a token without Actions read
			if path == "/repos/o/r/actions/runs" {
				g.g.Fail, g.g.FailAfter = method+" "+path, 0
			}
		}
	})
	if rep := g.gateJSON(); rep.Milestones[0].State != GatePassed {
		t.Fatalf("pass after the rerun, with no Actions read: %+v", rep.Milestones[0])
	}
	g.edit(func() { g.g.Hook, g.g.Fail, g.g.RequiredChecks = nil, "", []string{"build", "lint", "protected-paths"} })
	// A failure whose log names no 5xx is the code's: no rerun, and the gate fails.
	g.setPlan(889, func(p *MissionPlan) { p.Milestones[0].Gate = MilestoneGate{State: GateWaits} })
	g.tip(strings.Repeat("e", 40), "failure")
	g.edit(func() { g.g.JobLogs = map[int64]string{70: "--- FAIL: TestShip (0.01s)\n"} })
	if rep := g.gateJSON(); len(reruns) != 2 || rep.Milestones[0].State != GateFailed ||
		rep.Milestones[0].Checks[0] != (GateCheck{Check: "check:build", Result: gateFail, Detail: "build is failure on eeeeeee"}) {
		t.Fatalf("a code failure: reruns %v: %+v", reruns, rep.Milestones[0])
	}
}

// Criterion 1, the default eval runner: the evals of the touched seats are
// read from evals.yml's results for the dev tip, against evals/eval.toml's
// bars at that tip. A gating group with no run is dispatched once on dev and
// the gate waits; a run in progress waits; a case below its bar fails.
func TestGateEvalsReadEvalsResults(t *testing.T) {
	g := newGateWorld(t, "MS1=#20 MS2=#22", 20, 22)
	g.opts.GateEvals = nil
	g.edit(func() {
		is := g.g.Issues[20] // L03: the Engineering Manager's layer, whose seat owns the mission skill
		i := slices.IndexFunc(is.Labels, func(l fake.Label) bool { return l.Name == "layer:L02" })
		is.Labels[i] = fake.Label{Name: "layer:L03"}
		g.g.RefFiles[devTip]["evals/eval.toml"] = "min_pass_k = 0.8\nmin_mean = 3.5\n\n[[groups]]\nname = \"missions\"\nprefixes = [\"mission/\"]\n\n" +
			"[[groups]]\nname = \"plans\"\nprefixes = [\"plan/\"]\n\n[[groups]]\nname = \"inspect\"\nprefixes = [\"inspect/\"]\ngate = \"advisory\"\n"
	})
	g.closeIssue(20)
	g.eachRequestFails(func() (int, string, string) { return g.gate() })
	g.edit(func() { g.g.Dispatches = nil })
	rep := g.gateJSON()
	if rep.Milestones[0].State != GateWaits || len(g.g.Dispatches) != 1 || g.g.Dispatches[0]["workflow"] != "evals.yml" || g.g.Dispatches[0]["ref"] != "dev" ||
		!strings.Contains(rep.Milestones[0].Detail, "pending: dispatched evals.yml on dev for ^(") {
		t.Fatalf("gate with no eval results: %+v, dispatches %v", rep.Milestones[0], g.g.Dispatches)
	}
	if in, _ := g.g.Dispatches[0]["inputs"].(map[string]any); in["group"] != "missions" || !strings.Contains(in["cases"].(string), "|mission|") {
		t.Fatalf("dispatch inputs: %v", g.g.Dispatches[0])
	}
	run := fake.WorkflowRun{ID: 501, Path: ".github/workflows/evals.yml", HeadSHA: g.sha, Event: "workflow_dispatch", Status: "in_progress", HTMLURL: "https://github.com/o/r/actions/runs/501"}
	g.edit(func() { g.g.Runs = append(g.g.Runs, run) })
	if rep := g.gateJSON(); rep.Milestones[0].Detail != "pending: evals.yml run https://github.com/o/r/actions/runs/501 is in_progress" || len(g.g.Dispatches) != 1 {
		t.Fatalf("gate with the run in progress: %+v", rep.Milestones[0])
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, _ := zw.Create(fmt.Sprintf("%s-missions.json", g.sha))
	_, _ = f.Write([]byte(`{"cases": [{"case": "mission/stays-in-list", "skill": "mission", "pass_k": 1, "mean": 3.1},
		{"case": "mission/next-order", "skill": "mission", "pass_k": 1, "mean": 4.6}, {"case": "mission/flaky", "skill": "mission", "not_run": "pin"}]}`))
	_ = zw.Close()
	g.edit(func() {
		g.g.Runs[len(g.g.Runs)-1].Status, g.g.Runs[len(g.g.Runs)-1].Conclusion = "completed", "success"
		g.g.Artifacts = map[int64][]fake.ArtifactZip{501: {{ID: 601, Name: "eval-results-" + g.sha + "-missions", Zip: buf.Bytes()}}}
	})
	g.eachRequestFails(func() (int, string, string) { return g.gate() })
	rep = g.gateJSON()
	var evals []string
	for _, ck := range rep.Milestones[0].Checks {
		if strings.HasPrefix(ck.Check, "eval:") {
			evals = append(evals, ck.Check+"="+ck.Result+" ("+ck.Detail+")")
		}
	}
	want := []string{"eval:mission/stays-in-list=failed (mean 3.10, pass^k 1.00 (bars: mean 3.50, pass^k 0.80))", "eval:mission/next-order=passed (mean 4.60, pass^k 1.00)"}
	if rep.Milestones[0].State != GateFailed || !slices.Equal(evals, want) || len(g.g.Dispatches) != 1 {
		t.Fatalf("gate with the results: %+v\n got %v\nwant %v", rep.Milestones[0], evals, want)
	}
	// On a new tip, a dispatched run that finished with no results fails its group.
	again := func(sha string) GateMSState {
		g.setPlan(889, func(p *MissionPlan) { p.Milestones[0].Gate = MilestoneGate{State: GateWaits} })
		putLedger(t, g.bare, MissionGatesPath(889), "")
		g.tip(sha, "success")
		return g.gateJSON().Milestones[0]
	}
	g.edit(func() {
		g.g.Runs = append(g.g.Runs, fake.WorkflowRun{ID: 502, Path: ".github/workflows/evals.yml", HeadSHA: strings.Repeat("b", 40), Event: "workflow_dispatch", Status: "completed", Conclusion: "failure"})
	})
	if ms := again(strings.Repeat("b", 40)); ms.State != GateFailed || !slices.Contains(ms.Checks, GateCheck{Check: "eval:missions", Result: gateFail, Detail: "its dispatched evals.yml run left no results for bbbbbbb"}) {
		t.Fatalf("gate after a dispatched run with no results: %+v", ms)
	}
	// Results that judge no case (each one not run) prove nothing: the group fails.
	var empty bytes.Buffer
	ez := zip.NewWriter(&empty)
	ef, _ := ez.Create("results.json")
	_, _ = ef.Write([]byte(`{"cases": [{"case": "mission/flaky", "skill": "mission", "not_run": "pin"}]}`))
	_ = ez.Close()
	g.edit(func() {
		g.g.Runs = append(g.g.Runs, fake.WorkflowRun{ID: 503, Path: ".github/workflows/evals.yml", HeadSHA: strings.Repeat("c", 40), Event: "push", Status: "completed", Conclusion: "success"})
		g.g.Artifacts[503] = []fake.ArtifactZip{{ID: 603, Name: "eval-results-" + strings.Repeat("c", 40) + "-missions", Zip: empty.Bytes()}}
	})
	if ms := again(strings.Repeat("c", 40)); ms.State != GateFailed || !slices.Contains(ms.Checks, GateCheck{Check: "eval:missions", Result: gateFail, Detail: "its results for ccccccc judge no case of the touched seats' skills"}) {
		t.Fatalf("gate on results that judge no case: %+v", ms)
	}
	// A tip with no evals/eval.toml has no gating group: the evals pass.
	g.edit(func() {
		files := maps.Clone(g.g.RefFiles[devTip])
		delete(files, "evals/eval.toml")
		g.g.RefFiles[strings.Repeat("f", 40)] = files
	})
	if ms := again(strings.Repeat("f", 40)); ms.State != GatePassed ||
		!slices.Contains(ms.Checks, GateCheck{Check: "eval:engineering-manager", Result: gatePass, Detail: "no gating eval group covers the skills of these seats"}) {
		t.Fatalf("gate on a tip with no evals/eval.toml: %+v", ms)
	}
}

// Criterion 1, the default verify runner: `aayu verify --check all` runs on
// a worktree of the dev tip, each behaviour check is one gate check, and the
// worktree is gone afterwards.
func TestGateVerifyRunsTheSweepOnTheTip(t *testing.T) {
	g := newGateWorld(t, "MS1=#20 MS2=#22", 20, 22)
	g.opts.GateVerify = nil
	g.agent["PATH"] = os.Getenv("PATH")
	// A tip with no feature map has no behaviour check: verify --check all is one gate check.
	g.closeIssue(20)
	g.tip(git(t, g.dir, "rev-parse", "HEAD"), "success")
	if ms := g.gateJSON().Milestones[0]; ms.State != GatePassed || !slices.Contains(ms.Checks, GateCheck{Check: "verify:all", Result: gatePass, Detail: "no behaviour check failed"}) {
		t.Fatalf("gate on a tip with no feature map: %+v", ms)
	}
	g.setPlan(889, func(p *MissionPlan) { p.Milestones[0].Gate = MilestoneGate{State: GateWaits} })
	putLedger(t, g.bare, MissionGatesPath(889), "")
	sk := filepath.Join(g.dir, ".agents", "skills", "verify-app")
	_ = os.MkdirAll(filepath.Join(sk, "features"), 0o755)
	_ = os.WriteFile(filepath.Join(g.dir, "app.sh"), []byte("case $1 in hello) echo hi;; esac # cmd hello\n"), 0o644)
	_ = os.WriteFile(filepath.Join(sk, "control.sh"), []byte("cat >/dev/null\necho '{\"status\": \"failed\", \"reason\": \"printed nothing\", \"artifacts\": []}'\n"), 0o644)
	_ = os.WriteFile(filepath.Join(sk, "features", "README.md"), []byte("# app\n\n```map\n{\"product\": \"app\", \"control\": [\"sh\", \".agents/skills/verify-app/control.sh\"], \"surfaces\": [{\"kind\": \"command\", \"files\": [\"app.sh\"], \"pattern\": \"cmd ([a-z]+)\"}]}\n```\n\n1. [Hello](hello.md)\n"), 0o644)
	_ = os.WriteFile(filepath.Join(sk, "features", "hello.md"), []byte("# Hello\n\nSays hi.\n\n```feature\n{\"id\": \"hello\", \"sources\": [\"app.sh\"], \"surfaces\": {\"command\": [\"hello\"]}, \"verified_at\": \"\"}\n```\n\n## Sub-features\n\n## How to reach it\n\n## Behaviour checks\n\n```check\n{\"id\": \"hi\", \"kind\": \"cli\", \"evidence\": \"api-trace\", \"steps\": [{\"run\": [\"hello\"], \"stdout\": \"hi\"}]}\n```\n\n## Gotchas\n"), 0o644)
	git(t, g.dir, "add", "-A")
	git(t, g.dir, "commit", "--quiet", "-m", "app")
	git(t, g.dir, "update-ref", "refs/remotes/origin/dev", "HEAD")
	g.tip(git(t, g.dir, "rev-parse", "HEAD"), "success")
	rep := g.gateJSON()
	i := slices.IndexFunc(rep.Milestones[0].Checks, func(ck GateCheck) bool { return strings.HasPrefix(ck.Check, "verify:") })
	if rep.Milestones[0].State != GateFailed || i < 0 || rep.Milestones[0].Checks[i] != (GateCheck{Check: "verify:hello.hi", Result: gateFail, Detail: "failed printed nothing"}) {
		t.Fatalf("gate with a failing behaviour check: %+v", rep.Milestones[0])
	}
	if wts := git(t, g.dir, "worktree", "list"); strings.Count(wts, "\n") != 0 {
		t.Fatalf("the gate left a worktree:\n%s", wts)
	}
}

// Bug #1100: the merge gate (verify --ci) on a gate fix PR that says
// `Refs #889`. A PR the plan's gate_fixes lists rests on #889's approved
// record, not a Ready snapshot: it passes when the check it fixes passes at
// its head, and fails when that check still fails or #889 was approved
// again. An unlisted PR fails as before. The pack job (proof comment) checks
// the pack against the same record.
func TestMergeGateJudgesAListedGateFix(t *testing.T) {
	g := newGateWorld(t, "MS1=#20 MS2=#22", 20, 22)
	g.closeIssue(20)
	g.vfail = []string{"inbox.ship-once"}
	g.gateJSON()
	dir := g.dir
	base := git(t, dir, "rev-parse", "HEAD")
	git(t, dir, "checkout", "--quiet", "-b", "fix/gate-981")
	// inbox.ship-once passes only when the checkout has the fix.
	fdir := filepath.Join(dir, ".agents", "skills", "verify-app", "features")
	_ = os.MkdirAll(fdir, 0o755)
	_ = os.WriteFile(filepath.Join(fdir, "README.md"), []byte("# app\n\n```map\n{\"product\": \"app\", \"control\": [\"sh\", \"control.sh\"]}\n```\n\n1. [Inbox](inbox.md)\n"), 0o644)
	_ = os.WriteFile(filepath.Join(fdir, "inbox.md"), []byte("# Inbox\n\nx\n\n```feature\n{\"id\": \"inbox\", \"sources\": [\"inbox.txt\"], \"verified_at\": \"\"}\n```\n\n## Sub-features\n\n## How to reach it\n\n## Behaviour checks\n\n```check\n{\"id\": \"ship-once\", \"kind\": \"cli\", \"evidence\": \"api-trace\", \"steps\": [{\"run\": [\"cat\"]}]}\n```\n\n## Gotchas\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "control.sh"), []byte("cat >/dev/null\nif [ -f unreachable ]; then echo '{\"status\": \"unreachable\", \"reason\": \"no inbox to read\"}'; elif [ -f fixed ]; then echo '{\"status\": \"passed\"}'; else echo '{\"status\": \"failed\", \"reason\": \"two ship records\"}'; fi\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "fixed"), []byte("x\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "cov.info"), []byte("SF:x\nend_of_record\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, ".aayudha", "config.toml"), []byte("version = 1\n[verify.floors.coverage]\ncommand = [\"true\"]\nreport = \"cov.info\"\n"), 0o644)
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "--quiet", "-m", "fix(gate): ship once")
	g.fixPR(981, "verify:inbox.ship-once", "Refs #889")
	g.fixPR(982, "verify:inbox.ship-once", "Refs #889")
	head := func() string {
		sha := git(t, dir, "rev-parse", "HEAD")
		g.edit(func() {
			for _, n := range []int{981, 982} {
				g.g.Pulls[n].Head.SHA, g.g.Pulls[n].Base.SHA = sha, base
			}
		})
		return sha
	}
	head()
	if code, out, errOut := g.gate("--pr", "981"); code != 0 {
		t.Fatalf("gate --pr 981: exit %d: %s%s", code, out, errOut)
	}
	fix := g.plan2(889).GateFixes[0]
	ci := map[string]string{"GITHUB_TOKEN": "workflow-token", "PATH": os.Getenv("PATH"), "XDG_STATE_HOME": g.person["XDG_STATE_HOME"]}
	type gateResult struct { // verify --json, as the merge gate workflow reads it
		Head        string   `json:"head"`
		Gate        []string `json:"gate_problems"`
		GateFix     *GateFix `json:"gate_fix"`
		GateFixNote string   `json:"gate_fix_note"`
	}
	verifyPR := func(n int) (int, gateResult, string) {
		t.Helper()
		code, out, errOut := run(t, ci, g.opts, "verify", fmt.Sprint(n), "--floor", "coverage", "--ci", "--repo", "o/r", "--json")
		var res gateResult
		_ = json.NewDecoder(strings.NewReader(out)).Decode(&res)
		return code, res, out + errOut
	}

	// 1. Listed, under the current approval, and the check passes: no Ready snapshot problem; the pack rests on the approved record.
	code, res, out := verifyPR(981)
	if code != 0 || len(res.Gate) != 0 || res.GateFix == nil || res.GateFix.PR != 981 || strings.Contains(out, "Ready snapshot") ||
		res.GateFixNote != fmt.Sprintf("#981 fixes verify:inbox.ship-once in MS1 of #889, and that check passes at %s", short(res.Head)) {
		t.Fatalf("listed fix, check passes: exit %d %+v: %s", code, res.Gate, out)
	}
	p, err := proof.LoadLocal(proof.LocalDir(dir, 889))
	if err != nil || p.PR != 981 || p.PRHeadSHA != res.Head || p.CompletionGoalHash != fix.Approval.SHA256 || p.Links.ReadySnapshot != ready.ApprovalPath(889) {
		t.Fatalf("fix pack: %+v %v", p, err)
	}
	if code, out, errOut := run(t, ci, g.opts, "verify", "981", "--floor", "coverage", "--ci", "--repo", "o/r"); code != 0 ||
		!strings.Contains(out, "\n  gate fix: #981 fixes verify:inbox.ship-once in MS1 of #889, and that check passes at "+short(res.Head)+"\n") {
		t.Fatalf("text output of a listed fix: exit %d: %s%s", code, out, errOut)
	}
	gateEnv := map[string]string{"GITHUB_TOKEN": "workflow-token", "GITHUB_WORKFLOW_REF": "o/r/.github/workflows/aayudha-merge-gate.yml@refs/pull/981/merge",
		"GITHUB_EVENT_NAME": "pull_request_target", "XDG_STATE_HOME": g.person["XDG_STATE_HOME"]}
	if code, out, errOut := run(t, gateEnv, g.opts, "proof", "comment", "889", "--ci", "--repo", "o/r"); code != 0 || strings.Contains(out+errOut, "Ready snapshot") {
		t.Fatalf("pack job on the fix: exit %d: %s%s", code, out, errOut)
	}

	// 2. The same PR, and the check still fails at the head.
	git(t, dir, "rm", "--quiet", "fixed")
	git(t, dir, "commit", "--quiet", "-m", "fix(gate): not yet")
	sha := head()
	if code, res, out := verifyPR(981); code != ExitBlocked ||
		!slices.Equal(res.Gate, []string{"#981 fixes verify:inbox.ship-once in MS1 of #889, and that check still fails at " + sha[:7]}) {
		t.Fatalf("listed fix, check fails: exit %d %q: %s", code, res.Gate, out)
	}

	// A check that could not reach its precondition proves nothing: the fix does not pass on it.
	_ = os.WriteFile(filepath.Join(dir, "unreachable"), []byte("x\n"), 0o644)
	git(t, dir, "add", "unreachable")
	git(t, dir, "commit", "--quiet", "-m", "fix(gate): no inbox")
	sha = head()
	if code, res, out := verifyPR(981); code != ExitBlocked || len(res.Gate) != 1 ||
		!strings.HasPrefix(res.Gate[0], "#981 fixes verify:inbox.ship-once in MS1 of #889, and that check did not run at "+sha[:7]+": ") {
		t.Fatalf("listed fix, check unreachable: exit %d %q: %s", code, res.Gate, out)
	}
	git(t, dir, "rm", "--quiet", "unreachable")
	git(t, dir, "commit", "--quiet", "-m", "fix(gate): inbox back")
	head()

	// 3. An unlisted PR that refers to the Feature fails as before.
	if code, res, out := verifyPR(982); code != ExitBlocked || res.GateFix != nil || !slices.Equal(res.Gate, []string{"story #889 has no Ready snapshot on the ledger"}) {
		t.Fatalf("unlisted PR: exit %d %q: %s", code, res.Gate, out)
	}

	// A required CI job or an eval: the gate cannot run it, names it, and ship's required checks decide.
	_ = os.WriteFile(filepath.Join(dir, "fixed"), []byte("x\n"), 0o644)
	git(t, dir, "add", "fixed")
	git(t, dir, "commit", "--quiet", "-m", "fix(gate): ship once again")
	g.fixPR(983, "check:build", "Refs #889")
	head()
	g.edit(func() { g.g.Pulls[983].Head.SHA, g.g.Pulls[983].Base.SHA = g.g.Pulls[981].Head.SHA, base })
	g.setPlan(889, func(p *MissionPlan) {
		p.GateFixes = append(p.GateFixes, GateFix{PR: 983, Milestone: "MS1", Check: "check:build", DevSHA: fix.DevSHA, Approval: fix.Approval, At: fix.At})
	})
	if code, res, out := verifyPR(983); code != 0 || len(res.Gate) != 0 ||
		res.GateFixNote != "#983 fixes check:build in MS1 of #889: the merge gate does not run the required check build; ship's required checks decide" {
		t.Fatalf("listed fix of a CI job: exit %d %q %q: %s", code, res.Gate, res.GateFixNote, out)
	}
	g.fixPR(984, "eval:mission/stays-in-list", "Refs #889")
	g.edit(func() { g.g.Pulls[984].Head.SHA, g.g.Pulls[984].Base.SHA = g.g.Pulls[981].Head.SHA, base })
	g.setPlan(889, func(p *MissionPlan) {
		p.GateFixes = append(p.GateFixes, GateFix{PR: 984, Milestone: "MS1", Check: "eval:mission/stays-in-list", DevSHA: fix.DevSHA, Approval: fix.Approval, At: fix.At})
	})
	if code, res, out := verifyPR(984); code != 0 || len(res.Gate) != 0 || !strings.HasPrefix(res.GateFixNote,
		"#984 fixes eval:mission/stays-in-list in MS1 of #889: the merge gate does not run the eval mission/stays-in-list (evals.yml), and no required check runs it") {
		t.Fatalf("listed fix of an eval: exit %d %q %q: %s", code, res.Gate, res.GateFixNote, out)
	}

	// 4. #889 is approved again: the entry was made under the earlier approval (read from the ledger, never the PR body).
	record := git(t, g.bare, "show", "ledger:"+ready.ApprovalPath(889))
	again := strings.Replace(record, `"version": 1`, `"version": 2`, 1)
	if again == record {
		t.Fatalf("approved record has no version 1:\n%s", record)
	}
	putLedger(t, g.bare, ready.ApprovalPath(889), again+"\n")
	if code, res, out := verifyPR(981); code != ExitBlocked || !slices.Equal(res.Gate, []string{"the gate fix entry of #981 was made under an earlier approval of #889"}) {
		t.Fatalf("earlier approval: exit %d %q: %s", code, res.Gate, out)
	}
	if code, out, errOut := run(t, gateEnv, g.opts, "proof", "comment", "889", "--ci", "--repo", "o/r"); code != ExitBlocked ||
		!strings.Contains(errOut, "the gate fix entry of #981 was made under an earlier approval of #889") {
		t.Fatalf("pack job under an earlier approval: exit %d: %s%s", code, out, errOut)
	}
}

// Bug #1100, inspect 1746d7cf34: a gate fix passes only on a result for the
// check it names. A result for another check, or one that did not pass,
// proves nothing about it.
func TestGateFixPassedNeedsTheNamedCheck(t *testing.T) {
	ok := func(ref string) verify.CheckResult { return verify.CheckResult{Ref: ref, Status: verify.CheckPassed} }
	for _, c := range []struct {
		ref     string
		results []verify.CheckResult
		want    bool
	}{
		{"inbox.ship-once", []verify.CheckResult{ok("inbox.ship-once")}, true},
		{"inbox.ship-once", []verify.CheckResult{ok("mission.next-order")}, false},
		{"inbox.ship-once", nil, false},
		{"inbox.ship-once", []verify.CheckResult{ok("inbox.ship-once"), {Ref: "inbox.read", Status: verify.CheckUnreachable}}, false},
		{"inbox", []verify.CheckResult{ok("inbox.ship-once"), ok("inbox.read")}, true},
		{"inbox", []verify.CheckResult{ok("inboxes.read")}, false},
		{verify.SelectAll, []verify.CheckResult{ok("mission.next-order")}, true},
	} {
		if got := gateFixPassed(c.ref, c.results); got != c.want {
			t.Errorf("gateFixPassed(%q, %+v) = %t, want %t", c.ref, c.results, got, c.want)
		}
	}
}

// putPack lands a proof pack of story n's PR pr on the ledger, of kind
// (authoritative or advisory), as ship and proof push do.
func (g *gateWorld) putPack(n, pr int, kind string) {
	g.t.Helper()
	head := fmt.Sprintf("%040x", pr)
	b, err := proof.Encode(&proof.Pack{Repo: "o/r", Story: n, PR: pr, PRHeadSHA: head, Seat: "ai-native-engineer", Host: "cli", Identity: "ci",
		Authority: proof.Authority{Kind: kind}, CompletionGoalHash: strings.Repeat("ab", 32), CreatedAt: "2026-11-01T10:00:00Z"})
	if err != nil {
		g.t.Fatal(err)
	}
	putLedger(g.t, g.bare, proof.Dir(n, head, kind)+"/pack.json", string(b))
}

// Story #1174, criterion 4: Bugs #30 and #31 merged, and #31's PR has no
// authoritative pack on the ledger (an advisory one, and one of another PR,
// do not count): the BUGS gate fails with "#31 merged with no proof pack",
// and its fix waits for a person, never a fix PR. Once the pack lands, the
// gate runs again on the same dev tip and passes.
func TestGateBugsNeedProofPack(t *testing.T) {
	g := newBugsGateWorld(t, "", []int{20, 21}, []int{30, 31})
	for _, n := range []int{20, 21} {
		g.closeIssue(n)
	}
	for n, pr := range map[int]int{30: 130, 31: 131} {
		g.status(n, "merged", "") // ship leaves a Bug open until its release (Bug #1169)
		putLedger(t, g.bare, delegate.LedgerPath(n), fmt.Sprintf(`{"story": %d, "receiver": "amp", "mode": "cloud", "family": "anthropic", "pr": %d, "status": "merge-ready", "brief_sha256": "", "started_at": "2026-11-01T09:00:00Z"}`, n, pr))
	}
	g.putPack(30, 130, proof.Authoritative)
	g.putPack(31, 131, proof.Advisory)
	g.putPack(31, 99, proof.Authoritative)
	rep := g.gateJSON()
	if len(rep.Milestones) != 2 || rep.Milestones[0].State != GatePassed || rep.Milestones[1].Name != BugsMilestone || rep.Milestones[1].State != GateFailed {
		t.Fatalf("gate: %+v", rep.Milestones)
	}
	var checks []string
	for _, ck := range rep.Milestones[1].Checks {
		if strings.HasPrefix(ck.Check, "proof:") {
			checks = append(checks, ck.Check+"="+ck.Result+": "+ck.Detail)
		}
	}
	if want := []string{"proof:#30=passed: " + proof.Dir(30, fmt.Sprintf("%040x", 130), proof.Authoritative) + "/pack.json", "proof:#31=failed: #31 merged with no proof pack"}; !slices.Equal(checks, want) {
		t.Fatalf("BUGS proof checks: %q, want %q", checks, want)
	}
	if len(rep.Fixes) != 1 || rep.Fixes[0].Check != "proof:#31" || rep.Fixes[0].Action != "waits" || rep.Fixes[0].Brief != "" ||
		!strings.Contains(rep.Fixes[0].Reason, "aayu proof land") {
		t.Fatalf("fixes: %+v", rep.Fixes)
	}
	if p, _ := g.plan(889); p.Milestones[1].Gate.State != GateFailed {
		t.Fatalf("BUGS gate in the plan: %+v", p.Milestones[1].Gate)
	}
	// A delegation record aayu cannot read names no PR: the gate fails closed, and records no run.
	runs := len(g.runs())
	putLedger(t, g.bare, delegate.LedgerPath(31), "not json")
	if code, out, errOut := g.gate(); code != ExitExternal || !strings.Contains(errOut, "ledger delegate/31/state.json: not a delegation aayu wrote") || len(g.runs()) != runs {
		t.Fatalf("gate with #31's delegation unreadable: exit %d: %s%s", code, out, errOut)
	}
	putLedger(t, g.bare, delegate.LedgerPath(31), `{"story": 31, "receiver": "amp", "mode": "cloud", "family": "anthropic", "pr": 131, "status": "merge-ready", "brief_sha256": "", "started_at": "2026-11-01T09:00:00Z"}`)
	g.putPack(31, 131, proof.Authoritative)
	if rep := g.gateJSON(); rep.Milestones[1].State != GatePassed {
		t.Fatalf("gate after #31's pack landed: %+v", rep.Milestones)
	}
	if p, _ := g.plan(889); p.Milestones[1].Gate.State != GatePassed || p.Milestones[1].Gate.DevSHA != g.sha {
		t.Fatalf("BUGS gate in the plan: %+v", p.Milestones[1].Gate)
	}
}
