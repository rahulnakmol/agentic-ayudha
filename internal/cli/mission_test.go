package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// missionWorld is Story #997's scenario: Features with stories and Bugs on a
// fake GitHub and a local ledger, each Feature's baselines approved, and the
// inputs from work not merged yet (the person's tool, #886's estimate, and
// the Tatkal Feature) faked.
type missionWorld struct {
	t         *testing.T
	g         *fake.GitHub
	dir, bare string
	person    map[string]string
	agent     map[string]string
	opts      Options
	mu        sync.Mutex
	tool      string
	est       *CostEstimate
	tatkal    map[int]int
}

func newMissionWorld(t *testing.T) *missionWorld {
	t.Helper()
	m := &missionWorld{t: t, g: fake.New(t, "o/r", "User"), tool: "cursor", tatkal: map[int]int{}}
	m.g.Viewer = "rahulnakmol"
	m.dir, m.bare = ledgerCheckout(t)
	state := m.dir + "/state"
	m.person = map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "XDG_STATE_HOME": state}
	m.agent = map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "AMP_THREAD_ID": "T-agent-thread", "XDG_STATE_HOME": state}
	m.opts = Options{IsTerminal: stdinTTY, GitHubAPI: m.g.URL(), GitHubGap: -1, Dir: m.dir, DevicePoll: 10 * time.Millisecond,
		RepoTool: func(_ context.Context, repo string) (string, error) {
			if repo != "o/r" {
				t.Errorf("RepoTool(%s)", repo)
			}
			m.mu.Lock()
			defer m.mu.Unlock()
			return m.tool, nil
		},
		CostEstimate:  func(context.Context, int) (*CostEstimate, error) { m.mu.Lock(); defer m.mu.Unlock(); return m.est, nil },
		TatkalRelease: func(_ context.Context, f int) (int, error) { m.mu.Lock(); defer m.mu.Unlock(); return m.tatkal[f], nil },
	}
	m.issue(870, "Human-dependency engine", "initiative", 0, "")
	return m
}

func (m *missionWorld) edit(f func()) { m.g.Mu.Lock(); f(); m.g.Mu.Unlock() }

// issue adds issue n of type typ under parent, with depends-on links.
func (m *missionWorld) issue(n int, title, typ string, parent int, links string) *fake.Issue {
	m.t.Helper()
	body := fixtureBody(m.t, typ, func(f map[string]string) {
		delete(f, workitems.FieldLinks)
		if links != "" {
			f[workitems.FieldLinks] = links
		}
		if c, ok := f[workitems.FieldCriteria]; ok {
			f[workitems.FieldCriteria] = strings.Replace(c, "cart is empty", title+" is empty", 1)
		}
	})
	labels := []string{"type:" + typ}
	switch typ {
	case "story", "bug":
		labels = append(labels, "effort:S", "layer:L02", "priority:P1")
	case "feature":
		labels = append(labels, "status:planned")
	}
	is := m.g.AddIssue(title, body, labels...)
	m.edit(func() {
		delete(m.g.Issues, is.Number)
		is.Number, m.g.Issues[n] = n, is
		if parent != 0 {
			is.Parent = parent
			m.g.Issues[parent].Subs = append(m.g.Issues[parent].Subs, n)
		}
	})
	return is
}

