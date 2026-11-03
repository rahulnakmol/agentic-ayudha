package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/notify"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
)

// london is a time on 22 October 2026 in London, in summer time (UTC+1): the
// default windows there are 10:30-11:30 and 18:30-19:30.
func london(h, min int) time.Time { return time.Date(2026, 10, 22, h-1, min, 0, 0, time.UTC) }

// Story #1254, criteria 2 and 3: past [mission] max_hours outside a window,
// mission next still starts the next story and posts no time card; the
// request waits, queued for the next window's one message. In the window the
// card is shown, and starts go on until the window ends; from its end, with
// the card unanswered, new starts wait as before #1254, and the running
// stories go on.
func TestTimeLimitWaitsForAWindow(t *testing.T) {
	m := startedTwo(t, 25, 26, 27, 28)
	commitOnDev(m.g, ".aayudha/config.toml", "[build]\nagents = 8\n") // agents never hold a start here
	sink := m.windows()
	t0 := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	for _, n := range []int{20, 21} {
		m.status(n, "in-progress", "")
		m.delegation(n, t0, 0)
	}
	m.status(25, "ready", "")
	at := func(t time.Time) { m.opts.Now = func() time.Time { return t } }
	next := func(when, want string) {
		t.Helper()
		if code, out, errOut := m.next(889); code != 0 || out != want {
			t.Fatalf("mission next at %s: exit %d: %q, want %q%s", when, code, out, want, errOut)
		}
	}
	// 01:01 on 22 October: 48 h and a minute of mission time, and the next window opens at 10:30.
	at(t0.Add(48*time.Hour + time.Minute))
	next("01:01", "start #25\nwaits #26: not Ready (open)\nwaits #27: not Ready (open)\nwaits #28: not Ready (open)\n")
	if m.timeComments(889) != 0 || len(sink.Pages) != 0 {
		t.Fatalf("outside a window: %d time cards, pages %+v", m.timeComments(889), sink.Pages)
	}
	m.status(25, "in-progress", "")
	m.status(26, "ready", "")
	at(london(10, 29))
	next("10:29", "start #26\nwaits #27: not Ready (open)\nwaits #28: not Ready (open)\n")
	if m.timeComments(889) != 0 || len(sink.Pages) != 0 {
		t.Fatalf("a minute before the window: %d time cards, pages %+v", m.timeComments(889), sink.Pages)
	}
	// 10:30: the window opens. The queued card goes out in one message, the card is shown, and #27 still starts.
	m.status(26, "in-progress", "")
	m.status(27, "ready", "")
	at(london(10, 30))
	next("10:30", "start #27\nwaits #28: not Ready (open)\n")
	if len(sink.Pages) != 0 { // mission next sends no window message: the tick does, after every mission's cards (Goldfish round 3, finding 8)
		t.Fatalf("mission next at 10:30 sent %+v", sink.Pages)
	}
	if code, out, errOut := run(t, m.lane, m.opts, "mission", "tick", "--repo", "o/r"); code != 0 { // the window's one message is the tick's
		t.Fatalf("the tick at 10:30: exit %d: %s%s", code, out, errOut)
	}
	if m.timeComments(889) != 1 || len(sink.Pages) != 1 || !strings.Contains(sink.Pages[0].Body, "1. Mission time #889: #889 reached its 48 h time limit\n") {
		t.Fatalf("at the window's start: %d time cards, pages %+v", m.timeComments(889), sink.Pages)
	}
	if cards := missionCards(m.inboxList()); len(cards) != 1 || cards[0].Title != "#889 reached its 48 h time limit" {
		t.Fatalf("the inbox in the window: %+v", cards)
	}
	// 11:30: the card went unanswered to the window's end. New starts wait; the running stories go on.
	m.status(27, "in-progress", "")
	m.status(28, "ready", "")
	at(london(11, 29))
	if code, out, _ := run(t, m.agent, m.opts, "mission", "next", "889", "--repo", "o/r", "--dry-run"); code != 0 || !strings.Contains(out, "start #28") {
		t.Fatalf("a preview at 11:29: exit %d: %q", code, out)
	}
	at(london(11, 30))
	next("11:30", "waits #28: #889 reached max_hours (48 h)\n")
	if claims := m.claims(889); claims[25] != 1 || claims[26] != 1 || claims[27] != 1 || claims[28] != 0 {
		t.Fatalf("claims at the window's end: %v", claims)
	}
	if m.timeComments(889) != 1 || len(sink.Pages) != 1 {
		t.Fatalf("after the window: %d time cards, pages %+v", m.timeComments(889), sink.Pages)
	}
	// An edit of the unanswered card at 14:00 does not move its window: the hold stands (Goldfish round 2, finding 13).
	m.edit(func() {
		for _, c := range m.g.Issues[889].Comments {
			if strings.HasPrefix(c.Body, MissionTimeMarker) {
				c.UpdatedAt = london(14, 0)
			}
		}
	})
	at(london(14, 0))
	next("14:00", "waits #28: #889 reached max_hours (48 h)\n")
}

