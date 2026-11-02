package cli

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/inspect"
)

// Story #1254, criterion 6: in a window, `aayu inbox` holds every item that
// needs a person, in the order a person decides them: the Decision, the
// baselines, Approve to build, the mission starts, a mission's budget and
// time cards, a narrow-set PR at Ready to ship, an inspect approval, a P0 or
// P1 finding that waits for a person, and a PR at the review cap. One session
// shows them all, in that order, under one device code.
func TestWindowInboxHoldsEveryRequest(t *testing.T) {
	m, capped := morningWorld(t)
	m.windows()
	m.opts.Now = func() time.Time { return time.Date(2026, 11, 2, 10, 45, 0, 0, time.UTC) } // Monday 10:45 London, in the morning window
	m.issue(910, "Feature with a running mission", "feature", 0, "")
	sha := func(c string) string { return strings.Repeat(c, 40) }
	by := func(c *fake.Comment) *fake.Comment { c.User.Login = "rahulnakmol"; return c }
	ready := m.g.AddPR("feat(config): a narrow-set change", "Refs #20", "o", "story/20-narrow", sha("1"), "dev")
	m.edit(func() { // its merge gate is green: only a person's pass is missing
		if m.g.CheckRuns == nil {
			m.g.CheckRuns = map[string][]fake.CheckRun{}
		}
		for _, name := range []string{"aayudha/merge-gate", "aayudha/inspect", "aayudha/pack"} {
			m.g.CheckRuns[sha("1")] = append(m.g.CheckRuns[sha("1")], fake.CheckRun{Name: name, Status: "completed", Conclusion: "success", Suite: &fake.Suite{ID: 1}})
		}
		m.g.Runs = append(m.g.Runs, fake.WorkflowRun{ID: 8001, Path: ".github/workflows/" + MergeGateWorkflow, HeadSHA: sha("1"), Event: "pull_request_target",
			Status: "completed", Conclusion: "success", CheckSuiteID: 1, PullRequests: []fake.RunPR{{Number: ready.Number}}})
	})
	review := m.g.AddPR("feat: no reviewer could take it", "Refs #21", "o", "story/21-review", sha("2"), "dev")
	finding := m.g.AddPR("feat: a P1 finding stays", "Refs #22", "o", "story/22-finding", sha("3"), "dev")
	human, _ := json.Marshal(inspect.State{Human: sha("2"), Result: &inspect.Result{Head: sha("2"), Disposition: "fix"}})
	p1, _ := json.Marshal(inspect.State{Cycles: inspect.MaxCycles, Result: &inspect.Result{Head: sha("3"), Disposition: "fix",
		Findings: []inspect.Finding{{ID: "f1", Severity: "P1", Title: "a request is lost between windows", File: "internal/notify/notify.go", Line: 140},
			{ID: "f2", Severity: "P3", Title: "wording", File: "README.md"}}}})
	m.edit(func() {
		m.g.Issues[910].Comments = append(m.g.Issues[910].Comments,
			by(&fake.Comment{ID: 9101, Body: missionCard(1, "limit", 48, 48, "limit")}),
			by(&fake.Comment{ID: 9102, Body: MissionAskMarker + mustJSON(t, startsAsk(910, 660, 60)) + " -->\n**budget**\n"}))
		m.g.Issues[ready.Number].Comments = append(m.g.Issues[ready.Number].Comments, by(&fake.Comment{ID: 9103, Body: ReadyToShipMarker + " head=" + sha("1") + " -->\n**Ready to ship**"}))
		m.g.Issues[review.Number].Comments = append(m.g.Issues[review.Number].Comments, gateComment(9104, inspect.Marker+"\n<!-- aayudha:inspect-state "+string(human)+" -->\n"))
		m.g.Issues[finding.Number].Comments = append(m.g.Issues[finding.Number].Comments, gateComment(9105, inspect.Marker+"\n<!-- aayudha:inspect-state "+string(p1)+" -->\n"))
	})
	keys, items := m.listed(m.person)
	want := []string{"decision#5", "baseline#889/intent", "baseline#889/spec", "baseline#889/tech", "mission-start#900", "mission#910/starts-660", "mission-time#910",
		fmt.Sprintf("ready#%d", ready.Number), fmt.Sprintf("human-review#%d", review.Number), fmt.Sprintf("finding#%d", finding.Number), fmt.Sprintf("capped#%d", capped)}
	if !slices.Equal(keys, want) {
		t.Fatalf("inbox --list in the window: %v, want %v", keys, want)
	}
	if f := items[9]; f.Details[0] != "P1 f1: a request is lost between windows (internal/notify/notify.go:140)" || len(f.Details) != 1 || f.recommended() == nil || f.recommended().Command != "open" {
		t.Fatalf("the finding card: %+v", f)
	}
	m.device("rahulnakmol")
	// Accept the Decision, approve the three baselines and #889, start #889, Not now on #900, then skip the rest.
	code, out, errOut := runIn(t, m.person, m.opts, "aaaaa1y2ssssss", "inbox", "--repo", "o/r")
	order := []string{"Decision #5", "Baseline #889", "Baseline #889", "Baseline #889", "Approve to build #889", "Mission start #889", "Mission start #900",
		"Mission #910", "Mission time #910", fmt.Sprintf("Ready to ship #%d", ready.Number), fmt.Sprintf("Needs your review #%d", review.Number),
		fmt.Sprintf("P0 or P1 finding #%d", finding.Number), fmt.Sprintf("At the review cap #%d", capped)}
	if got := cardOrder(out); code != 0 || m.codes() != 1 || !slices.Equal(got, order) {
		t.Fatalf("one session in the window: exit %d, device codes %d, cards %v, want %v:\n%s%s", code, m.codes(), got, order, out, errOut)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
