package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
)

// Story #1178: the overnight card, on the evening of Story #1176.

// overnightWorld is the evening of eveningWorld, Monday 2 November 2026 at
// 18:00, with the Features of two releases: #889 (approved to build), #982 and
// #990 in v0.4.0, and #975 in v0.4.1. #982 is next in the release order: #975
// has a lower number, but its release comes later.
func overnightWorld(t *testing.T) (*missionWorld, *time.Time) {
	t.Helper()
	m, now := eveningWorld(t)
	for _, f := range []int{982, 990, 975} {
		m.issue(f, fmt.Sprintf("Feature %d", f), "feature", 0, "")
	}
	for f, v := range map[int]string{889: "v0.4.0", 982: "v0.4.0", 990: "v0.4.0", 975: "v0.4.1"} {
		m.edit(func() {
			m.g.Issues[f].Milestone = &fake.Milestone{Number: map[string]int{"v0.4.0": 1, "v0.4.1": 2}[v], Title: v, State: "open"}
		})
	}
	m.device("rahulnakmol")
	return m, now
}

// reviewAll records today's review of every Evening card as the inbox would:
// the evening review is done.
func (m *missionWorld) reviewAll() {
	m.t.Helper()
	var lines []string
	for _, it := range eveningList(m.t, m, m.person) {
		b, _ := json.Marshal(StoryReview{Story: it.Number, PR: it.PR, Head: it.Head, Pack: it.Pack, Verdict: ReviewAccepted, By: "rahulnakmol", At: "2026-11-02T18:00:00Z"})
		lines = append(lines, string(b))
	}
	putLedger(m.t, m.bare, MissionReviewsPath(889), strings.Join(lines, "\n")+"\n")
}

// overnightCards are env's Overnight cards in `aayu inbox --list --json`.
func overnightCards(t *testing.T, m *missionWorld, env map[string]string) []InboxItem {
	t.Helper()
	code, out, errOut := run(t, env, m.opts, "inbox", "--list", "--json", "--repo", "o/r")
	var l inboxList
	if code != 0 || json.Unmarshal([]byte(out), &l) != nil {
		t.Fatalf("inbox --list: exit %d: %s%s", code, out, errOut)
	}
	return slices.DeleteFunc(l.Items, func(it InboxItem) bool { return it.Kind != InboxOvernight })
}

// overnightPlan reads missions/overnight/<date>.json from the ledger; ok is false when there is none.
func (m *missionWorld) overnightPlan(date string) (OvernightPlan, bool) {
	raw, err := exec.Command("git", "-C", m.bare, "show", "ledger:"+OvernightPath(date)).Output()
	var p OvernightPlan
	if err != nil {
		return p, false
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		m.t.Fatalf("%s: %v\n%s", OvernightPath(date), err, raw)
	}
	return p, true
}

// nightTick runs the hourly tick as the build-ready loop's agent lane, on the world's clock: its text, then its report as a dry run.
func (m *missionWorld) nightTick() (TickReport, string) {
	m.t.Helper()
	code, out, errOut := run(m.t, m.lane, m.opts, "mission", "tick", "--repo", "o/r")
	if code != 0 {
		m.t.Fatalf("mission tick: exit %d: %s%s", code, out, errOut)
	}
	_, js, errOut := run(m.t, m.lane, m.opts, "mission", "tick", "--repo", "o/r", "--json", "--dry-run")
	var rep TickReport
	if err := json.Unmarshal([]byte(js), &rep); err != nil {
		m.t.Fatalf("mission tick --json: %v: %s%s", err, js, errOut)
	}
	return rep, out
}

