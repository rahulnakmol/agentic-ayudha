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

	// Solo mode: --solo-bypass records that no one reviewed the plan, only
	// after the human's own "Solo-mode bypass:" comment on the story, and
	// never as an approval.
	if code, _, errOut := run(t, human, opts, "plan", "2", "--approve", "--solo-bypass", "--repo", "o/r"); code != ExitUsage || !strings.Contains(errOut, "pass one") {
		t.Fatalf("approve and bypass together: %d %s", code, errOut)
	}
	if code, _, errOut := run(t, env, opts, "plan", "2", "--solo-bypass", "--repo", "o/r"); code != ExitBlocked || !strings.Contains(errOut, "refused for agents") {
		t.Fatalf("bypass by an agent: %d %s", code, errOut)
	}
	if code, _, errOut := run(t, human, opts, "plan", "2", "--solo-bypass", "--repo", "o/r"); code != ExitBlocked || !strings.Contains(errOut, `post a comment on the story starting "Solo-mode bypass:"`) {
		t.Fatalf("bypass without the comment: %d %s", code, errOut)
	}
	g.Mu.Lock()
	cm := &fake.Comment{ID: 81, Body: "Solo-mode bypass: no second person reviewed this plan; L07 proceeds unreviewed."}
	cm.User.Login, cm.User.Type = "founder", "User"
	g.Issues[2].Comments = append(g.Issues[2].Comments, cm)
	g.Mu.Unlock()
	if code, out, errOut := run(t, human, opts, "plan", "2", "--solo-bypass", "--repo", "o/r"); code != 0 || !strings.Contains(out, "no plan-time review of layers L07; solo-mode bypass recorded by founder") {
		t.Fatalf("bypass: %d %s %s", code, out, errOut)
	}
	_ = json.Unmarshal([]byte(git(t, bare, "show", "ledger:specs/2/plan.json")), &rec)
	if b := rec.Review.Bypass; b == nil || b.As != "solo-bypass" || b.Reviewer != "none (solo mode)" || b.RecordedBy != "founder" ||
		rec.Review.ApprovedBy != "" || rec.Review.As != "" {
		t.Fatalf("bypass record is not a bypass, or claims an approval: %+v %+v", rec.Review, rec.Review.Bypass)
	}
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
	if code, _, errOut := run(t, human, opts, "plan", "2", "--solo-bypass", "--repo", "o/r"); code != ExitUsage || !strings.Contains(errOut, "already reviewed by founder") {
		t.Fatalf("bypass of a reviewed plan: %d %s", code, errOut)
	}
}

// slice without --draft reports what to slice within: the Feature's footprint
// from its expected paths. slice refuses what it cannot slice: a closed
// issue, a story that is not XL, and an XL story without a Feature parent.
func TestSliceSources(t *testing.T) {
	g := fake.New(t, "o/r", "User")
	const fixture = "slice/proof-retention-6-layer/github/repos/o/r/issues/40.json"
	feature := featureIssue(t, g, fixture) // #1
	env := map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "AAYU_AGENT": "1"}
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1}

	code, out, errOut := run(t, env, opts, "slice", "1", "--repo", "o/r")
	if code != 0 || !strings.Contains(out, "#1: Feature #1, footprint L02, L03, L04, L06, L07, L09 from expected paths ") ||
		!strings.Contains(out, "next: draft XS-L stories within these paths (skills/slice), then `aayu slice 1 --draft <file>`") {
		t.Fatalf("slice without a draft: %d %s %s", code, out, errOut)
	}
	code, out, errOut = run(t, env, opts, "slice", "1", "--repo", "o/r", "--json")
	var res SliceResult
	if err := json.Unmarshal([]byte(out), &res); err != nil || code != 0 || res.Pass || res.Feature != 1 ||
		strings.Join(res.Footprint, ",") != "L02,L03,L04,L06,L07,L09" || len(res.ExpectedPaths) == 0 || res.Stories == nil || len(res.Stories) != 0 {
		t.Fatalf("slice --json without a draft: %d %v %s %s", code, err, out, errOut)
	}
	if len(g.Writes()) != 0 {
		t.Fatalf("slice without a draft wrote: %v", g.Writes())
	}

	closed := featureIssue(t, g, fixture) // #2
	closed.State = "closed"
	story := g.AddIssue("A plain story", feature.Body, "type:story", "status:planned", "effort:M")     // #3
	orphan := g.AddIssue("An XL story", feature.Body, "type:story", "status:planned", "effort:XL")     // #4
	underStory := g.AddIssue("An XL story", feature.Body, "type:story", "status:planned", "effort:XL") // #5
	underStory.Parent = story.Number
	for _, tc := range []struct {
		n, code int
		want    string
	}{
		{closed.Number, ExitBlocked, "slice: #2 is closed"},
		{story.Number, ExitUsage, `slice: #3 is a story (effort "M"); slice takes a Feature or an XL story`},
		{orphan.Number, ExitBlocked, "slice: XL story #4 has no parent Feature to slice it under"},
		{underStory.Number, ExitBlocked, "slice: XL story #5's parent #3 is a story, not a Feature"},
	} {
		if code, out, errOut := run(t, env, opts, "slice", strconv.Itoa(tc.n), "--repo", "o/r"); code != tc.code || !strings.Contains(errOut, tc.want) {
			t.Errorf("slice #%d: exit %d, want %d with %q: %s %s", tc.n, code, tc.code, tc.want, out, errOut)
		}
	}

	// A draft is decoded strictly: an unknown field is a usage error, as is a missing file.
	bad := filepath.Join(t.TempDir(), "draft.json")
	_ = os.WriteFile(bad, []byte(`{"feature": 1, "stories": [], "notes": "x"}`), 0o644)
	if code, _, errOut := run(t, env, opts, "slice", "1", "--repo", "o/r", "--draft", bad); code != ExitUsage || !strings.Contains(errOut, `unknown field "notes"`) {
		t.Errorf("draft with an unknown field: %d %s", code, errOut)
	}
	if code, _, errOut := run(t, env, opts, "slice", "1", "--repo", "o/r", "--draft", filepath.Join(t.TempDir(), "none.json")); code != ExitUsage {
		t.Errorf("missing draft: %d %s", code, errOut)
	}
	if len(g.Writes()) != 0 {
		t.Fatalf("a refused slice wrote: %v", g.Writes())
	}
}

