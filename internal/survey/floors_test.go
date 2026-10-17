package survey

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// floorCase is one fixture of a floor (Story #401's rule): a match the floor
// must report, a negated form it must not, and a near miss it must not.
type floorCase struct {
	kind, name string
	edit       func(d *Draft)
	want       string // the problem the floor reports; with fires false, the text that must not appear
	fires      bool
}

func runFloorCases(t *testing.T, ix *Index, base func() Draft, cases []floorCase) {
	t.Helper()
	for _, tc := range cases {
		d := base()
		tc.edit(&d)
		got := strings.Join(Check(ix, d).Problems, "\n")
		if strings.Contains(got, tc.want) != tc.fires {
			t.Errorf("%s %s: fires %v, want %v; %q in problems:\n%s", tc.kind, tc.name, !tc.fires, tc.fires, tc.want, got)
		}
	}
}

// The capability map covers every entry point of the index: by its entry,
// its line or a range that holds it, or the symbol of its definition (in a
// .cpp file for a header's declaration), and only from a capability claim.
func TestFloorCapabilityMap(t *testing.T) {
	ix := buildFixture(t, 0)
	drop := func(d *Draft, cite string) {
		var keep []string
		for _, c := range d.Capabilities[0].Cites {
			if c != cite {
				keep = append(keep, c)
			}
		}
		d.Capabilities[0].Cites = keep
	}
	runFloorCases(t, ix, func() Draft { return fixtureDraft(t) }, []floorCase{
		{"match", "header declarations left out of the map", func(d *Draft) { drop(d, "src/engine.h:6-10") },
			"no capability covers the entry point(s) src/engine.h:10 (method Engine::ok), 6 (class Engine), 8 (method Engine::Engine)", true},
		{"match", "a program's main left out", func(d *Draft) { d.Capabilities[2].Cites = []string{"lib/tool.py:6-9"} },
			"lib/tool.py:19 (__main__)", true},
		{"negated", "a declaration covered by its definition's symbol in the .cpp", func(d *Draft) { drop(d, "src/engine.h:6-10") },
			"9 (method Engine::eval)", false},
		{"negated", "a range that holds the entry", func(d *Draft) {
			drop(d, "src/engine.h:6-10")
			d.Capabilities[0].Cites = append(d.Capabilities[0].Cites, "src/engine.h:5-11")
		}, "src/engine.h", false},
		{"near miss", "the whole file cited, without lines", func(d *Draft) {
			drop(d, "src/engine.h:6-10")
			d.Capabilities[0].Cites = append(d.Capabilities[0].Cites, "src/engine.h")
		}, "src/engine.h:10 (method Engine::ok)", true},
		{"near miss", "the same lines in another file", func(d *Draft) {
			drop(d, "src/util.h:2-3")
			d.Capabilities[0].Cites = append(d.Capabilities[0].Cites, "src/engine.h:2-3")
		}, "src/util.h:2 (function parse), 3 (function report)", true},
		{"near miss", "an engineering claim cites it, no capability does", func(d *Draft) {
			drop(d, "src/util.h:2-3")
			d.Engineering[0].Cites = append(d.Engineering[0].Cites, "src/util.h:2-3")
		}, "src/util.h:2 (function parse)", true},
	})
}

// A draft holds at most one interview round of open questions.
func TestFloorQuestionCap(t *testing.T) {
	ix := buildFixture(t, 0)
	with := func(open, other int, status string) func(d *Draft) {
		return func(d *Draft) {
			for k := len(d.Questions); k < open+other; k++ {
				q := Question{ID: fmt.Sprintf("Q%d", k+1), Text: "Which flag?", Why: "It changes K1.", Status: "open"}
				if k >= open {
					q.Status, q.Reason, q.Answer = status, "the person parked it", "yes"
				}
				d.Questions = append(d.Questions, q)
			}
		}
	}
	const cap = "questions: 6 open; the interview asks at most 5 a round"
	runFloorCases(t, ix, func() Draft { return fixtureDraft(t) }, []floorCase{
		{"match", "six open questions", with(6, 0, ""), cap, true},
		{"negated", "five open, and three more already answered", with(5, 3, "answered"), "questions: ", false},
		{"near miss", "six questions, one deferred by the person", with(5, 1, "deferred"), "questions: ", false},
	})
}

// With more than one program, each case set names its runners, and every
// builtin its cases call is registered by each runner (through the call
// graph from the runner's main).
func TestFloorRunners(t *testing.T) {
	ix, err := Build(context.Background(), filepath.Join("testdata", "embed"), Options{Git: noGit})
	if err != nil {
		t.Fatal(err)
	}
	if len(ix.Builtins) != 2 {
		t.Fatalf("builtins = %v; want print and Math.sqrt only (not the commented registration, strcmp, or printf)", ix.Builtins)
	}
	const cli, tests = "entry:cli.cpp:3", "entry:run_tests.cpp:3"
	base := func() Draft {
		return Draft{Questions: []Question{{ID: "Q1", Text: "Which scripts use Math?", Why: "It decides G1.", Status: "open"}}, Modernisation: &Modernisation{CaseSets: []CaseSet{{ID: "G1", Title: "Scripts", From: []string{cli}, Compare: "exact stdout",
			Runners: []string{cli}, Inputs: []string{"tests/hello.js"}}}}}
	}
	set := func(f func(g *CaseSet)) func(d *Draft) { return func(d *Draft) { f(&d.Modernisation.CaseSets[0]) } }
	const notCLI = "which runner entry:cli.cpp:3 does not register (registered at builtin:interp.cpp:29)"
	runFloorCases(t, ix, base, []floorCase{
		{"match", "an input calls a builtin only the test runner registers", set(func(g *CaseSet) { g.Inputs = []string{"tests/*.js"} }),
			"case set G1: tests/math.js:1 calls Math.sqrt, " + notCLI, true},
		{"match", "the title names that builtin", set(func(g *CaseSet) { g.Title = "Math.sqrt on negative numbers" }),
			"case set G1: its title or comparison calls Math.sqrt, " + notCLI, true},
		{"match", "two programs and no runner", set(func(g *CaseSet) { g.Runners = nil }), "case set G1: name its runners", true},
		{"match", "a runner that is not a program", set(func(g *CaseSet) { g.Runners = []string{"entry:interp.h:11"} }), "is not a main or script entry", true},
		{"match", "an input that names no file", set(func(g *CaseSet) { g.Inputs = []string{"tests/*.lua"} }), "input \"tests/*.lua\" matches no file", true},
		{"match", "one of two runners lacks it", set(func(g *CaseSet) { g.Runners, g.Inputs = []string{tests, cli}, []string{"tests/math.js"} }), notCLI, true},
		{"negated", "the runner that registers it", set(func(g *CaseSet) { g.Runners, g.Inputs = []string{tests}, []string{"tests/math.js"} }), "case set G1", false},
		{"negated", "a builtin both runners register", set(func(g *CaseSet) { g.Inputs = []string{"tests/hello.js"} }), "case set G1", false},
		{"negated", "the file excluded, naming what decided it", set(func(g *CaseSet) {
			g.Inputs, g.Exclude = []string{"tests/*.js"}, []string{"tests/math.js needs Math (Q1)"}
		}), "case set G1", false},
		{"near miss", "a comment, a string, and another object's sqrt", set(func(g *CaseSet) { g.Inputs = []string{"tests/nearmiss.js"} }), "case set G1", false},
		{"near miss", "a title about Math without naming a builtin", set(func(g *CaseSet) { g.Title = "Math results (sqrt) and myMath.sqrt" }), "case set G1", false},
	})
}
