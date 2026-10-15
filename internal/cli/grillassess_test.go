package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/grill"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// bakery is a checkout of the synthetic bakery app the grill evals use
// (evals/cases/grill/testdata/bakery), with a fake GitHub holding its
// Feature #1 and two stories.
func bakery(t *testing.T) (dir string, g *fake.GitHub, opts Options) {
	t.Helper()
	dir = t.TempDir()
	if err := os.CopyFS(dir, os.DirFS(filepath.Join("..", "..", "evals", "cases", "grill", "testdata", "bakery"))); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	g = fake.New(t, "o/r", "User")
	g.AddIssue("Online ordering", fixtureBody(t, "feature", nil), "type:feature")
	g.AddIssue("Order pickup slots", "### Completion Goal\n\n1. [screenshot] state: `/order/pickup`; expected: 6 slots", "type:story")
	g.AddIssue("Show the menu with prices", "", "type:story")
	return dir, g, Options{GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir, Assets: os.DirFS(filepath.Join("..", ".."))}
}

// WP5.3b: `grill story --assess` reports the idea's vague qualifiers with
// their dimensions and the facts of the checkout (the related story among
// them), writes nothing, and in text wraps every fact in one untrusted block.
// It needs an idea and a checkout; brand has no assessment.
func TestGrillAssess(t *testing.T) {
	_, g, opts := bakery(t)
	env := map[string]string{"AAYU_GITHUB_TOKEN": "human-token"}
	idea := "The bakery app needs high-quality UX for ordering"
	code, out, errOut := run(t, env, opts, "grill", "story", "--repo", "o/r", "--assess", "--idea", idea, "--json")
	var a grill.Assessment
	if err := json.Unmarshal([]byte(out), &a); code != 0 || err != nil {
		t.Fatalf("assess: %d %v %s", code, err, errOut)
	}
	ids := map[string]bool{}
	for _, f := range a.Facts {
		ids[f.ID] = true
	}
	if len(a.Vague) != 1 || a.Vague[0].Qualifier != "high quality" || len(a.Vague[0].Dimensions) != 9 ||
		!ids["flow:/order"] || !ids["gap:error:/order"] || !ids["story:#2"] || ids["story:#3"] || !ids["floor:perf"] {
		t.Fatalf("assessment: vague %+v, facts %v", a.Vague, ids)
	}
	if w := g.Writes(); len(w) != 0 {
		t.Fatalf("--assess wrote to GitHub: %v", w)
	}
	code, out, _ = run(t, env, opts, "grill", "story", "--repo", "o/r", "--assess", "--idea", idea)
	open, closing := strings.Index(out, "<untrusted source=\"repo\">"), strings.Index(out, "</untrusted>")
	if code != 0 || open < 0 || closing < open || !strings.Contains(out[open:closing], "flow:/order  The /order flow has 5 screens") ||
		!strings.Contains(out[:open], "ux.tasks Key tasks") || !strings.Contains(out[:open], "default:") {
		t.Fatalf("text report:\n%s", out)
	}
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"grill", "story", "--assess"}, "needs --idea"},
		{[]string{"grill", "brand", "--assess", "--idea", "x"}, "grill brand has no --assess"},
	} {
		if code, _, errOut := run(t, env, opts, append(tc.args, "--repo", "o/r")...); code != ExitUsage || !strings.Contains(errOut, tc.want) {
			t.Errorf("%v: %d %s", tc.args, code, errOut)
		}
	}
	opts.Dir = t.TempDir()
	if code, _, errOut := run(t, env, opts, "grill", "story", "--repo", "o/r", "--assess", "--idea", idea); code != ExitUsage || !strings.Contains(errOut, "runs in a checkout") {
		t.Errorf("outside a checkout: %d %s", code, errOut)
	}
}

// grilledBody is the bakery story's body as the grill eval recorded it.
func grilledBody(t *testing.T, edit func(f map[string]string)) (string, string) {
	t.Helper()
	var it workitems.Item
	b, _ := os.ReadFile(filepath.Join("..", "..", "evals", "cases", "grill", "bakery-ux", "output", "issue.json"))
	if err := json.Unmarshal(b, &it); err != nil {
		t.Fatal(err)
	}
	if edit != nil {
		edit(it.Fields)
	}
	p, _ := grill.ParseProvenance(it.Fields[workitems.FieldProvenance])
	return workitems.Render(mustType(t, "story"), it.Fields), p.Request
}