// feature adds Feature f under the Initiative, after its stories and Bugs
// (added with issue, parent f), and approves its intent, spec, and
// technical design by hand, as TestReadyApproveToBuild does.
func (m *missionWorld) feature(f int, stories ...int) {
	m.t.Helper()
	m.edit(func() {
		is := m.g.Issues[f]
		is.Parent = 870
		m.g.Issues[870].Subs = append(m.g.Issues[870].Subs, f)
	})
	order, _ := json.Marshal(stories)
	putLedger(m.t, m.bare, fmt.Sprintf("specs/%d/design.json", f), fmt.Sprintf(`{"feature": %d, "layers": ["L02"], "story_order": %s}`, f, order))
	for _, part := range []string{"intent", "spec", "tech"} {
		if code, out, errOut := run(m.t, map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "AAYU_AGENT": "1", "XDG_STATE_HOME": m.person["XDG_STATE_HOME"]},
			m.opts, "ready", fmt.Sprint(f), "--repo", "o/r", "--part", part, "--request"); code != 0 {
			m.t.Fatalf("ready %d --part %s --request: exit %d: %s%s", f, part, code, out, errOut)
		}
		var b ready.Baseline
		if err := json.Unmarshal([]byte(git(m.t, m.bare, "show", "ledger:"+ready.BaselinePath(f, part, 1, true))), &b); err != nil {
			m.t.Fatal(err)
		}
		b.ApprovedBy, b.ApprovedAt, b.Presence = "rahulnakmol", "2026-10-26T00:00:00Z", "device-flow"
		data, _ := json.Marshal(b)
		putLedger(m.t, m.bare, ready.BaselinePath(f, part, 1, false), string(data))
	}
}

// device has the next presence check approved by approver.
func (m *missionWorld) device(approver string) {
	m.edit(func() {
		m.g.Device = &fake.DeviceFlow{ClientID: DefaultDeviceClientID, Approver: approver, Outcome: "approve"}
	})
}

func (m *missionWorld) codes() int {
	m.g.Mu.Lock()
	defer m.g.Mu.Unlock()
	if m.g.Device == nil {
		return 0
	}
	return m.g.Device.Started
}

func (m *missionWorld) commits() string { return git(m.t, m.bare, "rev-list", "--count", "ledger") }

// approve is Approve to build of f by @rahulnakmol, with args such as --milestones.
func (m *missionWorld) approve(f int, args ...string) (int, string, string) {
	m.t.Helper()
	m.device("rahulnakmol")
	return run(m.t, m.person, m.opts, append([]string{"ready", fmt.Sprint(f), "--repo", "o/r"}, args...)...)
}

// mission runs aayu mission <action> f in env, with the device code approved by approver.
func (m *missionWorld) mission(env map[string]string, approver, action string, f int, args ...string) (int, string, string) {
	m.t.Helper()
	m.device(approver)
	return run(m.t, env, m.opts, append([]string{"mission", action, fmt.Sprint(f), "--repo", "o/r"}, args...)...)
}

// plan reads missions/<f>/plan.json from the ledger, and its raw text.
func (m *missionWorld) plan(f int) (MissionPlan, string) {
	m.t.Helper()
	raw := git(m.t, m.bare, "show", "ledger:"+MissionPlanPath(f))
	var p MissionPlan
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		m.t.Fatalf("plan.json: %v\n%s", err, raw)
	}
	return p, raw
}

func (m *missionWorld) closeIssue(n int) { m.edit(func() { m.g.Issues[n].State = "closed" }) }

// scenario889 is Feature #889 with stories #20 to #23 and Feature #874 with
// story #950; links give each story's depends-on.
func (m *missionWorld) scenario889(links map[int]string) {
	m.t.Helper()
	m.issue(874, "Feature baselines", "feature", 0, "")
	m.issue(950, "Story 950", "story", 874, "")
	m.issue(889, "Agents pull the next story", "feature", 0, "")
	for n := 20; n <= 23; n++ {
		m.issue(n, fmt.Sprintf("Story %d", n), "story", 889, links[n])
	}
	m.feature(889, 20, 21, 22, 23)
}

