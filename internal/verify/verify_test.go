package verify

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// repo makes a git repository with a base commit and a head commit that
// writes files; it returns the root, base, and head.
func repo(t *testing.T, base, head map[string]string) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	git(t, root, "init", "--quiet", "-b", "dev")
	write := func(files map[string]string) {
		for p, s := range files {
			f := filepath.Join(root, filepath.FromSlash(p))
			_ = os.MkdirAll(filepath.Dir(f), 0o755)
			if err := os.WriteFile(f, []byte(s), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		git(t, root, "add", "-A")
		git(t, root, "commit", "--quiet", "--allow-empty", "-m", "c")
	}
	write(base)
	b := git(t, root, "rev-parse", "HEAD")
	write(head)
	return root, b, git(t, root, "rev-parse", "HEAD")
}

func newRun(t *testing.T, root, base, head string, path string) *Run {
	t.Helper()
	r := &Run{Root: root, Base: base, Head: head, Work: t.TempDir()}
	var err error
	if r.Changed, r.Lines, err = Changes(context.Background(), root, base, head); err != nil {
		t.Fatal(err)
	}
	r.Floors.Coverage.MinDiffPercent = 90
	r.Sandbox = &Sandbox{Home: t.TempDir(), Getenv: func(k string) string {
		if k == "PATH" {
			return path
		}
		return ""
	}}
	return r
}

// WP4.3 accept: 89.9 % fails and 90.0 % passes, at the exact boundary.
func TestDiffCoverageBoundary(t *testing.T) {
	changed := map[string][]int{"a.go": {}}
	cov := Lines{"a.go": {}}
	for n := 1; n <= 1000; n++ {
		changed["a.go"] = append(changed["a.go"], n)
		cov["a.go"][n] = n <= 899
	}
	if d := Diff(cov, changed, 90); d.Pass || d.Covered != 899 || d.Executable != 1000 {
		t.Fatalf("899/1000 (89.9 %%): %+v", d)
	}
	cov["a.go"][900] = true
	if d := Diff(cov, changed, 90); !d.Pass || d.Percent != 90 {
		t.Fatalf("900/1000 (90.0 %%): %+v", d)
	}
	// Lines the report does not list are not executable; no executable change passes.
	if d := Diff(Lines{}, map[string][]int{"README.md": {1, 2}}, 90); !d.Pass || d.Executable != 0 {
		t.Fatalf("no executable lines: %+v", d)
	}
	// A threshold with a fraction: 9 of 10 fails 90.5.
	if d := Diff(Lines{"b.go": {1: true, 2: true, 3: true, 4: true, 5: true, 6: true, 7: true, 8: true, 9: true, 10: false}}, map[string][]int{"b.go": {1, 2, 3, 4, 5, 6, 7, 8, 9, 10}}, 90.5); d.Pass {
		t.Fatalf("9/10 against 90.5: %+v", d)
	}
}

// The three report formats describe the same coverage the same way.
func TestParseCoverageFormats(t *testing.T) {
	goProfile := "mode: set\nexample.com/m/pkg/a.go:3.10,5.2 2 1\nexample.com/m/pkg/a.go:7.10,8.2 1 0\n"
	lcov := "TN:\nSF:/src/repo/pkg/a.go\nDA:3,1\nDA:4,1\nDA:5,1\nDA:7,0\nDA:8,0\nend_of_record\n"
	cobertura := `<?xml version="1.0"?><coverage><sources><source>/src/repo</source></sources><packages><package><classes>
<class filename="pkg/a.go"><lines><line number="3" hits="1"/><line number="4" hits="2"/><line number="5" hits="1"/><line number="7" hits="0"/><line number="8" hits="0"/></lines></class>
</classes></package></packages></coverage>`
	want := map[int]bool{3: true, 4: true, 5: true, 7: false, 8: false}
	for name, in := range map[string]string{"go coverprofile": goProfile, "lcov": lcov, "cobertura": cobertura} {
		l, format, err := ParseCoverage([]byte(in), "example.com/m", "/src/repo")
		if err != nil || format != name {
			t.Fatalf("%s: %v (%s)", name, err, format)
		}
		if got := l["pkg/a.go"]; fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("%s: %v, want %v", name, got, want)
		}
		d := Diff(l, map[string][]int{"pkg/a.go": {4, 6, 7}}, 50)
		if d.Covered != 1 || d.Executable != 2 || !d.Pass || !slices.Equal(d.Uncovered["pkg/a.go"], []int{7}) {
			t.Errorf("%s diff: %+v", name, d)
		}
	}
	if _, _, err := ParseCoverage([]byte("hello"), "", ""); err == nil {
		t.Error("garbage parsed as coverage")
	}
}