// Criteria 1 and 2: once the evening review is done, the Overnight card
// proposes authoring #982, the next Feature in the release order, the verify
// sweep, and the evals, each with its bounds from config; his approval writes
// missions/overnight/2026-11-02.json with each run, its bounds, and its end at
// the next start of [time] business_hours, under the session's one code.
func TestOvernightCardProposesAndRecords(t *testing.T) {
	m, _ := overnightWorld(t)
	if cards := overnightCards(t, m, m.person); len(cards) != 0 {
		t.Fatalf("an Overnight card before the evening review is done: %+v", cards)
	}

	// He reviews the three evening cards; the Overnight card follows in the same session, and he leaves it there.
	code, out, errOut := runIn(t, m.person, m.opts, "1y1y1not built today, try again tomorrow\nq", "inbox", "--repo", "o/r")
	tonight := "] Overnight #982: Tonight: Feature 982\n" +
		"  " + inboxKinds[InboxOvernight].Line + "\n" +
		"  Tonight, until Tue 3 Nov 09:00 Europe/London, the next start of [time] business_hours. Nothing else starts but the loops scheduled before.\n" +
		"  1. author #982 to the approval line: an agent drafts each part of its baseline (`aayu ready 982 --part <part> --request`); drafts only, every approval stays in the morning\n" +
		"     Bound: 8 h, 60 USD and the start of the day, Tue 3 Nov 09:00 Europe/London.\n" +
		"  2. the verify sweep: every feature-map behaviour check on the dev tip (`aayu verify --check all`)\n" +
		"     Bound: 8 h, 60 USD and the start of the day, Tue 3 Nov 09:00 Europe/London.\n" +
		"  3. the evals: the tier-2 evals of the dev tip (evals.yml)\n" +
		"     Bound: 8 h, 60 USD and the start of the day, Tue 3 Nov 09:00 Europe/London.\n"
	if code != 0 || len(m.reviews()) != 3 || !strings.Contains(out, tonight) ||
		!strings.Contains(out, "  1. Approve tonight's 3 runs, each within its bounds (preselected)\n  2. Not now: nothing runs tonight\n  3. Open #982 on GitHub\n") {
		t.Fatalf("inbox after the evening review: exit %d, reviews %+v:\n%s%s", code, m.reviews(), out, errOut)
	}
	if _, ok := m.overnightPlan("2026-11-02"); ok || m.codes() != 0 {
		t.Fatalf("left on the card: a plan, or %d codes", m.codes())
	}

	// The list shows the runs and their bounds, and never the approval: only a person approves, in `aayu inbox`.
	cards := overnightCards(t, m, m.person)
	ends := "2026-11-03T09:00:00Z" // 09:00 in London, on GMT
	if len(cards) != 1 || cards[0].Number != 982 || len(cards[0].Runs) != 3 || slices.ContainsFunc(cards[0].Options, func(a InboxAction) bool { return a.Command == overnightApprove }) {
		t.Fatalf("Overnight cards: %+v", cards)
	}
	for i, want := range []OvernightRun{{Run: RunAuthor, Feature: 982}, {Run: RunSweep}, {Run: RunEvals}} {
		if r := cards[0].Runs[i]; r.Run != want.Run || r.Feature != want.Feature || r.Hours != 8 || r.CostUSD != 60 || r.Ends != ends || r.Started != "" {
			t.Fatalf("run %d: %+v", i+1, r)
		}
	}
	// The bounds come from config: [autonomy.budget] wall_hours, here from the environment layer.
	m.person["AAYU_AUTONOMY__BUDGET__WALL_HOURS"] = "4"
	if cards := overnightCards(t, m, m.person); len(cards) != 1 || cards[0].Runs[0].Hours != 4 {
		t.Fatalf("Overnight card with wall_hours = 4: %+v", cards)
	}
	delete(m.person, "AAYU_AUTONOMY__BUDGET__WALL_HOURS")

	// He approves it: one device code covers the session, and the plan records each run, its bounds, and its end.
	code, out, errOut = runIn(t, m.person, m.opts, "1y", "inbox", "--repo", "o/r")
	if code != 0 || !strings.Contains(out, "done: @rahulnakmol set tonight's runs (author #982, verify-sweep, evals), recorded in missions/overnight/2026-11-02.json") {
		t.Fatalf("inbox approve: exit %d:\n%s%s", code, out, errOut)
	}
	p, ok := m.overnightPlan("2026-11-02")
	if !ok || p.Schema != OvernightSchema || p.Date != "2026-11-02" || p.By != "rahulnakmol" || p.At != "2026-11-02T18:00:00Z" || len(p.Runs) != 3 ||
		!slices.Equal(p.Runs, cards[0].Runs) || m.codes() != 1 {
		t.Fatalf("missions/overnight/2026-11-02.json: %+v (codes %d), want the card's runs %+v", p, m.codes(), cards[0].Runs)
	}
	if pr := p.Presence; pr.Login != "rahulnakmol" || pr.Via != "device-flow" || pr.Permission != "admin" || pr.ActionsRecord == "" || pr.ActionsRecord != m.presenceHash() {
		t.Fatalf("presence: %+v, want the session's presence.approve record %s", pr, m.presenceHash())
	}
	if cards := overnightCards(t, m, m.person); len(cards) != 0 {
		t.Fatalf("an Overnight card after tonight's runs are set: %+v", cards)
	}
}

