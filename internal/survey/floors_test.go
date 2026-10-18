package survey

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
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

// A header's declarations inside a namespace are named as their definitions
// are (a::parse, a::Foo::bar), so the definition's symbol covers them.
func TestFloorCapabilityMapNamespaces(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"api.h":   "namespace a {\nvoid parse();\nclass Foo {\npublic:\n  void bar();\n};\n}\nnamespace b {\nvoid parse();\nvoid load(int n);\nvoid load(double x);\n}\ninline namespace v1 {\nvoid tune();\n}\n",
		"api.cpp": "#include \"api.h\"\nnamespace a {\nvoid parse() {}\n}\nvoid a::Foo::bar() {}\nvoid b::parse() {}\nvoid b::load(int n) {}\nvoid b::load(double x) {}\nvoid v1::tune() {}\n",
		"more.h":  "namespace a {\nvoid parse();\n}\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ix, err := Build(context.Background(), dir, Options{Git: noGit})
	if err != nil {
		t.Fatal(err)
	}
	base := func() Draft {
		return Draft{Capabilities: []Claim{{ID: "C1", Name: "Parse", Text: "Parses.", Basis: "observed",
			Cites: []string{"sym:api.cpp#a::parse", "sym:api.cpp#a::Foo::bar", "sym:api.cpp#b::parse", "api.h:3", "sym:api.cpp#b::load", "sym:api.cpp#b::load~2", "sym:api.cpp#v1::tune"}}}}
	}
	cites := func(c ...string) func(d *Draft) { return func(d *Draft) { d.Capabilities[0].Cites = c } }
	runFloorCases(t, ix, base, []floorCase{
		{"negated", "each declaration covered by its definition's symbol, in an inline namespace too", func(*Draft) {}, "no capability covers", false},
		{"match", "the inline namespace's declaration left out", cites("sym:api.cpp#a::parse", "sym:api.cpp#a::Foo::bar", "sym:api.cpp#b::parse", "api.h:3", "sym:api.cpp#b::load", "sym:api.cpp#b::load~2"), "(function v1::tune)", true},
		{"match", "the method's definition left out", cites("sym:api.cpp#a::parse", "sym:api.cpp#b::parse", "api.h:3"), "5 (method a::Foo::bar)", true},
		{"near miss", "another namespace's parse cited for a::parse", cites("sym:api.cpp#b::parse", "sym:api.cpp#a::Foo::bar", "api.h:3", "sym:api.cpp#b::load", "sym:api.cpp#b::load~2"), "2 (function a::parse)", true},
		{"match", "one of two overloads cited", cites("sym:api.cpp#a::parse", "sym:api.cpp#a::Foo::bar", "sym:api.cpp#b::parse", "api.h:3", "sym:api.cpp#b::load"), "(function b::load)", true},
		{"negated", "a function declared in two headers, its one definition cited", func(*Draft) {}, "more.h", false},
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
	if len(ix.Builtins) != 5 {
		t.Fatalf("builtins = %v; want print, Math.sqrt, the file-scope Extra.f and Boot.go, and the constructor's JSON.parse only (not the commented registration, strcmp, or printf)", ix.Builtins)
	}
	// Two declarations on one header line are two entry points.
	if !slices.ContainsFunc(ix.Entries, func(e Fact) bool { return e.ID == "entry:interp.h:20#beta" && e.At == "interp.h:20" }) {
		t.Errorf("entries = %v; want beta beside alpha on interp.h:20", ix.Entries)
	}
	// A .hxx header is a header: its class is an entry point, as a .h header's is.
	if !slices.ContainsFunc(ix.Entries, func(e Fact) bool { return e.At == "util.hxx:3" && e.Detail == "class Box" }) {
		t.Errorf("entries = %v; want class Box in util.hxx", ix.Entries)
	}
	// An export annotation before a declaration does not hide it.
	for at, name := range map[string]string{"util.hxx:7": "function exported_api", "util.hxx:8": "function exported_attr", "util.hxx:9": "function exported_old", "util.hxx:12": "method Exported::run", "util.hxx:14": "function twice"} {
		if !slices.ContainsFunc(ix.Entries, func(e Fact) bool { return e.At == at && e.Detail == name }) {
			t.Errorf("entries = %v; want %s at %s", ix.Entries, name, at)
		}
	}
	// Three overloads on one line are three entry points.
	if n := len(slices.DeleteFunc(slices.Clone(ix.Entries), func(e Fact) bool { return e.At != "util.hxx:15" })); n != 3 {
		t.Errorf("entries = %v; want the three overloads of over on util.hxx:15", ix.Entries)
	}
	if slices.ContainsFunc(ix.Entries, func(e Fact) bool {
		return strings.Contains(e.Detail, "visibility") || strings.Contains(e.Detail, "deprecated")
	}) {
		t.Errorf("entries = %v; an attribute's arguments are not declarations", ix.Entries)
	}
	// A builtin citation counts toward its file's module.
	if r := Check(ix, Draft{Engineering: []Claim{{ID: "E1", Topic: "interfaces", Text: "Math.sqrt is a native.", Basis: "observed", Cites: []string{"builtin:interp.cpp:29"}}}}); !slices.Equal(r.Coverage.UncitedModule, []string{"mod:tests"}) {
		t.Errorf("coverage = %+v; the builtin cite covers interp.cpp's module", r.Coverage)
	}
	const cli, tests = "entry:cli.cpp:3", "entry:run_tests.cpp:5"
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
			g.Inputs, g.Exclude = []string{"tests/*.js"}, []string{"tests/math.js needs Math (Q1)", "tests/extra.js needs Extra (Q1)", "tests/json.js needs JSON (Q1)", "tests/boot.js needs Boot (Q1)"}
		}), "case set G1", false},
		{"match", "a file-scope registration in a file the runner never loads", set(func(g *CaseSet) { g.Inputs = []string{"tests/extra.js"} }),
			"tests/extra.js:1 calls Extra.f, which runner entry:cli.cpp:3 does not register", true},
		{"negated", "the runner that loads that file", set(func(g *CaseSet) { g.Runners, g.Inputs = []string{tests}, []string{"tests/extra.js"} }), "case set G1", false},
		{"negated", "a runner that constructs the object whose constructor registers it", set(func(g *CaseSet) { g.Runners, g.Inputs = []string{tests}, []string{"tests/json.js"} }), "case set G1", false},
		{"match", "a runner that only declares a pointer to that object", set(func(g *CaseSet) { g.Inputs = []string{"tests/json.js"} }),
			"tests/json.js:1 calls JSON.parse, which runner entry:cli.cpp:3 does not register", true},
		{"match", "a test file that drives nothing as the runner", set(func(g *CaseSet) { g.Runners = []string{"test:tests/hello.js"} }),
			"runner test:tests/hello.js is a test file that calls no function and loads no file outside the tests", true},
		{"match", "no inputs, and a test file it draws on calls a builtin the runner lacks", set(func(g *CaseSet) { g.From, g.Inputs = []string{cli, "test:tests/math.js"}, nil }),
			"case set G1: tests/math.js:1 calls Math.sqrt, " + notCLI, true},
		{"negated", "no inputs, and the test file it draws on calls only what the runner registers", set(func(g *CaseSet) { g.From, g.Inputs = []string{cli, "test:tests/hello.js"}, nil }), "case set G1", false},
		{"near miss", "a test file it draws on, with inputs that name other files", set(func(g *CaseSet) { g.From = []string{cli, "test:tests/math.js"} }), "case set G1", false},
		{"negated", "a file-scope registration in a file the runner's executable links", set(func(g *CaseSet) { g.Runners, g.Inputs = []string{tests}, []string{"tests/boot.js"} }), "case set G1", false},
		{"match", "the same registration, from an executable that does not link it", set(func(g *CaseSet) { g.Inputs = []string{"tests/boot.js"} }),
			"tests/boot.js:1 calls Boot.go, which runner entry:cli.cpp:3 does not register", true},
		{"match", "an exclusion that names tests/math.js does not exclude math.js", set(func(g *CaseSet) {
			g.Inputs, g.Exclude = []string{"math.js", "tests/math.js"}, []string{"tests/math.js needs Math (Q1)"}
		}), "case set G1: math.js:1 calls Math.sqrt", true},
		{"near miss", "a comment, a string, and another object's sqrt", set(func(g *CaseSet) { g.Inputs = []string{"tests/nearmiss.js"} }), "case set G1", false},
		{"near miss", "a title about Math without naming a builtin", set(func(g *CaseSet) { g.Title = "Math results (sqrt) and myMath.sqrt" }), "case set G1", false},
	})
}