// Goldfish review, round 2, finding 9: the tick shows a due time card in a
// window for a mission whose Kanha is local, which it never wakes.
func TestTickShowsTheCardsOfALocalKanha(t *testing.T) {
	m := startedTwo(t, 25)
	m.windows()
	var a ready.Approval
	if err := json.Unmarshal([]byte(git(t, m.bare, "show", "ledger:"+ready.ApprovalPath(889))), &a); err != nil {
		t.Fatal(err)
	}
	a.Parent.Mode, a.Parent.Machine = "local", "rahulnakmol@laptop"
	b, _ := json.Marshal(a)
	putLedger(t, m.bare, ready.ApprovalPath(889), string(b))
	t0 := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	for _, n := range []int{20, 21} {
		m.status(n, "in-progress", "")
		m.delegation(n, t0, 0)
	}
	m.opts.Now = func() time.Time { return london(10, 30) }
	if code, out, errOut := run(t, m.lane, m.opts, "mission", "tick", "--repo", "o/r"); code != 0 || m.timeComments(889) != 1 {
		t.Fatalf("the tick at 10:30: exit %d, %d time cards: %s%s", code, m.timeComments(889), out, errOut)
	}
}

// budgetWorld is #889's mission with an approved budget of 660 USD, 620 of
// it spent, and #25 Ready: the next start at its 60 USD cap would pass it.
func budgetWorld(t *testing.T, more ...int) *missionWorld {
	t.Helper()
	m := newMissionWorld(t)
	m.startedMission(889, append([]int{20, 25}, more...))
	m.setPlan(889, func(p *MissionPlan) { p.Budget = 660 })
	m.closeIssue(20)
	m.status(25, "ready", "")
	log := filepath.Join(m.dir, ".aayudha", "telemetry", "actions.jsonl")
	if err := os.WriteFile(log, []byte(`{"kind":"model.call","story":"o/r#20","status":"done","cost_usd":600}`+"\n"+
		`{"kind":"model.call","story":"o/r#25","status":"done","cost_usd":20}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return m
}

// budgetCards are #889's open budget cards of new starts.
func (m *missionWorld) budgetCards() []InboxItem {
	return slices.DeleteFunc(m.inboxList().Items, func(it InboxItem) bool { return it.Kind != InboxMission || !strings.HasPrefix(it.Ask, "starts-") })
}

// Story #1254, criterion 4: the mission's own budget is used up outside a
// window: mission next still starts the next story, and the budget card waits
// for the next window. In the window the card is shown; from its end new
// starts wait for the budget. The [budget] breaker still stops every lane, in
// a window or out of it.
func TestBudgetWaitsForAWindow(t *testing.T) {
	m := budgetWorld(t, 26, 27)
	sink := m.windows()
	at := func(t time.Time) { m.opts.Now = func() time.Time { return t } }
	at(london(14, 0))
	if code, out, errOut := m.next(889); code != 0 || out != "start #25\nwaits #26: not Ready (open)\nwaits #27: not Ready (open)\n" || len(m.budgetCards()) != 0 || len(sink.Pages) != 0 {
		t.Fatalf("over the budget at 14:00: exit %d: %q%s, pages %+v", code, out, errOut, sink.Pages)
	}
	m.status(25, "in-progress", "")
	m.status(26, "ready", "")
	at(london(18, 30))
	if code, out, errOut := m.next(889); code != 0 || out != "start #26\nwaits #27: not Ready (open)\n" {
		t.Fatalf("over the budget at 18:30: exit %d: %q%s", code, out, errOut)
	}
	if code, out, errOut := run(t, m.lane, m.opts, "mission", "tick", "--repo", "o/r"); code != 0 { // the window's one message is the tick's
		t.Fatalf("the tick at 18:30: exit %d: %s%s", code, out, errOut)
	}
	cards := m.budgetCards()
	// The raise covers what the mission committed, 720, and one more story: 780, not 660 + 60.
	if len(cards) != 1 || cards[0].Title != "#889's next story would pass its $660 budget" || len(sink.Pages) != 1 || cards[0].Options[0].Label != "Raise the budget to $780, with a reason" ||
		!strings.Contains(sink.Pages[0].Body, "#889's next story would pass its $660 budget") {
		t.Fatalf("at the window's start: cards %+v, pages %+v", cards, sink.Pages)
	}
	m.status(26, "in-progress", "")
	m.status(27, "ready", "")
	at(london(19, 30))
	if code, out, errOut := m.next(889); code != 0 || out != "waits #27: would pass #889's approved budget (720 + 60 > 660 USD)\n" || len(m.budgetCards()) != 1 {
		t.Fatalf("over the budget at the window's end: exit %d: %q%s", code, out, errOut)
	}
	// The [budget] breaker is money, not a card: it stops every agent lane at 14:00 as it does in a window.
	pause := filepath.Join(m.dir, ".aayudha", "state", "paused")
	for _, now := range []time.Time{london(14, 0), london(18, 45)} {
		at(now)
		if err := os.MkdirAll(filepath.Dir(pause), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(pause, []byte("per_day_usd"), 0o644); err != nil {
			t.Fatal(err)
		}
		if code, out, errOut := m.next(889); code != ExitBudget || out != "" || !strings.Contains(errOut, "paused by the [budget] breaker") {
			t.Fatalf("the breaker at %s: exit %d: %q%s", now.Format(time.Kitchen), code, out, errOut)
		}
		_ = os.Remove(pause)
	}
}

// Goldfish review, round 2, finding 10: past the budget with no story that
// can start (one runs, one is not Ready), the window still shows the budget
// card, so the story that becomes Ready after the window waits for it.
func TestBudgetCardWithNothingToStart(t *testing.T) {
	m := budgetWorld(t, 26)
	m.windows()
	at := func(t time.Time) { m.opts.Now = func() time.Time { return t } }
	at(london(14, 0))
	if code, out, errOut := m.next(889); code != 0 || out != "start #25\nwaits #26: not Ready (open)\n" {
		t.Fatalf("over the budget at 14:00: exit %d: %q%s", code, out, errOut)
	}
	m.status(25, "in-progress", "")
	at(london(18, 30))
	if code, out, errOut := run(t, m.lane, m.opts, "mission", "tick", "--repo", "o/r"); code != 0 || len(m.budgetCards()) != 1 {
		t.Fatalf("the tick at 18:30 with nothing to start: exit %d, cards %+v: %s%s", code, m.budgetCards(), out, errOut)
	}
	m.status(26, "ready", "")
	at(london(20, 0))
	if code, out, errOut := m.next(889); code != 0 || out != "waits #26: would pass #889's approved budget (660 + 60 > 660 USD)\n" {
		t.Fatalf("a story Ready after the window: exit %d: %q%s", code, out, errOut)
	}
}

// Goldfish review, round 1, finding 1: the newest trusted card is the
// mission's state. An account's limit card, edited in a window after another
// account marked an older warning seen, still holds new starts from that
// window's end, and between the windows after it.
func TestTimeHoldReadsTheNewestCard(t *testing.T) {
	m := startedTwo(t, 25, 26)
	m.windows()
	t0 := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	for _, n := range []int{20, 21} {
		m.status(n, "in-progress", "")
		m.delegation(n, t0, 0)
	}
	m.edit(func() {
		limit := &fake.Comment{ID: 7201, CreatedAt: london(9, 0), UpdatedAt: london(10, 45), Body: missionCard(1, "limit", 48, 48, "limit")}
		limit.User.Login = "rahulnakmol"
		seen := &fake.Comment{ID: 7202, CreatedAt: london(10, 40), Body: missionCard(1, "seen", 0, 0, "seen")}
		seen.User.Login = "maintainer"
		if m.g.Permissions == nil {
			m.g.Permissions = map[string]string{}
		}
		m.g.Permissions["maintainer"] = "write"
		m.g.Issues[889].Comments = append(m.g.Issues[889].Comments, limit, seen)
	})
	m.status(25, "ready", "")
	for _, now := range []time.Time{london(11, 30), london(14, 0)} {
		m.opts.Now = func() time.Time { return now }
		if code, out, errOut := m.next(889); code != 0 || out != "waits #25: #889 reached max_hours (48 h)\nwaits #26: not Ready (open)\n" {
			t.Fatalf("mission next at %s: exit %d: %q%s", now.Format(time.Kitchen), code, out, errOut)
		}
	}
}

// Goldfish review, round 1, finding 3, and round 2, finding 2: a gate that
// fails [mission] max_no_go times outside a window asks nobody: it records
// the stop, and its ask waits in the ledger's outbox. The window's first tick
// posts the ask, with no second gate run.
func TestGateAskWaitsForAWindow(t *testing.T) {
	g := newGateWorld(t, "MS1=#20 MS2=#22", 20, 22)
	g.windows()
	g.closeIssue(20)
	g.status(22, "ready", "")
	asks := func() int {
		g.g.Mu.Lock()
		defer g.g.Mu.Unlock()
		return len(slices.DeleteFunc(slices.Clone(g.g.Issues[889].Comments), func(c *fake.Comment) bool { return !strings.Contains(c.Body, MissionAskMarker) }))
	}
	g.opts.Now = func() time.Time { return london(10, 45) }
	for i, check := range []string{"inbox.ship-once", "inbox.ship-once"} {
		g.vfail = []string{check}
		g.tip(strings.Repeat(fmt.Sprint(i+1), 40), "success")
		_ = g.gateJSON()
		g.fixPR(981+i, "verify:"+check, "Refs #889")
		if code, _, errOut := g.gate("--pr", fmt.Sprint(981+i)); code != 0 {
			t.Fatalf("fix %d: exit %d: %s", i, code, errOut)
		}
		g.mergePR(981 + i)
	}
	g.vfail = []string{"mission.stays"}
	g.tip(strings.Repeat("3", 40), "success")
	g.opts.Now = func() time.Time { return london(14, 0) }
	if code, _, errOut := g.gate(); code != 0 || len(g.runs()) != 3 || asks() != 0 {
		t.Fatalf("the third failed gate at 14:00: exit %d, runs %d, asks %d: %s", code, len(g.runs()), asks(), errOut)
	}
	g.opts.Now = func() time.Time { return london(18, 30) }
	if code, out, errOut := run(t, g.lane, g.opts, "mission", "tick", "--repo", "o/r"); code != 0 || asks() != 1 || len(g.runs()) != 3 {
		t.Fatalf("the tick at 18:30: exit %d, runs %d, asks %d: %s%s", code, len(g.runs()), asks(), out, errOut)
	}
}

// Goldfish review, round 1, finding 2, and round 2, findings 3, 6, 12 and 14:
// a request made outside a window waits in the ledger's outbox, so a run in
// another clone sends the window's one message with it after the first clone
// is gone. A run that claimed the window and stopped gives it to the next run
// once its claim lapses; each window has one message, and the next has its own.
func TestWindowMessageFromAnyCheckout(t *testing.T) {
	m := startedTwo(t, 25)
	m.issue(5, "Choose the rules engine", "decision", 0, "")
	m.edit(func() { m.g.Issues[5].Body = decisionBody("true", "") })
	sink := m.windows()
	t0 := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	for _, n := range []int{20, 21} {
		m.status(n, "in-progress", "")
		m.delegation(n, t0, 0)
	}
	m.opts.Now = func() time.Time { return t0.Add(48*time.Hour + time.Minute) } // 01:01: the limit, outside a window
	if code, _, errOut := run(t, m.agent, m.opts, "decide", "5", "--accept", "1", "--repo", "o/r"); code != ExitBlocked || len(sink.Pages) != 0 {
		t.Fatalf("an agent's batch at 01:01 in clone A: exit %d, pages %+v: %s", code, sink.Pages, errOut)
	}
	// Clone A is gone: its state with it. Clone B has its own state directory, and only the ledger is shared.
	if err := os.RemoveAll(filepath.Join(m.dir, ".aayudha")); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(t.TempDir(), "clone-b")
	git(t, filepath.Dir(other), "clone", "--quiet", "--branch", "dev", m.bare, other)
	b := m.opts
	b.Dir = other
	tick := func(when time.Time) {
		t.Helper()
		b.Now = func() time.Time { return when }
		if code, out, errOut := run(t, m.lane, b, "mission", "tick", "--repo", "o/r"); code != 0 {
			t.Fatalf("the tick at %s in clone B: exit %d: %s%s", when, code, out, errOut)
		}
	}
	// A run that claimed the morning window and stopped before it sent: the next run waits for the claim to lapse.
	putLedger(t, m.bare, WindowsPath, `{"window":"2026-10-22T09:30:00Z","state":"claimed","at":"2026-10-22T09:30:00Z","until":"2026-10-22T09:40:00Z"}`+"\n")
	tick(london(10, 35))
	if len(sink.Pages) != 0 {
		t.Fatalf("a message while another run's claim holds: %+v", sink.Pages)
	}
	tick(london(10, 41))
	tick(london(11, 0))
	if len(sink.Pages) != 1 || !strings.Contains(sink.Pages[0].Body, "Mission time #889: #889 reached its 48 h time limit\n") ||
		!strings.Contains(sink.Pages[0].Body, "#5 waits for your approval\n   1. Accepts option 1 (A) of Decision #5") {
		t.Fatalf("the morning window's messages from clone B: %+v", sink.Pages)
	}
	if out := ledgerFile(t, m.bare, OutboxPath); strings.TrimSpace(out) != "" {
		t.Fatalf("the outbox after the window's message: %q", out)
	}
	tick(london(18, 30))
	if len(sink.Pages) != 2 || !strings.Contains(sink.Pages[1].Body, "Mission time #889") || strings.Contains(sink.Pages[1].Body, "waits for your approval") {
		t.Fatalf("the evening window's message: %+v", sink.Pages)
	}
}

// Goldfish review, round 2, finding 5: the [budget] breaker pauses every
// agent lane, and the tick starts and wakes nothing, but the window's one
// message still goes out: the person it reaches is the one who clears it.
func TestPausedTickSendsTheWindowMessage(t *testing.T) {
	m := startedTwo(t, 25)
	m.issue(5, "Choose the rules engine", "decision", 0, "")
	m.edit(func() { m.g.Issues[5].Body = decisionBody("true", "") })
	sink := m.windows()
	m.opts.Now = func() time.Time { return london(14, 0) }
	if code, _, errOut := run(t, m.agent, m.opts, "decide", "5", "--accept", "1", "--repo", "o/r"); code != ExitBlocked || len(sink.Pages) != 0 {
		t.Fatalf("an agent's batch at 14:00: exit %d, pages %+v: %s", code, sink.Pages, errOut)
	}
	pause := filepath.Join(m.dir, ".aayudha", "state", "paused")
	if err := os.MkdirAll(filepath.Dir(pause), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pause, []byte("per_day_usd"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The scheduled loop's guard lets the build-ready tick through only in a window, and every other loop never.
	build, err := os.ReadFile(filepath.Join("..", "..", "loops", BuildLoop+".toml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(m.dir, "loops"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(m.dir, "loops", BuildLoop+".toml"), build, 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, errOut := run(t, m.lane, m.opts, "hook", "loop:"+BuildLoop, "--repo", "o/r"); code != ExitBudget {
		t.Fatalf("the paused loop's guard at 14:00: exit %d: %s", code, errOut)
	}
	m.opts.Now = func() time.Time { return london(18, 30) }
	if code, out, errOut := run(t, m.lane, m.opts, "hook", "loop:"+BuildLoop, "--repo", "o/r"); code != 0 {
		t.Fatalf("the paused loop's guard at 18:30: exit %d: %s%s", code, out, errOut)
	}
	code, out, errOut := run(t, m.lane, m.opts, "mission", "tick", "--repo", "o/r")
	if code != ExitBudget || !strings.Contains(errOut, "paused by the [budget] breaker") || len(sink.Pages) != 1 ||
		!strings.Contains(sink.Pages[0].Body, "#5 waits for your approval") || strings.Contains(out, "woke") {
		t.Fatalf("the paused tick at 18:30: exit %d, pages %+v: %s%s", code, sink.Pages, out, errOut)
	}
}

// ledgerFile is path on the ledger branch of bare, or "" when it is absent.
func ledgerFile(t *testing.T, bare, path string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", bare, "show", "ledger:"+path).Output()
	if err != nil {
		return ""
	}
	return string(out)
}

// Goldfish review, round 3, findings 3 and 4: the window ends while the tick
// posts what waited. The ask it posted leaves the outbox; the next ask is not
// posted after the window's end and is not lost: the next window posts it,
// and sends the message.
func TestOutboxWaitsWhenTheWindowEnds(t *testing.T) {
	m := startedTwo(t)
	sink := m.windows()
	var lines strings.Builder
	for _, id := range []string{"one", "two"} {
		b, _ := json.Marshal(queued{Key: "ask:o/r#889:" + id, At: "2026-10-22T01:00:00Z", Ask: &queuedAsk{Feature: 889,
			Ask: missionAsk{ID: id, Title: "Ask " + id, Why: "why " + id, Options: []InboxAction{{N: 1, Label: "stop", Command: "aayu mission stop 889 --reason"}}}}})
		lines.Write(append(b, '\n'))
	}
	putLedger(t, m.bare, OutboxPath, lines.String())
	asks := func() int {
		m.g.Mu.Lock()
		defer m.g.Mu.Unlock()
		return len(slices.DeleteFunc(slices.Clone(m.g.Issues[889].Comments), func(c *fake.Comment) bool { return !strings.Contains(c.Body, MissionAskMarker) }))
	}
	var posted atomic.Bool // the first comment takes the clock past the window's end
	m.g.Clock = func() time.Time { posted.Store(true); return london(11, 29) }
	m.opts.Now = func() time.Time {
		if posted.Load() {
			return london(11, 30)
		}
		return london(11, 29)
	}
	if code, out, errOut := run(t, m.lane, m.opts, "mission", "tick", "--repo", "o/r"); code != 0 || asks() != 1 || len(sink.Pages) != 0 {
		t.Fatalf("the tick across the window's end: exit %d, asks %d, pages %+v: %s%s", code, asks(), sink.Pages, out, errOut)
	}
	if out := ledgerFile(t, m.bare, OutboxPath); strings.Contains(out, `"ask:o/r#889:one"`) || !strings.Contains(out, `"ask:o/r#889:two"`) {
		t.Fatalf("the outbox after the window's end: %q", out)
	}
	m.g.Clock = func() time.Time { return london(18, 30) }
	m.opts.Now = func() time.Time { return london(18, 30) }
	if code, out, errOut := run(t, m.lane, m.opts, "mission", "tick", "--repo", "o/r"); code != 0 || asks() != 2 || len(sink.Pages) != 1 {
		t.Fatalf("the tick at 18:30: exit %d, asks %d, pages %+v: %s%s", code, asks(), sink.Pages, out, errOut)
	}
}

// Goldfish review, round 3, finding 1: the Decision SLA sweep outside a
// window posts no escalation comment, which mentions the person: it waits in
// the outbox, and the window's first tick posts it.
func TestEscalationWaitsForAWindow(t *testing.T) {
	m := startedTwo(t)
	m.issue(5, "Choose the rules engine", "decision", 0, "")
	m.edit(func() { m.g.Issues[5].Body = decisionBody("true", "") })
	m.windows()
	escalations := func() int {
		m.g.Mu.Lock()
		defer m.g.Mu.Unlock()
		return len(slices.DeleteFunc(slices.Clone(m.g.Issues[5].Comments), func(c *fake.Comment) bool { return !strings.Contains(c.Body, "Decision SLA lapsed") }))
	}
	m.opts.Now = func() time.Time { return london(14, 0) }
	if code, out, errOut := run(t, m.person, m.opts, "decide", "--repo", "o/r", "--set", "decisions.escalate_to=founder-b"); code != 0 || escalations() != 0 ||
		!strings.Contains(ledgerFile(t, m.bare, OutboxPath), `"escalation:o/r#5"`) {
		t.Fatalf("the sweep at 14:00: exit %d, %d escalations: %s%s", code, escalations(), out, errOut)
	}
	m.opts.Now = func() time.Time { return london(18, 30) }
	if code, out, errOut := run(t, m.lane, m.opts, "mission", "tick", "--repo", "o/r"); code != 0 || escalations() != 1 {
		t.Fatalf("the tick at 18:30: exit %d, %d escalations: %s%s", code, escalations(), out, errOut)
	}
}

// Goldfish review, round 3, findings 10 and 12: with no [time] windows, the
// tick sends what waited in the outbox at once. A request whose issue is gone
// leaves the outbox and does not hold the rest.
func TestOutboxDrainsWithNoWindows(t *testing.T) {
	m := startedTwo(t)
	sink := &notify.Fake{}
	m.opts.NotifySink = sink
	var lines strings.Builder
	for _, q := range []queued{
		{Key: "ask:o/r#999:gone", Ask: &queuedAsk{Feature: 999, Ask: missionAsk{ID: "gone", Title: "Gone", Why: "its Feature was deleted"}}},
		{Key: "ask:o/r#889:one", Ask: &queuedAsk{Feature: 889, Ask: missionAsk{ID: "one", Title: "Ask one", Why: "why one"}}},
		{Key: "page:p", Page: &notify.Page{Key: "p", Subject: "a page that waited"}},
	} {
		b, _ := json.Marshal(q)
		lines.Write(append(b, '\n'))
	}
	putLedger(t, m.bare, OutboxPath, lines.String())
	m.opts.Now = func() time.Time { return london(14, 0) }
	code, out, errOut := run(t, m.lane, m.opts, "mission", "tick", "--repo", "o/r")
	m.g.Mu.Lock()
	asks := len(slices.DeleteFunc(slices.Clone(m.g.Issues[889].Comments), func(c *fake.Comment) bool { return !strings.Contains(c.Body, MissionAskMarker) }))
	m.g.Mu.Unlock()
	if code != 0 || asks != 1 || len(sink.Pages) != 1 || !strings.Contains(sink.Pages[0].Body, "a page that waited") || strings.TrimSpace(ledgerFile(t, m.bare, OutboxPath)) != "" {
		t.Fatalf("the tick with no windows: exit %d, asks %d, pages %+v, outbox %q: %s%s", code, asks, sink.Pages, ledgerFile(t, m.bare, OutboxPath), out, errOut)
	}
}
