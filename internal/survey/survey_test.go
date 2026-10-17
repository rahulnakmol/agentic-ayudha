package survey

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// noGit makes Build walk the fixture as a plain directory: the fixture sits
// inside this repository, whose git history must not leak into it.
func noGit(context.Context, string, ...string) string { return "" }

func buildFixture(t *testing.T, budget int) *Index {
	t.Helper()
	ix, err := Build(context.Background(), filepath.Join("testdata", "legacy"), Options{Budget: budget, Git: noGit})
	if err != nil {
		t.Fatal(err)
	}
	return ix
}

func TestIndexFacts(t *testing.T) {
	ix := buildFixture(t, 0)
	target := func(id string) Target {
		for _, x := range ix.Targets {
			if x.ID == id {
				return x
			}
		}
		t.Fatalf("no target %s in %v", id, ix.Targets)
		return Target{}
	}
	// Build graph: a CMake variable and a Make object list both expand to sources.
	if got := target("build:CMakeLists.txt#calc"); got.Kind != "executable" || !slices.Equal(got.Sources, []string{"src/engine.cpp", "src/main.cpp", "src/util.cpp"}) || got.At != "CMakeLists.txt:5" {
		t.Errorf("cmake calc = %+v", got)
	}
	if got := target("build:CMakeLists.txt#engine"); got.Kind != "library" || len(got.Sources) != 2 {
		t.Errorf("cmake engine = %+v", got)
	}
	if got := target("build:Makefile#calc"); !slices.Equal(got.Sources, []string{"src/engine.cpp", "src/main.cpp", "src/util.cpp"}) || got.At != "Makefile:8" {
		t.Errorf("make calc = %+v", got)
	}
	if got := target("build:Makefile#clean"); got.Kind != "rule" || len(got.Sources) != 0 {
		t.Errorf("make clean = %+v", got)
	}
	// A test's Markdown input is test data, not a product document.
	if len(ix.Docs) != 1 || ix.Docs[0].ID != "doc:README.md" {
		t.Errorf("docs = %v, want only doc:README.md", ix.Docs)
	}

	sym := map[string]Symbol{}
	for _, s := range ix.Symbols {
		sym[s.ID] = s
	}
	for id, at := range map[string]string{
		"sym:src/engine.cpp#Engine::eval":   "src/engine.cpp:16",
		"sym:src/engine.cpp#Engine::Engine": "src/engine.cpp:13",
		"sym:src/engine.h#Engine":           "src/engine.h:6",
		"sym:src/engine.h#Engine::ok":       "src/engine.h:10",
		"sym:src/util.cpp#report":           "src/util.cpp:12", // the name's line, not the brace's
		"sym:lib/Render.php#Render":         "lib/Render.php:4",
		"sym:lib/Render.php#Render::block":  "lib/Render.php:11",
		"sym:lib/tool.py#Store::load":       "lib/tool.py:7",
		"sym:src/main.cpp#main":             "src/main.cpp:4",
	} {
		if s, ok := sym[id]; !ok || s.At != at {
			t.Errorf("symbol %s = %+v, want at %s", id, s, at)
		}
	}
	// The macro's braces are not a definition; a string that looks like a call is not one.
	for id := range sym {
		if strings.Contains(id, "CHECK") {
			t.Errorf("macro read as a definition: %s", id)
		}
	}
	if s := sym["sym:src/engine.cpp#Engine::eval"]; s.End != 22 {
		t.Errorf("Engine::eval ends at %d, want 22", s.End)
	}
	if !sym["sym:lib/Render.php#Render::text"].Public || sym["sym:lib/Render.php#Render::block"].Public || sym["sym:lib/Render.php#Render::report"].Public {
		t.Error("PHP visibility: text public, block and report not")
	}

	// Calls resolve by name; a name with several definitions keeps them all.
	var parse, report *Edge
	for k, e := range ix.Edges {
		if e.Kind == "call" && e.From == "sym:src/engine.cpp#Engine::eval" {
			switch {
			case slices.Contains(e.To, "sym:src/util.cpp#parse"):
				parse = &ix.Edges[k]
			case slices.Contains(e.To, "sym:src/util.cpp#report"):
				report = &ix.Edges[k]
			}
		}
		if e.Kind == "call" && slices.Contains(e.To, "sym:src/engine.cpp#Engine::eval") && strings.HasPrefix(e.At, "src/engine.cpp") {
			t.Errorf("a string literal was read as a call: %+v", e)
		}
	}
	if parse == nil || parse.Ambiguous || parse.At != "src/engine.cpp:19" {
		t.Errorf("Engine::eval → parse = %+v", parse)
	}
	if report == nil || !report.Ambiguous || !slices.Contains(report.To, "sym:lib/tool.py#report") || !slices.Contains(report.To, "sym:lib/Render.php#Render::report") {
		t.Errorf("Engine::eval → report = %+v, want ambiguous across util.cpp, Render.php, tool.py", report)
	}

	// Includes: a local header is a file edge, a system header a dependency.
	var local, system bool
	for _, e := range ix.Edges {
		if e.Kind == "include" && e.From == "file:src/engine.cpp" && slices.Equal(e.To, []string{"file:src/util.h"}) && e.At == "src/engine.cpp:10" {
			local = true
		}
		if e.Kind == "include" && e.From == "file:src/util.cpp" && slices.Equal(e.To, []string{"ext:stdio.h"}) {
			system = true
		}
	}
	if !local || !system {
		t.Errorf("include edges: local %v, system %v", local, system)
	}
	deps := map[string]bool{}
	for _, d := range ix.Deps {
		deps[d.ID] = true
	}
	for _, id := range []string{"dep:include:stdio.h", "dep:cmake:Threads", "dep:include:json"} {
		if !deps[id] {
			t.Errorf("no dependency %s in %v", id, ix.Deps)
		}
	}

	// Known-issue blocks and inline markers; a marker inside a docstring is still a comment.
	markers := map[string]string{}
	for _, m := range ix.Markers {
		markers[m.At] = m.Kind + ": " + m.Text
	}
	for at, want := range map[string]string{
		"src/engine.cpp:5":  "known-issue: Division by zero crashes",
		"src/engine.cpp:6":  "known-issue: No operator precedence",
		"src/engine.cpp:17": "todo: support parentheses",
	} {
		if markers[at] != want {
			t.Errorf("marker at %s = %q, want %q (all: %v)", at, markers[at], want, markers)
		}
	}

	facts := map[string]bool{}
	for _, l := range [][]Fact{ix.Entries, ix.Surfaces, ix.Tests, ix.Docs} {
		for _, f := range l {
			facts[f.ID] = true
		}
	}
	for _, id := range []string{"entry:src/main.cpp:4", "entry:lib/tool.py:19", "entry:src/engine.h:6", "entry:lib/Render.php:6",
		"io:env:src/util.cpp:6", "io:cli-args:src/main.cpp:4", "io:dynamic-dispatch:lib/Render.php:14", "io:file:lib/tool.py:8",
		"test:tests/test001.js", "doc:README.md"} {
		if !facts[id] {
			t.Errorf("no fact %s", id)
		}
	}
	if facts["entry:src/engine.cpp:16"] {
		t.Error("a .cpp method is not API; its header's class is")
	}
}

