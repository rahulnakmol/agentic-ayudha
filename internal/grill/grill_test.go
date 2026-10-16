package grill

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/verify"
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

const goodCheckpoints = "1. The review screen shows the total and the pickup time (criterion 1)\n2. A paid order takes at most 3 screens (criteria 2)"

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
		// "Reduced motion" names a state; "nice transitions" in the same catalogue is still vague.
		{"reduced motion is a state", map[string]string{workitems.FieldCriteria: "1. [recording] state: the order steps with reduced motion on; expected: nothing slides and the step changes at once"}, nil},
		{"vague transitions", map[string]string{workitems.FieldCriteria: "1. [recording] state: the order steps; expected: nice transitions between steps"}, []string{`criterion 1 says "nice"`}},
		{"vague word only in quoted copy", map[string]string{workitems.FieldCriteria: "1. [screenshot] state: `/order`; expected: the \"Quick reorder\" button"}, nil},
		{"vague threshold", map[string]string{workitems.FieldThreshold: "fast enough"}, []string{"threshold: Threshold says \"fast\""}},
		{"grilled and complete", map[string]string{workitems.FieldProvenance: goodProvenance, workitems.FieldAssumptions: "none", workitems.FieldCheckpoints: goodCheckpoints,
			workitems.FieldOpenQuestions: "- answered: which task first\n- deferred: exact wording (staff write it)", workitems.FieldParked: "- loyalty stamps: a stamp card"}, nil},
		// WP5.8: every term of the request has its confirmed meaning, and the checkpoints reach every criterion.
		{"terms and checkpoints", map[string]string{workitems.FieldAssumptions: "none",
			workitems.FieldProvenance: "Request: \"Staff need SSO and a KPI board\"\nTerm: \"SSO\" → single sign-on with our Okta; \"yes, Okta\"\nTerm: \"KPIs\" → deferred: the human will list them\n" +
				"Term: \"RBAC\" → role-based access\n1. stated (request): \"Staff need SSO\"\n2. stated (request): \"a KPI board\"",
			workitems.FieldCheckpoints: "1. Staff sign in with Okta (criterion 1)\n3. The board shows (criteria 2, 5)\nThe board is fast",
		}, []string{
			`provenance: line 4: term "RBAC": write`,
			`provenance: term "KPIs" is deferred, but Open questions has no "deferred:" entry for it`,
			`checkpoints: "The board is fast": write`,
			"checkpoints: checkpoint 2 is numbered 3",
			"checkpoints: checkpoint 3 reaches criterion 5, which does not exist",
		}},
		{"term without its line", map[string]string{workitems.FieldAssumptions: "none", workitems.FieldCheckpoints: goodCheckpoints,
			workitems.FieldProvenance: "Request: \"The PDP needs a CTA\"\nTerm: \"CTA\" → one button; \"one button\"\n1. stated (request): \"The PDP\"\n2. stated (request): \"a CTA\"",
		}, []string{"the request says \"PDP\" but no `Term:` line records the meaning"}},
		{"criterion no checkpoint reaches", map[string]string{workitems.FieldProvenance: goodProvenance, workitems.FieldAssumptions: "none",
			workitems.FieldCheckpoints: "1. The review screen shows the total (criterion 1)"}, []string{"checkpoints: no checkpoint reaches criterion 2"}},
		{"placement the human said", map[string]string{workitems.FieldProvenance: goodProvenance, workitems.FieldAssumptions: "none",
			workitems.FieldCriteria: "1. [screenshot] state: `/order/review` with 2 items; expected: the total and the pickup time on the review screen, at the top of `/order/review`\n" +
				"2. [recording] state: `/order` empty basket; expected: a paid order in at most 3 screens",
		}, []string{`criterion 1 decides where something goes ("at the top of")`}},
		{"placement quoted from another reply", map[string]string{workitems.FieldAssumptions: "none",
			workitems.FieldProvenance: "Request: \"Ordering needs high-quality UX\"\nVague: \"high-quality UX\" → criteria 1, 2\n" +
				"1. confirmed (ux.states/payment-failure): \"keep the basket and retry on the same screen\" / \"people pay on the review screen\"\n" +
				"2. confirmed (ux.states/payment-failure): \"keep the basket and retry on the same screen\"",
			workitems.FieldCriteria: "1. [recording] state: `/order/review` with a declined card; expected: a \"Try again\" button on the same screen, with the total still on the review screen\n" +
				"2. [recording] state: `/order/review` with a declined card; expected: the basket stays on that screen",
		}, []string{`criterion 2 decides where something goes ("on that screen")`}},
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

