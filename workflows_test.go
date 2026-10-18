package aayudha

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"

	"github.com/rahulnakmol/agentic-ayudha/internal/initrepo"
)

// TestEveryWorkflowJobStartsWithHaltCheck holds the WP5.7 rule that `aayu halt` stops every workflow (Bug #173):
// each job with steps, in this repository's workflows and in the templates `init --gates` installs, starts with the
// halt check. The step runs only while vars.AAYUDHA_HALT is set, and its script must fail, so GitHub runs no
// later step of the job.
func TestEveryWorkflowJobStartsWithHaltCheck(t *testing.T) {
	var files []string
	for _, pattern := range []string{".github/workflows/*.yml", ".github/workflows/*.yaml", "internal/initrepo/*.yml", "internal/initrepo/*.yaml"} {
		m, _ := filepath.Glob(pattern)
		files = append(files, m...)
	}
	if len(files) < 10 {
		t.Fatalf("found only %d workflow files", len(files))
	}
	type step struct {
		Name string `yaml:"name"`
		If   string `yaml:"if"`
		Run  string `yaml:"run"`
	}
	var scripts []string
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var wf struct {
			Jobs map[string]struct {
				Steps []step `yaml:"steps"`
			} `yaml:"jobs"`
		}
		if err := yaml.Unmarshal([]byte(strings.ReplaceAll(string(b), "{{VERSION}}", "dev")), &wf); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		for name, job := range wf.Jobs {
			if len(job.Steps) == 0 {
				continue // a job that calls a reusable workflow has no steps of its own
			}
			s := job.Steps[0]
			if !strings.HasPrefix(s.Name, "Halt check") || s.If != "vars.AAYUDHA_HALT != ''" || !strings.Contains(s.Run, "exit ") {
				t.Errorf("%s: job %s does not start with the halt check step (aayu halt, WP5.7); its first step is %q", f, name, s.Name)
				continue
			}
			scripts = append(scripts, s.Run)
		}
	}
	if runtime.GOOS == "windows" {
		return
	}
	// The halt check's script, run as GitHub runs it while halted, fails with the halt message.
	for _, run := range scripts {
		cmd := exec.Command("bash", "-c", run)
		cmd.Env = append(os.Environ(), "AAYUDHA_HALT=halted by test at 2026-10-15T00:00:00Z")
		out, err := cmd.CombinedOutput()
		if err == nil || !strings.Contains(string(out), "Halted by aayu halt") {
			t.Errorf("halt check %q: want a failing exit and the halt message, got %v: %s", run, err, out)
		}
	}
}

// TestDevInstallRunsNoGitMaintenance: installing aayudha at dev fetches the module with git (GOPROXY=direct). Each
// git fetch in Go's module cache starts a detached `git maintenance run --auto`, whose repack rewrites .git/shallow
// while Go's next fetch reads it, so the install failed at random with "fatal: shallow file has changed since we
// read it". Every such step, here and in the templates `init --gates` installs, turns auto-maintenance off for the
// git that go install runs: the script is run with a stand-in go that prints the git config it would see.
func TestDevInstallRunsNoGitMaintenance(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash steps")
	}
	var files []string
	for _, pattern := range []string{".github/workflows/*.yml", "internal/initrepo/*.yml"} {
		m, _ := filepath.Glob(pattern)
		files = append(files, m...)
	}
	bin := t.TempDir()
	fake := "#!/bin/sh\ngit config --get maintenance.auto || echo unset\n"
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var wf struct {
			Jobs map[string]struct {
				Steps []struct {
					Run string `yaml:"run"`
				} `yaml:"steps"`
			} `yaml:"jobs"`
		}
		if err := yaml.Unmarshal([]byte(strings.ReplaceAll(string(b), "{{VERSION}}", "dev")), &wf); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		for name, job := range wf.Jobs {
			for _, s := range job.Steps {
				if !strings.Contains(s.Run, "GOPROXY=direct") || !strings.Contains(s.Run, "go install") {
					continue
				}
				found++
				cmd := exec.Command("bash", "-c", s.Run)
				cmd.Dir = t.TempDir()
				cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "AAYU_VERSION=dev")
				out, err := cmd.CombinedOutput()
				if err != nil || strings.TrimSpace(string(out)) != "false" {
					t.Errorf("%s: job %s installs dev with git auto-maintenance on (maintenance.auto = %q, %v)", f, name, strings.TrimSpace(string(out)), err)
				}
			}
		}
	}
	if found < 4 {
		t.Fatalf("found only %d dev install steps", found)
	}
}