// Criterion 10: an agent lane's `aayu mission start` is refused before
// anything is read, with no device code, and nothing is written.
func TestMissionStartRefusedForAgentLane(t *testing.T) {
	m := newMissionWorld(t)
	m.scenario889(nil)
	if code, out, errOut := m.approve(889); code != 0 {
		t.Fatalf("approve 889: exit %d: %s%s", code, out, errOut)
	}
	before, raw := m.commits(), git(t, m.bare, "show", "ledger:"+MissionPlanPath(889))
	m.g.Reset()
	code, out, errOut := m.mission(m.agent, "rahulnakmol", "start", 889)
	want := "aayu: mission start is a person's decision and is refused for an agent lane (AMP_THREAD_ID)"
	if code != ExitBlocked || !strings.Contains(errOut, want) || m.codes() != 0 || m.commits() != before || len(m.g.Writes()) != 0 ||
		git(t, m.bare, "show", "ledger:"+MissionPlanPath(889)) != raw {
		t.Fatalf("agent mission start: exit %d, device codes %d, commits %s → %s: %s%s", code, m.codes(), before, m.commits(), out, errOut)
	}
	// AAYU_AGENT=1 (MCP and aayu's loops) is an agent lane too.
	code, _, errOut = m.mission(map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "AAYU_AGENT": "1"}, "rahulnakmol", "start", 889)
	if code != ExitBlocked || !strings.Contains(errOut, "refused for an agent lane (AAYU_AGENT=1)") || m.commits() != before {
		t.Fatalf("AAYU_AGENT=1 mission start: exit %d: %s", code, errOut)
	}
}

// Criterion 1: Approve to build with milestones MS1 (#20, #21) and MS2 (#22,
// #23) writes the plan; Start, after the presence check, adds the starter,
// his tool for this repository, and the budget.
func TestMissionStartWritesPlan(t *testing.T) {
	m := newMissionWorld(t)
	m.scenario889(map[int]string{21: "depends-on #20"})
	n := m.commits()
	code, out, errOut := m.approve(889, "--milestones", "MS1=#20,#21 MS2=#22,#23")
	if code != 0 || m.codes() != 1 || !strings.Contains(errOut, "Approving: Mission plan: MS1 #20, #21 · MS2 #22, #23; every story and Bug is in one milestone") ||
		!strings.Contains(errOut, "Approving: Estimate: runs ≥ 4 stories + 2 × 2 milestones = 8; up to 5 stories at once") {
		t.Fatalf("approve 889: exit %d, device codes %d: %s%s", code, m.codes(), out, errOut)
	}
	if got := m.commits(); got == n {
		t.Fatal("Approve to build wrote nothing")
	}
	p, raw := m.plan(889)
	wantGraph := []MissionItem{
		{Number: 20, Kind: "story", Title: "Story 20", Milestone: "MS1", DependsOn: []int{}, Outside: []MissionDep{}},
		{Number: 21, Kind: "story", Title: "Story 21", Milestone: "MS1", DependsOn: []int{20}, Outside: []MissionDep{}},
		{Number: 22, Kind: "story", Title: "Story 22", Milestone: "MS2", DependsOn: []int{}, Outside: []MissionDep{}},
		{Number: 23, Kind: "story", Title: "Story 23", Milestone: "MS2", DependsOn: []int{}, Outside: []MissionDep{}},
	}
	if p.Approval != 1 || p.ApprovedBy != "rahulnakmol" || fmt.Sprint(p.Graph) != fmt.Sprint(wantGraph) || len(p.Milestones) != 2 ||
		!slices.Equal(p.Milestones[0].Stories, []int{20, 21}) || !slices.Equal(p.Milestones[1].Stories, []int{22, 23}) ||
		p.Agents != 5 || p.Estimate.Runs != 8 || p.Estimate.Basis != "runs ≥ 4 stories + 2 × 2 milestones = 8" || p.StartedBy != "" || p.Tool != "" {
		t.Fatalf("plan.json after Approve to build:\n%s", raw)
	}

	// Start: the effect before the device code, then the starter, the tool, and the budget.
	n = m.commits()
	code, out, errOut = m.mission(m.person, "rahulnakmol", "start", 889)
	if code != 0 || m.codes() != 1 || !strings.Contains(errOut, "It runs in cursor, the tool you set for this repository; up to 5 stories at once") ||
		strings.Index(errOut, "BUDGET $240") > strings.Index(errOut, "/login/device") {
		t.Fatalf("mission start 889: exit %d, device codes %d: %s%s", code, m.codes(), out, errOut)
	}
	p, raw = m.plan(889)
	if p.StartedBy != "rahulnakmol" || p.Tool != "cursor" || p.Budget != 240 || p.StartedAt == "" || p.Estimate.Runs != 8 ||
		!strings.Contains(raw, `"started_by": "rahulnakmol"`) || !strings.Contains(raw, `"tool": "cursor"`) || !strings.Contains(raw, `"budget": 240`) ||
		len(p.Controls) != 1 || p.Controls[0] != (MissionControl{Action: "start", By: "rahulnakmol", At: p.StartedAt, Presence: "device-flow"}) {
		t.Fatalf("plan.json after Start:\n%s", raw)
	}
	if got := git(t, m.bare, "rev-list", "--count", "ledger"); got == n || !strings.Contains(git(t, m.bare, "log", "-1", "--format=%s", "ledger"), "@rahulnakmol started the mission of #889 on cursor") {
		t.Fatalf("Start's ledger commit: %s", git(t, m.bare, "log", "-1", "--format=%s", "ledger"))
	}
}

