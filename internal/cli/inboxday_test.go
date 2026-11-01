package cli

import (
	"encoding/json"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/inspect"
)

// Story #1175: the morning is set. The inbox shows the day's cards in the
// order a person decides them, and a session that started a mission ends with
// the day's plan.

// morningWorld is approveWorld's #889, whose intent, spec, and tech drafts
// wait, with Decision #5, Feature #874 approved to build and not started, and
// a PR at the review cap. It returns the PR's number.
func morningWorld(t *testing.T) (*missionWorld, int) {
	t.Helper()
	m := approveWorld(t)
	m.issue(874, "Feature baselines", "feature", 0, "")
	m.issue(950, "Story 950", "story", 874, "")
	m.feature(874, 950)
	m.status(950, "ready", "")
	if code, out, errOut := m.approve(874); code != 0 {
		t.Fatalf("approve 874: exit %d: %s%s", code, out, errOut)
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
	if want := []string{"decision#5", "baseline#889/intent", "baseline#889/spec", "baseline#889/tech", "mission-start#874", capped}; !slices.Equal(keys, want) {
		t.Fatalf("inbox --list: %v, want %v", keys, want)
	}
	m.device("rahulnakmol")
	// Accept the Decision, approve the three baselines and #889, Not now on #874, start #889, skip the PR.
	code, out, errOut := runIn(t, m.person, m.opts, "aaaaa21ys", "inbox", "--repo", "o/r")
	want := []string{"Decision #5", "Baseline #889", "Baseline #889", "Baseline #889", "Approve to build #889",
		"Mission start #874", "Mission start #889", "At the review cap #" + strconv.Itoa(pr)}
	if got := cardOrder(out); code != 0 || m.codes() != 1 || !slices.Equal(got, want) {
		t.Fatalf("inbox: exit %d, device codes %d, cards %v, want %v:\n%s%s", code, m.codes(), got, want, out, errOut)
	}
	if p, _ := m.plan(889); p.StartedBy != "rahulnakmol" {
		t.Fatalf("the mission of #889 did not start:\n%s", out)
	}
}