// TestGitleaksScansOnlyTheCommitUnderTest: a secrets scan of a full-history checkout runs `gitleaks git`, which
// with no --log-opts scans `git log --all`: every fetched branch, so a planted test token on someone else's
// unmerged branch failed the security job of every PR. Each scan in the workflows and in the templates
// `init --gates` installs is run, in a repository whose HEAD has a clean history and whose other branch has the
// leak, with a stand-in gitleaks that lists the commits real gitleaks would read (its --log-opts, else --all).
func TestGitleaksScansOnlyTheCommitUnderTest(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash steps")
	}
	repo := t.TempDir()
	gitIn := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	gitIn("init", "-q", "-b", "dev")
	gitIn("commit", "-q", "--allow-empty", "-m", "base")
	base := gitIn("rev-parse", "HEAD")
	gitIn("checkout", "-q", "-b", "prep")
	gitIn("commit", "-q", "--allow-empty", "-m", "a token on another branch")
	other := gitIn("rev-parse", "HEAD")
	gitIn("checkout", "-q", "dev")
	bin := t.TempDir()
	fake := "#!/bin/sh\nopts=--all\nfor a in \"$@\"; do case $a in --log-opts=*) opts=${a#--log-opts=};; esac; done\nexec git log --format=%H $opts\n"
	if err := os.WriteFile(filepath.Join(bin, "gitleaks"), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, pattern := range []string{".github/workflows/*.yml", "internal/initrepo/*.yml"} {
		m, _ := filepath.Glob(pattern)
		files = append(files, m...)
	}
	gitleaks := regexp.MustCompile(`(?m)^\s*\S*gitleaks"? git .*$`)
	found := 0
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range gitleaks.FindAllString(string(b), -1) {
			found++
			cmd := exec.Command("bash", "-c", line)
			cmd.Dir = repo
			cmd.Env = append(os.Environ(), "RUNNER_TEMP="+bin, "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			out, err := cmd.CombinedOutput()
			if err != nil || !strings.Contains(string(out), base) || strings.Contains(string(out), other) {
				t.Errorf("%s: %q scans %q (%v); want HEAD's history only", f, strings.TrimSpace(line), out, err)
			}
		}
	}
	if found == 0 {
		t.Fatal("found no gitleaks git scan in the workflows")
	}
}