// Criteria 2 and 3: a draft plan that leaves story #24 and Bug #25 in no
// milestone is refused before the device code; one milestone with every
// story passes, and its gate runs when the last story merges.
func TestApproveToBuildRefusesStoryWithNoMilestone(t *testing.T) {
	m := newMissionWorld(t)
	m.issue(889, "Agents pull the next story", "feature", 0, "")
	for n := 20; n <= 24; n++ {
		m.issue(n, fmt.Sprintf("Story %d", n), "story", 889, "")
	}
	m.issue(25, "Answers vanish", "bug", 889, "")
	m.feature(889, 20, 21, 22, 23, 24)
	n := m.commits()
	m.g.Reset()
	code, out, errOut := m.approve(889, "--milestones", "MS1=#20,#21 MS2=#22,#23")
	if code != ExitBlocked || !strings.Contains(errOut, "#24 and #25 are in no milestone: every story and Bug belongs to one") ||
		m.codes() != 0 || m.commits() != n || slices.ContainsFunc(m.g.Writes(), func(r fake.Request) bool { return strings.Contains(r.Path, "/issues/") }) {
		t.Fatalf("a story and a Bug in no milestone: exit %d, device codes %d, commits %s → %s: %s%s", code, m.codes(), n, m.commits(), out, errOut)
	}
	// A story in two milestones, and a number that is not the Feature's, are refused too.
	for spec, want := range map[string]string{
		"MS1=#20,#21,#22 MS2=#22,#23,#24,#25": "#22 is in MS1 and MS2: every story and Bug belongs to one milestone",
		"MS1=#20,#21,#22,#23,#24,#25,#99":     "MS1 names #99, which is not a story or Bug of #889",
	} {
		if code, out, errOut := m.approve(889, "--milestones", spec); code != ExitBlocked || !strings.Contains(errOut, want) || m.commits() != n {
			t.Fatalf("--milestones %s: exit %d: %s%s", spec, code, out, errOut)
		}
	}

	// 3. One milestone with every story and Bug passes.
	if code, out, errOut := m.approve(889, "--milestones", "MS1=#20,#21,#22,#23,#24,#25"); code != 0 {
		t.Fatalf("one milestone: exit %d: %s%s", code, out, errOut)
	}
	p, raw := m.plan(889)
	if len(p.Milestones) != 1 || !slices.Equal(p.Milestones[0].Stories, []int{20, 21, 22, 23, 24, 25}) || p.Milestones[0].Gate != "runs when its last story merges" ||
		p.Graph[5].Kind != "bug" || p.Graph[5].Milestone != "MS1" || p.Estimate.Basis != "runs ≥ 6 stories and Bugs + 2 × 1 milestones = 8" {
		t.Fatalf("plan.json with one milestone:\n%s", raw)
	}
	for _, s := range []int{20, 21, 22, 23, 24} {
		m.closeIssue(s)
	}
	if code, out, errOut := m.mission(m.person, "", "show", 889); code != 0 || !strings.Contains(out, "MS1       building: 5 of 6 merged; the gate runs when its last story merges") {
		t.Fatalf("show with #25 open: exit %d: %s%s", code, out, errOut)
	}
	m.closeIssue(25)
	if code, out, errOut := m.mission(m.person, "", "show", 889); code != 0 || !strings.Contains(out, "MS1       gate: its last story merged, so its gate runs") {
		t.Fatalf("show with every story merged: exit %d: %s%s", code, out, errOut)
	}
}