func TestResolve(t *testing.T) {
	ix := buildFixture(t, 0)
	for _, ok := range []string{"src/engine.cpp:16", "src/engine.cpp:13-22", "sym:src/engine.cpp#Engine::eval", "marker:src/engine.cpp:5", "build:Makefile#calc", "README.md", "mod:src"} {
		if err := ix.Resolve(ok); err != nil {
			t.Errorf("Resolve(%q) = %v", ok, err)
		}
	}
	for _, bad := range []string{"src/engine.cpp:23", "src/engine.cpp:0", "src/engine.cpp:9-3", "src/missing.cpp:1", "sym:src/engine.cpp#Engine::run", "marker:src/engine.cpp:7", "engine is fast"} {
		if err := ix.Resolve(bad); err == nil {
			t.Errorf("Resolve(%q) passed; want an error", bad)
		}
	}
}

func TestDeterministicAndBounded(t *testing.T) {
	a, b := buildFixture(t, 0), buildFixture(t, 0)
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	if !bytes.Equal(ja, jb) {
		t.Fatal("two builds of the same checkout differ")
	}
	// A small budget splits src/ along file stems: engine.h stays with engine.cpp.
	ix := buildFixture(t, 150)
	var src []Module
	for _, m := range ix.Modules {
		if m.Dir == "src" {
			src = append(src, m)
		}
	}
	if len(src) < 2 || src[0].ID != "mod:src#1" {
		t.Fatalf("src modules = %+v, want a split into mod:src#1…", src)
	}
	for _, m := range src {
		if slices.Contains(m.Files, "src/engine.cpp") != slices.Contains(m.Files, "src/engine.h") {
			t.Errorf("engine.cpp and engine.h split apart: %+v", m)
		}
	}
	p, err := ix.ModulePack(src[0].ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Symbols) > 1 || len(p.Omitted) == 0 {
		t.Errorf("pack with limit 1: %d symbols, omitted %v", len(p.Symbols), p.Omitted)
	}
	if _, err := ix.ModulePack("mod:nope", 0); err == nil || !strings.Contains(err.Error(), "mod:src#1") {
		t.Errorf("unknown module error = %v, want the module list", err)
	}
}

