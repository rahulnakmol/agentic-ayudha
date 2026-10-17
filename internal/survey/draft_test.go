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
	if c := r.Coverage; c.ModulesCited != 2 || c.Modules != 3 || c.MarkersCited != 1 || c.Markers != 4 || c.OpenQuestions != 1 || c.Unknowns != 1 || c.EntriesCited < 1 {
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
	if r := Check(ix, d); !r.Pass || r.Coverage.OpenQuestions != 0 || r.Coverage.Unknowns != 0 {
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
		"## Modernisation path: Rust", "Parity: G1.", "Known-issue markers cited: 1 of 4. Not cited: `marker:lib/tool.py:1`"} {
		if !strings.Contains(md, want) {
			t.Errorf("SURVEY.md lacks %q:\n%s", want, md)
		}
	}
	d.ProductClaims[1].Basis = "observed"
	if md := Render(ix, d, Check(ix, d)); !strings.Contains(md, "## Problems") {
		t.Error("a failing draft's report lists its problems")
	}
}