// Criterion 4: the Tatkal Feature #920 of Release #906 has no Feature-done
// milestone, and its mission is complete only when the Release is ready to
// promote and publish, even when every story merged.
func TestMissionPlanTatkalHasNoFeatureDoneMilestone(t *testing.T) {
	m := newMissionWorld(t)
	m.issue(920, "Tatkal of v0.4.0", "feature", 0, "")
	m.issue(930, "Story 930", "story", 920, "")
	m.feature(920, 930)
	m.tatkal[920] = 906
	if code, out, errOut := m.approve(920); code != 0 || !strings.Contains(errOut, "complete when Release #906 is ready to promote and publish") {
		t.Fatalf("approve 920: exit %d: %s%s", code, out, errOut)
	}
	p, raw := m.plan(920)
	if p.Release != 906 || p.FeatureDone != "" || strings.Contains(raw, "feature_done") || p.CompleteWhen != "Release #906 is ready to promote and publish" {
		t.Fatalf("Tatkal plan.json:\n%s", raw)
	}
	m.closeIssue(930)
	if code, out, errOut := m.mission(m.person, "", "show", 920); code != 0 || !strings.Contains(out, "complete  when Release #906 is ready to promote and publish") {
		t.Fatalf("show 920: exit %d: %s%s", code, out, errOut)
	}
	// A Feature that is not a Tatkal Feature ends at its last milestone's gate.
	m.issue(889, "Agents pull the next story", "feature", 0, "")
	m.issue(20, "Story 20", "story", 889, "")
	m.feature(889, 20)
	if code, out, errOut := m.approve(889); code != 0 {
		t.Fatalf("approve 889: exit %d: %s%s", code, out, errOut)
	}
	if p, raw := m.plan(889); p.Release != 0 || p.FeatureDone != "MS1" || p.CompleteWhen != "MS1 passes its gate" {
		t.Fatalf("plan.json of #889:\n%s", raw)
	}
}

