package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
)

// Story #1015: mission time on the missionWorld of Story #997. #886's
// working hours are faked (Options.WorkingHours), the clock reads a fake
// clock, and the Kanha runs in Amp.

// story1015 is a story of a test Feature: its effort and its depends-on.
type story1015 struct {
	n      int
	effort string
	links  string
}

// hours sets #886's read: learned hours per size, and default hours per size.
func (m *missionWorld) hours(learned, defaults map[string]float64) {
	m.opts.WorkingHours = func(context.Context) (*WorkingHours, error) {
		return &WorkingHours{Learned: learned, Default: defaults, DefaultKey: "#886's working-hour default"}, nil
	}
}

// feature1015 adds Feature f with its stories, each with its effort, and
// approves its baselines; the mission runs on amp.
func (m *missionWorld) feature1015(f int, stories ...story1015) {
	m.t.Helper()
	m.issue(f, fmt.Sprintf("Feature %d", f), "feature", 0, "")
	var nums []int
	for _, s := range stories {
		is := m.issue(s.n, fmt.Sprintf("Story %d", s.n), "story", f, s.links)
		m.edit(func() {
			for i, l := range is.Labels {
				if strings.HasPrefix(l.Name, "effort:") {
					is.Labels[i] = fake.Label{ID: l.ID, Name: "effort:" + s.effort}
				}
			}
		})
		nums = append(nums, s.n)
	}
	m.feature(f, nums...)
	m.mu.Lock()
	m.tool = "amp"
	m.mu.Unlock()
}

// chain1015 is a chain of stories from n on, each depending on the one before, with these efforts.
func chain1015(n int, efforts ...string) []story1015 {
	var out []story1015
	for i, e := range efforts {
		s := story1015{n: n + i, effort: e}
		if i > 0 {
			s.links = fmt.Sprintf("depends-on #%d", n+i-1)
		}
		out = append(out, s)
	}
	return out
}

func (m *missionWorld) approved(f int) string {
	m.t.Helper()
	code, out, errOut := m.approve(f)
	if code != 0 {
		m.t.Fatalf("approve %d: exit %d: %s%s", f, code, out, errOut)
	}
	return errOut
}

// Criterion 1: mission 1's longest chain is 13 stories (7 S, 4 M, 2 L) with
// learned hours S 1, M 1.5, L 3: about 19 working hours, with no warning. A
// story off the chain adds nothing: the estimate is the longest chain, not the sum.
func TestEstimateLearnedHours(t *testing.T) {
	m := newMissionWorld(t)
	m.hours(map[string]float64{"S": 1, "M": 1.5, "L": 3}, map[string]float64{"S": 4, "M": 22, "L": 40})
	stories := chain1015(20, "S", "M", "S", "L", "S", "M", "S", "S", "M", "L", "S", "M", "S")
	m.feature1015(889, append(stories, story1015{n: 40, effort: "L"})...)
	errOut := m.approved(889)
	want := "Mission 1 time: about 19 working hours (7 × 1 h + 4 × 1.5 h + 2 × 3 h; source: #886 history)"
	if !strings.Contains(errOut, want) || strings.Contains(errOut, "Warning") || strings.Contains(errOut, "Over 48 h") {
		t.Fatalf("approve 889: want %q and no warning:\n%s", want, errOut)
	}
	p, raw := m.plan(889)
	if e := p.Missions[0].Estimate; e == nil || e.Hours != 19 || e.Learned != 19 || e.Mark != "" || len(e.Chain) != 13 || e.Chain[0] != 20 || e.Chain[12] != 32 {
		t.Fatalf("plan.json estimate:\n%s", raw)
	}
}

// Criterion 2: 4 S stories at a learned 1 h and 1 M with no history, whose
// #886 default is 22 h: about 26 h, and Approve to build warns and goes on.
func TestEstimateDefaultHoursWarns(t *testing.T) {
	m := newMissionWorld(t)
	m.hours(map[string]float64{"S": 1}, map[string]float64{"S": 4, "M": 22})
	m.feature1015(889, chain1015(20, "S", "S", "M", "S", "S")...)
	code, out, errOut := m.approve(889)
	if code != 0 || !strings.Contains(out, "Approved #889 to build") {
		t.Fatalf("approve 889: exit %d: %s%s", code, out, errOut)
	}
	for _, want := range []string{"about 26 h (S: #886 history; M: #886 default)", "Warning: mission 1 about 26 h: over [mission] warn_hours (24)"} {
		if !strings.Contains(errOut, want) {
			t.Fatalf("approve 889: want %q:\n%s", want, errOut)
		}
	}
	if p, raw := m.plan(889); p.Missions[0].Estimate.Hours != 26 || p.Missions[0].Estimate.Mark != "" {
		t.Fatalf("plan.json:\n%s", raw)
	}
}

// Criterion 3: 60 learned hours. Approve to build approves with the
// override line and marks the plan; Start with no reason is refused before
// the device code, and nothing starts.
func TestOver48LearnedNeedsReasonAtStart(t *testing.T) {
	m := newMissionWorld(t)
	m.hours(map[string]float64{"L": 30}, map[string]float64{"L": 40})
	m.feature1015(889, chain1015(20, "L", "L")...)
	errOut := m.approved(889)
	if !strings.Contains(errOut, "Over 48 h: split this Feature, or override with a reason (person only)") {
		t.Fatalf("approve 889:\n%s", errOut)
	}
	if p, raw := m.plan(889); p.Missions[0].Estimate.Mark != "over 48 h" || !strings.Contains(raw, `"mark": "over 48 h"`) {
		t.Fatalf("plan.json:\n%s", raw)
	}
	n := m.commits()
	code, out, errOut := m.mission(m.person, "rahulnakmol", "start", 889)
	if code != ExitBlocked || m.codes() != 0 || m.commits() != n ||
		!strings.Contains(errOut, "#889's estimate is 60 working hours, over 48 h: split this Feature, or start it with --reason") {
		t.Fatalf("start with no reason: exit %d, device codes %d: %s%s", code, m.codes(), out, errOut)
	}
	if p, _ := m.plan(889); p.StartedBy != "" {
		t.Fatal("a refused start started the mission")
	}
}

