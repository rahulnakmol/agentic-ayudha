package survey

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

func fixtureBacklog(t *testing.T) Backlog {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "backlog.json"))
	if err != nil {
		t.Fatal(err)
	}
	bl, err := ParseBacklog(b)
	if err != nil {
		t.Fatal(err)
	}
	return bl
}

func item(bl *Backlog, key string) *Item {
	for k := range bl.Items {
		if bl.Items[k].Key == key {
			return &bl.Items[k]
		}
	}
	panic(key)
}

// Every item passes the work-item schema at its stage and rests on what the
// survey knows; an unknown never backs an item, and a modernisation Story
// proves parity in its own Completion Goal.
func TestCheckBacklog(t *testing.T) {
	ix := buildFixture(t, 0)
	d := fixtureDraft(t)
	r := CheckBacklog(ix, d, fixtureBacklog(t))
	if !r.Pass {
		t.Fatalf("fixture backlog: %v %+v", r.Problems, r.Items)
	}
	if got := strings.Join(r.Order, " "); got != "goal rust-engine evaluate capture-golden port-eval div-zero no-env-debug" {
		t.Errorf("order = %s", got)
	}
	cases := []struct {
		name string
		edit func(bl *Backlog)
		want string
	}{
		{"unknown basis", func(bl *Backlog) { item(bl, "goal").Basis = []string{"P1"} }, "basis P1 is unknown in the survey (Q1 asks it)"},
		{"goal on code alone", func(bl *Backlog) { item(bl, "goal").Basis = []string{"E1"} }, "a Goal rests on a product claim the person confirmed or a document states"},
		{"open question as basis", func(bl *Backlog) { item(bl, "rust-engine").Basis = []string{"Q1"} }, "basis Q1 is a question that is not answered yet"},
		{"story on product claims only", func(bl *Backlog) { item(bl, "port-eval").Basis = []string{"P2"} }, "a Story rests on at least one engineering or capability claim"},
		{"bug with no marker or risk", func(bl *Backlog) { item(bl, "div-zero").Basis = []string{"C1"} }, "a Bug rests on a known-issue marker"},
		{"invented citation", func(bl *Backlog) { item(bl, "div-zero").Basis = []string{"marker:src/engine.cpp:7"} }, "no such fact in the index"},
		{"parity missing", func(bl *Backlog) { item(bl, "port-eval").Parity = nil }, "parity: a modernisation Story names the case sets"},
		{"parity not in the criteria", func(bl *Backlog) {
			item(bl, "port-eval").Fields[workitems.FieldCriteria] = "1. [api-trace] state: the Rust build; expected: the same outputs"
		}, "the Completion Goal must name case set G1"},
		{"not Ready", func(bl *Backlog) { delete(item(bl, "port-eval").Fields, workitems.FieldCriteria) }, "Completion Goal is required at Ready"},
		{"no effort", func(bl *Backlog) { item(bl, "port-eval").Labels = nil }, "Effort is required at Ready"},
		{"wrong parent type", func(bl *Backlog) { item(bl, "port-eval").Parent = "goal" }, "parent goal is a goal; a Story's parent must be a feature"},
		{"story with no Feature", func(bl *Backlog) { item(bl, "port-eval").Parent = "" }, "file it under the Feature it delivers"},
		{"dependency cycle", func(bl *Backlog) { item(bl, "capture-golden").DependsOn = []string{"port-eval"} }, "form a cycle"},
		{"two goals", func(bl *Backlog) {
			g := *item(bl, "goal")
			g.Key = "goal-2"
			bl.Items = append(bl.Items, g)
		}, "has one Goal; it has 2"},
		// The do-not-carry list becomes Decisions that constrain the Stories porting its code.
		{"story ports a do-not-carry", func(bl *Backlog) { item(bl, "port-eval").Avoids = nil }, "avoids: step M1 does not carry over N1"},
		{"no Decision for it", func(bl *Backlog) { bl.Items = bl.Items[:len(bl.Items)-1] }, "avoids N1, but no Decision item decides it"},
		{"Decision misses a Story", func(bl *Backlog) { item(bl, "no-env-debug").Constrains = []string{"capture-golden"} }, "avoids N1: its Decision no-env-debug must constrain this item"},
		{"Decision on a claim", func(bl *Backlog) { item(bl, "no-env-debug").Basis = []string{"E4"} }, "a survey Decision decides a do-not-carry entry"},
		{"constrains from a Story", func(bl *Backlog) { item(bl, "port-eval").Constrains = []string{"div-zero"} }, "only a Decision constrains other items"},
		// The compatibility contract: undecided levels block, exact ones need their own evidence.
		{"keeps an undecided interface", func(bl *Backlog) { item(bl, "port-eval").Contract = []string{"K1"} }, "contract K1: its level is undecided (Q1 asks)"},
	}
	exact := func(d Draft) Draft {
		d.Compatibility = append([]Interface(nil), d.Compatibility...)
		d.Compatibility[0].Level, d.Compatibility[0].DecidedBy = "exact", "request"
		d.Modernisation.Suites = append([]Suite(nil), d.Modernisation.Suites...)
		d.Compatibility[0].Proof = []string{"G1", "S1"}
		return d
	}
	for _, tc := range []struct {
		name string
		edit func(bl *Backlog)
		want string
	}{
		{"exact interface nobody keeps", func(*Backlog) {}, "interface K1 must match exactly, but no Story or Bug keeps it"},
		{"exact interface on other evidence", func(bl *Backlog) { item(bl, "port-eval").Contract = []string{"K1"} }, "contract K1 must match exactly: parity must include S1"},
	} {
		bl := fixtureBacklog(t)
		tc.edit(&bl)
		r := CheckBacklog(ix, exact(d), bl)
		all := append([]string(nil), r.Problems...)
		for _, ir := range r.Items {
			all = append(all, ir.Problems...)
		}
		if r.Pass || !strings.Contains(strings.Join(all, "\n"), tc.want) {
			t.Errorf("%s: pass %v, problems %v; want %q", tc.name, r.Pass, all, tc.want)
		}
	}
	bl := fixtureBacklog(t)
	pe := item(&bl, "port-eval")
	pe.Contract, pe.Parity = []string{"K1"}, []string{"G1", "S1"}
	pe.Fields[workitems.FieldCriteria] += "\n2. [api-trace] state: the Rust build and suite S1; expected: aayu survey --compat scores every area as the C++ build does"
	if r := CheckBacklog(ix, exact(d), bl); !r.Pass {
		t.Errorf("exact interface kept with its own evidence: %v %+v", r.Problems, r.Items)
	}
	for _, tc := range cases {
		bl := fixtureBacklog(t)
		tc.edit(&bl)
		r := CheckBacklog(ix, d, bl)
		var all []string
		all = append(all, r.Problems...)
		for _, ir := range r.Items {
			all = append(all, ir.Problems...)
		}
		if r.Pass || !strings.Contains(strings.Join(all, "\n"), tc.want) {
			t.Errorf("%s: pass %v, problems %v; want %q", tc.name, r.Pass, all, tc.want)
		}
	}
	// An answered question turns the unknown into a basis the Goal may rest on.
	d.Questions[0].Status, d.Questions[0].Answer = "answered", "the billing team runs it nightly"
	for k := range d.Compatibility {
		d.Compatibility[k].Level = "differ" // the person's answer decides the level
		d.Compatibility[k].Difference = "the billing script only reads the number"
	}
	bl = fixtureBacklog(t)
	item(&bl, "goal").Basis = []string{"Q1"}
	if r := CheckBacklog(ix, d, bl); !r.Pass {
		t.Errorf("goal on an answered question: %v %+v", r.Problems, r.Items)
	}
}