// Criterion 5: show prints each milestone with its stories and state, the
// agents, the budget used and left, and the estimate, each with its source;
// a paused mission prints one row "paused: <reason>", never "quiet".
func TestMissionShowPrintsSourcesAndPausedRow(t *testing.T) {
	m := newMissionWorld(t)
	m.scenario889(nil)
	m.edit(func() { m.g.Issues[21].Labels = append(m.g.Issues[21].Labels, fake.Label{Name: "status:in-progress"}) })
	m.est = &CostEstimate{USD: 420, LowUSD: 360, HighUSD: 660, Source: "#886"}
	if code, out, errOut := m.approve(889, "--milestones", "MS1=#20,#21 MS2=#22,#23"); code != 0 {
		t.Fatalf("approve: exit %d: %s%s", code, out, errOut)
	}
	if code, out, errOut := m.mission(m.person, "rahulnakmol", "start", 889); code != 0 {
		t.Fatalf("start: exit %d: %s%s", code, out, errOut)
	}
	m.closeIssue(20)
	code, out, errOut := m.mission(m.person, "", "show", 889)
	for _, want := range []string{
		"running: started by @rahulnakmol at ",
		"MS1       building: 1 of 2 merged; the gate runs when its last story merges · #20 merged, #21 in-progress",
		"MS2       building: 0 of 2 merged; the gate runs when its last story merges · #22 open, #23 open",
		"agents    5 (Decision #915 (5 at once))",
		"budget    $660 (the top of #886's range $360–$660 (estimate $420), locked at Start)",
		"used      $60 (1 merged x $60 ([budget] per_story_usd), until #872 measures spend)",
		"left      $600 (budget less used)",
		"estimate  8 runs (runs ≥ 4 stories + 2 × 2 milestones = 8, at Approve to build)",
	} {
		if code != 0 || !strings.Contains(out, want) {
			t.Fatalf("show: want %q: exit %d: %s%s", want, code, out, errOut)
		}
	}
	// A stop (#1002) records a pause: one row with its reason.
	p, _ := m.plan(889)
	p.Pause = &MissionPause{Reason: "a migration needs a person", By: "rahulnakmol", At: "2026-10-26T12:00:00Z"}
	data, _ := json.MarshalIndent(p, "", "  ")
	putLedger(t, m.bare, MissionPlanPath(889), string(data))
	code, out, errOut = m.mission(m.person, "", "show", 889)
	if code != 0 || strings.Count(out, "paused: ") != 1 || !strings.Contains(out, "\npaused: a migration needs a person\n") || strings.Contains(out, "quiet") || strings.Contains(out, "running") {
		t.Fatalf("show of a paused mission: exit %d: %s%s", code, out, errOut)
	}
	// --json carries each number with its source.
	code, out, _ = run(t, m.person, m.opts, "mission", "show", "889", "--repo", "o/r", "--json")
	var rep MissionShowReport
	if code != 0 || json.Unmarshal([]byte(out), &rep) != nil || rep.State != "paused: a migration needs a person" || rep.Budget == nil ||
		rep.Budget.Left != (SourcedNumber{Value: 600, Source: "budget less used"}) || rep.Estimate.Value != 8 || rep.Agents.Value != 5 {
		t.Fatalf("show --json: exit %d: %s", code, out)
	}
}

// Criterion 6: once started on cursor, the mission's tool is the plan's: a
// change to the starter's tool, a second Start, and a teammate's read leave
// it; the new tool applies only to a mission he starts later.
func TestMissionToolReadFromPlanNotConfig(t *testing.T) {
	m := newMissionWorld(t)
	m.scenario889(nil)
	m.feature(874, 950)
	for _, f := range []int{889, 874} {
		if code, out, errOut := m.approve(f); code != 0 {
			t.Fatalf("approve %d: exit %d: %s%s", f, code, out, errOut)
		}
	}
	if code, out, errOut := m.mission(m.person, "rahulnakmol", "start", 889); code != 0 {
		t.Fatalf("start 889: exit %d: %s%s", code, out, errOut)
	}
	m.mu.Lock()
	m.tool = "claude" // he sets his tool for this repository to claude
	m.mu.Unlock()
	n, before := m.commits(), git(t, m.bare, "show", "ledger:"+MissionPlanPath(889))
	if code, out, errOut := m.mission(m.person, "rahulnakmol", "start", 889); code != ExitBlocked || m.codes() != 0 || m.commits() != n ||
		!strings.Contains(errOut, "@rahulnakmol started it on cursor. Its tool and budget do not change while it runs.") {
		t.Fatalf("a second start: exit %d, device codes %d: %s%s", code, m.codes(), out, errOut)
	}
	m.g.Viewer = "bob" // a teammate reads the mission
	if code, out, errOut := m.mission(m.person, "", "show", 889); code != 0 || !strings.Contains(out, "tool      cursor (missions/889/plan.json: the tool of @rahulnakmol, who started it)") ||
		git(t, m.bare, "show", "ledger:"+MissionPlanPath(889)) != before {
		t.Fatalf("show 889 after the tool change: exit %d: %s%s", code, out, errOut)
	}
	m.g.Viewer = "rahulnakmol"
	if code, out, errOut := m.mission(m.person, "rahulnakmol", "start", 874); code != 0 {
		t.Fatalf("start 874: exit %d: %s%s", code, out, errOut)
	}
	if p, _ := m.plan(874); p.Tool != "claude" {
		t.Fatalf("#874's tool: %q", p.Tool)
	}
	if p, _ := m.plan(889); p.Tool != "cursor" {
		t.Fatalf("#889's tool: %q", p.Tool)
	}
	// No tool set: Start refuses, and nothing is written.
	m.mu.Lock()
	m.tool = ""
	m.mu.Unlock()
	m.issue(901, "Feature 901", "feature", 0, "")
	m.issue(60, "Story 60", "story", 901, "")
	m.feature(901, 60)
	if code, out, errOut := m.approve(901); code != 0 {
		t.Fatalf("approve 901: exit %d: %s%s", code, out, errOut)
	}
	n = m.commits()
	if code, out, errOut := m.mission(m.person, "rahulnakmol", "start", 901); code != ExitBlocked || m.codes() != 0 || m.commits() != n ||
		!strings.Contains(errOut, "Start records your tool for o/r from your user config, and none is set there. Nothing was started.") {
		t.Fatalf("start with no tool: exit %d: %s%s", code, out, errOut)
	}
}