// A program counts once however many entries it has (a shebang and a main
// guard), and a test file runs cases only when it drives the program: it
// reaches a function outside the test files, or loads such a file, directly
// or through a helper.
func TestFloorRunnerPrograms(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("tool.py", "#!/usr/bin/env python3\nimport lib\nimport plugins\n\n\ndef run(env):\n    lib.setup(env)\n    print(lib.render(1))\n\n\nif __name__ == \"__main__\":\n    run(lib.Env())\n\n\ndef hum(x):\n    return x\n\n\ndef unused(env):\n    env.register('hum', hum)\n")
	write("lib.py", "def render(x):\n    return str(x)\n\n\ndef shout(x):\n    return str(x).upper()\n\n\ndef dotf(x):\n    return x\n\n\ndef setup(env):\n    env.register('shout', shout)\n    env.register('dot', lib.dotf)\n")
	write("cases/dot.js", "dot(1);\n")
	write("cases/probe.js", "probe(1);\n")
	write("cases/hum.js", "hum(1);\n")
	write("examples/case.py", "shout(\"x\")\n")
	write("cases/loud.js", "shout(\"hi\");\n")
	write("plugins.py", "import lib\n\n\ndef yell(x):\n    return x\n\n\nlib.REG.register('yell', yell)\n")
	write("cases/yell.js", "yell(1);\n")
	write("quiet.py", "def whisper(x):\n    return x\n\n\ndef setup_quiet(env):\n    env.register('whisper', whisper)\n")
	write("cases/quiet.js", "whisper(\"x\");\n")
	write("tests/test_render.py", "import helpers\n\n\ndef test_render():\n    assert helpers.check(1)\n\n\ndef test_probe(env):\n    env.register('probe', helpers.check)\n")
	write("tests/helpers.py", "import lib\n\n\ndef check(x):\n    return lib.render(x) == str(x)\n")
	write("tests/make_data.py", "def make():\n    return [1, 2]\n")
	write("spec/render_spec.py", "import lib\n\nassert lib.render(3) == \"3\"\n")
	build := func() *Index {
		ix, err := Build(context.Background(), dir, Options{Git: noGit})
		if err != nil {
			t.Fatal(err)
		}
		return ix
	}
	ix := build()
	if n := len(slices.DeleteFunc(slices.Clone(ix.Entries), func(e Fact) bool { return e.Kind != "main" && e.Kind != "script" })); n != 2 {
		t.Fatalf("entries = %v; want tool.py's shebang and its main guard", ix.Entries)
	}
	base := func() Draft {
		return Draft{Modernisation: &Modernisation{CaseSets: []CaseSet{{ID: "G1", Title: "Rendering", From: []string{"sym:lib.py#render"}, Compare: "exact stdout"}}}}
	}
	set := func(f func(g *CaseSet)) func(d *Draft) { return func(d *Draft) { f(&d.Modernisation.CaseSets[0]) } }
	runFloorCases(t, ix, base, []floorCase{
		{"negated", "one program with a shebang and a main guard, no runners", func(*Draft) {}, "name its runners", false},
		{"negated", "no runners: the only program, which registers it", set(func(g *CaseSet) { g.Inputs = []string{"cases/loud.js"} }), "case set G1", false},
		{"match", "no runners: the only program never registers it", set(func(g *CaseSet) { g.Inputs = []string{"cases/quiet.js"} }),
			"cases/quiet.js:1 calls whisper, which runner entry:tool.py:1 does not register", true},
		{"match", "a test file that calls no program function", set(func(g *CaseSet) { g.Runners = []string{"test:tests/make_data.py"} }),
			"runner test:tests/make_data.py is a test file that calls no function and loads no file outside the tests", true},
		{"near miss", "a test that drives the program through a helper", set(func(g *CaseSet) { g.Runners = []string{"test:tests/test_render.py"} }), "case set G1", false},
		{"near miss", "a spec that calls the program only at file scope, after importing it", set(func(g *CaseSet) { g.Runners = []string{"test:spec/render_spec.py"} }), "case set G1", false},
	})
	write("other.py", "#!/usr/bin/env python3\nimport lib\n\n\ndef hush(x):\n    return x\n\n\ndef mute(env):\n    env.register('hush', hush)\n\n\ndef loud(env):\n    env.register('boom', hush)\n\n\nloud(lib.Env())\nprint(lib.render(2))\n")
	write("cases/hush.js", "hush(1);\n")
	write("cases/boom.js", "boom(1);\n")
	runFloorCases(t, build(), base, []floorCase{
		{"match", "a second program, no runners", func(*Draft) {}, "case set G1: name its runners", true},
		{"negated", "the main guard's runner, whose guard calls the function that registers it", set(func(g *CaseSet) { g.Runners, g.Inputs = []string{"entry:tool.py:11"}, []string{"cases/loud.js"} }), "case set G1", false},
		{"match", "a program that never calls that function", set(func(g *CaseSet) { g.Runners, g.Inputs = []string{"entry:other.py:1"}, []string{"cases/loud.js"} }),
			"cases/loud.js:1 calls shout, which runner entry:other.py:1 does not register", true},
		{"match", "an input in the host's own language", set(func(g *CaseSet) { g.Runners, g.Inputs = []string{"entry:other.py:1"}, []string{"examples/case.py"} }),
			"examples/case.py:1 calls shout, which runner entry:other.py:1 does not register", true},
		{"negated", "a registration whose callback is dotted (lib.dotf), from the runner that registers it", set(func(g *CaseSet) { g.Runners, g.Inputs = []string{"entry:tool.py:11"}, []string{"cases/dot.js"} }), "case set G1", false},
		{"match", "the dotted registration, from a runner that never makes it", set(func(g *CaseSet) { g.Runners, g.Inputs = []string{"entry:other.py:1"}, []string{"cases/dot.js"} }),
			"cases/dot.js:1 calls dot, which runner entry:other.py:1 does not register", true},
		{"negated", "a test harness that registers the builtin itself", set(func(g *CaseSet) {
			g.Runners, g.Inputs = []string{"test:tests/test_render.py"}, []string{"cases/probe.js"}
		}), "case set G1", false},
		{"match", "the harness's registration, from a program that never makes it", set(func(g *CaseSet) { g.Runners, g.Inputs = []string{"entry:other.py:1"}, []string{"cases/probe.js"} }),
			"cases/probe.js:1 calls probe, which runner entry:other.py:1 does not register", true},
		{"negated", "a script whose top-level code calls the function that registers it", set(func(g *CaseSet) { g.Runners, g.Inputs = []string{"entry:other.py:1"}, []string{"cases/boom.js"} }), "case set G1", false},
		{"match", "a function of the script its top-level code never calls", set(func(g *CaseSet) { g.Runners, g.Inputs = []string{"entry:other.py:1"}, []string{"cases/hush.js"} }),
			"cases/hush.js:1 calls hush, which runner entry:other.py:1 does not register", true},
		{"match", "a function of the guard's file the guard never calls", set(func(g *CaseSet) { g.Runners, g.Inputs = []string{"entry:tool.py:11"}, []string{"cases/hum.js"} }),
			"cases/hum.js:1 calls hum, which runner entry:tool.py:11 does not register", true},
		{"negated", "a file-scope registration in a module the runner imports", set(func(g *CaseSet) { g.Runners, g.Inputs = []string{"entry:tool.py:11"}, []string{"cases/yell.js"} }), "case set G1", false},
		{"match", "the same registration, from a program that never imports it", set(func(g *CaseSet) { g.Runners, g.Inputs = []string{"entry:other.py:1"}, []string{"cases/yell.js"} }),
			"cases/yell.js:1 calls yell, which runner entry:other.py:1 does not register", true},
	})
}
