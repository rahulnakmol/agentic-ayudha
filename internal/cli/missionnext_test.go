package cli

import (
	"context"
	"encoding/json"
	"errors"
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
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// Story #998: `aayu mission next <f>` on the missionWorld of Story #997. The
// Kanha runs in Amp (m.agent), and each mission is started on amp.

// startedMission adds Feature f with stories (Bugs when bug is set), in
// sub-issue order, with m.links, approves it, and starts its mission on amp.
func (m *missionWorld) startedMission(f int, stories []int, bugs ...int) {
	m.t.Helper()
	m.issue(f, fmt.Sprintf("Feature %d", f), "feature", 0, "")
	for _, n := range stories {
		typ := "story"
		if slices.Contains(bugs, n) {
			typ = "bug"
		}
		m.issue(n, fmt.Sprintf("Item %d", n), typ, f, m.links[n])
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
	commitOnDev(m.g, ".aayudha/config.toml", "[build]\nagents = 2\n")
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
	m.closeIssue(23)
	if code, out, _ := m.next(889); code != 0 || out != "Nothing to list: every open story and Bug of #889 runs, or none is open.\n" {
		t.Fatalf("with only #24 running: exit %d: %q", code, out)
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
	commitOnDev(m.g, ".aayudha/config.toml", "[build]\nagents = 1\n")
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
	// Another run's claim of #21 lands while this run claims #20: it took the last free agent.
	putLedger(t, m.bare, MissionClaimsPath(889), "")
	other, pushed := strings.Replace(first, `"story":20`, `"story":21`, 1), false
	race.LedgerAttempt = func(int, string) {
		if !pushed {
			pushed = true
			putLedger(t, m.bare, MissionClaimsPath(889), other)
		}
	}
	if code, out, errOut := run(t, m.agent, race, "mission", "next", "889", "--repo", "o/r"); code != 0 ||
		out != "waits #20: no free agent (1 of 1 busy after these starts)\nwaits #21: no free agent (1 of 1 busy after these starts)\n" {
		t.Fatalf("a race for the last agent: exit %d: %q%s", code, out, errOut)
	}
	// With 2 agents and a budget of one story, the story another run won holds its cap: this run starts nothing.
	putLedger(t, m.bare, MissionClaimsPath(889), "")
	commitOnDev(m.g, ".aayudha/config.toml", "[build]\nagents = 2\n")
	m.setPlan(889, func(p *MissionPlan) { p.Budget = 60 })
	pushed = false
	race.LedgerAttempt = func(int, string) {
		if !pushed {
			pushed = true
			putLedger(t, m.bare, MissionClaimsPath(889), first)
		}
	}
	if code, out, errOut := run(t, m.agent, race, "mission", "next", "889", "--repo", "o/r"); code != 0 ||
		out != "waits #20: claimed by another run\nwaits #21: would pass #889's approved budget (60 + 60 > 60 USD)\n" {
		t.Fatalf("a race within the budget: exit %d: %q%s", code, out, errOut)
	}
	// A Ready story with a live lease is a build that another run started: it holds an agent too.
	m.setPlan(889, func(p *MissionPlan) { p.Budget = 600 })
	putLedger(t, m.bare, MissionClaimsPath(889), "")
	m.edit(func() {
		is := m.g.Issues[21]
		is.Labels = append(is.Labels, fake.Label{Name: LeaseLabel})
		is.Comments = append(is.Comments, &fake.Comment{ID: 8021, Body: leaseBody("bob@laptop", time.Now().Add(time.Hour))})
	})
	if code, out, errOut := m.next(889); code != 0 || out != "start #20\nwaits #21: claimed by another run\n" {
		t.Fatalf("a leased story: exit %d: %q%s", code, out, errOut)
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
	// Each change since the list is named; a story a build already took (in progress) is no change.
	for _, n := range []int{22, 23, 24, 25} {
		m.issue(n, fmt.Sprintf("Story %d", n), "story", 889, "")
		m.status(n, "ready", "")
	}
	m.status(21, "planned", "")
	m.setPlan(889, func(p *MissionPlan) { p.Budget = 600 })
	if code, out, errOut := m.next(889); code != 0 || strings.Count(out, "start #") != 4 {
		t.Fatalf("mission next: exit %d: %q%s", code, out, errOut)
	}
	m.closeIssue(22)
	m.status(23, "planned", "")
	m.edit(func() { m.g.Issues[24].Body = "### Summary\n\nNo Completion Goal now.\n" })
	m.status(25, "in-progress", "")
	for n, want := range map[int]string{22: "now closed", 23: "now planned", 24: "now not Ready"} {
		if code, _, errOut := run(t, m.agent, m.opts, "build", fmt.Sprint(n), "--delegate", "amp", "--repo", "o/r"); code != ExitBlocked ||
			!strings.Contains(errOut, fmt.Sprintf("#%d changed since the list: %s; not started", n, want)) {
			t.Fatalf("build %d: exit %d: %s", n, code, errOut)
		}
	}
	if _, _, errOut := run(t, m.agent, m.opts, "build", "25", "--delegate", "amp", "--repo", "o/r"); strings.Contains(errOut, "changed since the list") {
		t.Fatalf("build 25, in progress: %s", errOut)
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
	// Nothing starts, either, for a Feature with no plan, a mission no one started or one that is paused, a
	// plan aayu did not write, or a run outside a checkout.
	m.issue(901, "Feature 901", "feature", 0, "")
	m.issue(60, "Story 60", "story", 901, "")
	m.feature(901, 60)
	if code, _, errOut := m.next(901); code != ExitBlocked || !strings.Contains(errOut, "#901 has no mission plan: a person approves it to build first. Nothing starts.") {
		t.Fatalf("next before Approve to build: exit %d: %s", code, errOut)
	}
	if code, out, errOut := m.approve(901); code != 0 {
		t.Fatalf("approve 901: exit %d: %s%s", code, out, errOut)
	}
	m.status(60, "ready", "")
	if code, _, errOut := m.next(901); code != ExitBlocked || !strings.Contains(errOut, "The mission of #901 has not started: a person starts it. Nothing starts.") {
		t.Fatalf("next before Start: exit %d: %s", code, errOut)
	}
	if code, out, errOut := m.mission(m.person, "rahulnakmol", "start", 901); code != 0 {
		t.Fatalf("start 901: exit %d: %s%s", code, out, errOut)
	}
	m.setPlan(901, func(p *MissionPlan) { p.Pause = &MissionPause{Reason: "a migration needs a person"} })
	if code, _, errOut := m.next(901); code != ExitBlocked || !strings.Contains(errOut, "The mission of #901 is paused: a migration needs a person. Nothing starts.") {
		t.Fatalf("next of a paused mission: exit %d: %s", code, errOut)
	}
	putLedger(t, m.bare, MissionPlanPath(901), "not json")
	if code, _, errOut := m.next(901); code != ExitExternal || !strings.Contains(errOut, "not a record aayu wrote for #901") {
		t.Fatalf("next of a forged plan: exit %d: %s", code, errOut)
	}
	away := m.opts
	away.Dir = t.TempDir()
	if code, _, errOut := run(t, m.agent, away, "mission", "next", "889", "--repo", "o/r"); code != ExitUsage || !strings.Contains(errOut, "ledger writes need a checkout") {
		t.Fatalf("next outside a checkout: exit %d: %s", code, errOut)
	}
	if len(m.claims(889)) != 0 || len(m.claims(901)) != 0 {
		t.Fatal("a refused next claimed a story")
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
	// #21 runs with no measured spend yet: it counts at its cap, so a later run cannot pass the budget either.
	m.issue(21, "Story 21", "story", 889, "")
	m.status(21, "in-progress", "")
	m.setPlan(889, func(p *MissionPlan) { p.Budget = 680 })
	if code, out, errOut := m.next(889); code != 0 || out != "waits #25: would pass #889's approved budget (680 + 60 > 680 USD)\n" {
		t.Fatalf("mission next with #21 running: exit %d: %q%s", code, out, errOut)
	}
	m.setPlan(889, func(p *MissionPlan) { p.Budget = 740 }) // 680 + 60 = 740 is within it
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
	// The Release's milestone also holds the Tatkal Feature itself, a story, and a Feature with no plan: none adds a Bug.
	m.issue(877, "Feature 877", "feature", 0, "")
	m.issue(942, "Bug 942", "bug", 877, "")
	m.status(942, "ready", "")
	m.edit(func() {
		for _, n := range []int{920, 930, 877} {
			m.g.Issues[n].Milestone = m.g.Milestones[0]
		}
	})
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
	// With #874 done again but no milestone for the Release's version, no Feature is in the Release.
	m.closeIssue(950)
	m.edit(func() { m.g.Milestones[0].Title = "v0.5.0" })
	if code, out, _ := m.next(920); code != 0 || out != "waits #930: not Ready (open)\n" {
		t.Fatalf("mission next 920 with no v0.4.0 milestone: exit %d: %q", code, out)
	}
}

// Criterion 12: --json is the aayudha.work/v1 list, with subthread on an item
// that already has one.
func TestMissionNextJSONShape(t *testing.T) {
	m := newMissionWorld(t)
	m.startedMission(889, []int{20, 22, 24})
	commitOnDev(m.g, ".aayudha/config.toml", "[build]\nagents = 1\n")
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

// GitHub failing at any read stops the list: exit 5, and nothing is claimed.
func TestMissionNextGitHubFailsClosed(t *testing.T) {
	m := tatkalWorld(t)
	m.issue(940, "Bug 940", "bug", 874, "")
	m.status(940, "ready", "")
	m.status(930, "ready", "")
	m.edit(func() {
		is := m.g.Issues[930]
		is.Labels = append(is.Labels, fake.Label{Name: LeaseLabel})
		is.Comments = append(is.Comments, &fake.Comment{ID: 8930, Body: leaseBody("bob@laptop", time.Now().Add(-time.Hour))}) // run out
	})
	for _, c := range []struct {
		fail  string
		after int
	}{
		{"GET /repos/o/r/issues/920", 0}, {"GET /repos/o/r/issues/920/sub_issues", 0}, {"GET /repos/o/r/issues/906", 0},
		{"GET /repos/o/r/milestones", 0}, {"GET /repos/o/r/issues", 0}, {"GET /repos/o/r/issues/874/sub_issues", 0},
		{"GET /repos/o/r/issues/930/comments", 0}, {"GET /repos/o/r/issues/930/comments", 1},
	} {
		m.edit(func() { m.g.Fail, m.g.FailAfter = c.fail, c.after })
		if code, out, errOut := m.next(920); code != ExitExternal || out != "" || len(m.claims(920)) != 0 {
			t.Fatalf("%s failing (after %d): exit %d: %s%s", c.fail, c.after, code, out, errOut)
		}
	}
	m.edit(func() { m.g.Fail = "" })
	m.opts.TatkalRelease = func(context.Context, int) (int, error) { return 0, errors.New("the Tatkal record is unreadable") }
	if code, _, errOut := m.next(920); code != ExitExternal || !strings.Contains(errOut, "the Tatkal record is unreadable") {
		t.Fatalf("Tatkal seam failing: exit %d: %s", code, errOut)
	}
}

// Story #999: a Ready story waits while an issue it depends on is open.
// #10 is a story of another Feature, so its state is read, not listed.
func dependsOnWorld(t *testing.T) *missionWorld {
	m := newMissionWorld(t)
	m.issue(888, "Other Feature", "feature", 0, "")
	m.issue(10, "Story 10", "story", 888, "")
	m.status(10, "in-progress", "")
	m.links = map[int]string{11: "depends-on #10"}
	m.startedMission(889, []int{11})
	m.status(11, "ready", "")
	return m
}

// Criterion 1: #10 is open and in progress, so #11 waits and is not claimed.
// A failed read of #10 stops the list: it never starts #11.
func TestMissionNextHoldsOpenDependsOn(t *testing.T) {
	m := dependsOnWorld(t)
	n := m.commits()
	m.edit(func() { m.g.Fail = "GET /repos/o/r/issues/10" })
	if code, out, errOut := m.next(889); code != ExitExternal || out != "" || len(m.claims(889)) != 0 {
		t.Fatalf("reading #10 fails: exit %d: %q%s", code, out, errOut)
	}
	m.edit(func() { m.g.Fail = "" })
	if code, out, errOut := m.next(889); code != 0 || out != "waits #11: depends-on #10 is open\n" || len(m.claims(889)) != 0 || m.commits() != n {
		t.Fatalf("mission next 889: exit %d: %q%s", code, out, errOut)
	}
}

// Criterion 2: once #10 closes as merged, #11 starts.
func TestMissionNextStartsAfterDependsOnMerges(t *testing.T) {
	m := dependsOnWorld(t)
	m.status(10, "merged", "")
	m.closeIssue(10)
	if code, out, errOut := m.next(889); code != 0 || out != "start #11\n" || m.claims(889)[11] != 1 {
		t.Fatalf("mission next 889: exit %d: %q%s", code, out, errOut)
	}
}

// Criterion 3: #11 depends on #10 (open, in this Feature) and #12 (closed, in
// another Feature). Only #10 holds it, and #13, later in the order, starts.
func TestMissionNextHoldsOnlyTheDependent(t *testing.T) {
	m := newMissionWorld(t)
	m.issue(888, "Other Feature", "feature", 0, "")
	m.issue(12, "Story 12", "story", 888, "")
	m.links = map[int]string{11: "depends-on #10\ndepends-on #12"}
	m.startedMission(889, []int{10, 11, 13})
	commitOnDev(m.g, ".aayudha/config.toml", "[build]\nagents = 2\n")
	m.status(12, "merged", "")
	m.closeIssue(12)
	m.status(10, "in-progress", "")
	m.status(11, "ready", "")
	m.status(13, "ready", "")
	if code, out, errOut := m.next(889); code != 0 || out != "waits #11: depends-on #10 is open\nstart #13\n" {
		t.Fatalf("mission next 889: exit %d: %q%s", code, out, errOut)
	}
	if got := m.claims(889); got[13] != 1 || len(got) != 1 {
		t.Fatalf("claims: %v", got)
	}
}

// A depends-on line that ParseLinks misses counts as open (Story #999, review
// guidance): a target in another repository, which aayu cannot read here, and
// a link with other capitals, whose target #12 is closed, and a line with no
// target after characters that grow when lowered.
func TestMissionNextUnparsedDependsOnIsOpen(t *testing.T) {
	m := newMissionWorld(t)
	m.issue(888, "Other Feature", "feature", 0, "")
	m.issue(12, "Story 12", "story", 888, "")
	m.closeIssue(12)
	m.links = map[int]string{11: "depends-on other/repo#10\nDepends-On #12\nȺȺȺȺȺȺȺȺȺȺȺȺ depends-on"}
	m.startedMission(889, []int{11})
	m.status(11, "ready", "")
	if code, out, errOut := m.next(889); code != 0 || out != "waits #11: depends-on other/repo#10, #12, ȺȺȺȺȺȺȺȺȺȺȺȺ depends-on are open\n" {
		t.Fatalf("mission next 889: exit %d: %q%s", code, out, errOut)
	}
}

// nextEnv runs aayu mission next f as the Kanha in Amp, with extra variables.
func (m *missionWorld) nextEnv(f int, extra map[string]string, args ...string) (int, string, string) {
	m.t.Helper()
	env := map[string]string{}
	for k, v := range m.agent {
		env[k] = v
	}
	for k, v := range extra {
		env[k] = v
	}
	return run(m.t, env, m.opts, append([]string{"mission", "next", fmt.Sprint(f), "--repo", "o/r"}, args...)...)
}

// readyAll makes each story Ready, in slice order.
func (m *missionWorld) readyAll(ns ...int) {
	for _, n := range ns {
		m.status(n, "ready", "")
	}
}

// starts counts the start lines of a list.
func starts(out string) int { return strings.Count(out, "start #") }

// Story #1000, criterion 1: with no [build] agents in any config, the list
// for #889 starts at most 5 of its 6 Ready stories (Decision #918).
func TestMissionNextAgentsDefault(t *testing.T) {
	m := newMissionWorld(t)
	stories := []int{20, 21, 22, 23, 24, 25}
	m.startedMission(889, stories)
	m.readyAll(stories...)
	code, out, errOut := m.next(889)
	if code != 0 || starts(out) != 5 || !strings.Contains(out, "waits #25: no free agent (5 of 5 busy after these starts)") {
		t.Fatalf("no [build] agents anywhere: exit %d: %q%s", code, out, errOut)
	}
	if got := m.claims(889); len(got) != 5 || got[25] != 0 {
		t.Fatalf("claims: %v", got)
	}
}

// Story #1000, criterion 2: dev's committed [build] agents caps the list; a
// local value (AAYU_BUILD__AGENTS, --set) only lowers it, and the embedded
// default (5) is no local value: dev's 7 holds over it.
func TestAgentsDefaultAndLocalOnlyLowers(t *testing.T) {
	stories := []int{20, 21, 22, 23, 24, 25, 26, 27, 28}
	for _, tc := range []struct {
		name, committed string
		local           map[string]string
		args            []string
		want            int
	}{
		{"committed 5, local 10", "[build]\nagents = 5\n", map[string]string{"AAYU_BUILD__AGENTS": "10"}, nil, 5},
		{"committed 3, --set 6", "[build]\nagents = 3\n", nil, []string{"--set", "build.agents=6"}, 3},
		{"committed 5, local 2", "[build]\nagents = 5\n", map[string]string{"AAYU_BUILD__AGENTS": "2"}, nil, 2},
		{"committed 7, no local", "[build]\nagents = 7\n", nil, nil, 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newMissionWorld(t)
			m.startedMission(889, stories)
			m.readyAll(stories...)
			commitOnDev(m.g, ".aayudha/config.toml", tc.committed)
			code, out, errOut := m.nextEnv(889, tc.local, tc.args...)
			if busy := fmt.Sprintf("no free agent (%d of %d busy after these starts)", tc.want, tc.want); code != 0 || starts(out) != tc.want || !strings.Contains(out, busy) {
				t.Fatalf("want %d starts: exit %d: %q%s", tc.want, code, out, errOut)
			}
		})
	}
}

// Story #1000, criterion 3: Approve to build with --agents 3 writes agents 3
// to specs/889/approved.json, and the list for #889 uses 3, while #874, with
// no --agents, uses dev's [build] agents (2). A bare Approve to build keeps 3.
func TestReadyAgentsFlagWritesApproval(t *testing.T) {
	m := newMissionWorld(t)
	commitOnDev(m.g, ".aayudha/config.toml", "[build]\nagents = 2\n")
	m.mu.Lock()
	m.tool = "amp"
	m.mu.Unlock()
	for f, ns := range map[int][]int{889: {20, 21, 22, 23}, 874: {30, 31, 32, 33}} {
		m.issue(f, fmt.Sprintf("Feature %d", f), "feature", 0, "")
		for _, n := range ns {
			m.issue(n, fmt.Sprintf("Item %d", n), "story", f, "")
		}
		m.feature(f, ns...)
		var args []string
		if f == 889 {
			args = []string{"--agents", "3"}
		}
		code, out, errOut := m.approve(f, args...)
		if code != 0 || f == 889 && !strings.Contains(errOut, "up to 3 stories at once (aayu ready 889 --agents 3 at Approve to build)") ||
			f == 874 && !strings.Contains(errOut, "up to 2 stories at once ([build] agents on dev)") {
			t.Fatalf("approve %d %v: exit %d: %s%s", f, args, code, out, errOut)
		}
		if code, out, errOut := m.mission(m.person, "rahulnakmol", "start", f); code != 0 {
			t.Fatalf("start %d: exit %d: %s%s", f, code, out, errOut)
		}
		m.readyAll(ns...)
	}
	approval := func() ready.Approval {
		var a ready.Approval
		if err := json.Unmarshal([]byte(git(t, m.bare, "show", "ledger:"+ready.ApprovalPath(889))), &a); err != nil {
			t.Fatal(err)
		}
		return a
	}
	if a := approval(); a.Agents != 3 || a.Version != 1 {
		t.Fatalf("specs/889/approved.json: agents %d, version %d", a.Agents, a.Version)
	}
	if raw := git(t, m.bare, "show", "ledger:"+ready.ApprovalPath(874)); strings.Contains(raw, `"agents"`) {
		t.Fatalf("specs/874/approved.json names agents with no --agents:\n%s", raw)
	}
	if code, out, errOut := m.next(889); code != 0 || starts(out) != 3 || !strings.Contains(out, "waits #23: no free agent (3 of 3 busy after these starts)") {
		t.Fatalf("next 889: exit %d: %q%s", code, out, errOut)
	}
	if code, out, errOut := m.next(874); code != 0 || starts(out) != 2 || !strings.Contains(out, "no free agent (2 of 2 busy after these starts)") {
		t.Fatalf("next 874: exit %d: %q%s", code, out, errOut)
	}
	// A bare Approve to build, which aayu names as the next command, keeps the Feature's agents: nothing to approve again.
	if code, out, errOut := m.approve(889); code != 0 || m.codes() != 0 || approval().Agents != 3 || approval().Version != 1 {
		t.Fatalf("bare approve 889: exit %d, %d device codes, approval %+v: %s%s", code, m.codes(), approval(), out, errOut)
	}
	// --agents names a Feature's value; a story has none, and 0 is no value.
	if code, _, errOut := run(t, m.person, m.opts, "ready", "20", "--repo", "o/r", "--agents", "2"); code != ExitUsage || !strings.Contains(errOut, "#20 is not a Feature") {
		t.Fatalf("--agents on a story: exit %d: %s", code, errOut)
	}
	if code, _, errOut := run(t, m.person, m.opts, "ready", "889", "--repo", "o/r", "--agents", "0"); code != ExitUsage {
		t.Fatalf("--agents 0: exit %d: %s", code, errOut)
	}
}

// Story #1000, criterion 4: a wake's prompt says "agents 10" and "budget
// 2000". aayu never reads a prompt: mission takes no input that sets a limit
// (its other inputs, such as --story or --reason, set none), so the prompt
// can reach it only as a flag, which is refused, or as a local value, which
// only lowers. The list uses the approved budget, then dev's [build] agents (5).
func TestPromptNeverSetsALimit(t *testing.T) {
	limits := []string{"agents", "budget", "max_no_go", "retry_max", "retry_backoff_minutes", "check_minutes", "warn_hours", "max_hours"}
	spec, ok := verbs.Find("mission")
	if !ok {
		t.Fatal("no mission verb")
	}
	for _, in := range spec.Inputs {
		if slices.Contains(limits, strings.ReplaceAll(in.Name, "-", "_")) {
			t.Fatalf("mission takes the limit --%s", in.Name)
		}
	}
	m := newMissionWorld(t)
	stories := []int{20, 21, 22, 23, 24, 25, 26}
	m.startedMission(889, stories)
	m.setPlan(889, func(p *MissionPlan) { p.Budget = 240 }) // 4 stories at the approval's 60 USD cap
	commitOnDev(m.g, ".aayudha/config.toml", "[build]\nagents = 5\n")
	m.readyAll(stories...)
	for _, flag := range [][]string{{"--agents", "10"}, {"--budget", "2000"}} {
		if code, out, errOut := m.next(889, flag...); code != ExitUsage || out != "" {
			t.Fatalf("mission next %v: exit %d: %q%s", flag, code, out, errOut)
		}
	}
	if got := m.claims(889); len(got) != 0 {
		t.Fatalf("a refused prompt flag claimed %v", got)
	}
	prompt := map[string]string{"AAYU_BUILD__AGENTS": "10", "AAYU_BUDGET__PER_STORY_USD": "1"}
	code, out, errOut := m.nextEnv(889, prompt)
	if code != 0 || starts(out) != 4 || !strings.Contains(out, "waits #24: would pass #889's approved budget (240 + 60 > 240 USD)") {
		t.Fatalf("mission next under the approved budget: exit %d: %q%s", code, out, errOut)
	}
	// A person raises the mission's budget: the agents cap holds at dev's 5, not the prompt's 10.
	m.setPlan(889, func(p *MissionPlan) { p.Budget = 600 })
	code, out, errOut = m.nextEnv(889, prompt)
	if code != 0 || starts(out) != 1 || !strings.Contains(out, "start #24\n") || !strings.Contains(out, "waits #25: no free agent (5 of 5 busy after these starts)") {
		t.Fatalf("mission next under [build] agents: exit %d: %q%s", code, out, errOut)
	}
	if got := m.claims(889); len(got) != 5 {
		t.Fatalf("claims: %v", got)
	}
}

// Story #1000: [build] agents is read from dev's tip on GitHub. When that read
// fails, approve, start, show, and next exit 5 and write nothing; none falls
// back to a local value or a constant.
func TestMissionAgentsReadFailsClosed(t *testing.T) {
	m := newMissionWorld(t)
	m.startedMission(889, []int{20, 21})
	m.status(20, "ready", "")
	m.issue(874, "Feature 874", "feature", 0, "")
	m.issue(30, "Item 30", "story", 874, "")
	m.feature(874, 30)
	if code, out, errOut := m.approve(874); code != 0 {
		t.Fatalf("approve 874: exit %d: %s%s", code, out, errOut)
	}
	commits := m.commits()
	failDev := func() { m.edit(func() { m.g.Fail, m.g.FailAfter = "GET /repos/o/r/git/ref/heads/dev", 0 }) }
	for name, cmd := range map[string]func() (int, string, string){
		"next 889": func() (int, string, string) { return m.next(889) },
		"show 889": func() (int, string, string) {
			return run(t, m.agent, m.opts, "mission", "show", "889", "--repo", "o/r")
		},
		"approve 889": func() (int, string, string) { return m.approve(889) },
		"start 874":   func() (int, string, string) { return m.mission(m.person, "rahulnakmol", "start", 874) },
	} {
		failDev()
		if code, out, errOut := cmd(); code != ExitExternal || !strings.Contains(errOut, "heads/dev") {
			t.Errorf("%s with dev's config unreadable: exit %d: %s%s", name, code, out, errOut)
		}
	}
	if m.commits() != commits || len(m.claims(889)) != 0 {
		t.Fatalf("a failed read wrote the ledger: %s -> %s commits, claims %v", commits, m.commits(), m.claims(889))
	}
}