// Criterion 7: #20 and #21 depend on nothing outside #889, and #22 depends on
// #874's story #950, which has not merged: mission 1 takes #20 and #21, and
// #22 waits for mission 2; #23, which depends on #22, waits with it.
func TestMissionSplitsOnOpenExternalDependsOn(t *testing.T) {
	m := newMissionWorld(t)
	m.scenario889(map[int]string{21: "depends-on #20", 22: "depends-on #950", 23: "depends-on #22"})
	if code, out, errOut := m.approve(889, "--milestones", "MS1=#20,#21 MS2=#22,#23"); code != 0 ||
		!strings.Contains(errOut, "Approving: Mission 1: #20, #21; #22 waits for mission 2: waits on #950 (#874); #23 waits for mission 2: waits on #22") {
		t.Fatalf("approve: exit %d: %s%s", code, out, errOut)
	}
	p, raw := m.plan(889)
	want := []Mission{
		{Number: 1, Stories: []MissionStory{{Number: 20}, {Number: 21}}},
		{Number: 2, Stories: []MissionStory{{Number: 22, Waits: "waits on #950 (#874)"}, {Number: 23, Waits: "waits on #22"}}},
	}
	if fmt.Sprint(p.Missions) != fmt.Sprint(want) || !strings.Contains(raw, `"waits": "waits on #950 (#874)"`) ||
		fmt.Sprint(p.Graph[2].Outside) != fmt.Sprint([]MissionDep{{Number: 950, Feature: 874}}) {
		t.Fatalf("missions in plan.json:\n%s", raw)
	}
	// A depends-on that merged already holds nothing back.
	m.closeIssue(950)
	if code, out, errOut := m.approve(889, "--milestones", "MS1=#20,#21,#22 MS2=#23"); code != 0 {
		t.Fatalf("approve again: exit %d: %s%s", code, out, errOut)
	}
	if p, raw := m.plan(889); len(p.Missions) != 1 || len(p.Missions[0].Stories) != 4 {
		t.Fatalf("missions with #950 merged:\n%s", raw)
	}
}