// WP5.3b: a grilled draft (--idea) passes only when concrete: it records
// its Provenance, quotes the idea, cites facts the checkout has, and keeps
// vague words out of the criteria; a passing one reports the readback. The
// same vague criterion on an issue is refused by `ready --check`.
func TestGrillConcreteDraft(t *testing.T) {
	_, g, opts := bakery(t)
	env := map[string]string{"AAYU_GITHUB_TOKEN": "human-token"}
	grillJSON := func(body, idea string) (int, GrillResult) {
		_, out, errOut := run(t, env, opts, "grill", "story", "--repo", "o/r", "--json", "--idea", idea, "--parent", "1",
			"--title", "Order bread for pickup in three screens", "--body", body, "--labels", "effort:M")
		var res GrillResult
		if err := json.NewDecoder(strings.NewReader(out)).Decode(&res); err != nil {
			t.Fatalf("grill: %v\n%s%s", err, out, errOut)
		}
		code := 0
		if !res.Pass {
			code = ExitBlocked
		}
		return code, res
	}
	body, idea := grilledBody(t, nil)
	if code, res := grillJSON(body, idea); code != 0 || !strings.HasPrefix(res.Readback, "Here is what done looks like for \"Order bread for pickup in three screens\":\n1. `/order` as the seeded customer with an empty basket: choosing one loaf") ||
		!strings.Contains(res.Readback, "Parked, not in this item: loyalty stamp card") {
		t.Fatalf("the recorded grilled story: %+v", res)
	}
	for _, tc := range []struct {
		name string
		edit func(f map[string]string)
		idea string
		want string
	}{
		{"no provenance", func(f map[string]string) { delete(f, workitems.FieldProvenance) }, idea, "provenance: a grilled spec (--idea) records its Provenance"},
		{"another request", nil, "Make ordering nicer", `Request quotes "The bakery app needs high-quality UX`},
		{"unknown fact", func(f map[string]string) {
			f[workitems.FieldProvenance] = strings.Replace(f[workitems.FieldProvenance], "facts: flow:/order", "facts: flow:/checkout", 1)
		}, idea, `criterion 1 cites "flow:/checkout", which ` + "`aayu grill story --assess`" + ` does not find`},
	} {
		body, _ := grilledBody(t, tc.edit)
		code, res := grillJSON(body, tc.idea)
		if got := problemsText(res); code != ExitBlocked || !strings.Contains(got, tc.want) {
			t.Errorf("%s: %d\n%s", tc.name, code, got)
		}
	}
	// A vague word beside the numbers that make it observable (390 px, 1280 px) is resolved.
	body, _ = grilledBody(t, func(f map[string]string) {
		f[workitems.FieldCriteria] = strings.Replace(f[workitems.FieldCriteria], "the screen shows without horizontal scrolling", "the layout stays clean, without horizontal scrolling", 1)
	})
	if code, res := grillJSON(body, idea); code != 0 {
		t.Errorf("a vague word with numbers: %s", problemsText(res))
	}
	vague := g.AddIssue("Search feels fast", fixtureBody(t, "story", func(f map[string]string) {
		f[workitems.FieldCriteria] = "1. [recording] state: `/menu/search` with the seeded menu; expected: results feel fast"
	}), "type:story", "effort:S")
	vague.Parent = 1
	code, out, _ := run(t, env, opts, "ready", "4", "--check", "--repo", "o/r")
	if code != ExitBlocked || !strings.Contains(out, `completion_goal: criterion 1 says "fast" with nothing to observe`) {
		t.Fatalf("ready --check on a vague criterion: %d\n%s", code, out)
	}
}

// WP5.3b on a terminal: the idea comes first; its vague qualifier becomes
// one question per dimension with the default and the checkout's facts; the
// answers are kept as the human gave them (a default as an assumption,
// defer as an open question, park as a parked idea); the review screen is a
// plain-language readback; and the story is written with its provenance.
func TestGrillTUIVagueIdea(t *testing.T) {
	_, g, opts := bakery(t)
	env := map[string]string{"AAYU_GITHUB_TOKEN": "human-token"}
	var dimView, goalView, review string
	var dims []string
	opts.TUI = func(tm tea.Model) error {
		m := tm.(*grillModel)
		keys(m, "Make search fast", "tab")
		for strings.HasPrefix(m.steps[m.i].ID, "dim:") {
			s := m.steps[m.i]
			dims = append(dims, s.ID)
			if len(dims) == 1 {
				dimView = m.View().Content
			}
			keys(m, map[int]string{1: "typing until results show", 2: "p95 under 200 ms", 3: "default", 5: "park", 6: "defer"}[len(dims)], "tab")
		}
		for m.steps[m.i].ID != "labels" {
			if m.steps[m.i].ID == workitems.FieldCriteria {
				goalView = m.View().Content
			}
			m.Update(tea.PasteMsg{Content: map[string]string{
				"title": "Search results within 200 ms", "summary": "As a customer, I want menu search to keep up with my typing.",
				workitems.FieldCriteria: "1. [metric] state: `/menu/search` with the seeded menu; expected: p95 keystroke to results under 200 ms",
				"measure":               "p95 keystroke-to-results time", "threshold": "under 200 ms", "scenario": "Type queries on the seeded menu.",
				"environment": "local app with the seeded menu; never production", "parent": "1",
			}[m.steps[m.i].ID]})
			keys(m, "tab")
		}
		keys(m, "effort:S", "tab")
		review = m.View().Content
		keys(m, "c")
		return nil
	}
	code, out, errOut := run(t, env, opts, "grill", "story", "--repo", "o/r")
	if code != 0 {
		t.Fatalf("grill story on a terminal: %d %s %s\n%s", code, out, errOut, review)
	}
	if len(dims) != 6 || dims[0] != "dim:fast:perf.operation" ||
		!strings.Contains(dimView, `"fast": Which operation`) || !strings.Contains(dimView, "Default: the one action the request names") || !strings.Contains(dimView, "Fact route:/menu/search") {
		t.Fatalf("dimension questions %v:\n%s", dims, dimView)
	}
	if !strings.Contains(goalView, "What you said, to turn into numbered criteria:") || !strings.Contains(goalView, "p95 under 200 ms") {
		t.Errorf("Completion Goal question:\n%s", goalView)
	}
	if !strings.Contains(review, "grill story: readback") || !strings.Contains(review, "Here is what done looks like for \"Search results within 200 ms\":") ||
		!strings.Contains(review, "The draft passes validation.") {
		t.Errorf("review:\n%s", review)
	}
	body := g.Issues[4].Body
	for _, want := range []string{
		"### Assumptions\n\n- Conditions: the seeded data set",
		"### Open questions\n\n- deferred: Should a budget in CI stop it",
		"### Parked ideas\n\n- Perceived speed: While the user waits",
		"### Provenance\n\nRequest: \"Make search fast\"\nVague: \"fast\" → criteria 1\n1. stated (interview): ",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("created body lacks %q:\n%s", want, body)
		}
	}
}