// TestSnapshotNamesItselfFromTheNewestRelease: goreleaser names a snapshot `incpatch .Version` from its current
// tag. v* tags sit on main's Release merge commits, which dev and rolling never contain, and a rolling commit
// carries a rolling-<date> tag, which goreleaser takes first and cannot parse as a version (release-snapshot failed
// on every rolling push). Each workflow running `release --snapshot` must first set GORELEASER_CURRENT_TAG to the
// newest v* tag by version, here v0.10.0: not an ancestor of HEAD, after v0.9.0 only in version order, above
// its prereleases v0.10.0-rc.1 and v0.10.0-rc.2; and v11.0.0, v10junk, and v99.0.0 do not count, since release.yml
// refuses a lightweight, malformed, or off-main tag.
func TestSnapshotNamesItselfFromTheNewestRelease(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash steps")
	}
	repo := t.TempDir()
	gitIn := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	gitIn("init", "-q", "-b", "rolling")
	gitIn("commit", "-q", "--allow-empty", "-m", "base")
	gitIn("checkout", "-q", "-b", "main")
	gitIn("commit", "-q", "--allow-empty", "-m", "Release: v0.9.0")
	gitIn("tag", "-a", "-m", "v0.9.0", "v0.9.0")
	gitIn("tag", "-a", "-m", "v0.10.0-rc.1", "v0.10.0-rc.1")
	gitIn("commit", "-q", "--allow-empty", "-m", "Release: v0.10.0")
	gitIn("tag", "-a", "-m", "v0.10.0", "v0.10.0")
	gitIn("tag", "-a", "-m", "v0.10.0-rc.2", "v0.10.0-rc.2") // tagged late; a prerelease still ranks below its release
	// Tags release.yml refuses never name a snapshot: lightweight, malformed, or off main.
	gitIn("tag", "v11.0.0")
	gitIn("tag", "-a", "-m", "v10junk", "v10junk")
	gitIn("update-ref", "refs/remotes/origin/main", "main") // the checkout's fetch-depth 0 fetches origin/main
	gitIn("checkout", "-q", "rolling")
	gitIn("commit", "-q", "--allow-empty", "-m", "Rolling snapshot")
	gitIn("tag", "rolling-2026-10-16.2")
	gitIn("tag", "-a", "-m", "v99.0.0", "v99.0.0")
	step := regexp.MustCompile(`(?m)^\s*run: (.*GORELEASER_CURRENT_TAG=.*)$`)
	files, _ := filepath.Glob(".github/workflows/*.yml")
	found := 0
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if snap := strings.Index(string(b), "release --snapshot"); snap >= 0 {
			if m := step.FindStringSubmatchIndex(string(b)); m == nil || m[0] > snap {
				t.Errorf("%s runs `goreleaser release --snapshot` without first setting GORELEASER_CURRENT_TAG", f)
			}
		}
		for _, m := range step.FindAllStringSubmatch(string(b), -1) {
			found++
			env := filepath.Join(t.TempDir(), "env")
			cmd := exec.Command("bash", "-c", m[1])
			cmd.Dir = repo
			cmd.Env = append(os.Environ(), "GITHUB_ENV="+env)
			out, err := cmd.CombinedOutput()
			got, _ := os.ReadFile(env)
			if err != nil || strings.TrimSpace(string(got)) != "GORELEASER_CURRENT_TAG=v0.10.0" {
				t.Errorf("%s: %q wrote %q (%v, %s); want GORELEASER_CURRENT_TAG=v0.10.0", f, m[1], got, err, out)
			}
		}
	}
	if found == 0 {
		t.Fatal("found no GORELEASER_CURRENT_TAG step in the workflows")
	}
}

// Founder decisions of 2026-10-15 and 2026-10-16 (Appendix B S-4.8c, S-2.9b, D-2.9b): product CI never calls a model,
// and this repository's model checks never gate. Every workflow `init --gates` installs holds no model key or
// AI-gateway environment; the four workflows that call the gateway for aayudha's own model code run on no pull
// request event (so none can be a required check of a PR), and ci.yml's gates (PR, rolling, stable) never name them.
func TestModelChecksNeverGateAndNeverShip(t *testing.T) {
	keys := regexp.MustCompile(`ai-gateway|AAYU_SECRET_|AAYU_MODELS__|AAYU_AI_GATEWAY`)
	for name, b := range initrepo.GateWorkflows("dev") {
		if m := keys.Find(b); m != nil {
			t.Errorf("init --gates installs %s, which names %q: product CI never calls a model", name, m)
		}
	}
	ci, err := os.ReadFile(".github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"inspect-live", "aayudha-model-lanes", "models-live", "ux-review-live"} {
		b, err := os.ReadFile(".github/workflows/" + f + ".yml")
		if err != nil {
			t.Fatal(err)
		}
		var wf struct {
			On   map[string]any `yaml:"on"`
			Jobs map[string]struct {
				Name string `yaml:"name"`
			} `yaml:"jobs"`
		}
		if err := yaml.Unmarshal(b, &wf); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		for ev := range wf.On {
			if strings.HasPrefix(ev, "pull_request") {
				t.Errorf("%s runs on %s, so it could become a required check", f, ev)
			}
		}
		names := []string{f + ".yml"} // and each job's check name (a job ID such as "live" is an ordinary word)
		for _, j := range wf.Jobs {
			names = append(names, j.Name)
		}
		for _, n := range names {
			if n != "" && strings.Contains(string(ci), n) {
				t.Errorf("ci.yml names %q of %s: a model check must not be part of a gate", n, f)
			}
		}
	}
}