// Criterion 8: when #950 merges, mission 2 starts with #22 and every waiting
// story whose depends-on outside #889 merged, under the approval and on the
// tool in plan.json, with no new Approve to build; one still held waits for
// mission 3.
func TestMissionTwoStartsWhenDependsOnMerges(t *testing.T) {
	m := newMissionWorld(t)
	m.issue(875, "Another Feature", "feature", 0, "")
	m.issue(951, "Story 951", "story", 875, "")
	m.scenario889(map[int]string{22: "depends-on #950", 23: "depends-on #951"})
	if code, out, errOut := m.approve(889, "--milestones", "MS1=#20,#21 MS2=#22,#23"); code != 0 {
		t.Fatalf("approve: exit %d: %s%s", code, out, errOut)
	}
	if code, out, errOut := m.mission(m.person, "rahulnakmol", "start", 889); code != 0 {
		t.Fatalf("start: exit %d: %s%s", code, out, errOut)
	}
	if code, out, errOut := m.mission(m.person, "", "show", 889); code != 0 || !strings.Contains(out, "mission 2 waits\n          #22 waits on #950 (#874)\n          #23 waits on #951 (#875)\n") {
		t.Fatalf("show before #950 merges: exit %d: %s%s", code, out, errOut)
	}
	m.mu.Lock()
	m.tool = "claude" // his tool changed since he started: mission 2 still runs on the plan's
	m.mu.Unlock()
	n, approved, planned := m.commits(), git(t, m.bare, "show", "ledger:"+ready.ApprovalPath(889)), git(t, m.bare, "show", "ledger:"+MissionPlanPath(889))
	m.closeIssue(950)
	code, out, errOut := m.mission(m.person, "", "show", 889)
	if code != 0 || m.codes() != 0 || !strings.Contains(out, "mission 1 running · #20, #21\n") ||
		!strings.Contains(out, "mission 2 running: started under approval 1 on cursor, with no new Approve to build · #22\n") ||
		!strings.Contains(out, "mission 3 waits\n          #23 waits on #951 (#875)\n") {
		t.Fatalf("show after #950 merged: exit %d: %s%s", code, out, errOut)
	}
	if m.commits() != n || git(t, m.bare, "show", "ledger:"+ready.ApprovalPath(889)) != approved || git(t, m.bare, "show", "ledger:"+MissionPlanPath(889)) != planned {
		t.Fatal("mission 2 wrote a new approval or changed the plan")
	}
	m.closeIssue(951)
	if code, out, errOut := m.mission(m.person, "", "show", 889); code != 0 || !strings.Contains(out, "on cursor, with no new Approve to build · #22, #23\n") || strings.Contains(out, "mission 3") {
		t.Fatalf("show after #951 merged: exit %d: %s%s", code, out, errOut)
	}
}

// Criterion 9: #886 estimates $420, range $360–$660: the start card shows
// the top of the range, Start writes budget 660 with its basis, and the
// budget does not change while the mission runs, not even at a new approval.
func TestMissionBudgetIsTopOfRange(t *testing.T) {
	m := newMissionWorld(t)
	m.scenario889(nil)
	m.est = &CostEstimate{USD: 420, LowUSD: 360, HighUSD: 660, Source: "#886"}
	if code, out, errOut := m.approve(889); code != 0 {
		t.Fatalf("approve: exit %d: %s%s", code, out, errOut)
	}
	code, out, errOut := m.mission(m.person, "rahulnakmol", "start", 889)
	card := "BUDGET $660 · the top of the range, locked when you start"
	if at := strings.Index(errOut, card); code != 0 || at < 0 || at > strings.Index(errOut, "/login/device") {
		t.Fatalf("start: exit %d: %s%s", code, out, errOut)
	}
	p, raw := m.plan(889)
	if p.Budget != 660 || !strings.Contains(raw, `"budget": 660,`) || p.BudgetBasis != "the top of #886's range $360–$660 (estimate $420)" {
		t.Fatalf("plan.json after Start:\n%s", raw)
	}
	// A new estimate, and a new approval of the running mission, leave the budget as Start locked it.
	m.est = &CostEstimate{USD: 500, LowUSD: 400, HighUSD: 900, Source: "#886"}
	if code, out, errOut := m.approve(889, "--milestones", "MS1=#20,#21 MS2=#22,#23"); code != 0 {
		t.Fatalf("approve again: exit %d: %s%s", code, out, errOut)
	}
	if p, raw := m.plan(889); p.Approval != 2 || p.Budget != 660 || p.Tool != "cursor" || p.StartedBy != "rahulnakmol" || len(p.Milestones) != 2 {
		t.Fatalf("plan.json after a new approval:\n%s", raw)
	}
}
