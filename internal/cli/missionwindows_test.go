package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
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
	if m.timeComments(889) != 1 || len(sink.Pages) != 1 || !strings.Contains(sink.Pages[0].Body, "#889: mission 1's time card (48 h, limit)") {
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
	cards := m.budgetCards()
	if len(cards) != 1 || cards[0].Title != "#889's next story would pass its $660 budget" || len(sink.Pages) != 1 ||
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
