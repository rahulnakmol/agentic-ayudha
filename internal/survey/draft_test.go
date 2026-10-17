package survey

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixtureDraft(t *testing.T) Draft {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "draft.json"))
	if err != nil {
		t.Fatal(err)
	}
	d, err := ParseDraft(b)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// A draft passes only when every claim's basis is supported: cited facts that
// exist, reasoning for an inference, an answered question for a confirmation,
// a question for an unknown, and never "observed" for a product claim.
func TestCheckDraft(t *testing.T) {
	ix := buildFixture(t, 0)
	r := Check(ix, fixtureDraft(t))
	if !r.Pass {
		t.Fatalf("fixture draft: %v", r.Problems)
	}
	if c := r.Coverage; c.ModulesCited != 2 || c.Modules != 3 || c.MarkersCited != 1 || c.Markers != 4 || c.OpenQuestions != 2 || c.Unknowns != 1 || c.EntriesCited < 1 || c.Traps != 1 || c.TrapsChecked != 1 {
		t.Errorf("coverage = %+v", c)
	}
	cases := []struct {
		name string
		edit func(d *Draft)
		want string
	}{
		{"product observed", func(d *Draft) { d.ProductClaims[1].Basis = "observed" }, "claim P2: a product claim cannot be observed"},
		{"line past the end", func(d *Draft) { d.Engineering[0].Cites[1] = "src/main.cpp:4-12" }, "src/main.cpp has 11 lines"},
		{"invented symbol", func(d *Draft) { d.Capabilities[0].Cites = []string{"sym:src/engine.cpp#Engine::simplify"} }, "no such fact in the index"},
		{"no cite", func(d *Draft) { d.Engineering[1].Cites = nil }, "claim E2: observed needs at least one citation"},
		{"inference without reasoning", func(d *Draft) { d.Engineering[3].Reasoning = "" }, "claim E4: inferred needs its reasoning"},
		{"confirmed by an open question", func(d *Draft) { d.ProductClaims[0].Basis, d.ProductClaims[0].Answer = "confirmed", "Q1" }, "claim P1: confirmed needs answer = the ID of an answered question"},
		{"unknown with no question", func(d *Draft) { d.ProductClaims[0].Question = "Q9" }, "claim P1: unknown needs question"},
		{"answered with no quote", func(d *Draft) { d.Questions[0].Status = "answered" }, "question Q1: answered, but no answer"},
		{"target the person never named", func(d *Draft) { d.Modernisation.Target = "Go" }, "the request does not name \"Go\""},
		{"step with no parity", func(d *Draft) { d.Modernisation.Steps[0].Parity = nil }, "step M1: name the case sets"},
		{"step cycle", func(d *Draft) { d.Modernisation.Steps[0].DependsOn = []string{"M2"} }, "in a cycle"},
		{"unknown module", func(d *Draft) { d.Modernisation.Steps[0].Modules = []string{"mod:engine"} }, "\"mod:engine\" is not a module"},
		{"exclusion with no reason", func(d *Draft) { d.Modernisation.CaseSets[0].Exclude = []string{"the crash"} }, "must name the question or claim"},
		{"wrong level ID", func(d *Draft) { d.Capabilities[0].ID = "E9" }, "IDs at this level are C1"},
		// Traps, the do-not-carry list, and the compatibility contract.
		{"trap rule invented", func(d *Draft) { d.Traps[0].Rule = "c-locales" }, "rule \"c-locales\" is not a trap rule"},
		{"trap without evidence", func(d *Draft) { d.Traps[0].Cites = nil }, "claim T1: observed needs at least one citation"},
		{"do-not-carry decided without the person", func(d *Draft) { d.DoNotCarry[0].Status = "confirmed" }, "do-not-carry N1: Q2 is not answered"},
		{"step ports a do-not-carry", func(d *Draft) { d.Modernisation.Steps[0].Avoids = nil }, "step M1 replaces mod:src, where do-not-carry N1 is"},
		{"level chosen for the person", func(d *Draft) { d.Compatibility[0].Level = "exact" }, "interface K1: Q1 is not answered"},
		{"exact on normalised evidence", func(d *Draft) {
			d.Compatibility[0].Level, d.Compatibility[0].DecidedBy = "exact", "request"
			d.Modernisation.CaseSets[0].Compare = "stdout with trailing spaces trimmed"
		}, "K1 must match exactly, but case set G1 compares"},
		{"screen proved by stdout", func(d *Draft) {
			d.Compatibility[1].Kind, d.Compatibility[1].Level, d.Compatibility[1].DecidedBy = "ui-form", "exact", "request"
		}, "K2 is user-visible (ui-form): prove it with a user journey"},
		{"dropped with no path", func(d *Draft) { d.Compatibility[1].Level, d.Compatibility[1].DecidedBy = "drop", "request" }, "deprecation does not give the path"},
		{"request decides without its words", func(d *Draft) {
			d.Compatibility[1].Level, d.Compatibility[1].DecidedBy, d.Compatibility[1].Deprecation = "drop", "request", "warn in 1.0, remove in 2.0"
		}, "K2: decided by the request: quote the request's words"},
		{"request quoted with words it lacks", func(d *Draft) {
			d.Compatibility[1].Level, d.Compatibility[1].DecidedBy, d.Compatibility[1].Deprecation = "drop", "request", "warn in 1.0, remove in 2.0"
			d.Compatibility[1].Quote = "with the same behaviour"
		}, "K2: decided by the request: quote the request's words"},
		{"exact without a pinned reference build", func(d *Draft) {
			d.Compatibility[0].Level, d.Compatibility[0].DecidedBy, d.Compatibility[0].Quote = "exact", "request", "rebuild it  in\nRust"
			d.Modernisation.CaseSets[0].Reference = " "
		}, "K1 must match exactly, but case set G1 names no pinned reference build"},
		{"no contract", func(d *Draft) { d.Compatibility = nil }, "list the compatibility contract"},
		{"migration with no way back", func(d *Draft) {
			d.Modernisation.Migrations = []Migration{{ID: "MG1", What: "calc history file", Cites: []string{"src/util.cpp:5"}, Proof: []string{"G1"}}}
		}, "migration MG1: give the tested way back"},
		{"shim forever", func(d *Draft) {
			d.Modernisation.Shims = []Shim{{ID: "SH1", Interface: "K2", Behaviour: "usage on stderr", Mechanism: "CALC_COMPAT=1"}}
		}, "shim SH1: needs the behaviour kept, how it is turned on, and the plan to remove it"},
		{"suite unfixable with no reason", func(d *Draft) { d.Modernisation.Suites[0].Unfixable = []string{"test001"} }, "unfixable \"test001\" needs its reason"},
	}
	for _, tc := range cases {
		d := fixtureDraft(t)
		tc.edit(&d)
		r := Check(ix, d)
		if r.Pass || !strings.Contains(strings.Join(r.Problems, "\n"), tc.want) {
			t.Errorf("%s: pass %v, problems %v; want %q", tc.name, r.Pass, r.Problems, tc.want)
		}
	}
	// The answer turns the unknown into a confirmed claim.
	d := fixtureDraft(t)
	d.Questions[0].Status, d.Questions[0].Answer = "answered", "our nightly billing script pipes it"
	d.ProductClaims[0].Basis, d.ProductClaims[0].Answer, d.ProductClaims[0].Question = "confirmed", "Q1", ""
	for k := range d.Compatibility {
		d.Compatibility[k].Level = "exact" // the person's answer decides the level
	}
	if r := Check(ix, d); !r.Pass || r.Coverage.OpenQuestions != 1 || r.Coverage.Unknowns != 0 {
		t.Errorf("answered draft: %v %+v", r.Problems, r.Coverage)
	}
	if _, err := ParseDraft([]byte(`{"product": "x", "claims": []}`)); err == nil {
		t.Error("an unknown key must fail the parse, never drop claims silently")
	}
}