// The coverage floor end to end: a configured command, an lcov report, and
// the changed lines of a real git diff; 899 of 1000 fails, 900 passes.
func TestCoverageFloorBoundary(t *testing.T) {
	var src strings.Builder
	for n := 1; n <= 1000; n++ {
		fmt.Fprintf(&src, "line %d\n", n)
	}
	for _, covered := range []int{899, 900} {
		var lcov strings.Builder
		lcov.WriteString("SF:src.txt\n")
		for n := 1; n <= 1000; n++ {
			hit := 0
			if n <= covered {
				hit = 1
			}
			fmt.Fprintf(&lcov, "DA:%d,%d\n", n, hit)
		}
		lcov.WriteString("end_of_record\n")
		root, base, head := repo(t, map[string]string{"README.md": "x\n"}, map[string]string{"src.txt": src.String(), "cov/lcov.info": lcov.String()})
		r := newRun(t, root, base, head, os.Getenv("PATH"))
		r.Floors.Coverage.Command, r.Floors.Coverage.Report = []string{"true"}, "cov/lcov.info"
		rep := Execute(context.Background(), r, []Floor{Coverage{}})
		c := rep.Checks[0]
		want := proof.Pass
		if covered == 899 {
			want = proof.Fail
		}
		if c.Result != want || rep.Hold != (want == proof.Pass) || len(rep.Artifacts) != 1 || rep.Artifacts[0].Path != "coverage/diff.json" {
			t.Fatalf("%d/1000: %+v hold=%t artifacts=%d", covered, c, rep.Hold, len(rep.Artifacts))
		}
		if covered == 899 && c.Value != 89.9 {
			t.Fatalf("value shown as %v, want 89.9", c.Value)
		}
	}
}

func TestParseDiff(t *testing.T) {
	d := ParseDiff([]byte("diff --git a/x.go b/x.go\n--- a/x.go\n+++ b/x.go\n@@ -3,0 +4,2 @@\n+a\n+b\n@@ -10 +12 @@\n-c\n+d\n" +
		"diff --git a/gone.go b/gone.go\n--- a/gone.go\n+++ /dev/null\n@@ -1,2 +0,0 @@\n-x\n-y\n" +
		"diff --git a/y.go b/y.go\n--- a/y.go\n+++ b/y.go\n@@ -5,2 +5,0 @@\n-p\n-q\n"))
	if !slices.Equal(d["x.go"], []int{4, 5, 12}) || len(d["y.go"]) != 0 {
		t.Fatalf("%v", d)
	}
	if _, ok := d["gone.go"]; ok {
		t.Fatalf("deleted file kept: %v", d)
	}
}

func TestFootprintFromLayers(t *testing.T) {
	l := ReadLayers(filepath.Join("..", ".."))
	if fp := l.Footprint([]string{"internal/verify/verify.go", "internal/cli/verify.go"}); !slices.Equal(fp, []string{"L02", "L04"}) {
		t.Fatalf("internal paths: %v", fp)
	}
	if fp := l.Footprint([]string{"packages/design-system/src/button.tsx", ".github/workflows/ci.yml", "go.mod"}); !slices.Equal(fp, []string{"L01", "L09"}) {
		t.Fatalf("L01 and L09 paths: %v", fp)
	}
	if fp := ReadLayers(t.TempDir()).Footprint([]string{"a.go"}); fp != nil {
		t.Fatalf("no LAYERS.md: %v", fp)
	}
}