// presenceHash is the hash of the newest presence.approve record in the checkout's actions log.
func (m *missionWorld) presenceHash() string {
	e := &Env{Options: Options{Dir: m.dir, Getenv: func(string) string { return "" }}}
	return e.presenceRecord("rahulnakmol")
}

// Criterion 4: each run stops at its bound. The tick starts tonight's runs;
// at 03:00, eight hours on, each stops at its 8 h bound, and the morning
// inbox names them. The next evening's run that spends past its 60 USD stops
// at its cost bound, and is not started again.
func TestOvernightRunStopsAtBound(t *testing.T) {
	m, now := overnightWorld(t)
	m.reviewAll()
	if code, out, errOut := runIn(t, m.person, m.opts, "1y", "inbox", "--repo", "o/r"); code != 0 || !strings.Contains(out, "done: @rahulnakmol set tonight's runs") {
		t.Fatalf("approve: exit %d:\n%s%s", code, out, errOut)
	}
	m.at(now, time.Date(2026, 11, 2, 19, 0, 0, 0, time.UTC))
	_, out := m.nightTick()
	if !strings.Contains(out, "overnight 2026-11-02: start author #982: author #982 to the approval line: an agent drafts each part of its baseline "+
		"(`aayu ready 982 --part <part> --request`); drafts only, every approval stays in the morning, until Tue 3 Nov 03:00 Europe/London, or 60 USD spent\n") ||
		!strings.Contains(out, "overnight 2026-11-02: start verify-sweep: ") || !strings.Contains(out, "overnight 2026-11-02: start evals: ") {
		t.Fatalf("tick at 19:00:\n%s", out)
	}
	if p, _ := m.overnightPlan("2026-11-02"); p.Runs[0].Started != "2026-11-02T19:00:00Z" || p.Runs[0].Stopped != nil {
		t.Fatalf("author #982 after the start: %+v", p.Runs[0])
	}
	m.at(now, time.Date(2026, 11, 3, 2, 0, 0, 0, time.UTC))
	if rep, out := m.nightTick(); len(rep.Overnight) != 3 || rep.Overnight[0].Action != "runs" || !strings.Contains(out, "overnight 2026-11-02: author #982 runs: 7 h of 8, 0 of 60 USD\n") {
		t.Fatalf("tick at 02:00: %+v\n%s", rep, out)
	}
	m.at(now, time.Date(2026, 11, 3, 3, 0, 0, 0, time.UTC))
	if _, out := m.nightTick(); !strings.Contains(out, "overnight 2026-11-02: stopped author #982 at its 8 h bound\n") || strings.Count(out, "at its 8 h bound") != 3 {
		t.Fatalf("tick at 03:00:\n%s", out)
	}
	p, _ := m.overnightPlan("2026-11-02")
	for _, r := range p.Runs {
		if r.Stopped == nil || r.Stopped.Bound != BoundHours || r.Stopped.At != "2026-11-03T03:00:00Z" {
			t.Fatalf("%s after its bound: %+v", r.name(), r)
		}
	}
	if rep, _ := m.nightTick(); len(rep.Overnight) != 0 { // a stopped run is never started again
		t.Fatalf("tick after the stops: %+v", rep.Overnight)
	}

	// The morning inbox names each run that stopped at its bound.
	m.at(now, time.Date(2026, 11, 3, 8, 0, 0, 0, time.UTC))
	cards := overnightCards(t, m, m.person)
	stopped := "author #982 stopped at Tue 3 Nov 03:00 Europe/London: its 8 h bound."
	if len(cards) != 1 || cards[0].Ask != overnightStopped || !slices.Contains(cards[0].Lines, stopped) ||
		!slices.Contains(cards[0].Lines, "verify-sweep stopped at Tue 3 Nov 03:00 Europe/London: its 8 h bound.") ||
		cards[0].Why != "author #982, verify-sweep and evals stopped at their bounds last night." {
		t.Fatalf("morning Overnight cards: %+v", cards)
	}
	code, out, errOut := runIn(t, m.person, m.opts, "q", "inbox", "--repo", "o/r")
	if code != 0 || !strings.Contains(out, "] Overnight #982: Last night: Feature 982\n") || !strings.Contains(out, "  "+stopped+"\n") {
		t.Fatalf("morning inbox: exit %d:\n%s%s", code, out, errOut)
	}

	// Tuesday evening: no milestone started today, so the review is done. Tonight's author run spends 61 USD by 19:30.
	m.at(now, time.Date(2026, 11, 3, 18, 0, 0, 0, time.UTC))
	if code, out, errOut := runIn(t, m.person, m.opts, "1y", "inbox", "--repo", "o/r"); code != 0 || !strings.Contains(out, "recorded in missions/overnight/2026-11-03.json") {
		t.Fatalf("approve on Tuesday: exit %d:\n%s%s", code, out, errOut)
	}
	m.at(now, time.Date(2026, 11, 3, 19, 0, 0, 0, time.UTC))
	m.nightTick()
	m.spend(time.Date(2026, 11, 3, 19, 30, 0, 0, time.UTC), 61)
	m.at(now, time.Date(2026, 11, 3, 20, 0, 0, 0, time.UTC))
	if _, out := m.nightTick(); !strings.Contains(out, "overnight 2026-11-03: stopped author #982 at its 60 USD bound (61 USD spent)\n") {
		t.Fatalf("tick after 61 USD:\n%s", out)
	}
	if p, _ := m.overnightPlan("2026-11-03"); p.Runs[0].Stopped == nil || p.Runs[0].Stopped.Bound != BoundCost {
		t.Fatalf("author #982 after its cost bound: %+v", p.Runs[0])
	}
	m.at(now, time.Date(2026, 11, 4, 7, 30, 0, 0, time.UTC))
	if cards := overnightCards(t, m, m.person); len(cards) != 1 ||
		!slices.Contains(cards[0].Lines, "author #982 stopped at Tue 3 Nov 20:00 Europe/London: its 60 USD bound (61 USD spent).") {
		t.Fatalf("Wednesday morning's Overnight cards: %+v", cards)
	}
}

