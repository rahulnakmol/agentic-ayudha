package grill

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

func catalog(t *testing.T) Catalog {
	t.Helper()
	c, err := LoadCatalog(os.DirFS("../.."))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// The catalogues parse, and Find picks qualifiers out of free text: the
// longest phrase wins over a shorter one inside it, hyphen and space
// variants match, and quoted wording (UI copy, paths) is never a qualifier.
func TestFind(t *testing.T) {
	c := catalog(t)
	for _, d := range []string{"ux", "perf", "simple", "sec", "scale", "rel"} {
		if _, ok := c.Domain(d); !ok {
			t.Fatalf("no %s catalogue", d)
		}
	}
	for text, want := range map[string]string{
		"The bakery app needs high-quality UX for ordering": "high-quality",
		"make it high quality and fast":                     "high quality,fast",
		"It should be easy to use":                          "easy to use",
		"the button says \"Quick order\" and `fast-path`":   "",
		"breakfast menus":                                   "",
		"Secure, scalable, and reliable checkout":           "Secure,scalable,reliable",
	} {
		var got []string
		for _, h := range c.Find(text) {
			got = append(got, h.Text)
		}
		if strings.Join(got, ",") != want {
			t.Errorf("Find(%q) = %v, want %s", text, got, want)
		}
	}
}

func story(fields map[string]string) workitems.Item {
	f := map[string]string{
		workitems.FieldCriteria: "1. [screenshot] state: `/order/review` with 2 items; expected: the total and the pickup time\n" +
			"2. [recording] state: `/order` empty basket; expected: a paid order in at most 3 screens",
		workitems.FieldThreshold: "all 2 criteria",
	}
	for k, v := range fields {
		f[k] = v
	}
	return workitems.Item{Type: "story", Title: "Order in three screens", Fields: f}
}

const goodProvenance = "Request: \"Ordering needs high-quality UX\"\nVague: \"high-quality UX\" → criteria 1, 2\n" +
	"1. stated (ux.trust): \"show the total and the pickup time\"\n2. confirmed (ux.steps): \"three screens at most\"; facts: flow:/order"

// Check: a criterion keeps a vague word only with something to observe; a
// grilled spec (Provenance present) resolves every qualifier of its request,
// gives every criterion one source, lists assumptions, answers or defers
// every open question, and keeps parked ideas out of the criteria.
func TestCheck(t *testing.T) {
	c := catalog(t)
	for _, tc := range []struct {
		name   string
		fields map[string]string
		want   []string // substrings of the problems; none means it passes
	}{
		{"not grilled, concrete", nil, nil},
		{"vague criterion", map[string]string{workitems.FieldCriteria: "1. [screenshot] state: `/order`; expected: the page feels intuitive"},
			[]string{`completion_goal: criterion 1 says "intuitive" with nothing to observe`, "skills/grill/references/vague/ux.md"}},
		{"vague word with a number", map[string]string{workitems.FieldCriteria: "1. [metric] state: `/menu/search` seeded; expected: fast: p95 under 200 ms"}, nil},
		{"vague word only in quoted copy", map[string]string{workitems.FieldCriteria: "1. [screenshot] state: `/order`; expected: the \"Quick reorder\" button"}, nil},
		{"vague threshold", map[string]string{workitems.FieldThreshold: "fast enough"}, []string{"threshold: Threshold says \"fast\""}},
		{"grilled and complete", map[string]string{workitems.FieldProvenance: goodProvenance, workitems.FieldAssumptions: "none",
			workitems.FieldOpenQuestions: "- answered: which task first\n- deferred: exact wording (staff write it)", workitems.FieldParked: "- loyalty stamps: a stamp card"}, nil},
		{"placement the human said", map[string]string{workitems.FieldProvenance: goodProvenance, workitems.FieldAssumptions: "none",
			workitems.FieldCriteria: "1. [screenshot] state: `/order/review` with 2 items; expected: the total and the pickup time on the review screen, at the top of `/order/review`\n" +
				"2. [recording] state: `/order` empty basket; expected: a paid order in at most 3 screens",
		}, []string{`criterion 1 decides where something goes ("at the top of")`}},
		{"grilled gaps", map[string]string{
			workitems.FieldProvenance:    "Request: \"Ordering needs high-quality UX and must be fast\"\nVague: \"fast\" → deferred: no number yet\n1. stated (ux.trust): \"show the total\"\n1. confirmed: \"again\"\n3. guessed: \"x\"",
			workitems.FieldOpenQuestions: "what wording?",
			workitems.FieldParked:        "- pickup time: let them change it later",
		}, []string{
			`provenance: criterion 3: source "guessed"`,
			"provenance: criterion 3 has a source but no such criterion exists",
			"provenance: criterion 1 has 2 sources; keep one",
			"provenance: criterion 2 has no source",
			`open_questions: "what wording?" is neither answered nor deferred`,
			`provenance: the request says "high-quality" but no ` + "`Vague:`" + ` line resolves it`,
			`provenance: "fast" is deferred, but Open questions has no "deferred:" entry for it`,
			"assumptions: a grilled spec lists the assumptions",
			`parked_ideas: parked idea "pickup time" appears in the Completion Goal`,
		}},
	} {
		var got []string
		for _, p := range Check(c, story(tc.fields)) {
			got = append(got, p.String())
		}
		all := strings.Join(got, "\n")
		if len(tc.want) == 0 && len(got) > 0 {
			t.Errorf("%s: want no problems, got:\n%s", tc.name, all)
		}
		for _, w := range tc.want {
			if !strings.Contains(all, w) {
				t.Errorf("%s: missing %q in:\n%s", tc.name, w, all)
			}
		}
	}
	if ps := Check(c, workitems.Item{Type: "feature", Fields: map[string]string{"summary": "make it fast"}}); ps != nil {
		t.Errorf("a Feature has no criteria to check: %v", ps)
	}
}

// Assess on the bakery fixture: the facts a grill conversation cites. The
// order flow and its screens, the missing error state on the pay screen (but
// not on the details screen, which handles one), components in use, tokens,
// events, the matching story only, configured floors and the missing API
// budget, and the layer footprint. With no matching route, every route is
// listed and the scope says so.
func TestAssess(t *testing.T) {
	root := "../../evals/cases/grill/testdata/bakery"
	l, err := config.Load(config.Sources{RepoDir: root})
	if err != nil {
		t.Fatal(err)
	}
	stories := []Issue{
		{Number: 12, Title: "Show the menu with prices", State: "closed"},
		{Number: 14, Title: "Order pickup slots", State: "open", Body: "### Completion Goal\n\n1. [screenshot] state: `/order/pickup`; expected: 6 slots"},
	}
	a := Assess(AssessInput{Root: root, Type: "story", Idea: "The bakery app needs high-quality UX for ordering", Catalog: catalog(t), Verify: l.Config.Verify, Stories: stories})
	facts := map[string]string{}
	for _, f := range a.Facts {
		facts[f.ID] = f.Text
	}
	for id, want := range map[string]string{
		"flow:/order":                  "5 screens: /order, /order/details, /order/options, /order/pickup, /order/review",
		"route:/order/review":          "Page /order/review",
		"route:/api/orders":            "API route",
		"gap:error:/order":             "4 of 5 /order screens (/order, /order/options, /order/pickup, /order/review)",
		"component:Stepper":            "(design system), used on /order, /order/details",
		"tokens:web/styles/tokens.css": "14 design tokens",
		"event:order_submitted":        "order_submitted",
		"test:web/e2e/order.spec.ts":   "covers /order",
		"story:#14":                    "#14 Order pickup slots (open); Completion Goal: 1. [screenshot]",
		"floor:perf":                   "LCP <= 2500 ms",
		"gap:api-budget":               "No API budget",
		"layer:L01":                    "5 of the matched files",
	} {
		if !strings.Contains(facts[id], want) {
			t.Errorf("%s = %q, want it to contain %q", id, facts[id], want)
		}
	}
	for _, id := range []string{"gap:loading:/order", "story:#12", "route:/menu"} {
		if _, ok := facts[id]; ok {
			t.Errorf("unexpected fact %s: %s", id, facts[id])
		}
	}
	if len(a.Vague) != 1 || a.Vague[0].Domain != "ux" || a.Vague[0].Dimensions[0].ID != "ux.tasks" || a.Untrusted == "" {
		t.Errorf("vague: %+v", a.Vague)
	}
	b := Assess(AssessInput{Root: root, Type: "story", Idea: "Everything should feel polished", Catalog: catalog(t), Verify: l.Config.Verify})
	if !strings.Contains(b.Scope, "no route matched") || !strings.Contains(b.Scope, "every one of the 8 page routes") {
		t.Errorf("scope without a match: %s", b.Scope)
	}
}

// A provenance tag may name the question's topic; Placements finds routes,
// named screens, and relative positions in an expected result, never a
// file path or a plural "screens".
func TestTopicsAndPlacements(t *testing.T) {
	p, ps := ParseProvenance("Request: \"x\"\n1. confirmed (ux.steps/screen-count): \"three\" / \"at most\"\n2. stated (ux.trust): \"total\"")
	if len(ps) > 0 || p.Criteria[0].Topic != "ux.steps/screen-count" || p.Criteria[0].Dimension != "ux.steps" || p.Criteria[1].Topic != "" ||
		strings.Join(p.Criteria[0].Quotes, "|") != "three|at most" {
		t.Fatalf("%+v %v", p, ps)
	}
	var got []string
	for _, pl := range Placements("the total on the review screen, below the \"Pay\" button, in `/order/[id]/review`, at most 3 screens, from `web/styles/tokens.css`") {
		got = append(got, pl.Key)
	}
	if strings.Join(got, ",") != "review,review,below" {
		t.Fatalf("placements: %v", got)
	}
	c := catalog(t)
	if d, _ := c.Dimension("ux.steps"); !slices.Contains(d.Topics, "confirmation-placement") {
		t.Fatalf("ux.steps topics: %v", d.Topics)
	}
}
