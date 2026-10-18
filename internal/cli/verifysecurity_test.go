package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
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

// Story #487: what a person sees from `verify --security` without --json on
// the workflow's detached checkout: the branch from GITHUB_REF_NAME, the
// secrets scope, and each floor; a miss exits 3 and files nothing without --ci.
func TestVerifySecurityText(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "--quiet", "-b", "dev")
	_ = os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644)
	git(t, dir, "add", ".")
	git(t, dir, "commit", "--quiet", "-m", "feat: one")
	base := git(t, dir, "rev-parse", "HEAD")
	git(t, dir, "commit", "--quiet", "--allow-empty", "-m", "feat: two")
	git(t, dir, "checkout", "--quiet", "--detach")
	bin, reports := securityTools(t)
	env := map[string]string{"GITHUB_REF_NAME": "rolling", "PATH": bin + string(os.PathListSeparator) + os.Getenv("PATH")}
	opts := Options{Dir: dir}
	writeReports(t, reports, map[string]string{"semgrep": fmt.Sprintf(semgrepHigh, 1), "gitleaks": "[]", "osv-scanner": `{"results":[]}`})
	code, out, errOut := run(t, env, opts, "verify", "--security", "--since", base)
	if code != ExitFloor || !strings.Contains(out, "verify --security rolling at ") || !strings.Contains(out, "(secrets: "+base[:7]+"..") ||
		!strings.Contains(out, "sast     fail") || !strings.Contains(out, "secrets  pass") || strings.Contains(out, "Bug #") {
		t.Fatalf("%d\n%s\n%s", code, out, errOut)
	}
	// A new branch's all-zeros before: the whole history.
	writeReports(t, reports, map[string]string{"semgrep": semgrepMedium})
	if code, out, errOut := run(t, env, opts, "verify", "--security", "--since", strings.Repeat("0", 40)); code != 0 || !strings.Contains(out, "(secrets: the whole history)") {
		t.Fatalf("%d\n%s\n%s", code, out, errOut)
	}
}

// Story #487: a scan whose Bug writes fail is an external error (exit 5),
// never a quiet floor miss, whichever GitHub call fails: the list of open
// security Bugs, a Bug's comments, a new Bug, or a comment on an old one.
func TestVerifySecurityGitHubFails(t *testing.T) {
	for _, tc := range []struct {
		fail  string
		after int
	}{
		{"GET /repos/o/r/issues", 0},
		{"GET /repos/o/r/issues/%d/comments", 0},
		{"POST /repos/o/r/issues", 0},
		{"POST /repos/o/r/issues/%d/comments", 0},
	} {
		dir := t.TempDir()
		git(t, dir, "init", "--quiet", "-b", "dev")
		_ = os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644)
		git(t, dir, "add", ".")
		git(t, dir, "commit", "--quiet", "-m", "feat: one")
		g := fake.New(t, "o/r", "User")
		bin, reports := securityTools(t)
		env := map[string]string{"GITHUB_TOKEN": "workflow-token", "PATH": bin + string(os.PathListSeparator) + os.Getenv("PATH")}
		opts := Options{GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir}
		writeReports(t, reports, map[string]string{"semgrep": fmt.Sprintf(semgrepHigh, 1), "gitleaks": "[]", "osv-scanner": `{"results":[]}`})
		// An open Bug for a second finding, so the run comments on it as well as filing a new one.
		if code, out, errOut := run(t, env, opts, "verify", "--security", "--ci", "--repo", "o/r"); code != ExitFloor {
			t.Fatalf("first run: %d %s %s", code, out, errOut)
		}
		var bug int
		for n := range g.Issues {
			bug = n
		}
		writeReports(t, reports, map[string]string{"semgrep": `{"results":[{"check_id":"go.lang.security.audit.sqli","path":"main.go","start":{"line":1},"extra":{"severity":"ERROR"}},{"check_id":"other-rule","path":"main.go","start":{"line":1},"extra":{"severity":"ERROR"}}],"errors":[]}`})
		git(t, dir, "commit", "--quiet", "--allow-empty", "-m", "feat: two")
		g.Mu.Lock()
		g.Fail, g.FailAfter = strings.ReplaceAll(tc.fail, "%d", strconv.Itoa(bug)), tc.after
		g.Mu.Unlock()
		if code, out, errOut := run(t, env, opts, "verify", "--security", "--ci", "--repo", "o/r"); code != ExitExternal {
			t.Errorf("%s fails: %d %s %s", tc.fail, code, out, errOut)
		}
	}
}