// Criterion 6: @rahulnakmol starts an over-48 h mission with a reason, which
// the start card shows; an agent that runs it is refused.
func TestMissionStartReason(t *testing.T) {
	m := newMissionWorld(t)
	m.hours(map[string]float64{"L": 30}, nil)
	m.feature1015(889, chain1015(20, "L", "L")...)
	m.approved(889)
	n := m.commits()
	code, _, errOut := m.mission(m.agent, "rahulnakmol", "start", 889, "--reason", "one migration")
	if want := "aayu: mission start is a person's decision and is refused for an agent lane (AMP_THREAD_ID)"; code != ExitBlocked || !strings.Contains(errOut, want) || m.commits() != n || m.codes() != 0 {
		t.Fatalf("agent start: exit %d: %s", code, errOut)
	}
	code, out, errOut := m.mission(m.person, "rahulnakmol", "start", 889, "--reason", "one migration")
	card := `OVERRIDE mission 1's estimate, 60 working hours, over 48 h: you start it anyway, for the reason "one migration"`
	if code != 0 || m.codes() != 1 || !strings.Contains(errOut, card) || strings.Index(errOut, card) > strings.Index(errOut, "/login/device") {
		t.Fatalf("start with a reason: exit %d, device codes %d: %s%s", code, m.codes(), out, errOut)
	}
	p, raw := m.plan(889)
	if p.StartedBy != "rahulnakmol" || len(p.Controls) != 1 || p.Controls[0].Reason != "one migration" || p.Controls[0].By != "rahulnakmol" ||
		p.Controls[0].Override != "mission 1's estimate, 60 working hours, over 48 h" || p.Controls[0].Presence != "device-flow" {
		t.Fatalf("plan.json after the override:\n%s", raw)
	}
}

// Criterion 4: no size on the chain has history, and #886's defaults bring
// it to 220 h: Approve to build warns and approves, marks it no history, and
// Start needs no reason.
func TestOver48DefaultsOnlyWarns(t *testing.T) {
	m := newMissionWorld(t)
	m.hours(nil, map[string]float64{"M": 22})
	m.feature1015(889, chain1015(20, "M", "M", "M", "M", "M", "M", "M", "M", "M", "M")...)
	errOut := m.approved(889)
	if !strings.Contains(errOut, "about 220 h, no history: over [mission] max_hours (48)") || strings.Contains(errOut, "Over 48 h") {
		t.Fatalf("approve 889:\n%s", errOut)
	}
	if p, raw := m.plan(889); p.Missions[0].Estimate.Mark != "no history" || p.Missions[0].Estimate.Learned != 0 {
		t.Fatalf("plan.json:\n%s", raw)
	}
	if code, out, errOut := m.mission(m.person, "rahulnakmol", "start", 889); code != 0 {
		t.Fatalf("start of a no-history estimate: exit %d: %s%s", code, out, errOut)
	}
}

// Criterion 5: 30 learned hours and 40 default hours: a warning, and no
// refusal, because the learned hours alone are under 48.
func TestMixedEstimateRefusesOnLearnedOnly(t *testing.T) {
	m := newMissionWorld(t)
	m.hours(map[string]float64{"S": 10}, map[string]float64{"S": 2, "M": 40})
	m.feature1015(889, chain1015(20, "S", "S", "M", "S")...)
	errOut := m.approved(889)
	if !strings.Contains(errOut, "30 h learned + 40 h default (no history for M)") || !strings.Contains(errOut, "over [mission] max_hours (48)") ||
		strings.Contains(errOut, "Over 48 h") {
		t.Fatalf("approve 889:\n%s", errOut)
	}
	if code, out, errOut := m.mission(m.person, "rahulnakmol", "start", 889); code != 0 {
		t.Fatalf("start of a mixed estimate: exit %d: %s%s", code, out, errOut)
	}
}

// Criterion 13: the plan is 70 h, mission 1 30 h and mission 2 40 h: both
// warn, none is refused, and mission 2's clock starts at 0 h.
func TestLimitsPerMission(t *testing.T) {
	m := newMissionWorld(t)
	m.hours(map[string]float64{"S": 10}, nil)
	m.issue(874, "Feature baselines", "feature", 0, "")
	m.issue(950, "Story 950", "story", 874, "")
	second := chain1015(30, "S", "S", "S", "S")
	second[0].links = "depends-on #950"
	m.feature1015(889, append(chain1015(20, "S", "S", "S"), second...)...)
	errOut := m.approved(889)
	for _, want := range []string{"Warning: mission 1 about 30 h: over [mission] warn_hours (24)", "Warning: mission 2 about 40 h: over [mission] warn_hours (24)"} {
		if !strings.Contains(errOut, want) || strings.Contains(errOut, "Over 48 h") {
			t.Fatalf("approve 889: want %q:\n%s", want, errOut)
		}
	}
	if code, out, errOut := m.mission(m.person, "rahulnakmol", "start", 889); code != 0 {
		t.Fatalf("start: exit %d: %s%s", code, out, errOut)
	}
	// Mission 1 ran 30 h; mission 2 has started nothing.
	t0 := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	m.delegation(20, t0, 0)
	m.opts.Now = func() time.Time { return t0.Add(30 * time.Hour) }
	rep := m.show(889)
	if len(rep.Clock) != 2 || rep.Clock[0].Hours != 30 || rep.Clock[1].Hours != 0 || rep.Clock[1].Limit != 48 || rep.Clock[1].reached() {
		t.Fatalf("clocks: %+v", rep.Clock)
	}
}

// delegation records story n's hand-off at start, with its PR (0: none) and events.
func (m *missionWorld) delegation(n int, start time.Time, pr int, events ...string) {
	m.t.Helper()
	putLedger(m.t, m.bare, fmt.Sprintf("delegate/%d/state.json", n), fmt.Sprintf(`{"story": %d, "receiver": "amp", "mode": "cloud", "task": "T-sub-%d", "status": "working",
		"started_at": %q, "pr": %d, "events": [%s]}`, n, n, start.Format(time.RFC3339), pr, strings.Join(events, ",")))
}