// The vagueness gate (WP5.8): a vague qualifier, a term that is not plain,
// or (with no qualifier) nothing observable sends a request to be unpacked;
// a request with something to observe and nothing to decode passes. A
// number inside a term (B2B) or an issue reference is not something to
// observe, and an identifier is not a term.
func TestGate(t *testing.T) {
	c := catalog(t)
	for _, tc := range []struct {
		idea, verdict string
		reasons       []string // kind:text
	}{
		{"make onboarding slick with nice transitions", GateUnpack, []string{"vague:slick", "vague:nice", "vague:transitions"}},
		{"Staff need SSO for the POS", GateUnpack, []string{"term:SSO", "term:POS", "outcome:no observable result"}},
		{"Let customers reorder their usual", GateUnpack, []string{"outcome:no observable result"}},
		{"Under Feature #40, add a reorder button", GateUnpack, []string{"outcome:no observable result"}},
		{"Add a B2B order form", GateUnpack, []string{"term:B2B", "outcome:no observable result"}},
		{"Set AAYU_AGENT in the CI job and update the README", GateUnpack, []string{"outcome:no observable result"}},
		{"maybe something like a loyalty card", GateUnpack, []string{"vague:maybe", "vague:something like"}},
		{"Show the basket total on `/order/review` like \"£12.50\"", GatePass, nil},
		{"Add a --json flag to tally list", GatePass, nil},
		{"Fix the crash when the basket has 0 items", GatePass, nil},
	} {
		a := Assessment{Idea: tc.idea, Terms: c.Abbreviations(tc.idea)}
		for _, h := range c.Find(tc.idea) {
			a.Vague = append(a.Vague, VagueHit{Qualifier: h.Qualifier, Text: h.Text, Domain: h.Domain})
		}
		g := GateFor(c, a)
		var got []string
		for _, r := range g.Reasons {
			got = append(got, r.Kind+":"+r.Text)
		}
		if g.Verdict != tc.verdict || strings.Join(got, ",") != strings.Join(tc.reasons, ",") {
			t.Errorf("%q: %s %v, want %s %v", tc.idea, g.Verdict, got, tc.verdict, tc.reasons)
		}
	}
	if ts := c.Abbreviations("The PDP and KPIs, in i18n and a11y"); len(ts) != 4 || len(ts[0].Meanings) != 2 || ts[1].Text != "KPIs" || ts[3].Text != "a11y" {
		t.Errorf("abbreviations: %+v", ts)
	}
}