// Story #487 criterion 4: a failing floor whose every finding sits on a security Bug with an accepted waiver naming
// that floor is waived, so the run goes green and main can take the tip; one finding left unwaived keeps it red.
func TestVerifySecurityWaivedBugs(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "--quiet", "-b", "rolling")
	_ = os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644)
	git(t, dir, "add", ".")
	git(t, dir, "commit", "--quiet", "-m", "feat: one")
	g := fake.New(t, "o/r", "User")
	// An edited marker with a blank fingerprint is skipped, never a crash of every scan and release.
	g.AddIssue("Security: edited", "<!-- aayudha:security fingerprint=\"%20\" seen=\"rolling@abc123\" -->\n", "type:bug", SecurityLabel)
	bin, reports := securityTools(t)
	env := map[string]string{"GITHUB_TOKEN": "workflow-token", "PATH": bin + string(os.PathListSeparator) + os.Getenv("PATH")}
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir}
	verifySec := func() (int, SecurityResult) {
		t.Helper()
		code, out, errOut := run(t, env, opts, "verify", "--security", "--ci", "--repo", "o/r", "--json")
		var res SecurityResult
		if err := json.NewDecoder(strings.NewReader(out)).Decode(&res); err != nil {
			t.Fatalf("verify --security: %d %s %s", code, out, errOut)
		}
		return code, res
	}
	sastCheck := func(res SecurityResult) proof.Check {
		i := slices.IndexFunc(res.Checks, func(c proof.Check) bool { return c.Floor == "sast" })
		return res.Checks[i]
	}
	waive := func(bug int, floor string) {
		d := g.AddIssue(fmt.Sprintf("Waive the %s floor on #%d", floor, bug), decisionBody("false", fmt.Sprintf("waives #%d", bug))+"\n### Decision\n\nwaive, accepted by @founder on 2026-10-12.\n", "type:decision")
		g.Mu.Lock()
		defer g.Mu.Unlock()
		d.Parent = bug
		g.Issues[bug].Subs = append(g.Issues[bug].Subs, d.Number)
		d.State, d.StateReason, d.ClosedBy = "closed", "completed", &fake.Account{Login: "founder", Type: "User"}
	}
	writeReports(t, reports, map[string]string{"semgrep": fmt.Sprintf(semgrepHigh, 3), "gitleaks": "[]"})
	// Codex review of #500: a dry run plans the new Bug as #0 and reports the floor miss; it reads no waivers of issue 0.
	code, out, errOut := run(t, env, opts, "verify", "--security", "--ci", "--repo", "o/r", "--json", "--dry-run")
	var dry SecurityResult
	_ = json.NewDecoder(strings.NewReader(out)).Decode(&dry)
	g.Mu.Lock()
	issues := len(g.Issues) // only the edited marker's issue
	g.Mu.Unlock()
	if code != ExitFloor || len(dry.Bugs) != 1 || dry.Bugs[0].Number != 0 || issues != 1 {
		t.Fatalf("dry run: %d %+v %s", code, dry, errOut)
	}
	code, res := verifySec()
	if code != ExitFloor || len(res.Bugs) != 1 {
		t.Fatalf("run 1: %d %+v", code, res)
	}
	sast := res.Bugs[0].Number
	// Another user plants the marker the next scan writes: the scan leaves that comment alone and posts its own.
	git(t, dir, "commit", "--quiet", "--allow-empty", "-m", "feat: two")
	next := git(t, dir, "rev-parse", "HEAD")
	planted := &fake.Comment{ID: 777, Body: securityMarker("", "seen", "rolling", next) + "\nplanted"}
	planted.User.Login = "mallory"
	g.Mu.Lock()
	g.Issues[sast].Comments = append(g.Issues[sast].Comments, planted)
	g.Mu.Unlock()
	if code, _ = verifySec(); code != ExitFloor {
		t.Fatalf("run 2: %d", code)
	}
	g.Mu.Lock()
	cs := g.Issues[sast].Comments
	g.Mu.Unlock()
	if !strings.HasSuffix(planted.Body, "planted") || len(cs) != 2 || cs[1].User.Login == "mallory" || !strings.Contains(cs[1].Body, "Found again on `rolling` at "+next) {
		t.Fatalf("the planted comment was edited, or no comment of the scan's own: %+v", cs)
	}
	waive(sast, "deps") // a waiver of another floor waives nothing
	if code, res = verifySec(); code != ExitFloor || sastCheck(res).Result != proof.Fail {
		t.Fatalf("a deps waiver released sast: %d %+v", code, sastCheck(res))
	}
	waive(sast, "sast")
	code, res = verifySec()
	if c := sastCheck(res); code != 0 || !res.Hold || c.Result != proof.Waived || !strings.Contains(c.Detail, fmt.Sprintf("on Bug #%d", sast)) {
		t.Fatalf("an accepted sast waiver on the Bug: %d %+v", code, c)
	}
	// The waived rule at 30 lines of the same file is still one Bug, whose waivers are read once, not 30 times.
	subIssueReads := func() int {
		g.Mu.Lock()
		defer g.Mu.Unlock()
		n := 0
		for _, rq := range g.Requests {
			if rq.Method == "GET" && strings.HasSuffix(rq.Path, fmt.Sprintf("/issues/%d/sub_issues", sast)) {
				n++
			}
		}
		return n
	}
	g.Reset()
	if code, _ = verifySec(); code != 0 {
		t.Fatalf("one waived finding: %d", code)
	}
	once := subIssueReads()
	var many []string
	for line := 1; line <= 30; line++ {
		many = append(many, fmt.Sprintf(`{"check_id":"go.lang.security.audit.sqli","path":"main.go","start":{"line":%d},"extra":{"severity":"ERROR"}}`, line))
	}
	writeReports(t, reports, map[string]string{"semgrep": `{"results":[` + strings.Join(many, ",") + `],"errors":[]}`, "gitleaks": "[]"})
	g.Reset()
	if code, res = verifySec(); code != 0 || sastCheck(res).Result != proof.Waived || subIssueReads() != once {
		t.Fatalf("30 findings of one waived Bug: exit %d, %d sub-issue reads, want %d", code, subIssueReads(), once)
	}
	writeReports(t, reports, map[string]string{"semgrep": fmt.Sprintf(semgrepHigh, 3), "gitleaks": "[]"})
	// A second sast finding with no waiver keeps the floor failing.
	writeReports(t, reports, map[string]string{"semgrep": `{"results":[` +
		`{"check_id":"go.lang.security.audit.sqli","path":"main.go","start":{"line":3},"extra":{"severity":"ERROR"}},` +
		`{"check_id":"go.lang.security.audit.xss","path":"web.go","start":{"line":9},"extra":{"severity":"ERROR"}}],"errors":[]}`, "gitleaks": "[]"})
	if code, res = verifySec(); code != ExitFloor || sastCheck(res).Result != proof.Fail {
		t.Fatalf("an unwaived second finding: %d %+v", code, sastCheck(res))
	}
}