// TestEvalStepsAreSavedAndResumed: tier 2 saves each run's steps under evals/results/steps/<group>, uploads them
// even when the job fails, and a dispatch with resume_run downloads exactly that artifact of the earlier run into the
// same directory. The resume script is run with a stand-in gh that answers for run 4242 at commit abc123.
func TestEvalStepsAreSavedAndResumed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash steps")
	}
	b, err := os.ReadFile(".github/workflows/evals.yml")
	if err != nil {
		t.Fatal(err)
	}
	type step struct {
		Name string            `yaml:"name"`
		Uses string            `yaml:"uses"`
		If   string            `yaml:"if"`
		Run  string            `yaml:"run"`
		With map[string]string `yaml:"with"`
	}
	var wf struct {
		On struct {
			Dispatch struct {
				Inputs map[string]any `yaml:"inputs"`
			} `yaml:"workflow_dispatch"`
		} `yaml:"on"`
		Jobs map[string]struct {
			Steps []step `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(b, &wf); err != nil {
		t.Fatal(err)
	}
	if _, ok := wf.On.Dispatch.Inputs["resume_run"]; !ok {
		t.Error("evals.yml has no resume_run dispatch input")
	}
	var resume, upload string
	saves := false
	for _, s := range wf.Jobs["tier2"].Steps {
		switch {
		case strings.Contains(s.Run, "go run ./internal/evals/cmd/eval -tier 2"):
			saves = strings.Contains(s.Run, `-steps "evals/results/steps/$GROUP"`)
		case strings.Contains(s.Run, "eval-steps-"):
			resume = s.Run
		case strings.HasPrefix(s.Uses, "actions/upload-artifact@") && s.With["path"] == "evals/results/steps/${{ matrix.group }}":
			if s.If != "always()" {
				t.Errorf("the steps upload runs only %q; a failed job must still upload what it saved", s.If)
			}
			upload = s.With["name"]
		}
	}
	if !saves || upload != "eval-steps-${{ github.sha }}-${{ matrix.group }}" || resume == "" {
		t.Fatalf("tier 2 saves steps %v, uploads %q, resume step %v", saves, upload, resume != "")
	}
	bin, work := t.TempDir(), t.TempDir()
	gh := "#!/bin/sh\ncase \"$1 $2\" in\n\"run view\") echo abc123 ;;\n\"run download\") echo \"$@\" >> \"$GH_LOG\" ;;\nesac\n"
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(gh), 0o755); err != nil {
		t.Fatal(err)
	}
	logf := filepath.Join(work, "gh.log")
	run := func(id string) (string, error) {
		cmd := exec.Command("bash", "-c", resume)
		cmd.Dir = work
		cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "GH_LOG="+logf, "RESUME="+id, "GROUP=survey")
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	if out, err := run("4242"); err != nil {
		t.Fatalf("resume from run 4242: %v: %s", err, out)
	}
	got, _ := os.ReadFile(logf)
	if want := "run download 4242 --name eval-steps-abc123-survey --dir evals/results/steps/survey"; strings.TrimSpace(string(got)) != want {
		t.Errorf("downloaded %q, want %q (the name the upload gives run 4242's survey steps)", strings.TrimSpace(string(got)), want)
	}
	_ = os.Remove(logf)
	if out, err := run("4242 --repo other/repo"); err == nil {
		t.Errorf("a resume_run that is not a run ID was accepted: %s", out)
	}
	if _, err := os.Stat(logf); err == nil {
		t.Error("a resume_run that is not a run ID still downloaded")
	}
}

// TestProtectedPathsRunsCollapse: every PR push, edit, and review used to queue its own protected-paths family run
// (115 queued at once, PRs waiting 20 minutes for the required check). Superseded runs now cancel, but a required
// check must still report on the latest head and review state: the groups and conditions are evaluated here
// against event payloads, as GitHub evaluates them.
func TestProtectedPathsRunsCollapse(t *testing.T) {
	type wfile struct {
		Concurrency struct {
			Group  string `yaml:"group"`
			Cancel bool   `yaml:"cancel-in-progress"`
		} `yaml:"concurrency"`
		Jobs map[string]struct {
			If string `yaml:"if"`
		} `yaml:"jobs"`
	}
	load := func(f string) wfile {
		t.Helper()
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var wf wfile
		if err := yaml.Unmarshal([]byte(strings.ReplaceAll(string(b), "{{VERSION}}", "dev")), &wf); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		if !wf.Concurrency.Cancel && !strings.HasSuffix(f, "protected-paths-review.yml") { // a review signal is one echo
			t.Errorf("%s: superseded runs must cancel (concurrency.cancel-in-progress: true)", f)
		}
		return wf
	}
	group := func(wf wfile, ctx map[string]any) string { return ghTemplate(t, wf.Concurrency.Group, ctx) }
	pr := func(number float64, head string, attempt string) map[string]any {
		return map[string]any{"github": map[string]any{"run_attempt": attempt, "run_id": "r" + head + attempt,
			"event_name": "pull_request_target",
			"event":      map[string]any{"pull_request": map[string]any{"number": number, "head": map[string]any{"sha": head}}}}}
	}

	gate := load(".github/workflows/protected-paths.yml")
	if group(gate, pr(7, "s2", "1")) != group(gate, pr(7, "s1", "1")) {
		t.Error("protected-paths: a push does not cancel the PR's run for its older head")
	}
	if group(gate, pr(7, "s2", "2")) == group(gate, pr(7, "s3", "1")) || group(gate, pr(7, "s1", "2")) == group(gate, pr(7, "s2", "1")) {
		t.Error("protected-paths: a re-run of an older head's run (protected-paths-rerun) can cancel the newest head's only run")
	}
	if group(gate, pr(7, "s2", "2")) != group(gate, pr(7, "s2", "3")) {
		t.Error("protected-paths: re-runs of one head do not collapse")
	}
	if group(gate, pr(7, "s1", "1")) == group(gate, pr(8, "s1", "1")) {
		t.Error("protected-paths: two PRs share a group")
	}

	signal := load(".github/workflows/protected-paths-review.yml").Jobs["signal"].If
	review := func(login, typ, state string) map[string]any {
		return map[string]any{"github": map[string]any{"event": map[string]any{
			"review":       map[string]any{"user": map[string]any{"login": login, "type": typ}, "state": state},
			"pull_request": map[string]any{"user": map[string]any{"login": "rahulnakmol"}}}}}
	}
	for _, c := range []struct {
		login, typ, state string
		want              bool
	}{
		{"chatgpt-codex-connector[bot]", "Bot", "commented", false},
		{"some-app[bot]", "Bot", "approved", false},
		{"machine[bot]", "User", "approved", false},
		{"RahulNakmol", "User", "approved", false}, // the author, in another case
		{"alice", "User", "commented", false},
		{"alice", "User", "approved", true},
		{"alice", "User", "changes_requested", true},
		{"alice", "User", "dismissed", true}, // a dismissed approval can turn the gate red
	} {
		if got := ghTruthy(ghEval(t, signal, review(c.login, c.typ, c.state))); got != c.want {
			t.Errorf("protected-paths-review: a %s review by %s (%s) signals %t; want %t", c.state, c.login, c.typ, got, c.want)
		}
	}

	rerunWF := load(".github/workflows/protected-paths-rerun.yml")
	run := func(conclusion, sha, id string) map[string]any {
		return map[string]any{"github": map[string]any{"run_id": id,
			"event": map[string]any{"workflow_run": map[string]any{"conclusion": conclusion, "head_sha": sha}}}}
	}
	if ghTruthy(ghEval(t, rerunWF.Jobs["rerun"].If, run("skipped", "s1", "1"))) || !ghTruthy(ghEval(t, rerunWF.Jobs["rerun"].If, run("success", "s1", "1"))) {
		t.Error("protected-paths-rerun: must re-run after a successful signal only")
	}
	if group(rerunWF, run("success", "s1", "1")) != group(rerunWF, run("success", "s1", "2")) {
		t.Error("protected-paths-rerun: re-runs for one head do not collapse")
	}
	if g := group(rerunWF, run("skipped", "s1", "3")); g == group(rerunWF, run("success", "s1", "1")) || g == group(rerunWF, run("skipped", "s1", "4")) {
		t.Error("protected-paths-rerun: a skipped signal's no-op run can cancel a real re-run")
	}

	for _, f := range []string{".github/workflows/aayudha-merge-gate.yml", "internal/initrepo/aayudha-merge-gate.yml"} {
		mg := load(f)
		comment := map[string]any{"github": map[string]any{"event_name": "issue_comment", "event": map[string]any{"issue": map[string]any{"number": 7.0}}}}
		if group(mg, pr(7, "s2", "1")) != group(mg, pr(7, "s1", "1")) || group(mg, pr(7, "s1", "1")) == group(mg, pr(8, "s1", "1")) || group(mg, comment) == group(mg, pr(7, "s1", "1")) {
			t.Errorf("%s: the gate must collapse per PR, apart from the reply runs", f)
		}
	}
}

// ghTemplate expands each ${{ }} in s.
func ghTemplate(t *testing.T, s string, ctx map[string]any) string {
	t.Helper()
	var b strings.Builder
	for {
		i := strings.Index(s, "${{")
		if i < 0 {
			return b.String() + s
		}
		j := strings.Index(s[i:], "}}")
		if j < 0 {
			t.Fatalf("unterminated expression in %q", s)
		}
		b.WriteString(s[:i])
		b.WriteString(ghString(ghEval(t, s[i+3:i+j], ctx)))
		s = s[i+j+2:]
	}
}

func ghString(v any) string {
	switch v := v.(type) {
	case nil:
		return ""
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return fmt.Sprint(v)
	}
}

func ghTruthy(v any) bool {
	switch v := v.(type) {
	case nil:
		return false
	case bool:
		return v
	case string:
		return v != ""
	case float64:
		return v != 0
	}
	return true
}

// ghEval evaluates the GitHub Actions expression subset the workflows use: literals, context paths, ! == != && ||,
// parentheses, endsWith, and format. As in GitHub, string comparison ignores case and && and || yield an operand.
func ghEval(t *testing.T, src string, ctx map[string]any) any {
	t.Helper()
	toks := regexp.MustCompile(`'(?:[^']|'')*'|&&|\|\||==|!=|[!(),]|[A-Za-z_][\w.\-]*|\d+(?:\.\d+)?`).FindAllString(src, -1)
	pos := 0
	peek := func() string {
		if pos < len(toks) {
			return toks[pos]
		}
		return ""
	}
	var or func() any
	var primary func() any
	primary = func() any {
		tok := peek()
		pos++
		switch {
		case tok == "(":
			v := or()
			pos++ // )
			return v
		case tok == "!":
			return !ghTruthy(primary())
		case strings.HasPrefix(tok, "'"):
			return strings.ReplaceAll(tok[1:len(tok)-1], "''", "'")
		case tok == "true" || tok == "false":
			return tok == "true"
		case tok == "null":
			return nil
		case tok != "" && tok[0] >= '0' && tok[0] <= '9':
			f, _ := strconv.ParseFloat(tok, 64)
			return f
		case peek() == "(":
			pos++
			var args []any
			for peek() != ")" {
				args = append(args, or())
				if peek() == "," {
					pos++
				}
			}
			pos++
			switch strings.ToLower(tok) {
			case "endswith":
				return strings.HasSuffix(strings.ToLower(ghString(args[0])), strings.ToLower(ghString(args[1])))
			case "format":
				out := ghString(args[0])
				for i, a := range args[1:] {
					out = strings.ReplaceAll(out, "{"+strconv.Itoa(i)+"}", ghString(a))
				}
				return out
			}
			t.Fatalf("unsupported function %s in %q", tok, src)
		}
		var v any = ctx
		for _, k := range strings.Split(tok, ".") {
			m, _ := v.(map[string]any)
			v = m[k]
		}
		return v
	}
	cmp := func() any {
		l := primary()
		if op := peek(); op == "==" || op == "!=" {
			pos++
			eq := strings.EqualFold(ghString(l), ghString(primary()))
			return eq == (op == "==")
		}
		return l
	}
	and := func() any {
		v := cmp()
		for peek() == "&&" {
			pos++
			r := cmp()
			if ghTruthy(v) {
				v = r
			}
		}
		return v
	}
	or = func() any {
		v := and()
		for peek() == "||" {
			pos++
			r := and()
			if !ghTruthy(v) {
				v = r
			}
		}
		return v
	}
	v := or()
	if pos != len(toks) {
		t.Fatalf("could not parse %q at %q", src, peek())
	}
	return v
}