// spend records a metered model call of usd at t in the checkout's actions log.
func (m *missionWorld) spend(at time.Time, usd float64) {
	m.t.Helper()
	e := &Env{Options: Options{Dir: m.dir, Getenv: func(string) string { return "" }}}
	log, err := actions.Open(e.ActionsPath(), actions.Options{Now: func() time.Time { return at }})
	if err == nil {
		_, err = log.Do(context.Background(), actions.Action{Kind: "model.call", Target: "grok-4.7", Story: "#982"},
			func(context.Context) (actions.Outcome, error) { return actions.Outcome{CostUSD: usd}, nil })
	}
	if err != nil {
		m.t.Fatal(err)
	}
}

// Criterion 5: he leaves the Overnight card. Nothing is written, the card
// stays for the evening, and the night's ticks start no overnight run: only
// the loops scheduled before run.
func TestOvernightLeftStartsNothing(t *testing.T) {
	m, now := overnightWorld(t)
	m.reviewAll()
	code, out, errOut := runIn(t, m.person, m.opts, "2", "inbox", "--repo", "o/r")
	if code != 0 || !strings.Contains(out, "Not now: nothing runs tonight: nothing was written, and the card stays in the inbox.") {
		t.Fatalf("leave: exit %d:\n%s%s", code, out, errOut)
	}
	if _, ok := m.overnightPlan("2026-11-02"); ok || m.codes() != 0 {
		t.Fatalf("a plan after he left the card, or %d codes", m.codes())
	}
	if cards := overnightCards(t, m, m.person); len(cards) != 1 || cards[0].Ask != overnightTonight {
		t.Fatalf("Overnight cards after he left it: %+v", cards)
	}
	for _, at := range []time.Time{time.Date(2026, 11, 2, 19, 0, 0, 0, time.UTC), time.Date(2026, 11, 3, 2, 0, 0, 0, time.UTC), time.Date(2026, 11, 3, 8, 0, 0, 0, time.UTC)} {
		m.at(now, at)
		if rep, out := m.nightTick(); len(rep.Overnight) != 0 || strings.Contains(out, "overnight") {
			t.Fatalf("tick at %s: %+v\n%s", at, rep.Overnight, out)
		}
	}
	if _, ok := m.overnightPlan("2026-11-02"); ok {
		t.Fatal("a plan after the night's ticks")
	}
	if cards := overnightCards(t, m, m.person); len(cards) != 0 { // the morning names nothing: nothing ran
		t.Fatalf("morning Overnight cards: %+v", cards)
	}
}

