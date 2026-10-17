package survey

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// --probe finds what reading the code cannot: output that changes between
// identical runs, output that depends on TZ or HOME, files the program writes
// (under strace), and functions an lcov tracefile shows the cases never reach.
// Each finding becomes a citable probe fact.
func TestProbe(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "legacy.sh")
	script := `#!/bin/sh
case "$1" in
  pid) echo "run $$" ;;
  tz) echo "zone ${TZ}" ;;
  home) cat "$HOME/.aayu-probe" 2>/dev/null || echo none ;;
  write) cd "$HOME" && echo saved > out.log; echo ok ;;
  load) cat lib/tool.py README.md "$LIBDIR"/*.rb "${GEM_PATH##*:}"/gems/x.rb "$2" > /dev/null; echo ok ;;
esac
`
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	// The checker build reports a runtime error on one case and prints
	// something else on another, beside its report on stderr.
	checker := filepath.Join(dir, "checked.sh")
	if err := os.WriteFile(checker, []byte(`#!/bin/sh
case "$1" in
  home) echo "Fortran runtime error: Index '11' of dimension 1 of array 'x' above upper bound of 10" >&2; cat "$HOME/.aayu-probe" 2>/dev/null || echo none ;;
  write) echo "previous warning" >&2; echo "$PWD/lib/write.rb:3:in 'save': can't modify frozen String: \"\" (FrozenError)" >&2 ;;
  *) exec `+bin+` "$@" ;;
esac
`), 0o755); err != nil {
		t.Fatal(err)
	}
	libdir := t.TempDir() // a gem directory outside the checkout
	for i := range 6 {
		if err := os.WriteFile(filepath.Join(libdir, fmt.Sprintf("m%d.rb", i)), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gems := t.TempDir() // a library path the case sets: loads from it are expected
	if err := os.MkdirAll(filepath.Join(gems, "gems"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gems, "gems", "x.rb"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	c := Cases{Cases: []Case{
		{ID: "pid", Set: "G1", Argv: []string{"{bin}", "pid"}},
		{ID: "tz", Set: "G1", Argv: []string{"{bin}", "tz"}},
		{ID: "home", Set: "G1", Argv: []string{"{bin}", "home"}},
		{ID: "write", Set: "G1", Argv: []string{"{bin}", "write"}},
		{ID: "load", Set: "G1", Argv: []string{"{bin}", "load", "Makefile"}, Env: []string{"LIBDIR=" + libdir, "GEM_PATH=/no/such/dir:" + gems}},
	}}
	// The index is read back from index.json, as `aayu survey --golden` reads it.
	idx := filepath.Join(dir, "index.json")
	if err := buildFixture(t, 0).Save(idx); err != nil {
		t.Fatal(err)
	}
	ix, err := Load(idx)
	if err != nil {
		t.Fatal(err)
	}
	root, _ := filepath.Abs(filepath.Join("testdata", "legacy"))
	lcov := filepath.Join(dir, "cov.info")
	// parse (src/util.cpp:5-10) never runs; report (12-16) does.
	if err := os.WriteFile(lcov, []byte("SF:"+filepath.Join(root, "src", "util.cpp")+"\nDA:6,0\nDA:7,0\nDA:9,0\nDA:14,3\nDA:15,3\nend_of_record\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The checkout is given relative, as `aayu survey .` gives it; lcov's paths are absolute.
	p, err := Probe(context.Background(), ix, filepath.Join("testdata", "legacy"), bin, "cases.json", c, ProbeOptions{Coverage: lcov, CheckBin: checker})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Fact{}
	for _, f := range p.Facts {
		got[f.ID] = f
	}
	for _, want := range []string{"probe:nondeterministic:pid", "probe:env-tz:tz", "probe:env-home:home", "probe:unreached:src/util.cpp#parse", "probe:checker:home", "probe:check-differs:write"} {
		if _, ok := got[want]; !ok {
			t.Errorf("no probe fact %s in %v", want, p.Facts)
		}
	}
	// A difference on the checker build says why: its error, not a warning before it.
	if f := got["probe:check-differs:write"]; !strings.Contains(f.Detail, "(its stderr: lib/write.rb:3:in 'save'") || !strings.Contains(f.Detail, "(FrozenError)") {
		t.Errorf("the check-differs fact does not carry the checker's error: %q", f.Detail)
	}
	// A case that differs between identical runs says nothing about its environment.
	for _, no := range []string{"probe:nondeterministic:tz", "probe:env-tz:pid", "probe:unreached:src/util.cpp#report", "probe:env-home:tz", "probe:reads:" + bin, "probe:check-differs:home"} { // stderr is the checker's report, not a difference
		if _, ok := got[no]; ok {
			t.Errorf("unexpected probe fact %s", no)
		}
	}
	if _, err := exec.LookPath("strace"); err == nil {
		if f, ok := got["probe:writes:~/out.log"]; !ok || !strings.Contains(f.Detail, "write") {
			t.Errorf("strace is installed, but no write of out.log: %v", p.Facts)
		}
		// A data file read is a dependency; the checkout's own source, as an interpreter loads it, is not.
		if _, ok := got["probe:reads:README.md"]; !ok {
			t.Errorf("no read of README.md: %v", p.Facts)
		}
		if _, ok := got["probe:reads:lib/tool.py"]; ok {
			t.Error("a read of the checkout's own source is reported as a dependency")
		}
		if _, ok := got["probe:reads:Makefile"]; ok {
			t.Error("the input the case names on its command line is reported as a dependency")
		}
		// Many files under one directory outside the checkout are one dependency.
		if f, ok := got["probe:reads:"+libdir+"/"]; !ok || !strings.Contains(f.Detail, "6 files under") {
			t.Errorf("no single read of %s/: %v", libdir, p.Facts)
		}
		// A library path the case sets (GEM_PATH) is expected, not a dependency; LIBDIR above is not one.
		for id := range got {
			if strings.Contains(id, gems) {
				t.Errorf("a load from the case's GEM_PATH is reported as a dependency: %s", id)
			}
		}
		if !strings.Contains(strings.Join(p.Ran, "\n"), "1 file(s) read under GEM_PATH entry "+gems) {
			t.Errorf("the library-path reads are not summarised: %v", p.Ran)
		}
		if _, ok := got["probe:reads:"+filepath.Join(libdir, "m0.rb")]; ok {
			t.Error("a file of a grouped directory is reported on its own")
		}
	} else if !strings.Contains(strings.Join(p.Skipped, " "), "strace is not installed") {
		t.Errorf("a skipped probe must say so: %v", p.Skipped)
	}
	// A draft cites a probe fact once probes.json sits next to the capture.
	out := t.TempDir()
	if err := os.MkdirAll(filepath.Join(out, "golden", "cases"), 0o755); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(p)
	if err := os.WriteFile(filepath.Join(out, "golden", "cases", "probes.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ix.Resolve("probe:env-tz:tz"); err == nil {
		t.Error("a probe fact resolves before its probes.json is loaded")
	}
	if err := ix.LoadProbes(out); err != nil || ix.Resolve("probe:env-tz:tz") != nil || ix.Resolve("probe:env-tz:nope") == nil {
		t.Errorf("load probes: %v", err)
	}
}
