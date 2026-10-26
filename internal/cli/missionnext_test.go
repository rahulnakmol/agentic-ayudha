package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// Story #998: `aayu mission next <f>` on the missionWorld of Story #997. The
// Kanha runs in Amp (m.agent), and each mission is started on amp.

// startedMission adds Feature f with stories (Bugs when bug is set), in
// sub-issue order, approves it, and starts its mission on amp.
func (m *missionWorld) startedMission(f int, stories []int, bugs ...int) {
	m.t.Helper()
	m.issue(f, fmt.Sprintf("Feature %d", f), "feature", 0, "")
	for _, n := range stories {
		typ := "story"
		if slices.Contains(bugs, n) {
			typ = "bug"
		}
		m.issue(n, fmt.Sprintf("Item %d", n), typ, f, "")
	}
	m.feature(f, stories...)
	m.mu.Lock()
	m.tool = "amp"
	m.mu.Unlock()
	if code, out, errOut := m.approve(f); code != 0 {
		m.t.Fatalf("approve %d: exit %d: %s%s", f, code, out, errOut)
	}
	if code, out, errOut := m.mission(m.person, "rahulnakmol", "start", f); code != 0 {
		m.t.Fatalf("start %d: exit %d: %s%s", f, code, out, errOut)
	}
}

// status sets issue n's status label, and its priority when prio is set.
func (m *missionWorld) status(n int, status, prio string) {
	m.edit(func() {
		is := m.g.Issues[n]
		is.Labels = slices.DeleteFunc(is.Labels, func(l fake.Label) bool {
			return strings.HasPrefix(l.Name, "status:") || prio != "" && strings.HasPrefix(l.Name, "priority:")
		})
		is.Labels = append(is.Labels, fake.Label{Name: "status:" + status})
		if prio != "" {
			is.Labels = append(is.Labels, fake.Label{Name: "priority:" + prio})
		}
	})
}

// readyAt posts issue n's Ready snapshot comment at a time.
func (m *missionWorld) readyAt(n int, at time.Time) {
	m.edit(func() {
		m.g.Issues[n].Comments = append(m.g.Issues[n].Comments, &fake.Comment{ID: int64(9000 + n), Body: ready.SnapshotMarker + "\nReady", CreatedAt: at})
	})
}

func (m *missionWorld) setPlan(f int, edit func(*MissionPlan)) {
	m.t.Helper()
	p, _ := m.plan(f)
	edit(&p)
	data, _ := json.MarshalIndent(p, "", "  ")
	putLedger(m.t, m.bare, MissionPlanPath(f), string(data))
}

// next runs aayu mission next f as the Kanha in Amp.
func (m *missionWorld) next(f int, args ...string) (int, string, string) {
	m.t.Helper()
	return run(m.t, m.agent, m.opts, append([]string{"mission", "next", fmt.Sprint(f), "--repo", "o/r"}, args...)...)
}

// claims are the live, unreleased claim lines of f's mission, by story.
func (m *missionWorld) claims(f int) map[int]int {
	raw, _ := exec.Command("git", "-C", m.bare, "show", "ledger:"+MissionClaimsPath(f)).Output() // none yet: no claims
	released, out := map[string]bool{}, map[int]int{}
	var lines []loopClaim
	for _, l := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var c loopClaim
		if json.Unmarshal([]byte(l), &c) == nil {
			if c.Release {
				released[c.Token] = true
			}
			lines = append(lines, c)
		}
	}
	for _, c := range lines {
		if !c.Release && !released[c.Token] {
			out[c.Story]++
		}
	}
	return out
}

// Criterion 1: with 2 agents and none running, two P1 stories start in slice
// order (sub-issue order, not number or Ready time), and the P2 story, first
// in slice order, waits for a free agent.
func TestMissionNextOrderAndAgents(t *testing.T) {
	m := newMissionWorld(t)
	m.startedMission(889, []int{22, 20, 21})
	m.setPlan(889, func(p *MissionPlan) { p.Agents = 2 })
	m.status(22, "ready", "P2")
	for _, n := range []int{20, 21} {
		m.status(n, "ready", "")
	}
	m.readyAt(21, time.Now().Add(-2*time.Hour)) // Ready before #20, but slice 2
	m.readyAt(20, time.Now().Add(-time.Hour))
	n := m.commits()
	code, out, errOut := m.next(889)
	if want := "start #20\nstart #21\nwaits #22: no free agent (2 of 2 busy after these starts)\n"; code != 0 || out != want {
		t.Fatalf("mission next 889: exit %d, want %q: %q%s", code, want, out, errOut)
	}
	if got := m.claims(889); got[20] != 1 || got[21] != 1 || len(got) != 2 || m.commits() == n {
		t.Fatalf("claims after the list: %v", got)
	}
	// The next run sees both claimed and every agent busy: nothing new starts.
	if code, out, errOut := m.next(889); code != 0 || strings.Contains(out, "start ") ||
		!strings.Contains(out, "waits #20: claimed by another run\nwaits #21: claimed by another run\nwaits #22: no free agent (2 of 2 busy after these starts)") {
		t.Fatalf("second run: exit %d: %s%s", code, out, errOut)
	}
}