// The text report of a slice: per-story and whole-draft problems for a
// draft that fails, the next step for one that passes, --dry-run --create
// planning the stories without writing them, and the created issue numbers.
func TestSliceText(t *testing.T) {
	g := fake.New(t, "o/r", "User")
	feature := featureIssue(t, g, "slice/proof-retention-6-layer/github/repos/o/r/issues/40.json") // #1
	env := map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "AAYU_AGENT": "1"}
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1}
	draft := func(edit func(*planning.SliceDraft)) string {
		return evalDraft(t, "slice/proof-retention-6-layer/output/slice.json", func(d *planning.SliceDraft) { d.Feature = feature.Number; edit(d) })
	}
	slice := func(path string, extra ...string) (int, string) {
		code, out, errOut := run(t, env, opts, append([]string{"slice", "1", "--repo", "o/r", "--draft", path}, extra...)...)
		return code, out + errOut
	}

	code, out := slice(draft(func(d *planning.SliceDraft) {
		d.Stories[1].Labels = []string{"effort:XL"}
		d.Stories[0].DependsOn = []int{5}
	}), "--create")
	if code != ExitBlocked || !strings.Contains(out, "5 stories, does not pass yet") || !strings.Contains(out, "     problem: ") ||
		!strings.Contains(out, "  problem: depends_on forms a cycle") || len(g.Writes()) != 0 {
		t.Fatalf("failing draft: %d %s", code, out)
	}

	good := draft(func(*planning.SliceDraft) {})
	if code, out := slice(good); code != 0 || !strings.Contains(out, "5 stories, passes") || !strings.Contains(out, "run again with --create to write the stories") {
		t.Fatalf("passing draft without --create: %d %s", code, out)
	}
	if code, out := slice(good, "--create", "--dry-run"); code != 0 || !strings.Contains(out, "--dry-run: the stories are planned, not written") || len(g.Writes()) != 0 || len(g.Issues) != 1 {
		t.Fatalf("--dry-run --create: %d, %d writes, %d issues: %s", code, len(g.Writes()), len(g.Issues), out)
	}
	code, out = slice(good, "--create")
	if code != 0 || len(g.Issues) != 6 {
		t.Fatalf("--create: %d, %d issues: %s", code, len(g.Issues), out)
	}
	for n := 2; n <= 6; n++ {
		if !strings.Contains(out, ") → #"+strconv.Itoa(n)+"\n") {
			t.Errorf("the report does not name created story #%d:\n%s", n, out)
		}
	}
}

