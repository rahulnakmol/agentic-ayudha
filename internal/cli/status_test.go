package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/notify"
	"github.com/rahulnakmol/agentic-ayudha/internal/status"
)

// statusTree is an asymmetric tree:
//
//	#1 Goal
//	├─ #2 Initiative
//	│  ├─ #4 Feature: #6 done, #7 released, #8 blocked (at-risk, high risk, over budget)
//	│  └─ #5 Feature with a Result: #9 released, #10 cancelled (closable)
//	└─ #3 Initiative
//	   └─ #11 Feature: 105 ready stories #12–#116 (low risk; more than one page of sub-issues),
//	      Decision #117 (open since Monday: overdue; constrains story #12, which shows blocked),
//	      Decision #118 (provisional, opened today; ratify or revert by Friday 13:00 London)
func statusTree(t *testing.T) *fake.GitHub {
	g := fake.New(t, "o/r", "User")
	add := func(parent *fake.Issue, title string, labels ...string) *fake.Issue {
		is := g.AddIssue(title, "", labels...)
		if parent != nil {
			parent.Subs = append(parent.Subs, is.Number)
			is.Parent = parent.Number
		}
		return is
	}
	goal := add(nil, "Halve time to benefit", "type:goal", "status:in-progress")
	i1 := add(goal, "Self-service check", "type:initiative", "status:in-progress")
	i2 := add(goal, "Faster decisions", "type:initiative", "status:in-progress")
	f1 := add(i1, "Questionnaire", "type:feature", "status:in-progress")
	f2 := add(i1, "Eligibility result page", "type:feature", "status:in-progress")
	f2.Body = "### Summary\n\nResult page.\n\n### Result\n\nCompletion +3.1 pts (readout #40)."
	add(f1, "Story A", "type:story", "status:done")
	add(f1, "Story B", "type:story", "status:released")
	add(f1, "Story C", "type:story", "status:blocked", "health:at-risk", "risk:high")
	add(f2, "Story D", "type:story", "status:released")
	add(f2, "Story E", "type:story", "status:cancelled")
	f3 := add(i2, "Decision engine", "type:feature", "status:in-progress")
	for i := range 105 {
		add(f3, fmt.Sprintf("Rule %d", i+1), "type:story", "status:ready", "risk:low")
	}
	d1 := add(f3, "Choose the rules engine", "type:decision", "status:discovery")
	d1.Body = "### Question\n\nWhich engine?\n\n### Links\n\nconstrains #12"
	d2 := add(f3, "Cache decisions", "type:decision", "status:discovery", "decision:provisional")
	d2.Body = "### Question\n\nCache?\n\n### Decision\n\nProvisional: … Ratify or revert by 2026-10-16T12:00:00Z (Fri 16 Oct 2026 13:00 BST)."
	d2.CreatedAt = "2026-10-15T11:00:00Z"
	if goal.Number != 1 || f3.Number != 11 || d2.Number != 118 {
		t.Fatalf("tree numbering: goal %d, feature %d, last decision %d", goal.Number, f3.Number, d2.Number)
	}
	return g
}