func TestRenderSurvey(t *testing.T) {
	ix := buildFixture(t, 0)
	d := fixtureDraft(t)
	md := Render(ix, d, Check(ix, d))
	for _, want := range []string{"# Survey: calc", "## Limits of this survey", "| C1 | Evaluate an expression |", "**P1** (unknown, Q1)",
		"## Modernisation path: Rust", "Parity: G1.", "Known-issue markers cited: 1 of 4. Not cited: `marker:lib/tool.py:1`",
		"## Traps checked", "Rule `c-locale`", "| N1 | Debug tracing switched on", "proposed (Q2) |", "| K2 | exit-code |", "Does not carry over N1.",
		"### Coexistence before cut-over: shadow", "Trap facts checked: 1 of 1."} {
		if !strings.Contains(md, want) {
			t.Errorf("SURVEY.md lacks %q:\n%s", want, md)
		}
	}
	// Model-written text shows as written: no HTML tag or emphasis is read into
	// it, a code span stays as it is, and each heading is used once.
	d.Engineering[0].Text = "Blocks call block<Type>Continue by name; xss* and strict* files switch modes; see `a_b*c`."
	md = Render(ix, d, Check(ix, d))
	if !strings.Contains(md, `block\<Type>Continue by name; xss\* and strict\* files`) || !strings.Contains(md, "`a_b*c`") || strings.Count(md, "### Risks\n") != 1 || !strings.Contains(md, "### Modernisation risks\n") {
		t.Errorf("escaping:\n%s", md)
	}
	d.ProductClaims[1].Basis = "observed"
	if md := Render(ix, d, Check(ix, d)); !strings.Contains(md, "## Problems") {
		t.Error("a failing draft's report lists its problems")
	}
}
