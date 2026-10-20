package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
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
	g.Milestones = []*fake.Milestone{{Number: 1, Title: "v0.3.0", Description: "made by hand", State: "open"}, // no owner yet
		{Number: 11, Title: "v0.3.0", State: "closed"}} // a shipped one with the same title: the open one is the plan
	dir, bare := ledgerCheckout(t)
	agent := map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "AAYU_AGENT": "1", "XDG_STATE_HOME": filepath.Join(dir, "state")}
	person := map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "XDG_STATE_HOME": filepath.Join(dir, "state")} // a terminal with no agent marker
	opts := Options{IsTerminal: stdinTTY, GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir, DevicePoll: 10 * time.Millisecond}
	scopeRun := func(env map[string]string, args ...string) (int, ScopeReport, string) {
		t.Helper()
		code, out, errOut := run(t, env, opts, append([]string{"scope", "1", "--repo", "o/r", "--json", "--set", "identity.app_slug=aayudha-r"}, args...)...)
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

	// A budget past 100% would allow replacing the whole plan without a person.
	if code, _, out := scopeRun(agent, "--add", "2", "--set", "scope.budget_percent=200"); code != ExitUsage || !strings.Contains(out, "budget_percent is 200") {
		t.Fatalf("budget_percent 200: exit %d: %s", code, out)
	}

	// Plan: before a baseline, changes are free; the milestone is made on the first add.
	for _, n := range []string{"2", "3", "4"} {
		if code, _, out := scopeRun(agent, "--add", n); code != 0 {
			t.Fatalf("--add %s: exit %d: %s", n, code, out)
		}
	}
	g.Mu.Lock()
	claimed := g.Milestones[0].Description
	g.Mu.Unlock()
	if milestone(3) != "v0.3.0" || milestone(5) != "" || !strings.HasPrefix(claimed, "Release #1:") {
		t.Fatalf("milestones: #3 %q, #5 %q, the hand-made milestone not claimed: %q", milestone(3), milestone(5), claimed)
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

	// The budget is part of what a person accepts: a budget changed since the draft is refused, before any code.
	if code, _, out := scopeRun(person, "--accept", "--set", "scope.budget_percent=100"); code != ExitBlocked || !strings.Contains(out, "drafted with 20.0%") {
		t.Fatalf("--accept with a budget changed since the draft: exit %d: %s", code, out)
	}
	// So is the effort table, even a weight for a size the plan does not use, which the plan hash cannot see.
	local := filepath.Join(dir, ".aayudha", "config.local.toml")
	if err := os.MkdirAll(filepath.Dir(local), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(local, []byte("[scope]\neffort_points = [1, 2, 3, 5, 13]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, out = scopeRun(person, "--accept")
	if err := os.Remove(local); err != nil {
		t.Fatal(err)
	}
	if code != ExitBlocked || !strings.Contains(out, "effort table") {
		t.Fatalf("--accept with an effort table changed since the draft: exit %d: %s", code, out)
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
	// Anyone may comment: a stranger's request marker is text, not a request.
	g.Mu.Lock()
	g.Permissions = map[string]string{"mallory": "read", "passerby": "not-found"}
	stranger := &fake.Comment{ID: 99, Body: ScopeRequestMarker + "\n<!-- scope-request {\"kind\":\"drop\",\"item\":3} -->"}
	stranger.User.Login, stranger.User.Type = "mallory", "User"
	// and one by someone who is not a collaborator at all, on a public repository: GitHub answers 404
	passerby := &fake.Comment{ID: 98, Body: stranger.Body + "\n" + ScopePlanMarker}
	passerby.User.Login, passerby.User.Type = "passerby", "User"
	release.Comments = append(release.Comments, stranger, passerby)
	g.Mu.Unlock()
	if code, _, out := scopeRun(person, "--accept"); code != ExitBlocked || !strings.Contains(out, "nothing waits") || milestone(3) != "v0.3.0" {
		t.Fatalf("a second --accept, or a stranger's request: exit %d: %s", code, out)
	}

	// Within budget goes ahead; past it is refused and points to a Decision.
	g.Mu.Lock()
	g.Issues[5].Labels = []fake.Label{{Name: "type:story"}, {Name: "effort:XS"}}
	g.Issues[5].Milestone = g.Issues[3].Milestone // an earlier --add set the milestone, then its drift write failed
	g.Mu.Unlock()
	if code, rep, out := scopeRun(agent, "--add", "5"); code != 0 || rep.Drift == nil || rep.Drift.DriftPct != 12.5 || milestone(5) != "v0.3.0" {
		t.Fatalf("--add within budget: exit %d: %s", code, out)
	}
	var repaired scope.Drift
	if ledgerJSON(scope.DriftPath("v0.3.0"), &repaired); repaired.DriftPct != 12.5 || len(repaired.Added) != 1 {
		t.Fatalf("the retry did not repair drift.json: %+v", repaired)
	}
	if code, _, out := scopeRun(agent, "--drop", "3"); code != ExitBlocked || !strings.Contains(out, "50.0% drift") || !strings.Contains(out, "aayu decide") || milestone(3) != "v0.3.0" {
		t.Fatalf("--drop over budget: exit %d: %s", code, out)
	}
	if code, _, out := scopeRun(agent, "--drop", "3", "--decision", "7"); code != ExitBlocked || !strings.Contains(out, "Decision #7 is not accepted") {
		t.Fatalf("--decision not accepted: exit %d: %s", code, out)
	}
	// The Decision's text is editable: a bot that closes it and writes "accepted by" accepts nothing.
	g.Mu.Lock()
	decision.State, decision.StateReason = "closed", "completed"
	decision.Body = workitems.SetSection(decision.Body, "Decision", "Option 1, accepted by @founder on 2026-10-18.")
	decision.ClosedBy = &fake.Account{Login: "aayudha-r[bot]", Type: "Bot"}
	g.Mu.Unlock()
	if code, _, out := scopeRun(agent, "--drop", "3", "--decision", "7"); code != ExitBlocked || !strings.Contains(out, "not accepted by a person") {
		t.Fatalf("--decision closed by a bot: exit %d: %s", code, out)
	}
	g.Mu.Lock()
	decision.ClosedBy = &fake.Account{Login: "founder", Type: "User"}
	g.Mu.Unlock()
	if code, rep, out := scopeRun(agent, "--drop", "3", "--decision", "7"); code != 0 || rep.Request == nil || rep.Request.Kind != "drop" || milestone(3) != "v0.3.0" {
		t.Fatalf("--drop asks a person: exit %d: %s", code, out)
	}
	code, list, _ := run(t, agent, opts, "inbox", "--list", "--json", "--repo", "o/r")
	if code != 0 || !strings.Contains(list, `"kind": "scope"`) || !strings.Contains(list, "aayu scope 1 --accept") {
		t.Fatalf("inbox --list: exit %d:\n%s", code, list)
	}
	// A change by hand while the request waits: the person accepts only the plan the request showed.
	g.Mu.Lock()
	l.Labels = []fake.Label{{Name: "type:story"}, {Name: "effort:XL"}}
	g.Mu.Unlock()
	if code, _, out := scopeRun(person, "--accept"); code != ExitBlocked || !strings.Contains(out, "plan changed since the request") || milestone(3) != "v0.3.0" {
		t.Fatalf("--accept after a change by hand: exit %d: %s", code, out)
	}
	// A drop waits; the item is retyped to Feedback: it leaves the plan by --assign, never by accepting the drop.
	g.Mu.Lock()
	l.Labels = []fake.Label{{Name: "type:story"}, {Name: "effort:L"}}
	m.Labels = []fake.Label{{Name: "type:feedback"}}
	g.Mu.Unlock()
	if code, _, out := scopeRun(person, "--accept"); code != ExitBlocked || !strings.Contains(out, "is a feedback now") || milestone(3) != "v0.3.0" {
		t.Fatalf("--accept a drop of an item retyped to Feedback: exit %d: %s", code, out)
	}
	g.Mu.Lock()
	m.Labels = []fake.Label{{Name: "type:story"}, {Name: "effort:M"}}
	// The bot posted this request: the person cannot edit it, so the acceptance is a new comment.
	g.AuthorOnlyEdits = true
	var asked *fake.Comment
	for _, c := range release.Comments {
		if strings.Contains(c.Body, `"decision":7`) {
			asked = c
		}
	}
	asked.User.Login, asked.User.Type = "aayudha-r[bot]", "Bot"
	askedBody := asked.Body
	// While it waits, someone moves #3 by hand to a shipped milestone with the next release's title.
	shippedNext, openNext := &fake.Milestone{Number: 9, Title: "v0.3.9", State: "closed"}, &fake.Milestone{Number: 10, Title: "v0.3.9", State: "open"}
	g.Milestones = append(g.Milestones, shippedNext, openNext)
	m.Milestone = shippedNext
	g.Mu.Unlock()
	approve()
	code, _, out = scopeRun(person, "--accept")
	g.Mu.Lock()
	rides := m.Milestone == openNext
	g.Milestones, m.Milestone = g.Milestones[:len(g.Milestones)-2], nil // back to no later release
	g.Mu.Unlock()
	if code != 0 || !rides || started() != 1 {
		t.Fatalf("person accepts the drop: exit %d, #3 rides the open next release %v: %s", code, rides, out)
	}
	g.Mu.Lock()
	newest := release.Comments[len(release.Comments)-1]
	g.Mu.Unlock()
	if asked.Body != askedBody || !strings.Contains(newest.Body, "Accepted (aayu scope)") || newest.User.Login != "founder" {
		t.Fatalf("acceptance of a bot's request: asked %q, newest by %s %q", asked.Body, newest.User.Login, newest.Body)
	}
	if code, _, out := scopeRun(person, "--accept"); code != ExitBlocked || !strings.Contains(out, "nothing waits") {
		t.Fatalf("the bot's request still waits: exit %d: %s", code, out)
	}
	var drift scope.Drift
	ledgerJSON(scope.DriftPath("v0.3.0"), &drift)
	if drift.DriftPct != 50 || drift.OverBudgetDecision != 7 || len(drift.Removed) != 1 || drift.Removed[0].Number != 3 || len(drift.Added) != 1 {
		t.Fatalf("drift: %+v", drift)
	}

	// Over budget on Decision 7, a change the Decision does not hold asks a person, even one worth zero points.
	epic := g.AddIssue("Release notes", fixtureBody(t, "feature", nil), "type:feature")
	if code, _, out := scopeRun(agent, "--add", strconv.Itoa(epic.Number)); code != ExitBlocked || !strings.Contains(out, "a person decides") || milestone(epic.Number) != "" {
		t.Fatalf("a zero-point add over budget: exit %d, milestone %q: %s", code, milestone(epic.Number), out)
	}

	// After another writer's acceptance sits above the asker's own request, the next request still reaches a person.
	g.Mu.Lock()
	asked.User.Login, asked.User.Type = "founder", "User"
	newest.User.Login = "maintainer"
	g.Permissions["maintainer"] = "maintain" // GitHub's Maintain role is a writer, so readers stop at its acceptance
	g.Mu.Unlock()
	if code, rep, out := scopeRun(agent, "--add", strconv.Itoa(epic.Number), "--decision", "7"); code != 0 || rep.Request == nil {
		t.Fatalf("a request after another's acceptance: exit %d: %s", code, out)
	}
	if code, list, _ := run(t, agent, opts, "inbox", "--list", "--json", "--repo", "o/r"); code != 0 || !strings.Contains(list, fmt.Sprintf("add #%d", epic.Number)) {
		t.Fatalf("the next request is hidden under the acceptance: exit %d:\n%s", code, list)
	}

	// Feedback: by milestone and horizon label, never its body, and not drift.
	if code, _, out := scopeRun(agent, "--assign", "6", "--to", "next"); code != ExitBlocked || !strings.Contains(out, "no release milestone after v0.3.0") {
		t.Fatalf("--to next with none: exit %d: %s", code, out)
	}
	g.Mu.Lock()
	g.Milestones = append(g.Milestones, &fake.Milestone{Number: 2, Title: "v0.10.0", State: "open"},
		&fake.Milestone{Number: 6, Title: "v0.4.0", State: "closed"}, // GitHub allows one title twice: the shipped one is never the next
		&fake.Milestone{Number: 3, Title: "v0.4.0", State: "open"},
		&fake.Milestone{Number: 4, Title: "v0.3.5", State: "closed"}) // a shipped release is never the next one
	g.Mu.Unlock()
	msNumber := func(n int) int {
		g.Mu.Lock()
		defer g.Mu.Unlock()
		if ms := g.Issues[n].Milestone; ms != nil {
			return ms.Number
		}
		return 0
	}
	if code, _, out := scopeRun(agent, "--assign", "6", "--to", "next"); code != 0 || msNumber(6) != 3 || !slices.Contains(g.LabelNames(6), "horizon:next") || slices.Contains(g.LabelNames(6), "horizon:now") {
		t.Fatalf("--to next: exit %d, %q %v: %s", code, milestone(6), g.LabelNames(6), out)
	}
	if code, rep, out := scopeRun(agent, "--assign", "6", "--to", "backlog"); code != 0 || milestone(6) != "" || !slices.Contains(g.LabelNames(6), "horizon:later") || rep.Drift.DriftPct != 50 {
		t.Fatalf("--to backlog: exit %d: %s", code, out)
	}
	if fb.Body != feedbackBody {
		t.Fatalf("Feedback body changed:\n%s", fb.Body)
	}

	// A Decision covers the changes it was accepted for, never a later one made by hand.
	g.Mu.Lock()
	g.Issues[4].Labels = []fake.Label{{Name: "type:story"}, {Name: "effort:M"}}
	g.Mu.Unlock()
	if code, _, out := scopeRun(agent, "--handoff"); code != ExitBlocked || !strings.Contains(out, "no accepted Decision holds") {
		t.Fatalf("--handoff after a resize by hand: exit %d: %s", code, out)
	}
	// Nor does a new request sweep that resize under the next person's acceptance.
	if code, _, out := scopeRun(agent, "--add", strconv.Itoa(epic.Number), "--decision", "7"); code != ExitBlocked || !strings.Contains(out, "no Decision holds") {
		t.Fatalf("a request beside drift no Decision holds: exit %d: %s", code, out)
	}
	// A retried --add of an item already in the plan neither clears the Decision nor says "nothing to do".
	if code, _, out := scopeRun(agent, "--add", "4"); code != ExitBlocked || !strings.Contains(out, "no accepted Decision holds") {
		t.Fatalf("a no-op --add beside a resize by hand: exit %d: %s", code, out)
	}
	var kept scope.Drift
	if ledgerJSON(scope.DriftPath("v0.3.0"), &kept); kept.OverBudgetDecision != 7 {
		t.Fatalf("the no-op --add cleared the Decision: %+v", kept)
	}
	// A change that cuts that drift but leaves it over budget still needs a person.
	if code, _, out := scopeRun(agent, "--drop", "5"); code != ExitBlocked || !strings.Contains(out, "no Decision holds") || milestone(5) != "v0.3.0" {
		t.Fatalf("a cut that stays over budget: exit %d, #5 in %q: %s", code, milestone(5), out)
	}
	g.Mu.Lock()
	g.Issues[4].Labels = []fake.Label{{Name: "type:story"}, {Name: "effort:L"}}
	g.Mu.Unlock()

	// Hand-off: the planned Stories, not their Feature, which aayu release would expand to every Story under it.
	if code, _, out := scopeRun(agent, "--handoff"); code != 0 {
		t.Fatalf("--handoff: exit %d: %s", code, out)
	}
	// A closed milestone is a shipped release: scope never plans into it.
	g.Mu.Lock()
	g.Milestones = append(g.Milestones, &fake.Milestone{Number: 5, Title: "v0.5.0", State: "closed"})
	g.Mu.Unlock()
	shipped := g.AddIssue("v0.5.0", fixtureBody(t, "release", func(f map[string]string) { f["version"], f[workitems.FieldLinks] = "v0.5.0", "" }), "type:release")
	if code, _, errOut := run(t, agent, opts, "scope", strconv.Itoa(shipped.Number), "--repo", "o/r", "--add", "3"); code != ExitBlocked || !strings.Contains(errOut, "milestone v0.5.0 is closed") || milestone(3) != "" {
		t.Fatalf("a closed version milestone: exit %d: %s", code, errOut)
	}
	// Two open milestones with the release's title: scope does not guess which one is the plan.
	g.Mu.Lock()
	g.Milestones = append(g.Milestones, &fake.Milestone{Number: 12, Title: "v0.3.0", State: "open"})
	g.Mu.Unlock()
	if code, _, out := scopeRun(agent); code != ExitBlocked || !strings.Contains(out, "two open milestones") {
		t.Fatalf("two open v0.3.0 milestones: exit %d: %s", code, out)
	}
	g.Mu.Lock()
	g.Milestones = g.Milestones[:len(g.Milestones)-1]
	g.Mu.Unlock()
	// One version, one Release: a second Release for v0.3.0 cannot use the first one's records.
	twin := g.AddIssue("v0.3.0 again", fixtureBody(t, "release", func(f map[string]string) { f["version"], f[workitems.FieldLinks] = "v0.3.0", "" }), "type:release")
	if code, _, errOut := run(t, agent, opts, "scope", strconv.Itoa(twin.Number), "--repo", "o/r", "--add", "3"); code != ExitBlocked || !strings.Contains(errOut, "belongs to Release #1") || milestone(3) != "" {
		t.Fatalf("a second Release for one version: exit %d: %s", code, errOut)
	}
	g.Mu.Lock()
	rel, _ := workitems.Parse(1, release.Title, release.Body, "", []string{"type:release"})
	links, comments := rel.Fields[workitems.FieldLinks], len(release.Comments)
	g.Mu.Unlock()
	if strings.TrimSpace(links) != "includes #4\nincludes #5" || comments != 7 {
		t.Fatalf("hand-off: links %q, %d comments\n%s", links, comments, release.Body)
	}
	// After hand-off, a change keeps the plan ready to hand off: a Summary edited since the baseline stops it.
	g.Mu.Lock()
	intentBody := release.Body
	release.Body = workitems.SetSection(release.Body, workitems.FieldSummary, "A different release.")
	g.Mu.Unlock()
	if code, _, out := scopeRun(agent, "--drop", "5"); code != ExitBlocked || !strings.Contains(out, "Summary changed") || milestone(5) != "v0.3.0" {
		t.Fatalf("--drop after hand-off with a new Summary: exit %d, #5 in %q: %s", code, milestone(5), out)
	}
	g.Mu.Lock()
	release.Body = intentBody
	g.Mu.Unlock()
	// A dropped item rides the next open release, not the backlog.
	if code, rep, out := scopeRun(agent, "--drop", "5"); code != 0 || milestone(5) != "v0.4.0" || !strings.Contains(rep.Done, "it rides v0.4.0") {
		t.Fatalf("--drop with a later release: exit %d, milestone %q: %s", code, milestone(5), out)
	}
	// After hand-off, a change also changes what aayu release ships.
	g.Mu.Lock()
	rel, _ = workitems.Parse(1, release.Title, release.Body, "", []string{"type:release"})
	g.Mu.Unlock()
	if links := strings.TrimSpace(rel.Fields[workitems.FieldLinks]); links != "includes #4" {
		t.Fatalf("links after a drop past hand-off: %q", links)
	}
	// Hand-off is the plan comment, not the links: with no includes left (a plan
	// emptied of Stories), the next change still writes them.
	g.Mu.Lock()
	release.Body = workitems.SetSection(release.Body, workitems.FieldLinks, "")
	g.Mu.Unlock()
	if code, _, out := scopeRun(agent, "--add", "4"); code != 0 {
		t.Fatalf("--add 4 again: exit %d: %s", code, out)
	}
	g.Mu.Lock()
	rel, _ = workitems.Parse(1, release.Title, release.Body, "", []string{"type:release"})
	g.Mu.Unlock()
	if links := strings.TrimSpace(rel.Fields[workitems.FieldLinks]); links != "includes #4" {
		t.Fatalf("links after hand-off with none left: %q", links)
	}
	// A Story put in the milestone by hand, over budget: a retried --add never ships it without a person.
	byHand := g.AddIssue("Planned by hand", fixtureBody(t, "story", nil), "type:story", "effort:L")
	g.Mu.Lock()
	byHand.Milestone = g.Issues[4].Milestone
	g.Mu.Unlock()
	if code, _, out := scopeRun(agent, "--add", strconv.Itoa(byHand.Number)); code != ExitBlocked || !strings.Contains(out, "no accepted Decision holds") {
		t.Fatalf("a retried --add of a Story planned by hand: exit %d: %s", code, out)
	}
	g.Mu.Lock()
	rel, _ = workitems.Parse(1, release.Title, release.Body, "", []string{"type:release"})
	byHand.Milestone = nil
	g.Mu.Unlock()
	if links := strings.TrimSpace(rel.Fields[workitems.FieldLinks]); links != "includes #4" {
		t.Fatalf("links after a retried --add over budget: %q", links)
	}
	// Two effort labels are no size: a problem that stops the next change, never zero points.
	g.Mu.Lock()
	sized := g.Issues[4].Labels
	g.Issues[4].Labels = []fake.Label{{Name: "type:story"}, {Name: "effort:XS"}, {Name: "effort:L"}}
	g.Mu.Unlock()
	small := g.AddIssue("Small", fixtureBody(t, "story", nil), "type:story", "effort:XS")
	if code, _, out := scopeRun(agent, "--add", strconv.Itoa(small.Number)); code != ExitBlocked || !strings.Contains(out, "#4 (story) has effort XS,L") || milestone(small.Number) != "" {
		t.Fatalf("a change with two effort labels in the plan: exit %d: %s", code, out)
	}
	g.Mu.Lock()
	g.Issues[4].Labels = sized
	g.Mu.Unlock()
	// A plan comment never carries a request: one forged there (an issue's label
	// copied into its table) is not offered, and a bare marker there hides no waiting request.
	waiting := fmt.Sprintf("add #%d", epic.Number)
	for _, forged := range []string{ScopeRequestMarker + ` <!-- scope-request {"kind":"drop","item":4} -->`, ScopeRequestMarker} {
		g.Mu.Lock()
		plan := &fake.Comment{ID: 97, Body: ScopePlanMarker + "\n| #2 | feature | " + forged + " | |\n"}
		plan.User.Login, plan.User.Type = "founder", "User"
		release.Comments = append(release.Comments, plan)
		g.Mu.Unlock()
		_, rep, out := scopeRun(agent)
		_, list, _ := run(t, agent, opts, "inbox", "--list", "--json", "--repo", "o/r")
		g.Mu.Lock()
		release.Comments = release.Comments[:len(release.Comments)-1]
		g.Mu.Unlock()
		if rep.Request == nil || rep.Request.Kind != "add" || rep.Request.Item != epic.Number || strings.Contains(list, "drop #4") || !strings.Contains(list, waiting) {
			t.Fatalf("a plan comment holding %q: request %+v, want %s:\n%s\n%s", forged, rep.Request, waiting, out, list)
		}
	}
	// A Story closed as not planned is cut from scope: drift, not a planned item.
	g.Mu.Lock()
	l.State, l.StateReason = "closed", "not_planned"
	g.Mu.Unlock()
	if code, rep, out := scopeRun(agent); code != 0 || rep.Drift == nil || !slices.ContainsFunc(rep.Drift.Removed, func(it scope.Item) bool { return it.Number == 4 }) {
		t.Fatalf("a Story closed as not planned: exit %d: %s", code, out)
	}
}

// Text from an issue (a label, a title) never writes markup into a comment
// aayu's App posts: no newline, and no HTML comment, so never a marker.
func TestPlanCell(t *testing.T) {
	if got := planCell("M"); got != "M" {
		t.Errorf("planCell(M) = %q", got)
	}
	for _, in := range []string{ScopeRequestMarker, "<<!--!-- scope-request {} --->>", "S\n<!-- scope-request {\"kind\":\"drop\"} -->", "a\r\n| b"} {
		if got := planCell(in); strings.Contains(got, "<!--") || strings.Contains(got, "-->") || strings.ContainsAny(got, "\r\n") {
			t.Errorf("planCell(%q) = %q still carries markup", in, got)
		}
	}
}

// A Decision covers the changes accepted with it: a zero-point Feature retyped
// as a Story with the same effort is a new change, not one the Decision held.
func TestScopeDecisionCoversOnlyItsChanges(t *testing.T) {
	recorded := scope.Drift{Added: []scope.Item{{Number: 8, Type: "feature", Effort: "S"}, {Number: 9, Type: "story", Effort: "L", Points: 5}}}
	now := scope.Drift{Added: []scope.Item{{Number: 8, Type: "story", Effort: "S", Points: 2}, {Number: 9, Type: "story", Effort: "L", Points: 5}}}
	if within(now, recorded) {
		t.Fatal("a retyped Feature rode the Decision accepted for the Story")
	}
	if !within(scope.Drift{Added: recorded.Added[1:]}, recorded) {
		t.Fatal("a subset of the accepted changes lost its Decision")
	}
	resized := scope.Drift{Resized: []scope.Resize{{Number: 4, Type: "story", Title: "Baseline it", From: "L", To: "M"}}}
	if retitled := (scope.Drift{Resized: []scope.Resize{{Number: 4, Type: "story", Title: "Baseline the plan", From: "L", To: "M"}}}); !within(retitled, resized) {
		t.Fatal("a title edit cost a resize its Decision")
	}
}

// The request protocol end to end, with the bot asking and a person
// accepting: only an author edits a comment, so each acceptance is a new
// comment, and the bot's next request goes below it, where every reader looks.
// A re-drafted plan is a new card, and a Decision reopened since holds nothing.
func TestScopeRequestWalk(t *testing.T) {
	g := fake.New(t, "o/r", "User")
	g.Viewer, g.AuthorOnlyEdits = "founder", true
	g.TokenLogins = map[string]fake.Account{"bot-token": {Login: "aayudha-bot[bot]", Type: "Bot"}}
	release := g.AddIssue("v0.3.0", fixtureBody(t, "release", func(f map[string]string) {
		f["version"], f[workitems.FieldLinks] = "v0.3.0", ""
	}), "type:release") // #1
	g.AddIssue("Draft a plan", fixtureBody(t, "story", nil), "type:story", "effort:M")     // #2, 3 points
	l := g.AddIssue("Baseline it", fixtureBody(t, "story", nil), "type:story", "effort:L") // #3, 5 points
	g.AddIssue("Assign feedback", fixtureBody(t, "story", nil), "type:story", "effort:S")  // #4, 2 points
	decision := g.AddIssue("Let #4 into v0.3.0", fixtureBody(t, "decision", func(f map[string]string) {
		f[workitems.FieldSummary] += " It decides the scope of Release #1."
	}), "type:decision") // #5
	decision.State, decision.StateReason = "closed", "completed"
	decision.Body = workitems.SetSection(decision.Body, "Decision", "Option 1, accepted by @founder on 2026-10-19.")
	decision.ClosedBy = &fake.Account{Login: "founder", Type: "User"}
	dir, bare := ledgerCheckout(t)
	state := filepath.Join(dir, "state")
	bot := map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "AAYU_BOT_TOKEN": "bot-token", "AAYU_AGENT": "1", "XDG_STATE_HOME": state}
	person := map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "XDG_STATE_HOME": state}
	opts := Options{IsTerminal: stdinTTY, GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir, DevicePoll: 10 * time.Millisecond}
	app := []string{"--repo", "o/r", "--set", "identity.app_id=42", "--set", "identity.app_slug=aayudha-bot"}
	scopeRun := func(env map[string]string, args ...string) (int, ScopeReport, string) {
		t.Helper()
		pre := []string{"scope", "1", "--json", "--repo", "o/r"}
		pre = append(pre, app[4:]...) // aayu's App: whose requests every reader trusts
		if env["AAYU_BOT_TOKEN"] != "" {
			pre = append(pre, app[2:4]...) // the bot writes as it; the person signs in with their own device code
		}
		code, out, errOut := run(t, env, opts, append(pre, args...)...)
		var rep ScopeReport
		_ = json.Unmarshal([]byte(out), &rep)
		return code, rep, out + errOut
	}
	inbox := func() string {
		t.Helper()
		_, list, _ := run(t, bot, opts, append([]string{"inbox", "--list", "--json"}, app...)...)
		return list
	}
	accept := func() {
		t.Helper()
		g.Mu.Lock()
		g.Device = &fake.DeviceFlow{ClientID: DefaultDeviceClientID, Approver: "founder", Outcome: "approve"}
		g.Mu.Unlock()
		if code, _, out := scopeRun(person, "--accept"); code != 0 {
			t.Fatalf("person --accept: exit %d: %s", code, out)
		}
	}
	thread := func() (who []string, newest string) {
		g.Mu.Lock()
		defer g.Mu.Unlock()
		for _, c := range release.Comments {
			if strings.Contains(c.Body, ScopeRequestMarker) {
				who, newest = append(who, c.User.Login), c.Body
			}
		}
		return who, newest
	}

	for _, n := range []string{"2", "3"} {
		if code, _, out := scopeRun(bot, "--add", n); code != 0 {
			t.Fatalf("--add %s: exit %d: %s", n, code, out)
		}
	}
	_, first, _ := scopeRun(bot, "--draft")
	// A resize by hand, then a new draft: the bot's own request is edited, and the inbox card changes with it.
	g.Mu.Lock()
	l.Labels = []fake.Label{{Name: "type:story"}, {Name: "effort:M"}}
	g.Mu.Unlock()
	code, second, out := scopeRun(bot, "--draft")
	if code != 0 || first.Request == nil || second.Request == nil || first.Request.Hash == second.Request.Hash {
		t.Fatalf("re-draft: exit %d: %s", code, out)
	}
	if list := inbox(); !strings.Contains(list, second.Request.Hash[:12]) || strings.Contains(list, first.Request.Hash[:12]) {
		t.Fatalf("the card after a re-draft does not show the new plan %s:\n%s", second.Request.Hash[:12], list)
	}
	// The version's milestone renamed while the code waits: the plan is read again, and nothing is accepted.
	g.Mu.Lock()
	g.Device = &fake.DeviceFlow{ClientID: DefaultDeviceClientID, Approver: "founder", Outcome: "approve", OnPoll: func(int) {
		g.Milestones[0].Title = "v0.3.0-old"
	}}
	g.Mu.Unlock()
	code, _, out = scopeRun(person, "--accept")
	g.Mu.Lock()
	g.Milestones[0].Title = "v0.3.0"
	g.Mu.Unlock()
	if code != ExitBlocked || !strings.Contains(out, "changed") {
		t.Fatalf("a milestone renamed during the code: exit %d: %s", code, out)
	}
	// 1. bot asks, 2. person accepts: a new comment under the bot's.
	accept()
	if who, newest := thread(); !slices.Equal(who, []string{"aayudha-bot[bot]", "founder"}) || !strings.Contains(newest, "Accepted (aayu scope)") {
		t.Fatalf("after the first acceptance: %v, newest %q", who, newest)
	}
	if code, _, out := scopeRun(bot, "--handoff"); code != 0 {
		t.Fatalf("--handoff: exit %d: %s", code, out)
	}
	// The same plan drafted again with a new budget is a new baseline, not a retry of the old one.
	if code, _, out := scopeRun(bot, "--draft", "--set", "scope.budget_percent=10"); code != 0 {
		t.Fatalf("re-draft with a 10%% budget: exit %d: %s", code, out)
	}
	g.Mu.Lock()
	g.Device = &fake.DeviceFlow{ClientID: DefaultDeviceClientID, Approver: "founder", Outcome: "approve"}
	g.Mu.Unlock()
	if code, rep, out := scopeRun(person, "--accept", "--set", "scope.budget_percent=10"); code != 0 || rep.Baseline != 2 || rep.Drift == nil || rep.Drift.BudgetPct != 10 {
		t.Fatalf("accept the same plan with a 10%% budget: exit %d: %s", code, out)
	}
	// 3. bot asks again, over budget (2 of 6 points): below the acceptance, not in its old comment above it.
	if code, rep, out := scopeRun(bot, "--add", "4", "--decision", "5"); code != 0 || rep.Request == nil || rep.Request.Kind != "add" {
		t.Fatalf("the second request: exit %d: %s", code, out)
	}
	if who, newest := thread(); !slices.Equal(who, []string{"aayudha-bot[bot]", "founder", "aayudha-bot[bot]", "founder", "aayudha-bot[bot]"}) || !strings.Contains(newest, `"kind":"add"`) {
		t.Fatalf("after the second request: %v, newest %q", who, newest)
	}
	// Another installed App's marker comment, newer than the request, neither hides nor replaces it.
	g.Mu.Lock()
	other := &fake.Comment{ID: 98, Body: ScopeRequestMarker + "\n**Accepted (aayu scope):** quoted by another app.\n"}
	other.User.Login, other.User.Type = "other-app[bot]", "Bot"
	release.Comments = append(release.Comments, other)
	g.Mu.Unlock()
	if list := inbox(); !strings.Contains(list, "add #4") {
		t.Fatalf("the second request is not in the inbox:\n%s", list)
	}
	// The baselines removed from the ledger while the code waits: the second read has no baseline, and nothing is accepted.
	seed := filepath.Join(t.TempDir(), "seed")
	git(t, filepath.Dir(seed), "clone", "--quiet", "--branch", "ledger", bare, seed)
	ledgerGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = seed
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("git %v: %v\n%s", args, err, out)
		}
	}
	removed := false
	g.Mu.Lock()
	g.Device = &fake.DeviceFlow{ClientID: DefaultDeviceClientID, Approver: "founder", Outcome: "approve", OnPoll: func(int) {
		if !removed {
			removed = true
			ledgerGit("rm", "--quiet", "releases/v0.3.0/baseline-1.json", "releases/v0.3.0/baseline-2.json")
			ledgerGit("commit", "--quiet", "-m", "remove the baselines")
			ledgerGit("push", "--quiet", "origin", "ledger")
		}
	}}
	g.Mu.Unlock()
	code, _, out = scopeRun(person, "--accept")
	g.Mu.Lock()
	moved := g.Issues[4].Milestone != nil
	g.Mu.Unlock()
	if code != ExitBlocked || !strings.Contains(out, "no baseline") || moved {
		t.Fatalf("the baselines removed during the code: exit %d, #4 moved %v: %s", code, moved, out)
	}
	ledgerGit("revert", "--no-edit", "HEAD")
	ledgerGit("push", "--quiet", "origin", "ledger")
	// A change to the plan while the code waits: nothing is accepted.
	g.Mu.Lock()
	g.Device = &fake.DeviceFlow{ClientID: DefaultDeviceClientID, Approver: "founder", Outcome: "approve", OnPoll: func(int) {
		g.Issues[2].Labels = []fake.Label{{Name: "type:story"}, {Name: "effort:L"}}
	}}
	g.Mu.Unlock()
	code, _, out = scopeRun(person, "--accept")
	g.Mu.Lock()
	moved = g.Issues[4].Milestone != nil
	g.Issues[2].Labels = []fake.Label{{Name: "type:story"}, {Name: "effort:M"}}
	g.Mu.Unlock()
	if code != ExitBlocked || !strings.Contains(out, "changed") || moved {
		t.Fatalf("a plan changed during the code: exit %d, #4 moved %v: %s", code, moved, out)
	}
	// 4. person accepts again; then nothing waits.
	accept()
	g.Mu.Lock()
	planned := g.Issues[4].Milestone != nil && g.Issues[4].Milestone.Title == "v0.3.0"
	rel, _ := workitems.Parse(1, release.Title, release.Body, "", []string{"type:release"})
	g.Mu.Unlock()
	// The plan was handed off, so the accepted add is in what aayu release ships.
	if links := strings.TrimSpace(rel.Fields[workitems.FieldLinks]); links != "includes #2\nincludes #3\nincludes #4" {
		t.Fatalf("links after an accepted add past hand-off: %q", links)
	}
	if who, _ := thread(); !slices.Equal(who, []string{"aayudha-bot[bot]", "founder", "aayudha-bot[bot]", "founder", "aayudha-bot[bot]", "other-app[bot]", "founder"}) || !planned {
		t.Fatalf("after the second acceptance: %v, #4 planned %v", who, planned)
	}
	if code, _, out := scopeRun(person, "--accept"); code != ExitBlocked || !strings.Contains(out, "nothing waits") {
		t.Fatalf("a third --accept: exit %d: %s", code, out)
	}
	// The drift record names Decision #5; once it is reopened, it holds nothing.
	if code, _, out := scopeRun(bot, "--handoff"); code != 0 {
		t.Fatalf("--handoff on an accepted Decision: exit %d: %s", code, out)
	}
	g.Mu.Lock()
	decision.State, decision.StateReason = "open", ""
	g.Mu.Unlock()
	if code, _, out := scopeRun(bot, "--handoff"); code != ExitBlocked || !strings.Contains(out, "no accepted Decision holds") {
		t.Fatalf("--handoff on a reopened Decision: exit %d: %s", code, out)
	}
	// A failed read of the comments is an error, never "nothing waits".
	g.Mu.Lock()
	g.Fail, g.FailAfter = "GET /repos/o/r/issues/1/comments", 0
	g.Mu.Unlock()
	if code, _, out := scopeRun(bot); code != ExitExternal {
		t.Fatalf("a failed comments read: exit %d: %s", code, out)
	}
	// #4 taken out by hand after hand-off, and that plan accepted as a new baseline:
	// what aayu release ships follows the new baseline, not the old links.
	g.Mu.Lock()
	g.Fail = ""
	g.Issues[4].Milestone = nil
	g.Mu.Unlock()
	if code, _, out := scopeRun(bot, "--draft"); code != 0 {
		t.Fatalf("re-draft without #4: exit %d: %s", code, out)
	}
	accept()
	g.Mu.Lock()
	rel, _ = workitems.Parse(1, release.Title, release.Body, "", []string{"type:release"})
	plan := ""
	for _, c := range release.Comments {
		if strings.Contains(c.Body, ScopePlanMarker) {
			plan = c.Body
		}
	}
	g.Mu.Unlock()
	if links := strings.TrimSpace(rel.Fields[workitems.FieldLinks]); links != "includes #2\nincludes #3" || !strings.Contains(plan, "Baseline 3,") || strings.Contains(plan, "| #4 |") {
		t.Fatalf("after a new baseline past hand-off: links %q, plan:\n%s", links, plan)
	}
}
