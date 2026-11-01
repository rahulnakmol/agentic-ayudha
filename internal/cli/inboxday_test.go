package cli

import (
	"encoding/json"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/rahulnakmol/agentic-ayudha/internal/inspect"
)

// Story #1175: the morning is set. The inbox shows the day's cards in the
// order a person decides them, and a session that started a mission ends with
// the day's plan.

// morningWorld is approveWorld's #889, whose intent, spec, and tech drafts
// wait, with Decision #5, Feature #900 approved to build and not started, and
// a PR at the review cap. It returns the PR's number.
func morningWorld(t *testing.T) (*missionWorld, int) {
	t.Helper()
	m := approveWorld(t)
	m.issue(900, "Feature baselines", "feature", 0, "")
	m.issue(950, "Story 950", "story", 900, "")
	m.feature(900, 950)
	m.status(950, "ready", "")
	if code, out, errOut := m.approve(900); code != 0 {
		t.Fatalf("approve 900: exit %d: %s%s", code, out, errOut)
	}
	m.issue(5, "Choose the rules engine", "decision", 0, "")
	m.edit(func() { m.g.Issues[5].Body = decisionBody("true", "") })
	pr := m.g.AddPR("fix(cli): capped", "Refs #170", "o", "fix/170", strings.Repeat("d", 40), "dev")
	st, _ := json.Marshal(inspect.State{Cycles: inspect.MaxCycles, Stopped: strings.Repeat("d", 40), Rounds: []inspect.Round{{Head: "a"}, {Head: "b"}, {Head: "c"}}, Result: &inspect.Result{Disposition: "fix"}})
	m.edit(func() {
		m.g.Issues[pr.Number].Comments = append(m.g.Issues[pr.Number].Comments, gateComment(77, inspect.Marker+"\n### inspect: fix\n\n<!-- aayudha:inspect-state "+string(st)+" -->\n"))
	})
	return m, pr.Number
}

var cardHead = regexp.MustCompile(`(?m)^\[\d+/\d+\] (.+?) #(\d+): `)

// cardOrder is each card the line-by-line inbox showed, as "Word #n".
func cardOrder(out string) []string {
	var got []string
	for _, h := range cardHead.FindAllStringSubmatch(out, -1) {
		got = append(got, h[1]+" #"+h[2])
	}
	return got
}

// Criterion 1: with a Decision, #889's three Baseline cards, its Approve to
// build card, and a Mission start card waiting, the inbox lists them in that
// order before the other cards, and one session shows #889's Approve to build
// and Mission start cards after its baselines, before the other cards.
func TestMorningCardOrder(t *testing.T) {
	morning := []string{InboxDecision, InboxBaseline, InboxApprove, InboxStart}
	if !slices.Equal(inboxOrder[:len(morning)], morning) {
		t.Fatalf("the inbox orders %v first, want %v, then the other cards", inboxOrder[:len(morning)], morning)
	}
	m, pr := morningWorld(t)
	keys, _ := m.listed(m.person)
	capped := "capped#" + strconv.Itoa(pr)
	if want := []string{"decision#5", "baseline#889/intent", "baseline#889/spec", "baseline#889/tech", "mission-start#900", capped}; !slices.Equal(keys, want) {
		t.Fatalf("inbox --list: %v, want %v", keys, want)
	}
	m.device("rahulnakmol")
	// Accept the Decision, approve the three baselines and #889, start #889, Not now on #900, skip the PR.
	// #889's Mission start comes before #900's, which waited from the start: the order by number, as --list has it.
	code, out, errOut := runIn(t, m.person, m.opts, "aaaaa1y2s", "inbox", "--repo", "o/r")
	want := []string{"Decision #5", "Baseline #889", "Baseline #889", "Baseline #889", "Approve to build #889",
		"Mission start #889", "Mission start #900", "At the review cap #" + strconv.Itoa(pr)}
	if got := cardOrder(out); code != 0 || m.codes() != 1 || !slices.Equal(got, want) {
		t.Fatalf("inbox: exit %d, device codes %d, cards %v, want %v:\n%s%s", code, m.codes(), got, want, out, errOut)
	}
	if p, _ := m.plan(889); p.StartedBy != "rahulnakmol" {
		t.Fatalf("the mission of #889 did not start:\n%s", out)
	}
}