// closedAt closes issue n at a time.
func (m *missionWorld) closedAt(n int, at time.Time) {
	m.edit(func() { m.g.Issues[n].State, m.g.Issues[n].ClosedAt = "closed", at.Format(time.RFC3339) })
}

// checkRun adds a check run on sha from one time to another.
func (m *missionWorld) checkRun(sha, name string, from, to time.Time) {
	m.edit(func() {
		if m.g.CheckRuns == nil {
			m.g.CheckRuns = map[string][]fake.CheckRun{}
		}
		m.g.CheckRuns[sha] = append(m.g.CheckRuns[sha], fake.CheckRun{ID: int64(len(m.g.CheckRuns[sha]) + 1), Name: name, Status: "completed", Conclusion: "success",
			StartedAt: from.Format(time.RFC3339), CompletedAt: to.Format(time.RFC3339)})
	})
}

func (m *missionWorld) show(f int) MissionShowReport {
	m.t.Helper()
	code, out, errOut := run(m.t, m.person, m.opts, "mission", "show", fmt.Sprint(f), "--repo", "o/r", "--json")
	var rep MissionShowReport
	if code != 0 || json.Unmarshal([]byte(out), &rep) != nil {
		m.t.Fatalf("mission show %d: exit %d: %s%s", f, code, out, errOut)
	}
	return rep
}

// startedTwo is Feature #889 with S stories #20, #21 (and more), approved and started on amp.
func startedTwo(t *testing.T, more ...int) *missionWorld {
	m := newMissionWorld(t)
	stories := []story1015{{n: 20, effort: "S"}, {n: 21, effort: "S"}}
	for _, n := range more {
		stories = append(stories, story1015{n: n, effort: "S"})
	}
	m.feature1015(889, stories...)
	m.approved(889)
	if code, out, errOut := m.mission(m.person, "rahulnakmol", "start", 889); code != 0 {
		t.Fatalf("start: exit %d: %s%s", code, out, errOut)
	}
	return m
}

// Criterion 7: mission 1 ran 30 h on the wall clock; for 5 h every running
// story waited on a person (Ready to ship), for 2 h on a usage limit, and
// for 4 h on GitHub CI. show says 19 h of 48 with the parts, and the plan
// keeps each story's working total.
func TestClockPausesOnPersonUsageCI(t *testing.T) {
	m := startedTwo(t)
	t0 := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	at := func(h float64) time.Time { return t0.Add(time.Duration(h * float64(time.Hour))) }
	head := strings.Repeat("a", 40)
	pr := m.g.AddPR("feat: story 20", "Refs #20", "o", "story/20", head, "dev")
	m.delegation(20, t0, pr.Number, fmt.Sprintf(`{"at": %q, "status": "paused", "detail": "Amp's usage limit", "until": %q}`, at(5).Format(time.RFC3339), at(7).Format(time.RFC3339)))
	m.checkRun(head, "build", at(20), at(23))
	m.checkRun(head, "aayudha/inspect", at(21), at(24)) // the CI review overlaps build: 4 h of CI, not 7
	m.edit(func() {                                     // ship's Ready to ship, as the account ship writes as
		c := &fake.Comment{ID: 501, CreatedAt: at(25), Body: ReadyToShipMarker + " head=" + head + " -->\n**Ready to ship.**"}
		c.User.Login = m.g.Viewer
		m.g.Issues[pr.Number].Comments = append(m.g.Issues[pr.Number].Comments, c)
	})
	m.closedAt(20, at(30))
	m.delegation(21, at(1), 0) // #21 works from 1 h to 3 h, while #20 works too
	m.closedAt(21, at(3))
	m.opts.Now = func() time.Time { return at(40) }

	code, out, errOut := run(t, m.person, m.opts, "mission", "show", "889", "--repo", "o/r")
	if want := "clock     mission 1: 19 h of 48 (11 h not counted: 5 h person, 2 h usage limit, 4 h CI)\n"; code != 0 || !strings.Contains(out, want) {
		t.Fatalf("mission show 889: exit %d, want %q:\n%s%s", code, want, out, errOut)
	}
	if code, out, errOut := m.next(889); code != 0 {
		t.Fatalf("mission next 889: exit %d: %s%s", code, out, errOut)
	}
	p, raw := m.plan(889)
	got := map[int]float64{}
	for _, s := range p.Missions[0].Stories {
		got[s.Number] = s.WorkingHours
	}
	if got[20] != 19 || got[21] != 2 || !strings.Contains(raw, `"working_hours": 19`) {
		t.Fatalf("plan.json working hours %v:\n%s", got, raw)
	}
	// A new Approve to build keeps them.
	m.edit(func() { m.g.Issues[889].Title = "Feature 889, renamed" })
	m.approved(889)
	if p, _ := m.plan(889); p.Missions[0].Stories[0].WorkingHours != 19 {
		t.Fatalf("working hours after a new approval: %+v", p.Missions[0].Stories)
	}
}

// Criterion 8: #20 waits on its CI review while #21 still works: the clock runs.
func TestClockRunsWhileOneWorks(t *testing.T) {
	m := startedTwo(t)
	t0 := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	head := strings.Repeat("b", 40)
	pr := m.g.AddPR("feat: story 20", "Refs #20", "o", "story/20", head, "dev")
	m.delegation(20, t0, pr.Number)
	m.checkRun(head, "aayudha/inspect", t0.Add(time.Hour), t0.Add(3*time.Hour))
	m.delegation(21, t0, 0)
	m.closedAt(20, t0.Add(5*time.Hour))
	m.closedAt(21, t0.Add(5*time.Hour))
	m.opts.Now = func() time.Time { return t0.Add(6 * time.Hour) }
	c := m.show(889).Clock[0]
	if c.Hours != 5 || c.NotCounted[waitCI] != 0 {
		t.Fatalf("clock: %+v", c)
	}
	// With #21 gone, the same CI wait pauses it.
	m.closedAt(21, t0.Add(time.Hour))
	if c := m.show(889).Clock[0]; c.Hours != 3 || c.NotCounted[waitCI] != 2 {
		t.Fatalf("clock with #20 alone: %+v", c)
	}
}