// Fortran (fixed and free form), COBOL, Pascal, and VB6 are read for symbols,
// calls, includes, and traps; Lua is counted only, and says so.
func TestKeywordLanguages(t *testing.T) {
	ix, err := Build(context.Background(), filepath.Join("testdata", "keyword"), Options{Git: noGit})
	if err != nil {
		t.Fatal(err)
	}
	sym := map[string]Symbol{}
	for _, s := range ix.Symbols {
		sym[s.ID] = s
	}
	for id, want := range map[string]string{
		"sym:src/solver.f#solve":                 "program src/solver.f:2-10",
		"sym:src/solver.f#step":                  "subroutine src/solver.f:20-26", // a continued argument list
		"sym:src/solver.f#fnorm":                 "function src/solver.f:27-32",   // a typed function
		"sym:src/drv.f#drv":                      "program src/drv.f:2-9",         // a main program with no PROGRAM statement
		"sym:lib/grid.f90#grid":                  "module lib/grid.f90:2-22",
		"sym:lib/grid.f90#grid::area":            "function lib/grid.f90:14-18", // pure, with result()
		"sym:lib/grid.f90#grid::clear":           "subroutine lib/grid.f90:19-21",
		"sym:lib/grid.f90#grid::point":           "type lib/grid.f90:5-7",
		"sym:src/billing.cbl#billing::calc-para": "paragraph src/billing.cbl:13-14",
		"sym:src/units.pas#convert":              "function src/units.pas:13-17",
		"sym:vb/Ledger.bas#ledger::fee":          "method vb/Ledger.bas:10-12",
	} {
		s, ok := sym[id]
		if got := s.Kind + " " + s.At + "-" + itoa(s.End); !ok || got != want {
			t.Errorf("%s = %q, want %q", id, got, want)
		}
	}
	if _, ok := sym["sym:src/params.inc#params"]; ok {
		t.Error("an include file's declarations are a main program")
	}
	for _, id := range []string{"entry:src/drv.f:2", "io:unit-io:src/drv.f:5"} { // READ (NREAD, …): a unit chosen at run time
		if err := ix.Resolve(id); err != nil {
			t.Error(err)
		}
	}
	if _, ok := sym["sym:src/units.pas#fake"]; ok {
		t.Error("a procedure inside a { } comment is a symbol")
	}
	calls := map[string]bool{}
	for _, e := range ix.Edges {
		calls[e.Kind+" "+e.From+" "+strings.Join(e.To, ",")] = true
	}
	for _, want := range []string{
		"call sym:src/solver.f#step sym:src/solver.f#init",                                   // CALL in a fixed-form unit (sequence columns ignored)
		"call sym:src/solver.f#solve sym:src/solver.f#fnorm",                                 // a function reference, not a CALL
		"call sym:src/drv.f#drv sym:src/solver.f#init",                                       // ... and from the unnamed main program
		"call sym:lib/grid.f90#main sym:lib/grid.f90#grid::resize",                           // a program calls a module procedure
		"call sym:lib/grid.f90#grid::resize sym:lib/grid.f90#grid::clear",                    // ... and the procedure, not its module, calls on
		"call sym:src/billing.cbl#billing::main-para sym:src/billing.cbl#billing::calc-exit", // PERFORM … THRU
		"call sym:src/units.pas#convert sym:src/units.pas#scale",                             // a Pascal call
		"call sym:vb/Ledger.bas#ledger::save sym:vb/Ledger.bas#ledger::total",                // a VB call without parentheses' Call keyword
		"include file:src/solver.f file:src/params.inc",                                      // INCLUDE (an .inc file in a Fortran checkout)
		"import file:src/units.pas file:lib/helpers.pas",                                     // uses
		"import file:lib/grid.f90 ext:iso_fortran_env",                                       // USE of an intrinsic module
	} {
		if !calls[want] {
			t.Errorf("no edge %q", want)
		}
	}
	for e := range calls {
		if strings.HasPrefix(e, "call sym:lib/grid.f90#grid ") || strings.Contains(e, "fake") || strings.Contains(e, "billing::calc-para sym:src/billing.cbl#billing::calc-exit") {
			t.Errorf("unexpected edge %q", e)
		}
	}
	traps := map[string]bool{}
	for _, f := range ix.Traps {
		traps[f.ID] = true
	}
	for _, want := range []string{
		"trap:fortran-common:src/solver.f:3", "trap:fortran-equivalence:src/solver.f:14", "trap:fortran-assumed-size:src/solver.f:24",
		"trap:fortran-implicit-typing:src/solver.f:2", "trap:fortran-implicit-typing:src/solver.f:12", "trap:build-semantic-flags:Makefile:3",
		"trap:build-semantic-flags:cmake/Flags.cmake:2", // an included build file; a flag after "("
		"trap:cobol-redefines:src/billing.cbl:7", "trap:cobol-binary-decimal:src/billing.cbl:6", "trap:pascal-directives:src/units.pas:2",
		"trap:vb-on-error:vb/Ledger.bas:6", "trap:vb-no-option-explicit:vb/Ledger.bas:1", "trap:vb-16bit-integer:vb/Ledger.bas:5",
	} {
		if !traps[want] {
			t.Errorf("no trap %s", want)
		}
		if err := ix.Resolve(want); err != nil {
			t.Errorf("trap does not resolve as a citation: %v", err)
		}
	}
	// A commented-out flag is not in the build.
	for _, f := range ix.Traps {
		if want := map[string]string{"Makefile:3": "-fno-automatic", "cmake/Flags.cmake:2": "-march=native -ffast-math"}[f.At]; f.Kind == "build-semantic-flags" && f.Detail != want {
			t.Errorf("build flags at %s %q, want %q (commented-out flags are not in the build)", f.At, f.Detail, want)
		}
	}
	// STEP and FNORM declare IMPLICIT NONE; module procedures inherit the module's.
	for _, no := range []string{"trap:fortran-implicit-typing:src/solver.f:20", "trap:fortran-implicit-typing:src/solver.f:27", "trap:fortran-implicit-typing:lib/grid.f90:9", "trap:fortran-assumed-size:lib/grid.f90:28"} {
		if traps[no] {
			t.Errorf("unexpected trap %s", no)
		}
	}
	var counted []string
	for _, l := range ix.Languages {
		if l.CountedOnly {
			counted = append(counted, l.Name)
		}
	}
	if !slices.Equal(counted, []string{"lua"}) || !strings.Contains(Render(ix, Draft{}, Report{}), "Counted by file and line only, never read (no symbols, calls, imports, or traps, so no claim about them comes from the index): lua (1 files, 1 lines)") {
		t.Errorf("counted-only languages %v, or SURVEY.md does not say so", counted)
	}
}

// Every trap rule the index can report has its questions in the skill's
// reference, under a heading with the rule's name.
func TestTrapRulesHaveQuestions(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "skills", "survey", "references", "traps.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range TrapRules() {
		if !strings.Contains(string(b), "\n### "+r+"\n") {
			t.Errorf("skills/survey/references/traps.md has no heading for trap rule %s", r)
		}
	}
}
