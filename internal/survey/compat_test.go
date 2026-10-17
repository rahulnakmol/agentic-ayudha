package survey

import (
	"context"
	"encoding/json"
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
  write) echo saved > "$HOME/out.log"; echo ok ;;
esac
`
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	c := Cases{Cases: []Case{
		{ID: "pid", Set: "G1", Argv: []string{"{bin}", "pid"}},
		{ID: "tz", Set: "G1", Argv: []string{"{bin}", "tz"}},
		{ID: "home", Set: "G1", Argv: []string{"{bin}", "home"}},
		{ID: "write", Set: "G1", Argv: []string{"{bin}", "write"}},
	}}
	ix := buildFixture(t, 0)
	root, _ := filepath.Abs(filepath.Join("testdata", "legacy"))
	lcov := filepath.Join(dir, "cov.info")
	// parse (src/util.cpp:5-10) never runs; report (12-16) does.
	if err := os.WriteFile(lcov, []byte("SF:"+filepath.Join(root, "src", "util.cpp")+"\nDA:6,0\nDA:7,0\nDA:9,0\nDA:14,3\nDA:15,3\nend_of_record\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Probe(context.Background(), ix, root, bin, "cases.json", c, ProbeOptions{Coverage: lcov})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Fact{}
	for _, f := range p.Facts {
		got[f.ID] = f
	}
	for _, want := range []string{"probe:nondeterministic:pid", "probe:env-tz:tz", "probe:env-home:home", "probe:unreached:src/util.cpp#parse"} {
		if _, ok := got[want]; !ok {
			t.Errorf("no probe fact %s in %v", want, p.Facts)
		}
	}
	// A case that differs between identical runs says nothing about its environment.
	for _, no := range []string{"probe:nondeterministic:tz", "probe:env-tz:pid", "probe:unreached:src/util.cpp#report", "probe:env-home:tz", "probe:reads:" + bin} {
		if _, ok := got[no]; ok {
			t.Errorf("unexpected probe fact %s", no)
		}
	}
	if _, err := exec.LookPath("strace"); err == nil {
		if f, ok := got["probe:writes:~/out.log"]; !ok || !strings.Contains(f.Detail, "write") {
			t.Errorf("strace is installed, but no write of out.log: %v", p.Facts)
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