// The bounds are the ones recorded at the approval, and every run has one:
// with [autonomy.budget] wall_hours and cost_usd at 0, a run still stops at
// the next start of the day, which the morning does not report as a bound
// it reached. An approval races nothing: a plan written while the code waits
// is kept, and a plan aayu cannot read starts nothing.
func TestOvernightBoundsAndEdges(t *testing.T) {
	m, now := overnightWorld(t)
	m.reviewAll()
	other := `{"schema":"aayudha.overnight/v1","date":"2026-11-02","by":"someone-else","at":"2026-11-02T18:00:00Z","presence":{"login":"someone-else","via":"device-flow","permission":"write","actions_record":"x"},"runs":[]}` + "\n"
	m.edit(func() {
		m.g.Hook = func(method, path string) {
			if method == "POST" && path == "/login/device/code" { // while the code waits, another session sets tonight
				putLedger(t, m.bare, OvernightPath("2026-11-02"), other)
			}
		}
	})
	code, out, errOut := runIn(t, m.person, m.opts, "1y", "inbox", "--repo", "o/r")
	m.edit(func() { m.g.Hook = nil })
	if p, _ := m.overnightPlan("2026-11-02"); p.By != "someone-else" || !strings.Contains(out, "tonight's runs were set already, in missions/overnight/2026-11-02.json; nothing was written") {
		t.Fatalf("approval after another set tonight: exit %d, plan %+v:\n%s%s", code, p, out, errOut)
	}

	// Tuesday evening, with no hour and no cost bound in config.
	m.at(now, time.Date(2026, 11, 3, 18, 0, 0, 0, time.UTC))
	m.person["AAYU_AUTONOMY__BUDGET__WALL_HOURS"], m.person["AAYU_AUTONOMY__BUDGET__COST_USD"] = "0", "0"
	code, out, errOut = runIn(t, m.person, m.opts, "1y", "inbox", "--repo", "o/r")
	if code != 0 || strings.Count(out, "     Bound: the start of the day, Wed 4 Nov 09:00 Europe/London.\n") != 3 || !strings.Contains(out, "recorded in missions/overnight/2026-11-03.json") {
		t.Fatalf("approve with no hour or cost bound: exit %d:\n%s%s", code, out, errOut)
	}
	m.at(now, time.Date(2026, 11, 3, 19, 0, 0, 0, time.UTC))
	if _, out := m.nightTick(); !strings.Contains(out, "(`aayu verify --check all`), until Wed 4 Nov 09:00 Europe/London\n") {
		t.Fatalf("tick at 19:00:\n%s", out)
	}
	m.spend(time.Date(2026, 11, 3, 20, 0, 0, 0, time.UTC), 500)
	m.at(now, time.Date(2026, 11, 3, 23, 0, 0, 0, time.UTC))
	if _, out := m.nightTick(); !strings.Contains(out, "overnight 2026-11-03: evals runs: 4 h\n") {
		t.Fatalf("tick at 23:00:\n%s", out)
	}
	m.at(now, time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC))
	if _, out := m.nightTick(); strings.Count(out, "at the start of the day, Wed 4 Nov 09:00 Europe/London\n") != 3 {
		t.Fatalf("tick at 09:00:\n%s", out)
	}
	m.at(now, time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC))
	if cards := overnightCards(t, m, m.person); len(cards) != 0 {
		t.Fatalf("morning cards for runs that ended with the night: %+v", cards)
	}

	// Wednesday: the code waits past midnight. The evening of the card has ended, so nothing is written.
	m.at(now, time.Date(2026, 11, 4, 18, 0, 0, 0, time.UTC))
	m.person["AAYU_AUTONOMY__BUDGET__WALL_HOURS"], m.person["AAYU_AUTONOMY__BUDGET__COST_USD"] = "0.25", "60"
	m.edit(func() {
		m.g.Hook = func(method, path string) {
			if method == "POST" && path == "/login/device/code" {
				m.at(now, time.Date(2026, 11, 5, 0, 1, 0, 0, time.UTC))
			}
		}
	})
	code, out, errOut = runIn(t, m.person, m.opts, "1y", "inbox", "--repo", "o/r")
	m.edit(func() { m.g.Hook = nil })
	if _, ok := m.overnightPlan("2026-11-05"); ok || !strings.Contains(out, "the evening of the card ended while the code was waiting; nothing was written") {
		t.Fatalf("approval past midnight: exit %d:\n%s%s", code, out, errOut)
	}
	if _, ok := m.overnightPlan("2026-11-04"); ok {
		t.Fatal("a plan for Wednesday after the code waited past midnight")
	}
	// He approves it again at 18:00, with a quarter-hour bound. No tick comes until Thursday 10:00, past both the
	// 19:15 hour bound and the 09:00 start of the day: each run stops at the first, at the time it was reached.
	m.at(now, time.Date(2026, 11, 4, 18, 0, 0, 0, time.UTC))
	if code, out, errOut := runIn(t, m.person, m.opts, "1y", "inbox", "--repo", "o/r"); code != 0 || !strings.Contains(out, "recorded in missions/overnight/2026-11-04.json") {
		t.Fatalf("approve on Wednesday: exit %d:\n%s%s", code, out, errOut)
	}
	m.at(now, time.Date(2026, 11, 4, 19, 0, 0, 0, time.UTC))
	m.nightTick()
	m.at(now, time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC))
	if _, out := m.nightTick(); strings.Count(out, "at its 0.3 h bound\n") != 3 {
		t.Fatalf("tick on Thursday at 10:00:\n%s", out)
	}
	for _, r := range func() []OvernightRun { p, _ := m.overnightPlan("2026-11-04"); return p.Runs }() {
		if r.Stopped == nil || r.Stopped.Bound != BoundHours || r.Stopped.At != "2026-11-04T19:15:00Z" {
			t.Fatalf("%s after a late tick: %+v", r.name(), r.Stopped)
		}
	}
	// Thursday, with a 20 h bound and no tick until Friday 10:00: the start of the day, at 09:00, comes first.
	m.person["AAYU_AUTONOMY__BUDGET__WALL_HOURS"] = "20"
	m.at(now, time.Date(2026, 11, 5, 18, 0, 0, 0, time.UTC))
	if code, out, errOut := runIn(t, m.person, m.opts, "1y", "inbox", "--repo", "o/r"); code != 0 || !strings.Contains(out, "recorded in missions/overnight/2026-11-05.json") {
		t.Fatalf("approve on Thursday: exit %d:\n%s%s", code, out, errOut)
	}
	m.at(now, time.Date(2026, 11, 5, 19, 0, 0, 0, time.UTC))
	m.nightTick()
	m.at(now, time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC))
	if _, out := m.nightTick(); strings.Count(out, "at the start of the day, Fri 6 Nov 09:00 Europe/London\n") != 3 {
		t.Fatalf("tick on Friday at 10:00:\n%s", out)
	}
	if p, _ := m.overnightPlan("2026-11-05"); p.Runs[0].Stopped == nil || p.Runs[0].Stopped.Bound != BoundDay || p.Runs[0].Stopped.At != "2026-11-06T09:00:00Z" {
		t.Fatalf("author #982 after a late tick: %+v", p.Runs[0].Stopped)
	}

	putLedger(t, m.bare, OvernightPath("2026-11-07"), "{}\n")
	if rep, out := m.nightTick(); len(rep.Overnight) != 1 || !strings.Contains(out, "overnight: unreadable, nothing starts: ledger missions/overnight/2026-11-07.json: not an overnight plan aayu wrote\n") {
		t.Fatalf("tick with an unreadable plan: %+v\n%s", rep.Overnight, out)
	}
	if code, out, errOut := run(t, m.person, m.opts, "inbox", "--list", "--repo", "o/r"); code == 0 || !strings.Contains(errOut, "not an overnight plan aayu wrote") {
		t.Fatalf("inbox --list with an unreadable plan: exit %d: %s%s", code, out, errOut)
	}
}