// Bug #1169: ship leaves #20 open with status:merged until its release. Its
// run ends when its PR merged, at 4 h, not at the release; #21 still works.
func TestClockEndsAtMergeWhileStoryOpen(t *testing.T) {
	m := startedTwo(t)
	t0 := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	pr := m.g.AddPR("feat: story 20", "Refs #20", "o", "story/20", strings.Repeat("c", 40), "dev")
	m.edit(func() { pr.Merged, pr.State, pr.MergedAt = true, "closed", t0.Add(4*time.Hour) })
	m.delegation(20, t0, pr.Number)
	m.status(20, "merged", "")
	m.delegation(21, t0, 0)
	m.closedAt(21, t0.Add(2*time.Hour))
	m.opts.Now = func() time.Time { return t0.Add(30 * time.Hour) }
	if c := m.show(889).Clock[0]; c.Hours != 4 || len(c.Stories) != 2 || !c.Stories[0].Ended || c.Stories[0].WorkingHours != 4 {
		t.Fatalf("clock: %+v", c)
	}
	// The release closes #20 at 29 h: its run still ended at the merge.
	m.status(20, "released", "")
	m.closedAt(20, t0.Add(29*time.Hour))
	if c := m.show(889).Clock[0]; c.Hours != 4 {
		t.Fatalf("clock after the release closed #20: %+v", c)
	}
	// A failed read of the PR stops show: it never guesses the end.
	m.edit(func() { m.g.Fail = fmt.Sprintf("GET /repos/o/r/pulls/%d", pr.Number) })
	if code, out, errOut := run(t, m.person, m.opts, "mission", "show", "889", "--repo", "o/r"); code != ExitExternal {
		t.Fatalf("show with the PR read failing: exit %d: %s%s", code, out, errOut)
	}
}

// Criterion 9: #20's thread runs .agents/check --full for 25 minutes before
// a push: the minutes count; the CI run after the push does not.
func TestLocalCheckCounts(t *testing.T) {
	m := startedTwo(t)
	t0 := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	head := strings.Repeat("c", 40)
	pr := m.g.AddPR("feat: story 20", "Refs #20", "o", "story/20", head, "dev")
	m.delegation(20, t0, pr.Number, fmt.Sprintf(`{"at": %q, "status": "working", "detail": ".agents/check --full"}`, t0.Format(time.RFC3339)))
	m.checkRun(head, "build", t0.Add(25*time.Minute), t0.Add(35*time.Minute))
	m.closedAt(20, t0.Add(35*time.Minute))
	m.opts.Now = func() time.Time { return t0.Add(time.Hour) }
	c := m.show(889).Clock[0]
	if math.Abs(c.Hours-25.0/60) > 1e-9 || math.Abs(c.NotCounted[waitCI]-10.0/60) > 1e-9 {
		t.Fatalf("clock: %+v", c)
	}
}

// inboxList is the person's `aayu inbox --list --json`.
func (m *missionWorld) inboxList() inboxList {
	m.t.Helper()
	code, out, errOut := run(m.t, m.person, m.opts, "inbox", "--list", "--json", "--repo", "o/r")
	var l inboxList
	if code != 0 || json.Unmarshal([]byte(out), &l) != nil {
		m.t.Fatalf("inbox --list: exit %d: %s%s", code, out, errOut)
	}
	return l
}

func missionCards(l inboxList) []InboxItem {
	return slices.DeleteFunc(slices.Clone(l.Items), func(it InboxItem) bool { return it.Kind != InboxMissionTime })
}

func (m *missionWorld) timeComments(f int) int {
	m.g.Mu.Lock()
	defer m.g.Mu.Unlock()
	return len(slices.DeleteFunc(slices.Clone(m.g.Issues[f].Comments), func(c *fake.Comment) bool { return !strings.Contains(c.Body, MissionTimeMarker) }))
}

// Criterion 10: at 24 h of mission time, one inbox card "#889 has run 24 h",
// with no recommendation, and nothing stops.
func Test24hCard(t *testing.T) {
	m := startedTwo(t)
	t0 := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	m.status(20, "in-progress", "")
	m.delegation(20, t0, 0)
	m.status(21, "ready", "")
	m.opts.Now = func() time.Time { return t0.Add(23 * time.Hour) }
	if code, out, _ := m.next(889); code != 0 || out != "start #21\n" || m.timeComments(889) != 0 {
		t.Fatalf("at 23 h: exit %d: %q, %d cards", code, out, m.timeComments(889))
	}
	m.opts.Now = func() time.Time { return t0.Add(24*time.Hour + time.Minute) }
	for range 2 {
		if code, out, errOut := m.next(889); code != 0 || strings.Contains(out, "max_hours") {
			t.Fatalf("at 24 h: exit %d: %s%s", code, out, errOut)
		}
	}
	cards := missionCards(m.inboxList())
	if m.timeComments(889) != 1 || len(cards) != 1 || cards[0].Title != "#889 has run 24 h" || cards[0].recommended() != nil ||
		!strings.Contains(cards[0].Counsel, "No recommendation: the mission gives none.") {
		t.Fatalf("at 24 h: %d comments, cards %+v", m.timeComments(889), cards)
	}
}

// Criteria 11 and 12: at 48 h, with #20 and #21 running and #25 startable,
// #25 waits and the running stories run on; one inbox card offers an
// extension with a reason and a stop.
func Test48hHoldCardAndWaits(t *testing.T) {
	m := startedTwo(t, 25)
	t0 := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	for _, n := range []int{20, 21} {
		m.status(n, "in-progress", "")
		m.delegation(n, t0, 0)
	}
	m.status(25, "ready", "")
	m.opts.Now = func() time.Time { return t0.Add(48*time.Hour + time.Minute) }
	for range 2 {
		if code, out, errOut := m.next(889); code != 0 || out != "waits #25: #889 reached max_hours (48 h)\n" {
			t.Fatalf("at 48 h: exit %d: %q%s", code, out, errOut)
		}
	}
	if got := m.claims(889); len(got) != 0 {
		t.Fatalf("claims at 48 h: %v", got)
	}
	cards := missionCards(m.inboxList())
	if len(cards) != 1 || m.timeComments(889) != 1 {
		t.Fatalf("cards at 48 h: %+v", cards)
	}
	c := cards[0]
	if c.Title != "#889 reached its 48 h time limit" || c.Why != "new starts wait: the mission of #889 used its 48 working hours. Running stories keep going." ||
		!strings.Contains(c.Counsel, "No recommendation: the mission gives none.") || c.recommended() != nil || len(c.Options) != 2 ||
		c.Options[0].Label != "Extend for 48 h, with a reason" || c.Options[0].Ask == "" || c.Options[1].Label != "Stop the mission" {
		t.Fatalf("the 48 h card: %+v", c)
	}
}