// fakeTool writes an executable script that writes report to the path after
// flag (or prints it), then exits with code.
func fakeTool(t *testing.T, dir, name, flag, report string, code int) {
	t.Helper()
	script := fmt.Sprintf(`#!/bin/sh
case "$1" in --version|version) echo "%s 0.0.0-fake"; exit 0;; esac
out=""
while [ $# -gt 0 ]; do [ "$1" = "%s" ] && out="$2"; shift; done
cat > "${out:-/dev/stdout}" <<'JSON'
%s
JSON
exit %d
`, name, flag, report, code)
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

// WP4.4 thresholds and parsing, with stand-in tools that print recorded
// reports: findings above zero critical/high fail, medium and low do not.
func TestScannerFloorsFindings(t *testing.T) {
	root, base, head := repo(t, map[string]string{"main.go": "package main\n", "go.mod": "module example.com/m\n\ngo 1.22\n"}, map[string]string{"main.go": "package main\n\nfunc main() {}\n"})
	cases := []struct {
		name    string
		floor   Floor
		tool    string
		flag    string
		planted string
		clean   string
	}{
		{"sast", SAST{}, "semgrep", "--output",
			`{"results":[{"check_id":"go.lang.security.injection.tainted-sql-string.tainted-sql-string","path":"main.go","start":{"line":3},"extra":{"severity":"ERROR","metadata":{"impact":"MEDIUM"}}},{"check_id":"use-of-md5","path":"main.go","start":{"line":4},"extra":{"severity":"WARNING"}}],"errors":[]}`,
			`{"results":[{"check_id":"use-of-md5","path":"main.go","start":{"line":4},"extra":{"severity":"WARNING"}}],"errors":[]}`},
		{"secrets", Secrets{}, "gitleaks", "--report-path",
			`[{"RuleID":"github-pat","File":"config.txt","StartLine":1,"Commit":"abcdef1234"}]`, `[]`},
		{"deps", Deps{}, "osv-scanner", "--output",
			`{"results":[{"packages":[{"package":{"name":"golang.org/x/text","version":"0.3.0","ecosystem":"Go"},"groups":[{"ids":["GO-2020-0015"],"max_severity":"7.5"}],"vulnerabilities":[]}]}]}`,
			`{"results":[{"packages":[{"package":{"name":"left-pad","version":"1.0.0","ecosystem":"npm"},"groups":[{"ids":["GHSA-x"],"max_severity":"5.3"}]}]}]}`},
	}
	for _, c := range cases {
		for _, planted := range []bool{true, false} {
			bin := t.TempDir()
			report := c.clean
			if planted {
				report = c.planted
			}
			fakeTool(t, bin, c.tool, c.flag, report, 0)
			r := newRun(t, root, base, head, bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			res := Execute(context.Background(), r, []Floor{c.floor})
			want := proof.Pass
			if planted {
				want = proof.Fail
			}
			if got := res.Checks[0]; got.Result != want || got.Version == "" {
				t.Errorf("%s planted=%t: %+v", c.name, planted, got)
			}
		}
	}
	// A missing tool blocks the floor; it never passes.
	r := newRun(t, root, base, head, t.TempDir())
	t.Setenv("PATH", t.TempDir())
	if res := Execute(context.Background(), r, []Floor{SAST{}}); res.Checks[0].Result != proof.Blocked || res.Hold {
		t.Fatalf("missing semgrep: %+v", res.Checks[0])
	}
}

// WP4.4 accept with the real tools (run by the CI security job, which
// installs the pinned semgrep, gitleaks, and osv-scanner): planted findings
// fail each floor; a clean fixture passes. The planted fixtures are written
// at run time so this repository's own scans never see them.
func TestFloorsWithRealTools(t *testing.T) {
	if os.Getenv("AAYU_FLOOR_TOOLS") != "1" {
		t.Skip("set AAYU_FLOOR_TOOLS=1 with semgrep, gitleaks, and osv-scanner on PATH")
	}
	clean := map[string]string{"go.mod": "module example.com/clean\n\ngo 1.22\n", "main.go": "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"hello\") }\n"}
	sql := "package main\n\nimport (\n\t\"database/sql\"\n\t\"fmt\"\n\t\"net/http\"\n)\n\nfunc lookup(w http.ResponseWriter, r *http.Request, db *sql.DB) {\n\tq := fmt.Sprintf(\"SELECT * FROM users WHERE name = '%s'\", r.URL.Query().Get(\"name\"))\n\t_, _ = db.Query(q)\n}\n"
	pat := "token = \"gh" + "p_" + "k3Jq9ZtXw2LmN8vRb5Ty1HcPd7Fs4Ga6Ue0Q\"\n"
	planted := map[string]string{"go.mod": "module example.com/planted\n\ngo 1.22\n\nrequire golang.org/x/text v0.3.0\n", "lookup.go": sql, "config.txt": pat}
	for name, files := range map[string]map[string]string{"clean": clean, "planted": planted} {
		root, base, head := repo(t, map[string]string{"README.md": "fixture\n"}, files)
		r := newRun(t, root, base, head, os.Getenv("PATH"))
		rep := Execute(context.Background(), r, []Floor{SAST{}, Secrets{}, Deps{}})
		for _, c := range rep.Checks {
			want := proof.Pass
			if name == "planted" {
				want = proof.Fail
			}
			t.Logf("%s %s: %s (%s %s) %s", name, c.Floor, c.Result, c.Tool, c.Version, c.Detail)
			if c.Result != want {
				t.Errorf("%s fixture, %s floor: %s, want %s: %s", name, c.Floor, c.Result, want, c.Detail)
			}
		}
	}
}

func TestTestEvidence(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, ".aayudha", "evidence"), 0o755)
	_ = os.WriteFile(EvidencePath(root, 7), []byte("[[criterion]]\nid = 1\ntests = [\"TestGood\"]\n\n[[criterion]]\nid = 2\ntests = [\"pkg/b:TestBad\"]\n\n[[criterion]]\nid = 9\ntests = [\"TestGood\"]\n"), 0o644)
	events := `{"Action":"run","Package":"m/pkg/a","Test":"TestGood"}
{"Action":"output","Package":"m/pkg/a","Test":"TestGood","Output":"ok\n"}
{"Action":"pass","Package":"m/pkg/a","Test":"TestGood"}
{"Action":"run","Package":"m/pkg/b","Test":"TestBad"}
{"Action":"fail","Package":"m/pkg/b","Test":"TestBad"}
`
	r := &Run{Root: root, Story: 7, TestEvents: []byte(events), Criteria: []workitems.Criterion{{N: 1, Evidence: "metric"}, {N: 2, Evidence: "api-trace"}}}
	ev, problems, err := TestEvidence(r)
	if err != nil {
		t.Fatal(err)
	}
	if len(ev) != 1 || ev[0].Path != "evidence/criterion-1.jsonl" || ev[0].Kind != "metric" || !slices.Equal(ev[0].Criteria, []int{1}) || strings.Count(string(ev[0].Data), "\n") != 3 {
		t.Fatalf("evidence: %+v", ev)
	}
	if !slices.Contains(problems, "criterion 2: test pkg/b:TestBad did not pass in this run") || !slices.Contains(problems, "criterion 9 in the evidence map is not in the Ready snapshot") {
		t.Fatalf("problems: %q", problems)
	}
	p, _ := Pack(r, Report{Artifacts: ev}, proof.Pack{})
	if un := Unmapped(r, p); !slices.Equal(un, []int{2}) {
		t.Fatalf("unmapped: %v", un)
	}
}