// Criterion 2: a blocked story and one whose host waits out a usage limit
// wait, and neither starts, even with agents free.
func TestMissionNextWaitReasons(t *testing.T) {
	m := newMissionWorld(t)
	m.startedMission(889, []int{23, 24})
	m.status(23, "blocked", "")
	m.status(24, "in-progress", "")
	until := time.Date(2026, 10, 26, 20, 10, 0, 0, time.Local)
	putLedger(t, m.bare, "delegate/24/state.json", fmt.Sprintf(`{"story": 24, "receiver": "amp", "mode": "local", "task": "T-sub-24", "status": "paused",
		"events": [{"at": "2026-10-26T13:00:00Z", "status": "paused", "detail": "Amp's subscription usage limit", "until": %q}]}`, until.UTC().Format(time.RFC3339)))
	m.opts.Now = func() time.Time { return until.Add(-time.Hour) }
	code, out, errOut := m.next(889)
	if code != 0 || out != "waits #23: blocked\nwaits #24: amp paused until 20:10 · subthread T-sub-24\n" {
		t.Fatalf("mission next 889: exit %d: %q%s", code, out, errOut)
	}
	// Once the limit resets, the story is running again: not listed.
	m.opts.Now = func() time.Time { return until.Add(time.Minute) }
	if code, out, _ := m.next(889); code != 0 || out != "waits #23: blocked\n" || len(m.claims(889)) != 0 {
		t.Fatalf("after the reset: exit %d: %q", code, out)
	}
}

// Criterion 3: a Ready story of another Feature is never listed.
func TestMissionNextOtherFeatureNeverListed(t *testing.T) {
	m := newMissionWorld(t)
	m.issue(888, "Other Feature", "feature", 0, "")
	m.issue(30, "Story 30", "story", 888, "")
	m.startedMission(889, []int{20})
	m.status(30, "ready", "P0")
	m.status(20, "ready", "")
	code, out, errOut := m.next(889, "--json")
	var list WorkList
	if code != 0 || json.Unmarshal([]byte(out), &list) != nil || strings.Contains(out, "#30") || len(list.Items) != 1 || list.Items[0].Key != "#20" {
		t.Fatalf("mission next 889: exit %d: %s%s", code, out, errOut)
	}
}

// Criterion 4: two runs ask at the same time with one free agent. The other
// run's claim of #20 lands while this one writes its own: this run loses and
// releases its line, so #20 starts once.
func TestMissionNextOneClaimTwoRuns(t *testing.T) {
	m := newMissionWorld(t)
	m.startedMission(889, []int{20, 21})
	m.setPlan(889, func(p *MissionPlan) { p.Agents = 1 })
	m.status(20, "ready", "")
	m.status(21, "ready", "")
	if code, out, errOut := m.next(889); code != 0 || out != "start #20\nwaits #21: no free agent (1 of 1 busy after these starts)\n" {
		t.Fatalf("first run: exit %d: %q%s", code, out, errOut)
	}
	first := git(t, m.bare, "show", "ledger:"+MissionClaimsPath(889)) + "\n"
	putLedger(t, m.bare, MissionClaimsPath(889), "") // both runs read no claim
	race, pushed := m.opts, false
	race.LedgerAttempt = func(int, string) {
		if !pushed {
			pushed = true
			putLedger(t, m.bare, MissionClaimsPath(889), first)
		}
	}
	code, out, errOut := run(t, m.agent, race, "mission", "next", "889", "--repo", "o/r")
	if code != 0 || !pushed || out != "waits #20: claimed by another run\nwaits #21: no free agent (1 of 1 busy after these starts)\n" {
		t.Fatalf("second run: exit %d: %q%s", code, out, errOut)
	}
	if got := m.claims(889); got[20] != 1 || len(got) != 1 {
		t.Fatalf("#20 must be claimed once: %v\n%s", got, git(t, m.bare, "show", "ledger:"+MissionClaimsPath(889)))
	}
}