// Product types decide which dimensions a qualifier asks about and which
// facts are reported: "fast" asks about startup for the tally CLI and about
// interaction latency for the bakery web app; the CLI's commands and flags
// and the web app's motion are facts, and a CLI gets no design-token gap.
func TestAssessProducts(t *testing.T) {
	c := catalog(t)
	assess := func(fixture, idea string) (Assessment, map[string]string) {
		root := "../../evals/cases/grill/testdata/" + fixture
		l, err := config.Load(config.Sources{RepoDir: root})
		if err != nil {
			t.Fatal(err)
		}
		a := Assess(AssessInput{Root: root, Type: "story", Idea: idea, Catalog: c, Verify: l.Config.Verify})
		facts := map[string]string{}
		for _, f := range a.Facts {
			facts[f.ID] = f.Text
		}
		return a, facts
	}
	dims := func(a Assessment) []string {
		var out []string
		for _, d := range a.Vague[0].Dimensions {
			out = append(out, d.ID)
		}
		return out
	}
	cli, cf := assess("tally", "make tally fast")
	if strings.Join(cli.Products, ",") != "cli" || !slices.Contains(dims(cli), "perf.startup") || slices.Contains(dims(cli), "perf.interaction") {
		t.Errorf("tally: products %v, dimensions %v", cli.Products, dims(cli))
	}
	for id, want := range map[string]string{
		"command:tally":     "6 commands and 27 flags in all (init, add, list, report, export, config); 3 flags apply to the tool itself",
		"command:tally add": "7 flags: --amount, --category, --currency, --date, --note, --tag, --split",
		"product:cli":       "package main in cmd/tally",
	} {
		if !strings.Contains(cf[id], want) {
			t.Errorf("%s = %q, want %q", id, cf[id], want)
		}
	}
	for _, id := range []string{"gap:tokens", "gap:motion", "floor:a11y", "gap:perf-budget", "gap:api-budget"} {
		if _, ok := cf[id]; ok {
			t.Errorf("tally is a CLI, but the assessment has %s", id)
		}
	}
	web, wf := assess("bakery", "make search fast")
	if strings.Join(web.Products, ",") != "web,api" || slices.Contains(dims(web), "perf.startup") || !slices.Contains(dims(web), "perf.interaction") {
		t.Errorf("bakery: products %v, dimensions %v", web.Products, dims(web))
	}
	if !strings.Contains(wf["motion:web/styles/buttons.css"], "durations 150ms") || wf["gap:reduced-motion"] == "" {
		t.Errorf("bakery motion facts: %q, %q", wf["motion:web/styles/buttons.css"], wf["gap:reduced-motion"])
	}
	// WP5.8: an API's endpoints come from its OpenAPI document, and a library's
	// surface from its entry file, so their questions are grounded too.
	api, af := assess("quotes", "make the quotes API fast")
	if strings.Join(api.Products, ",") != "api" || !slices.Contains(dims(api), "perf.endpoint") || slices.Contains(dims(api), "perf.interaction") ||
		af["route:/quotes"] != "API route /quotes (POST)." || af["route:/quotes/{id}"] != "API route /quotes/{id} (GET)." || af["route:/health"] != "" || af["gap:api-budget"] == "" {
		t.Errorf("quotes: products %v, dimensions %v, facts %v", api.Products, dims(api), af)
	}
	lib, lf := assess("slugkit", "make slugkit simple to use")
	if strings.Join(lib.Products, ",") != "library" || !slices.Contains(dims(lib), "simple.lib-surface") || slices.Contains(dims(lib), "simple.cli-surface") ||
		lf["export:slugkit"] != "Library slugkit exports 1 name(s) from src/index.js: createSlugger(options)." || lf["test:test/index.test.js"] != "Test test/index.test.js calls createSlugger." {
		t.Errorf("slugkit: products %v, dimensions %v, facts %v", lib.Products, dims(lib), lf)
	}
	if _, ok := wf["export:bakery-web"]; ok {
		t.Error("a web app has no library surface fact")
	}
}

// WP4.5b.5: a mapped feature the idea's words touch is a feature fact naming
// the checks that prove it; a feature the idea does not touch is not.
func TestAssessFeatureFacts(t *testing.T) {
	maps := []*verify.FeatureMap{{Product: "notes", Features: []verify.Feature{
		{ID: "mark-done", File: "x/features/mark-done.md", Surfaces: map[string][]string{"command": {"done"}}, Checks: []verify.Check{{ID: "missing", Kind: "cli"}}},
		{ID: "store", File: "x/features/store.md", Surfaces: map[string][]string{"func": {"Load", "Save"}}, Checks: []verify.Check{{ID: "round-trip", Kind: "lib"}}},
	}}}
	a := Assess(AssessInput{Root: t.TempDir(), Type: "story", Idea: "Undo a note marked done by mistake", Catalog: catalog(t), Maps: maps})
	var got []Fact
	for _, f := range a.Facts {
		if f.Kind == "feature" {
			got = append(got, f)
		}
	}
	if len(got) != 1 || got[0].ID != "feature:mark-done" || got[0].Source != "x/features/mark-done.md" || !strings.Contains(got[0].Text, "mark-done.missing (cli)") {
		t.Errorf("feature facts: %+v", got)
	}
}