// Criterion 12: an agent's `aayu mission extend` is refused; the person's,
// after the presence check, records the reason, and new starts resume for
// another [mission] max_hours.
func TestMissionExtend(t *testing.T) {
	m := startedTwo(t, 25, 26)
	t0 := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	for _, n := range []int{20, 21} {
		m.status(n, "in-progress", "")
		m.delegation(n, t0, 0)
	}
	m.status(25, "ready", "")
	m.opts.Now = func() time.Time { return t0.Add(48*time.Hour + time.Minute) }
	if code, out, _ := m.next(889); code != 0 || out != "waits #25: #889 reached max_hours (48 h)\nwaits #26: not Ready (open)\n" {
		t.Fatalf("at 48 h: exit %d: %q", code, out)
	}
	n := m.commits()
	code, _, errOut := m.mission(m.agent, "rahulnakmol", "extend", 889, "--reason", "two stories left")
	if want := "aayu: mission extend is a person's decision and is refused for an agent lane (AMP_THREAD_ID)"; code != ExitBlocked || !strings.Contains(errOut, want) ||
		m.commits() != n || m.codes() != 0 {
		t.Fatalf("agent extend: exit %d: %s", code, errOut)
	}
	if code, _, errOut := m.mission(m.person, "rahulnakmol", "extend", 889); code != ExitUsage || m.codes() != 0 {
		t.Fatalf("extend with no reason: exit %d: %s", code, errOut)
	}
	code, out, errOut := m.mission(m.person, "rahulnakmol", "extend", 889, "--reason", "two stories left")
	if code != 0 || m.codes() != 1 || !strings.Contains(out, "@rahulnakmol extended mission 1 of #889 to 96 h: new starts resume. Reason: two stories left") ||
		strings.Index(errOut, `REASON "two stories left"`) > strings.Index(errOut, "/login/device") {
		t.Fatalf("extend: exit %d, device codes %d: %s%s", code, m.codes(), out, errOut)
	}
	p, raw := m.plan(889)
	last := p.Controls[len(p.Controls)-1]
	if last.Action != "extend" || last.Mission != 1 || last.Hours != 48 || last.Reason != "two stories left" || last.By != "rahulnakmol" || last.Presence != "device-flow" {
		t.Fatalf("plan.json after extend:\n%s", raw)
	}
	if code, out, _ := m.next(889); code != 0 || out != "start #25\nwaits #26: not Ready (open)\n" {
		t.Fatalf("after extend: exit %d: %q", code, out)
	}
	if cards := missionCards(m.inboxList()); len(cards) != 0 {
		t.Fatalf("cards after extend: %+v", cards)
	}
	// Another account with write access posted the 48 h limit card before the person extended: the newest card,
	// the extension, is the mission's state, so the inbox shows no card.
	m.edit(func() {
		c := &fake.Comment{ID: 7100, UpdatedAt: time.Now().Add(-time.Hour), Body: missionCard(1, "limit", 48, 48, "limit")}
		c.User.Login = "maintainer"
		if m.g.Permissions == nil {
			m.g.Permissions = map[string]string{}
		}
		m.g.Permissions["maintainer"] = "write"
		m.g.Issues[889].Comments = append(m.g.Issues[889].Comments, c)
	})
	if cards := missionCards(m.inboxList()); len(cards) != 0 {
		t.Fatalf("an older card by another account outlived the extension: %+v", cards)
	}
	// At 96 h it holds again, under its extended limit.
	m.status(25, "in-progress", "")
	m.status(26, "ready", "")
	m.opts.Now = func() time.Time { return t0.Add(96*time.Hour + time.Minute) }
	if code, out, _ := m.next(889); code != 0 || out != "waits #26: #889 reached its extended time limit (96 h)\n" {
		t.Fatalf("at 96 h: exit %d: %q", code, out)
	}
	// The extension keeps the 48 h the person approved when max_hours later becomes 24: 24 + 48, not 2 × 24.
	code, out, errOut = run(t, m.person, m.opts, "mission", "show", "889", "--repo", "o/r", "--json", "--set", "mission.max_hours=24")
	var rep MissionShowReport
	if code != 0 || json.Unmarshal([]byte(out), &rep) != nil || rep.Clock[0].Limit != 72 {
		t.Fatalf("the limit after max_hours changed: exit %d: %s%s", code, out, errOut)
	}
}

