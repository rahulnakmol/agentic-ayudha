package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/planning"
)

// evalDraft reads a recorded draft from evals/cases and writes it, changed by
// edit, to a temporary file for the fake repository's issue numbers.
func evalDraft[T any](t *testing.T, rel string, edit func(*T)) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "evals", "cases", filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	var d T
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	edit(&d)
	out, _ := json.Marshal(d)
	p := filepath.Join(t.TempDir(), "draft.json")
	if err := os.WriteFile(p, out, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func featureIssue(t *testing.T, g *fake.GitHub, rel string) *fake.Issue {
	t.Helper()
	var is struct {
		Title, Body string
	}
	b, _ := os.ReadFile(filepath.Join("..", "..", "evals", "cases", filepath.FromSlash(rel)))
	if err := json.Unmarshal(b, &is); err != nil {
		t.Fatal(err)
	}
	return g.AddIssue(is.Title, is.Body, "type:feature", "status:planned")
}

// WP5.4 accept: a 6-layer Feature (with an L07 migration) sliced into XS-L
// stories that carry their footprint and dependencies; the checks refuse an
// XL story, a cycle, and an uncovered layer; an XL story is returned as at
// least two stories and closed as "split into".
func TestSlice(t *testing.T) {
	g := fake.New(t, "o/r", "User")
	feature := featureIssue(t, g, "slice/proof-retention-6-layer/github/repos/o/r/issues/40.json") // #1
	env := map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "AAYU_AGENT": "1"}
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1}
	draft := func(edit func(*planning.SliceDraft)) string {
		return evalDraft(t, "slice/proof-retention-6-layer/output/slice.json", func(d *planning.SliceDraft) { d.Feature = feature.Number; edit(d) })
	}
	slice := func(n int, path string, extra ...string) (int, SliceResult, string) {
		code, out, errOut := run(t, env, opts, append([]string{"slice", strconv.Itoa(n), "--repo", "o/r", "--json", "--draft", path}, extra...)...)
		var res SliceResult
		_ = json.NewDecoder(strings.NewReader(out)).Decode(&res)
		return code, res, errOut + out
	}

	for name, tc := range map[string]struct {
		edit func(*planning.SliceDraft)
		want string
	}{
		"xl story": {func(d *planning.SliceDraft) { d.Stories[1].Labels = []string{"effort:XL"} }, "effort:XL blocks Ready"},
		"cycle":    {func(d *planning.SliceDraft) { d.Stories[0].DependsOn = []int{5} }, "depends_on forms a cycle"},
		"uncovered": {func(d *planning.SliceDraft) {
			d.Stories = append(d.Stories[:3], d.Stories[4])
			d.Stories[3].DependsOn = []int{3}
		}, "layer L06 of the Feature's footprint has no story"},
		"outside paths":  {func(d *planning.SliceDraft) { d.Stories[3].Paths = []string{"internal/models/**"} }, "internal/models/** is outside the Feature's expected paths"},
		"no criterion":   {func(d *planning.SliceDraft) { d.Stories[2].Fields["completion_goal"] = "the flag works" }, "not a numbered criterion"},
		"self reference": {func(d *planning.SliceDraft) { d.Stories[2].DependsOn = []int{3} }, "3 is not another story of this draft"},
	} {
		if code, _, out := slice(feature.Number, draft(tc.edit), "--create"); code != ExitBlocked || !strings.Contains(out, tc.want) {
			t.Errorf("%s: exit %d, want 4 with %q:\n%s", name, code, tc.want, out)
		}
	}
	if len(g.Issues) != 1 {
		t.Fatalf("a refused slice wrote %d issues", len(g.Issues)-1)
	}

	code, res, out := slice(feature.Number, draft(func(*planning.SliceDraft) {}), "--create")
	if code != 0 || !res.Pass || strings.Join(res.Footprint, ",") != "L02,L03,L04,L06,L07,L09" || len(res.Issues) != 5 {
		t.Fatalf("slice: %d %s", code, out)
	}
	// Created in dependency order; story 5 depends on stories 3 and 4.
	migration, flag, schedule := res.Issues[0], res.Issues[2], res.Issues[4]
	if labels := g.LabelNames(migration); !contains(labels, "layer:L07") || !contains(labels, "effort:M") || !contains(labels, "identity:human-fallback") {
		t.Fatalf("migration story labels: %v", labels)
	}
	if labels := g.LabelNames(flag); !contains(labels, "layer:L02") || !contains(labels, "layer:L03") {
		t.Fatalf("flag story labels: %v", labels)
	}
	g.Mu.Lock()
	body, parent := g.Issues[schedule].Body, g.Issues[schedule].Parent
	g.Mu.Unlock()
	if parent != feature.Number || !strings.Contains(body, "depends-on #"+strconv.Itoa(flag)+"\ndepends-on #"+strconv.Itoa(res.Issues[3])) {
		t.Fatalf("schedule story: parent %d body:\n%s", parent, body)
	}
	for _, n := range res.Issues {
		if code, out, errOut := run(t, env, opts, "ready", strconv.Itoa(n), "--check", "--repo", "o/r"); code != 0 {
			t.Fatalf("story #%d does not pass ready: %s %s", n, out, errOut)
		}
	}

	// An XL story under the Feature: one story is refused, two close it as split.
	xl := g.AddIssue("Retention everywhere", g.Issues[migration].Body, "type:story", "status:planned", "effort:XL")
	xl.Parent = feature.Number
	feature.Subs = append(feature.Subs, xl.Number)
	two := func(keep int) func(*planning.SliceDraft) {
		return func(d *planning.SliceDraft) {
			d.Feature = xl.Number
			d.Stories = d.Stories[:keep]
			d.Stories[len(d.Stories)-1].DependsOn = nil
			if keep == 2 {
				d.Stories[1].DependsOn = []int{1}
			}
		}
	}
	if code, _, out := slice(xl.Number, evalDraft(t, "slice/proof-retention-6-layer/output/slice.json", two(1)), "--create"); code != ExitBlocked || !strings.Contains(out, "is XL: slice it into at least two XS-L stories") {
		t.Fatalf("XL into one story: %d %s", code, out)
	}
	code, res, out = slice(xl.Number, evalDraft(t, "slice/proof-retention-6-layer/output/slice.json", two(2)), "--create")
	if code != 0 || !res.Closed || len(res.Issues) != 2 {
		t.Fatalf("XL split: %d %s", code, out)
	}
	g.Mu.Lock()
	closed, comments := g.Issues[xl.Number].State, g.Issues[xl.Number].Comments
	newParent := g.Issues[res.Issues[0]].Parent
	g.Mu.Unlock()
	if closed != "closed" || len(comments) != 1 || !strings.Contains(comments[0].Body, "Split into #"+strconv.Itoa(res.Issues[0])+", #"+strconv.Itoa(res.Issues[1])) ||
		!contains(g.LabelNames(xl.Number), "status:cancelled") || newParent != feature.Number {
		t.Fatalf("XL after split: state %s, comments %v, labels %v, new stories under #%d", closed, comments, g.LabelNames(xl.Number), newParent)
	}
}