// WP2.8 accept: an asymmetric fixture tree with known rollups, and the digest golden file.
func TestStatusRollupsAndDigest(t *testing.T) {
	g := statusTree(t)
	dir := t.TempDir()
	// Spend recorded against stories: #8 is over the 25 USD per-story budget.
	logPath := filepath.Join(dir, "state", "aayudha", "telemetry", "actions.jsonl")
	_ = os.MkdirAll(filepath.Dir(logPath), 0o755)
	_ = os.WriteFile(logPath, []byte(`{"seq":1,"kind":"model.call","story":"#8","status":"done","cost_usd":61}`+"\n"+`{"seq":2,"kind":"model.call","story":"o/r#6","status":"done","cost_usd":1.5}`+"\n"), 0o644)
	sink := &notify.Fake{}
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) // Thursday 13:00 London: business hours
	env := map[string]string{"AAYU_GITHUB_TOKEN": "t", "XDG_STATE_HOME": filepath.Join(dir, "state")}
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1, Now: func() time.Time { return now }, NotifySink: sink, Dir: dir}

	code, out, errOut := run(t, env, opts, "status", "--repo", "o/r", "--json", "--digest")
	if code != 0 {
		t.Fatalf("status: exit %d: %s", code, errOut)
	}
	var rep struct {
		Roots  []*status.Rollup `json:"roots"`
		Digest string           `json:"digest"`
		Sent   string           `json:"digest_status"`
	}
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatal(err)
	}
	if len(rep.Roots) != 1 {
		t.Fatalf("%d roots, want the one open Goal", len(rep.Roots))
	}
	goal := rep.Roots[0]
	i1, i2 := goal.Children[0], goal.Children[1]
	f1, f2, f3 := i1.Children[0], i1.Children[1], i2.Children[0]
	check := func(name string, got, want any) {
		t.Helper()
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
	}
	check("goal items", goal.Items, 117)
	check("goal by status", goal.ByStatus, map[string]int{"done": 1, "released": 2, "blocked": 1, "cancelled": 1, "ready": 105, "in-progress": 5, "discovery": 2})
	check("goal by risk", goal.ByRisk, map[string]int{"high": 1, "low": 105})
	check("goal by health", goal.ByHealth, map[string]int{"at-risk": 1})
	check("goal blocked (status:blocked, or constrained by an open Decision)", goal.Blocked, []int{8, 12})
	check("goal over budget", goal.OverBudget, []int{8})
	check("goal shipped but unmeasured", goal.ShippedUnmeasured, []int{7, 9})
	check("goal provisional", goal.Provisional, []int{118})
	// Thursday 13:00 to Friday 13:00 London: 5 business hours Thursday, 4 Friday.
	check("goal provisional hours", goal.ProvisionalHours, "#118 9 h")
	check("goal cost", goal.CostUSD, 62.5)
	check("goal proof coverage", goal.ProofCoverage, status.ProofNA)
	check("goal decisions near SLA", len(goal.DecisionsNearSLA), 1)
	if d := goal.DecisionsNearSLA[0]; d.Number != 117 || !d.Overdue || d.Deadline.UTC().Format(time.RFC3339) != "2026-10-14T15:00:00Z" {
		t.Errorf("decision near SLA: %+v, want #117 overdue since Wednesday 16:00 London", d)
	}
	check("initiative 1 items", i1.Items, 7)
	check("initiative 1 cost", i1.CostUSD, 62.5)
	check("initiative 2 items", i2.Items, 108)
	check("initiative 2 shipped", i2.ShippedUnmeasured, []int{})
	check("feature 11 items (paginated sub-issues)", f3.Items, 107)
	check("closable", []bool{f1.Closable, f2.Closable, f3.Closable}, []bool{false, true, false})

	golden := filepath.Join("testdata", "status", "digest.golden")
	if *updateGolden {
		_ = os.MkdirAll(filepath.Dir(golden), 0o755)
		_ = os.WriteFile(golden, []byte(rep.Digest), 0o644)
	}
	want, err := os.ReadFile(golden)
	if err != nil || string(want) != rep.Digest {
		t.Errorf("digest differs from %s (-update-golden):\n%s", golden, rep.Digest)
	}
	if rep.Sent != notify.Sent || len(sink.Pages) != 1 || sink.Pages[0].Body != rep.Digest {
		t.Errorf("digest page: %s, %d pages", rep.Sent, len(sink.Pages))
	}
	if w := g.Writes(); len(w) != 0 {
		t.Errorf("status without --apply wrote %v", w)
	}
	// The same day's second digest is deduplicated.
	if code, _, _ := run(t, env, opts, "status", "--repo", "o/r", "--digest"); code != 0 || len(sink.Pages) != 1 {
		t.Errorf("second digest: exit %d, %d pages", code, len(sink.Pages))
	}

	// --apply: the Goal comment, and the parent-close rule closes Feature #5.
	code, out, errOut = run(t, env, opts, "status", "1", "--repo", "o/r", "--apply")
	if code != 0 || !strings.Contains(out, "closed #5") {
		t.Fatalf("status --apply: exit %d: %s %s", code, out, errOut)
	}
	g.Mu.Lock()
	defer g.Mu.Unlock()
	if g.Issues[5].State != "closed" || len(g.Issues[1].Comments) != 1 || !strings.Contains(g.Issues[1].Comments[0].Body, status.CommentMarker) {
		t.Errorf("after --apply: #5 %s, goal comments %d", g.Issues[5].State, len(g.Issues[1].Comments))
	}
	if !slices.ContainsFunc(g.Issues[1].Comments, func(c *fake.Comment) bool { return strings.Contains(c.Body, "| #1 goal Halve time to benefit | 117 |") }) {
		t.Errorf("goal comment:\n%s", g.Issues[1].Comments[0].Body)
	}
}

// A story still labelled status:ready while an open PR builds it counts as in
// progress (megaman: build held a lease and PR #5 was open, and status still
// counted Story #4 as Ready; D-5.5c.18). A PR that only mentions a story does
// not move it.
func TestStatusCountsAStoryWithAnOpenPRInProgress(t *testing.T) {
	g := fake.New(t, "o/r", "User")
	goal := g.AddIssue("Ship sums", "", "type:goal", "status:in-progress")
	building := g.AddIssue("Add mathx sum", "", "type:story", "status:ready")
	waiting := g.AddIssue("Add mathx product", "", "type:story", "status:ready")
	branch := g.AddIssue("Add mathx max", "", "type:story", "status:ready")
	for _, s := range []*fake.Issue{building, waiting, branch} {
		goal.Subs = append(goal.Subs, s.Number)
		s.Parent = goal.Number
	}
	g.AddPR("feat: add mathx sum", fmt.Sprintf("Refs #%d", building.Number), "codex", "codex/sum", strings.Repeat("a", 40), "dev")
	g.AddPR("feat: add mathx max", "", "founder", fmt.Sprintf("story/%d-add-mathx-max", branch.Number), strings.Repeat("b", 40), "dev")
	g.AddPR("docs: notes", fmt.Sprintf("See #%d later.", waiting.Number), "founder", "docs/notes", strings.Repeat("c", 40), "dev")
	env := map[string]string{"AAYU_GITHUB_TOKEN": "t", "XDG_STATE_HOME": filepath.Join(t.TempDir(), "state")}
	code, out, errOut := run(t, env, Options{GitHubAPI: g.URL(), GitHubGap: -1, Dir: t.TempDir()}, "status", "--repo", "o/r", "--json")
	var rep struct {
		Roots []*status.Rollup `json:"roots"`
	}
	if err := json.Unmarshal([]byte(out), &rep); code != 0 || err != nil || len(rep.Roots) != 1 {
		t.Fatalf("status: %d %v %s", code, err, errOut)
	}
	if got := rep.Roots[0].ByStatus; fmt.Sprint(got) != fmt.Sprint(map[string]int{"in-progress": 2, "ready": 1}) {
		t.Fatalf("by status %v, want 2 in progress (Refs #%d, story/%d-…) and 1 ready", got, building.Number, branch.Number)
	}
}