// plan without --draft reports what to plan within; plan refuses a story
// that does not pass Ready; a story without layer labels takes the plan's
// layers as its footprint, written as its labels; --dry-run writes nothing.
func TestPlanFootprint(t *testing.T) {
	g := fake.New(t, "o/r", "User")
	feature := featureIssue(t, g, "plan/config-migration/github/repos/o/r/issues/41/parent.json") // #1
	var fx struct{ Title, Body string }
	b, _ := os.ReadFile(filepath.Join("..", "..", "evals", "cases", "plan", "config-migration", "github", "repos", "o", "r", "issues", "41.json"))
	_ = json.Unmarshal(b, &fx)
	story := g.AddIssue(fx.Title, fx.Body, "type:story", "status:ready", "effort:M", "layer:L07") // #2
	unlabelled := g.AddIssue(fx.Title, fx.Body, "type:story", "status:ready", "effort:M")         // #3
	unready := g.AddIssue("Half written", "", "type:story", "status:planned")                     // #4
	for _, s := range []*fake.Issue{story, unlabelled, unready} {
		s.Parent = feature.Number
		feature.Subs = append(feature.Subs, s.Number)
	}
	dir, bare := ledgerCheckout(t)
	env := map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "AAYU_AGENT": "1"}
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir}
	draft := func(n int) string {
		return evalDraft(t, "plan/config-migration/output/plan.json", func(d *planning.PlanDraft) { d.Story = n })
	}

	code, out, errOut := run(t, env, opts, "plan", "2", "--repo", "o/r")
	if code != 0 || !strings.Contains(out, "#2 "+fx.Title+": footprint L07; 2 criteria; never layers L04, L07, L09") ||
		!strings.Contains(out, "next: draft one checkpoint per footprint layer (skills/plan), then `aayu plan 2 --draft <file>`") {
		t.Fatalf("plan without a draft: %d %s %s", code, out, errOut)
	}
	code, out, errOut = run(t, env, opts, "plan", "2", "--repo", "o/r", "--json")
	var res PlanResult
	if err := json.Unmarshal([]byte(out), &res); err != nil || code != 0 || res.Story != 2 || strings.Join(res.Footprint, ",") != "L07" ||
		len(res.Criteria) != 2 || !contains(res.Never, "L07") || res.Pass || res.Next == "" {
		t.Fatalf("plan --json without a draft: %d %v %s %s", code, err, out, errOut)
	}

	if code, _, errOut := run(t, env, opts, "plan", "4", "--repo", "o/r"); code != ExitBlocked || !strings.Contains(errOut, "a plan is for a story or bug that passes Ready; this story has") {
		t.Fatalf("plan of a story that is not Ready: %d %s", code, errOut)
	}

	if code, out, errOut := run(t, env, opts, "plan", "2", "--repo", "o/r", "--draft", draft(2)); code != 0 || !strings.Contains(out, "run again with --create to write it to the ledger") {
		t.Fatalf("passing plan without --create: %d %s %s", code, out, errOut)
	}
	if code, out, errOut := run(t, env, opts, "plan", "2", "--repo", "o/r", "--draft", draft(2), "--create", "--dry-run"); code != 0 ||
		!strings.Contains(out, "--dry-run: the plan is planned, not written") || strings.Contains(git(t, bare, "ls-tree", "-r", "--name-only", "ledger"), "specs/") {
		t.Fatalf("plan --dry-run --create: %d %s %s", code, out, errOut)
	}

	if code, _, errOut := run(t, env, opts, "plan", "99", "--repo", "o/r"); code != ExitExternal {
		t.Fatalf("plan of an issue that does not exist: %d %s", code, errOut)
	}

	// The layer labels are written before the plan: when they fail, the
	// ledger has no plan, and a retry writes both.
	g.Fail = "POST /repos/o/r/issues/3/labels"
	if code, _, errOut := run(t, env, opts, "plan", "3", "--repo", "o/r", "--draft", draft(3), "--create"); code != ExitExternal ||
		strings.Contains(git(t, bare, "ls-tree", "-r", "--name-only", "ledger"), "specs/") {
		t.Fatalf("plan whose label write fails: %d %s", code, errOut)
	}
	code, out, errOut = run(t, env, opts, "plan", "3", "--repo", "o/r", "--draft", draft(3), "--create", "--json")
	res = PlanResult{}
	if err := json.Unmarshal([]byte(out), &res); err != nil || code != 0 || !res.FromPlan || strings.Join(res.Footprint, ",") != "L07" || res.Ledger != "specs/3/plan.json" {
		t.Fatalf("plan of a story without layer labels: %d %v %s %s", code, err, out, errOut)
	}
	if labels := g.LabelNames(3); !contains(labels, "layer:L07") {
		t.Fatalf("the plan's layers were not written as labels: %v", labels)
	}
	if code, out, _ := run(t, env, opts, "plan", "3", "--repo", "o/r"); code != 0 || !strings.Contains(out, "footprint L07;") {
		t.Fatalf("plan after the labels were written: %d %s", code, out)
	}
}