// WP5.4 accept: a plan has exactly one checkpoint per layer of the story's
// footprint and a rollback on its L07 checkpoint; it is written to the
// ledger with the EM plan-time review an [autonomy] never layer requires,
// which only a human records.
func TestPlan(t *testing.T) {
	g := fake.New(t, "o/r", "User")
	g.Viewer = "founder"
	feature := featureIssue(t, g, "plan/config-migration/github/repos/o/r/issues/41/parent.json") // #1
	var fx struct {
		Title, Body string
		Labels      []struct{ Name string }
	}
	b, _ := os.ReadFile(filepath.Join("..", "..", "evals", "cases", "plan", "config-migration", "github", "repos", "o", "r", "issues", "41.json"))
	_ = json.Unmarshal(b, &fx)
	story := g.AddIssue(fx.Title, fx.Body, "type:story", "status:ready", "effort:M", "layer:L07") // #2
	story.Parent, feature.Subs = feature.Number, []int{2}
	dir, bare := ledgerCheckout(t)
	env := map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "AAYU_AGENT": "1"}
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir}
	draft := func(edit func(*planning.PlanDraft)) string {
		return evalDraft(t, "plan/config-migration/output/plan.json", func(d *planning.PlanDraft) { d.Story = story.Number; edit(d) })
	}
	plan := func(path string, extra ...string) (int, string) {
		code, out, errOut := run(t, env, opts, append([]string{"plan", "2", "--repo", "o/r", "--draft", path}, extra...)...)
		return code, out + errOut
	}
	for name, tc := range map[string]struct {
		edit func(*planning.PlanDraft)
		want string
	}{
		"no rollback": {func(d *planning.PlanDraft) { d.Checkpoints[0].Rollback = "" }, "checkpoint 1 (L07): an operational-data checkpoint needs a rollback step"},
		"two for L07": {func(d *planning.PlanDraft) { d.Checkpoints = append(d.Checkpoints, d.Checkpoints[0]) }, "checkpoint 2: a second checkpoint for L07"},
		"extra layer": {func(d *planning.PlanDraft) {
			c := d.Checkpoints[0]
			c.Layer = "L02"
			d.Checkpoints = append(d.Checkpoints, c)
		}, "checkpoint for L02 is outside the story's footprint (L07)"},
		"missing layer": {func(d *planning.PlanDraft) { d.Checkpoints[0].Layer = "L04" }, "layer L07 of the story's footprint has no checkpoint"},
		"no tests":      {func(d *planning.PlanDraft) { d.Checkpoints[0].Tests = nil }, "name the tests that prove it"},
		"criterion":     {func(d *planning.PlanDraft) { d.Checkpoints[0].Criteria = []int{1} }, "criterion 2 has no checkpoint whose tests prove it"},
	} {
		if code, out := plan(draft(tc.edit), "--create"); code != ExitBlocked || !strings.Contains(out, tc.want) {
			t.Errorf("%s: exit %d, want 4 with %q:\n%s", name, code, tc.want, out)
		}
	}
	code, out := plan(draft(func(*planning.PlanDraft) {}), "--create")
	if code != 0 || !strings.Contains(out, "EM plan-time review required (never layers L07)") || !strings.Contains(out, "wrote specs/2/plan.json") {
		t.Fatalf("plan: %d %s", code, out)
	}
	var rec PlanRecord
	if err := json.Unmarshal([]byte(git(t, bare, "show", "ledger:specs/2/plan.json")), &rec); err != nil || !rec.Review.Required || rec.Review.ApprovedBy != "" || len(rec.Checkpoints) != 1 {
		t.Fatalf("plan record: %v %+v", err, rec)
	}
	if code, _, errOut := run(t, env, opts, "plan", "2", "--approve", "--repo", "o/r"); code != ExitBlocked || !strings.Contains(errOut, "refused for agents") {
		t.Fatalf("approve by an agent: %d %s", code, errOut)
	}
	human := map[string]string{"AAYU_GITHUB_TOKEN": "human-token"}
	if code, _, errOut := run(t, human, opts, "plan", "2", "--approve", "--repo", "o/r", "--set", "autonomy.approver=someone-else"); code != ExitBlocked || !strings.Contains(errOut, "the [autonomy] approver is someone-else") {
		t.Fatalf("approve by a human who is not the approver: %d %s", code, errOut)
	}
	if code, out, errOut := run(t, human, opts, "plan", "2", "--approve", "--repo", "o/r"); code != 0 || !strings.Contains(out, "recorded by founder (as solo)") {
		t.Fatalf("approve: %d %s %s", code, out, errOut)
	}
	_ = json.Unmarshal([]byte(git(t, bare, "show", "ledger:specs/2/plan.json")), &rec)
	if rec.Review.ApprovedBy != "founder" || rec.Review.As != "solo" || rec.Review.ApprovedAt == "" {
		t.Fatalf("approved plan record: %+v", rec.Review)
	}
}