// The 48 h card's choices from the person's inbox: Stop waits for #1002 and
// changes nothing; Extend asks for the reason, then the device code, and
// extends; the 24 h card is marked seen and leaves the inbox.
func TestMissionCardsInInbox(t *testing.T) {
	m := startedTwo(t, 25)
	t0 := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	m.status(20, "in-progress", "")
	m.delegation(20, t0, 0)
	m.status(21, "ready", "")
	m.status(25, "ready", "")
	m.opts.IsTerminal = func(any) bool { return true }
	m.opts.Now = func() time.Time { return t0.Add(25 * time.Hour) }
	if code, out, errOut := m.next(889); code != 0 || out != "start #21\nstart #25\n" {
		t.Fatalf("at 25 h: exit %d: %q%s", code, out, errOut)
	}
	code, out, errOut := runIn(t, m.person, m.opts, "a y q", "inbox", "--repo", "o/r")
	if code != 0 || !strings.Contains(out, "Mission time #889: #889 has run 24 h") || !strings.Contains(out, "done: mark it seen") || len(missionCards(m.inboxList())) != 0 {
		t.Fatalf("seen: exit %d: %s%s", code, out, errOut)
	}
	if code, out, _ := run(t, m.person, m.opts, "inbox", "--list", "--repo", "o/r"); code != 0 || strings.Contains(out, "has run 24 h") {
		t.Fatalf("list after seen: %s", out)
	}

	m.opts.Now = func() time.Time { return t0.Add(48*time.Hour + time.Minute) }
	m.status(21, "in-progress", "")
	m.status(25, "in-progress", "")
	if code, out, errOut := m.next(889); code != 0 || out != "Nothing to list: every open story and Bug of #889 runs, or none is open.\n" {
		t.Fatalf("at 48 h: exit %d: %q%s", code, out, errOut)
	}
	if code, out, _ := run(t, m.person, m.opts, "inbox", "--list", "--repo", "o/r"); code != 0 || !strings.Contains(out, "reached its 48 h time limit") {
		t.Fatalf("list at 48 h: %s", out)
	}
	m.device("rahulnakmol")
	code, out, errOut = runIn(t, m.person, m.opts, "1two stories left\nq", "inbox", "--repo", "o/r")
	if p, raw := m.plan(889); code != 0 || p.Controls[len(p.Controls)-1].Reason != "two stories left" || len(missionCards(m.inboxList())) != 0 {
		t.Fatalf("extend from the inbox: exit %d: %s%s\n%s", code, out, errOut, raw)
	}
	// At the next limit, Stop the mission asks for the reason, then the device code, and stops it (#1002).
	m.opts.Now = func() time.Time { return t0.Add(96*time.Hour + time.Minute) }
	if code, out, errOut := m.next(889); code != 0 {
		t.Fatalf("at 96 h: exit %d: %q%s", code, out, errOut)
	}
	m.device("rahulnakmol")
	code, out, errOut = runIn(t, m.person, m.opts, "2out of time\nq", "inbox", "--repo", "o/r")
	if p, raw := m.plan(889); code != 0 || p.Pause == nil || p.Pause.Reason != "out of time" || len(missionCards(m.inboxList())) != 0 {
		t.Fatalf("stop from the inbox: exit %d: %s%s\n%s", code, out, errOut, raw)
	}
}

// Refusals and failures of the clock and extend: each exits with nothing
// written, and a failed GitHub read is exit 5.
func TestMissionTimeRefusals(t *testing.T) {
	m := startedTwo(t)
	t0 := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	head := strings.Repeat("d", 40)
	pr := m.g.AddPR("feat: story 20", "Refs #20", "o", "story/20", head, "dev")
	m.delegation(20, t0, pr.Number)
	m.checkRun(head, "build", t0.Add(time.Hour), t0.Add(2*time.Hour))
	m.opts.Now = func() time.Time { return t0.Add(10 * time.Hour) }
	n := m.commits()
	if code, _, errOut := m.mission(m.person, "rahulnakmol", "extend", 889, "--reason", "early"); code != ExitBlocked || m.codes() != 0 ||
		!strings.Contains(errOut, "No mission of #889 reached its time limit: nothing to extend (mission 1 9 h of 48 (1 h not counted: 1 h CI))") {
		t.Fatalf("extend before the limit: exit %d: %s", code, errOut)
	}
	for _, path := range []string{"GET /repos/o/r/issues/20", fmt.Sprintf("GET /repos/o/r/issues/%d/comments", pr.Number),
		fmt.Sprintf("GET /repos/o/r/pulls/%d/commits", pr.Number), "GET /repos/o/r/commits/" + head + "/check-runs"} {
		m.edit(func() { m.g.Fail, m.g.FailAfter = path, 0 })
		if code, out, errOut := m.next(889); code != ExitExternal || out != "" {
			t.Fatalf("%s failing: next exit %d: %s%s", path, code, out, errOut)
		}
		m.edit(func() { m.g.Fail, m.g.FailAfter = path, 0 })
		if code, _, errOut := run(t, m.person, m.opts, "mission", "show", "889", "--repo", "o/r"); code != ExitExternal {
			t.Fatalf("%s failing: show exit %d: %s", path, code, errOut)
		}
	}
	m.edit(func() { m.g.Fail = "" })
	if m.commits() != n {
		t.Fatal("a refusal wrote the ledger")
	}

	// At the limit: a dry run writes nothing, a card that fails to post is exit 5, and a limit that moves while
	// the code waits extends nothing.
	m.opts.Now = func() time.Time { return t0.Add(60 * time.Hour) }
	m.edit(func() { m.g.Fail, m.g.FailAfter = "POST /repos/o/r/issues/889/comments", 0 })
	if code, _, errOut := m.next(889); code != ExitExternal {
		t.Fatalf("the card failing: exit %d: %s", code, errOut)
	}
	m.edit(func() { m.g.Fail = "" })
	if code, out, errOut := m.mission(m.person, "rahulnakmol", "extend", 889, "--reason", "r", "--dry-run"); code != 0 || m.commits() != n ||
		!strings.Contains(out, "--dry-run: ledger missions/889/plan.json planned, not written") {
		t.Fatalf("extend --dry-run: exit %d: %s%s", code, out, errOut)
	}
	m.opts.Now = func() time.Time { return t0.Add(100 * time.Hour) } // still past 96 h after the extension it races with
	m.edit(func() {
		m.g.Device = &fake.DeviceFlow{ClientID: DefaultDeviceClientID, Approver: "rahulnakmol", Outcome: "approve",
			OnPoll: func(int) {
				m.setPlan(889, func(p *MissionPlan) { p.Controls = append(p.Controls, MissionControl{Action: "extend", Mission: 1}) })
			}}
	})
	if code, _, errOut := run(t, m.person, m.opts, "mission", "extend", "889", "--repo", "o/r", "--reason", "r"); code != ExitBlocked ||
		!strings.Contains(errOut, "changed while the code was waiting. Nothing was extended.") {
		t.Fatalf("a limit change during the code: exit %d: %s", code, errOut)
	}
	m.setPlan(889, func(p *MissionPlan) { p.Controls = p.Controls[:1] })
	race, pushed := m.opts, false
	race.LedgerAttempt = func(int, string) {
		if !pushed {
			pushed = true
			putLedger(t, m.bare, "specs/889/other.json", "{}")
		}
	}
	m.device("rahulnakmol")
	if code, _, errOut := run(t, m.person, race, "mission", "extend", "889", "--repo", "o/r", "--reason", "r"); code != ExitBlocked ||
		!strings.Contains(errOut, "The ledger changed while aayu wrote the extension of #889. Nothing was extended. Run it again.") {
		t.Fatalf("a ledger write during extend: exit %d: %s", code, errOut)
	}
	m.opts.WorkingHours = func(context.Context) (*WorkingHours, error) { return nil, fmt.Errorf("the history is unreadable") }
	m.edit(func() { m.g.Issues[889].Title = "renamed" })
	if code, _, errOut := m.approve(889); code != ExitExternal || !strings.Contains(errOut, "the history is unreadable") {
		t.Fatalf("#886's read failing: exit %d: %s", code, errOut)
	}
	// Not started: nothing to extend.
	w := newMissionWorld(t)
	w.feature1015(889, story1015{n: 20, effort: "S"})
	w.approved(889)
	if code, _, errOut := w.mission(w.person, "rahulnakmol", "extend", 889, "--reason", "r"); code != ExitBlocked || !strings.Contains(errOut, "has not started: nothing to extend") {
		t.Fatalf("extend before Start: exit %d: %s", code, errOut)
	}
}

