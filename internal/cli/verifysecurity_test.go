package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
)

// securityTools writes stand-in semgrep, gitleaks, and osv-scanner that copy
// reports/<tool>.json to the report path they are given; gitleaks also keeps
// its arguments, so the test can see which history it was asked to scan.
func securityTools(t *testing.T) (bin, reports string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell stand-ins")
	}
	bin, reports = t.TempDir(), t.TempDir()
	for tool, flag := range map[string]string{"semgrep": "--output", "gitleaks": "--report-path", "osv-scanner": "--output"} {
		script := fmt.Sprintf(`#!/bin/sh
case "$1" in --version|version) echo "%[1]s 0.0.0-fake"; exit 0;; esac
echo "$@" > '%[3]s/%[1]s.args'
out=""
while [ $# -gt 0 ]; do [ "$1" = "%[2]s" ] && out="$2"; shift; done
cat '%[3]s/%[1]s.json' > "$out"
`, tool, flag, reports)
		if err := os.WriteFile(filepath.Join(bin, tool), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return bin, reports
}

const (
	semgrepHigh   = `{"results":[{"check_id":"go.lang.security.audit.sqli","path":"main.go","start":{"line":%d},"extra":{"severity":"ERROR"}}],"errors":[]}`
	semgrepMedium = `{"results":[{"check_id":"use-of-md5","path":"main.go","start":{"line":4},"extra":{"severity":"WARNING"}}],"errors":[]}`
	leak          = `[{"RuleID":"github-pat","File":"config.txt","StartLine":1,"Commit":"abcdef1234"}]`
	osvHigh       = `{"results":[{"source":{"path":"go.mod"},"packages":[{"package":{"name":"golang.org/x/text","version":"%s","ecosystem":"Go"},"groups":[{"ids":["GO-2020-0015","CVE-2020-14040"],"max_severity":"7.5"}]}]}]}`
)

func writeReports(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for tool, b := range files {
		if err := os.WriteFile(filepath.Join(dir, tool+".json"), []byte(b), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// Decision #480, Story #487 criteria 2 and 3: a push's failing findings file
// one aayudha:security Bug each, keyed by floor, ID, and path; later runs
// comment instead of filing; a finding the floor no longer reports is marked
// gone on that branch, except a secret after a range scan, which read only
// the pushed commits.
func TestVerifySecurityBugs(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "--quiet", "-b", "dev")
	_ = os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n\ngo 1.22\n"), 0o644)
	git(t, dir, "add", ".")
	git(t, dir, "commit", "--quiet", "-m", "feat: one")
	first := git(t, dir, "rev-parse", "HEAD")
	git(t, dir, "commit", "--quiet", "--allow-empty", "-m", "feat: two")
	second := git(t, dir, "rev-parse", "HEAD")

	g := fake.New(t, "o/r", "User")
	other := g.AddIssue("Unrelated bug", "### Summary\n\nnot a security finding\n", "type:bug")
	bin, reports := securityTools(t)
	env := map[string]string{"GITHUB_TOKEN": "workflow-token", "PATH": bin + string(os.PathListSeparator) + os.Getenv("PATH")}
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir}
	verifySec := func(since string) (int, SecurityResult, string) {
		t.Helper()
		args := []string{"verify", "--security", "--ci", "--repo", "o/r", "--json"}
		if since != "" {
			args = append(args, "--since", since)
		}
		code, out, errOut := run(t, env, opts, args...)
		var res SecurityResult
		if err := json.NewDecoder(strings.NewReader(out)).Decode(&res); err != nil {
			t.Fatalf("verify --security: %d %s %s", code, out, errOut)
		}
		return code, res, errOut
	}
	security := func() map[int]*fake.Issue {
		g.Mu.Lock()
		defer g.Mu.Unlock()
		out := map[int]*fake.Issue{}
		for n, is := range g.Issues {
			for _, l := range is.Labels {
				if l.Name == SecurityLabel {
					out[n] = is
				}
			}
		}
		return out
	}
	gitleaksArgs := func() string {
		b, _ := os.ReadFile(filepath.Join(reports, "gitleaks.args"))
		return string(b)
	}

	// Run 1, dev at first, a new branch (before is all zeros): a high SAST
	// finding, a secret, and a high advisory each file a Bug; a medium SAST
	// finding files none. The secrets scan reads the whole history of HEAD.
	writeReports(t, reports, map[string]string{"semgrep": fmt.Sprintf(semgrepHigh, 3), "gitleaks": leak, "osv-scanner": fmt.Sprintf(osvHigh, "0.3.0")})
	git(t, dir, "checkout", "--quiet", "--detach", first)
	git(t, dir, "branch", "--quiet", "-f", "dev", first)
	git(t, dir, "checkout", "--quiet", "dev")
	code, res, errOut := verifySec(strings.Repeat("0", 40))
	if code != ExitFloor || res.Branch != "dev" || res.Base != "" || len(res.Bugs) != 3 {
		t.Fatalf("run 1: %d %+v %s", code, res, errOut)
	}
	if a := gitleaksArgs(); !strings.Contains(a, "--log-opts "+first) || strings.Contains(a, "..") {
		t.Fatalf("a new branch's secrets scan reads the whole history of HEAD: %s", a)
	}
	bugs := security()
	if len(bugs) != 3 {
		t.Fatalf("want 3 security Bugs, got %d", len(bugs))
	}
	var sast, secret, deps *fake.Issue
	for _, is := range bugs {
		switch {
		case strings.Contains(is.Title, "sast finding go.lang.security.audit.sqli in main.go"):
			sast = is
		case strings.Contains(is.Title, "secrets finding github-pat in config.txt"):
			secret = is
		case strings.Contains(is.Title, "deps finding CVE-2020-14040 golang.org/x/text in go.mod"):
			deps = is
		}
		if !strings.Contains(is.Body, "`dev` at "+first) || !strings.Contains(strings.Join(g.LabelNames(is.Number), ","), "type:bug") {
			t.Errorf("Bug #%d lacks the branch and commit or the type: %s", is.Number, is.Body)
		}
	}
	if sast == nil || secret == nil || deps == nil {
		t.Fatalf("Bugs by finding: %v", bugs)
	}

	// Run 2, dev at second, pushed from first: the SAST finding moved a line and
	// the package moved a version, so both are the same findings; nothing new is
	// filed, each Bug gets one comment. The secrets scan reads only the push.
	git(t, dir, "reset", "--quiet", "--hard", second)
	writeReports(t, reports, map[string]string{"semgrep": fmt.Sprintf(semgrepHigh, 9), "osv-scanner": fmt.Sprintf(osvHigh, "0.3.2")})
	code, res, _ = verifySec(first)
	if code != ExitFloor || res.Base != first || len(security()) != 3 {
		t.Fatalf("run 2: %d %+v", code, res)
	}
	if a := gitleaksArgs(); !strings.Contains(a, "--log-opts "+first+".."+second) {
		t.Fatalf("a push's secrets scan reads only the pushed commits: %s", a)
	}
	for _, is := range []*fake.Issue{sast, secret, deps} {
		if n := len(is.Comments); n != 1 || !strings.Contains(is.Comments[0].Body, "Found again on `dev` at "+second) {
			t.Fatalf("Bug #%d after run 2: %d comments %+v", is.Number, n, is.Comments)
		}
	}
	// The same run again changes nothing.
	verifySec(first)
	if len(sast.Comments) != 1 || len(security()) != 3 {
		t.Fatalf("a repeated run commented again: %+v", sast.Comments)
	}

	// A range scan that no longer sees the secret does not clear it: the
	// secret may sit in history before the pushed commits.
	writeReports(t, reports, map[string]string{"gitleaks": `[]`})
	verifySec(first)
	if len(secret.Comments) != 1 {
		t.Fatalf("a range scan cleared a secret: %+v", secret.Comments)
	}

	// Run 3, rolling at second, a range scan where the secret is not in the
	// pushed commits and SAST is clean: the SAST Bug is gone on rolling only if
	// it was seen there (it was not), the secret is not cleared by a range scan.
	git(t, dir, "checkout", "--quiet", "-b", "rolling")
	writeReports(t, reports, map[string]string{"semgrep": semgrepMedium, "gitleaks": `[]`})
	code, res, _ = verifySec(first)
	if code != ExitFloor || len(sast.Comments) != 1 || len(secret.Comments) != 1 || !strings.Contains(deps.Comments[len(deps.Comments)-1].Body, "on `rolling` at "+second) {
		t.Fatalf("run 3: %d %+v", code, res)
	}

	// Run 4, nightly on dev (no before): the whole history has no secret and
	// SAST is clean, so both Bugs are marked gone on dev; deps stays.
	git(t, dir, "checkout", "--quiet", "dev")
	code, res, errOut = verifySec("")
	if code != ExitFloor {
		t.Fatalf("run 4: %d %+v %s", code, res, errOut)
	}
	for _, is := range []*fake.Issue{sast, secret} {
		if last := is.Comments[len(is.Comments)-1].Body; !strings.Contains(last, "Not found on `dev` at "+second) {
			t.Fatalf("Bug #%d not marked gone on dev: %s", is.Number, last)
		}
	}
	// All clean: exit 0, and nothing is marked gone twice.
	writeReports(t, reports, map[string]string{"osv-scanner": `{"results":[]}`})
	n := len(sast.Comments)
	if code, res, errOut := verifySec(""); code != 0 || !res.Hold || len(sast.Comments) != n {
		t.Fatalf("clean run: %d %+v %s", code, res, errOut)
	}
	if len(other.Comments) != 0 {
		t.Fatalf("an unrelated Bug was touched: %+v", other.Comments)
	}
}

// Over MaxSecurityBugs new findings in one run: the first ten are filed and
// the rest are listed on the tenth, never dropped silently.
func TestVerifySecurityCapsNewBugs(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "--quiet", "-b", "main")
	_ = os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n"), 0o644)
	git(t, dir, "add", ".")
	git(t, dir, "commit", "--quiet", "-m", "feat: one")
	g := fake.New(t, "o/r", "User")
	bin, reports := securityTools(t)
	var results []string
	for i := range 12 {
		results = append(results, fmt.Sprintf(`{"check_id":"rule-%02d","path":"main.go","start":{"line":1},"extra":{"severity":"ERROR"}}`, i))
	}
	writeReports(t, reports, map[string]string{"semgrep": `{"results":[` + strings.Join(results, ",") + `],"errors":[]}`, "gitleaks": `[]`, "osv-scanner": `{"results":[]}`})
	env := map[string]string{"GITHUB_TOKEN": "workflow-token", "PATH": bin + string(os.PathListSeparator) + os.Getenv("PATH")}
	code, out, errOut := run(t, env, Options{GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir}, "verify", "--security", "--ci", "--repo", "o/r", "--json")
	var res SecurityResult
	_ = json.NewDecoder(strings.NewReader(out)).Decode(&res)
	if code != ExitFloor || len(res.Bugs) != MaxSecurityBugs || len(res.Unfiled) != 2 {
		t.Fatalf("cap: %d %+v %s", code, res, errOut)
	}
	last := g.Issues[res.Bugs[len(res.Bugs)-1].Number]
	if len(last.Comments) != 1 || !strings.Contains(last.Comments[0].Body, "sast rule-11 main.go") {
		t.Fatalf("unfiled findings not listed on the last Bug: %+v", last.Comments)
	}
}