// plan --approve records the EM plan-time review only for a plan on the
// ledger that needs one, by the EM seat's paired human when there is one,
// else the [autonomy] approver.
func TestPlanApprove(t *testing.T) {
	g := fake.New(t, "o/r", "User")
	g.Viewer = "founder"
	feature := featureIssue(t, g, "plan/config-migration/github/repos/o/r/issues/41/parent.json") // #1
	var fx struct{ Title, Body string }
	b, _ := os.ReadFile(filepath.Join("..", "..", "evals", "cases", "plan", "config-migration", "github", "repos", "o", "r", "issues", "41.json"))
	_ = json.Unmarshal(b, &fx)
	l07 := g.AddIssue(fx.Title, fx.Body, "type:story", "status:ready", "effort:M", "layer:L07") // #2
	l02 := g.AddIssue(fx.Title, fx.Body, "type:story", "status:ready", "effort:M", "layer:L02") // #3
	l07.Parent, l02.Parent, feature.Subs = feature.Number, feature.Number, []int{2, 3}
	dir, bare := ledgerCheckout(t)
	agent := map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "AAYU_AGENT": "1"}
	human := map[string]string{"AAYU_GITHUB_TOKEN": "human-token"}
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir}

	// A review (--approve) and a solo-mode bypass of one (--solo-bypass) both need a plan, a ledger, and a human.
	root := t.TempDir()
	git(t, root, "init", "--quiet", "--bare", "origin.git")
	git(t, root, "clone", "--quiet", "origin.git", "work")
	noLedger := Options{GitHubAPI: g.URL(), GitHubGap: -1, Dir: filepath.Join(root, "work")}
	for _, flag := range []string{"--approve", "--solo-bypass"} {
		if code, _, errOut := run(t, human, opts, "plan", "2", flag, "--repo", "o/r"); code != ExitBlocked || !strings.Contains(errOut, "#2 has no plan on the ledger") {
			t.Fatalf("%s without a plan: %d %s", flag, code, errOut)
		}
		if code, _, errOut := run(t, human, noLedger, "plan", "2", flag, "--repo", "o/r"); code != ExitBlocked || !strings.Contains(errOut, "#2 has no plan on the ledger") {
			t.Fatalf("%s without a ledger: %d %s", flag, code, errOut)
		}
		// It is a human's: without a human GitHub login it is refused.
		if code, _, errOut := run(t, map[string]string{}, opts, "plan", "2", flag, "--repo", "o/r"); code != ExitBlocked {
			t.Fatalf("%s without a human login: %d %s", flag, code, errOut)
		}
	}
	for n, layer := range map[int]string{2: "L07", 3: "L02"} {
		d := evalDraft(t, "plan/config-migration/output/plan.json", func(d *planning.PlanDraft) { d.Story, d.Checkpoints[0].Layer = n, layer })
		if code, out, errOut := run(t, agent, opts, "plan", strconv.Itoa(n), "--repo", "o/r", "--draft", d, "--create"); code != 0 {
			t.Fatalf("plan #%d: %d %s %s", n, code, out, errOut)
		}
	}
	for _, flag := range []string{"--approve", "--solo-bypass"} {
		if code, _, errOut := run(t, human, opts, "plan", "3", flag, "--repo", "o/r"); code != ExitUsage || !strings.Contains(errOut, "touches no [autonomy] never layer; no review is needed") {
			t.Fatalf("%s on a plan that needs no review: %d %s", flag, code, errOut)
		}
	}

	paired := func(login string) []string {
		return []string{"--set", "seats.engineering-manager.mode=paired", "--set", "seats.engineering-manager.human=" + login}
	}
	if code, _, errOut := run(t, human, opts, append([]string{"plan", "2", "--approve", "--repo", "o/r"}, paired("alice")...)...); code != ExitBlocked ||
		!strings.Contains(errOut, "the Engineering Manager seat's human is alice, not founder") {
		t.Fatalf("approve by someone other than the EM's paired human: %d %s", code, errOut)
	}
	if code, out, errOut := run(t, human, opts, append([]string{"plan", "2", "--approve", "--repo", "o/r", "--set", "autonomy.approver=someone-else"}, paired("founder")...)...); code != 0 ||
		!strings.Contains(out, "recorded by founder (as seat)") {
		t.Fatalf("approve by the EM's paired human: %d %s %s", code, out, errOut)
	}
	var rec PlanRecord
	if err := json.Unmarshal([]byte(git(t, bare, "show", "ledger:specs/2/plan.json")), &rec); err != nil || rec.Review.ApprovedBy != "founder" || rec.Review.As != "seat" {
		t.Fatalf("plan record after the seat's review: %v %+v", err, rec.Review)
	}
	if code, out, errOut := run(t, human, opts, "plan", "2", "--approve", "--repo", "o/r", "--set", "autonomy.approver=founder"); code != 0 || !strings.Contains(out, "recorded by founder (as approver)") {
		t.Fatalf("approve by the [autonomy] approver: %d %s %s", code, out, errOut)
	}
}