// The clock's and the estimate's edges, on their own: a gap with no story
// running, a paused mission, a check run still running, a size #886 does not
// know, and a depends-on cycle.
func TestMissionTimeEdges(t *testing.T) {
	t0 := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	at := func(h float64) time.Time { return t0.Add(time.Duration(h * float64(time.Hour))) }
	hours, wall, not, working := clockOf([]storyRun{
		{n: 20, start: at(0), end: at(2)},
		{n: 21, start: at(3), end: at(10), waits: []clockWait{{waitCI, at(4), at(6)}, {waitPerson, at(5), at(8)}}},
	})
	if hours != 5 || wall != 10 || not[waitIdle] != 1 || not[waitCI] != 1 || not[waitPerson] != 3 || working[21] != 3 {
		t.Fatalf("clock: %v of %v, not counted %v, working %v", hours, wall, not, working)
	}
	graph := []MissionItem{{Number: 1, Effort: "XXL", DependsOn: []int{2}}, {Number: 2, Effort: "S", DependsOn: []int{1}}, {Number: 3, Effort: "S", DependsOn: []int{9}}}
	est := estimateMission(graph, Mission{Number: 1, Stories: []MissionStory{{Number: 1}, {Number: 2}, {Number: 3}}},
		&WorkingHours{Learned: map[string]float64{"S": 1}, Default: map[string]float64{"XXL": 50}}, func(int) bool { return false }, 48)
	if est.Hours != 51 || est.Mark != markNoHistory || !strings.Contains(est.Basis, "XXL: #886 default") {
		t.Fatalf("estimate: %+v", est)
	}

	// A paused mission waits on a person from its pause; a check run still running waits until now.
	m := startedTwo(t)
	head := strings.Repeat("e", 40)
	pr := m.g.AddPR("feat: story 20", "Refs #20", "o", "story/20", head, "dev")
	m.delegation(20, t0, pr.Number)
	m.edit(func() {
		m.g.CheckRuns = map[string][]fake.CheckRun{head: {{ID: 1, Name: "build", Status: "in_progress", StartedAt: at(6).Format(time.RFC3339)}}}
	})
	m.setPlan(889, func(p *MissionPlan) { // stopped at 8 h, after a stop of #20 alone from 1 h to 2 h that a resume ended
		p.Pause = &MissionPause{Reason: "stopped", At: at(8).Format(time.RFC3339)}
		p.Controls = append(p.Controls, MissionControl{Action: "stop", Story: 20, At: at(1).Format(time.RFC3339)},
			MissionControl{Action: "resume", Story: 20, At: at(2).Format(time.RFC3339)}, MissionControl{Action: "stop", Story: 21, At: at(0).Format(time.RFC3339)},
			MissionControl{Action: "stop", At: at(8).Format(time.RFC3339)})
	})
	m.opts.Now = func() time.Time { return at(10) }
	if c := m.show(889).Clock[0]; c.Hours != 5 || c.NotCounted[waitCI] != 2 || c.NotCounted[waitPerson] != 3 {
		t.Fatalf("paused clock: %+v", c)
	}
	// A reason at Start with no override is recorded as it is.
	r := newMissionWorld(t)
	r.hours(map[string]float64{"S": 1}, nil)
	r.feature1015(889, story1015{n: 20, effort: "S"})
	r.approved(889)
	if code, _, errOut := r.mission(r.person, "rahulnakmol", "start", 889, "--reason", "first run"); code != 0 || !strings.Contains(errOut, `REASON "first run"`) {
		t.Fatalf("start with a reason: exit %d: %s", code, errOut)
	}
	if code, out, _ := run(t, r.person, r.opts, "mission", "show", "889", "--repo", "o/r"); code != 0 ||
		!strings.Contains(out, "          mission 1: about 1 working hours (1 × 1 h; source: #886 history)\n") {
		t.Fatalf("show: %s", out)
	}
}

