package cli

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// Story #798: clearing a set of housekeeping, or the whole inbox, is
// celebrated briefly: words in a seat's voice, and with motion a burst of
// confetti over the card pane that never covers the key bar, never blocks a
// key, and never follows a failure.
func TestInboxCheer(t *testing.T) {
	clock := time.Date(2026, 10, 19, 17, 0, 0, 0, time.UTC)
	confettiIn := func(s string) bool { return strings.ContainsAny(s, "*+•°~✦") }
	scene := func(motion, decision bool, failing int) (*inboxModel, *time.Time) {
		now := clock
		var items []InboxItem
		if decision {
			items = append(items, InboxItem{Kind: InboxDecision, Number: 685, Title: "Which paths need a person?", Seat: verbs.Founder, Why: "why", Counsel: "counsel",
				Options: []InboxAction{{N: 1, Label: "The narrow set", Recommended: true, Command: "aayu decide 685 --accept 1"}}})
		}
		for _, n := range []int{640, 642, 644} {
			items = append(items, InboxItem{Kind: InboxFixed, Number: n, Title: fmt.Sprintf("Bug %d", n), PR: n + 1, Seat: verbs.EngineeringManager, Why: "why", Counsel: "counsel",
				Options: []InboxAction{{N: 1, Label: fmt.Sprintf("close #%d as completed", n), Recommended: true, Command: "inbox: close as fixed"}}})
		}
		m := newInboxModel("o/r", "rahulnakmol", "", !motion)
		m.motion, m.now = motion, func() time.Time { return now }
		m.twin = func(s verbs.SeatID) string {
			return map[verbs.SeatID]string{verbs.Auditor: "Vidhi (Auditor)", verbs.Founder: "Atri (Founder)"}[s]
		}
		m.Update(tea.WindowSizeMsg{Width: 120, Height: 34})
		m.Update(inboxGatheredMsg{items: items, at: clock})
		m.act = func(it InboxItem, a InboxAction, w *inboxWork) (InboxItem, string, error) {
			if it.Number == failing {
				return it, "", ghRefusal(http.MethodPatch, fmt.Sprintf("/repos/o/r/issues/%d", it.Number), 422, "Validation Failed")
			}
			return it, "", nil
		}
		return m, &now
	}
	view := func(m *inboxModel) []string {
		m.dirty = true
		return strings.Split(ansi.Strip(m.View().Content), "\n")
	}
	card := func(lines []string) string { // the card pane, right of the divider
		var out []string
		for _, l := range lines {
			if _, r, ok := strings.Cut(l, "│"); ok {
				out = append(out, r)
			}
		}
		return strings.Join(out, "\n")
	}

	// Criterion 1: C on three housekeeping items, all done.
	m, now := scene(true, true, 0)
	teaKeys(m, "C", "y")
	if m.status != "Housekeeping cleared: 3 tidied. Job well done." {
		t.Fatalf("status after clearing three: %q", m.status)
	}
	*now = clock.Add(700 * time.Millisecond)
	lines := view(m)
	flat := strings.Join(strings.Fields(card(lines)), " ")
	if !strings.Contains(flat, "🎉 Job well done") || !strings.Contains(flat, "📜 Vidhi, Auditor") || !strings.Contains(flat, "Three loose ends closed, each with its") ||
		!confettiIn(card(lines)) {
		t.Errorf("the cheer mid-burst:\n%s", strings.Join(lines, "\n"))
	}
	if bar := lines[len(lines)-1]; confettiIn(bar) || !strings.Contains(bar, "q quit") {
		t.Errorf("the key bar is covered: %q", bar)
	}
	*now = clock.Add(1600 * time.Millisecond) // the burst is over; the words stay
	if c := card(view(m)); confettiIn(c) || !strings.Contains(c, "Job well done") {
		t.Errorf("after the burst:\n%s", c)
	}
	// A key during it does what it always does, and ends it.
	at := m.cur
	teaKeys(m, "j")
	if m.cheer != nil || m.cur == at {
		t.Errorf("j during the cheer: cheer %v, cursor %d -> %d", m.cheer, at, m.cur)
	}

	// Criterion 2: the last item that needs a person, done: inbox zero.
	m.cur = slicesIndex(m, 685)
	teaKeys(m, "enter")
	*now = clock.Add(2 * time.Second)
	if c := strings.Join(strings.Fields(card(view(m))), " "); m.status != "Inbox zero. Job well done." || !strings.Contains(c, "🧘 Atri, Founder") || !strings.Contains(c, "Nothing waits on you.") {
		t.Errorf("inbox zero: status %q\n%s", m.status, c)
	}

	// Criterion 3: with motion off (NO_COLOR or [ui] motion = false), the words only.
	m, now = scene(false, true, 0)
	teaKeys(m, "C", "y")
	*now = clock.Add(700 * time.Millisecond)
	if c := card(view(m)); m.status != "Housekeeping cleared: 3 tidied. Job well done." || confettiIn(c) || !strings.Contains(c, "Job well done") {
		t.Errorf("without motion: status %q\n%s", m.status, c)
	}

	// Criterion 4: one of the three fails: no celebration.
	m, _ = scene(true, false, 642)
	teaKeys(m, "C", "y")
	if m.cheer != nil || m.status != "Applied 3: 2 done, 1 not done. Written changes are not undone here." {
		t.Errorf("a set with a failure: cheer %v, status %q", m.cheer, m.status)
	}

	// A set of one is still a set: C clears the last housekeeping item while
	// a Decision still waits (CodeRabbit r4330641134 on #851).
	m, _ = scene(false, true, 0)
	m.state[InboxItem{Kind: InboxFixed, Number: 640}.key()], m.state[InboxItem{Kind: InboxFixed, Number: 642}.key()] = "done", "done"
	teaKeys(m, "C", "y")
	if m.status != "Housekeeping cleared: 1 tidied. Job well done." {
		t.Errorf("a set of one: status %q", m.status)
	}

	// Criterion 5: an inbox that opens empty has nothing to celebrate.
	m = newInboxModel("o/r", "rahulnakmol", "", false)
	m.motion, m.now = true, func() time.Time { return clock }
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 34})
	m.Update(inboxGatheredMsg{items: nil, at: clock})
	if m.cheer != nil || strings.Contains(strings.Join(view(m), "\n"), "Job well done") {
		t.Errorf("an empty inbox celebrated")
	}
}