// Criterion 5: a halted repository exits 4 and a paused breaker exits 6, and
// neither claims anything.
func TestMissionNextHaltedOrPaused(t *testing.T) {
	m := newMissionWorld(t)
	m.startedMission(889, []int{20})
	m.status(20, "ready", "")
	n := m.commits()
	halted := map[string]string{HaltVariable: "tool incident"}
	for k, v := range m.agent {
		halted[k] = v
	}
	if code, out, errOut := run(t, halted, m.opts, "mission", "next", "889", "--repo", "o/r"); code != ExitBlocked || out != "" || !strings.Contains(errOut, "the repository is halted") {
		t.Fatalf("halted: exit %d: %s%s", code, out, errOut)
	}
	pause := filepath.Join(m.dir, ".aayudha", "state", "paused")
	if err := os.MkdirAll(filepath.Dir(pause), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(pause, []byte("per_day_usd"), 0o644)
	for _, env := range []map[string]string{m.agent, m.person} { // a person's run starts nothing either
		if code, out, errOut := run(t, env, m.opts, "mission", "next", "889", "--repo", "o/r"); code != ExitBudget || out != "" || !strings.Contains(errOut, "paused by the [budget] breaker") {
			t.Fatalf("breaker paused: exit %d: %s%s", code, out, errOut)
		}
	}
	if m.commits() != n {
		t.Fatal("a halted or paused run wrote to the ledger")
	}
}

// Criterion 6: the list marked #20 start, and #20 was blocked before the
// Kanha acted: build reads the live state, starts nothing, and frees the
// claim. A story no list marked keeps build's own message.
func TestBuildDelegateReadsLiveState(t *testing.T) {
	m := newMissionWorld(t)
	m.startedMission(889, []int{20, 21})
	m.status(20, "ready", "")
	if code, out, errOut := m.next(889); code != 0 || out != "start #20\nwaits #21: not Ready (open)\n" {
		t.Fatalf("mission next: exit %d: %q%s", code, out, errOut)
	}
	m.status(20, "blocked", "")
	m.g.Reset()
	code, out, errOut := run(t, m.agent, m.opts, "build", "20", "--delegate", "amp", "--repo", "o/r")
	if code != ExitBlocked || !strings.Contains(errOut, "#20 changed since the list: now blocked; not started") ||
		slices.ContainsFunc(m.g.Writes(), func(r fake.Request) bool { return strings.Contains(r.Path, "/issues/20") }) {
		t.Fatalf("build 20 --delegate amp: exit %d: %s%s", code, out, errOut)
	}
	if got := m.claims(889); len(got) != 0 {
		t.Fatalf("the claim of #20 is not released: %v", got)
	}
	m.status(21, "blocked", "")
	if code, _, errOut := run(t, m.agent, m.opts, "build", "21", "--delegate", "amp", "--repo", "o/r"); code != ExitBlocked ||
		strings.Contains(errOut, "changed since the list") || !strings.Contains(errOut, "not labelled status:ready") {
		t.Fatalf("build 21, on no list: exit %d: %s", code, errOut)
	}
}

// Criterion 7: a Feature whose approved intent drifted is not approved to
// build at its current hashes: exit 4, and nothing is listed or claimed.
func TestMissionNextNotApproved(t *testing.T) {
	m := newMissionWorld(t)
	m.startedMission(889, []int{20})
	m.status(20, "ready", "")
	n := m.commits()
	m.edit(func() { m.g.Issues[889].Body += "\nA new paragraph of intent.\n" })
	if code, out, errOut := m.next(889); code != ExitBlocked || out != "" || !strings.Contains(errOut, "#889 is not approved to build: intent v1 drifted") || m.commits() != n {
		t.Fatalf("mission next of a drifted Feature: exit %d: %s%s", code, out, errOut)
	}
}

// Criterion 8: with 620 USD spent of an approved 660, a story at its 60 USD
// cap would pass the budget: it waits, and nothing starts.
func TestMissionNextBudgetFilter(t *testing.T) {
	m := newMissionWorld(t)
	m.startedMission(889, []int{20, 25})
	m.setPlan(889, func(p *MissionPlan) { p.Budget = 660 })
	m.closeIssue(20)
	m.status(25, "ready", "")
	log := filepath.Join(m.dir, ".aayudha", "telemetry", "actions.jsonl")
	f, err := os.OpenFile(log, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString(`{"kind":"model.call","story":"o/r#20","status":"done","cost_usd":600}` + "\n" + `{"kind":"model.call","story":"o/r#25","status":"done","cost_usd":20}` + "\n" +
		`{"kind":"model.call","story":"o/r#30","status":"done","cost_usd":500}` + "\n") // #30 is no story of #889
	_ = f.Close()
	if code, out, errOut := m.next(889); code != 0 || out != "waits #25: would pass #889's approved budget (620 + 60 > 660 USD)\n" || len(m.claims(889)) != 0 {
		t.Fatalf("mission next over budget: exit %d: %q%s", code, out, errOut)
	}
	m.setPlan(889, func(p *MissionPlan) { p.Budget = 680 }) // 620 + 60 = 680 is within it
	if code, out, errOut := m.next(889); code != 0 || out != "start #25\n" {
		t.Fatalf("mission next at the budget: exit %d: %q%s", code, out, errOut)
	}
}

// tatkalWorld is Release #906 (v0.4.0) with Feature #874, whose mission is
// done (#950 merged), and its Tatkal Feature #920 with story #930, both
// started.
func tatkalWorld(t *testing.T) *missionWorld {
	m := newMissionWorld(t)
	m.issue(906, "Release: v0.4.0", "release", 0, "")
	m.edit(func() {
		rel := m.g.Issues[906]
		rel.Body = fixtureBody(t, "release", func(f map[string]string) { f["version"], f[workitems.FieldLinks] = "v0.4.0", "" })
		ms := &fake.Milestone{Number: 1, Title: "v0.4.0", State: "open"}
		m.g.Milestones = append(m.g.Milestones, ms)
	})
	m.tatkal[920] = 906
	m.startedMission(874, []int{950})
	m.startedMission(920, []int{930})
	m.setPlan(920, func(p *MissionPlan) { p.Budget = 600 }) // its ad hoc budget
	m.edit(func() { m.g.Issues[874].Milestone = m.g.Milestones[0] })
	m.closeIssue(950)
	return m
}

// Criterion 9: story #931 joins the Tatkal Feature after its one approval and
// becomes Ready, with no new Approve to build: it starts.
func TestMissionNextTatkalStoryJoins(t *testing.T) {
	m := tatkalWorld(t)
	approved := git(t, m.bare, "show", "ledger:"+ready.ApprovalPath(920))
	m.issue(931, "Story 931", "story", 920, "")
	m.status(931, "ready", "")
	if code, out, errOut := m.next(920); code != 0 || out != "waits #930: not Ready (open)\nstart #931\n" {
		t.Fatalf("mission next 920: exit %d: %q%s", code, out, errOut)
	}
	if git(t, m.bare, "show", "ledger:"+ready.ApprovalPath(920)) != approved {
		t.Fatal("the list wrote a new approval")
	}
}

// Criterion 10: a Ready Bug of the Feature is listed by the same order as a
// story, marked as a Bug.
func TestMissionNextListsBugs(t *testing.T) {
	m := newMissionWorld(t)
	m.startedMission(889, []int{26, 27}, 26)
	m.setPlan(889, func(p *MissionPlan) { p.Budget = 600 }) // the approval's budget counts its stories only
	m.status(26, "ready", "")
	m.status(27, "ready", "")
	if p, _ := m.plan(889); p.Milestones[0].Name != "MS1" || !slices.Equal(p.Milestones[0].Stories, []int{26, 27}) {
		t.Fatalf("milestones: %+v", p.Milestones)
	}
	if code, out, errOut := m.next(889); code != 0 || out != "start #26 (bug)\nstart #27\n" {
		t.Fatalf("mission next 889: exit %d: %q%s", code, out, errOut)
	}
}

// Criterion 11: #874's mission is done, so its Ready Bugs start from the
// Tatkal mission of its Release, in Ready time order; #874 lists nothing.
func TestMissionNextDoneFeatureBugGoesToTatkal(t *testing.T) {
	m := tatkalWorld(t)
	m.issue(940, "Bug 940", "bug", 874, "")
	m.issue(941, "Bug 941", "bug", 874, "")
	m.status(940, "ready", "")
	m.status(941, "ready", "")
	m.readyAt(940, time.Now().Add(-time.Hour))
	m.readyAt(941, time.Now().Add(-2*time.Hour)) // Ready first
	n := m.commits()
	code, out, errOut := m.next(874, "--json")
	var list WorkList
	if code != 0 || json.Unmarshal([]byte(out), &list) != nil || len(list.Items) != 0 || m.commits() != n {
		t.Fatalf("mission next 874: exit %d: %s%s", code, out, errOut)
	}
	if code, out, errOut := m.next(874); code != 0 || out != "The mission of #874 is done: nothing starts. A new Bug of #874 starts from its Release's Tatkal mission.\n" {
		t.Fatalf("mission next 874: exit %d: %q%s", code, out, errOut)
	}
	if code, out, errOut := m.next(920); code != 0 || out != "waits #930: not Ready (open)\nstart #941 (bug)\nstart #940 (bug)\n" {
		t.Fatalf("mission next 920: exit %d: %q%s", code, out, errOut)
	}
	// While #874's mission is not done (a story of its plan reopened), its Bugs stay with it.
	m.edit(func() { m.g.Issues[950].State = "open" })
	if code, out, _ := m.next(920); code != 0 || strings.Contains(out, "#94") {
		t.Fatalf("mission next 920 with #874 not done: exit %d: %q", code, out)
	}
}

// Criterion 12: --json is the aayudha.work/v1 list, with subthread on an item
// that already has one.
func TestMissionNextJSONShape(t *testing.T) {
	m := newMissionWorld(t)
	m.startedMission(889, []int{20, 22, 24})
	m.setPlan(889, func(p *MissionPlan) { p.Agents = 1 })
	m.status(20, "ready", "")
	m.status(22, "ready", "P2")
	m.status(24, "planned", "P2")
	code, out, errOut := m.next(889, "--json")
	want := `{"schema": "aayudha.work/v1", "scope": {"kind": "feature", "id": "889"}, "items": [{"key": "#20", "action": "start"}, {"key": "#22", "action": "waits", "reason": "no free agent"}, {"key": "#24", "action": "waits", "reason": "not Ready"}]}`
	if code != 0 || canonicalJSON(t, out) != canonicalJSON(t, want) {
		t.Fatalf("mission next 889 --json: exit %d:\n%s%s", code, out, errOut)
	}
	putLedger(t, m.bare, "delegate/22/state.json", `{"story": 22, "receiver": "amp", "task": "T-sub-22", "status": "failed"}`)
	code, out, _ = m.next(889, "--json")
	var list WorkList
	if code != 0 || json.Unmarshal([]byte(out), &list) != nil || list.Items[1] != (WorkListItem{Key: "#22", Action: "waits", Reason: "no free agent", Subthread: "T-sub-22"}) ||
		strings.Count(out, "subthread") != 1 {
		t.Fatalf("subthread: exit %d: %s", code, out)
	}
}

func canonicalJSON(t *testing.T, s string) string {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("%v: %s", err, s)
	}
	b, _ := json.Marshal(v)
	return string(b)
}

// Criterion 13: only the mission's recorded tool reads its list.
func TestMissionNextRefusedOnOtherTool(t *testing.T) {
	m := newMissionWorld(t)
	m.issue(874, "Feature baselines", "feature", 0, "")
	m.issue(950, "Story 950", "story", 874, "")
	m.feature(874, 950)
	m.mu.Lock()
	m.tool = "codex"
	m.mu.Unlock()
	if code, out, errOut := m.approve(874); code != 0 {
		t.Fatalf("approve: exit %d: %s%s", code, out, errOut)
	}
	m.g.Viewer = "bob"
	if code, out, errOut := m.mission(m.person, "bob", "start", 874); code != 0 {
		t.Fatalf("start: exit %d: %s%s", code, out, errOut)
	}
	m.status(950, "ready", "")
	n := m.commits()
	claude := map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "CLAUDECODE": "1", "XDG_STATE_HOME": m.agent["XDG_STATE_HOME"]}
	if code, out, errOut := run(t, claude, m.opts, "mission", "next", "874", "--repo", "o/r"); code != ExitBlocked || out != "" || m.commits() != n ||
		!strings.Contains(errOut, "#874's mission runs on codex, the tool of @bob, who started it: run this there") {
		t.Fatalf("mission next on claude: exit %d: %s%s", code, out, errOut)
	}
	codex := map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "CODEX_THREAD_ID": "t-1", "XDG_STATE_HOME": m.agent["XDG_STATE_HOME"]}
	if code, out, errOut := run(t, codex, m.opts, "mission", "next", "874", "--repo", "o/r"); code != 0 || out != "start #950\n" {
		t.Fatalf("mission next on codex: exit %d: %q%s", code, out, errOut)
	}
}