// A spend that cannot be read is never budget left: a run with a cost bound
// stops; one with none runs on (finding 5 of the Goldfish review).
func TestOvernightSpendUnreadableStops(t *testing.T) {
	e := &Env{Options: Options{Dir: t.TempDir(), Getenv: func(string) string { return "" }, Now: func() time.Time { return time.Date(2026, 11, 2, 20, 0, 0, 0, time.UTC) }}}
	if err := os.MkdirAll(e.ActionsPath(), 0o755); err != nil { // a directory where the log is: it opens, and every read fails
		t.Fatal(err)
	}
	clock, _ := config.NewClock("Europe/London", "Mon-Fri 09:00-18:00")
	for _, tc := range []struct {
		cost float64
		want string
	}{{60, "stop"}, {0, "runs"}} {
		r := OvernightRun{Run: RunSweep, Hours: 8, CostUSD: tc.cost, Ends: "2026-11-03T09:00:00Z", Started: "2026-11-02T19:00:00Z"}
		line, _ := e.overnightStep(clock, &r)
		if line.Action != tc.want || tc.want == "stop" && !strings.Contains(line.Detail, "its 60 USD bound: its spend cannot be read (") {
			t.Fatalf("cost bound %v with an unreadable log: %+v", tc.cost, line)
		}
	}
}