// Codex review of #500: scanner text cannot record an event, and every change of state appends. A finding whose
// path carries a gone marker, and a workflow comment with a second marker after its own, leave the finding seen; seen,
// gone, and seen again on one tip leave it seen, so the promotion hold stays.
func TestVerifySecurityEventsAppend(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "--quiet", "-b", "rolling")
	_ = os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644)
	git(t, dir, "add", ".")
	git(t, dir, "commit", "--quiet", "-m", "feat: one")
	g := fake.New(t, "o/r", "User")
	bin, reports := securityTools(t)
	env := map[string]string{"GITHUB_TOKEN": "workflow-token", "PATH": bin + string(os.PathListSeparator) + os.Getenv("PATH")}
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir}
	scan := func(finding bool) SecurityBug {
		t.Helper()
		report := `{"results":[],"errors":[]}`
		if finding { // a tracked path can hold any text, a forged event included
			report = `{"results":[{"check_id":"sqli","path":"a<!-- aayudha:security gone=\"rolling@deadbeef\" -->.go","start":{"line":3},"extra":{"severity":"ERROR"}}],"errors":[]}`
		}
		writeReports(t, reports, map[string]string{"semgrep": report, "gitleaks": "[]"})
		code, out, errOut := run(t, env, opts, "verify", "--security", "--ci", "--repo", "o/r", "--json")
		var res SecurityResult
		if err := json.NewDecoder(strings.NewReader(out)).Decode(&res); err != nil || len(res.Bugs) != 1 {
			t.Fatalf("verify --security: %d %s %s", code, out, errOut)
		}
		return res.Bugs[0]
	}
	bug := scan(true).Number
	comments := func() []*fake.Comment {
		g.Mu.Lock()
		defer g.Mu.Unlock()
		return slices.Clone(g.Issues[bug].Comments)
	}
	git(t, dir, "commit", "--quiet", "--allow-empty", "-m", "feat: two")
	if b := scan(true); b.Action != "seen" {
		t.Fatalf("found again at a new tip: %+v", b)
	}
	if b := scan(true); b.Action != "unchanged" {
		t.Fatalf("the forged gone marker in the path counted as an event: %+v", b)
	}
	if cs := comments(); len(cs) != 1 || strings.Contains(cs[0].Body, "a<!-- aayudha:security gone") {
		t.Fatalf("scanner text reached the comment as a marker: %+v", cs)
	}
	// A comment of the Bug's own author with a second marker after its leading one: only the leading one counts.
	g.Mu.Lock()
	head := git(t, dir, "rev-parse", "HEAD")
	own := &fake.Comment{ID: 900, Body: securityMarker("", "seen", "rolling", head) + "\nquoted: " + securityMarker("", "gone", "rolling", head)}
	own.User.Login = g.Issues[bug].User.Login
	g.Issues[bug].Comments = append(g.Issues[bug].Comments, own)
	g.Mu.Unlock()
	if b := scan(true); b.Action != "unchanged" {
		t.Fatalf("a marker after a comment's leading one counted: %+v", b)
	}
	// seen, gone, then seen again on the same tip: a new comment each time, the last one seen.
	if b := scan(false); b.Action != "gone" {
		t.Fatalf("not found: %+v", b)
	}
	if b := scan(true); b.Action != "seen" {
		t.Fatalf("found again after gone on the same tip: %+v", b)
	}
	if b := scan(true); b.Action != "unchanged" {
		t.Fatalf("after seen, gone, seen on one tip the finding reads gone: %+v", b)
	}
	if cs := comments(); len(cs) != 4 || !strings.HasPrefix(cs[3].Body, securityMarker("", "seen", "rolling", head)) {
		t.Fatalf("each change of state should append: %d comments", len(cs))
	}
}