// The body puts the survey key before the first section, where the parser
// does not look, and the digest moves when anything the person reviews moves.
func TestBodyAndDigest(t *testing.T) {
	bl := fixtureBacklog(t)
	it := *item(&bl, "port-eval")
	body := Body(bl.Product, it, "depends-on #4")
	if k, ok := ParseMarker("calc", body); !ok || k != "port-eval" {
		t.Fatalf("marker: %q %v", k, ok)
	}
	if _, ok := ParseMarker("other", body); ok {
		t.Error("a marker belongs to its product")
	}
	parsed, ps := workitems.Parse(5, it.Title, body, "", []string{"type:story", "effort:M"})
	if len(ps) > 0 || parsed.Fields[workitems.FieldLinks] != "depends-on #4" || parsed.Fields[workitems.FieldSummary] != it.Fields[workitems.FieldSummary] {
		t.Fatalf("parse of the filed body: %v %+v", ps, parsed.Fields)
	}
	r := CheckBacklog(buildFixture(t, 0), fixtureDraft(t), bl)
	base := r.Digest
	item(&bl, "port-eval").Title = "Port eval to Rust, again"
	if Digest(bl, r.Order) == base {
		t.Error("a changed title must change the digest")
	}
	md := RenderBacklog(bl, r, "survey.json", "backlog.json")
	if !strings.Contains(md, "--create --reviewed "+base) || !strings.Contains(md, "<!-- aayudha:survey product=calc key=port-eval -->") {
		t.Errorf("backlog.md:\n%s", md)
	}
}
