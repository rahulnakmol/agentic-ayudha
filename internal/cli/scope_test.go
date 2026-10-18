package cli

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/scope"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// aayu scope end to end: an agent plans v0.3.0 in its milestone and drafts
// it; only a person, through GitHub's device flow even at a terminal with no
// agent marker, makes it the baseline; a change within the 20% budget goes
// ahead, one past it is refused, then only asked for against an accepted
// Decision, and a person applies it; Feedback moves by milestone and horizon
// label only; the hand-off writes the Release's includes links.
func TestScope(t *testing.T) {
	g := fake.New(t, "o/r", "User")
	g.Viewer = "founder"
	const client = "Ov23liScope"
	g.Variables = map[string]string{"AAYUDHA_DEVICE_CLIENT_ID": client}
	release := g.AddIssue("v0.3.0", fixtureBody(t, "release", func(f map[string]string) {
		f["version"], f[workitems.FieldLinks] = "v0.3.0", ""
	}), "type:release") // #1
	feature := g.AddIssue("Release planning", fixtureBody(t, "feature", nil), "type:feature")   // #2
	m := g.AddIssue("Draft a plan", fixtureBody(t, "story", nil), "type:story", "effort:M")     // #3, 3 points
	l := g.AddIssue("Baseline it", fixtureBody(t, "story", nil), "type:story", "effort:L")      // #4, 5 points
	xs := g.AddIssue("Assign feedback", fixtureBody(t, "story", nil), "type:story", "effort:S") // #5, 2 points
	feedbackBody := "### Summary\n\nPlans grow faster than we can contain them.\n"
	fb := g.AddIssue("Too many ideas", feedbackBody, "type:feedback", "intake", "horizon:now") // #6
	decision := g.AddIssue("Let #5 into v0.3.0", fixtureBody(t, "decision", func(f map[string]string) {
		f[workitems.FieldSummary] += " It decides the scope of Release #1."
	}), "type:decision") // #7
	for _, s := range []*fake.Issue{m, l, xs} {
		feature.Subs, s.Parent = append(feature.Subs, s.Number), feature.Number
	}
	dir, bare := ledgerCheckout(t)
	agent := map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "AAYU_AGENT": "1", "XDG_STATE_HOME": filepath.Join(dir, "state")}
	person := map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "XDG_STATE_HOME": filepath.Join(dir, "state")} // a terminal with no agent marker
	opts := Options{IsTerminal: stdinTTY, GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir, DevicePoll: 10 * time.Millisecond}
	scopeRun := func(env map[string]string, args ...string) (int, ScopeReport, string) {
		t.Helper()
		code, out, errOut := run(t, env, opts, append([]string{"scope", "1", "--repo", "o/r", "--json"}, args...)...)
		var rep ScopeReport
		_ = json.Unmarshal([]byte(out), &rep)
		return code, rep, out + errOut
	}
	ledgerJSON := func(path string, v any) {
		t.Helper()
		if err := json.Unmarshal([]byte(git(t, bare, "show", "ledger:"+path)), v); err != nil {
			t.Fatalf("ledger %s: %v", path, err)
		}
	}
	milestone := func(n int) string {
		g.Mu.Lock()
		defer g.Mu.Unlock()
		if ms := g.Issues[n].Milestone; ms != nil {
			return ms.Title
		}
		return ""
	}
	approve := func() {
		g.Mu.Lock()
		g.Device = &fake.DeviceFlow{ClientID: client, Approver: "founder", Outcome: "approve"}
		g.Mu.Unlock()
	}
	started := func() int { g.Mu.Lock(); defer g.Mu.Unlock(); return g.Device.Started }

	// Plan: before a baseline, changes are free; the milestone is made on the first add.
	for _, n := range []string{"2", "3", "4"} {
		if code, _, out := scopeRun(agent, "--add", n); code != 0 {
			t.Fatalf("--add %s: exit %d: %s", n, code, out)
		}
	}
	if milestone(3) != "v0.3.0" || milestone(5) != "" {
		t.Fatalf("milestones: #3 %q, #5 %q", milestone(3), milestone(5))
	}
	code, rep, out := scopeRun(agent, "--draft")
	if code != 0 || rep.Points != 8 || len(rep.Items) != 3 || rep.Request == nil || rep.Request.Kind != "baseline" {
		t.Fatalf("--draft: exit %d: %s", code, out)
	}
	var draft scope.Draft
	ledgerJSON(scope.DraftPath("v0.3.0"), &draft)
	if draft.Hash != rep.Request.Hash || draft.Points != 8 || draft.Items[1].Parent != feature.Number {
		t.Fatalf("draft: %+v", draft)
	}

	// Accepting is a person's: refused for an agent lane, and a device code even at a bare terminal.
	if code, _, out := scopeRun(agent, "--accept"); code != ExitBlocked || !strings.Contains(out, "refused for an agent lane (AAYU_AGENT=1)") {
		t.Fatalf("agent --accept: exit %d: %s", code, out)
	}
	approve()
	if code, rep, out := scopeRun(person, "--accept"); code != 0 || rep.Baseline != 1 || started() != 1 {
		t.Fatalf("person --accept: exit %d, device codes %d: %s", code, started(), out)
	}
	var base scope.Baseline
	ledgerJSON(scope.BaselinePath("v0.3.0", 1), &base)
	if base.BaselinedBy != "founder" || base.Points != 8 || base.Hash != draft.Hash || base.BudgetPct != 20 {
		t.Fatalf("baseline: %+v", base)
	}
	if code, _, out := scopeRun(person, "--accept"); code != ExitBlocked || !strings.Contains(out, "nothing waits") {
		t.Fatalf("a second --accept: exit %d: %s", code, out)
	}

	// Within budget goes ahead; past it is refused and points to a Decision.
	g.Mu.Lock()
	g.Issues[5].Labels = []fake.Label{{Name: "type:story"}, {Name: "effort:XS"}}
	g.Mu.Unlock()
	if code, rep, out := scopeRun(agent, "--add", "5"); code != 0 || rep.Drift == nil || rep.Drift.DriftPct != 12.5 || milestone(5) != "v0.3.0" {
		t.Fatalf("--add within budget: exit %d: %s", code, out)
	}
	if code, _, out := scopeRun(agent, "--drop", "3"); code != ExitBlocked || !strings.Contains(out, "50.0% drift") || !strings.Contains(out, "aayu decide") || milestone(3) != "v0.3.0" {
		t.Fatalf("--drop over budget: exit %d: %s", code, out)
	}
	if code, _, out := scopeRun(agent, "--drop", "3", "--decision", "7"); code != ExitBlocked || !strings.Contains(out, "Decision #7 is not accepted") {
		t.Fatalf("--decision not accepted: exit %d: %s", code, out)
	}
	g.Mu.Lock()
	decision.State, decision.StateReason = "closed", "completed"
	decision.Body = workitems.SetSection(decision.Body, "Decision", "Option 1, accepted by @founder on 2026-10-18.")
	g.Mu.Unlock()
	if code, rep, out := scopeRun(agent, "--drop", "3", "--decision", "7"); code != 0 || rep.Request == nil || rep.Request.Kind != "drop" || milestone(3) != "v0.3.0" {
		t.Fatalf("--drop asks a person: exit %d: %s", code, out)
	}
	code, list, _ := run(t, agent, opts, "inbox", "--list", "--json", "--repo", "o/r")
	if code != 0 || !strings.Contains(list, `"kind": "scope"`) || !strings.Contains(list, "aayu scope 1 --accept") {
		t.Fatalf("inbox --list: exit %d:\n%s", code, list)
	}
	approve()
	if code, _, out := scopeRun(person, "--accept"); code != 0 || milestone(3) != "" || started() != 1 {
		t.Fatalf("person accepts the drop: exit %d: %s", code, out)
	}
	var drift scope.Drift
	ledgerJSON(scope.DriftPath("v0.3.0"), &drift)
	if drift.DriftPct != 50 || drift.OverBudgetDecision != 7 || len(drift.Removed) != 1 || drift.Removed[0].Number != 3 || len(drift.Added) != 1 {
		t.Fatalf("drift: %+v", drift)
	}

	// Feedback: by milestone and horizon label, never its body, and not drift.
	if code, _, out := scopeRun(agent, "--assign", "6", "--to", "next"); code != ExitBlocked || !strings.Contains(out, "no release milestone after v0.3.0") {
		t.Fatalf("--to next with none: exit %d: %s", code, out)
	}
	g.Mu.Lock()
	g.Milestones = append(g.Milestones, &fake.Milestone{Number: 2, Title: "v0.10.0", State: "open"}, &fake.Milestone{Number: 3, Title: "v0.4.0", State: "open"})
	g.Mu.Unlock()
	if code, _, out := scopeRun(agent, "--assign", "6", "--to", "next"); code != 0 || milestone(6) != "v0.4.0" || !slices.Contains(g.LabelNames(6), "horizon:next") || slices.Contains(g.LabelNames(6), "horizon:now") {
		t.Fatalf("--to next: exit %d, %q %v: %s", code, milestone(6), g.LabelNames(6), out)
	}
	if code, rep, out := scopeRun(agent, "--assign", "6", "--to", "backlog"); code != 0 || milestone(6) != "" || !slices.Contains(g.LabelNames(6), "horizon:later") || rep.Drift.DriftPct != 50 {
		t.Fatalf("--to backlog: exit %d: %s", code, out)
	}
	if fb.Body != feedbackBody {
		t.Fatalf("Feedback body changed:\n%s", fb.Body)
	}

	// Hand-off: the Release ships the Feature, which aayu release expands to its stories.
	if code, _, out := scopeRun(agent, "--handoff"); code != 0 {
		t.Fatalf("--handoff: exit %d: %s", code, out)
	}
	g.Mu.Lock()
	rel, _ := workitems.Parse(1, release.Title, release.Body, "", []string{"type:release"})
	links, comments := rel.Fields[workitems.FieldLinks], len(release.Comments)
	g.Mu.Unlock()
	if strings.TrimSpace(links) != "includes #2" || comments != 2 {
		t.Fatalf("hand-off: links %q, %d comments\n%s", links, comments, release.Body)
	}
}