// When both missions reach their limits, the inbox shows mission 2's card,
// and Extend there extends mission 2, the mission the person saw.
func TestExtendFromCardExtendsItsMission(t *testing.T) {
	m := newMissionWorld(t)
	m.hours(map[string]float64{"S": 1}, nil)
	m.issue(874, "Feature baselines", "feature", 0, "")
	m.issue(950, "Story 950", "story", 874, "")
	m.feature1015(889, story1015{n: 20, effort: "S"}, story1015{n: 30, effort: "S", links: "depends-on #950"})
	m.approved(889)
	if code, out, errOut := m.mission(m.person, "rahulnakmol", "start", 889); code != 0 {
		t.Fatalf("start: exit %d: %s%s", code, out, errOut)
	}
	t0 := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	m.delegation(20, t0, 0)
	m.delegation(30, t0, 0)
	m.opts.Now = func() time.Time { return t0.Add(49 * time.Hour) }
	if code, out, errOut := m.next(889); code != 0 {
		t.Fatalf("next: exit %d: %s%s", code, out, errOut)
	}
	cards := missionCards(m.inboxList())
	if len(cards) != 1 || cards[0].f.mission != 0 && cards[0].f.mission != 2 {
		t.Fatalf("cards: %+v", cards)
	}
	m.opts.IsTerminal = func(any) bool { return true }
	m.device("rahulnakmol")
	code, out, errOut := runIn(t, m.person, m.opts, "1two stories left\nq", "inbox", "--repo", "o/r")
	p, raw := m.plan(889)
	if last := p.Controls[len(p.Controls)-1]; code != 0 || last.Action != "extend" || last.Mission != 2 {
		t.Fatalf("extend from mission 2's card: exit %d: %s%s\n%s", code, out, errOut, raw)
	}
}

// CodeRabbit on 8f03a76: a released claim starts no run, a look-alike card by
// an account without write access neither holds back aayu's card nor is
// edited, and an ended story's run is read from the plan, not from GitHub.
func TestMissionClockTrustsOnlyItsRecords(t *testing.T) {
	m := startedTwo(t)
	t0 := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	// #21 was claimed at 0 h and released: only #20, from 30 h, runs.
	putLedger(t, m.bare, MissionClaimsPath(889), fmt.Sprintf(`{"story": 21, "token": "a", "at": %q}`+"\n"+`{"story": 21, "token": "a", "at": %q, "release": true}`+"\n",
		t0.Format(time.RFC3339), t0.Add(time.Minute).Format(time.RFC3339)))
	pr := m.g.AddPR("feat: story 20", "Refs #20", "o", "story/20", strings.Repeat("f", 40), "dev")
	m.delegation(20, t0.Add(30*time.Hour), pr.Number)
	// Someone without write access posts ship's Ready to ship marker at 35 h: it is no wait on a person.
	m.edit(func() {
		c := &fake.Comment{ID: 7000, CreatedAt: t0.Add(35 * time.Hour), Body: ReadyToShipMarker + " head=" + strings.Repeat("f", 40) + " -->\n**Ready to ship.**"}
		c.User.Login = "mallory"
		m.g.Issues[pr.Number].Comments = append(m.g.Issues[pr.Number].Comments, c)
	})
	m.opts.Now = func() time.Time { return t0.Add(55 * time.Hour) }
	if c := m.show(889).Clock[0]; c.Hours != 25 || len(c.Stories) != 1 {
		t.Fatalf("a released claim ran, or a look-alike Ready to ship paused the clock: %+v", c)
	}
	// aayu's App's own Ready to ship marker, at 45 h, is a person wait once [identity] app_slug names the App.
	m.edit(func() {
		app := &fake.Comment{ID: 7003, CreatedAt: t0.Add(45 * time.Hour), Body: ReadyToShipMarker + " head=" + strings.Repeat("f", 40) + " -->\n**Ready to ship.**"}
		app.User.Login, app.User.Type = "aayudha-bot[bot]", "Bot"
		m.g.Issues[pr.Number].Comments = append(m.g.Issues[pr.Number].Comments, app)
	})
	if c := m.show(889).Clock[0]; c.Hours != 25 {
		t.Fatalf("an unnamed App's marker paused the clock: %+v", c)
	}
	code, out, errOut := run(t, m.person, m.opts, "mission", "show", "889", "--repo", "o/r", "--json", "--set", "identity.app_slug=aayudha-bot")
	var rep MissionShowReport
	if code != 0 || json.Unmarshal([]byte(out), &rep) != nil || rep.Clock[0].Hours != 15 {
		t.Fatalf("aayu's App's Ready to ship is a person wait: exit %d: %s%s", code, out, errOut)
	}
	// Someone without write access, and an account GitHub answers 404 for, post look-alike warning cards: aayu
	// still posts its own, leaves theirs, and fails on neither.
	fakeBody := missionCard(1, "warn", 24, 48, "fake")
	m.edit(func() {
		c := &fake.Comment{ID: 7001, Body: fakeBody}
		c.User.Login = "mallory"
		gone := &fake.Comment{ID: 7002, Body: fakeBody}
		gone.User.Login = "gone"
		if m.g.Permissions == nil {
			m.g.Permissions = map[string]string{}
		}
		m.g.Permissions["gone"] = "not-found"
		m.g.Issues[889].Comments = append(m.g.Issues[889].Comments, c, gone)
	})
	if code, out, errOut := m.next(889); code != 0 {
		t.Fatalf("next: exit %d: %s%s", code, out, errOut)
	}
	if cards := missionCards(m.inboxList()); m.timeComments(889) != 3 || len(cards) != 1 || cards[0].Title != "#889 has run 24 h" {
		t.Fatalf("cards with look-alikes: %d comments, %+v", m.timeComments(889), cards)
	}
	if m.g.Issues[889].Comments[len(m.g.Issues[889].Comments)-3].Body != fakeBody {
		t.Fatal("aayu edited a look-alike card")
	}
	// #20 ends: next records its run, and later clocks read no issue of it.
	m.closedAt(20, t0.Add(40*time.Hour))
	if code, out, errOut := m.next(889); code != 0 {
		t.Fatalf("next after #20 ended: exit %d: %s%s", code, out, errOut)
	}
	p, raw := m.plan(889)
	if s := p.Missions[0].Stories[0]; s.Number != 20 || s.WorkingHours != 10 || s.Run == nil || s.Run.End != t0.Add(40*time.Hour).Format(time.RFC3339) {
		t.Fatalf("plan.json after #20 ended:\n%s", raw)
	}
	m.edit(func() { m.g.Fail, m.g.FailAfter = fmt.Sprintf("GET /repos/o/r/pulls/%d/commits", pr.Number), 0 }) // read only while #20 runs
	if c := m.show(889).Clock[0]; c.Hours != 10 {
		t.Fatalf("clock from the record: %+v", c)
	}
}