// The deps floor names the lockfiles git tracks rather than scanning
// recursively, so a checkout inside an ignored directory (a build worktree
// under .aayudha/worktrees/) is still scanned instead of passing on nothing.
func TestTrackedLockfiles(t *testing.T) {
	root, _, _ := repo(t, map[string]string{".gitignore": "wt/\n", "go.mod": "module m\n", "web/pnpm-lock.yaml": "lockfileVersion: '9.0'\n",
		"internal/x/testdata/go.mod": "module fixture\n", "vendor/y/go.mod": "module y\n", "README.md": "x\n"}, nil)
	if got := TrackedLockfiles(context.Background(), root); !slices.Equal(got, []string{"go.mod", "web/pnpm-lock.yaml"}) {
		t.Fatalf("lockfiles: %v", got)
	}
	wt := filepath.Join(root, "wt", "1")
	git(t, root, "worktree", "add", "--quiet", "--detach", wt)
	if got := TrackedLockfiles(context.Background(), wt); !slices.Equal(got, []string{"go.mod", "web/pnpm-lock.yaml"}) {
		t.Fatalf("lockfiles in a worktree under an ignored directory: %v", got)
	}
	if got := TrackedLockfiles(context.Background(), t.TempDir()); got != nil {
		t.Fatalf("not a repository: %v", got)
	}
}

// osv-scanner's Go call analysis: advisories in code the module never calls
// do not fail the floor (osv-scanner itself exits 0 on them); a called one does.
func TestParseOSVCallAnalysis(t *testing.T) {
	report := func(called bool) []byte {
		return []byte(fmt.Sprintf(`{"results":[{"packages":[{"package":{"name":"golang.org/x/crypto","version":"0.55.0","ecosystem":"Go"},
"groups":[{"ids":["GO-2026-5932"],"max_severity":"","experimental_analysis":{"GO-2026-5932":{"called":%t,"unimportant":false}}}]}]}]}`, called))
	}
	s, err := ParseOSV(report(false))
	if err != nil || s.High != 0 || s.Uncalled != 1 || !strings.Contains(s.detail(), "1 more not called") {
		t.Fatalf("uncalled: %+v %v", s, err)
	}
	if s, err = ParseOSV(report(true)); err != nil || s.High != 1 || s.Uncalled != 0 {
		t.Fatalf("called: %+v %v", s, err)
	}
}