// dayLine889 is the day line of #889 as approveWorld's Sunday clock reads it:
// MS1 today, and the review at 18:00 on Monday, the next business day.
const dayLine889 = "  #889 · today MS1: #20, #21, #22, #30 · evening review Mon 2 Nov 18:00 Europe/London, the end of [time] business_hours\n"

// Criterion 2: he approves #889 and starts its mission in the session, and
// leaves #900 for later: the last card says The day is set, with one line for
// #889 only: its milestone today, its stories, and the evening review at the
// end of [time] business_hours. The full-screen inbox ends with the same card.
func TestDayIsSet(t *testing.T) {
	t.Run("line by line", func(t *testing.T) {
		m, pr := morningWorld(t)
		m.device("rahulnakmol")
		code, out, errOut := runIn(t, m.person, m.opts, "aaaaa1y2s", "inbox", "--repo", "o/r")
		last := strings.LastIndex(out, "At the review cap #"+strconv.Itoa(pr))
		set := strings.Index(out, "\n"+dayIsSet+"\n"+dayLine889+"\ninbox: ")
		if code != 0 || last < 0 || set < last || strings.Count(out, dayIsSet) != 1 || strings.Contains(out[max(set, 0):], "#900") {
			t.Fatalf("inbox: exit %d:\n%s%s", code, out, errOut)
		}
	})
	t.Run("full screen", func(t *testing.T) {
		m := lockWorld(t, "cursor")
		// Monday 19:30 in UTC is Tuesday 01:00 in Kolkata, before its hours: the review is Tuesday at 18:00 there.
		m.opts.Now = func() time.Time { return time.Date(2026, 11, 2, 19, 30, 0, 0, time.UTC) }
		m.person["AAYU_TIME__TZ"] = "Asia/Kolkata"
		opts := m.opts
		opts.TUI = func(tm tea.Model) error {
			im := tm.(*inboxModel)
			teaDrive(im, im.Init())
			im.cur = slices.IndexFunc(im.rows, func(i int) bool { return im.items[i].Kind == InboxStart })
			teaKeys(im, "1", "enter", "enter", "q")
			return nil
		}
		m.device("rahulnakmol")
		code, out, errOut := runIn(t, m.person, opts, "", "inbox", "--repo", "o/r")
		want := "\n" + dayIsSet + "\n  #889 · today MS1: #20, #21, #22 · evening review Tue 3 Nov 18:00 Asia/Kolkata, the end of [time] business_hours\n\ninbox: 1 done"
		if p, _ := m.plan(889); code != 0 || p.StartedBy != "rahulnakmol" || !strings.Contains(out, want) {
			t.Fatalf("full-screen inbox: exit %d, started by %q:\n%s%s", code, p.StartedBy, out, errOut)
		}
	})
}

// Criterion 3: he leaves before the Mission start card, by quitting or with
// Not now: no The day is set line, nothing starts, and the start cards wait
// for the next session.
func TestDayNotSetBeforeStart(t *testing.T) {
	for name, keys := range map[string]string{"quit": "aaaaaq", "not now": "aaaaa22s"} {
		t.Run(name, func(t *testing.T) {
			m, _ := morningWorld(t)
			m.device("rahulnakmol")
			code, out, errOut := runIn(t, m.person, m.opts, keys, "inbox", "--repo", "o/r")
			if code != 0 || !strings.Contains(out, "Mission start #889: ") || strings.Contains(out, dayIsSet) {
				t.Fatalf("inbox %s: exit %d:\n%s%s", keys, code, out, errOut)
			}
			for _, f := range []int{900, 889} {
				if p, raw := m.plan(f); p.StartedBy != "" || p.Lock != nil {
					t.Fatalf("leaving started #%d:\n%s", f, raw)
				}
			}
			m.g.Reset()
			waits, _ := m.listed(m.person)
			if !slices.Contains(waits, "mission-start#900") || !slices.Contains(waits, "mission-start#889") || len(m.g.Writes()) != 0 {
				t.Fatalf("the next session's inbox: %v, writes %v", waits, m.g.Writes())
			}
		})
	}
}
